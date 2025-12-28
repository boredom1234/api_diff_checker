package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api_diff_checker/config"
	"api_diff_checker/core"
)

const (
	// MaxRequestBodySize limits the request body to 10MB
	MaxRequestBodySize = 10 * 1024 * 1024

	// Server timeouts
	ReadTimeout     = 30 * time.Second
	WriteTimeout    = 5 * time.Minute // Long timeout for slow API responses
	IdleTimeout     = 60 * time.Second
	ShutdownTimeout = 30 * time.Second
)

type Server struct {
	Engine     *core.Engine
	httpServer *http.Server
}

func Start(engine *core.Engine) error {
	s := &Server{Engine: engine}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./static")))
	mux.HandleFunc("/api/run", s.corsMiddleware(s.handleRun))
	mux.HandleFunc("/api/health", s.corsMiddleware(s.handleHealth))

	s.httpServer = &http.Server{
		Addr:         ":9876",
		Handler:      mux,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		IdleTimeout:  IdleTimeout,
	}

	// Handle graceful shutdown
	go s.handleShutdown()

	fmt.Println("Server listening at http://localhost:9876")
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s *Server) handleShutdown() {
	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		fmt.Printf("Error during shutdown: %v\n", err)
	}
	fmt.Println("Server stopped.")
}

// corsMiddleware adds CORS headers to responses
func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	// Read body with size check
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if err.Error() == "http: request body too large" {
			s.errorResponse(w, "Request body too large (max 10MB)", http.StatusRequestEntityTooLarge)
		} else {
			s.errorResponse(w, "Failed to read request body: "+err.Error(), http.StatusBadRequest)
		}
		return
	}

	if len(body) == 0 {
		s.errorResponse(w, "Empty request body", http.StatusBadRequest)
		return
	}

	var cfg config.Config
	if err := json.Unmarshal(body, &cfg); err != nil {
		s.errorResponse(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate config
	validation := cfg.Validate()
	if !validation.IsValid() {
		s.errorResponse(w, "Validation failed: "+validation.Error(), http.StatusBadRequest)
		return
	}

	// Log warnings if any
	for _, warning := range validation.Warnings {
		fmt.Printf("[WARN] Config: %s\n", warning)
	}

	// Create context with timeout based on number of commands and versions
	// Allow more time for larger configurations
	estimatedTime := time.Duration(len(cfg.Commands)*len(cfg.Versions)) * cfg.GetTimeout()
	if estimatedTime < time.Minute {
		estimatedTime = time.Minute
	}
	if estimatedTime > WriteTimeout {
		estimatedTime = WriteTimeout - time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), estimatedTime)
	defer cancel()

	result, err := s.Engine.RunWithContext(ctx, &cfg)
	if err != nil && result == nil {
		s.errorResponse(w, "Execution failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Even if there was an error, we might have partial results
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		// Log the error but can't send response at this point
		fmt.Printf("[ERROR] Failed to encode response: %v\n", err)
	}
}

func (s *Server) errorResponse(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

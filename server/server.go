package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"api_diff_checker/config"
	"api_diff_checker/core"
)

type Server struct {
	Engine *core.Engine
}

func Start(engine *core.Engine) error {
	s := &Server{Engine: engine}

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/api/run", s.handleRun)

	fmt.Println("Server listening at http://localhost:9876")
	return http.ListenAndServe(":9876", nil)
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid JSON config: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate config a bit
	if len(cfg.Versions) == 0 {
		http.Error(w, "No versions specified", http.StatusBadRequest)
		return
	}
	if len(cfg.Commands) == 0 {
		http.Error(w, "No commands specified", http.StatusBadRequest)
		return
	}

	result, err := s.Engine.Run(&cfg)
	if err != nil {
		http.Error(w, "Execution failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Level        string    `json:"level"`
	Version      string    `json:"version,omitempty"`
	Command      string    `json:"command,omitempty"`
	Message      string    `json:"message"`
	ErrorDetails string    `json:"error_details,omitempty"`
}

type Logger struct {
	mu       sync.Mutex
	LogFile  *os.File
	filePath string
	toStdOut bool
	maxSize  int64 // Maximum log file size in bytes (0 = no limit)
}

const (
	// DefaultMaxLogSize is 10MB
	DefaultMaxLogSize = 10 * 1024 * 1024
)

// New creates a new logger that writes to the specified file
func New(logPath string, toStdOut bool) (*Logger, error) {
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	return &Logger{
		LogFile:  f,
		filePath: logPath,
		toStdOut: toStdOut,
		maxSize:  DefaultMaxLogSize,
	}, nil
}

// NewWithMaxSize creates a logger with a custom max size
func NewWithMaxSize(logPath string, toStdOut bool, maxSize int64) (*Logger, error) {
	logger, err := New(logPath, toStdOut)
	if err != nil {
		return nil, err
	}
	logger.maxSize = maxSize
	return logger, nil
}

// Log writes a log entry to the file and optionally to stdout
func (l *Logger) Log(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Check if log rotation is needed
	if l.maxSize > 0 {
		if err := l.checkRotation(); err != nil {
			// Log rotation error to stderr as fallback
			fmt.Fprintf(os.Stderr, "[LOGGER ERROR] Failed to rotate log: %v\n", err)
		}
	}

	// File output (JSON)
	data, err := json.Marshal(entry)
	if err != nil {
		// Log marshal error to stderr
		fmt.Fprintf(os.Stderr, "[LOGGER ERROR] Failed to marshal log entry: %v\n", err)
		return
	}

	if _, err := l.LogFile.Write(data); err != nil {
		// Log write error to stderr as fallback
		fmt.Fprintf(os.Stderr, "[LOGGER ERROR] Failed to write to log file: %v\n", err)
		// Also print the original log entry to stderr so it's not lost
		fmt.Fprintf(os.Stderr, "[FALLBACK] %s: %s\n", entry.Level, entry.Message)
	}

	if _, err := l.LogFile.WriteString("\n"); err != nil {
		fmt.Fprintf(os.Stderr, "[LOGGER ERROR] Failed to write newline to log file: %v\n", err)
	}

	// Terminal output (human-readable)
	if l.toStdOut {
		l.printToStdout(entry)
	}
}

// printToStdout writes a human-readable log entry to stdout
func (l *Logger) printToStdout(entry LogEntry) {
	if entry.Level == "ERROR" {
		fmt.Printf("[ERROR] %v: %s", entry.Version, entry.Message)
		if entry.ErrorDetails != "" {
			fmt.Printf(" - %s", entry.ErrorDetails)
		}
		fmt.Println()
	} else if entry.Level == "WARN" {
		fmt.Printf("[WARN] %v: %s\n", entry.Version, entry.Message)
	} else {
		fmt.Printf("[INFO] %v: %s\n", entry.Version, entry.Message)
	}
}

// checkRotation checks if the log file needs rotation and rotates if necessary
func (l *Logger) checkRotation() error {
	info, err := l.LogFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	if info.Size() < l.maxSize {
		return nil // No rotation needed
	}

	// Rotate the log file
	return l.rotate()
}

// rotate renames the current log file and creates a new one
func (l *Logger) rotate() error {
	// Close current file
	if err := l.LogFile.Close(); err != nil {
		return fmt.Errorf("failed to close log file for rotation: %w", err)
	}

	// Rename current file with timestamp
	timestamp := time.Now().Format("20060102T150405")
	rotatedPath := fmt.Sprintf("%s.%s", l.filePath, timestamp)
	if err := os.Rename(l.filePath, rotatedPath); err != nil {
		// Try to reopen the original file
		l.LogFile, _ = os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Create new log file
	f, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}

	l.LogFile = f
	return nil
}

// LogInfo is a convenience method for INFO level logs
func (l *Logger) LogInfo(version, message string) {
	l.Log(LogEntry{Level: "INFO", Version: version, Message: message})
}

// LogError is a convenience method for ERROR level logs
func (l *Logger) LogError(version, message, errorDetails string) {
	l.Log(LogEntry{Level: "ERROR", Version: version, Message: message, ErrorDetails: errorDetails})
}

// LogWarn is a convenience method for WARN level logs
func (l *Logger) LogWarn(version, message string) {
	l.Log(LogEntry{Level: "WARN", Version: version, Message: message})
}

// Close closes the log file
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.LogFile != nil {
		if err := l.LogFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "[LOGGER ERROR] Failed to close log file: %v\n", err)
		}
	}
}

// Flush ensures all buffered data is written to the file
func (l *Logger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.LogFile != nil {
		return l.LogFile.Sync()
	}
	return nil
}

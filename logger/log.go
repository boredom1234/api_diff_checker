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
	toStdOut bool
}

func New(logPath string, toStdOut bool) (*Logger, error) {
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &Logger{
		LogFile:  f,
		toStdOut: toStdOut,
	}, nil
}

func (l *Logger) Log(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// File output (JSON)
	data, _ := json.Marshal(entry)
	l.LogFile.Write(data)
	l.LogFile.WriteString("\n")

	// Terminal output
	if l.toStdOut {
		if entry.Level == "ERROR" {
			fmt.Printf("[ERROR] %v: %s\n", entry.Version, entry.Message)
		} else {
			fmt.Printf("[INFO] %v: %s\n", entry.Version, entry.Message)
		}
	}
}

func (l *Logger) Close() {
	l.LogFile.Close()
}

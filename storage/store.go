package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store handles saving responses and indexing
type Store struct {
	BaseDir string
	mu      sync.Mutex
	Index   Index
}

type Index struct {
	Commands []CommandEntry `json:"commands"`
}

type CommandEntry struct {
	CommandHash string            `json:"command_hash"`
	CommandRaw  string            `json:"command_raw"`
	Executions  []ExecutionRecord `json:"executions"`
}

type ExecutionRecord struct {
	Version      string    `json:"version"`
	Timestamp    time.Time `json:"timestamp"`
	ResponseFile string    `json:"response_file"`
	Status       string    `json:"status"` // "success", "error"
	Error        string    `json:"error,omitempty"`
}

func NewStore(baseDir string) *Store {
	return &Store{
		BaseDir: baseDir,
		Index: Index{
			Commands: []CommandEntry{},
		},
	}
}

func (s *Store) SaveResponse(command, version string, response []byte, execErr error) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmdHash := hash(command)
	timestamp := time.Now()
	tsStr := timestamp.Format("20060102T150405")

	// 1. Create filename
	// v<version>_<command-hash>_<timestamp>.json
	// Clean version for filename
	safeVer := version // validation could be added
	filename := fmt.Sprintf("v%s_%s_%s.json", safeVer, cmdHash[:8], tsStr)
	filePath := filepath.Join(s.BaseDir, filename)

	// Ensure dir exists
	if _, err := os.Stat(s.BaseDir); os.IsNotExist(err) {
		os.MkdirAll(s.BaseDir, 0755)
	}

	execRecord := ExecutionRecord{
		Version:   version,
		Timestamp: timestamp,

		Status: "success",
	}

	if execErr != nil {
		execRecord.Status = "error"
		execRecord.Error = execErr.Error()
	} else {
		// Pretty print JSON
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, response, "", "  "); err == nil {
			os.WriteFile(filePath, prettyJSON.Bytes(), 0644)
		} else {
			// Save raw if not json
			os.WriteFile(filePath, response, 0644)
		}
		execRecord.ResponseFile = filename
	}

	s.updateIndex(command, cmdHash, execRecord)
	s.saveIndex()

	return filePath, nil
}

func (s *Store) updateIndex(command, hash string, record ExecutionRecord) {
	// Find command entry
	found := false
	for i, entry := range s.Index.Commands {
		if entry.CommandHash == hash {
			s.Index.Commands[i].Executions = append(s.Index.Commands[i].Executions, record)
			found = true
			break
		}
	}
	if !found {
		s.Index.Commands = append(s.Index.Commands, CommandEntry{
			CommandHash: hash,
			CommandRaw:  command,
			Executions:  []ExecutionRecord{record},
		})
	}
}

func (s *Store) saveIndex() {
	data, _ := json.MarshalIndent(s.Index, "", "  ")
	os.WriteFile(filepath.Join(s.BaseDir, "index.json"), data, 0644)
}

func hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	s := &Store{
		BaseDir: baseDir,
		Index: Index{
			Commands: []CommandEntry{},
		},
	}

	// Load existing index if present
	if err := s.LoadIndex(); err != nil {
		// Log but continue - we'll create a fresh index
		fmt.Printf("[WARN] Could not load existing index: %v\n", err)
	}

	return s
}

// LoadIndex loads the index from disk
func (s *Store) LoadIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	indexPath := filepath.Join(s.BaseDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No existing index, that's fine
			return nil
		}
		return fmt.Errorf("failed to read index: %w", err)
	}

	if err := json.Unmarshal(data, &s.Index); err != nil {
		return fmt.Errorf("failed to parse index: %w", err)
	}

	return nil
}

// sanitizeFilename removes or replaces characters that are invalid in filenames
func sanitizeFilename(name string) string {
	// Replace problematic characters with underscores
	re := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	sanitized := re.ReplaceAllString(name, "_")

	// Replace spaces with underscores
	sanitized = regexp.MustCompile(`\s+`).ReplaceAllString(sanitized, "_")

	// Collapse multiple underscores
	sanitized = regexp.MustCompile(`_+`).ReplaceAllString(sanitized, "_")

	// Trim underscores from ends
	sanitized = regexp.MustCompile(`^_+|_+$`).ReplaceAllString(sanitized, "")

	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "unnamed"
	}

	// Limit length
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	return sanitized
}

func (s *Store) SaveResponse(command, version string, response []byte, execErr error) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmdHash := hash(command)
	timestamp := time.Now()
	tsStr := timestamp.Format("20060102T150405")

	// Sanitize version for filename
	safeVer := sanitizeFilename(version)
	filename := fmt.Sprintf("v%s_%s_%s.json", safeVer, cmdHash[:8], tsStr)
	filePath := filepath.Join(s.BaseDir, filename)

	// Ensure dir exists with proper error handling
	if err := os.MkdirAll(s.BaseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	execRecord := ExecutionRecord{
		Version:   version,
		Timestamp: timestamp,
		Status:    "success",
	}

	if execErr != nil {
		execRecord.Status = "error"
		execRecord.Error = execErr.Error()
	} else if response != nil {
		// Pretty print JSON
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, response, "", "  "); err == nil {
			if writeErr := os.WriteFile(filePath, prettyJSON.Bytes(), 0644); writeErr != nil {
				return "", fmt.Errorf("failed to write response file: %w", writeErr)
			}
		} else {
			// Save raw if not JSON
			if writeErr := os.WriteFile(filePath, response, 0644); writeErr != nil {
				return "", fmt.Errorf("failed to write response file: %w", writeErr)
			}
		}
		execRecord.ResponseFile = filename
	}

	s.updateIndex(command, cmdHash, execRecord)
	if err := s.saveIndexLocked(); err != nil {
		// Log error but don't fail the whole operation
		fmt.Printf("[WARN] Failed to save index: %v\n", err)
	}

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

// saveIndexLocked saves the index to disk (must be called with mutex held)
func (s *Store) saveIndexLocked() error {
	data, err := json.MarshalIndent(s.Index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	indexPath := filepath.Join(s.BaseDir, "index.json")
	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	return nil
}

// SaveIndex is a public method to force saving the index
func (s *Store) SaveIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveIndexLocked()
}

func hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// GetResponsePath returns the full path for a response file
func (s *Store) GetResponsePath(filename string) string {
	return filepath.Join(s.BaseDir, filename)
}

// CleanOldResponses removes response files older than the specified duration
func (s *Store) CleanOldResponses(maxAge time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	cleaned := 0

	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read storage directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "index.json" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(s.BaseDir, entry.Name())
			if err := os.Remove(filePath); err == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}

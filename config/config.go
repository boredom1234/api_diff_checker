package config

import (
	"encoding/json"
	"os"
)

// Config represents the users input configuration
type Config struct {
	// Versions maps a version name to its base URL
	// Example: "v1" -> "http://localhost:9876", "v2" -> "http://localhost:9090"
	Versions map[string]string `json:"versions"`

	// Commands is a list of raw curl commands to execute
	// Users should use the placeholder {{BASE_URL}} in these commands
	// which will be replaced by the specific version's URL.
	Commands []string `json:"commands"`

	// KeysOnly if true, compares only JSON structure (keys), not values
	KeysOnly bool `json:"keys_only,omitempty"`
}

// Load reads a config file from path
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

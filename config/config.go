package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultTimeout is the default timeout for command execution
const DefaultTimeout = 30 * time.Second

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

	// Timeout specifies command execution timeout in seconds (default: 30)
	Timeout int `json:"timeout,omitempty"`
}

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult holds all validation errors and warnings
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []string
}

// IsValid returns true if there are no errors
func (v *ValidationResult) IsValid() bool {
	return len(v.Errors) == 0
}

// Error returns a combined error message
func (v *ValidationResult) Error() string {
	if len(v.Errors) == 0 {
		return ""
	}
	var msgs []string
	for _, e := range v.Errors {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "; ")
}

// Validate checks the configuration for errors and returns validation results
func (c *Config) Validate() *ValidationResult {
	result := &ValidationResult{}

	// Check versions
	if len(c.Versions) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "versions",
			Message: "at least one version is required",
		})
	} else {
		for name, baseURL := range c.Versions {
			// Check for empty name
			if strings.TrimSpace(name) == "" {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "versions",
					Message: "version name cannot be empty",
				})
				continue
			}

			// Check for empty URL
			if strings.TrimSpace(baseURL) == "" {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("versions[%s]", name),
					Message: "URL cannot be empty",
				})
				continue
			}

			// Validate URL format
			parsedURL, err := url.Parse(baseURL)
			if err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("versions[%s]", name),
					Message: fmt.Sprintf("invalid URL: %v", err),
				})
				continue
			}

			// Check URL has scheme
			if parsedURL.Scheme == "" {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("versions[%s]", name),
					Message: "URL must have a scheme (http:// or https://)",
				})
			}

			// Check URL has host
			if parsedURL.Host == "" {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("versions[%s]", name),
					Message: "URL must have a host",
				})
			}
		}
	}

	// Check commands
	if len(c.Commands) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "commands",
			Message: "at least one command is required",
		})
	} else {
		hasPlaceholder := false
		for i, cmd := range c.Commands {
			if strings.TrimSpace(cmd) == "" {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("commands[%d]", i),
					Message: "command cannot be empty",
				})
				continue
			}

			if strings.Contains(cmd, "{{BASE_URL}}") {
				hasPlaceholder = true
			}
		}

		// Warn if no command uses the placeholder
		if !hasPlaceholder && len(c.Commands) > 0 {
			result.Warnings = append(result.Warnings,
				"no commands contain {{BASE_URL}} placeholder - commands will not use version URLs")
		}
	}

	// Validate timeout
	if c.Timeout < 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "timeout",
			Message: "timeout cannot be negative",
		})
	}

	return result
}

// GetTimeout returns the configured timeout or default
func (c *Config) GetTimeout() time.Duration {
	if c.Timeout <= 0 {
		return DefaultTimeout
	}
	return time.Duration(c.Timeout) * time.Second
}

// Load reads a config file from path and validates it
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Validate configuration
	validation := cfg.Validate()
	if !validation.IsValid() {
		return nil, fmt.Errorf("config validation failed: %s", validation.Error())
	}

	// Print warnings if any
	for _, warning := range validation.Warnings {
		fmt.Printf("[WARN] Config: %s\n", warning)
	}

	return &cfg, nil
}

// LoadFromJSON parses config from JSON bytes (used by web server)
func LoadFromJSON(data []byte) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Validate configuration
	validation := cfg.Validate()
	if !validation.IsValid() {
		return nil, fmt.Errorf("config validation failed: %s", validation.Error())
	}

	return &cfg, nil
}

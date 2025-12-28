package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-shellwords"
)

// DefaultTimeout is the default execution timeout for commands
const DefaultTimeout = 30 * time.Second

type ExecutionResult struct {
	Command   string    `json:"command"`
	Version   string    `json:"version"`
	Response  []byte    `json:"-"` // Don't embed in log automatically, save to file
	Timestamp time.Time `json:"timestamp"`
	Duration  string    `json:"duration"`
	Error     string    `json:"error,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`    // Always capture stderr for debugging
	TimedOut  bool      `json:"timed_out,omitempty"` // True if command exceeded timeout
}

// normalizeCommand removes backslash line continuations, tabs, and extra whitespace
// that are common when copying commands from browser DevTools.
func normalizeCommand(cmd string) string {
	// Remove backslash followed by newline (line continuation)
	cmd = strings.ReplaceAll(cmd, "\\\n", " ")
	cmd = strings.ReplaceAll(cmd, "\\\r\n", " ")
	// Also handle cases where backslash is followed by spaces then newline
	cmd = strings.ReplaceAll(cmd, "\\   ", " ")
	cmd = strings.ReplaceAll(cmd, "\\  ", " ")
	cmd = strings.ReplaceAll(cmd, "\\ ", " ")
	// Replace tabs with spaces
	cmd = strings.ReplaceAll(cmd, "\t", " ")
	// Replace carriage returns
	cmd = strings.ReplaceAll(cmd, "\r", "")
	// Replace multiple spaces with single space
	spaceRegex := regexp.MustCompile(`\s+`)
	cmd = spaceRegex.ReplaceAllString(cmd, " ")
	return strings.TrimSpace(cmd)
}

// validateCommand checks if the command appears to be a curl command
// Returns a warning message if not curl, empty string if valid
func validateCommand(args []string) string {
	if len(args) == 0 {
		return "empty command"
	}
	cmdName := strings.ToLower(args[0])
	if cmdName != "curl" && cmdName != "curl.exe" {
		return fmt.Sprintf("command '%s' is not curl - execution may behave unexpectedly", args[0])
	}
	return ""
}

// Execute runs the curl command after replacing {{BASE_URL}} with the target base URL.
// Uses the provided timeout, or DefaultTimeout if timeout is 0.
func Execute(commandTmpl string, version string, baseURL string, timeout time.Duration) (*ExecutionResult, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	// 1. Normalize command (handle line continuations, tabs, etc.)
	normalizedCmd := normalizeCommand(commandTmpl)

	// 2. Replace placeholder
	finalCmdStr := strings.ReplaceAll(normalizedCmd, "{{BASE_URL}}", baseURL)

	// 3. Parse command into args
	args, err := shellwords.Parse(finalCmdStr)
	if err != nil {
		return &ExecutionResult{
			Command:   finalCmdStr,
			Version:   version,
			Timestamp: time.Now(),
			Error:     fmt.Sprintf("failed to parse command: %v", err),
		}, fmt.Errorf("failed to parse command: %w", err)
	}

	if len(args) == 0 {
		return &ExecutionResult{
			Command:   finalCmdStr,
			Version:   version,
			Timestamp: time.Now(),
			Error:     "empty command after parsing",
		}, fmt.Errorf("empty command")
	}

	// 4. Validate command (warn if not curl)
	if warning := validateCommand(args); warning != "" {
		// Log warning but continue execution
		fmt.Printf("[WARN] %s: %s\n", version, warning)
	}

	cmdName := args[0]
	cmdArgs := args[1:]

	// 5. Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	duration := time.Since(start)

	result := &ExecutionResult{
		Command:   finalCmdStr,
		Version:   version,
		Timestamp: start,
		Duration:  duration.String(),
		Stderr:    strings.TrimSpace(stderr.String()), // Always capture stderr
	}

	// Check if the error was due to context timeout
	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.Error = fmt.Sprintf("command timed out after %s", timeout)
		return result, ctx.Err()
	}

	if err != nil {
		result.Error = fmt.Sprintf("execution failed: %v", err)
		if result.Stderr != "" {
			result.Error += fmt.Sprintf(" | stderr: %s", result.Stderr)
		}
		return result, err
	}

	result.Response = stdout.Bytes()
	return result, nil
}

// ExecuteWithDefaults runs Execute with default timeout
func ExecuteWithDefaults(commandTmpl string, version string, baseURL string) (*ExecutionResult, error) {
	return Execute(commandTmpl, version, baseURL, DefaultTimeout)
}

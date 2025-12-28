package executor

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mattn/go-shellwords"
)

type ExecutionResult struct {
	Command   string    `json:"command"`
	Version   string    `json:"version"`
	Response  []byte    `json:"-"` // Don't embed in log automatically, save to file
	Timestamp time.Time `json:"timestamp"`
	Duration  string    `json:"duration"`
	Error     string    `json:"error,omitempty"`
}

// normalizeCommand removes backslash line continuations and extra whitespace
// that are common when copying commands from browser DevTools.
func normalizeCommand(cmd string) string {
	// Remove backslash followed by newline (line continuation)
	cmd = strings.ReplaceAll(cmd, "\\\n", " ")
	cmd = strings.ReplaceAll(cmd, "\\\r\n", " ")
	// Also handle cases where backslash is followed by spaces then newline
	cmd = strings.ReplaceAll(cmd, "\\   ", " ")
	cmd = strings.ReplaceAll(cmd, "\\  ", " ")
	cmd = strings.ReplaceAll(cmd, "\\ ", " ")
	// Replace multiple spaces with single space
	for strings.Contains(cmd, "  ") {
		cmd = strings.ReplaceAll(cmd, "  ", " ")
	}
	return strings.TrimSpace(cmd)
}

// Execute runs the curl command after replacing {{BASE_URL}} with the target base URL.
func Execute(commandTmpl string, version string, baseURL string) (*ExecutionResult, error) {
	// 1. Normalize command (handle line continuations)
	normalizedCmd := normalizeCommand(commandTmpl)

	// 2. Replace placeholder
	finalCmdStr := strings.ReplaceAll(normalizedCmd, "{{BASE_URL}}", baseURL)

	// 2. Parse command into args
	args, err := shellwords.Parse(finalCmdStr)
	if err != nil {
		return &ExecutionResult{
			Command: finalCmdStr,
			Version: version,
			Error:   fmt.Sprintf("failed to parse command: %v", err),
		}, err
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	// We assume the first arg is 'curl'. If not, we might warn, but we'll run it anyway.
	cmdName := args[0]
	cmdArgs := args[1:]

	start := time.Now()
	cmd := exec.Command(cmdName, cmdArgs...)
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
	}

	if err != nil {
		result.Error = fmt.Sprintf("execution failed: %v | stderr: %s", err, stderr.String())
		return result, err
	}

	result.Response = stdout.Bytes()
	return result, nil
}

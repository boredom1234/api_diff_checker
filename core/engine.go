package core

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"api_diff_checker/comparator"
	"api_diff_checker/config"
	"api_diff_checker/executor"
	"api_diff_checker/logger"
	"api_diff_checker/storage"
)

// DefaultTimeout for the entire run operation
const DefaultRunTimeout = 10 * time.Minute

type Engine struct {
	Store  *storage.Store
	Logger *logger.Logger
}

type RunResult struct {
	CommandResults []CommandResult `json:"command_results"`
	Errors         []string        `json:"errors,omitempty"` // Aggregated non-fatal errors
}

type CommandResult struct {
	TestCaseName string            `json:"test_case_name"`    // Name of the test case
	Commands     map[string]string `json:"commands"`          // Version -> command mapping
	Command      string            `json:"command,omitempty"` // Legacy: single command (kept for backward compat)
	Diffs        []VersionDiff     `json:"diffs"`
	ExecInfo     []ExecInfo        `json:"execution_info"` // Version -> FilePath/Exec details
}

type ExecInfo struct {
	Version  string `json:"version"`
	File     string `json:"file"`
	Error    string `json:"error,omitempty"`
	TimedOut bool   `json:"timed_out,omitempty"`
}

type VersionDiff struct {
	VersionA   string                 `json:"version_a"`
	VersionB   string                 `json:"version_b"`
	DiffResult *comparator.DiffResult `json:"diff_result"`
	OldContent string                 `json:"old_content,omitempty"`
	NewContent string                 `json:"new_content,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

func NewEngine(store *storage.Store, l *logger.Logger) *Engine {
	return &Engine{
		Store:  store,
		Logger: l,
	}
}

// execResult is used for collecting results from goroutines via channel
type execResult struct {
	version  string
	filePath string
	execInfo ExecInfo
	err      error
}

func (e *Engine) Run(cfg *config.Config) (*RunResult, error) {
	return e.RunWithContext(context.Background(), cfg)
}

func (e *Engine) RunWithContext(ctx context.Context, cfg *config.Config) (*RunResult, error) {
	// Apply overall timeout if context doesn't have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultRunTimeout)
		defer cancel()
	}

	// Sorted versions for consistency
	var versions []string
	for v := range cfg.Versions {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	// Get normalized test cases (handles both new and legacy formats)
	testCases := cfg.GetTestCases()

	runResult := &RunResult{
		CommandResults: make([]CommandResult, len(testCases)),
	}

	timeout := cfg.GetTimeout()

	for tcIdx, testCase := range testCases {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			runResult.Errors = append(runResult.Errors, fmt.Sprintf("operation cancelled: %v", ctx.Err()))
			return runResult, ctx.Err()
		default:
		}

		cmdRes := CommandResult{
			TestCaseName: testCase.Name,
			Commands:     testCase.Commands,
		}

		fmt.Printf("\n--- Executing Test Case: %s ---\n", testCase.Name)

		// Use channel to collect results from goroutines (avoid race condition)
		resultChan := make(chan execResult, len(versions))
		var wg sync.WaitGroup

		for _, vName := range versions {
			baseURL := cfg.Versions[vName]
			// Get the command for this specific version
			cmdForVersion, ok := testCase.Commands[vName]
			if !ok {
				// Version not in this test case, skip
				fmt.Printf("[WARN] Test case '%s' has no command for version '%s', skipping\n", testCase.Name, vName)
				continue
			}

			wg.Add(1)

			go func(v, url, cmdRaw string) {
				defer wg.Done()

				// Panic recovery
				defer func() {
					if r := recover(); r != nil {
						errMsg := fmt.Sprintf("panic during execution: %v", r)
						e.Logger.Log(logger.LogEntry{
							Level: "ERROR", Version: v, Command: cmdRaw,
							Message: "Panic recovered", ErrorDetails: errMsg,
						})
						resultChan <- execResult{
							version: v,
							execInfo: ExecInfo{
								Version: v,
								Error:   errMsg,
							},
							err: fmt.Errorf(errMsg),
						}
					}
				}()

				res, err := executor.Execute(cmdRaw, v, url, timeout)
				result := execResult{
					version:  v,
					execInfo: ExecInfo{Version: v, TimedOut: res != nil && res.TimedOut},
				}

				if err != nil {
					e.Logger.Log(logger.LogEntry{
						Level: "ERROR", Version: v, Command: cmdRaw,
						Message: "Execution failed", ErrorDetails: err.Error(),
					})
					_, _ = e.Store.SaveResponse(cmdRaw, v, nil, err)
					result.execInfo.Error = err.Error()
					if res != nil && res.TimedOut {
						result.execInfo.Error = fmt.Sprintf("timeout after %s", timeout)
					}
					result.err = err
				} else {
					path, saveErr := e.Store.SaveResponse(cmdRaw, v, res.Response, nil)
					if saveErr != nil {
						e.Logger.Log(logger.LogEntry{Level: "ERROR", Version: v, Message: "Failed to save response", ErrorDetails: saveErr.Error()})
						result.execInfo.Error = "Save failed: " + saveErr.Error()
						result.err = saveErr
					} else {
						e.Logger.Log(logger.LogEntry{Level: "INFO", Version: v, Command: cmdRaw, Message: "Response saved", ErrorDetails: path})
						result.execInfo.File = path
						result.filePath = path
					}
				}

				resultChan <- result
			}(vName, baseURL, cmdForVersion)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(resultChan)

		// Collect results from channel (thread-safe)
		results := make(map[string]string) // Version -> FilePath
		for result := range resultChan {
			cmdRes.ExecInfo = append(cmdRes.ExecInfo, result.execInfo)
			if result.filePath != "" {
				results[result.version] = result.filePath
			}
		}

		// Sort ExecInfo by version for consistent display
		sort.Slice(cmdRes.ExecInfo, func(i, j int) bool {
			return cmdRes.ExecInfo[i].Version < cmdRes.ExecInfo[j].Version
		})

		// Compare versions
		if len(versions) > 1 {
			for i := 0; i < len(versions)-1; i++ {
				vBase := versions[i]
				vTarget := versions[i+1]

				file1, ok1 := results[vBase]
				file2, ok2 := results[vTarget]

				vDiff := VersionDiff{
					VersionA: vBase,
					VersionB: vTarget,
				}

				if ok1 && ok2 {
					diff, old, new, err := e.compareFiles(file1, file2, vBase, vTarget, cfg.KeysOnly)
					if err != nil {
						vDiff.Error = err.Error()
					} else {
						vDiff.DiffResult = diff
						vDiff.OldContent = old
						vDiff.NewContent = new
					}
				} else {
					var missing []string
					if !ok1 {
						missing = append(missing, vBase)
					}
					if !ok2 {
						missing = append(missing, vTarget)
					}
					vDiff.Error = fmt.Sprintf("failed to get responses for version(s): %s",
						joinStrings(missing, ", "))
				}
				cmdRes.Diffs = append(cmdRes.Diffs, vDiff)
			}
		}

		runResult.CommandResults[tcIdx] = cmdRes
	}

	return runResult, nil
}

func (e *Engine) compareFiles(file1, file2, v1, v2 string, keysOnly bool) (*comparator.DiffResult, string, string, error) {
	b1, err := os.ReadFile(file1)
	if err != nil {
		return nil, "", "", fmt.Errorf("read file1 error: %w", err)
	}
	b2, err := os.ReadFile(file2)
	if err != nil {
		return nil, "", "", fmt.Errorf("read file2 error: %w", err)
	}

	if len(b1) == 0 {
		return nil, "", "", fmt.Errorf("empty response content for %s", v1)
	}
	if len(b2) == 0 {
		return nil, "", "", fmt.Errorf("empty response content for %s", v2)
	}

	opts := comparator.CompareOptions{KeysOnly: keysOnly}
	diff, err := comparator.CompareWithOptions(b1, b2, file1, file2, opts)
	if err != nil {
		return nil, "", "", err
	}
	return diff, string(b1), string(b2), nil
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

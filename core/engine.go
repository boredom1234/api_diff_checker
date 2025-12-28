package core

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"api_diff_checker/comparator"
	"api_diff_checker/config"
	"api_diff_checker/executor"
	"api_diff_checker/logger"
	"api_diff_checker/storage"
)

type Engine struct {
	Store  *storage.Store
	Logger *logger.Logger
}

type RunResult struct {
	CommandResults []CommandResult `json:"command_results"`
}

type CommandResult struct {
	Command  string        `json:"command"`
	Diffs    []VersionDiff `json:"diffs"`
	ExecInfo []ExecInfo    `json:"execution_info"` // Version -> FilePath/Exec details
}

type ExecInfo struct {
	Version string `json:"version"`
	File    string `json:"file"`
	Error   string `json:"error,omitempty"`
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

func (e *Engine) Run(cfg *config.Config) (*RunResult, error) {
	// Sorted versions for consistency
	var versions []string
	for v := range cfg.Versions {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	runResult := &RunResult{
		CommandResults: make([]CommandResult, len(cfg.Commands)),
	}

	for cmdIdx, cmdRaw := range cfg.Commands {
		cmdRes := CommandResult{
			Command: cmdRaw,
		}

		fmt.Printf("\n--- Executing Command: %s ---\n", cmdRaw)

		// Map: Version -> FilePath (for internal comparison use)
		results := make(map[string]string)
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, vName := range versions {
			baseURL := cfg.Versions[vName]
			wg.Add(1)

			go func(v, url string) {
				defer wg.Done()

				res, err := executor.Execute(cmdRaw, v, url)
				execInfo := ExecInfo{Version: v}

				if err != nil {
					e.Logger.Log(logger.LogEntry{
						Level: "ERROR", Version: v, Command: cmdRaw,
						Message: "Execution failed", ErrorDetails: err.Error(),
					})
					_, _ = e.Store.SaveResponse(cmdRaw, v, nil, err)
					execInfo.Error = err.Error()
				} else {
					path, saveErr := e.Store.SaveResponse(cmdRaw, v, res.Response, nil)
					if saveErr != nil {
						e.Logger.Log(logger.LogEntry{Level: "ERROR", Version: v, Message: "Failed to save response", ErrorDetails: saveErr.Error()})
						execInfo.Error = "Save failed: " + saveErr.Error()
					} else {
						e.Logger.Log(logger.LogEntry{Level: "INFO", Version: v, Command: cmdRaw, Message: "Response saved", ErrorDetails: path})
						execInfo.File = path

						mu.Lock()
						results[v] = path
						mu.Unlock()
					}
				}

				mu.Lock()
				cmdRes.ExecInfo = append(cmdRes.ExecInfo, execInfo)
				mu.Unlock()
			}(vName, baseURL)
		}
		wg.Wait()

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
					vDiff.Error = "One or both versions failed execution"
				}
				cmdRes.Diffs = append(cmdRes.Diffs, vDiff)
			}
		} else {
			// Single version - nothing to compare
			// Could maybe compare against previous run? But keeping simple for now.
		}

		runResult.CommandResults[cmdIdx] = cmdRes
	}

	return runResult, nil
}

func (e *Engine) compareFiles(file1, file2, v1, v2 string, keysOnly bool) (*comparator.DiffResult, string, string, error) {
	b1, err := os.ReadFile(file1)
	if err != nil {
		return nil, "", "", fmt.Errorf("read file1 error: %v", err)
	}
	b2, err := os.ReadFile(file2)
	if err != nil {
		return nil, "", "", fmt.Errorf("read file2 error: %v", err)
	}

	if len(b1) == 0 || len(b2) == 0 {
		return nil, "", "", fmt.Errorf("empty response content")
	}

	opts := comparator.CompareOptions{KeysOnly: keysOnly}
	diff, err := comparator.CompareWithOptions(b1, b2, file1, file2, opts)
	if err != nil {
		return nil, "", "", err
	}
	return diff, string(b1), string(b2), nil
}

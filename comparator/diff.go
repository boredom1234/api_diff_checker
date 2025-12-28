package comparator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/wI2L/jsondiff"
)

type DiffResult struct {
	TextDiff  string `json:"text_diff"`
	JsonPatch []byte `json:"json_patch"`
	Summary   string `json:"summary"`
}

// CompareOptions allows customization of comparison behavior
type CompareOptions struct {
	KeysOnly bool // If true, only compare JSON structure (keys), not values
}

func Compare(original, modified []byte, name1, name2 string) (*DiffResult, error) {
	return CompareWithOptions(original, modified, name1, name2, CompareOptions{KeysOnly: false})
}

func CompareWithOptions(original, modified []byte, name1, name2 string, opts CompareOptions) (*DiffResult, error) {
	var v1, v2 interface{}
	if err := json.Unmarshal(original, &v1); err != nil {
		return nil, fmt.Errorf("invalid json in original: %v", err)
	}
	if err := json.Unmarshal(modified, &v2); err != nil {
		return nil, fmt.Errorf("invalid json in modified: %v", err)
	}

	// If keys-only mode, extract and compare only the structure
	if opts.KeysOnly {
		v1 = extractKeys(v1)
		v2 = extractKeys(v2)

		// Re-marshal for text diff
		original, _ = json.MarshalIndent(v1, "", "  ")
		modified, _ = json.MarshalIndent(v2, "", "  ")
	}

	// 1. Unified Diff (Text)
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(original)),
		B:        difflib.SplitLines(string(modified)),
		FromFile: name1,
		ToFile:   name2,
		Context:  3,
	}
	textDiff, _ := difflib.GetUnifiedDiffString(diff)

	// 2. JSON Patch (RFC 6902)
	patch, err := jsondiff.Compare(v1, v2)
	if err != nil {
		return nil, fmt.Errorf("jsondiff failed: %v", err)
	}

	patchBytes, err := json.MarshalIndent(patch, "", "  ")
	if err != nil {
		patchBytes = []byte("[]")
	}

	// 3. Summary
	var summary string
	if opts.KeysOnly {
		summary = summarizeKeyDifferences(v1, v2)
	} else {
		summary = summarizeDifferences(original, modified)
	}

	return &DiffResult{
		TextDiff:  textDiff,
		JsonPatch: patchBytes,
		Summary:   summary,
	}, nil
}

// extractKeys recursively extracts only the structure (keys) from JSON
// Values are replaced with their type indicators
func extractKeys(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, child := range val {
			result[k] = extractKeys(child)
		}
		return result
	case []interface{}:
		if len(val) > 0 {
			// For arrays, just show structure of first element
			return []interface{}{extractKeys(val[0])}
		}
		return []interface{}{}
	case string:
		return "<string>"
	case float64:
		return "<number>"
	case bool:
		return "<boolean>"
	case nil:
		return "<null>"
	default:
		return "<unknown>"
	}
}

func summarizeKeyDifferences(v1, v2 interface{}) string {
	keys1 := collectAllKeys(v1, "")
	keys2 := collectAllKeys(v2, "")

	var changes []string

	// Check for removed keys
	for k := range keys1 {
		if _, ok := keys2[k]; !ok {
			changes = append(changes, fmt.Sprintf("Key '%s' removed", k))
		}
	}

	// Check for added keys
	for k := range keys2 {
		if _, ok := keys1[k]; !ok {
			changes = append(changes, fmt.Sprintf("Key '%s' added", k))
		}
	}

	// Sort for consistent output
	sort.Strings(changes)

	if len(changes) == 0 {
		return "No structural changes (keys match)"
	}
	return strings.Join(changes, ", ")
}

func collectAllKeys(v interface{}, prefix string) map[string]bool {
	keys := make(map[string]bool)

	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			fullKey := k
			if prefix != "" {
				fullKey = prefix + "." + k
			}
			keys[fullKey] = true
			// Recurse into nested objects
			for nestedKey := range collectAllKeys(child, fullKey) {
				keys[nestedKey] = true
			}
		}
	case []interface{}:
		if len(val) > 0 {
			arrayPrefix := prefix + "[]"
			for nestedKey := range collectAllKeys(val[0], arrayPrefix) {
				keys[nestedKey] = true
			}
		}
	}

	return keys
}

func summarizeDifferences(json1, json2 []byte) string {
	var m1, m2 map[string]interface{}
	// If not objects (e.g. arrays), simple check
	if err := json.Unmarshal(json1, &m1); err != nil {
		return "Comparison for summary skipped (not simple object)"
	}
	if err := json.Unmarshal(json2, &m2); err != nil {
		return "Comparison for summary skipped (not simple object)"
	}

	var changes []string

	// Check keys in m1
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok {
			changes = append(changes, fmt.Sprintf("Field '%s' removed", k))
			continue
		}
		if fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			changes = append(changes, fmt.Sprintf("Field '%s' changed", k))
		}
	}
	// Check keys in m2 that are not in m1
	for k := range m2 {
		if _, ok := m1[k]; !ok {
			changes = append(changes, fmt.Sprintf("Field '%s' added", k))
		}
	}

	if len(changes) == 0 {
		return "No top-level changes"
	}
	return strings.Join(changes, ", ")
}

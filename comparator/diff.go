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
	IsJSON    bool   `json:"is_json"` // Indicates if both inputs were valid JSON
}

// CompareOptions allows customization of comparison behavior
type CompareOptions struct {
	KeysOnly bool // If true, only compare JSON structure (keys), not values
}

// isValidJSON checks if the byte slice is valid JSON
func isValidJSON(data []byte) bool {
	var js interface{}
	return json.Unmarshal(data, &js) == nil
}

// Compare compares two byte slices and returns a diff result
func Compare(original, modified []byte, name1, name2 string) (*DiffResult, error) {
	return CompareWithOptions(original, modified, name1, name2, CompareOptions{KeysOnly: false})
}

// CompareWithOptions compares with configurable options
func CompareWithOptions(original, modified []byte, name1, name2 string, opts CompareOptions) (*DiffResult, error) {
	// Check if both are valid JSON
	isJSON1 := isValidJSON(original)
	isJSON2 := isValidJSON(modified)

	// If either is not JSON, do a plain text comparison
	if !isJSON1 || !isJSON2 {
		return compareAsText(original, modified, name1, name2, isJSON1, isJSON2)
	}

	// Both are JSON, proceed with JSON comparison
	return compareAsJSON(original, modified, name1, name2, opts)
}

// compareAsText performs a plain text diff when content is not JSON
func compareAsText(original, modified []byte, name1, name2 string, isJSON1, isJSON2 bool) (*DiffResult, error) {
	// Create unified diff
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(original)),
		B:        difflib.SplitLines(string(modified)),
		FromFile: name1,
		ToFile:   name2,
		Context:  3,
	}
	textDiff, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return nil, fmt.Errorf("failed to create text diff: %w", err)
	}

	// Create summary
	var summary string
	if !isJSON1 && !isJSON2 {
		summary = "Both responses are non-JSON content"
	} else if !isJSON1 {
		summary = fmt.Sprintf("Response from %s is not valid JSON", name1)
	} else {
		summary = fmt.Sprintf("Response from %s is not valid JSON", name2)
	}

	// Check if contents are identical
	if string(original) == string(modified) {
		summary += " (content is identical)"
		textDiff = ""
	}

	return &DiffResult{
		TextDiff:  textDiff,
		JsonPatch: []byte("[]"), // No JSON patch for non-JSON content
		Summary:   summary,
		IsJSON:    false,
	}, nil
}

// compareAsJSON performs a JSON-aware comparison
func compareAsJSON(original, modified []byte, name1, name2 string, opts CompareOptions) (*DiffResult, error) {
	var v1, v2 interface{}
	if err := json.Unmarshal(original, &v1); err != nil {
		return nil, fmt.Errorf("invalid json in original: %w", err)
	}
	if err := json.Unmarshal(modified, &v2); err != nil {
		return nil, fmt.Errorf("invalid json in modified: %w", err)
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
	textDiff, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		textDiff = fmt.Sprintf("Failed to create diff: %v", err)
	}

	// 2. JSON Patch (RFC 6902)
	patch, err := jsondiff.Compare(v1, v2)
	if err != nil {
		return nil, fmt.Errorf("jsondiff failed: %w", err)
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
		summary = summarizeDifferences(v1, v2)
	}

	return &DiffResult{
		TextDiff:  textDiff,
		JsonPatch: patchBytes,
		Summary:   summary,
		IsJSON:    true,
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

// summarizeDifferences creates a human-readable summary of changes
func summarizeDifferences(v1, v2 interface{}) string {
	// Handle arrays at the top level
	arr1, isArr1 := v1.([]interface{})
	arr2, isArr2 := v2.([]interface{})

	if isArr1 && isArr2 {
		return summarizeArrayDifferences(arr1, arr2)
	}

	// Handle objects at the top level
	m1, isMap1 := v1.(map[string]interface{})
	m2, isMap2 := v2.(map[string]interface{})

	if !isMap1 || !isMap2 {
		if fmt.Sprintf("%v", v1) == fmt.Sprintf("%v", v2) {
			return "No top-level changes"
		}
		return "Top-level value changed"
	}

	var changes []string

	// Check keys in m1
	for k, val1 := range m1 {
		val2, ok := m2[k]
		if !ok {
			changes = append(changes, fmt.Sprintf("Field '%s' removed", k))
			continue
		}
		if !deepEqual(val1, val2) {
			changes = append(changes, fmt.Sprintf("Field '%s' changed", k))
		}
	}
	// Check keys in m2 that are not in m1
	for k := range m2 {
		if _, ok := m1[k]; !ok {
			changes = append(changes, fmt.Sprintf("Field '%s' added", k))
		}
	}

	// Sort for consistent output
	sort.Strings(changes)

	if len(changes) == 0 {
		return "No top-level changes"
	}
	return strings.Join(changes, ", ")
}

// summarizeArrayDifferences handles top-level array comparisons
func summarizeArrayDifferences(arr1, arr2 []interface{}) string {
	len1, len2 := len(arr1), len(arr2)

	if len1 != len2 {
		return fmt.Sprintf("Array length changed: %d → %d items", len1, len2)
	}

	changedCount := 0
	for i := 0; i < len1; i++ {
		if !deepEqual(arr1[i], arr2[i]) {
			changedCount++
		}
	}

	if changedCount == 0 {
		return "No top-level changes"
	}

	return fmt.Sprintf("Array: %d of %d items changed", changedCount, len1)
}

// deepEqual performs a deep comparison of two values
func deepEqual(v1, v2 interface{}) bool {
	b1, err1 := json.Marshal(v1)
	b2, err2 := json.Marshal(v2)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(b1) == string(b2)
}

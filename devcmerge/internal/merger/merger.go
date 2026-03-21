package merger

import (
	"encoding/json"
	"fmt"
	"os"
	"sort" // Added for sorting slices

	"github.com/tidwall/jsonc" // Correct import for jsonc parsing
)

// ParseJSONC reads a file and parses it as JSONC.
// It returns the parsed content as a map[string]any.
func ParseJSONC(filePath string) (map[string]any, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	
	// Convert JSONC to standard JSON by removing comments
	cleanedData := jsonc.ToJSON(data)

	var result map[string]any
	if err := json.Unmarshal(cleanedData, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON for %s: %w", filePath, err)
	}

	return result, nil
}

// MergeConfigs implements the merging logic based on the specification.
// It recursively merges two maps.
func MergeConfigs(base, override map[string]any) (map[string]any, error) {
	// Start with a deep copy of the base config to avoid modifying the original map
	merged := make(map[string]any)
	for k, v := range base {
		// Deep copy nested maps and slices to ensure modifications don't affect original data
		if vMap, ok := v.(map[string]any); ok {
			copiedMap, err := deepCopyMap(vMap)
			if err != nil {
				return nil, fmt.Errorf("error deep copying map for key '%s': %w", k, err)
			}
			merged[k] = copiedMap
		} else if vSlice, ok := v.([]any); ok {
			copiedSlice, err := deepCopySlice(vSlice)
			if err != nil {
				return nil, fmt.Errorf("error deep copying slice for key '%s': %w", k, err)
			}
			merged[k] = copiedSlice
		} else {
			merged[k] = v // Primitive types are copied by value
		}
	}

	// Apply override configurations
	for key, overrideValue := range override {
		if baseValue, exists := merged[key]; exists {
			// Handle nested objects: Recursive mergeConfigs
			if baseMap, ok := baseValue.(map[string]any); ok {
				if overrideMap, ok := overrideValue.(map[string]any); ok {
					nestedMerged, err := MergeConfigs(baseMap, overrideMap)
					if err != nil {
						return nil, fmt.Errorf("error merging nested map for key '%s': %w", key, err)
					}
					merged[key] = nestedMerged
					continue // Move to next key in override
				}
			}

			// Handle slices: specific merging logic based on key
			if overrideSlice, ok := overrideValue.([]any); ok {
				if baseSlice, ok := baseValue.([]any); ok {
					var err error
					switch key {
					case "extensions", "appPort":
						merged[key], err = mergeIntelligentArrays(baseSlice, overrideSlice, key)
						if err != nil {
							return nil, fmt.Errorf("error merging intelligent array for key '%s': %w", key, err)
						}
					case "postCreateCommand", "postStartCommand", "postAttachCommand":
						merged[key], err = mergeCommandArrays(baseSlice, overrideSlice)
						if err != nil {
							return nil, fmt.Errorf("error merging command arrays for key '%s': %w", key, err)
						}
					default:
						// For other arrays, override with the override slice (deep copy to avoid modification issues)
						copiedSlice, err := deepCopySlice(overrideSlice)
						if err != nil {
							return nil, fmt.Errorf("error deep copying override slice for key '%s': %w", key, err)
						}
						merged[key] = copiedSlice
					}
					continue // Move to next key in override
				}
			}
			// If types mismatch or not specific array types, override with the override value
			merged[key] = overrideValue
		} else {
			// Key does not exist in base, simply add it from override
			// Deep copy if it's a map or slice
			if ovMap, ok := overrideValue.(map[string]any); ok {
				copiedMap, err := deepCopyMap(ovMap)
				if err != nil {
					return nil, fmt.Errorf("error deep copying new map for key '%s': %w", key, err)
				}
				merged[key] = copiedMap
			} else if ovSlice, ok := overrideValue.([]any); ok {
				copiedSlice, err := deepCopySlice(ovSlice)
				if err != nil {
					return nil, fmt.Errorf("error deep copying new slice for key '%s': %w", key, err)
				}
				merged[key] = copiedSlice
			} else {
				merged[key] = overrideValue
			}
		}
	}
	return merged, nil
}

// deepCopyMap creates a deep copy of a map[string]any.
func deepCopyMap(m map[string]any) (map[string]any, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var c map[string]any
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return c, nil
}

// deepCopySlice creates a deep copy of a []any.
func deepCopySlice(s []any) ([]any, error) {
	if s == nil {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var c []any
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return c, nil
}

// mergeCommandArrays concatenates two slices of strings, handling potential non-string elements gracefully.
func mergeCommandArrays(baseSlice, overrideSlice []any) ([]any, error) {
	var baseStrings []string
	for _, item := range baseSlice {
		if s, ok := item.(string); ok {
			baseStrings = append(baseStrings, s)
		} else {
			fmt.Printf("Warning: Non-string element found in base command array: %v\n", item)
		}
	}

	var overrideStrings []string
	for _, item := range overrideSlice {
		if s, ok := item.(string); ok {
			overrideStrings = append(overrideStrings, s)
		} else {
			fmt.Printf("Warning: Non-string element found in override command array: %v\n", item)
		}
	}

	// Concatenate
	result := append(baseStrings, overrideStrings...)

	var anySlice []any
	for _, s := range result {
		anySlice = append(anySlice, s)
	}
	return anySlice, nil
}

// mergeIntelligentArrays intelligently merges specific array types like 'extensions', 'appPort'.
// It returns a new slice of any and an error if any occurs during processing.
func mergeIntelligentArrays(base, override []any, key string) ([]any, error) {
	switch key {
	case "extensions":
		return mergeExtensions(base, override)
	case "appPort":
		return mergeAppPort(base, override)
	default:
		// Fallback for unhandled keys (should not be reached if called correctly)
		copiedSlice, err := deepCopySlice(override)
		if err != nil {
			return nil, fmt.Errorf("error deep copying override slice for unhandled intelligent array key '%s': %w", key, err)
		}
		return copiedSlice, nil
	}
}



func mergeExtensions(base, override []any) ([]any, error) {
	mergedExtensions := make(map[string]struct{}) // Using struct{} for a set of strings

	for _, item := range base {
		if s, ok := item.(string); ok {
			mergedExtensions[s] = struct{}{}
		} else {
			fmt.Printf("Warning: Non-string extension item found in base config: %v\n", item)
		}
	}
	for _, item := range override {
		if s, ok := item.(string); ok {
			mergedExtensions[s] = struct{}{}
		} else {
			fmt.Printf("Warning: Non-string extension item found in override config: %v\n", item)
		}
	}

	result := make([]any, 0, len(mergedExtensions))
	for ext := range mergedExtensions {
		result = append(result, ext)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].(string) < result[j].(string)
	})
	return result, nil
}

func mergeAppPort(base, override []any) ([]any, error) {
	mergedPorts := make(map[any]struct{})

	for _, item := range base {
		mergedPorts[item] = struct{}{}
	}
	for _, item := range override {
		mergedPorts[item] = struct{}{}
	}

	result := make([]any, 0, len(mergedPorts))
	for port := range mergedPorts {
		result = append(result, port)
	}
	sort.SliceStable(result, func(i, j int) bool {
		// Sorting mixed types in Go slices requires careful handling.
		// This attempts a basic comparison for common types.
		// If types are truly mixed and not directly comparable, their relative order is maintained.
		switch vI := result[i].(type) {
		case string:
			if vJ, ok := result[j].(string); ok {
				return vI < vJ
			}
		case float64:
			if vJ, ok := result[j].(float64); ok {
				return vI < vJ
			}
		case int:
			if vJ, ok := result[j].(int); ok {
				return vI < vJ
			}
		}
		return false
	})
	return result, nil
}

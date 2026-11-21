// sections.go — Storage section filtering and size summaries for localStorage, sessionStorage and cookies.
// Docs: docs/features/feature/observe/index.md

package pagestorage

import "encoding/json"

// summarizeStorageMap returns a summary of a key-value storage map.
func summarizeStorageMap(data map[string]any) map[string]any {
	keys := make([]string, 0, len(data))
	totalBytes := 0
	for k, v := range data {
		keys = append(keys, k)
		totalBytes += len(k)
		if s, ok := v.(string); ok {
			totalBytes += len(s)
		} else {
			b, _ := json.Marshal(v)
			totalBytes += len(b)
		}
	}
	sampleKeys := keys
	if len(sampleKeys) > 5 {
		sampleKeys = sampleKeys[:5]
	}
	return map[string]any{
		"key_count":   len(data),
		"total_bytes": totalBytes,
		"sample_keys": sampleKeys,
	}
}

// summarizeCookies returns a summary of a cookie array.
func summarizeCookies(cookies []any) map[string]any {
	names := make([]string, 0, len(cookies))
	totalBytes := 0
	for _, c := range cookies {
		if m, ok := c.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				names = append(names, name)
			}
			b, _ := json.Marshal(m)
			totalBytes += len(b)
		}
	}
	sampleKeys := names
	if len(sampleKeys) > 5 {
		sampleKeys = sampleKeys[:5]
	}
	return map[string]any{
		"key_count":   len(cookies),
		"total_bytes": totalBytes,
		"sample_keys": sampleKeys,
	}
}

// filterStorageMap filters a storage map by key. Returns nil if key not found.
func filterStorageMap(data map[string]any, key string) map[string]any {
	if key == "" {
		return data
	}
	if v, ok := data[key]; ok {
		return map[string]any{key: v}
	}
	return map[string]any{}
}

// filterCookies filters a cookie array by name.
func filterCookies(cookies []any, name string) []any {
	if name == "" {
		return cookies
	}
	var filtered []any
	for _, c := range cookies {
		if m, ok := c.(map[string]any); ok {
			if n, ok := m["name"].(string); ok && n == name {
				filtered = append(filtered, c)
			}
		}
	}
	return filtered
}

// attachSections copies the selected storage sections, filtered and optionally summarized.
func attachSections(response map[string]any, stateResult map[string]any, params StorageOptions) {
	if params.includes("local") {
		if v, ok := stateResult["localStorage"].(map[string]any); ok {
			response["local_storage"] = storageMapValue(v, params.Key, params.Summary)
		}
	}
	if params.includes("session") {
		if v, ok := stateResult["sessionStorage"].(map[string]any); ok {
			response["session_storage"] = storageMapValue(v, params.Key, params.Summary)
		}
	}
	if params.includes("cookies") {
		if v, ok := stateResult["cookies"].([]any); ok {
			response["cookies"] = cookiesValue(v, params.Key, params.Summary)
		}
	}
}

func storageMapValue(v map[string]any, key string, summary bool) map[string]any {
	filtered := filterStorageMap(v, key)
	if summary {
		return summarizeStorageMap(filtered)
	}
	return filtered
}

func cookiesValue(v []any, name string, summary bool) any {
	filtered := filterCookies(v, name)
	if summary {
		return summarizeCookies(filtered)
	}
	return filtered
}

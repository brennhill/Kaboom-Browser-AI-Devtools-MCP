// strings.go — String and string-keyed-map helpers shared across internal packages.
// Both are dependency-free pure functions over the same key/label vocabulary, so
// they share a file rather than each occupying one.

package util

import "sort"

// Truncate returns s unchanged if len(s) <= maxLen. Otherwise, it truncates
// and appends "..." so the total output length equals maxLen.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// SortedMapKeys returns the keys of a string-keyed map in sorted order.
func SortedMapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

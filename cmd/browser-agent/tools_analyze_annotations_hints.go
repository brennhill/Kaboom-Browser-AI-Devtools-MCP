// tools_analyze_annotations_hints.go — Error correlation for annotation responses.
// Why: Correlates console errors with annotation timestamps.
// Docs: docs/features/feature/annotated-screenshots/index.md
// Note: Pure hint builders moved to cmd/browser-agent/internal/analyzehandler/annotation_hints.go.

package main

import "time"

// findErrorsNearTimestamp returns up to 5 error-level log entries within +/-window of the
// given timestamp (millis). Returns a slice of maps with message and ts fields.
func (h *ToolHandler) findErrorsNearTimestamp(tsMillis int64, window time.Duration) []map[string]string {
	entries, _ := h.GetLogEntries()
	annotTime := time.UnixMilli(tsMillis)
	windowStart := annotTime.Add(-window)
	windowEnd := annotTime.Add(window)

	var matched []map[string]string
	for i := len(entries) - 1; i >= 0 && len(matched) < 5; i-- {
		entry := entries[i]
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		tsStr, _ := entry["ts"].(string)
		if tsStr == "" {
			continue
		}
		entryTime, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			continue
		}
		if entryTime.Before(windowStart) || entryTime.After(windowEnd) {
			continue
		}
		msg, _ := entry["message"].(string)
		matched = append(matched, map[string]string{
			"message": msg,
			"ts":      tsStr,
		})
	}
	return matched
}

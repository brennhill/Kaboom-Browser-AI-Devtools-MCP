// Purpose: Provides shared time helpers — tolerant RFC3339 timestamp parsing and human-readable duration formatting.
// Why: Centralizes tolerant timestamp parsing behavior used across telemetry ingestion paths,
// and keeps the duration formatter beside it so both sides of the time story live in one file.
// Docs: docs/features/feature/backend-log-streaming/index.md

package util

import (
	"fmt"
	"time"
)

// ParseTimestamp parses an RFC3339 timestamp string, trying RFC3339Nano first
// (since it's a superset of RFC3339), then RFC3339 as a fallback.
// Returns zero time on failure.
func ParseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}

// FormatDuration formats a duration as human-readable (e.g., "5s", "2m30s", "1h15m").
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm%02ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%02dm", hours, mins)
}

// response.go — Owns shared response, asynchronous safety, and time-formatting helpers.
package util

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"time"
)

// RequireMethod checks that r.Method matches method. On mismatch, writes 405 JSON response and returns false.
func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return false
	}
	return true
}

// JSONResponse writes a JSON response with the given status code and data
func JSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] Error encoding JSON response: %v\n", err)
	}
}

// SafeGo launches fn in a goroutine with deferred panic recovery.
// On panic: logs stack trace to stderr. Does NOT os.Exit — background
// panics should be survivable so the daemon stays up.
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "[Kaboom] PANIC in background goroutine: %v\n%s\n", r, debug.Stack())
			}
		}()
		fn()
	}()
}

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

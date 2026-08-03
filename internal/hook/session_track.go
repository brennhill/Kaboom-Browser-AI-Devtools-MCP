// session_track.go — Session tracking hook for PostToolUse events.
// Records file interactions, detects redundant reads, and injects session summaries.

package hook

import (
	"fmt"
	"strings"
	"time"
)

// SessionTrackResult holds the output of the session tracking hook.
type SessionTrackResult struct {
	Context string
	Action  string // "recorded", "redundant_read", "summary"
}

// FormatContext returns the additionalContext string for the hook output.
func (r *SessionTrackResult) FormatContext() string {
	return r.Context
}

// RunSessionTrack records the tool use and optionally injects session context.
// Returns nil if nothing to inject (but still records the touch).
func RunSessionTrack(input Input, sessionDir string) *SessionTrackResult {
	return runSessionTrack(input, sessionDir, ReadTouches, AppendTouch)
}

func runSessionTrack(
	input Input,
	sessionDir string,
	readTouches func(string) ([]TouchEntry, error),
	appendTouch func(string, TouchEntry) error,
) *SessionTrackResult {
	entries, err := readTouches(sessionDir)
	if err != nil {
		return &SessionTrackResult{
			Context: "[Session] Tracking unavailable (session_touch_read_failed). The latest action was not recorded.",
			Action:  "persistence_failed",
		}
	}
	fields := input.ParseToolInput()
	action := classifyAction(input.ToolName)
	filePath := fields.FilePath

	summary := buildTouchSummary(input, fields)

	entry := TouchEntry{
		Timestamp: time.Now(),
		Tool:      input.ToolName,
		File:      filePath,
		Action:    action,
		Summary:   summary,
	}

	// Check for redundant read BEFORE recording this touch.
	var result *SessionTrackResult
	if action == "read" && filePath != "" {
		result = checkRedundantRead(entries, filePath)
	}

	// Always attempt to record the touch; a failed append supersedes contextual
	// hints so the caller never mistakes an unpersisted event for session state.
	if err := appendTouch(sessionDir, entry); err != nil {
		return &SessionTrackResult{
			Context: "[Session] Tracking unavailable (session_touch_append_failed). The latest action was not recorded.",
			Action:  "persistence_failed",
		}
	}

	// For edits/writes, inject session summary.
	if result == nil && (action == "edit" || action == "write") {
		if s := sessionSummary(append(entries, entry)); s != "" {
			result = &SessionTrackResult{Context: s, Action: "summary"}
		}
	}

	return result
}

// checkRedundantRead checks if a file was already read this session.
func checkRedundantRead(entries []TouchEntry, filePath string) *SessionTrackResult {
	wasRead, readAt := wasFileRead(entries, filePath)
	if !wasRead {
		return nil
	}

	elapsed := time.Since(readAt)
	elapsedStr := formatDuration(elapsed)

	// Check if it was edited since the last read.
	wasEdited, editAt := wasFileEdited(entries, filePath, readAt)
	if wasEdited {
		editElapsed := formatDuration(time.Since(editAt))
		return &SessionTrackResult{
			Context: fmt.Sprintf("[Session] You read this file %s ago. You edited it %s ago.", elapsedStr, editElapsed),
			Action:  "redundant_read",
		}
	}

	return &SessionTrackResult{
		Context: fmt.Sprintf("[Session] You read this file %s ago. No edits since.", elapsedStr),
		Action:  "redundant_read",
	}
}

// classifyAction maps tool names to action types.
func classifyAction(toolName string) string {
	switch toolName {
	case "Read", "read_file":
		return "read"
	case "Edit", "replace_in_file", "edit_file":
		return "edit"
	case "Write", "write_file":
		return "write"
	case "Bash", "run_shell_command":
		return "bash"
	default:
		return "other"
	}
}

// buildTouchSummary extracts a short summary from the tool input.
func buildTouchSummary(input Input, fields ToolInputFields) string {
	switch classifyAction(input.ToolName) {
	case "edit":
		return truncStr(fields.NewString, maxSummaryLen)
	case "bash":
		return truncStr(fields.Command, maxSummaryLen)
	case "write":
		return truncStr(fields.Content, maxSummaryLen)
	}
	return ""
}

func truncStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d sec", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d hr", int(d.Hours()))
}

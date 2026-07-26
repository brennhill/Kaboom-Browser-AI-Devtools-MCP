// Purpose: Manages playback session lifecycle including start, result collection, and completion.
// Why: Separates session orchestration from individual action execution.
package playback

import (
	"fmt"
	"time"
)

// Start opens a playback session for a recording, rejecting recordings that
// cannot be loaded or that captured no actions.
func Start(src RecordingSource, recordingID string) (*Session, error) {
	rec, err := src.LookupRecording(recordingID)
	if err != nil {
		return nil, fmt.Errorf("playback_recording_not_found: Recording %s not found: %w", recordingID, err)
	}

	if rec == nil || len(rec.Actions) == 0 {
		return nil, fmt.Errorf("playback_no_actions: Recording has no actions to replay")
	}

	session := &Session{
		RecordingID:      recordingID,
		StartedAt:        time.Now(),
		Results:          make([]Result, 0),
		SelectorFailures: make(map[string]int),
	}

	return session, nil
}

// Execute opens a session and replays every action in the recording into it.
func Execute(src RecordingSource, recordingID string) (*Session, error) {
	session, err := Start(src, recordingID)
	if err != nil {
		return nil, err
	}

	// Re-read: the recording can be deleted between Start and here.
	rec, _ := src.LookupRecording(recordingID)
	if rec == nil {
		return nil, fmt.Errorf("playback_load_failed: Could not load recording")
	}

	for i, action := range rec.Actions {
		result := executeAction(i, action)
		session.Results = append(session.Results, result)

		if result.Status == "error" {
			session.ActionsFailed++
			if action.Selector != "" {
				session.SelectorFailures[action.Selector]++
			}
			continue
		}

		session.ActionsExecuted++
	}

	return session, nil
}

// Status summarizes a session for the MCP response.
func Status(session *Session) map[string]any {
	totalTime := time.Since(session.StartedAt)

	status := "ok"
	if session.ActionsFailed > 0 {
		status = "partial"
	}
	if session.ActionsExecuted == 0 {
		status = "failed"
	}

	return map[string]any{
		"status":            status,
		"actions_executed":  session.ActionsExecuted,
		"actions_failed":    session.ActionsFailed,
		"actions_total":     session.ActionsExecuted + session.ActionsFailed,
		"duration_ms":       totalTime.Milliseconds(),
		"results_count":     len(session.Results),
		"selector_failures": session.SelectorFailures,
	}
}

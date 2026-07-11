// helpers.go — Shared recording/playback response builders.
// Why: Keeps common formatting DRY across recording-related handlers.
// Docs: docs/features/feature/flow-recording/index.md

package recordinghandler

import (
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// BuildPlaybackResult constructs canonical playback completion payload.
func BuildPlaybackResult(req mcp.JSONRPCRequest, recordingID string, session *capture.PlaybackSession) mcp.JSONRPCResponse {
	status := "ok"
	if session.ActionsFailed > 0 {
		status = "partial"
	}
	total := session.ActionsExecuted + session.ActionsFailed
	responseData := map[string]any{
		"status":            status,
		"recording_id":      recordingID,
		"actions_executed":  session.ActionsExecuted,
		"actions_failed":    session.ActionsFailed,
		"actions_total":     total,
		"duration_ms":       time.Since(session.StartedAt).Milliseconds(),
		"results_count":     len(session.Results),
		"selector_failures": session.SelectorFailures,
	}
	message := fmt.Sprintf("Playback complete: %d/%d actions executed", session.ActionsExecuted, total)
	return mcp.Succeed(req, message, responseData)
}

// Purpose: Thin adapter for recording helpers — delegates to toolrecording sub-package.
// Docs: docs/features/feature/flow-recording/index.md

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrecording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// buildPlaybackResult delegates to the toolrecording sub-package.
func (h *ToolHandler) buildPlaybackResult(req JSONRPCRequest, recordingID string, session *capture.PlaybackSession) JSONRPCResponse {
	return toolrecording.BuildPlaybackResult(req, recordingID, session)
}

// appendServerLog appends one entry to bounded in-memory daemon logs.
// Delegates to LogStore.addEntries to maintain counters, rotation, and onEntries callback.
func (h *ToolHandler) appendServerLog(entry LogEntry) {
	h.server.logs.AddEntries([]LogEntry{entry})
}

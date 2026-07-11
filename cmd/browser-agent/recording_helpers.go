// Purpose: Thin adapter for recording helpers — delegates to recordinghandler sub-package.
// Docs: docs/features/feature/flow-recording/index.md

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/recordinghandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// buildPlaybackResult delegates to the recordinghandler sub-package.
func (h *ToolHandler) buildPlaybackResult(req JSONRPCRequest, recordingID string, session *capture.PlaybackSession) JSONRPCResponse {
	return recordinghandler.BuildPlaybackResult(req, recordingID, session)
}

// appendServerLog appends one entry to bounded in-memory daemon logs.
// Delegates to LogStore.addEntries to maintain counters, rotation, and onEntries callback.
func (h *ToolHandler) appendServerLog(entry LogEntry) {
	h.server.logs.addEntries([]LogEntry{entry})
}

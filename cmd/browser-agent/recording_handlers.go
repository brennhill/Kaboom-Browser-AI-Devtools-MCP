// recording_handlers.go — Main-package adapters for the recording MCP subsystem.
// Why: Dispatch stays in package main while recording behavior and state live together.
// Docs: docs/features/feature/flow-recording/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrecording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func (h *ToolHandler) toolConfigureEventRecordingStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.recordingHandler.EventRecordingStart(req, args)
}

func (h *ToolHandler) toolConfigureEventRecordingStop(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.recordingHandler.EventRecordingStop(req, args)
}

func (h *ToolHandler) toolGetRecordings(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.recordingHandler.Recordings(req, args)
}

func (h *ToolHandler) toolGetRecordingActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.recordingHandler.RecordingActions(req, args)
}

func (h *ToolHandler) toolConfigurePlayback(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.recordingHandler.Playback(req, args)
}

func (h *ToolHandler) toolGetPlaybackResults(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.recordingHandler.PlaybackResults(req, args)
}

func (h *ToolHandler) toolConfigureLogDiff(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.recordingHandler.LogDiff(req, args)
}

func (h *ToolHandler) toolGetLogDiffReport(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.recordingHandler.LogDiffReport(req, args)
}

// buildPlaybackResult preserves the package-main test seam while delegating formatting.
func (h *ToolHandler) buildPlaybackResult(req JSONRPCRequest, recordingID string, session *capture.PlaybackSession) JSONRPCResponse {
	return toolrecording.BuildPlaybackResult(req, recordingID, session)
}

// appendServerLog delegates bounded daemon-log storage to the server-owned store.
func (h *ToolHandler) appendServerLog(entry LogEntry) {
	h.server.logs.AddEntries([]LogEntry{entry})
}

// tools_configure_sequence.go — Adapts configure sequence actions to their cohesive handler package.
// Docs: docs/features/feature/batch-sequences/index.md

package main

import (
	"encoding/json"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/sequencehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
)

// Sequence remains an alias for root-package tests and compatibility.
type Sequence = toolconfigure.Sequence

// replayMu is intentionally shared by batch and saved-sequence execution.
var replayMu sync.Mutex

// extractErrorMessage preserves the historical root helper for existing callers.
func extractErrorMessage(response JSONRPCResponse) string {
	if message := replay.ErrorMessage(response); message != "" {
		return message
	}
	return "unknown error"
}

func (h *ToolHandler) sequenceHandler() *sequencehandler.Handler {
	return sequencehandler.New(sequencehandler.Deps{
		Store:          h.sessionStoreImpl,
		ReplayMu:       &replayMu,
		Interact:       h.toolInteract,
		WaitForCommand: h.capture.WaitForCommand,
		RecordAction:   h.recordAIAction,
	})
}

func (h *ToolHandler) toolConfigureSaveSequence(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.sequenceHandler().Save(req, args)
}

func (h *ToolHandler) toolConfigureGetSequence(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.sequenceHandler().Get(req, args)
}

func (h *ToolHandler) toolConfigureListSequences(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.sequenceHandler().List(req, args)
}

func (h *ToolHandler) toolConfigureDeleteSequence(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.sequenceHandler().Delete(req, args)
}

func (h *ToolHandler) toolConfigureReplaySequence(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.sequenceHandler().Replay(req, args)
}

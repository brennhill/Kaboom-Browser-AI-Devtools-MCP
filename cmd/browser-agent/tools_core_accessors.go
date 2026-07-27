// Purpose: ToolHandler lifecycle and shared accessor helpers.
// Why: Keeps boilerplate getters/lifecycle methods out of the core wiring file.

package main

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testgenhandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
)

// Close cancels the shutdown context, unblocking any in-flight readiness gates.
func (h *ToolHandler) Close() {
	if h.shutdownCancel != nil {
		h.shutdownCancel()
	}
}

// GetCapture returns the capture instance.
func (h *ToolHandler) GetCapture() *capture.Store {
	return h.capture
}

// GetLogEntries returns a snapshot of the server's log entries and their timestamps.
// The returned slices are copies — safe to use without holding the server lock.
func (h *ToolHandler) GetLogEntries() ([]LogEntry, []time.Time) {
	return h.server.logs.EntriesWithAddedAt()
}

// GetLogTotalAdded returns the monotonic counter of total log entries ever added.
func (h *ToolHandler) GetLogTotalAdded() int64 {
	return h.server.logs.TotalAdded()
}

func (h *ToolHandler) ToolsList() []MCPTool {
	return schema.AllTools()
}

// armEvidenceForCommand delegates evidence arming to the interactActionHandler.
func (h *ToolHandler) armEvidenceForCommand(correlationID, action string, args json.RawMessage, clientID string) {
	h.interactAction().ArmEvidenceForCommand(correlationID, action, args, clientID)
}

// getCommandResult returns a command result by correlation ID from the capture store.
func (h *ToolHandler) getCommandResult(correlationID string) (*queries.CommandResult, bool) {
	return h.capture.GetCommandResult(correlationID)
}

// IsExtensionConnected reports whether the browser extension is connected.
// Satisfies toolobserve.Deps.
func (h *ToolHandler) IsExtensionConnected() bool {
	return h.capture.IsExtensionConnected()
}

// PushInbox returns the push inbox, or nil if unavailable.
// Satisfies toolobserve.Deps.
func (h *ToolHandler) PushInbox() *push.PushInbox {
	return h.server.pushInbox
}

// GetAnnotationStore returns the annotation store for draw mode data.
func (h *ToolHandler) GetAnnotationStore() *AnnotationStore {
	return h.annotationStore
}

// GetVersion returns the server version string.
func (h *ToolHandler) GetVersion() string {
	return version
}

// GetToolCallLimiter returns the tool call limiter.
func (h *ToolHandler) GetToolCallLimiter() RateLimiter {
	return h.toolCallLimiter
}

// GetRedactionEngine returns the redaction engine.
func (h *ToolHandler) GetRedactionEngine() RedactionEngine {
	return h.redactionEngine
}

// testGen returns the test-generation sub-handler (internal/testgenhandler).
func (h *ToolHandler) testGen() *testgenhandler.Handler {
	return h.testGenHandler
}

// tools_configure_deps_adapter.go — Adapts ToolHandler to satisfy configurehandler.Deps interface.
// Why: Provides narrow accessor methods that bridge ToolHandler fields to the configure sub-package.

package main

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// NoiseConfig satisfies configurehandler.Deps.
func (h *ToolHandler) NoiseConfig() *noise.NoiseConfig {
	return h.noiseConfig
}

// ConsoleEntries satisfies configurehandler.Deps.
func (h *ToolHandler) ConsoleEntries() []noise.LogEntry {
	h.server.logs.mu.RLock()
	entries := make([]noise.LogEntry, len(h.server.logs.entries))
	for i, e := range h.server.logs.entries {
		entries[i] = noise.LogEntry(e)
	}
	h.server.logs.mu.RUnlock()
	return entries
}

// NetworkBodies satisfies configurehandler.Deps.
func (h *ToolHandler) NetworkBodies() []types.NetworkBody {
	return h.capture.GetNetworkBodies()
}

// AllWebSocketEvents satisfies configurehandler.Deps.
func (h *ToolHandler) AllWebSocketEvents() []types.WebSocketEvent {
	return h.capture.GetAllWebSocketEvents()
}

// GetTrackingStatus satisfies configurehandler.Deps.
// Note: Already satisfies observe.Deps via capture delegation — different interface path.
func (h *ToolHandler) GetTrackingStatus() (bool, int, string) {
	return h.capture.GetTrackingStatus()
}

// GetPilotStatus satisfies configurehandler.Deps.
func (h *ToolHandler) GetPilotStatus() any {
	return h.capture.GetPilotStatus()
}

// GetToolModuleExamples satisfies configurehandler.Deps.
func (h *ToolHandler) GetToolModuleExamples(toolName string) any {
	h.ensureToolModules()
	if module, ok := h.toolModules.get(toolName); ok {
		if examples := module.Examples(); len(examples) > 0 {
			return examples
		}
	}
	return nil
}

// GetSecurityMode satisfies configurehandler.Deps.
func (h *ToolHandler) GetSecurityMode() (string, bool, []string) {
	return h.capture.GetSecurityMode()
}

// SetSecurityMode satisfies configurehandler.Deps.
func (h *ToolHandler) SetSecurityMode(mode string, rewrites []string) {
	h.capture.SetSecurityMode(mode, rewrites)
}

// GetTelemetryMode satisfies configurehandler.Deps.
func (h *ToolHandler) GetTelemetryMode() string {
	return h.server.logs.getTelemetryMode()
}

// SetTelemetryMode satisfies configurehandler.Deps.
func (h *ToolHandler) SetTelemetryMode(mode string) {
	h.server.logs.setTelemetryMode(mode)
}

// InteractActionSetJitter satisfies configurehandler.Deps.
func (h *ToolHandler) InteractActionSetJitter(ms int) {
	h.interactAction().SetJitter(ms)
}

// InteractActionGetJitter satisfies configurehandler.Deps.
func (h *ToolHandler) InteractActionGetJitter() int {
	return h.interactAction().GetJitter()
}

// HasCapture satisfies configurehandler.Deps.
func (h *ToolHandler) HasCapture() bool {
	return h.capture != nil
}

// GetActiveCodebase satisfies configurehandler.Deps.
func (h *ToolHandler) GetActiveCodebase() string {
	return h.server.GetActiveCodebase()
}

// StartTestBoundary satisfies configurehandler.Deps.
func (h *ToolHandler) StartTestBoundary(testID string) {
	h.activeBoundariesMu.Lock()
	defer h.activeBoundariesMu.Unlock()
	if h.activeBoundaries == nil {
		h.activeBoundaries = make(map[string]time.Time)
	}
	h.activeBoundaries[testID] = time.Now()
}

// EndTestBoundary satisfies configurehandler.Deps.
func (h *ToolHandler) EndTestBoundary(testID string) bool {
	h.activeBoundariesMu.Lock()
	defer h.activeBoundariesMu.Unlock()
	_, active := h.activeBoundaries[testID]
	if active {
		delete(h.activeBoundaries, testID)
	}
	return active
}

// AuditTrailReady satisfies configurehandler.Deps.
func (h *ToolHandler) AuditTrailReady() bool {
	return h.auditTrail != nil
}

// QueryAuditLog satisfies configurehandler.Deps.
func (h *ToolHandler) QueryAuditLog(filter audit.Filter) []audit.Entry {
	return h.auditTrail.Query(filter)
}

// ClearAuditLog satisfies configurehandler.Deps. Clears the trail and its session map under lock.
func (h *ToolHandler) ClearAuditLog() int {
	cleared := h.auditTrail.Clear()
	h.auditMu.Lock()
	h.auditSessionMap = make(map[string]string)
	h.auditMu.Unlock()
	return cleared
}

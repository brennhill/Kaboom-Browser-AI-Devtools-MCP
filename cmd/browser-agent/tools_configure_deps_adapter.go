// tools_configure_deps_adapter.go — Adapts ToolHandler to satisfy toolconfigure.Deps interface.
// Why: Provides narrow accessor methods that bridge ToolHandler fields to the configure sub-package.

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// NoiseConfig satisfies toolconfigure.Deps.
func (h *ToolHandler) NoiseConfig() *noise.NoiseConfig {
	return h.noiseConfig
}

// ConsoleEntries satisfies toolconfigure.Deps.
func (h *ToolHandler) ConsoleEntries() []noise.LogEntry {
	snapshot := h.server.logs.Entries()
	entries := make([]noise.LogEntry, len(snapshot))
	for i, e := range snapshot {
		entries[i] = noise.LogEntry(e)
	}
	return entries
}

// NetworkBodies satisfies toolconfigure.Deps.
func (h *ToolHandler) NetworkBodies() []types.NetworkBody {
	return h.capture.GetNetworkBodies()
}

// AllWebSocketEvents satisfies toolconfigure.Deps.
func (h *ToolHandler) AllWebSocketEvents() []types.WebSocketEvent {
	return h.capture.GetAllWebSocketEvents()
}

// GetTrackingStatus satisfies toolconfigure.Deps.
// Note: Already satisfies observe.Deps via capture delegation — different interface path.
func (h *ToolHandler) GetTrackingStatus() (bool, int, string) {
	return h.capture.GetTrackingStatus()
}

// GetPilotStatus satisfies toolconfigure.Deps.
func (h *ToolHandler) GetPilotStatus() any {
	return h.capture.GetPilotStatus()
}

// GetToolModuleExamples satisfies toolconfigure.Deps.
func (h *ToolHandler) GetToolModuleExamples(toolName string) any {
	h.ensureToolModules()
	if module, ok := h.toolModules.get(toolName); ok {
		if examples := module.Examples(); len(examples) > 0 {
			return examples
		}
	}
	return nil
}

// GetSecurityMode satisfies toolconfigure.Deps.
func (h *ToolHandler) GetSecurityMode() (string, bool, []string) {
	return h.capture.GetSecurityMode()
}

// SetSecurityMode satisfies toolconfigure.Deps.
func (h *ToolHandler) SetSecurityMode(mode string, rewrites []string) {
	h.capture.SetSecurityMode(mode, rewrites)
}

// GetTelemetryMode satisfies toolconfigure.Deps.
func (h *ToolHandler) GetTelemetryMode() string {
	return h.server.logs.TelemetryMode()
}

// SetTelemetryMode satisfies toolconfigure.Deps.
func (h *ToolHandler) SetTelemetryMode(mode string) {
	h.server.logs.SetTelemetryMode(mode)
}

// InteractActionSetJitter satisfies toolconfigure.Deps.
func (h *ToolHandler) InteractActionSetJitter(ms int) {
	h.interactAction().SetJitter(ms)
}

// InteractActionGetJitter satisfies toolconfigure.Deps.
func (h *ToolHandler) InteractActionGetJitter() int {
	return h.interactAction().GetJitter()
}

// HasCapture satisfies toolconfigure.Deps.
func (h *ToolHandler) HasCapture() bool {
	return h.capture != nil
}

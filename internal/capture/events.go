// events.go — Enriches incoming telemetry and composes independent owners.
// Purpose: Owns cross-store ingestion concerns and coordinated runtime resets.
// Why: Each telemetry family owns retention while this layer applies active test
// context and binary metadata before handing values to that canonical owner.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/perfstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// StateResetter owns the coordinated reset of capture runtime stores.
type StateResetter struct {
	extension     *ExtensionRuntime
	telemetry     *telemetrystore.Store
	performance   *perfstore.Store
	extensionLogs *logstore.Extension
}

// NewStateResetter binds reset behavior to the canonical state owners.
func NewStateResetter(capture *Capture) *StateResetter {
	return &StateResetter{
		extension:     capture.extension,
		telemetry:     capture.telemetry,
		performance:   capture.perf,
		extensionLogs: capture.extensionLogs,
	}
}

// ClearAll resets all capture-owned in-memory telemetry state — INCLUDING
// extension logs — and returns the number of extension-log entries cleared.
func (r *StateResetter) ClearAll() int {
	r.extension.ClearTestBoundaries()
	r.telemetry.ClearAll()
	r.performance.Clear()
	return r.extensionLogs.Clear()
}

func (h *HTTPHandlers) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	body, ok := h.readIngestBody(w, r)
	if !ok {
		return
	}
	var payload struct {
		Events []types.WebSocketEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if !h.recordAndRecheck(w, len(payload.Events)) {
		return
	}
	h.capture.telemetry.AddWebSocketEvents(payload.Events)
	w.WriteHeader(http.StatusOK)
}

func (h *HTTPHandlers) HandleWebSocketStatus(w http.ResponseWriter, _ *http.Request) {
	status := h.capture.telemetry.WebSockets().Status(types.WebSocketStatusFilter{})
	util.JSONResponse(w, http.StatusOK, status)
}

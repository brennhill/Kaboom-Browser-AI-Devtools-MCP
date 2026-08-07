// handler.go — Serves bounded local browser telemetry snapshots over HTTP.

package telemetryapi

import (
	"net/http"
	"strconv"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

const validTypes = "logs, network_waterfall, network_bodies, websocket_events, actions, performance_snapshots, extension_logs, websocket_status"

// Handler returns the local telemetry read endpoint.
func Handler(logs *logstore.Store, captured *capture.Capture) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		telemetryType := request.URL.Query().Get("type")
		if telemetryType == "" {
			httpapi.JSON(w, http.StatusBadRequest, map[string]string{"error": "Missing required 'type' parameter", "hint": "Valid types: " + validTypes})
			return
		}
		limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
		result, count, ok := snapshot(telemetryType, limit, logs, captured)
		if !ok {
			httpapi.JSON(w, http.StatusBadRequest, map[string]string{"error": "Unknown telemetry type: " + telemetryType, "hint": "Valid types: " + validTypes})
			return
		}
		if telemetryType == "websocket_status" {
			status := result.(types.WebSocketStatusResponse) // snapshot owns this concrete result.
			httpapi.JSON(w, http.StatusOK, map[string]any{"type": telemetryType, "connections": status.Connections, "closed": status.Closed, "count": count})
			return
		}
		httpapi.JSON(w, http.StatusOK, map[string]any{"type": telemetryType, "items": result, "count": count})
	}
}

func snapshot(telemetryType string, limit int, logs *logstore.Store, captured *capture.Capture) (any, int, bool) {
	switch telemetryType {
	case "logs":
		entries := tail(logs.Entries(), limit)
		return entries, len(entries), true
	case "network_waterfall":
		entries := tail(captured.Telemetry().NetworkWaterfall().Entries(), limit)
		return entries, len(entries), true
	case "network_bodies":
		entries := tail(captured.Telemetry().NetworkBodies().Snapshot().Bodies, limit)
		return entries, len(entries), true
	case "websocket_events":
		entries := tail(captured.Telemetry().WebSockets().Events(types.WebSocketEventFilter{}), limit)
		return entries, len(entries), true
	case "actions":
		entries := tail(captured.Telemetry().Actions().Snapshot().Actions, limit)
		return entries, len(entries), true
	case "performance_snapshots":
		entries := tail(captured.Performance().Entries(), limit)
		return entries, len(entries), true
	case "extension_logs":
		entries := tail(captured.ExtensionLogs().Entries(), limit)
		return entries, len(entries), true
	case "websocket_status":
		status := captured.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
		return status, len(status.Connections), true
	default:
		return nil, 0, false
	}
}

func tail[T any](entries []T, limit int) []T {
	if limit > 0 && len(entries) > limit {
		return entries[len(entries)-limit:]
	}
	return entries
}

// Purpose: Appends passive telemetry summaries (error/network/action deltas since last call) to MCP tool responses.
// Why: Gives agents ambient situational awareness without requiring explicit observe calls between actions.

package main

import (
	"encoding/json"
	"time"
)

const defaultTelemetryClientKey = "_default"

const (
	telemetryModeOff  = "off"
	telemetryModeAuto = "auto"
	telemetryModeFull = "full"
)

// telemetryCursorTTL is the maximum age for a cursor entry before cleanup removes it.
const telemetryCursorTTL = 30 * time.Minute

// telemetryCursorMaxEntries caps the cursor map to prevent unbounded growth
// even if TTL cleanup hasn't run yet.
const telemetryCursorMaxEntries = 200

type passiveTelemetryCursor struct {
	errorTotal        int64
	networkTotal      int64
	networkErrorTotal int64
	wsTotal           int64
	actionTotal       int64
	lastSeen          time.Time
}

func (h *MCPHandler) maybeAddTelemetrySummary(resp JSONRPCResponse, clientID, toolName, modeOverride string) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}

	summary, changed := h.buildTelemetrySummary(clientID, toolName)
	mode := h.resolveTelemetryMode(modeOverride)
	if mode == telemetryModeOff {
		return resp
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	// Only mutate canonical MCP tool result payloads.
	if len(result.Content) == 0 {
		return resp
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	result.Metadata["telemetry_changed"] = changed
	if mode == telemetryModeFull || (mode == telemetryModeAuto && changed) {
		result.Metadata["telemetry_summary"] = summary
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

func (h *MCPHandler) buildTelemetrySummary(clientID, toolName string) (map[string]any, bool) {
	current := h.currentTelemetryCursor()
	deltas := h.telemetryDeltasForClient(clientID, current)
	changed := deltas.errorTotal > 0 ||
		deltas.networkTotal > 0 ||
		deltas.networkErrorTotal > 0 ||
		deltas.wsTotal > 0 ||
		deltas.actionTotal > 0

	summary := map[string]any{
		"new_errors_since_last_call":           deltas.errorTotal,
		"new_network_requests_since_last_call": deltas.networkTotal,
		"new_network_errors_since_last_call":   deltas.networkErrorTotal,
		"new_websocket_events_since_last_call": deltas.wsTotal,
		"new_actions_since_last_call":          deltas.actionTotal,
		"trigger_tool":                         toolName,
		"retrieved_at":                         time.Now().UTC().Format(time.RFC3339),
	}

	cap := h.toolHandler.GetCapture()
	if cap != nil {
		summary["extension_connected"] = cap.IsExtensionConnected()
		enabled, tabID, tabURL := cap.GetTrackingStatus()
		summary["tracking_enabled"] = enabled
		if tabID > 0 {
			summary["tracked_tab_id"] = tabID
		}
		if tabURL != "" {
			summary["tracked_tab_url"] = tabURL
		}
	}
	if clientID != "" {
		summary["client_id"] = clientID
	}

	return summary, changed
}

func (h *MCPHandler) currentTelemetryCursor() passiveTelemetryCursor {
	current := passiveTelemetryCursor{}

	if h.server != nil {
		current.errorTotal = h.server.logs.ErrorTotalAdded()
	}

	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return current
	}
	current.networkTotal = cap.GetNetworkTotalAdded()
	current.networkErrorTotal = cap.GetNetworkErrorTotalAdded()
	current.wsTotal = cap.GetWebSocketTotalAdded()
	current.actionTotal = cap.GetActionTotalAdded()
	return current
}

func (h *MCPHandler) telemetryDeltasForClient(clientID string, current passiveTelemetryCursor) passiveTelemetryCursor {
	key := clientID
	if key == "" {
		key = defaultTelemetryClientKey
	}

	h.telemetryMu.Lock()
	defer h.telemetryMu.Unlock()

	if h.telemetryCursors == nil {
		h.telemetryCursors = make(map[string]passiveTelemetryCursor)
	}

	previous, ok := h.telemetryCursors[key]
	current.lastSeen = time.Now()
	h.telemetryCursors[key] = current

	// Inline eviction: if map exceeds cap, remove oldest entries.
	if len(h.telemetryCursors) > telemetryCursorMaxEntries {
		h.evictStaleCursorsLocked()
	}

	if !ok {
		return passiveTelemetryCursor{}
	}

	return passiveTelemetryCursor{
		errorTotal:        clampDelta(current.errorTotal, previous.errorTotal),
		networkTotal:      clampDelta(current.networkTotal, previous.networkTotal),
		networkErrorTotal: clampDelta(current.networkErrorTotal, previous.networkErrorTotal),
		wsTotal:           clampDelta(current.wsTotal, previous.wsTotal),
		actionTotal:       clampDelta(current.actionTotal, previous.actionTotal),
	}
}

func clampDelta(current, previous int64) int64 {
	if current <= previous {
		return 0
	}
	return current - previous
}

func parseTelemetryModeOverride(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(args, &payload); err != nil {
		return ""
	}
	raw, ok := payload["telemetry_mode"].(string)
	if !ok {
		return ""
	}
	mode, ok := normalizeTelemetryMode(raw)
	if !ok {
		return ""
	}
	return mode
}

func normalizeTelemetryMode(mode string) (string, bool) {
	switch mode {
	case telemetryModeOff, telemetryModeAuto, telemetryModeFull:
		return mode, true
	default:
		return "", false
	}
}

// evictStaleCursorsLocked removes cursor entries older than telemetryCursorTTL.
// Must be called with h.telemetryMu held.
func (h *MCPHandler) evictStaleCursorsLocked() {
	cutoff := time.Now().Add(-telemetryCursorTTL)
	for key, cursor := range h.telemetryCursors {
		if cursor.lastSeen.Before(cutoff) {
			delete(h.telemetryCursors, key)
		}
	}
}

func (h *MCPHandler) resolveTelemetryMode(modeOverride string) string {
	if mode, ok := normalizeTelemetryMode(modeOverride); ok {
		return mode
	}
	if h.server != nil {
		mode, ok := normalizeTelemetryMode(h.server.logs.TelemetryMode())
		if ok {
			return mode
		}
	}
	return telemetryModeAuto
}

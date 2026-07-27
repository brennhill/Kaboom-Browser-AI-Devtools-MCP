// Purpose: Validates and executes MCP tools/call, then applies response guards.
// Why: Isolates tool-call lifecycle concerns from transport and generic method dispatch.

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// handleToolsCall validates tool call payload, executes tool, then applies response guards.
//
// Failure semantics:
// - Invalid JSON args, missing tool handler, unknown tool, and rate-limit breaches are explicit errors.
// - Tool post-processing (redaction/warnings/telemetry) is best-effort and never blocks success path.
func (h *MCPHandler) handleToolsCall(req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion, ID: req.ID,
			Error: &JSONRPCError{Code: -32602, Message: "Invalid params: " + err.Error()},
		}
	}

	if h.toolHandler == nil {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion, ID: req.ID,
			Error: &JSONRPCError{Code: -32601, Message: "Unknown tool: " + params.Name},
		}
	}

	h.warnUnknownToolArguments(params.Name, params.Arguments)

	if err := h.checkToolRateLimit(); err != nil {
		telemetry.AppError("tool_rate_limited", nil)
		return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Error: err}
	}

	resp, handled := h.toolHandler.HandleToolCall(req, params.Name, params.Arguments)
	if !handled {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion, ID: req.ID,
			Error: &JSONRPCError{Code: -32601, Message: "Unknown tool: " + params.Name},
		}
	}

	telemetryModeOverride := parseTelemetryModeOverride(params.Arguments)
	resp = h.applyToolResponsePostProcessing(resp, req.ClientID, params.Name, telemetryModeOverride)
	return resp
}

// checkToolRateLimit enforces per-process tool call throttling.
//
// Failure semantics:
// - Nil limiter means unlimited mode.
func (h *MCPHandler) checkToolRateLimit() *JSONRPCError {
	limiter := h.toolHandler.GetToolCallLimiter()
	if limiter != nil && !limiter.Allow() {
		return &JSONRPCError{
			Code:    -32603,
			Message: "Tool call rate limit exceeded (500 calls/minute). Please wait before retrying.",
		}
	}
	return nil
}

func (h *MCPHandler) warnUnknownToolArguments(toolName string, args json.RawMessage) {
	if h.server == nil || h.toolHandler == nil || len(args) == 0 {
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return
	}
	if len(raw) == 0 {
		return
	}

	allowed := h.allowedToolArgumentKeys(toolName, raw)
	if len(allowed) == 0 {
		return
	}

	unknown := make([]string, 0)
	for k := range raw {
		if _, ok := allowed[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		h.server.AddWarning(fmt.Sprintf("unknown parameter '%s' for tool '%s' (ignored)", k, toolName))
	}
}

func (h *MCPHandler) allowedToolArgumentKeys(toolName string, rawArgs map[string]json.RawMessage) map[string]struct{} {
	tools := h.toolHandler.ToolsList()
	for _, tool := range tools {
		if tool.Name != toolName {
			continue
		}

		keys := make(map[string]struct{})
		props, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			return keys
		}
		for k := range props {
			keys[k] = struct{}{}
		}
		return keys
	}
	return nil
}

func (h *MCPHandler) applyToolResponsePostProcessing(resp JSONRPCResponse, clientID, toolName, telemetryModeOverride string) JSONRPCResponse {
	redactor := h.toolHandler.GetRedactionEngine()
	if redactor != nil && resp.Result != nil {
		resp.Result = redactor.RedactJSON(resp.Result)
	}
	if h.server != nil {
		resp = appendWarningsToResponse(resp, h.server.TakeWarnings())
	}
	resp = h.maybeAddSecurityModeWarning(resp)
	resp = h.maybeAddVersionWarning(resp)
	resp = maybeAddUpdateAvailableWarning(resp)
	resp = maybeAddUpgradeWarning(resp)
	resp = h.maybeAddPendingIntents(resp)
	return h.maybeAddTelemetrySummary(resp, clientID, toolName, telemetryModeOverride)
}

func (h *MCPHandler) maybeAddPendingIntents(resp JSONRPCResponse) JSONRPCResponse {
	if h.server == nil || h.server.intentStore == nil || resp.Result == nil {
		return resp
	}
	if !h.server.intentStore.NudgeAndClean() {
		return resp
	}
	warning := "ACTION REQUIRED: The user clicked 'Audit' in the browser. " +
		"Run the Kaboom audit workflow (/kaboom/audit or /audit fallback) " +
		"for a full six-lane report.\n\n"
	return prependWarningToResponse(resp, warning)
}

func prependWarningToResponse(resp JSONRPCResponse, warning string) JSONRPCResponse {
	return mcp.PrependWarningToResponse(resp, warning)
}

func (h *MCPHandler) maybeAddSecurityModeWarning(resp JSONRPCResponse) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return resp
	}
	mode, productionParity, rewrites := cap.GetSecurityMode()
	if mode == capture.SecurityModeNormal {
		return resp
	}
	resp = prependWarningToResponse(resp, "[ALTERED ENVIRONMENT] security_mode=insecure_proxy; production_parity=false. CSP headers are rewritten for debugging.\n\n")
	return mutateToolResult(resp, func(result *MCPToolResult) {
		if result.Metadata == nil {
			result.Metadata = make(map[string]any)
		}
		result.Metadata["security_mode"] = mode
		result.Metadata["production_parity"] = productionParity
		result.Metadata["insecure_rewrites_applied"] = rewrites
	})
}

func (h *MCPHandler) maybeAddVersionWarning(resp JSONRPCResponse) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return resp
	}
	extVer, srvVer, mismatch := cap.GetVersionMismatch()
	if !mismatch {
		return resp
	}
	warning := fmt.Sprintf("WARNING: Version mismatch detected — server v%s, extension v%s. Update your extension to avoid issues.\n\n", srvVer, extVer)
	return prependWarningToResponse(resp, warning)
}

var (
	updateNotifyLastShown time.Time
	updateNotifyMu        sync.Mutex
)

func maybeAddUpdateAvailableWarning(resp JSONRPCResponse) JSONRPCResponse {
	if resp.Result == nil {
		return resp
	}
	if binaryUpgradeState != nil {
		if pending, _, _ := binaryUpgradeState.UpgradeInfo(); pending {
			return resp
		}
	}
	availableVersion := getAvailableVersion()
	if availableVersion == "" || !daemonlife.IsNewerVersion(availableVersion, version) {
		return resp
	}
	updateNotifyMu.Lock()
	recentlyShown := !updateNotifyLastShown.IsZero() && time.Since(updateNotifyLastShown) < 24*time.Hour
	if !recentlyShown {
		updateNotifyLastShown = time.Now()
	}
	updateNotifyMu.Unlock()
	if recentlyShown {
		return resp
	}
	warning := fmt.Sprintf("UPDATE AVAILABLE: Kaboom v%s is available (current: v%s). Run: npm install -g kaboom-agentic-browser@latest\n\n", availableVersion, version)
	return prependWarningToResponse(resp, warning)
}

func maybeAddUpgradeWarning(resp JSONRPCResponse) JSONRPCResponse {
	if binaryUpgradeState == nil || resp.Result == nil {
		return resp
	}
	pending, newVersion, detectedAt := binaryUpgradeState.UpgradeInfo()
	if !pending {
		return resp
	}
	elapsed := time.Since(detectedAt).Truncate(time.Second)
	warning := fmt.Sprintf("NOTICE: Kaboom v%s detected on disk (current: v%s, detected %s ago). Auto-restart imminent. Your next tool call will use the new version.\n\n", newVersion, version, elapsed)
	return prependWarningToResponse(resp, warning)
}

const defaultTelemetryClientKey = "_default"

const (
	telemetryModeOff  = "off"
	telemetryModeAuto = "auto"
	telemetryModeFull = "full"
)

const telemetryCursorTTL = 30 * time.Minute
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

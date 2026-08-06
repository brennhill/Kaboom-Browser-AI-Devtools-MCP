// owner.go — Adds bounded per-client passive browser telemetry to MCP tool responses.

package mcptelemetry

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

const (
	// ModeOff disables passive telemetry response metadata.
	ModeOff = "off"
	// ModeAuto includes details only when browser activity changed.
	ModeAuto = "auto"
	// ModeFull always includes the passive telemetry summary.
	ModeFull = "full"

	defaultClientKey = "_default"
	cursorTTL        = 30 * time.Minute
	cursorMaxEntries = 200
)

type cursor struct {
	errorTotal        int64
	networkTotal      int64
	networkErrorTotal int64
	webSocketTotal    int64
	actionTotal       int64
	lastSeen          time.Time
}

// Config supplies current aggregate totals and deterministic owner services.
type Config struct {
	ErrorTotal  func() int64
	Mode        func() string
	Now         func() time.Time
	Diagnosticf func(format string, args ...any)
}

// Owner retains bounded passive telemetry cursors independently per MCP client.
type Owner struct {
	mu       sync.Mutex
	cursors  map[string]cursor
	captured *capture.Capture
	config   Config
}

// New constructs a passive telemetry owner.
func New(config Config) *Owner {
	if config.ErrorTotal == nil {
		config.ErrorTotal = func() int64 { return 0 }
	}
	if config.Mode == nil {
		config.Mode = func() string { return ModeAuto }
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Diagnosticf == nil {
		config.Diagnosticf = diag.Printf
	}
	return &Owner{cursors: make(map[string]cursor), config: config}
}

// SetCapture installs the capture source during one-time MCP backend composition.
func (o *Owner) SetCapture(captured *capture.Capture) {
	o.captured = captured
}

// Augment adds telemetry metadata according to the configured or per-call mode.
func (o *Owner) Augment(response mcp.JSONRPCResponse, clientID, toolName string, arguments json.RawMessage) mcp.JSONRPCResponse {
	if response.Result == nil {
		return response
	}
	summary, changed := o.buildSummary(clientID, toolName)
	mode := o.resolveMode(parseModeOverride(arguments))
	if mode == ModeOff {
		return response
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil || len(result.Content) == 0 {
		o.config.Diagnosticf("[Kaboom] passive telemetry could not decode an MCP tool response; metadata was omitted\n")
		return response
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	result.Metadata["telemetry_changed"] = changed
	if mode == ModeFull || (mode == ModeAuto && changed) {
		result.Metadata["telemetry_summary"] = summary
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		o.config.Diagnosticf("[Kaboom] passive telemetry could not encode MCP metadata; original response retained\n")
		return response
	}
	response.Result = resultJSON
	return response
}

func (o *Owner) buildSummary(clientID, toolName string) (map[string]any, bool) {
	current := o.currentCursor()
	delta := o.deltaForClient(clientID, current)
	changed := delta.errorTotal > 0 || delta.networkTotal > 0 || delta.networkErrorTotal > 0 || delta.webSocketTotal > 0 || delta.actionTotal > 0
	summary := map[string]any{
		"new_errors_since_last_call": delta.errorTotal, "new_network_requests_since_last_call": delta.networkTotal,
		"new_network_errors_since_last_call": delta.networkErrorTotal, "new_websocket_events_since_last_call": delta.webSocketTotal,
		"new_actions_since_last_call": delta.actionTotal, "trigger_tool": toolName,
		"retrieved_at": o.config.Now().UTC().Format(time.RFC3339),
	}
	if o.captured != nil {
		summary["extension_connected"] = o.captured.Extension().IsExtensionConnected()
		enabled, tabID, tabURL := o.captured.Extension().GetTrackingStatus()
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

func (o *Owner) currentCursor() cursor {
	current := cursor{errorTotal: o.config.ErrorTotal()}
	if o.captured == nil {
		return current
	}
	current.networkTotal = o.captured.Telemetry().NetworkBodies().Stats().TotalAdded
	current.networkErrorTotal = o.captured.Telemetry().NetworkBodies().Stats().ErrorTotalAdded
	current.webSocketTotal = o.captured.Telemetry().WebSockets().Stats().TotalAdded
	current.actionTotal = o.captured.Telemetry().Actions().Stats().TotalAdded
	return current
}

func (o *Owner) deltaForClient(clientID string, current cursor) cursor {
	key := clientID
	if key == "" {
		key = defaultClientKey
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	previous, found := o.cursors[key]
	current.lastSeen = o.config.Now()
	o.cursors[key] = current
	if len(o.cursors) > cursorMaxEntries {
		o.evictExpiredLocked()
	}
	if !found {
		return cursor{}
	}
	return cursor{
		errorTotal:        clampDelta(current.errorTotal, previous.errorTotal),
		networkTotal:      clampDelta(current.networkTotal, previous.networkTotal),
		networkErrorTotal: clampDelta(current.networkErrorTotal, previous.networkErrorTotal),
		webSocketTotal:    clampDelta(current.webSocketTotal, previous.webSocketTotal),
		actionTotal:       clampDelta(current.actionTotal, previous.actionTotal),
	}
}

func (o *Owner) evictExpiredLocked() {
	cutoff := o.config.Now().Add(-cursorTTL)
	for key, value := range o.cursors {
		if value.lastSeen.Before(cutoff) {
			delete(o.cursors, key)
		}
	}
}

func (o *Owner) resolveMode(override string) string {
	if mode, valid := normalizeMode(override); valid {
		return mode
	}
	if mode, valid := normalizeMode(o.config.Mode()); valid {
		return mode
	}
	return ModeAuto
}

func parseModeOverride(arguments json.RawMessage) string {
	if len(arguments) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(arguments, &payload); err != nil {
		return "" // EXPECTED_ABSENCE: tool validation owns malformed arguments; telemetry has no independent mode to apply.
	}
	mode, _ := payload["telemetry_mode"].(string)
	return mode
}

func normalizeMode(mode string) (string, bool) {
	switch mode {
	case ModeOff, ModeAuto, ModeFull:
		return mode, true
	default:
		return "", false
	}
}

func clampDelta(current, previous int64) int64 {
	if current <= previous {
		return 0
	}
	return current - previous
}

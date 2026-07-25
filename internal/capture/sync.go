// Purpose: Implements /sync transport flow for settings, logs, command results, and pending command delivery.
// Why: Consolidates extension-daemon synchronization into a single resilient protocol surface.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/query-service/index.md

package capture

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// extractBrowserName returns a generic browser name from a User-Agent string.
// Only the browser family is returned — no version, OS, or device details.
func extractBrowserName(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "brave"):
		return "brave"
	case strings.Contains(ua, "edg/"):
		return "edge"
	case strings.Contains(ua, "chrome"):
		return "chrome"
	case strings.Contains(ua, "firefox"):
		return "firefox"
	case strings.Contains(ua, "safari"):
		return "safari"
	default:
		return "unknown"
	}
}

// =============================================================================
// Request/Response Types
// =============================================================================

// SyncRequest is the authoritative extension heartbeat payload for /sync.
//
// Invariants:
// - ExtSessionID is stable per extension runtime and changes on reload/update.
// - CommandResults and InProgress are best-effort snapshots; server must tolerate partial batches.
//
// Failure semantics:
// - Missing optional fields are treated as "no update" rather than protocol errors.
type SyncRequest struct {
	// Session identification
	ExtSessionID string `json:"ext_session_id"`

	// Extension version for compatibility checking
	ExtensionVersion string `json:"extension_version,omitempty"`

	// Extension settings (replaces /settings POST)
	Settings *SyncSettings `json:"settings,omitempty"`

	// Extension logs batch (replaces /extension-logs POST)
	ExtensionLogs []ExtensionLog `json:"extension_logs,omitempty"`

	// Ack last processed command ID (for reliable delivery)
	LastCommandAck string `json:"last_command_ack,omitempty"`

	// Command results batch (replaces multiple result POST endpoints)
	CommandResults []SyncCommandResult `json:"command_results,omitempty"`

	// Active commands currently executing in the extension.
	// Used to reconcile server/extension state and detect silent command loss.
	InProgress []SyncInProgress `json:"in_progress,omitempty"`

	// Feature usage flags from the extension (boolean "was this used since last sync").
	// Only UI-originated actions; see allowedFeatureKeys for the accepted set.
	FeaturesUsed map[string]bool `json:"features_used,omitempty"`
}

// SyncSettings contains extension settings from the sync request.
type SyncSettings struct {
	PilotEnabled     bool   `json:"pilot_enabled"`
	TrackingEnabled  bool   `json:"tracking_enabled"`
	TrackedTabID     int    `json:"tracked_tab_id"`
	TrackedTabURL    string `json:"tracked_tab_url"`
	TrackedTabTitle  string `json:"tracked_tab_title"`
	TabStatus        string `json:"tab_status,omitempty"`
	TrackedTabActive *bool  `json:"tracked_tab_active,omitempty"`
	CaptureLogs      bool   `json:"capture_logs"`
	CaptureNetwork   bool   `json:"capture_network"`
	CaptureWebSocket bool   `json:"capture_websocket"`
	CaptureActions   bool   `json:"capture_actions"`
	CspRestricted    bool   `json:"csp_restricted"`
	CspLevel         string `json:"csp_level"`
}

// SyncCommandResult is a command result from the extension.
type SyncCommandResult struct {
	ID            string          `json:"id"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	Status        string          `json:"status"` // "complete", "error", "timeout", "cancelled"
	Result        json.RawMessage `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
}

// SyncInProgress represents extension-reported active command execution state.
//
// Invariants:
// - Either ID or CorrelationID must be present after normalization.
// - Status is normalized to lower-case running/pending vocabulary.
type SyncInProgress struct {
	ID            string   `json:"id"`
	CorrelationID string   `json:"correlation_id,omitempty"`
	Type          string   `json:"type,omitempty"`
	Status        string   `json:"status,omitempty"` // running | pending
	ProgressPct   *float64 `json:"progress_pct,omitempty"`
	StartedAt     string   `json:"started_at,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
}

// SyncResponse is the response body for /sync.
type SyncResponse struct {
	// Server acknowledged the sync
	Ack bool `json:"ack"`

	// Commands for extension to execute (replaces /pending-queries GET)
	Commands []SyncCommand `json:"commands"`

	// Server-controlled poll interval (dynamic backoff)
	NextPollMs int `json:"next_poll_ms"`

	// Server time for drift detection
	ServerTime string `json:"server_time"`

	// Server version for compatibility
	ServerVersion string `json:"server_version,omitempty"`

	// InstallID is the server's persistent anonymous install identifier.
	// The extension adopts this as the single source of truth for all analytics.
	InstallID string `json:"install_id,omitempty"`

	// Capture overrides from AI (empty for now, placeholder for future feature)
	CaptureOverrides map[string]string `json:"capture_overrides"`
}

// SyncCommand is a command from server to extension.
type SyncCommand struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Params        json.RawMessage `json:"params"`
	TabID         int             `json:"tab_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	TraceID       string          `json:"trace_id,omitempty"`
}

// =============================================================================
// Handler
// =============================================================================

// HandleSync processes extension heartbeats and command transport in one endpoint.
//
// Invariants:
// - Connection state is updated before command/result reconciliation.
// allowedFeatureKeys is the set of known UI-originated feature keys.
// Only these are forwarded to the usage counter to prevent unbounded cardinality.
// WIRE-SYNCED: mirror of the `UIFeature` union in src/background/ui-usage-tracker.ts
// — add a key to BOTH or it is silently dropped here (CLAUDE.md rule 12).
var allowedFeatureKeys = map[string]bool{
	"screenshot":       true,
	"annotations":      true,
	"video":            true,
	"dom_action":       true,
	"action_recording": true,
}

// filterFeaturesUsed returns only the allowed keys from the raw features map.
func filterFeaturesUsed(raw map[string]bool) map[string]bool {
	if len(raw) == 0 {
		return nil
	}
	filtered := make(map[string]bool, len(allowedFeatureKeys))
	for key, val := range raw {
		if allowedFeatureKeys[key] {
			filtered[key] = val
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// - Lifecycle callbacks are emitted out-of-lock via util.SafeGo.
//
// Failure semantics:
// - Invalid JSON returns 400 and does not mutate capture state.
// - Extension disconnect transitions expire pending queries to avoid indefinite LLM waits.
// - Long-poll returns within bounded timeout even when no commands are queued.
func (c *Capture) HandleSync(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	now := time.Now()
	clientID := r.Header.Get("X-Kaboom-Client")

	state := c.updateSyncConnectionState(req, clientID, now)

	if !state.wasConnected || state.isReconnect {
		telemetry.BeaconEvent("extension_connect", map[string]string{"browser": extractBrowserName(r.Header.Get("User-Agent"))})
		util.SafeGo(func() {
			c.emitLifecycleEvent("extension_connected", map[string]any{
				"ext_session_id":     state.extSessionID,
				"is_reconnect":       state.isReconnect,
				"disconnect_seconds": state.timeSinceLastPoll.Seconds(),
			})
		})
	}

	// Forward extension feature usage to the usage counter via callback.
	// Only known UI-originated keys are forwarded to prevent unbounded counter cardinality.
	if filtered := filterFeaturesUsed(req.FeaturesUsed); len(filtered) > 0 {
		c.mu.RLock()
		cb := c.featuresCallback
		c.mu.RUnlock()
		if cb != nil {
			cb(filtered)
		}
	}

	c.processSyncCommandResults(req.CommandResults, clientID)
	if req.LastCommandAck != "" {
		c.AcknowledgePendingQuery(req.LastCommandAck)
	}

	if state.wasDisconnected {
		telemetry.AppError("extension_disconnect", nil)
		c.queryDispatcher.ExpireAllPendingQueries("extension_disconnected")
		util.SafeGo(func() {
			c.emitLifecycleEvent("extension_disconnected", map[string]any{
				"ext_session_id": state.extSessionID,
				"client_id":      clientID,
			})
		})
	}

	// Reconcile started commands against extension heartbeat state.
	// If a command disappears from heartbeat in_progress without a terminal result,
	// fail it fast instead of waiting for eventual timeout.
	c.reconcileInProgressCommandState(req.InProgress)

	pendingQueries := c.GetPendingQueries()
	if len(pendingQueries) == 0 {
		c.WaitForPendingQueries(syncLongPollTimeout())
		pendingQueries = c.GetPendingQueries()
	}

	c.updateSyncLogs(req, now, state.pilotEnabled, len(pendingQueries))

	commands := buildSyncCommands(pendingQueries)

	nextPollMs := 1000
	if len(commands) > 0 {
		nextPollMs = 200
	}
	if shouldEmitSyncSnapshot(req, state, len(commands)) {
		util.SafeGo(func() {
			c.emitLifecycleEvent("sync_snapshot", map[string]any{
				"ext_session_id":       state.extSessionID,
				"client_id":            clientID,
				"pilot_enabled":        state.pilotEnabled,
				"in_progress_count":    state.inProgressCount,
				"pending_commands_out": len(commands),
				"command_results_in":   len(req.CommandResults),
				"last_command_ack":     req.LastCommandAck,
				"next_poll_ms":         nextPollMs,
			})
		})
	}

	resp := SyncResponse{
		Ack:              true,
		Commands:         commands,
		NextPollMs:       nextPollMs,
		ServerTime:       now.Format(time.RFC3339),
		ServerVersion:    c.GetServerVersion(),
		InstallID:        telemetry.GetInstallID(),
		CaptureOverrides: c.buildCaptureOverrides(),
	}

	util.JSONResponse(w, http.StatusOK, resp)
}

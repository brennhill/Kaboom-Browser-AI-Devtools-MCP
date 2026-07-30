// Purpose: Implements the /sync transport flow — settings, logs, command results, pending command delivery and long-poll policy.
// Why: Consolidates extension-daemon synchronization into a single resilient protocol surface.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/query-service/index.md

package capture

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// SyncHandler owns the extension heartbeat, command transport, and
// disconnect-reconciliation boundary.
type SyncHandler struct {
	capture *Capture
}

// NewSyncHandler binds extension sync transport to canonical capture owners.
func NewSyncHandler(capture *Capture) *SyncHandler {
	return &SyncHandler{capture: capture}
}

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
	ExtensionLogs []types.ExtensionLog `json:"extension_logs,omitempty"`

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
func (h *SyncHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
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

	state := h.capture.extension.updateSyncConnectionState(req, clientID, now)

	if !state.wasConnected || state.isReconnect {
		util.SafeGo(func() {
			h.capture.Lifecycle().Emit(lifecycle.EventExtensionConnected, map[string]any{
				"ext_session_id":     state.extSessionID,
				"is_reconnect":       state.isReconnect,
				"disconnect_seconds": state.timeSinceLastPoll.Seconds(),
			})
		})
	}

	// Forward extension feature usage to the usage counter via callback.
	// Only known UI-originated keys are forwarded to prevent unbounded counter cardinality.
	if filtered := filterFeaturesUsed(req.FeaturesUsed); len(filtered) > 0 {
		h.capture.FeatureUsage().Notify(filtered)
	}

	h.processSyncCommandResults(req.CommandResults, clientID)
	if req.LastCommandAck != "" {
		h.capture.Queries().AcknowledgePendingQuery(req.LastCommandAck)
	}

	if state.wasDisconnected {
		telemetry.AppError("extension_disconnect")
		h.capture.queryDispatcher.ExpireAllPendingQueries("extension_disconnected")
		util.SafeGo(func() {
			h.capture.Lifecycle().Emit(lifecycle.EventExtensionDisconnected, map[string]any{
				"ext_session_id": state.extSessionID,
				"client_id":      clientID,
			})
		})
	}

	// Reconcile started commands against extension heartbeat state.
	// If a command disappears from heartbeat in_progress without a terminal result,
	// fail it fast instead of waiting for eventual timeout.
	h.reconcileInProgressCommandState(req.InProgress, now)

	pendingQueries := h.capture.Queries().GetPendingQueries()
	if len(pendingQueries) == 0 {
		h.capture.Queries().WaitForPendingQueries(syncLongPollTimeout())
		pendingQueries = h.capture.Queries().GetPendingQueries()
	}

	h.updateSyncLogs(req, now, state.pilotEnabled, len(pendingQueries))

	commands := buildSyncCommands(pendingQueries)

	nextPollMs := 1000
	if len(commands) > 0 {
		nextPollMs = 200
	}
	if shouldEmitSyncSnapshot(req, state, len(commands)) {
		util.SafeGo(func() {
			h.capture.Lifecycle().Emit(lifecycle.EventSyncSnapshot, map[string]any{
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
		ServerVersion:    h.capture.Extension().ServerVersion(),
		InstallID:        telemetry.GetInstallID(),
		CaptureOverrides: h.buildCaptureOverrides(),
	}

	util.JSONResponse(w, http.StatusOK, resp)
}

// buildSyncCommands converts pending queries to sync commands.
func buildSyncCommands(pending []queries.PendingQueryResponse) []SyncCommand {
	commands := make([]SyncCommand, len(pending))
	for i, q := range pending {
		commands[i] = SyncCommand{
			ID:            q.ID,
			Type:          q.Type,
			Params:        q.Params,
			TabID:         q.TabID,
			CorrelationID: q.CorrelationID,
			TraceID:       q.TraceID,
		}
	}
	return commands
}

// shouldEmitSyncSnapshot determines whether lifecycle telemetry should include a sync snapshot.
func shouldEmitSyncSnapshot(req SyncRequest, state syncConnectionState, commandsOut int) bool {
	if state.isReconnect || state.wasDisconnected || !state.wasConnected {
		return true
	}
	if len(req.CommandResults) > 0 || commandsOut > 0 {
		return true
	}
	if req.LastCommandAck != "" {
		return true
	}
	return false
}

func (h *SyncHandler) buildCaptureOverrides() map[string]string {
	mode, productionParity, rewrites := h.capture.extension.GetSecurityMode()
	if mode == SecurityModeNormal {
		return map[string]string{}
	}

	overrides := map[string]string{
		"security_mode":     mode,
		"production_parity": "false",
	}
	if productionParity {
		overrides["production_parity"] = "true"
	}
	if len(rewrites) > 0 {
		overrides["insecure_rewrites_applied"] = strings.Join(rewrites, ",")
	}
	return overrides
}

const (
	syncLongPollDefaultTimeout = 5 * time.Second
	syncLongPollTestTimeout    = 100 * time.Millisecond
)

func syncLongPollTimeout() time.Duration {
	if strings.HasSuffix(os.Args[0], ".test") {
		return syncLongPollTestTimeout
	}
	return syncLongPollDefaultTimeout
}

type syncConnectionState struct {
	wasConnected      bool
	isReconnect       bool
	wasDisconnected   bool
	timeSinceLastPoll time.Duration
	extSessionID      string
	pilotEnabled      bool
	inProgressCount   int
}

func (r *ExtensionRuntime) updateSyncConnectionState(req SyncRequest, clientID string, now time.Time) syncConnectionState {
	r.mu.Lock()
	defer r.mu.Unlock()

	state := syncConnectionState{
		wasConnected:      r.state.lastExtensionConnected,
		timeSinceLastPoll: now.Sub(r.state.lastPollAt),
	}
	state.wasDisconnected = !r.state.lastSyncSeen.IsZero() && now.Sub(r.state.lastSyncSeen) >= extensionDisconnectThreshold
	state.isReconnect = state.wasDisconnected

	r.state.lastPollAt = now
	r.state.lastExtensionConnected = true
	r.state.lastSyncSeen = now
	r.state.lastSyncClientID = clientID
	if req.ExtSessionID != "" && req.ExtSessionID != r.state.extSessionID {
		r.state.extSessionID = req.ExtSessionID
		r.state.extSessionChangedAt = now
	}
	state.extSessionID = r.state.extSessionID

	if req.Settings != nil {
		r.state.pilotEnabled = req.Settings.PilotEnabled
		r.state.pilotStatusKnown = true
		r.state.pilotUpdatedAt = now
		r.state.pilotSource = PilotSourceExtensionSync
		r.state.trackingEnabled = req.Settings.TrackingEnabled
		r.state.trackedTabID = req.Settings.TrackedTabID
		r.state.trackedTabURL = req.Settings.TrackedTabURL
		r.state.trackedTabTitle = req.Settings.TrackedTabTitle
		r.state.trackingUpdated = now
		switch req.Settings.TabStatus {
		case "loading", "complete":
			r.state.tabStatus = req.Settings.TabStatus
		default:
			r.state.tabStatus = ""
		}
		r.state.trackedTabActive = req.Settings.TrackedTabActive
		r.state.cspRestricted = req.Settings.CspRestricted
		r.state.cspLevel = req.Settings.CspLevel
	}
	if req.InProgress != nil {
		r.state.inProgress = normalizeInProgressList(req.InProgress)
		r.state.inProgressUpdated = now
	}
	state.pilotEnabled = r.state.pilotEnabled
	state.inProgressCount = len(r.state.inProgress)
	return state
}

func (h *SyncHandler) processSyncCommandResults(results []SyncCommandResult, clientID string) {
	for _, result := range results {
		if result.ID != "" {
			if result.CorrelationID != "" {
				h.capture.Queries().SetQueryResultWithClientNoCommandComplete(result.ID, result.Result, clientID)
			} else {
				h.capture.Queries().SetQueryResultWithClient(result.ID, result.Result, clientID)
			}
		}
		if result.CorrelationID != "" {
			h.capture.Queries().ApplyCommandResult(result.CorrelationID, result.Status, result.Result, result.Error)
		}
	}
}

func (h *SyncHandler) updateSyncLogs(req SyncRequest, now time.Time, pilotEnabled bool, queryCount int) {
	h.capture.DiagnosticLogs().AddPolling(types.PollingLogEntry{
		Timestamp:    now,
		Endpoint:     "sync",
		Method:       "POST",
		ExtSessionID: req.ExtSessionID,
		PilotEnabled: &pilotEnabled,
		QueryCount:   queryCount,
	})
	h.capture.extensionLogs.addAt(req.ExtensionLogs, now)
	if req.ExtensionVersion != "" {
		h.capture.extension.SetExtensionVersion(req.ExtensionVersion)
	}
}

// GetPendingQueriesDisconnectAware reconciles extension liveness before
// returning the current query queue for sync delivery.
func (h *SyncHandler) GetPendingQueriesDisconnectAware() []queries.PendingQueryResponse {
	_, disconnected := h.capture.extension.Disconnected()

	if disconnected {
		h.capture.queryDispatcher.ExpireAllPendingQueries("extension_disconnected")
		return nil
	}
	return h.capture.Queries().GetPendingQueries()
}

func normalizeInProgressList(in []SyncInProgress) []SyncInProgress {
	if in == nil {
		return nil
	}
	if len(in) == 0 {
		return []SyncInProgress{}
	}
	const maxInProgress = 100
	limit := len(in)
	if limit > maxInProgress {
		limit = maxInProgress
	}
	out := make([]SyncInProgress, 0, limit)
	for i := 0; i < limit; i++ {
		entry := in[i]
		entry.ID = strings.TrimSpace(entry.ID)
		entry.CorrelationID = strings.TrimSpace(entry.CorrelationID)
		entry.Type = strings.TrimSpace(entry.Type)
		entry.Status = strings.TrimSpace(strings.ToLower(entry.Status))
		if entry.Status == "" {
			entry.Status = "running"
		}
		if entry.ProgressPct != nil {
			progress := *entry.ProgressPct
			if progress < 0 {
				progress = 0
			}
			if progress > 100 {
				progress = 100
			}
			entry.ProgressPct = &progress
		}
		if entry.ID == "" && entry.CorrelationID == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func commandHasStarted(command *queries.CommandResult) bool {
	if command == nil {
		return false
	}
	for _, event := range command.TraceEvents {
		if event.Stage == "started" || event.Stage == "resolved" || event.Stage == "errored" || event.Stage == "timed_out" {
			return true
		}
	}
	return strings.Contains(command.TraceTimeline, "started")
}

const missingInProgressGrace = 2 * time.Second

func (h *SyncHandler) reconcileInProgressCommandState(inProgress []SyncInProgress, now time.Time) {
	if inProgress == nil {
		return
	}

	active := make(map[string]struct{}, len(inProgress))
	for _, entry := range inProgress {
		if entry.CorrelationID != "" {
			active[entry.CorrelationID] = struct{}{}
		}
	}
	pending := h.capture.Queries().GetPendingCommands()
	toFail, toFailIDs := h.capture.extension.reconcileMissingCommands(pending, active, now)

	for i, correlationID := range toFail {
		queryID := ""
		if i < len(toFailIDs) {
			queryID = toFailIDs[i]
		}
		h.capture.Queries().ApplyCommandResult(
			correlationID,
			"error",
			nil,
			"extension_lost_command: command acknowledged by extension but missing from in_progress heartbeats",
		)
		util.SafeGo(func() {
			h.capture.Lifecycle().Emit(lifecycle.EventCommandStateDesync, map[string]any{
				"correlation_id": correlationID,
				"query_id":       queryID,
				"reason":         "missing_in_progress_heartbeat",
			})
		})
	}
}

func (r *ExtensionRuntime) reconcileMissingCommands(
	pending []*queries.CommandResult,
	active map[string]struct{},
	now time.Time,
) (toFail []string, toFailIDs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pendingCorrelations := make(map[string]struct{}, len(pending))
	for _, command := range pending {
		if command == nil || command.CorrelationID == "" {
			continue
		}
		correlationID := command.CorrelationID
		pendingCorrelations[correlationID] = struct{}{}
		if _, ok := active[correlationID]; ok {
			delete(r.state.missingInProgressSince, correlationID)
			continue
		}
		if !commandHasStarted(command) {
			continue
		}
		missingSince, observedMissing := r.state.missingInProgressSince[correlationID]
		if !observedMissing {
			r.state.missingInProgressSince[correlationID] = now
			continue
		}
		if now.Sub(missingSince) >= missingInProgressGrace {
			toFail = append(toFail, correlationID)
			toFailIDs = append(toFailIDs, command.QueryID)
			delete(r.state.missingInProgressSince, correlationID)
		}
	}
	for correlationID := range r.state.missingInProgressSince {
		if _, stillPending := pendingCorrelations[correlationID]; !stillPending {
			delete(r.state.missingInProgressSince, correlationID)
		}
	}
	return toFail, toFailIDs
}

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

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// SyncHandler owns the extension heartbeat, command transport, and
// disconnect-reconciliation boundary.
type SyncHandler struct {
	capture               *Capture
	waitForPendingQueries func(time.Duration)
}

// NewSyncHandler binds extension sync transport to canonical capture owners.
func NewSyncHandler(capture *Capture) *SyncHandler {
	return &SyncHandler{
		capture:               capture,
		waitForPendingQueries: capture.Queries().WaitForPendingQueries,
	}
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
// Handler
// =============================================================================

// HandleSync processes extension heartbeats and command transport in one endpoint.
//
// Invariants:
// - Connection state is updated before command/result reconciliation.
func enabledFeatures(raw *SyncFeaturesUsed) map[string]bool {
	if raw == nil {
		return nil
	}
	enabled := make(map[string]bool, 5)
	if raw.Screenshot {
		enabled["screenshot"] = true
	}
	if raw.Annotations {
		enabled["annotations"] = true
	}
	if raw.Video {
		enabled["video"] = true
	}
	if raw.DOMAction {
		enabled["dom_action"] = true
	}
	if raw.ActionRecording {
		enabled["action_recording"] = true
	}
	if len(enabled) == 0 {
		return nil
	}
	return enabled
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
	if state.staleGeneration {
		h.rejectStaleGeneration(w, req, state.connectionGeneration, "extension_sync")
		return
	}

	if !state.wasConnected || state.isReconnect {
		util.SafeGo(func() {
			h.capture.Lifecycle().Emit(lifecycle.EventExtensionConnected, map[string]any{
				"ext_session_id":     state.extSessionID,
				"is_reconnect":       state.isReconnect,
				"disconnect_seconds": state.timeSinceLastPoll.Seconds(),
			})
		})
	}

	if currentGeneration, applied := h.capture.extension.applyIfCurrentSyncGeneration(
		req.ExtSessionID,
		state.connectionGeneration,
		func() {
			// Only known UI-originated keys are forwarded to prevent unbounded cardinality.
			if filtered := enabledFeatures(req.FeaturesUsed); len(filtered) > 0 {
				h.capture.FeatureUsage().Notify(filtered)
			}
			h.processSyncCommandResults(req.CommandResults, clientID, state.connectionGeneration)
			if req.LastCommandAck != "" {
				h.capture.Queries().AcknowledgePendingQuery(req.LastCommandAck)
			}
		},
	); !applied {
		h.rejectStaleGeneration(w, req, currentGeneration, "extension_sync_apply")
		return
	}

	if state.wasDisconnected {
		telemetry.AppError(incident.CodeExtensionDisconnect)
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
	currentInProgress := currentGenerationInProgress(req.InProgress, state.connectionGeneration)
	for _, entry := range req.InProgress {
		if entry.ConnectionGeneration != 0 && entry.ConnectionGeneration != state.connectionGeneration {
			h.capture.Lifecycle().Emit(lifecycle.EventStaleGenerationRejected, map[string]any{
				"correlation_id":      entry.CorrelationID,
				"received_generation": entry.ConnectionGeneration,
				"current_generation":  state.connectionGeneration,
				"bridge":              "sync_command_progress",
			})
		}
	}
	h.reconcileInProgressCommandState(currentInProgress, now)

	pendingQueries := h.capture.Queries().GetPendingQueries()
	if len(pendingQueries) == 0 {
		h.waitForPendingQueries(syncLongPollTimeout())
		pendingQueries = h.capture.Queries().GetPendingQueries()
	}
	if currentGeneration, current := h.capture.extension.isCurrentSyncGeneration(
		req.ExtSessionID,
		state.connectionGeneration,
	); !current {
		h.rejectStaleGeneration(w, req, currentGeneration, "extension_sync_response")
		return
	}

	h.updateSyncLogs(req, now, state.pilotEnabled, len(pendingQueries))

	commands := buildSyncCommands(pendingQueries, state.connectionGeneration)

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
		Ack:                  true,
		ConnectionGeneration: state.connectionGeneration,
		Commands:             commands,
		NextPollMs:           nextPollMs,
		ServerTime:           now.Format(time.RFC3339),
		ServerVersion:        h.capture.Extension().ServerVersion(),
		InstallID:            telemetry.GetInstallID(),
		CaptureOverrides:     h.buildCaptureOverrides(),
	}

	util.JSONResponse(w, http.StatusOK, resp)
}

func (h *SyncHandler) rejectStaleGeneration(w http.ResponseWriter, req SyncRequest, currentGeneration uint64, bridge string) {
	h.capture.Lifecycle().Emit(lifecycle.EventStaleGenerationRejected, map[string]any{
		"correlation_id":      req.ExtSessionID,
		"received_generation": req.ConnectionGeneration,
		"current_generation":  currentGeneration,
		"bridge":              bridge,
	})
	util.JSONResponse(w, http.StatusConflict, map[string]any{
		"ack":                   false,
		"error":                 "stale_connection_generation",
		"connection_generation": currentGeneration,
	})
}

// buildSyncCommands converts pending queries to sync commands.
func buildSyncCommands(pending []queries.PendingQueryResponse, generation uint64) []SyncCommand {
	commands := make([]SyncCommand, len(pending))
	for i, q := range pending {
		commands[i] = SyncCommand{
			ID:                   q.ID,
			Type:                 q.Type,
			Params:               q.Params,
			TabID:                q.TabID,
			CorrelationID:        q.CorrelationID,
			TraceID:              q.TraceID,
			ConnectionGeneration: generation,
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
	wasConnected         bool
	isReconnect          bool
	wasDisconnected      bool
	timeSinceLastPoll    time.Duration
	extSessionID         string
	pilotEnabled         bool
	inProgressCount      int
	connectionGeneration uint64
	staleGeneration      bool
}

func (r *ExtensionRuntime) updateSyncConnectionState(req SyncRequest, clientID string, now time.Time) syncConnectionState {
	r.mu.Lock()
	defer r.mu.Unlock()
	wasActuallyConnected := extensionStateConnected(r.state, now)

	state := syncConnectionState{
		wasConnected:      r.state.lastExtensionConnected,
		timeSinceLastPoll: now.Sub(r.state.lastPollAt),
	}
	if r.state.connectionGeneration == 0 {
		r.state.connectionGeneration = 1
	}
	if r.state.extSessionID != "" && req.ConnectionGeneration != 0 && req.ConnectionGeneration != r.state.connectionGeneration {
		state.connectionGeneration = r.state.connectionGeneration
		state.staleGeneration = true
		return state
	}
	if req.ExtSessionID != "" && req.ExtSessionID != r.state.extSessionID {
		if r.state.extSessionID != "" {
			r.state.connectionGeneration++
		}
	}
	state.connectionGeneration = r.state.connectionGeneration
	state.wasDisconnected = !r.state.lastSyncSeen.IsZero() && !extensionStateConnected(r.state, now)
	state.isReconnect = state.wasDisconnected

	r.state.lastPollAt = now
	r.state.lastExtensionConnected = true
	r.state.lastSyncSeen = now
	r.state.lastSyncClientID = clientID
	if !wasActuallyConnected {
		r.signalConnectionChangeLocked()
	}
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
		r.state.inProgress = normalizeInProgressList(currentGenerationInProgress(req.InProgress, state.connectionGeneration))
		r.state.inProgressUpdated = now
	}
	state.pilotEnabled = r.state.pilotEnabled
	state.inProgressCount = len(r.state.inProgress)
	return state
}

func (r *ExtensionRuntime) isCurrentSyncGeneration(sessionID string, generation uint64) (uint64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.connectionGeneration,
		sessionID != "" && sessionID == r.state.extSessionID && generation == r.state.connectionGeneration
}

func (r *ExtensionRuntime) applyIfCurrentSyncGeneration(
	sessionID string,
	generation uint64,
	apply func(),
) (uint64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if sessionID == "" || sessionID != r.state.extSessionID || generation != r.state.connectionGeneration {
		return r.state.connectionGeneration, false
	}
	apply()
	return r.state.connectionGeneration, true
}

func (h *SyncHandler) processSyncCommandResults(results []SyncCommandResult, clientID string, currentGeneration uint64) {
	for _, result := range results {
		// EXPECTED_ABSENCE: a result without its own generation inherits the
		// already-validated enclosing heartbeat generation; logging that normal
		// compact form would create a false stale-work incident.
		if result.ConnectionGeneration != 0 && result.ConnectionGeneration != currentGeneration {
			h.capture.Lifecycle().Emit(lifecycle.EventStaleGenerationRejected, map[string]any{
				"correlation_id":      result.CorrelationID,
				"received_generation": result.ConnectionGeneration,
				"current_generation":  currentGeneration,
				"bridge":              "sync_command_result",
			})
			continue
		}
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

func currentGenerationInProgress(in []SyncInProgress, generation uint64) []SyncInProgress {
	if in == nil {
		return nil
	}
	out := make([]SyncInProgress, 0, len(in))
	for _, entry := range in {
		// EXPECTED_ABSENCE: progress without its own generation inherits the
		// already-validated enclosing heartbeat generation; logging this compact
		// form would create a false stale-work incident.
		if entry.ConnectionGeneration == 0 || entry.ConnectionGeneration == generation {
			out = append(out, entry)
		}
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

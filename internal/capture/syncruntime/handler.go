// Purpose: Implements the /sync transport flow — settings, logs, command results, pending command delivery and long-poll policy.
// Why: Consolidates extension-daemon synchronization into a single resilient protocol surface.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/query-service/index.md

package syncruntime

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/featureusage"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/extclient"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Dependencies declares the independently synchronized owners used by sync.
type Dependencies struct {
	Runtime        *Runtime
	Queries        *queries.QueryDispatcher
	Lifecycle      *lifecycle.Observer
	FeatureUsage   *featureusage.Observer
	ExtensionLogs  *logstore.Extension
	DiagnosticLogs *logstore.Diagnostic
	// Recorder is optional: when set, the command exchange is captured for
	// offline replay. Nil in production.
	Recorder ExchangeRecorder
}

// Handler owns the extension heartbeat, command transport, and
// disconnect-reconciliation boundary.
type Handler struct {
	runtime               *Runtime
	queries               *queries.QueryDispatcher
	lifecycle             *lifecycle.Observer
	featureUsage          *featureusage.Observer
	extensionLogs         *logstore.Extension
	diagnosticLogs        *logstore.Diagnostic
	waitForPendingQueries func(time.Duration)
	recorder              ExchangeRecorder
}

const maxSyncPostBody = 5 << 20

// NewHandler binds extension sync transport to its canonical owners.
func NewHandler(deps Dependencies) *Handler {
	return &Handler{
		runtime:               deps.Runtime,
		queries:               deps.Queries,
		lifecycle:             deps.Lifecycle,
		featureUsage:          deps.FeatureUsage,
		extensionLogs:         deps.ExtensionLogs,
		diagnosticLogs:        deps.DiagnosticLogs,
		waitForPendingQueries: deps.Queries.WaitForPendingQueries,
		recorder:              deps.Recorder,
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
func (h *Handler) HandleSync(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeSyncRequest(w, r)
	if !ok {
		return
	}

	now := time.Now()
	clientID := r.Header.Get("X-Kaboom-Client")

	// A probe validates this endpoint's contract; it is not a browser. Adopting it
	// would bump the connection generation — superseding the real extension's
	// in-flight poll — let its partial settings erase the tracked tab the extension
	// reported, and drain commands the extension will never run.
	if extclient.IsProbe(clientID) {
		h.respondProbeSync(w, now)
		return
	}

	state := h.runtime.updateSyncConnectionState(req, clientID, now)
	if state.staleGeneration {
		h.rejectStaleGeneration(w, req, state.connectionGeneration, "extension_sync")
		return
	}

	h.emitExtensionConnected(state)
	if currentGeneration, applied := h.runtime.applyIfCurrentSyncGeneration(
		req.ExtSessionID,
		state.connectionGeneration,
		func() { h.reconcileSyncExchange(req, clientID, state.connectionGeneration) },
	); !applied {
		h.rejectStaleGeneration(w, req, currentGeneration, "extension_sync_apply")
		return
	}

	h.handleSyncDisconnect(state, clientID)
	h.reconcileSyncProgress(req, state, now)

	pendingQueries := h.awaitPendingQueries()
	if currentGeneration, current := h.runtime.isCurrentSyncGeneration(
		req.ExtSessionID,
		state.connectionGeneration,
	); !current {
		h.rejectStaleGeneration(w, req, currentGeneration, "extension_sync_response")
		return
	}

	h.updateSyncLogs(req, now, state.pilotEnabled, len(pendingQueries))

	commands := buildSyncCommands(pendingQueries, state.connectionGeneration)
	h.recordIssued(commands)
	nextPollMs := syncNextPollMs(len(commands))
	h.emitSyncSnapshotEvent(req, state, len(commands), nextPollMs, clientID)

	resp := SyncResponse{
		Ack:                  true,
		ConnectionGeneration: state.connectionGeneration,
		Commands:             commands,
		NextPollMs:           nextPollMs,
		ServerTime:           now.Format(time.RFC3339),
		ServerVersion:        h.runtime.ServerVersion(),
		InstallID:            telemetry.GetInstallID(),
		CaptureOverrides:     h.buildCaptureOverrides(),
	}

	util.JSONResponse(w, http.StatusOK, resp)
}

func (h *Handler) decodeSyncRequest(w http.ResponseWriter, r *http.Request) (SyncRequest, bool) {
	if !util.RequireMethod(w, r, "POST") {
		return SyncRequest{}, false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSyncPostBody)
	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return SyncRequest{}, false
	}
	return req, true
}

func (h *Handler) emitExtensionConnected(state syncConnectionState) {
	if !state.wasConnected || state.isReconnect {
		util.SafeGo(func() {
			h.lifecycle.Emit(lifecycle.EventExtensionConnected, map[string]any{
				"ext_session_id":     state.extSessionID,
				"is_reconnect":       state.isReconnect,
				"disconnect_seconds": state.timeSinceLastPoll.Seconds(),
			})
		})
	}
}

func (h *Handler) reconcileSyncExchange(req SyncRequest, clientID string, connectionGeneration uint64) {
	// Only known UI-originated keys are forwarded to prevent unbounded cardinality.
	if filtered := enabledFeatures(req.FeaturesUsed); len(filtered) > 0 {
		h.featureUsage.Notify(filtered)
	}
	h.recordCompleted(req.CommandResults)
	h.processSyncCommandResults(req.CommandResults, clientID, connectionGeneration)
	if req.LastCommandAck != "" {
		h.queries.AcknowledgePendingQuery(req.LastCommandAck)
	}
}

func (h *Handler) handleSyncDisconnect(state syncConnectionState, clientID string) {
	if !state.wasDisconnected {
		return
	}
	telemetry.AppError(incident.CodeExtensionDisconnect)
	h.queries.ExpireAllPendingQueries("extension_disconnected")
	util.SafeGo(func() {
		h.lifecycle.Emit(lifecycle.EventExtensionDisconnected, map[string]any{
			"ext_session_id": state.extSessionID,
			"client_id":      clientID,
		})
	})
}

// Reconcile started commands against extension heartbeat state.
// If a command disappears from heartbeat in_progress without a terminal result,
// fail it fast instead of waiting for eventual timeout.
func (h *Handler) reconcileSyncProgress(req SyncRequest, state syncConnectionState, now time.Time) {
	currentInProgress := currentGenerationInProgress(req.InProgress, state.connectionGeneration)
	for _, entry := range req.InProgress {
		if entry.ConnectionGeneration != 0 && entry.ConnectionGeneration != state.connectionGeneration {
			h.lifecycle.Emit(lifecycle.EventStaleGenerationRejected, map[string]any{
				"correlation_id":      entry.CorrelationID,
				"received_generation": entry.ConnectionGeneration,
				"current_generation":  state.connectionGeneration,
				"bridge":              "sync_command_progress",
			})
		}
	}
	h.reconcileInProgressCommandState(currentInProgress, now)
}

func (h *Handler) awaitPendingQueries() []queries.PendingQueryResponse {
	pendingQueries := h.queries.GetPendingQueries()
	if len(pendingQueries) == 0 {
		h.waitForPendingQueries(syncLongPollTimeout())
		pendingQueries = h.queries.GetPendingQueries()
	}
	return pendingQueries
}

func syncNextPollMs(commandsOut int) int {
	if commandsOut > 0 {
		return 200
	}
	return 1000
}

func (h *Handler) emitSyncSnapshotEvent(req SyncRequest, state syncConnectionState, commandsOut, nextPollMs int, clientID string) {
	if !shouldEmitSyncSnapshot(req, state, commandsOut) {
		return
	}
	util.SafeGo(func() {
		h.lifecycle.Emit(lifecycle.EventSyncSnapshot, map[string]any{
			"ext_session_id":       state.extSessionID,
			"client_id":            clientID,
			"pilot_enabled":        state.pilotEnabled,
			"in_progress_count":    state.inProgressCount,
			"pending_commands_out": commandsOut,
			"command_results_in":   len(req.CommandResults),
			"last_command_ack":     req.LastCommandAck,
			"next_poll_ms":         nextPollMs,
		})
	})
}

// respondProbeSync answers a contract probe with a well-formed, empty envelope.
// It deliberately adopts nothing: no session, no generation, no settings, no
// commands, and no evidence that an extension is connected.
func (h *Handler) respondProbeSync(w http.ResponseWriter, now time.Time) {
	util.JSONResponse(w, http.StatusOK, SyncResponse{
		Ack:              true,
		Commands:         []SyncCommand{},
		NextPollMs:       1000,
		ServerTime:       now.Format(time.RFC3339),
		ServerVersion:    h.runtime.ServerVersion(),
		CaptureOverrides: map[string]string{},
	})
}

func (h *Handler) rejectStaleGeneration(w http.ResponseWriter, req SyncRequest, currentGeneration uint64, bridge string) {
	h.lifecycle.Emit(lifecycle.EventStaleGenerationRejected, map[string]any{
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

// ExchangeRecorder observes the two halves of a command exchange, so one live
// run can be recorded and replayed by a fake extension. That is what lets the
// connected UAT categories — the only tests that prove a browser feature still
// works — run without Chrome, an extension and a person.
//
// The interface lives here, with the code that has the data; the implementation
// lives in internal/synctranscript and is wired at composition, which keeps this
// package free of any dependency on the replay machinery. A nil recorder is the
// normal production state and every hook is then a no-op.
type ExchangeRecorder interface {
	// Issued notes a command handed to the extension.
	Issued(id, kind string, params json.RawMessage)
	// Completed notes the terminal outcome the extension returned for it.
	Completed(id, status string, result json.RawMessage, failure string)
}

// recordIssued reports every command in a delivered batch.
func (h *Handler) recordIssued(commands []SyncCommand) {
	if h.recorder == nil {
		// EXPECTED_ABSENCE: recording is opt-in and off in production, so the
		// nil case is the overwhelmingly common one, not a fault.
		return
	}
	for _, command := range commands {
		h.recorder.Issued(command.ID, command.Type, command.Params)
	}
}

// recordCompleted reports every terminal outcome in an incoming batch.
func (h *Handler) recordCompleted(results []SyncCommandResult) {
	if h.recorder == nil {
		// EXPECTED_ABSENCE: as above — no recorder configured.
		return
	}
	for _, result := range results {
		h.recorder.Completed(result.ID, result.Status, result.Result, result.Error)
	}
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

func (h *Handler) buildCaptureOverrides() map[string]string {
	mode, productionParity, rewrites := h.runtime.GetSecurityMode()
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

func (r *Runtime) updateSyncConnectionState(req SyncRequest, clientID string, now time.Time) syncConnectionState {
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
	if r.syncGenerationStale(req) {
		state.connectionGeneration = r.state.connectionGeneration
		state.staleGeneration = true
		return state
	}
	r.bumpSyncGenerationForNewSession(req)
	state.connectionGeneration = r.state.connectionGeneration
	state.wasDisconnected = !r.state.lastSyncSeen.IsZero() && !extensionStateConnected(r.state, now)
	state.isReconnect = state.wasDisconnected

	r.markSyncSeen(clientID, now, wasActuallyConnected)
	state.extSessionID = r.adoptExtSession(req, now)

	if req.Settings != nil {
		r.applySyncSettingsLocked(req.Settings, now)
	}
	if req.InProgress != nil {
		r.state.inProgress = normalizeInProgressList(currentGenerationInProgress(req.InProgress, state.connectionGeneration))
		r.state.inProgressUpdated = now
	}
	state.pilotEnabled = r.state.pilotEnabled
	state.inProgressCount = len(r.state.inProgress)
	return state
}

func (r *Runtime) syncGenerationStale(req SyncRequest) bool {
	return r.state.extSessionID != "" && req.ConnectionGeneration != 0 &&
		req.ConnectionGeneration != r.state.connectionGeneration
}

func (r *Runtime) bumpSyncGenerationForNewSession(req SyncRequest) {
	if req.ExtSessionID != "" && req.ExtSessionID != r.state.extSessionID {
		if r.state.extSessionID != "" {
			r.state.connectionGeneration++
		}
	}
}

func (r *Runtime) markSyncSeen(clientID string, now time.Time, wasActuallyConnected bool) {
	r.state.lastPollAt = now
	r.state.lastExtensionConnected = true
	r.state.lastSyncSeen = now
	r.state.lastSyncClientID = clientID
	if !wasActuallyConnected {
		r.signalConnectionChangeLocked()
	}
}

func (r *Runtime) adoptExtSession(req SyncRequest, now time.Time) string {
	if req.ExtSessionID != "" && req.ExtSessionID != r.state.extSessionID {
		r.state.extSessionID = req.ExtSessionID
		r.state.extSessionChangedAt = now
	}
	return r.state.extSessionID
}

func (r *Runtime) applySyncSettingsLocked(settings *SyncSettings, now time.Time) {
	trackingChanged := r.trackingChangedLocked(settings)
	r.state.pilotEnabled = settings.PilotEnabled
	r.state.pilotStatusKnown = true
	r.state.pilotUpdatedAt = now
	r.state.pilotSource = PilotSourceExtensionSync
	r.state.trackingEnabled = settings.TrackingEnabled
	r.state.trackedTabID = settings.TrackedTabID
	r.state.trackedTabURL = settings.TrackedTabURL
	r.state.trackedTabTitle = settings.TrackedTabTitle
	r.state.trackingUpdated = now
	r.state.tabStatus = normalizedTabStatus(settings.TabStatus)
	r.state.trackedTabActive = settings.TrackedTabActive
	r.state.cspRestricted = settings.CspRestricted
	r.state.cspLevel = settings.CspLevel
	if trackingChanged {
		r.signalTrackingChangeLocked()
	}
}

func (r *Runtime) trackingChangedLocked(settings *SyncSettings) bool {
	return r.state.trackingEnabled != settings.TrackingEnabled ||
		r.state.trackedTabID != settings.TrackedTabID ||
		r.state.trackedTabURL != settings.TrackedTabURL ||
		r.state.trackedTabTitle != settings.TrackedTabTitle ||
		r.state.tabStatus != normalizedTabStatus(settings.TabStatus) ||
		!optionalBoolEqual(r.state.trackedTabActive, settings.TrackedTabActive)
}

func normalizedTabStatus(status string) string {
	switch status {
	case "loading", "complete":
		return status
	default:
		return ""
	}
}

func optionalBoolEqual(left, right *bool) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (r *Runtime) isCurrentSyncGeneration(sessionID string, generation uint64) (uint64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.connectionGeneration,
		sessionID != "" && sessionID == r.state.extSessionID && generation == r.state.connectionGeneration
}

func (r *Runtime) applyIfCurrentSyncGeneration(
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

func (h *Handler) processSyncCommandResults(results []SyncCommandResult, clientID string, currentGeneration uint64) {
	for _, result := range results {
		// EXPECTED_ABSENCE: a result without its own generation inherits the
		// already-validated enclosing heartbeat generation; logging that normal
		// compact form would create a false stale-work incident.
		if result.ConnectionGeneration != 0 && result.ConnectionGeneration != currentGeneration {
			h.lifecycle.Emit(lifecycle.EventStaleGenerationRejected, map[string]any{
				"correlation_id":      result.CorrelationID,
				"received_generation": result.ConnectionGeneration,
				"current_generation":  currentGeneration,
				"bridge":              "sync_command_result",
			})
			continue
		}
		if result.ID != "" {
			if result.CorrelationID != "" {
				h.queries.SetQueryResultWithClientNoCommandComplete(result.ID, result.Result, clientID)
			} else {
				h.queries.SetQueryResultWithClient(result.ID, result.Result, clientID)
			}
		}
		if result.CorrelationID != "" {
			h.queries.ApplyCommandResult(result.CorrelationID, result.Status, result.Result, result.Error)
		}
	}
}

func (h *Handler) updateSyncLogs(req SyncRequest, now time.Time, pilotEnabled bool, queryCount int) {
	h.diagnosticLogs.AddPolling(types.PollingLogEntry{
		Timestamp:    now,
		Endpoint:     "sync",
		Method:       "POST",
		ExtSessionID: req.ExtSessionID,
		PilotEnabled: &pilotEnabled,
		QueryCount:   queryCount,
	})
	h.extensionLogs.AddAt(req.ExtensionLogs, now)
	if req.ExtensionVersion != "" {
		h.runtime.SetExtensionVersion(req.ExtensionVersion)
	}
	h.runtime.setCommandContractID(req.CommandContractID)
}

// GetPendingQueriesDisconnectAware reconciles extension liveness before
// returning the current query queue for sync delivery.
func (h *Handler) GetPendingQueriesDisconnectAware() []queries.PendingQueryResponse {
	_, disconnected := h.runtime.Disconnected()

	if disconnected {
		h.queries.ExpireAllPendingQueries("extension_disconnected")
		return nil
	}
	return h.queries.GetPendingQueries()
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

func (h *Handler) reconcileInProgressCommandState(inProgress []SyncInProgress, now time.Time) {
	if inProgress == nil {
		return
	}

	active := make(map[string]struct{}, len(inProgress))
	for _, entry := range inProgress {
		if entry.CorrelationID != "" {
			active[entry.CorrelationID] = struct{}{}
		}
	}
	pending := h.queries.GetPendingCommands()
	toFail, toFailIDs := h.runtime.reconcileMissingCommands(pending, active, now)

	for i, correlationID := range toFail {
		queryID := ""
		if i < len(toFailIDs) {
			queryID = toFailIDs[i]
		}
		h.queries.ApplyCommandResult(
			correlationID,
			"error",
			nil,
			"extension_lost_command: command acknowledged by extension but missing from in_progress heartbeats",
		)
		util.SafeGo(func() {
			h.lifecycle.Emit(lifecycle.EventCommandStateDesync, map[string]any{
				"correlation_id": correlationID,
				"query_id":       queryID,
				"reason":         "missing_in_progress_heartbeat",
			})
		})
	}
}

func (r *Runtime) reconcileMissingCommands(
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

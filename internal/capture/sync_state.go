// Purpose: Applies /sync side effects to capture state — heartbeat transitions, payload ingestion, command reconciliation.
// Why: Keeps lock-scoped state mutation isolated from the HTTP transport flow that drives it.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/query-service/index.md

package capture

import (
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// syncConnectionState is an immutable lock-scope snapshot used after releasing c.mu.
//
// Invariants:
// - Values are derived from one atomic read/update cycle in updateSyncConnectionState.
// - Safe for use in async callbacks because it does not reference mutable capture internals.
type syncConnectionState struct {
	wasConnected      bool
	isReconnect       bool
	wasDisconnected   bool
	timeSinceLastPoll time.Duration
	extSessionID      string
	pilotEnabled      bool
	inProgressCount   int
}

// updateSyncConnectionState applies heartbeat state transitions under c.mu.
//
// Invariants:
// - Caller receives a detached snapshot for post-lock lifecycle emission.
// - req.Settings/in_progress updates overwrite prior extension view atomically.
//
// Failure semantics:
// - Absent settings/in_progress leaves previous values intact.
func (c *Capture) updateSyncConnectionState(req SyncRequest, clientID string, now time.Time) syncConnectionState {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := syncConnectionState{
		wasConnected:      c.extensionState.lastExtensionConnected,
		timeSinceLastPoll: now.Sub(c.extensionState.lastPollAt),
	}
	state.wasDisconnected = !c.extensionState.lastSyncSeen.IsZero() && now.Sub(c.extensionState.lastSyncSeen) >= extensionDisconnectThreshold
	state.isReconnect = state.wasDisconnected

	c.extensionState.lastPollAt = now
	c.extensionState.lastExtensionConnected = true
	c.extensionState.lastSyncSeen = now
	c.extensionState.lastSyncClientID = clientID

	if req.ExtSessionID != "" && req.ExtSessionID != c.extensionState.extSessionID {
		c.extensionState.extSessionID = req.ExtSessionID
		c.extensionState.extSessionChangedAt = now
	}
	state.extSessionID = c.extensionState.extSessionID

	if req.Settings != nil {
		c.extensionState.pilotEnabled = req.Settings.PilotEnabled
		c.extensionState.pilotStatusKnown = true
		c.extensionState.pilotUpdatedAt = now
		c.extensionState.pilotSource = PilotSourceExtensionSync
		c.extensionState.trackingEnabled = req.Settings.TrackingEnabled
		c.extensionState.trackedTabID = req.Settings.TrackedTabID
		c.extensionState.trackedTabURL = req.Settings.TrackedTabURL
		c.extensionState.trackedTabTitle = req.Settings.TrackedTabTitle
		c.extensionState.trackingUpdated = now
		switch req.Settings.TabStatus {
		case "loading", "complete":
			c.extensionState.tabStatus = req.Settings.TabStatus
		default:
			c.extensionState.tabStatus = ""
		}
		c.extensionState.trackedTabActive = req.Settings.TrackedTabActive
		c.extensionState.cspRestricted = req.Settings.CspRestricted
		c.extensionState.cspLevel = req.Settings.CspLevel
	}
	if req.InProgress != nil {
		c.extensionState.inProgress = normalizeInProgressList(req.InProgress)
		c.extensionState.inProgressUpdated = now
	}
	state.pilotEnabled = c.extensionState.pilotEnabled
	state.inProgressCount = len(c.extensionState.inProgress)
	return state
}

// processSyncCommandResults applies extension result/status updates.
//
// Invariants:
// - Correlated commands use status from ApplyCommandResult as source of truth.
//
// Failure semantics:
// - Unknown query/command IDs are ignored to keep sync idempotent.
// - Query results can be stored even if lifecycle completion arrives separately.
func (c *Capture) processSyncCommandResults(results []SyncCommandResult, clientID string) {
	for _, result := range results {
		if result.ID != "" {
			if result.CorrelationID != "" {
				// Correlated async commands carry explicit lifecycle status below.
				// Do not force "complete" from query-id bookkeeping.
				c.SetQueryResultWithClientNoCommandComplete(result.ID, result.Result, clientID)
			} else {
				c.SetQueryResultWithClient(result.ID, result.Result, clientID)
			}
		}
		if result.CorrelationID != "" {
			c.ApplyCommandResult(result.CorrelationID, result.Status, result.Result, result.Error)
		}
	}
}

// updateSyncLogs ingests extension logs and metadata under c.mu.
//
// Invariants:
// - Extension log buffer uses amortized compaction (at 1.5x capacity) to avoid per-entry copying.
// - Redaction is applied before logs enter persistent in-memory buffers.
//
// Failure semantics:
// - Invalid/missing timestamps are normalized to server receive time.
func (c *Capture) updateSyncLogs(req SyncRequest, now time.Time, pilotEnabled bool, queryCount int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logPollingActivity(PollingLogEntry{
		Timestamp:    now,
		Endpoint:     "sync",
		Method:       "POST",
		ExtSessionID: req.ExtSessionID,
		PilotEnabled: &pilotEnabled,
		QueryCount:   queryCount,
	})

	for _, log := range req.ExtensionLogs {
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}
		log = c.redactExtensionLog(log)
		c.extensionLogs.append(log)
	}

	if req.ExtensionVersion != "" {
		c.extensionState.extensionVersion = req.ExtensionVersion
	}
}

// normalizeInProgressList sanitizes extension heartbeat command state for reconciliation.
//
// Invariants:
// - Returns nil only when caller supplied nil (distinguishes "unsupported" vs "empty list").
// - Output is capped to maxInProgress to bound memory and CPU cost per heartbeat.
//
// Failure semantics:
// - Malformed/empty entries are dropped rather than failing the whole sync request.
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
			p := *entry.ProgressPct
			if p < 0 {
				p = 0
			}
			if p > 100 {
				p = 100
			}
			entry.ProgressPct = &p
		}
		if entry.ID == "" && entry.CorrelationID == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// commandHasStarted returns true once trace evidence indicates extension execution began.
//
// Failure semantics:
// - Missing trace context returns false, which delays desync failure until stronger evidence exists.
func commandHasStarted(cmd *queries.CommandResult) bool {
	if cmd == nil {
		return false
	}
	for _, evt := range cmd.TraceEvents {
		if evt.Stage == "started" || evt.Stage == "resolved" || evt.Stage == "errored" || evt.Stage == "timed_out" {
			return true
		}
	}
	return strings.Contains(cmd.TraceTimeline, "started")
}

// reconcileInProgressCommandState detects commands lost after extension acknowledgement.
//
// Invariants:
// - A command is failed only after two consecutive missed heartbeats once "started" is observed.
// - missingInProgressByCorr map is pruned for no-longer-pending commands each cycle.
//
// Failure semantics:
// - nil inProgress means "client does not support heartbeat reporting" and reconciliation is skipped.
// - Desync failures emit terminal command errors so callers do not wait for full timeout.
func (c *Capture) reconcileInProgressCommandState(inProgress []SyncInProgress) {
	if inProgress == nil {
		// Older extension/client that doesn't report in_progress yet.
		return
	}

	active := make(map[string]struct{}, len(inProgress))
	for _, entry := range inProgress {
		if entry.CorrelationID != "" {
			active[entry.CorrelationID] = struct{}{}
		}
	}

	pending := c.GetPendingCommands()
	pendingCorr := make(map[string]struct{}, len(pending))
	toFail := make([]string, 0)
	toFailIDs := make([]string, 0)

	func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.extensionState.missingInProgressByCorr == nil {
			c.extensionState.missingInProgressByCorr = make(map[string]int)
		}
		for _, cmd := range pending {
			if cmd == nil || cmd.CorrelationID == "" {
				continue
			}
			corr := cmd.CorrelationID
			pendingCorr[corr] = struct{}{}

			if _, ok := active[corr]; ok {
				delete(c.extensionState.missingInProgressByCorr, corr)
				continue
			}
			if !commandHasStarted(cmd) {
				continue
			}
			c.extensionState.missingInProgressByCorr[corr]++
			if c.extensionState.missingInProgressByCorr[corr] >= 2 {
				toFail = append(toFail, corr)
				toFailIDs = append(toFailIDs, cmd.QueryID)
				delete(c.extensionState.missingInProgressByCorr, corr)
			}
		}

		for corr := range c.extensionState.missingInProgressByCorr {
			if _, stillPending := pendingCorr[corr]; !stillPending {
				delete(c.extensionState.missingInProgressByCorr, corr)
			}
		}
	}()

	for i, corr := range toFail {
		queryID := ""
		if i < len(toFailIDs) {
			queryID = toFailIDs[i]
		}
		c.ApplyCommandResult(
			corr,
			"error",
			nil,
			"extension_lost_command: command acknowledged by extension but missing from in_progress heartbeats",
		)
		util.SafeGo(func() {
			c.emitLifecycleEvent("command_state_desync", map[string]any{
				"correlation_id": corr,
				"query_id":       queryID,
				"reason":         "missing_in_progress_heartbeat",
			})
		})
	}
}

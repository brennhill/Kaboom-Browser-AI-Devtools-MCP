// Purpose: Records query lifecycle stage transitions (queued/sent/resolved/timed_out) for diagnostic tracing.
// Docs: docs/features/feature/query-service/index.md

package queries

import (
	"sort"
	"strings"
	"time"
)

const (
	traceStageQueued   = "queued"
	traceStageSent     = "sent"
	traceStageStarted  = "started"
	traceStageResolved = "resolved"
	traceStageTimedOut = "timed_out"
	traceStageErrored  = "errored"
)

func deriveTraceID(explicit string, correlationID string, queryID string) string {
	if explicit != "" {
		return explicit
	}
	if correlationID != "" {
		return correlationID
	}
	if queryID != "" {
		return "trace_" + queryID
	}
	return ""
}

// buildTraceTimeline produces a stable human-readable stage chain.
//
// Invariants:
// - Event order is preserved; no re-sorting is performed here.
// - Empty stages are skipped to avoid polluting timeline output.
func buildTraceTimeline(events []CommandTraceEvent) string {
	if len(events) == 0 {
		return ""
	}
	stages := make([]string, 0, len(events))
	for _, evt := range events {
		if evt.Stage == "" {
			continue
		}
		stages = append(stages, evt.Stage)
	}
	return strings.Join(stages, " -> ")
}

// traceStageFromStatus maps canonical command status into trace stage vocabulary.
//
// Failure semantics:
// - Unknown statuses yield empty stage and are intentionally ignored by appendTraceEventLocked.
func traceStageFromStatus(status string) string {
	switch normalizeCommandStatus(status) {
	case "pending":
		return traceStageStarted
	case "complete":
		return traceStageResolved
	case "error", "cancelled":
		return traceStageErrored
	case "timeout", "expired":
		return traceStageTimedOut
	default:
		return ""
	}
}

// ensureTraceContextLocked hydrates identifiers/timestamps before trace mutation.
//
// Invariants:
// - Caller must hold resultsMu.
// - Existing non-empty identity fields are preserved.
func (qd *QueryDispatcher) ensureTraceContextLocked(cmd *CommandResult, correlationID string, queryID string, traceID string, now time.Time) {
	if cmd == nil {
		return
	}
	if cmd.CorrelationID == "" {
		cmd.CorrelationID = correlationID
	}
	if cmd.TraceID == "" {
		cmd.TraceID = deriveTraceID(traceID, correlationID, queryID)
	}
	if cmd.QueryID == "" && queryID != "" {
		cmd.QueryID = queryID
	}
	if cmd.UpdatedAt.IsZero() {
		cmd.UpdatedAt = now
	}
}

// hasTraceStageLocked checks whether a stage already exists in the trace.
//
// Invariants:
// - Caller must hold resultsMu.
func (qd *QueryDispatcher) hasTraceStageLocked(cmd *CommandResult, stage string) bool {
	if cmd == nil || stage == "" {
		return false
	}
	for _, evt := range cmd.TraceEvents {
		if evt.Stage == stage {
			return true
		}
	}
	return false
}

// appendTraceEventLocked appends a stage exactly once and updates trace metadata.
//
// Invariants:
// - Caller must hold resultsMu.
// - Each stage is de-duplicated; repeated stage writes only advance UpdatedAt.
//
// Failure semantics:
// - Empty stage or nil command are ignored to keep lifecycle ingestion non-fatal.
func (qd *QueryDispatcher) appendTraceEventLocked(cmd *CommandResult, stage string, source string, status string, message string, at time.Time) {
	if cmd == nil || stage == "" {
		return
	}
	if qd.hasTraceStageLocked(cmd, stage) {
		if at.After(cmd.UpdatedAt) {
			cmd.UpdatedAt = at
		}
		return
	}

	cmd.TraceEvents = append(cmd.TraceEvents, CommandTraceEvent{
		Stage:   stage,
		At:      at,
		Source:  source,
		Status:  status,
		Message: message,
	})
	cmd.TraceTimeline = buildTraceTimeline(cmd.TraceEvents)
	cmd.UpdatedAt = at
}

// recordTraceEvent records a stage for either active or failed command history.
//
// Failure semantics:
// - Missing correlation IDs or absent command records are ignored (best-effort diagnostics path).
func (qd *QueryDispatcher) recordTraceEvent(correlationID string, stage string, source string, status string, message string, at time.Time) {
	if correlationID == "" || stage == "" {
		return
	}

	qd.resultsMu.Lock()
	defer qd.resultsMu.Unlock()

	if cmd, exists := qd.activeCommands[correlationID]; exists {
		qd.ensureTraceContextLocked(cmd, correlationID, "", "", at)
		qd.appendTraceEventLocked(cmd, stage, source, status, message, at)
		return
	}
	for _, cmd := range qd.terminalHistory {
		if cmd == nil || cmd.CorrelationID != correlationID {
			continue
		}
		qd.ensureTraceContextLocked(cmd, correlationID, "", "", at)
		qd.appendTraceEventLocked(cmd, stage, source, status, message, at)
		return
	}
}

// copyCommandResultWithTrace deep-copies trace slices for lock-free callers.
//
// Invariants:
// - Returned pointer never aliases internal trace slice memory.
func copyCommandResultWithTrace(src *CommandResult) *CommandResult {
	if src == nil {
		return nil
	}
	cp := *src
	cp.Result = cloneJSON(src.Result)
	if len(src.TraceEvents) > 0 {
		cp.TraceEvents = make([]CommandTraceEvent, len(src.TraceEvents))
		copy(cp.TraceEvents, src.TraceEvents)
	}
	return &cp
}

// GetRecentCommandTraces returns the latest command traces across active and failed commands.
// Sorted by UpdatedAt descending and bounded by limit (if > 0).
func (qd *QueryDispatcher) GetRecentCommandTraces(limit int) []*CommandResult {
	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	combined := make([]*CommandResult, 0, len(qd.activeCommands)+len(qd.terminalHistory))
	seen := make(map[string]struct{}, len(qd.activeCommands)+len(qd.terminalHistory))
	add := func(cmd *CommandResult) {
		if cmd == nil {
			return
		}
		key := cmd.CorrelationID
		if key == "" {
			key = cmd.TraceID
		}
		if key != "" {
			if _, exists := seen[key]; exists {
				return
			}
			seen[key] = struct{}{}
		}
		combined = append(combined, copyCommandResultWithTrace(cmd))
	}

	for _, cmd := range qd.terminalHistory {
		add(cmd)
	}
	for _, cmd := range qd.activeCommands {
		add(cmd)
	}

	sort.Slice(combined, func(i, j int) bool {
		left := combined[i].UpdatedAt
		right := combined[j].UpdatedAt
		if left.IsZero() {
			left = combined[i].CreatedAt
		}
		if right.IsZero() {
			right = combined[j].CreatedAt
		}
		return left.After(right)
	})

	if limit > 0 && len(combined) > limit {
		combined = combined[:limit]
	}
	return combined
}

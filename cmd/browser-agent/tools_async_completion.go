// tools_async_completion.go — Waits for async commands and shapes their final MCP responses.
// Why: Connectivity-aware waiting and result formatting form one command-completion protocol.

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asyncresult"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

const maxTransientsPerResult = asyncresult.MaxTransientsPerResult

func (h *ToolHandler) attachTransientElements(responseData map[string]any, since time.Time) {
	if h == nil || responseData == nil {
		return
	}
	asyncresult.AttachTransientElements(responseData, h.capture.GetAllEnhancedActions(), since)
}

// finalizeResponseEnrichment attaches evidence, transient elements, and retry context
// to the response data in a single call. Consolidates the repeated triplet pattern.
func (h *ToolHandler) finalizeResponseEnrichment(corrID string, responseData map[string]any, cmd queries.CommandResult) {
	h.interactAction().AttachEvidencePayload(corrID, responseData)
	h.attachTransientElements(responseData, cmd.CreatedAt)
	h.interactAction().AttachRetryContext(corrID, responseData, cmd.Status, cmd.Error)
}

func (h *ToolHandler) formatCommandResult(req JSONRPCRequest, cmd queries.CommandResult, corrID string) JSONRPCResponse {
	traceID := cmd.TraceID
	if traceID == "" {
		traceID = cmd.CorrelationID
	}

	responseData := map[string]any{
		"correlation_id":   cmd.CorrelationID,
		"trace_id":         traceID,
		"status":           cmd.Status,
		"lifecycle_status": asyncresult.CanonicalLifecycleStatus(cmd.Status),
		"queued":           false,
		"created_at":       cmd.CreatedAt.Format(time.RFC3339),
		"elapsed_ms":       cmd.ElapsedMs(),
	}
	attachTraceSummary(responseData, cmd)

	// Track async command outcome for analytics.
	if h.usageTracker != nil && cmd.Status != "pending" {
		h.usageTracker.RecordAsyncOutcome(cmd.Status)
	}

	switch cmd.Status {
	case "complete":
		responseData["final"] = true
		return h.formatCompleteCommand(req, cmd, corrID, responseData)
	case "error":
		return h.formatErrorCommandResult(req, cmd, corrID, responseData)
	case "expired":
		return h.formatExpiredCommandResult(req, cmd, corrID, responseData)
	case "timeout":
		return h.formatTimeoutCommandResult(req, cmd, corrID, responseData)
	case "cancelled":
		return h.formatCancelledCommandResult(req, cmd, corrID, responseData)
	default:
		responseData["final"] = false
		summary := fmt.Sprintf("Command %s: %s", corrID, cmd.Status)
		return succeed(req, summary, responseData)
	}
}

func (h *ToolHandler) formatErrorCommandResult(req JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) JSONRPCResponse {
	responseData["final"] = true
	if cmd.Error == "" {
		cmd.Error = "Command failed in extension"
	}
	responseData["error"] = cmd.Error
	if len(cmd.Result) > 0 {
		responseData["result"] = cmd.Result
		_, _ = asyncresult.EnrichCommandResponseData(cmd.Result, responseData)
		asyncresult.StripEnrichedFieldsFromResult(responseData)
	}
	asyncresult.AnnotateCSPFailure(responseData, cmd.Error, cmd.Result)
	asyncresult.AnnotateInteractFailureRecovery(responseData, cmd.Error, cmd.Result)

	// Add corrective hints for common out-of-order errors.
	if strings.Contains(cmd.Error, "No active recording") {
		responseData["retry"] = "No recording is active. Start one first: interact({what: 'screen_recording_start', name: 'my-recording'}) or configure({what: 'event_recording_start', name: 'my-recording'})"
	}

	h.finalizeResponseEnrichment(corrID, responseData, cmd)
	summary := fmt.Sprintf("FAILED — Command %s error: %s", corrID, cmd.Error)
	return failJSON(req, summary, responseData)
}

func (h *ToolHandler) formatExpiredCommandResult(req JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) JSONRPCResponse {
	responseData["final"] = true
	responseData["error"] = ErrExtTimeout
	responseData["message"] = fmt.Sprintf("Command %s expired before the extension could execute it. Error: %s", corrID, cmd.Error)
	responseData["retry"] = "The browser extension may be disconnected or the page is not active. Check observe with what='pilot' to verify extension status, then retry the command."
	responseData["hint"] = h.DiagnosticHintString()

	h.finalizeResponseEnrichment(corrID, responseData, cmd)
	summary := fmt.Sprintf("FAILED — Command %s expired: %s", corrID, cmd.Error)
	return failJSON(req, summary, responseData)
}

func (h *ToolHandler) formatTimeoutCommandResult(req JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) JSONRPCResponse {
	responseData["final"] = true
	responseData["error"] = ErrExtTimeout
	responseData["message"] = fmt.Sprintf("Command %s timed out waiting for the extension to respond. Error: %s", corrID, cmd.Error)
	retryMsg := "Extension connected but page execution timed out. This page may block content scripts (common on Google, Chrome Web Store, etc.). Try navigating to a different page: interact({what: 'navigate', url: 'https://example.com'})"
	if !h.capture.IsExtensionConnected() {
		retryMsg = "Extension is disconnected. Ensure the Kaboom extension shows 'Connected' and a tab is tracked, then retry."
	}
	responseData["retry"] = retryMsg
	responseData["hint"] = h.DiagnosticHintString()

	h.finalizeResponseEnrichment(corrID, responseData, cmd)
	summary := fmt.Sprintf("FAILED — Command %s timed out: %s", corrID, cmd.Error)
	return failJSON(req, summary, responseData)
}

func (h *ToolHandler) formatCancelledCommandResult(req JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) JSONRPCResponse {
	responseData["final"] = true
	responseData["error"] = ErrExtError
	responseData["message"] = fmt.Sprintf("Command %s was cancelled before completion.", corrID)
	if cmd.Error != "" {
		responseData["detail"] = cmd.Error
	}

	h.finalizeResponseEnrichment(corrID, responseData, cmd)
	summary := fmt.Sprintf("FAILED — Command %s cancelled", corrID)
	return failJSON(req, summary, responseData)
}

func attachTraceSummary(responseData map[string]any, cmd queries.CommandResult) {
	traceID := cmd.TraceID
	if traceID == "" {
		traceID = cmd.CorrelationID
	}
	if traceID == "" && len(cmd.TraceEvents) == 0 {
		return
	}

	trace := map[string]any{
		"trace_id": traceID,
		"timeline": cmd.TraceTimeline,
	}
	if cmd.QueryID != "" {
		trace["query_id"] = cmd.QueryID
	}
	if len(cmd.TraceEvents) > 0 {
		trace["last_stage"] = cmd.TraceEvents[len(cmd.TraceEvents)-1].Stage
	}
	responseData["trace"] = trace
}

func (h *ToolHandler) formatCompleteCommand(req JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) JSONRPCResponse {
	normalizedResult, normalizedErr := asyncresult.NormalizeCompleteCommandResult(corrID, cmd.Result)
	responseData["result"] = normalizedResult
	responseData["completed_at"] = cmd.CompletedAt.Format(time.RFC3339)
	responseData["timing_ms"] = cmd.CompletedAt.Sub(cmd.CreatedAt).Milliseconds()

	if cmd.Error == "" && normalizedErr != "" {
		cmd.Error = normalizedErr
	}

	if embeddedErr, hasEmbeddedErr := asyncresult.EnrichCommandResponseData(normalizedResult, responseData, corrID); cmd.Error == "" && hasEmbeddedErr {
		cmd.Error = embeddedErr
	}
	asyncresult.StripEnrichedFieldsFromResult(responseData)
	h.attachPerfDiffIfAvailable(corrID, responseData)

	if cmd.Error != "" {
		responseData["error"] = cmd.Error
		asyncresult.AnnotateCSPFailure(responseData, cmd.Error, normalizedResult)
		asyncresult.AnnotateInteractFailureRecovery(responseData, cmd.Error, normalizedResult)
		h.finalizeResponseEnrichment(corrID, responseData, cmd)
		summary := fmt.Sprintf("FAILED — Command %s completed with error: %s", corrID, cmd.Error)
		return failJSON(req, summary, responseData)
	}

	h.finalizeResponseEnrichment(corrID, responseData, cmd)
	asyncresult.StripSuccessOnlyFields(responseData)
	asyncresult.StripRetryContextOnSuccess(responseData)
	// #447: Strip verbose fields when summary mode is active
	if h.loadSummaryPref() {
		asyncresult.StripSummaryModeFields(responseData)
	}
	summary := fmt.Sprintf("Command %s: complete", corrID)
	return succeed(req, summary, responseData)
}

func (h *ToolHandler) attachPerfDiffIfAvailable(corrID string, responseData map[string]any) {
	beforeSnap, ok := h.capture.GetAndDeleteBeforeSnapshot(corrID)
	if !ok {
		return
	}

	// The "after" perf snapshot arrives ~2.5s after page load (2s content script
	// delay + 500ms batcher debounce). Poll briefly for a snapshot newer than
	// the "before" baseline. Without this wait, we'd compare the same snapshot
	// to itself (zero diff) or find nothing.
	afterSnap, found := h.capture.GetPerformanceSnapshotByURL(beforeSnap.URL)
	if !found || afterSnap.Timestamp == beforeSnap.Timestamp {
		for retry := 0; retry < 5; retry++ {
			time.Sleep(500 * time.Millisecond)
			afterSnap, found = h.capture.GetPerformanceSnapshotByURL(beforeSnap.URL)
			if found && afterSnap.Timestamp != beforeSnap.Timestamp {
				break // Found a genuinely new snapshot.
			}
		}
	}
	if !found || afterSnap.Timestamp == beforeSnap.Timestamp {
		return
	}

	before := performance.SnapshotToPageLoadMetrics(beforeSnap)
	after := performance.SnapshotToPageLoadMetrics(afterSnap)
	responseData["perf_diff"] = performance.ComputePerfDiff(before, after)
}

// Wait/poll cadence for sync-by-default commands. Tests may shorten these;
// production never mutates them.
var (
	asyncInitialWait  = 15 * time.Second
	asyncRetryWait    = 5 * time.Second
	asyncPollInterval = 500 * time.Millisecond
)

func (h *ToolHandler) waitForCommandWithConnectivity(correlationID string, timeout time.Duration) (*queries.CommandResult, bool, bool, int64) {
	deadline := time.Now().Add(timeout)
	waited := int64(0)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			cmd, found := h.capture.GetCommandResult(correlationID)
			disconnected := found && cmd != nil && cmd.Status == "pending" && !h.capture.IsExtensionConnected()
			return cmd, found, disconnected, waited
		}
		waitStep := asyncPollInterval
		if waitStep <= 0 || waitStep > remaining {
			waitStep = remaining
		}
		stepStart := time.Now()
		cmd, found := h.capture.WaitForCommand(correlationID, waitStep)
		waited += time.Since(stepStart).Milliseconds()
		if !found {
			return nil, false, false, waited
		}
		if cmd.Status != "pending" {
			return cmd, true, false, waited
		}
		if !h.capture.IsExtensionConnected() {
			return cmd, true, true, waited
		}
	}
}

func (h *ToolHandler) finalizePendingDisconnect(req JSONRPCRequest, correlationID string) JSONRPCResponse {
	h.capture.ApplyCommandResult(correlationID, "error", nil, "extension_disconnected")
	if cmd, found := h.capture.GetCommandResult(correlationID); found && cmd != nil {
		return h.formatCommandResult(req, *cmd, correlationID)
	}
	return fail(req, ErrNoData,
		"Extension disconnected while command was pending",
		"Ensure the extension is connected, then retry the action.",
		h.diagnosticHint(),
		withFinal(true))
}

// MaybeWaitForCommand implements sync-by-default command completion. Explicit
// background/async requests return a polling handle immediately.
func (h *ToolHandler) MaybeWaitForCommand(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse {
	var params struct {
		Sync       *bool `json:"sync"`
		Wait       *bool `json:"wait"`
		Background bool  `json:"background"`
		TimeoutMs  int   `json:"timeout_ms"`
	}
	lenientUnmarshal(args, &params)

	isSync := true
	if params.Background || (params.Sync != nil && !*params.Sync) || (params.Wait != nil && !*params.Wait) {
		isSync = false
	}
	if !isSync {
		return succeed(req, queuedSummary, map[string]any{
			"status": "queued", "lifecycle_status": "queued",
			"correlation_id": correlationID, "trace_id": correlationID,
			"queued": true, "final": false,
		})
	}
	if !h.capture.IsExtensionConnected() {
		return fail(req, ErrNoData, "Extension is not connected", "Ensure the Kaboom extension shows 'Connected' and a tab is tracked.", h.diagnosticHint())
	}

	initialWait, retryWait := asyncInitialWait, asyncRetryWait
	if params.TimeoutMs > 0 {
		totalBudget := time.Duration(params.TimeoutMs) * time.Millisecond
		if totalBudget < 100*time.Millisecond {
			totalBudget = 100 * time.Millisecond
		}
		if totalBudget > 120*time.Second {
			totalBudget = 120 * time.Second
		}
		initialWait = totalBudget * 3 / 4
		retryWait = totalBudget - initialWait
	}

	attempts := 1
	totalWaitMs := int64(0)
	cmd, found, disconnected, waitedMs := h.waitForCommandWithConnectivity(correlationID, initialWait)
	totalWaitMs += waitedMs
	if !found {
		return fail(req, ErrInternal, "Command not found after queuing", "Internal error — do not retry")
	}
	if disconnected {
		return h.finalizePendingDisconnect(req, correlationID)
	}
	if cmd.Status == "pending" && h.capture.IsExtensionConnected() {
		attempts = 2
		cmd, found, disconnected, waitedMs = h.waitForCommandWithConnectivity(correlationID, retryWait)
		totalWaitMs += waitedMs
		if !found {
			return fail(req, ErrInternal, "Command not found after retry", "Internal error — do not retry")
		}
		if disconnected {
			return h.finalizePendingDisconnect(req, correlationID)
		}
	}
	if cmd.Status == "pending" {
		if !h.capture.IsExtensionConnected() {
			return h.finalizePendingDisconnect(req, correlationID)
		}
		stillProcessing := map[string]any{
			"status": "still_processing", "lifecycle_status": "running",
			"correlation_id": correlationID, "trace_id": correlationID,
			"queued": false, "final": false, "elapsed_ms": cmd.ElapsedMs(),
			"queue_depth": h.capture.QueueDepth(),
			"retry_context": map[string]any{
				"attempts": attempts, "total_wait_ms": totalWaitMs,
				"extension_connected": h.capture.IsExtensionConnected(),
			},
			"suggested_retry_ms": 2000,
			"message":            "Action is taking longer than expected. Polling is now required. Use observe({what:'command_result', correlation_id:'" + correlationID + "'}) to check the result.",
		}
		if pos := h.capture.QueuePosition(correlationID); pos >= 0 {
			stillProcessing["queue_position"] = pos
		}
		return succeed(req, "Action still processing", stillProcessing)
	}
	return h.formatCommandResult(req, *cmd, correlationID)
}

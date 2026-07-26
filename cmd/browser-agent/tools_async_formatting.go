// Purpose: Formats async command results into stable MCP response envelopes.
// Why: Keep lifecycle polling separate from payload shaping and error semantics.

package main

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asyncresult"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

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

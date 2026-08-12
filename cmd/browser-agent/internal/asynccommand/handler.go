// handler.go — Waits for async commands and shapes their final MCP responses.
// Why: Connectivity-aware waiting and result formatting form one command-completion protocol.

package asynccommand

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asyncresult"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/commandcontract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// MaxTransientsPerResult bounds transient UI enrichment.
const MaxTransientsPerResult = asyncresult.MaxTransientsPerResult

const a11yQueryTimeout = 30 * time.Second

// Deps names the owners and cross-feature enrichment operations used by commands.
type Deps struct {
	Context              context.Context
	Capture              *capture.Capture
	DiagnosticHint       func(*mcp.StructuredError)
	DiagnosticHintString func() string
	AttachEvidence       func(string, map[string]any)
	AttachRetryContext   func(string, map[string]any, string, string)
	SummaryEnabled       func() bool
	RecordAsyncOutcome   func(string)
}

// WaitConfig controls synchronous command completion cadence.
type WaitConfig struct {
	Initial      time.Duration
	Retry        time.Duration
	PollInterval time.Duration
	Command      func(string, time.Duration) (*queries.CommandResult, bool)
	Now          func() time.Time
}

// Handler owns the complete asynchronous command lifecycle.
type Handler struct {
	deps Deps
	Wait WaitConfig
}

// New constructs an asynchronous command lifecycle owner.
func New(deps Deps) *Handler {
	if deps.Context == nil {
		deps.Context = context.Background()
	}
	if deps.DiagnosticHint == nil {
		deps.DiagnosticHint = func(*mcp.StructuredError) {}
	}
	if deps.DiagnosticHintString == nil {
		deps.DiagnosticHintString = func() string { return "" }
	}
	if deps.AttachEvidence == nil {
		deps.AttachEvidence = func(string, map[string]any) {}
	}
	if deps.AttachRetryContext == nil {
		deps.AttachRetryContext = func(string, map[string]any, string, string) {}
	}
	waitForCommand := func(string, time.Duration) (*queries.CommandResult, bool) {
		return nil, false
	}
	if deps.Capture != nil {
		waitForCommand = deps.Capture.Queries().WaitForCommand
	}
	return &Handler{
		deps: deps,
		Wait: WaitConfig{
			Initial: 15 * time.Second, Retry: 5 * time.Second,
			PollInterval: 500 * time.Millisecond,
			Command:      waitForCommand,
			Now:          time.Now,
		},
	}
}

// AttachTransientElements adds transient UI events created after the command.
func (h *Handler) AttachTransientElements(responseData map[string]any, since time.Time) {
	if h == nil || responseData == nil {
		return
	}
	asyncresult.AttachTransientElements(responseData, h.deps.Capture.Telemetry().Actions().Snapshot().Actions, since)
}

func (h *Handler) EnqueuePendingQuery(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
	if h.deps.Capture.Extension().IsExtensionConnected() && h.deps.Capture.Extension().CommandContractID() != commandcontract.ID {
		return mcp.Fail(req, mcp.ErrExtError,
			"command_contract_mismatch: the loaded extension cannot safely execute this daemon's commands",
			"Reload the Kaboom extension, then retry after System Doctor reports a matching command contract.",
			h.deps.DiagnosticHint,
		), true
	}
	_, err := h.deps.Capture.Queries().CreatePendingQueryWithTimeout(query, timeout, req.ClientID)
	if err == nil {
		return mcp.JSONRPCResponse{}, false
	}
	if errors.Is(err, queries.ErrQueueFull) {
		return mcp.Fail(req, mcp.ErrQueueFull,
			fmt.Sprintf("Command queue is full; unable to enqueue action type=%q", query.Type),
			"Wait for in-flight commands to complete, then retry.",
			mcp.WithRetryable(true), mcp.WithRetryAfterMs(1000), h.deps.DiagnosticHint,
			mcp.WithRecoveryToolCall(map[string]any{
				"tool": "observe", "arguments": map[string]any{"what": "pending_commands"},
			}),
		), true
	}
	return mcp.Fail(req, mcp.ErrInternal,
		fmt.Sprintf("Failed to enqueue command type=%q: %v", query.Type, err),
		"Internal error — do not retry until server health is restored.",
		h.deps.DiagnosticHint,
	), true
}

// BuildA11yQueryParams creates the extension payload for an accessibility query.
func BuildA11yQueryParams(scope string, tags []string, frame any, forceRefresh bool) map[string]any {
	params := map[string]any{}
	if scope != "" {
		params["scope"] = scope
	}
	if len(tags) > 0 {
		params["tags"] = tags
	}
	if forceRefresh {
		params["force_refresh"] = true
	}
	if frame != nil {
		params["frame"] = frame
	}
	return params
}

func (h *Handler) ExecuteA11yQuery(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error) {
	paramsJSON, _ := json.Marshal(BuildA11yQueryParams(scope, tags, frame, forceRefresh))
	queryID, err := h.deps.Capture.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type: "a11y", Params: paramsJSON,
	}, a11yQueryTimeout, "")
	if err != nil {
		return nil, err
	}
	return h.deps.Capture.Queries().WaitForResult(queryID, a11yQueryTimeout)
}

// ExecuteStyleProbe runs one computed-style query and returns the raw payload.
//
// design_audit needs the probe result as data, not as an MCP response, because
// it runs three analyzers over the same capture — so it takes the same
// create-and-wait path as ExecuteA11yQuery rather than the queue-and-format
// path the computed_styles mode uses to hand a result straight to the caller.
func (h *Handler) ExecuteStyleProbe(selector string, maxElements int, includeCustomProperties bool) (json.RawMessage, error) {
	params := map[string]any{"selector": selector}
	if maxElements > 0 {
		params["max_elements"] = maxElements
	}
	if includeCustomProperties {
		params["include_custom_properties"] = true
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	queryID, err := h.deps.Capture.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type: "computed_styles", Params: paramsJSON,
	}, a11yQueryTimeout, "")
	if err != nil {
		return nil, err
	}
	return h.deps.Capture.Queries().WaitForResult(queryID, a11yQueryTimeout)
}

// finalizeResponseEnrichment attaches evidence, transient elements, and retry context
// to the response data in a single call. Consolidates the repeated triplet pattern.
func (h *Handler) finalizeResponseEnrichment(corrID string, responseData map[string]any, cmd queries.CommandResult) {
	h.deps.AttachEvidence(corrID, responseData)
	h.AttachTransientElements(responseData, cmd.CreatedAt)
	h.deps.AttachRetryContext(corrID, responseData, cmd.Status, cmd.Error)
}

// FormatCommandResult creates the terminal or polling response for a command.
func (h *Handler) FormatCommandResult(req mcp.JSONRPCRequest, cmd queries.CommandResult, corrID string) mcp.JSONRPCResponse {
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
	AttachTraceSummary(responseData, cmd)

	// Track async command outcome for analytics.
	if h.deps.RecordAsyncOutcome != nil && cmd.Status != "pending" {
		h.deps.RecordAsyncOutcome(cmd.Status)
	}

	switch cmd.Status {
	case "complete":
		responseData["final"] = true
		return h.formatCompletedCommand(req, cmd, corrID, responseData)
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
		return mcp.Succeed(req, summary, responseData)
	}
}

func (h *Handler) formatErrorCommandResult(req mcp.JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) mcp.JSONRPCResponse {
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
	return toolresp.FailJSON(req, summary, responseData)
}

func (h *Handler) formatExpiredCommandResult(req mcp.JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) mcp.JSONRPCResponse {
	responseData["final"] = true
	responseData["error"] = mcp.ErrExtTimeout
	responseData["message"] = fmt.Sprintf("Command %s expired before the extension could execute it. Error: %s", corrID, cmd.Error)
	responseData["retry"] = "The browser extension may be disconnected or the page is not active. Check observe with what='pilot' to verify extension status, then retry the command."
	responseData["hint"] = h.deps.DiagnosticHintString()

	h.finalizeResponseEnrichment(corrID, responseData, cmd)
	summary := fmt.Sprintf("FAILED — Command %s expired: %s", corrID, cmd.Error)
	return toolresp.FailJSON(req, summary, responseData)
}

func (h *Handler) formatTimeoutCommandResult(req mcp.JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) mcp.JSONRPCResponse {
	responseData["final"] = true
	responseData["error"] = mcp.ErrExtTimeout
	responseData["message"] = fmt.Sprintf("Command %s timed out waiting for the extension to respond. Error: %s", corrID, cmd.Error)
	retryMsg := "Extension connected but page execution timed out. This page may block content scripts (common on Google, Chrome Web Store, etc.). Try navigating to a different page: interact({what: 'navigate', url: 'https://example.com'})"
	if !h.deps.Capture.Extension().IsExtensionConnected() {
		retryMsg = "Extension is disconnected. Ensure the Kaboom extension shows 'Connected' and a tab is tracked, then retry."
	}
	responseData["retry"] = retryMsg
	responseData["hint"] = h.deps.DiagnosticHintString()

	h.finalizeResponseEnrichment(corrID, responseData, cmd)
	summary := fmt.Sprintf("FAILED — Command %s timed out: %s", corrID, cmd.Error)
	return toolresp.FailJSON(req, summary, responseData)
}

func (h *Handler) formatCancelledCommandResult(req mcp.JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) mcp.JSONRPCResponse {
	responseData["final"] = true
	responseData["error"] = mcp.ErrExtError
	responseData["message"] = fmt.Sprintf("Command %s was cancelled before completion.", corrID)
	if cmd.Error != "" {
		responseData["detail"] = cmd.Error
	}

	h.finalizeResponseEnrichment(corrID, responseData, cmd)
	summary := fmt.Sprintf("FAILED — Command %s cancelled", corrID)
	return toolresp.FailJSON(req, summary, responseData)
}

// AttachTraceSummary adds compact command trace metadata to a response.
func AttachTraceSummary(responseData map[string]any, cmd queries.CommandResult) {
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

func (h *Handler) formatCompletedCommand(req mcp.JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) mcp.JSONRPCResponse {
	normalizedResult, normalizedErr := asyncresult.NormalizeCompletedCommandResult(corrID, cmd.Result)
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
		return toolresp.FailJSON(req, summary, responseData)
	}

	h.finalizeResponseEnrichment(corrID, responseData, cmd)
	asyncresult.StripSuccessOnlyFields(responseData)
	asyncresult.StripRetryContextOnSuccess(responseData)
	// #447: Strip verbose fields when summary mode is active
	if h.deps.SummaryEnabled != nil && h.deps.SummaryEnabled() {
		asyncresult.StripSummaryModeFields(responseData)
	}
	summary := fmt.Sprintf("Command %s: complete", corrID)
	return mcp.Succeed(req, summary, responseData)
}

func (h *Handler) attachPerfDiffIfAvailable(corrID string, responseData map[string]any) {
	beforeSnap, ok := h.deps.Capture.Performance().TakeBefore(corrID)
	if !ok {
		return
	}

	// Content capture publishes the post-navigation snapshot asynchronously.
	// The store's generation channel provides a bounded, cancellation-aware wait
	// without polling the hot path or comparing a baseline to itself.
	afterSnap, changed := h.deps.Capture.Performance().WaitForURLSnapshotChange(
		h.deps.Context, beforeSnap.URL, beforeSnap.Timestamp, 2500*time.Millisecond,
	)
	if !changed {
		return
	}

	before := performance.SnapshotToPageLoadMetrics(beforeSnap)
	after := performance.SnapshotToPageLoadMetrics(afterSnap)
	responseData["perf_diff"] = performance.ComputePerfDiff(before, after)
}

func (h *Handler) waitForCommandWithConnectivity(correlationID string, timeout time.Duration) (*queries.CommandResult, bool, bool, int64) {
	deadline := h.Wait.Now().Add(timeout)
	waited := int64(0)
	for {
		remaining := deadline.Sub(h.Wait.Now())
		if remaining <= 0 {
			cmd, found := h.deps.Capture.Queries().GetCommandResult(correlationID)
			disconnected := found && cmd != nil && cmd.Status == "pending" && !h.deps.Capture.Extension().IsExtensionConnected()
			return cmd, found, disconnected, waited
		}
		waitStep := h.Wait.PollInterval
		if waitStep <= 0 || waitStep > remaining {
			waitStep = remaining
		}
		stepStart := h.Wait.Now()
		cmd, found := h.Wait.Command(correlationID, waitStep)
		waited += h.Wait.Now().Sub(stepStart).Milliseconds()
		if !found {
			return nil, false, false, waited
		}
		if cmd.Status != "pending" {
			return cmd, true, false, waited
		}
		if !h.deps.Capture.Extension().IsExtensionConnected() {
			return cmd, true, true, waited
		}
	}
}

func (h *Handler) finalizePendingDisconnect(req mcp.JSONRPCRequest, correlationID string) mcp.JSONRPCResponse {
	h.deps.Capture.Queries().ApplyCommandResult(correlationID, "error", nil, "extension_disconnected")
	if cmd, found := h.deps.Capture.Queries().GetCommandResult(correlationID); found && cmd != nil {
		return h.FormatCommandResult(req, *cmd, correlationID)
	}
	return mcp.Fail(req, mcp.ErrNoData,
		"Extension disconnected while command was pending",
		"Ensure the extension is connected, then retry the action.",
		h.deps.DiagnosticHint,
		mcp.WithFinal(true))
}

// MaybeWaitForCommand implements sync-by-default command completion. Explicit
// background/async requests return a polling handle immediately.
func (h *Handler) MaybeWaitForCommand(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
	var params struct {
		Sync       *bool `json:"sync"`
		Wait       *bool `json:"wait"`
		Background bool  `json:"background"`
		TimeoutMs  int   `json:"timeout_ms"`
	}
	mcp.LenientUnmarshal(args, &params)

	isSync := true
	if params.Background || (params.Sync != nil && !*params.Sync) || (params.Wait != nil && !*params.Wait) {
		isSync = false
	}
	if !isSync {
		return mcp.Succeed(req, queuedSummary, map[string]any{
			"status": "queued", "lifecycle_status": "queued",
			"correlation_id": correlationID, "trace_id": correlationID,
			"queued": true, "final": false,
			"hint": "Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to retrieve the result.",
		})
	}
	if !h.deps.Capture.Extension().IsExtensionConnected() {
		return mcp.Fail(req, mcp.ErrNoData, "Extension is not connected", "Ensure the Kaboom extension shows 'Connected' and a tab is tracked.", h.deps.DiagnosticHint)
	}

	initialWait, retryWait := h.Wait.Initial, h.Wait.Retry
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
		return mcp.Fail(req, mcp.ErrInternal, "Command not found after queuing", "Internal error — do not retry")
	}
	if disconnected {
		return h.finalizePendingDisconnect(req, correlationID)
	}
	if cmd.Status == "pending" && h.deps.Capture.Extension().IsExtensionConnected() {
		attempts = 2
		cmd, found, disconnected, waitedMs = h.waitForCommandWithConnectivity(correlationID, retryWait)
		totalWaitMs += waitedMs
		if !found {
			return mcp.Fail(req, mcp.ErrInternal, "Command not found after retry", "Internal error — do not retry")
		}
		if disconnected {
			return h.finalizePendingDisconnect(req, correlationID)
		}
	}
	if cmd.Status == "pending" {
		if !h.deps.Capture.Extension().IsExtensionConnected() {
			return h.finalizePendingDisconnect(req, correlationID)
		}
		stillProcessing := map[string]any{
			"status": "still_processing", "lifecycle_status": "running",
			"correlation_id": correlationID, "trace_id": correlationID,
			"queued": false, "final": false, "elapsed_ms": cmd.ElapsedMs(),
			"queue_depth": h.deps.Capture.Queries().QueueDepth(),
			"retry_context": map[string]any{
				"attempts": attempts, "total_wait_ms": totalWaitMs,
				"extension_connected": h.deps.Capture.Extension().IsExtensionConnected(),
			},
			"suggested_retry_ms": 2000,
			"message":            "Action is taking longer than expected. Polling is now required. Use observe({what:'command_result', correlation_id:'" + correlationID + "'}) to check the result.",
		}
		if pos := h.deps.Capture.Queries().QueuePosition(correlationID); pos >= 0 {
			stillProcessing["queue_position"] = pos
		}
		return mcp.Succeed(req, "Action still processing", stillProcessing)
	}
	return h.FormatCommandResult(req, *cmd, correlationID)
}

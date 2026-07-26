// Purpose: Sync-by-default async command wait and timeout behavior.

package main

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// Wait/poll cadence for the sync-by-default async path. Package-level vars
// rather than consts so tests can shorten them; production never mutates them.
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

// ============================================
// Sync-by-Default Command Dispatch
// ============================================

// MaybeWaitForCommand blocks until an async command completes, or returns a "queued"
// response if background execution is requested or the safety timeout is reached.
// This is the core implementation of "Synchronous-by-Default" mode.
//
// Accepts optional timeout_ms parameter (via args JSON) to override the default
// wait duration. When timeout_ms > 0, the total wait budget is set to that value
// instead of the default 15s + 5s retry. Issue #275.
func (h *ToolHandler) MaybeWaitForCommand(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse {
	var params struct {
		Sync       *bool `json:"sync"`
		Wait       *bool `json:"wait"`
		Background bool  `json:"background"`
		TimeoutMs  int   `json:"timeout_ms"`
	}
	lenientUnmarshal(args, &params)

	// Default to sync unless Background is true or Sync/Wait explicitly set to false
	isSync := true
	if params.Background {
		isSync = false
	}
	if params.Sync != nil && !*params.Sync {
		isSync = false
	}
	if params.Wait != nil && !*params.Wait {
		isSync = false
	}

	if !isSync {
		return succeed(req, queuedSummary, map[string]any{
			"status":           "queued",
			"lifecycle_status": "queued",
			"correlation_id":   correlationID,
			"trace_id":         correlationID,
			"queued":           true,
			"final":            false,
		})
	}

	// Extension connection check: requireExtension already waited for the cold-start
	// readiness gate before dispatching the command (P1-2 fix: no double wait).
	// Here we only do an instant check to catch disconnections that occurred after
	// requireExtension passed but before we reached this point.
	if !h.capture.IsExtensionConnected() {
		return fail(req, ErrNoData, "Extension is not connected", "Ensure the Kaboom extension shows 'Connected' and a tab is tracked.", h.diagnosticHint())
	}

	// Determine wait budget from timeout_ms or defaults.
	// timeout_ms > 0 overrides the default 15s initial + 5s retry pattern.
	// Clamp to [100ms, 120s] to prevent misuse.
	initialWait := asyncInitialWait
	retryWait := asyncRetryWait
	if params.TimeoutMs > 0 {
		totalBudget := time.Duration(params.TimeoutMs) * time.Millisecond
		if totalBudget < 100*time.Millisecond {
			totalBudget = 100 * time.Millisecond
		}
		if totalBudget > 120*time.Second {
			totalBudget = 120 * time.Second
		}
		// Allocate 75% to initial wait, 25% to retry
		initialWait = totalBudget * 3 / 4
		retryWait = totalBudget - initialWait
	}

	// Wait for result (15s default for "Sync-by-Default" pattern, or timeout_ms).
	// Most DOM actions (click, type) take < 500ms over long-polling.
	// 15s is safe for most navigations while staying well under the 60s IDE timeout.
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

	// Single retry: if still pending and extension is connected, wait more.
	// This catches commands that complete just after the initial timeout.
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

	// If still pending after retry, return a "still_processing" handle so the agent can poll.
	if cmd.Status == "pending" {
		if !h.capture.IsExtensionConnected() {
			return h.finalizePendingDisconnect(req, correlationID)
		}
		stillProcessing := map[string]any{
			"status":           "still_processing",
			"lifecycle_status": "running",
			"correlation_id":   correlationID,
			"trace_id":         correlationID,
			"queued":           false,
			"final":            false,
			"elapsed_ms":       cmd.ElapsedMs(),
			"queue_depth":      h.capture.QueueDepth(),
			"retry_context": map[string]any{
				"attempts":            attempts,
				"total_wait_ms":       totalWaitMs,
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

	// Result received — format using standard command result formatter
	return h.formatCommandResult(req, *cmd, correlationID)
}

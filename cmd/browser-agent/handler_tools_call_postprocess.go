// Purpose: Applies cross-cutting tool response post-processing (redaction, warnings, telemetry).
// Why: Keeps tools/call execution flow focused while encapsulating response-guard policies.

package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// applyToolResponsePostProcessing applies redaction and operator warnings to tool output.
func (h *MCPHandler) applyToolResponsePostProcessing(resp JSONRPCResponse, clientID, toolName, telemetryModeOverride string) JSONRPCResponse {
	redactor := h.toolHandler.GetRedactionEngine()
	if redactor != nil && resp.Result != nil {
		resp.Result = redactor.RedactJSON(resp.Result)
	}
	if h.server != nil {
		resp = appendWarningsToResponse(resp, h.server.TakeWarnings())
	}
	resp = h.maybeAddSecurityModeWarning(resp)
	resp = h.maybeAddVersionWarning(resp)
	resp = maybeAddUpdateAvailableWarning(resp)
	resp = maybeAddUpgradeWarning(resp)
	resp = h.maybeAddPendingIntents(resp)
	return h.maybeAddTelemetrySummary(resp, clientID, toolName, telemetryModeOverride)
}

// maybeAddPendingIntents prepends a strong nudge when the user has requested an audit
// via the tracked-site UI but no terminal session was available to inject the prompt.
// Nudges up to intentMaxNudges times before discarding, so the intent survives
// if the AI doesn't act on it immediately.
func (h *MCPHandler) maybeAddPendingIntents(resp JSONRPCResponse) JSONRPCResponse {
	if h.server == nil || h.server.intentStore == nil || resp.Result == nil {
		return resp
	}
	if !h.server.intentStore.NudgeAndClean() {
		return resp
	}
	warning := "ACTION REQUIRED: The user clicked 'Audit' in the browser. " +
		"Run the Kaboom audit workflow (/kaboom/audit or /audit fallback) " +
		"for a full six-lane report.\n\n"
	return prependWarningToResponse(resp, warning)
}

// prependWarningToResponse prepends a warning string to the first text block of an MCP response.
func prependWarningToResponse(resp JSONRPCResponse, warning string) JSONRPCResponse {
	return mcp.PrependWarningToResponse(resp, warning)
}

func (h *MCPHandler) maybeAddSecurityModeWarning(resp JSONRPCResponse) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return resp
	}

	mode, productionParity, rewrites := cap.GetSecurityMode()
	if mode == capture.SecurityModeNormal {
		return resp
	}

	warning := "[ALTERED ENVIRONMENT] security_mode=insecure_proxy; production_parity=false. CSP headers are rewritten for debugging.\n\n"
	resp = prependWarningToResponse(resp, warning)

	return mutateToolResult(resp, func(r *MCPToolResult) {
		if r.Metadata == nil {
			r.Metadata = make(map[string]any)
		}
		r.Metadata["security_mode"] = mode
		r.Metadata["production_parity"] = productionParity
		r.Metadata["insecure_rewrites_applied"] = rewrites
	})
}

// maybeAddVersionWarning prepends a version mismatch warning when extension/server major.minor differ.
func (h *MCPHandler) maybeAddVersionWarning(resp JSONRPCResponse) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return resp
	}
	extVer, srvVer, mismatch := cap.GetVersionMismatch()
	if !mismatch {
		return resp
	}

	warning := fmt.Sprintf("WARNING: Version mismatch detected — server v%s, extension v%s. Update your extension to avoid issues.\n\n", srvVer, extVer)
	return prependWarningToResponse(resp, warning)
}

// updateNotifyLastShown tracks when the update-available warning was last shown.
// Guarded by updateNotifyMu: the MCP/HTTP request path can run concurrently.
var (
	updateNotifyLastShown time.Time
	updateNotifyMu        sync.Mutex
)

// maybeAddUpdateAvailableWarning prepends a notice when a newer release is available.
func maybeAddUpdateAvailableWarning(resp JSONRPCResponse) JSONRPCResponse {
	if resp.Result == nil {
		return resp
	}

	if binaryUpgradeState != nil {
		if pending, _, _ := binaryUpgradeState.UpgradeInfo(); pending {
			return resp
		}
	}

	availVer := getAvailableVersion()

	if availVer == "" || !isNewerVersion(availVer, version) {
		return resp
	}
	// Atomic check-and-set of the 24h throttle so concurrent requests don't race.
	updateNotifyMu.Lock()
	recentlyShown := !updateNotifyLastShown.IsZero() && time.Since(updateNotifyLastShown) < 24*time.Hour
	if !recentlyShown {
		updateNotifyLastShown = time.Now()
	}
	updateNotifyMu.Unlock()
	if recentlyShown {
		return resp
	}

	warning := fmt.Sprintf("UPDATE AVAILABLE: Kaboom v%s is available (current: v%s). Run: npm install -g kaboom-agentic-browser@latest\n\n", availVer, version)
	return prependWarningToResponse(resp, warning)
}

// maybeAddUpgradeWarning prepends a binary-upgrade notice when auto-restart is pending.
func maybeAddUpgradeWarning(resp JSONRPCResponse) JSONRPCResponse {
	if binaryUpgradeState == nil || resp.Result == nil {
		return resp
	}
	pending, newVer, detectedAt := binaryUpgradeState.UpgradeInfo()
	if !pending {
		return resp
	}

	elapsed := time.Since(detectedAt).Truncate(time.Second)
	warning := fmt.Sprintf("NOTICE: Kaboom v%s detected on disk (current: v%s, detected %s ago). Auto-restart imminent. Your next tool call will use the new version.\n\n", newVer, version, elapsed)
	return prependWarningToResponse(resp, warning)
}

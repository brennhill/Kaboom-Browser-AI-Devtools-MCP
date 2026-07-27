// Purpose: Implements pilot/extension/csp/tab-tracking gate checks for tool handlers.
// Why: Keeps runtime precondition checks and recovery hints isolated from error alias definitions.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// injectCSPBlockedActions adds CSP-blocked action guidance to JSON tool results.
func (h *ToolHandler) injectCSPBlockedActions(resp JSONRPCResponse) JSONRPCResponse {
	restricted, level := h.capture.GetCSPStatus()
	if !restricted {
		return resp
	}
	actions, reason := capture.CSPBlockedActions(level)
	if actions == nil {
		return resp
	}
	return mcp.MutateToolResult(resp, func(result *MCPToolResult) {
		if len(result.Content) == 0 {
			return
		}
		text := result.Content[0].Text
		jsonStart := strings.IndexByte(text, '{')
		if jsonStart < 0 {
			return
		}
		var data map[string]any
		if json.Unmarshal([]byte(text[jsonStart:]), &data) != nil {
			return
		}
		data["blocked_actions"] = actions
		data["blocked_reason"] = reason
		dataJSON, err := json.Marshal(data)
		if err == nil {
			result.Content[0].Text = text[:jsonStart] + string(dataJSON)
		}
	})
}

// guardCheck is a precondition that returns (response, true) to short-circuit the caller.
type guardCheck func(req JSONRPCRequest, opts ...func(*mcp.StructuredError)) (JSONRPCResponse, bool)

// checkGuards runs each guard in order, returning the first blocking response.
// Eliminates the repeated 6-line requirePilot+requireExtension boilerplate:
//
//	Before: if resp, blocked := h.requirePilot(req); blocked { return resp }
//	        if resp, blocked := h.requireExtension(req); blocked { return resp }
//	After:  if resp, blocked := checkGuards(req, h.requirePilot, h.requireExtension); blocked { return resp }
func checkGuards(req JSONRPCRequest, guards ...guardCheck) (JSONRPCResponse, bool) {
	for _, g := range guards {
		if resp, blocked := g(req); blocked {
			return resp, true
		}
	}
	return JSONRPCResponse{}, false
}

// checkGuardsWithOpts runs each guard in order with extra mcp.StructuredError options,
// returning the first blocking response. Used by handlers like handleDOMPrimitive
// that need to pass contextOpts (action, selector) through to guard error responses.
func checkGuardsWithOpts(req JSONRPCRequest, opts []func(*mcp.StructuredError), guards ...guardCheck) (JSONRPCResponse, bool) {
	for _, g := range guards {
		if resp, blocked := g(req, opts...); blocked {
			return resp, true
		}
	}
	return JSONRPCResponse{}, false
}

// DiagnosticHintString renders the runtime state that explains guard failures.
func (h *ToolHandler) DiagnosticHintString() string {
	extConnected := h.capture.IsExtensionConnected()
	pilotEnabled := h.capture.IsPilotEnabled()
	pilotState := ""
	if status, ok := h.capture.GetPilotStatus().(map[string]any); ok {
		if state, ok := status["state"].(string); ok {
			pilotState = state
		}
		if effective, ok := status["enabled"].(bool); ok {
			pilotEnabled = effective
		}
	}
	enabled, tabID, tabURL := h.capture.GetTrackingStatus()

	var parts []string
	if extConnected {
		parts = append(parts, "extension=connected")
	} else {
		parts = append(parts, "extension=DISCONNECTED")
	}
	pilotStatus := "pilot=DISABLED"
	switch pilotState {
	case "assumed_enabled":
		pilotStatus = "pilot=ASSUMED_ENABLED(startup)"
	case "explicitly_disabled":
		pilotStatus = "pilot=DISABLED(explicit)"
	case "enabled":
		pilotStatus = "pilot=enabled"
	default:
		if pilotEnabled {
			pilotStatus = "pilot=enabled"
		}
	}
	parts = append(parts, pilotStatus)
	if enabled && tabURL != "" {
		parts = append(parts, fmt.Sprintf("tracked_tab=%q (id=%d)", tabURL, tabID))
	} else {
		parts = append(parts, "tracked_tab=NONE")
	}
	cspRestricted, cspLevel := h.capture.GetCSPStatus()
	if cspRestricted {
		parts = append(parts, fmt.Sprintf("csp=RESTRICTED(%s)", cspLevel))
	} else {
		parts = append(parts, "csp=clear")
	}
	return "Current state: " + strings.Join(parts, ", ")
}

func (h *ToolHandler) diagnosticHint() func(*mcp.StructuredError) {
	return mcp.WithHint(h.DiagnosticHintString())
}

// requirePilot returns (resp, true) if AI Web Pilot is disabled, short-circuiting the caller.
// Usage: if resp, blocked := h.requirePilot(req); blocked { return resp }
func (h *ToolHandler) requirePilot(req JSONRPCRequest, extraOpts ...func(*mcp.StructuredError)) (JSONRPCResponse, bool) {
	if h.capture.IsPilotActionAllowed() {
		return JSONRPCResponse{}, false
	}
	opts := append([]func(*mcp.StructuredError){
		h.diagnosticHint(),
		mcp.WithRecoveryToolCall(map[string]any{
			"tool":      "observe",
			"arguments": map[string]any{"what": "pilot"},
		}),
	}, extraOpts...)
	return mcp.Fail(req, mcp.ErrCodePilotDisabled, "AI Web Pilot is explicitly disabled",
		"Enable AI Web Pilot in the extension popup", opts...,
	), true
}

// requireExtension returns (resp, true) if the browser extension is not connected,
// short-circuiting the caller with a structured error. On cold starts it waits up to
// ExtensionReadinessTimeout (5s) for the extension to connect before giving up.
// Usage: if resp, blocked := h.requireExtension(req); blocked { return resp }
func (h *ToolHandler) requireExtension(req JSONRPCRequest, extraOpts ...func(*mcp.StructuredError)) (JSONRPCResponse, bool) {
	timeout := h.extensionReadinessTimeout
	if timeout <= 0 {
		timeout = capture.ExtensionReadinessTimeout
	}
	// Use shutdownCtx so the wait aborts promptly when the server shuts down,
	// preventing goroutine leaks. Falls back to context.Background() if the
	// handler was constructed without a shutdown context (e.g., in tests).
	ctx := h.shutdownCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if h.capture.WaitForExtensionConnected(ctx, timeout) {
		return JSONRPCResponse{}, false
	}
	opts := append([]func(*mcp.StructuredError){
		h.diagnosticHint(),
		mcp.WithRetryable(true),
		mcp.WithRetryAfterMs(3000),
		mcp.WithRecoveryToolCall(map[string]any{
			"tool":      "observe",
			"arguments": map[string]any{"what": "pilot"},
		}),
	}, extraOpts...)
	return mcp.Fail(req, mcp.ErrNoData, "Extension not connected. Commands cannot be dispatched.",
		"Check that the Kaboom browser extension is installed and the page is open.",
		opts...,
	), true
}

// requireCSPClear returns (resp, true) if the page's CSP blocks script execution
// for the given world. Only world="main" is blocked — "auto" and "isolated" bypass
// page CSP because the extension's ISOLATED world is not subject to page CSP, and
// "auto" falls back from MAIN → ISOLATED → structured executor automatically.
// Usage: if resp, blocked := h.requireCSPClear(req, world); blocked { return resp }
func (h *ToolHandler) requireCSPClear(req JSONRPCRequest, world string) (JSONRPCResponse, bool) {
	// Only MAIN world execution is blocked by page CSP.
	// ISOLATED world runs in the extension's security context (bypasses page CSP).
	// AUTO tries MAIN first, then falls back to ISOLATED/structured — the extension handles this.
	if world != "main" {
		return JSONRPCResponse{}, false
	}
	restricted, level := h.capture.GetCSPStatus()
	if !restricted {
		return JSONRPCResponse{}, false
	}
	// Recovery template: LLM should re-send its original call with world='auto'.
	// The 'script' param is intentionally omitted — the LLM fills it from its original call.
	return mcp.Fail(req, mcp.ErrExtError,
		fmt.Sprintf("Page CSP blocks MAIN world script execution (level: %s). Use world='auto' or world='isolated' to bypass.", level),
		"Retry with world='auto' (falls back to isolated/structured), world='isolated' (DOM access, no page JS), or use DOM primitives (click, type).",
		h.diagnosticHint(),
		mcp.WithRecoveryToolCall(map[string]any{
			"tool":      "interact",
			"arguments": map[string]any{"what": "execute_js", "world": "auto"},
		}),
	), true
}

// requireSessionStore returns (resp, true) if the session store is not initialized.
// Usage: if resp, blocked := h.requireSessionStore(req); blocked { return resp }
func (h *ToolHandler) requireSessionStore(req JSONRPCRequest) (JSONRPCResponse, bool) {
	if h.sessionStoreImpl != nil {
		return JSONRPCResponse{}, false
	}
	return mcp.Fail(req, mcp.ErrNotInitialized, "Session store not initialized", "Internal error — do not retry"), true
}

// requireTabTracking returns (resp, true) if no tab is being tracked,
// short-circuiting the caller with an immediate structured error (~5ms) instead of
// queuing a command that would time out or target the wrong tab.
// Usage: if resp, blocked := h.requireTabTracking(req); blocked { return resp }
func (h *ToolHandler) requireTabTracking(req JSONRPCRequest, extraOpts ...func(*mcp.StructuredError)) (JSONRPCResponse, bool) {
	enabled, _, _ := h.capture.GetTrackingStatus()
	if enabled {
		return JSONRPCResponse{}, false
	}
	opts := append([]func(*mcp.StructuredError){
		h.diagnosticHint(),
		mcp.WithRetryable(true),
		mcp.WithRetryAfterMs(2000),
	}, extraOpts...)
	return mcp.Fail(req, mcp.ErrNoData, "No tab is being tracked. Navigate to a page first.",
		"Open a page in the browser, or call interact(what='navigate', url='...').",
		opts...,
	), true
}

// requireString returns (resp, true) if value is empty, short-circuiting the caller.
// Usage: if resp, blocked := requireString(req, params.Name, "name", "Add the 'name' parameter"); blocked { return resp }
func requireString(req JSONRPCRequest, value, paramName, hint string) (JSONRPCResponse, bool) {
	if value == "" {
		return mcp.Fail(req, mcp.ErrMissingParam,
			fmt.Sprintf("Required parameter '%s' is missing", paramName),
			hint, mcp.WithParam(paramName)), true
	}
	return JSONRPCResponse{}, false
}

// requirePositiveInt returns (resp, true) if value is not a positive integer, short-circuiting the caller.
// Usage: if resp, blocked := requirePositiveInt(req, params.Count, "count", "Add a positive 'count'"); blocked { return resp }
func requirePositiveInt(req JSONRPCRequest, value int, paramName, hint string) (JSONRPCResponse, bool) {
	if value <= 0 {
		return mcp.Fail(req, mcp.ErrMissingParam,
			fmt.Sprintf("Required parameter '%s' must be a positive integer", paramName),
			hint, mcp.WithParam(paramName)), true
	}
	return JSONRPCResponse{}, false
}

// requireOneOf returns (resp, true) if value is not in validValues, short-circuiting the caller.
// Usage: if resp, blocked := requireOneOf(req, params.Mode, "mode", []string{"a","b"}, "Use a valid mode"); blocked { return resp }
func requireOneOf(req JSONRPCRequest, value string, paramName string, validValues []string, hint string) (JSONRPCResponse, bool) {
	for _, v := range validValues {
		if value == v {
			return JSONRPCResponse{}, false
		}
	}
	return mcp.Fail(req, mcp.ErrMissingParam,
		fmt.Sprintf("Parameter '%s' must be one of: %s", paramName, strings.Join(validValues, ", ")),
		hint, mcp.WithParam(paramName)), true
}

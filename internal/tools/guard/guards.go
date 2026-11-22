// Purpose: Implements pilot/extension/csp/tab-tracking gate checks for tool handlers.
// Why: Keeps runtime precondition checks and recovery hints isolated from error alias definitions.

package toolguard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/sync/runtime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/mcp"
)

// DefaultExtensionReadinessTimeout bounds cold-start extension recovery.
const DefaultExtensionReadinessTimeout = 5 * time.Second

// Check is the canonical browser-runtime precondition function contract.
type Check func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool)

// Guards owns browser-runtime preconditions and their diagnostic responses.
type Guards struct {
	extension                 *syncruntime.Runtime
	shutdownCtx               context.Context
	extensionReadinessTimeout time.Duration
}

// New constructs runtime guards over the extension connection owner; a nil
// runtime (degraded composition) makes every guard report unavailable.
func New(extension *syncruntime.Runtime, shutdownCtx context.Context, extensionReadinessTimeout time.Duration) *Guards {
	return &Guards{
		extension:                 extension,
		shutdownCtx:               shutdownCtx,
		extensionReadinessTimeout: extensionReadinessTimeout,
	}
}

// SetExtensionReadinessTimeout overrides the cold-start wait duration.
func (g *Guards) SetExtensionReadinessTimeout(timeout time.Duration) {
	g.extensionReadinessTimeout = timeout
}

// InjectCSPBlockedActions adds CSP-blocked action guidance to JSON tool results.
func (g *Guards) InjectCSPBlockedActions(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	if g.extension == nil {
		return resp
	}
	restricted, level := g.extension.GetCSPStatus()
	if !restricted {
		return resp
	}
	actions, reason := syncruntime.CSPBlockedActions(level)
	if actions == nil {
		return resp
	}
	return mcp.MutateResultPayload(resp, func(data map[string]any) bool {
		data["blocked_actions"] = actions
		data["blocked_reason"] = reason
		return true
	})
}

// DiagnosticHintString renders the runtime state that explains guard failures.
func (g *Guards) DiagnosticHintString() string {
	if g.extension == nil {
		return "capture=unavailable"
	}
	extConnected := g.extension.IsExtensionConnected()
	pilotEnabled := g.extension.IsPilotEnabled()
	pilotState := ""
	if status, ok := g.extension.GetPilotStatus().(map[string]any); ok {
		if state, ok := status["state"].(string); ok {
			pilotState = state
		}
		if effective, ok := status["enabled"].(bool); ok {
			pilotEnabled = effective
		}
	}
	enabled, tabID, tabURL := g.extension.GetTrackingStatus()

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
	cspRestricted, cspLevel := g.extension.GetCSPStatus()
	if cspRestricted {
		parts = append(parts, fmt.Sprintf("csp=RESTRICTED(%s)", cspLevel))
	} else {
		parts = append(parts, "csp=clear")
	}
	return "Current state: " + strings.Join(parts, ", ")
}

func (g *Guards) DiagnosticHint() func(*mcp.StructuredError) {
	return mcp.WithHint(g.DiagnosticHintString())
}

// requirePilot returns (resp, true) if AI Web Pilot is disabled, short-circuiting the caller.
// Usage: if resp, blocked := g.Guards.RequirePilot(req); blocked { return resp }
func (g *Guards) RequirePilot(req mcp.JSONRPCRequest, extraOpts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	if g.extension != nil && g.extension.IsPilotActionAllowed() {
		return mcp.JSONRPCResponse{}, false
	}
	opts := append([]func(*mcp.StructuredError){
		g.DiagnosticHint(),
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
// DefaultExtensionReadinessTimeout for the extension to connect before giving up.
// Usage: if resp, blocked := g.Guards.RequireExtension(req); blocked { return resp }
func (g *Guards) RequireExtension(req mcp.JSONRPCRequest, extraOpts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	timeout := g.extensionReadinessTimeout
	if timeout <= 0 {
		timeout = DefaultExtensionReadinessTimeout
	}
	// Use shutdownCtx so the wait aborts promptly when the server shuts down,
	// preventing goroutine leaks. Falls back to context.Background() if the
	// handler was constructed without a shutdown context (e.g., in tests).
	ctx := g.shutdownCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if g.extension != nil && g.extension.WaitForExtensionConnected(ctx, timeout) {
		return mcp.JSONRPCResponse{}, false
	}
	opts := append([]func(*mcp.StructuredError){
		g.DiagnosticHint(),
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
// Usage: if resp, blocked := g.Guards.RequireCSPClear(req, world); blocked { return resp }
func (g *Guards) RequireCSPClear(req mcp.JSONRPCRequest, world string) (mcp.JSONRPCResponse, bool) {
	// Only MAIN world execution is blocked by page CSP.
	// ISOLATED world runs in the extension's security context (bypasses page CSP).
	// AUTO tries MAIN first, then falls back to ISOLATED/structured — the extension handles this.
	if world != "main" {
		return mcp.JSONRPCResponse{}, false
	}
	if g.extension == nil {
		return mcp.JSONRPCResponse{}, false
	}
	restricted, level := g.extension.GetCSPStatus()
	if !restricted {
		return mcp.JSONRPCResponse{}, false
	}
	// Recovery template: LLM should re-send its original call with world='auto'.
	// The 'script' param is intentionally omitted — the LLM fills it from its original call.
	return mcp.Fail(req, mcp.ErrExtError,
		fmt.Sprintf("Page CSP blocks MAIN world script execution (level: %s). Use world='auto' or world='isolated' to bypass.", level),
		"Retry with world='auto' (falls back to isolated/structured), world='isolated' (DOM access, no page JS), or use DOM primitives (click, type).",
		g.DiagnosticHint(),
		mcp.WithRecoveryToolCall(map[string]any{
			"tool":      "interact",
			"arguments": map[string]any{"what": "execute_js", "world": "auto"},
		}),
	), true
}

// requireTabTracking returns (resp, true) if no tab is being tracked,
// short-circuiting the caller with an immediate structured error (~5ms) instead of
// queuing a command that would time out or target the wrong tab.
// Usage: if resp, blocked := g.Guards.RequireTabTracking(req); blocked { return resp }
func (g *Guards) RequireTabTracking(req mcp.JSONRPCRequest, extraOpts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	enabled := false
	if g.extension != nil {
		enabled, _, _ = g.extension.GetTrackingStatus()
	}
	if enabled {
		return mcp.JSONRPCResponse{}, false
	}
	opts := append([]func(*mcp.StructuredError){
		g.DiagnosticHint(),
		mcp.WithRetryable(true),
		mcp.WithRetryAfterMs(2000),
	}, extraOpts...)
	return mcp.Fail(req, mcp.ErrNoData, "No tab is being tracked. Navigate to a page first.",
		"Open a page in the browser, or call interact(what='navigate', url='...').",
		opts...,
	), true
}

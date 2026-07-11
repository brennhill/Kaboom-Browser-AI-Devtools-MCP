// deps.go — Dependency injection for the interacthandler sub-package.
// Purpose: Declares the external dependencies interact handlers need from the main package.
// Why: Decouples interact handlers from the main package's god object without circular imports.

package interacthandler

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// GuardCheck mirrors the main package's guardCheck type.
type GuardCheck = func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool)

// Deps holds all external dependencies interact handlers need from the caller.
type Deps struct {
	// -- Gate checks --

	// RequirePilot checks that pilot mode is enabled.
	RequirePilot GuardCheck

	// RequireExtension checks that the extension is connected.
	RequireExtension GuardCheck

	// RequireTabTracking checks that tab tracking is active.
	RequireTabTracking GuardCheck

	// RequireCSPClear checks CSP restrictions for a given world.
	RequireCSPClear func(req mcp.JSONRPCRequest, world string) (mcp.JSONRPCResponse, bool)

	// -- Command dispatch --

	// EnqueuePendingQuery queues a command for the extension.
	EnqueuePendingQuery func(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool)

	// MaybeWaitForCommand waits for a command result or returns queued status.
	MaybeWaitForCommand func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse

	// -- Capture store --

	// Capture returns the capture store.
	Capture func() *capture.Store

	// -- Recording --

	// RecordAIAction records an AI-driven action to the enhanced actions buffer.
	RecordAIAction func(action, url string, extra map[string]any)

	// RecordAIEnhancedAction records a fully populated AI-driven action.
	RecordAIEnhancedAction func(action capture.EnhancedAction)

	// RecordDOMPrimitiveAction records a DOM primitive action for reproduction.
	RecordDOMPrimitiveAction func(action, selector, text, value string)

	// -- Cross-tool dispatch --

	// ToolInteract dispatches an interact request (used by batch for nested calls).
	ToolInteract func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// ToolAnalyze dispatches an analyze request (used by a11y+SARIF workflow).
	ToolAnalyze func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// ToolExportSARIF dispatches a SARIF export request.
	ToolExportSARIF func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// -- Response enrichment --

	// EnrichNavigateResponse appends page content to a navigate response.
	EnrichNavigateResponse func(resp mcp.JSONRPCResponse, req mcp.JSONRPCRequest, tabID int) mcp.JSONRPCResponse

	// InjectCSPBlockedActions adds CSP-blocked action warnings to a response.
	InjectCSPBlockedActions func(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse

	// -- Screenshot/observe proxies --

	// GetScreenshot captures a screenshot via the observe tool.
	GetScreenshot func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// GetPageInfo returns page info via the observe tool.
	GetPageInfo func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// -- Annotation store --

	// MarkDrawStarted signals that draw mode has been initiated.
	MarkDrawStarted func()

	// -- Server info --

	// GetListenPort returns the daemon's listening port.
	GetListenPort func() int

	// -- Evidence capture --

	// DefaultEvidenceCapture captures an evidence screenshot.
	DefaultEvidenceCapture func(clientID string) EvidenceShot

	// -- Session store --

	// RequireSessionStore checks that the session store is available.
	RequireSessionStore func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool)

	// DiagnosticHint returns a StructuredError option for diagnostic hints.
	DiagnosticHint func() func(*mcp.StructuredError)

	// GetRedactionEngine returns the redaction engine (may be nil).
	GetRedactionEngine func() RedactionEngine

	// GetCommandResult retrieves a command result by correlation ID.
	GetCommandResult func(correlationID string) (*queries.CommandResult, bool)

	// -- Shared concurrency --

	// ReplayMu is the shared mutex for batch/replay serialization.
	// Points to the same mutex used by sequence replay in the main package.
	ReplayMu *sync.Mutex
}

// RedactionEngine mirrors the main package's RedactionEngine interface.
type RedactionEngine interface {
	RedactMapValues(m map[string]any) map[string]any
}

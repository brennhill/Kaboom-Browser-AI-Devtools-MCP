// deps.go — Dependency injection and MCP boundary aliases for toolinteract.
// Purpose: Declares external seams and package-local protocol helpers.
// Why: Decouples handlers from the main package without circular imports.

package toolinteract

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/interactstate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/interactupload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
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

// NewUploadInteractHandler creates an upload handler with narrowed dependencies.
func NewUploadInteractHandler(deps *Deps, actionHandler *InteractActionHandler) *interactupload.Handler {
	return interactupload.New(&interactupload.Deps{
		RequirePilot: func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
			return deps.RequirePilot(req, opts...)
		},
		RequireExtension: func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
			return deps.RequireExtension(req, opts...)
		},
		RequireTabTracking: func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
			return deps.RequireTabTracking(req, opts...)
		},
		EnqueuePendingQuery: func(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
			return deps.EnqueuePendingQuery(req, query, timeout)
		},
		RecordAIAction: func(action, url string, extra map[string]any) {
			deps.RecordAIAction(action, url, extra)
		},
	}, actionHandler)
}

// NewStateInteractHandler creates a state handler with narrowed dependencies.
//
// The narrow interactstate.Deps is built here rather than by the caller, and every
// field forwards through the wide *Deps pointer instead of copying the function
// value out of it — so a test that swaps a seam on the shared Deps after
// construction still wins, exactly as it did when this handler read h.deps directly.
func NewStateInteractHandler(deps *Deps, store *persistence.SessionStore) *interactstate.Handler {
	return interactstate.New(&interactstate.Deps{
		IsPilotActionAllowed: func() bool { return deps.Capture().IsPilotActionAllowed() },
		IsExtensionConnected: func() bool { return deps.Capture().IsExtensionConnected() },
		GetTrackingStatus:    func() (bool, int, string) { return deps.Capture().GetTrackingStatus() },
		GetTrackedTabTitle:   func() string { return deps.Capture().GetTrackedTabTitle() },
		WaitForCommand: func(correlationID string, timeout time.Duration) (*queries.CommandResult, bool) {
			return deps.Capture().WaitForCommand(correlationID, timeout)
		},
		EnqueuePendingQuery: func(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
			return deps.EnqueuePendingQuery(req, query, timeout)
		},
		RecordAIAction: func(action, url string, extra map[string]any) {
			deps.RecordAIAction(action, url, extra)
		},
		RequireSessionStore: func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
			return deps.RequireSessionStore(req)
		},
		DiagnosticHint: func() func(*mcp.StructuredError) { return deps.DiagnosticHint() },
		// interactstate asks for a total Redact; the nil-engine case is the host's
		// to answer, so the "no engine configured" branch stays here.
		Redact: func(m map[string]any) map[string]any {
			if re := deps.GetRedactionEngine(); re != nil {
				return re.RedactMapValues(m)
			}
			return m
		},
	}, store)
}

type JSONRPCRequest = mcp.JSONRPCRequest
type JSONRPCResponse = mcp.JSONRPCResponse
type MCPToolResult = mcp.MCPToolResult
type MCPContentBlock = mcp.MCPContentBlock
type StructuredError = mcp.StructuredError

const JSONRPCVersion = mcp.JSONRPCVersion

const (
	ErrInvalidJSON          = mcp.ErrInvalidJSON
	ErrMissingParam         = mcp.ErrMissingParam
	ErrInvalidParam         = mcp.ErrInvalidParam
	ErrUnknownMode          = mcp.ErrUnknownMode
	ErrPathNotAllowed       = mcp.ErrPathNotAllowed
	ErrNotInitialized       = mcp.ErrNotInitialized
	ErrNoData               = mcp.ErrNoData
	ErrCodePilotDisabled    = mcp.ErrCodePilotDisabled
	ErrOsAutomationDisabled = mcp.ErrOsAutomationDisabled
	ErrRateLimited          = mcp.ErrRateLimited
	ErrCursorExpired        = mcp.ErrCursorExpired
	ErrExtTimeout           = mcp.ErrExtTimeout
	ErrExtError             = mcp.ErrExtError
	ErrQueueFull            = mcp.ErrQueueFull
	ErrInternal             = mcp.ErrInternal
	ErrMarshalFailed        = mcp.ErrMarshalFailed
	ErrExportFailed         = mcp.ErrExportFailed
)

var (
	succeed   = mcp.Succeed
	fail      = mcp.Fail
	parseArgs = mcp.ParseArgs
)

func requireString(req JSONRPCRequest, value, paramName, hint string) (JSONRPCResponse, bool) {
	if value != "" {
		return JSONRPCResponse{}, false
	}
	return fail(req, ErrMissingParam,
		"Required parameter '"+paramName+"' is missing",
		hint,
		withParam(paramName)), true
}

func lenientUnmarshal(args json.RawMessage, v any) {
	mcp.LenientUnmarshal(args, v)
}

func buildQueryParams(fields map[string]any) json.RawMessage {
	return mcp.SafeMarshal(fields, "{}")
}

func safeMarshal(v any, fallback string) json.RawMessage {
	return mcp.SafeMarshal(v, fallback)
}

func withParam(p string) func(*StructuredError)    { return mcp.WithParam(p) }
func withHint(h string) func(*StructuredError)     { return mcp.WithHint(h) }
func withAction(a string) func(*StructuredError)   { return mcp.WithAction(a) }
func withSelector(s string) func(*StructuredError) { return mcp.WithSelector(s) }
func withRetryable(retryable bool) func(*StructuredError) {
	return mcp.WithRetryable(retryable)
}
func withRetryAfterMs(ms int) func(*StructuredError) { return mcp.WithRetryAfterMs(ms) }
func withFinal(final bool) func(*StructuredError)    { return mcp.WithFinal(final) }
func withRecoveryToolCall(toolCall map[string]any) func(*StructuredError) {
	return mcp.WithRecoveryToolCall(toolCall)
}

func checkGuards(req JSONRPCRequest, guards ...GuardCheck) (JSONRPCResponse, bool) {
	for _, guard := range guards {
		if resp, blocked := guard(req); blocked {
			return resp, true
		}
	}
	return JSONRPCResponse{}, false
}

func checkGuardsWithOpts(req JSONRPCRequest, opts []func(*StructuredError), guards ...GuardCheck) (JSONRPCResponse, bool) {
	for _, guard := range guards {
		if resp, blocked := guard(req, opts...); blocked {
			return resp, true
		}
	}
	return JSONRPCResponse{}, false
}

func mutateToolResult(resp JSONRPCResponse, fn func(*MCPToolResult)) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	fn(&result)
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

func appendWarningsToResponse(resp JSONRPCResponse, warnings []string) JSONRPCResponse {
	return mcp.AppendWarningsToResponse(resp, warnings)
}

var newCorrelationID = toolresp.NewCorrelationID

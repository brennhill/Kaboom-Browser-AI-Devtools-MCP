// tools_interact_command_builder.go — Fluent builder for interact command dispatch.
// Why: Eliminates the repeated correlate→arm→guard→enqueue→wait boilerplate across 30+ interact handlers.
// Docs: docs/core/common-patterns.md

package toolinteract

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// commandBuilder provides a fluent API for the common interact handler sequence:
//  1. Run guard checks (requirePilot, requireExtension, requireTabTracking, etc.)
//  2. Generate a correlation ID with a prefix
//  3. Arm evidence for the command
//  4. Build or set query params
//  5. Enqueue a pending query
//  6. Optionally record an AI action
//  7. Wait for the command result via MaybeWaitForCommand
//
// Usage:
//
//	return h.newCommand("highlight").
//	    correlationPrefix("highlight").
//	    reason("highlight").
//	    queryType("highlight").
//	    queryParams(args).
//	    tabID(params.TabID).
//	    guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
//	    recordAction("highlight", "", map[string]any{"selector": params.Selector}).
//	    queuedMessage("Highlight queued").
//	    execute(req, args)
type commandBuilder struct {
	handler *InteractActionHandler

	// Identity
	name string // descriptive name (for debugging; not used in output)

	// Correlation
	corrPrefix string // prefix for newCorrelationID
	reasonStr  string // reason for armEvidenceForCommand

	// Query
	qType    string          // pending query type (e.g. "execute", "browser_action", "dom_action")
	qParams  json.RawMessage // serialized query params; nil = use waitArgs from execute()
	qTabID   int             // tab ID for the pending query
	qTimeout time.Duration   // enqueue timeout; zero = queries.AsyncCommandTimeout

	// Guards
	guardFns  []toolguard.Check
	guardOpts []func(*mcp.StructuredError) // optional opts passed to checkGuardsWithOpts

	// AI recording (optional)
	doRecord  bool
	recAction string
	recURL    string
	recExtra  map[string]any

	// CSP guard (optional)
	cspWorld string // world value for requireCSPClear; empty = skip

	// Pre-enqueue callback (optional). Called after correlation ID is generated
	// but before the query is enqueued. Used for side effects that need the
	// correlation ID (e.g. stashPerfSnapshotImpl).
	preEnqueueFn func(correlationID string)

	// Post-enqueue callback (optional). Called after the query is successfully
	// enqueued but before MaybeWaitForCommand. Used for recording actions with
	// non-standard signatures (for example DOM primitive recording).
	postEnqueueFn func()

	// Response
	queuedMsg string // message for MaybeWaitForCommand when command is async
}

// newCommand creates a new commandBuilder bound to the interactActionHandler.
// The name is descriptive only (for debugging/logging).
func (h *InteractActionHandler) newCommand(name string) *commandBuilder {
	return &commandBuilder{
		handler: h,
		name:    name,
	}
}

// correlationPrefix sets the prefix for the generated correlation ID.
func (b *commandBuilder) correlationPrefix(prefix string) *commandBuilder {
	b.corrPrefix = prefix
	return b
}

// reason sets the reason string passed to armEvidenceForCommand.
func (b *commandBuilder) reason(r string) *commandBuilder {
	b.reasonStr = r
	return b
}

// queryType sets the PendingQuery.Type field.
func (b *commandBuilder) queryType(t string) *commandBuilder {
	b.qType = t
	return b
}

// queryParams sets pre-serialized query parameters.
func (b *commandBuilder) queryParams(p json.RawMessage) *commandBuilder {
	b.qParams = p
	return b
}

// buildParams constructs query parameters with the package's empty-object fallback policy.
func (b *commandBuilder) buildParams(m map[string]any) *commandBuilder {
	b.qParams = marshalQueryParams(m)
	return b
}

// tabID sets the tab ID for the pending query.
func (b *commandBuilder) tabID(id int) *commandBuilder {
	b.qTabID = id
	return b
}

// guards adds guard checks that run before the command is enqueued.
// Guards are run in order; the first blocking guard short-circuits.
func (b *commandBuilder) guards(fns ...toolguard.Check) *commandBuilder {
	b.guardFns = append(b.guardFns, fns...)
	return b
}

// guardsWithOpts adds guard checks with mcp.StructuredError options.
// This is used by handlers like handleDOMPrimitive that need to pass
// contextOpts (action, selector) through to guard error responses.
// Note: opts are accumulated, not replaced — multiple calls are safe.
func (b *commandBuilder) guardsWithOpts(opts []func(*mcp.StructuredError), fns ...toolguard.Check) *commandBuilder {
	b.guardOpts = append(b.guardOpts, opts...)
	b.guardFns = append(b.guardFns, fns...)
	return b
}

// preEnqueue registers a callback invoked after correlation ID generation but before enqueue.
// Useful for side effects like stashPerfSnapshotImpl that need the correlation ID.
func (b *commandBuilder) preEnqueue(fn func(correlationID string)) *commandBuilder {
	b.preEnqueueFn = fn
	return b
}

// postEnqueue registers a callback invoked after successful enqueue but before MaybeWaitForCommand.
// Used for recording actions with non-standard signatures (for example DOM primitives).
func (b *commandBuilder) postEnqueue(fn func()) *commandBuilder {
	b.postEnqueueFn = fn
	return b
}

// cspGuard adds a CSP check for the given world after other guards.
// Only world="main" is blocked — "auto" and "isolated" bypass page CSP.
func (b *commandBuilder) cspGuard(world string) *commandBuilder {
	b.cspWorld = world
	return b
}

// recordAction configures AI action recording after the command is enqueued.
func (b *commandBuilder) recordAction(action, url string, extra map[string]any) *commandBuilder {
	b.doRecord = true
	b.recAction = action
	b.recURL = url
	b.recExtra = extra
	return b
}

// queuedMessage sets the message shown when the command is async (queued).
func (b *commandBuilder) queuedMessage(msg string) *commandBuilder {
	b.queuedMsg = msg
	return b
}

// execute runs the full command sequence: guards → correlate → arm → enqueue → record → wait.
// waitArgs is the original args passed to MaybeWaitForCommand for sync/background resolution.
func (b *commandBuilder) execute(req mcp.JSONRPCRequest, waitArgs json.RawMessage) mcp.JSONRPCResponse {
	resp, _ := b.executeWithCorrelation(req, waitArgs)
	return resp
}

// executeWithCorrelation is like execute but also returns the generated correlation ID.
// Useful for handlers that need the correlation ID for post-processing (e.g. element index).
// Returns empty string if guards blocked before correlation ID generation.
func (b *commandBuilder) executeWithCorrelation(req mcp.JSONRPCRequest, waitArgs json.RawMessage) (mcp.JSONRPCResponse, string) {
	// 0. Validate required fields
	if b.corrPrefix == "" {
		b.corrPrefix = b.name // fall back to builder name
	}
	if b.qType == "" {
		return mcp.Fail(req, mcp.ErrMissingParam, "commandBuilder: queryType is required", "Set queryType before calling execute"), ""
	}

	// 1. Run guards
	if len(b.guardOpts) > 0 {
		if resp, blocked := checkGuardsWithOpts(req, b.guardOpts, b.guardFns...); blocked {
			return resp, ""
		}
	} else if len(b.guardFns) > 0 {
		if resp, blocked := checkGuards(req, b.guardFns...); blocked {
			return resp, ""
		}
	}

	// 1b. Run CSP guard if configured
	if b.cspWorld != "" {
		if resp, blocked := b.handler.deps.RequireCSPClear(req, b.cspWorld); blocked {
			return resp, ""
		}
	}

	// 2. Generate correlation ID and arm evidence
	correlationID := toolresp.NewCorrelationID(b.corrPrefix)
	b.handler.ArmEvidenceForCommand(correlationID, b.reasonStr, waitArgs, req.ClientID)

	// 2b. Pre-enqueue callback (e.g. stash perf snapshot)
	if b.preEnqueueFn != nil {
		b.preEnqueueFn(correlationID)
	}

	// 3. Resolve query params
	params := b.qParams
	if params == nil {
		params = waitArgs
	}

	// 4. Resolve timeout
	timeout := b.qTimeout
	if timeout == 0 {
		timeout = queries.AsyncCommandTimeout
	}

	// 5. Enqueue pending query
	query := queries.PendingQuery{
		Type:          b.qType,
		Params:        params,
		TabID:         b.qTabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := b.handler.deps.EnqueuePendingQuery(req, query, timeout); blocked {
		return enqueueResp, correlationID
	}

	// 6. Record AI action (optional)
	if b.doRecord {
		b.handler.deps.RecordAIAction(b.recAction, b.recURL, b.recExtra)
	}

	// 6b. Post-enqueue callback (for example DOM primitive recording)
	if b.postEnqueueFn != nil {
		b.postEnqueueFn()
	}

	// 7. Wait for command
	return b.handler.deps.MaybeWaitForCommand(req, correlationID, waitArgs, b.queuedMsg), correlationID
}

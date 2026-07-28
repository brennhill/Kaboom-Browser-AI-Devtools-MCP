// routing.go — Shared alias resolution and generic dispatch for all MCP tools.

package toolrouting

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Handler is the unified function signature for tool mode handlers.
// All five tools (observe, analyze, configure, generate, interact) use this signature.
type Handler[H any] func(h H, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

// describeCapabilitiesRecovery points callers at the canonical tool-mode registry.
func describeCapabilitiesRecovery(toolName string) func(*mcp.StructuredError) {
	return mcp.WithRecoveryToolCall(map[string]any{
		"tool": "configure",
		"arguments": map[string]any{
			"what": "describe_capabilities",
			"tool": toolName,
		},
	})
}

// Registry bundles the handler map and metadata for a tool.
type Registry[H any] struct {
	Handlers   map[string]Handler[H]
	Resolution Resolution
	// PreDispatch is called after mode resolution but before handler dispatch.
	// Returns modified args and optional response (non-nil short-circuits dispatch).
	PreDispatch func(h H, req mcp.JSONRPCRequest, args json.RawMessage, what string) (json.RawMessage, *mcp.JSONRPCResponse)
	// PostDispatch is called after the handler returns, before alias warning.
	PostDispatch func(h H, req mcp.JSONRPCRequest, resp mcp.JSONRPCResponse, what string) mcp.JSONRPCResponse
}

// Dispatch resolves the mode, looks up the handler, and dispatches.
// Handles the resolve→lookup→not-found→call pattern shared by all five tools.
func Dispatch[H any](h H, req mcp.JSONRPCRequest, args json.RawMessage, reg Registry[H]) mcp.JSONRPCResponse {
	what, errResp := resolveToolMode(req, args, reg.Resolution)
	if errResp != nil {
		return *errResp
	}

	handler, ok := reg.Handlers[what]
	if !ok {
		validModes := reg.Resolution.ValidModes
		resp := mcp.Fail(req, mcp.ErrUnknownMode, "Unknown "+reg.Resolution.ToolName+" mode: "+what,
			"Use a valid mode from the 'what' enum", mcp.WithParam("what"), mcp.WithHint("Valid values: "+validModes), describeCapabilitiesRecovery(reg.Resolution.ToolName))
		return resp
	}

	if reg.PreDispatch != nil {
		var preResp *mcp.JSONRPCResponse
		args, preResp = reg.PreDispatch(h, req, args, what)
		if preResp != nil {
			return *preResp
		}
	}

	resp := handler(h, req, args)

	if reg.PostDispatch != nil {
		resp = reg.PostDispatch(h, req, resp, what)
	}

	return resp
}

// Resolution bundles context needed for mode resolution error messages.
type Resolution struct {
	ToolName   string // For error messages (e.g. "observe", "analyze")
	ValidModes string // Sorted comma-separated list for hints
}

// resolveToolMode extracts the canonical 'what' parameter.
func resolveToolMode(
	req mcp.JSONRPCRequest,
	args json.RawMessage,
	res Resolution,
) (what string, errResp *mcp.JSONRPCResponse) {
	if len(args) > 0 {
		var params struct {
			What string `json:"what"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			resp := mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
			return "", &resp
		}
		what = params.What
	}

	if what == "" {
		resp := mcp.Fail(req, mcp.ErrMissingParam,
			"Required parameter 'what' is missing",
			"Add the 'what' parameter and call again",
			mcp.WithParam("what"),
			mcp.WithHint("Valid values: "+res.ValidModes))
		return "", &resp
	}
	return what, nil
}

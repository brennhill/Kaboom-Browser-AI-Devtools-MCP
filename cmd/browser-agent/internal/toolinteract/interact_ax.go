// interact_ax.go — The `find` action: natural-language element lookup over the
// accessibility tree.
//
// Why this exists: every other targeting path in this package starts from a CSS selector or
// a DOM-derived index. Neither can name a control whose semantics live in ARIA rather than
// markup — a canvas-drawn widget, an aria-label that differs from the visible text, a role
// overridden on a plain div. `find` asks Chrome's accessibility tree instead, which is the
// same view assistive technology sees.
//
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// axFindTimeoutDefaultMs bounds a full accessibility snapshot plus geometry resolution for
// the ranked candidates. A large page has thousands of AX nodes.
const axFindTimeoutDefaultMs = 10_000

// axFindTimeoutMaxMs caps what a caller may ask for, so one query cannot occupy the
// extension queue indefinitely.
const axFindTimeoutMaxMs = 30_000

// HandleFind resolves a natural-language description to accessibility candidates.
//
// Returns ranked candidates rather than one answer: an ambiguous query must stay ambiguous
// in the response, or the agent has no way to tell "the only match" from "the first of
// several plausible matches" and will blind-click whichever came back first.
func (h *PageActions) HandleFind(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Query     string `json:"query"`
		TabID     int    `json:"tab_id,omitempty"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Query == "" {
		return mcp.Fail(req, mcp.ErrMissingParam,
			"find requires a 'query' describing the element",
			"Pass query='add to cart button' or query='search bar'.",
			mcp.WithParam("query"))
	}

	if params.TimeoutMs <= 0 {
		params.TimeoutMs = axFindTimeoutDefaultMs
	}
	if params.TimeoutMs > axFindTimeoutMaxMs {
		params.TimeoutMs = axFindTimeoutMaxMs
	}

	return h.runtime.newCommand("ax_find").
		correlationPrefix("ax_find").
		reason("find").
		queryType("ax_find").
		buildParams(map[string]any{
			"query":      params.Query,
			"timeout_ms": params.TimeoutMs,
		}).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		queuedMessage("find queued").
		execute(req, args)
}

// Purpose: Handles get_readable, get_markdown, and page_summary content extraction via structured extension query types.
// Why: Replaces unsafe IIFE script injection with CSP-safe content-script message-passing for text extraction.
// Docs: docs/features/feature/interact-explore/index.md
// Implements get_readable, get_markdown, and page_summary using dedicated query types
// routed through content script message-passing (CSP-safe, ISOLATED world).
// Issue #257: Moved from "execute" query type with embedded IIFE scripts to
// structured query types that the content script handles directly.
package interacthandler

import (
	"encoding/json"
	"time"
)

const (
	// navigatePageSummaryWait is the time to wait for the page summary content
	// extraction after navigation. The extension-side query uses a 4s timeout,
	// so this must be slightly longer to allow for round-trip overhead.
	navigatePageSummaryWait = 5 * time.Second
)

// handleContentExtraction is the shared handler for get_readable, get_markdown, and page_summary.
// All three use the same pattern: gate checks, timeout validation, create a pending query with
// the dedicated query type, and wait for the content script to respond.
func (h *InteractActionHandler) HandleContentExtraction(req JSONRPCRequest, args json.RawMessage, queryType string, correlationPrefix string) JSONRPCResponse {
	var params struct {
		TabID     int `json:"tab_id,omitempty"`
		TimeoutMs int `json:"timeout_ms,omitempty"`
	}
	lenientUnmarshal(args, &params)

	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 10_000
	}
	if params.TimeoutMs > 30_000 {
		params.TimeoutMs = 30_000
	}

	return h.newCommand(queryType).
		correlationPrefix(correlationPrefix).
		reason(queryType).
		queryType(queryType).
		buildParams(map[string]any{
			"timeout_ms": params.TimeoutMs,
		}).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		queuedMessage(queryType+" queued").
		execute(req, args)
}

func (h *InteractActionHandler) HandleGetReadable(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.HandleContentExtraction(req, args, "get_readable", "readable")
}

func (h *InteractActionHandler) HandleGetMarkdown(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.HandleContentExtraction(req, args, "get_markdown", "markdown")
}

// NavigatePageSummaryWait is exported for use by the main package's enrichNavigateResponse.
const NavigatePageSummaryWait = navigatePageSummaryWait

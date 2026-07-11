// Purpose: Enriches navigate responses with page content extraction.
// Why: Appends page summary, vitals, and metadata to navigate results.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/interacthandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// enrichNavigateResponse appends page content to a successful navigate response.
// Uses the "page_summary" query type to extract text content, headings, and metadata
// via the content script (ISOLATED world, CSP-safe).
func (h *ToolHandler) enrichNavigateResponse(resp JSONRPCResponse, req JSONRPCRequest, tabID int) JSONRPCResponse {
	// Only enrich successful (non-error) responses
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return resp
	}

	// Get current page info from tracking state
	_, _, tabURL := h.capture.GetTrackingStatus()
	tabTitle := h.capture.GetTrackedTabTitle()

	// Get performance vitals
	vitals := h.capture.GetPerformanceSnapshots()

	// Request page summary via dedicated query type (CSP-safe, no eval).
	// Use 4s query timeout to finish before the 5s Go-side wait.
	summaryCorrelationID := newCorrelationID("nav_content")
	summaryParams := buildQueryParams(map[string]any{
		"timeout_ms": 4000,
	})
	summaryQuery := queries.PendingQuery{
		Type:          "page_summary",
		Params:        summaryParams,
		TabID:         tabID,
		CorrelationID: summaryCorrelationID,
	}
	if enqueueResp, blocked := h.EnqueuePendingQuery(req, summaryQuery, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	// Wait for page summary (5s — page should already be loaded).
	// Best-effort enrichment: if extraction fails, navigate still succeeds with empty content.
	var textContent string
	cmd, found := h.capture.WaitForCommand(summaryCorrelationID, interacthandler.NavigatePageSummaryWait)
	if found && cmd.Status != "pending" && cmd.Result != nil {
		var summaryResult map[string]any
		if json.Unmarshal(cmd.Result, &summaryResult) == nil {
			if preview, ok := summaryResult["main_content_preview"].(string); ok {
				textContent = preview
			}
		}
	}

	// Parse existing result text and append enrichment data
	if len(result.Content) > 0 {
		enrichment := map[string]any{
			"url":          tabURL,
			"title":        tabTitle,
			"text_content": textContent,
		}
		if len(vitals) > 0 {
			enrichment["vitals"] = vitals[len(vitals)-1]
		}
		enrichJSON, _ := json.Marshal(enrichment)
		result.Content = append(result.Content, MCPContentBlock{
			Type: "text",
			Text: "Page content:\n" + string(enrichJSON),
		})
	}

	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

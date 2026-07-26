// Purpose: Provides MCP response builders (text, markdown, JSON, error) and safe marshal/unmarshal helpers for tool results.
// Why: Standardizes response shaping across all five tools through a single set of formatting functions.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func safeMarshal(v any, fallback string) json.RawMessage {
	return mcp.SafeMarshal(v, fallback)
}

// buildQueryParams marshals a string-keyed map into JSON for query dispatch.
// Falls back to `{}` on marshal failure (impossible for map[string]any with primitive values).
func buildQueryParams(fields map[string]any) json.RawMessage {
	return safeMarshal(fields, "{}")
}

func lenientUnmarshal(args json.RawMessage, v any) {
	mcp.LenientUnmarshal(args, v)
}

func mcpTextResponse(text string) json.RawMessage {
	return mcp.TextResponse(text)
}

// The response vocabulary is not implemented here: internal/mcp is the source of
// truth for the builders, internal/toolresp for the two it lacks. These names stay
// so the ~80 tool handler files in this package keep their concise call sites.
var (
	succeed     = mcp.Succeed
	succeedText = mcp.SucceedText
	succeedRaw  = toolresp.SucceedRaw
	fail        = mcp.Fail
	failJSON    = toolresp.FailJSON
	parseArgs   = mcp.ParseArgs
)

func mcpJSONErrorResponse(summary string, data any) json.RawMessage {
	return mcp.JSONErrorResponse(summary, data)
}

func mcpJSONResponse(summary string, data any) json.RawMessage {
	return mcp.JSONResponse(summary, data)
}

func appendWarningsToResponse(resp JSONRPCResponse, warnings []string) JSONRPCResponse {
	return mcp.AppendWarningsToResponse(resp, warnings)
}

// mutateToolResult unmarshals the response result into MCPToolResult, applies the
// mutation function, and remarshals. Returns the original response unchanged if
// unmarshal or remarshal fails.
func mutateToolResult(resp JSONRPCResponse, fn func(*MCPToolResult)) JSONRPCResponse {
	return mcp.MutateToolResult(resp, fn)
}

// injectCSPBlockedActions adds blocked_actions and blocked_reason to a JSON
// response when the current page CSP restricts script execution. When CSP is
// clear the response is returned unchanged (zero token cost). (#262)
func (h *ToolHandler) injectCSPBlockedActions(resp JSONRPCResponse) JSONRPCResponse {
	restricted, level := h.capture.GetCSPStatus()
	if !restricted {
		return resp
	}
	actions, reason := capture.CSPBlockedActions(level)
	if actions == nil {
		return resp
	}

	return mutateToolResult(resp, func(r *MCPToolResult) {
		if len(r.Content) == 0 {
			return
		}

		text := r.Content[0].Text
		// Find the JSON object within the text (after the summary line).
		jsonStart := -1
		for i := 0; i < len(text); i++ {
			if text[i] == '{' {
				jsonStart = i
				break
			}
		}
		if jsonStart < 0 {
			return
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
			return
		}

		data["blocked_actions"] = actions
		data["blocked_reason"] = reason

		dataJSON, err := json.Marshal(data)
		if err != nil {
			return
		}

		r.Content[0].Text = text[:jsonStart] + string(dataJSON)
	})
}

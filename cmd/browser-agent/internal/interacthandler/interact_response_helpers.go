// Purpose: Provides shared interact response helpers for queued detection and enrichment.
// Why: Keeps response-shaping logic isolated from interact dispatch orchestration.

package interacthandler

import (
	"encoding/json"
	"strings"
	"time"

	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// isResponseQueued checks if an MCP response is a queued async response.
func isResponseQueued(resp JSONRPCResponse) bool {
	if resp.Result == nil {
		return false
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return false
	}
	if len(result.Content) == 0 {
		return false
	}

	for _, c := range result.Content {
		if c.Type != "text" || len(c.Text) == 0 {
			continue
		}
		text := c.Text
		if idx := strings.Index(text, "\n{"); idx >= 0 {
			text = text[idx+1:]
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(text), &data); err != nil {
			continue
		}
		if status, ok := data["status"].(string); ok && status == "queued" {
			return true
		}
	}
	return false
}

// appendScreenshotToResponse captures a screenshot and appends it as an inline image block.
func (h *InteractActionHandler) AppendScreenshotToResponse(resp JSONRPCResponse, req JSONRPCRequest) JSONRPCResponse {
	screenshotReq := JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: req.ID}
	screenshotResp := h.deps.GetScreenshot(screenshotReq, nil)

	var screenshotResult MCPToolResult
	if err := json.Unmarshal(screenshotResp.Result, &screenshotResult); err != nil {
		return resp // best effort: keep original response on parse failure
	}

	for _, block := range screenshotResult.Content {
		if block.Type != "image" || block.Data == "" {
			continue
		}
		var result MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return resp
		}
		result.Content = append(result.Content, block)
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return resp
		}
		resp.Result = json.RawMessage(resultJSON)
		break
	}

	return resp
}

// appendInteractiveToResponse appends list_interactive text to the response.
func (h *InteractActionHandler) AppendInteractiveToResponse(resp JSONRPCResponse, req JSONRPCRequest) JSONRPCResponse {
	listReq := JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: req.ID, ClientID: req.ClientID}
	listArgs := buildQueryParams(map[string]any{"what": "list_interactive", "visible_only": true})
	listResp := h.HandleListInteractive(listReq, listArgs)

	var listResult MCPToolResult
	if err := json.Unmarshal(listResp.Result, &listResult); err != nil || listResult.IsError {
		return resp
	}

	for _, block := range listResult.Content {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		var result MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return resp
		}
		result.Content = append(result.Content, MCPContentBlock{
			Type: "text",
			Text: "\n--- Interactive Elements ---\n" + block.Text,
		})
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return resp
		}
		resp.Result = json.RawMessage(resultJSON)
		break
	}
	return resp
}

// readPageContext returns a compact page context payload (url/title/tab_id) from observe(what="page").
func (h *InteractActionHandler) readPageContext(req JSONRPCRequest) (map[string]any, bool) {
	pageReq := JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: req.ID}
	pageResp := h.deps.GetPageInfo(pageReq, nil)

	var pageResult MCPToolResult
	if err := json.Unmarshal(pageResp.Result, &pageResult); err != nil || pageResult.IsError {
		return nil, false
	}

	var data map[string]any
	for _, block := range pageResult.Content {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		text := block.Text
		if idx := strings.Index(text, "\n{"); idx >= 0 {
			text = text[idx+1:]
		}
		if err := json.Unmarshal([]byte(text), &data); err == nil {
			break
		}
	}
	if data == nil {
		return nil, false
	}

	out := map[string]any{}
	if url, ok := data["url"].(string); ok && url != "" {
		out["url"] = url
	}
	if title, ok := data["title"].(string); ok && title != "" {
		out["title"] = title
	}
	if tabID, ok := data["tab_id"]; ok {
		out["tab_id"] = tabID
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// appendPageContextToResponse appends a compact page context block to the response.
func (h *InteractActionHandler) AppendPageContextToResponse(resp JSONRPCResponse, req JSONRPCRequest) JSONRPCResponse {
	pageCtx, ok := h.readPageContext(req)
	if !ok {
		return resp
	}

	ctxJSON, err := json.Marshal(pageCtx)
	if err != nil {
		return resp
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	if result.Metadata == nil {
		result.Metadata = map[string]any{}
	}
	result.Metadata["page_context"] = pageCtx

	result.Content = append(result.Content, MCPContentBlock{
		Type: "text",
		Text: "\n--- Page Context ---\n" + string(ctxJSON),
	})
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// appendWorkflowTraceToResponse appends a normalized workflow trace envelope
// into MCP metadata while preserving the existing response shape/content.
func (h *InteractActionHandler) AppendWorkflowTraceToResponse(
	resp JSONRPCResponse,
	workflow string,
	trace []WorkflowStep,
	start time.Time,
	status string,
) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	envelope := act.BuildWorkflowTraceEnvelope(workflow, trace, start, time.Now(), status)
	if result.Metadata == nil {
		result.Metadata = map[string]any{}
	}
	result.Metadata["trace_id"] = envelope.TraceID
	result.Metadata["workflow_trace"] = envelope

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

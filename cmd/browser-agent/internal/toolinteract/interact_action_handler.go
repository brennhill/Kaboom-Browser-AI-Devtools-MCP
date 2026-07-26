// interact_action_handler.go — InteractActionHandler: the type every interact action
// hangs off, its jitter policy, and the response enrichment applied on the way out
// (screenshot, interactive elements, page context, workflow trace).
// Why one file: enrichment is not a subsystem — it is what this handler does to a
// response before returning it, and it reads only h.deps.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// InteractActionHandler handles interact action dispatch, jitter, evidence, and retry contracts.
type InteractActionHandler struct {
	deps *Deps

	// Action jitter: randomized micro-delays before interact actions.
	jitterMu          sync.RWMutex
	actionJitterMaxMs int // max jitter before each interact action (0 = disabled)

	// Optional evidence capture state keyed by correlation_id.
	evidenceMu        sync.Mutex
	evidenceByCommand map[string]*commandEvidenceState

	// Deterministic retry contract metadata keyed by correlation_id.
	retryContractMu sync.Mutex
	retryByCommand  map[string]*commandRetryState

	// Scoped element index registry used by list_interactive/index follow-up actions.
	elementIndexRegistry *elementIndexRegistry
}

// NewInteractActionHandler creates a new InteractActionHandler with the given dependencies.
func NewInteractActionHandler(deps *Deps) *InteractActionHandler {
	return &InteractActionHandler{
		deps:                 deps,
		evidenceByCommand:    make(map[string]*commandEvidenceState),
		retryByCommand:       make(map[string]*commandRetryState),
		elementIndexRegistry: newElementIndexRegistry(),
	}
}

// SetJitter sets the maximum jitter delay in milliseconds.
func (h *InteractActionHandler) SetJitter(maxMs int) {
	h.jitterMu.Lock()
	defer h.jitterMu.Unlock()
	h.actionJitterMaxMs = maxMs
}

// GetJitter returns the current maximum jitter delay in milliseconds.
func (h *InteractActionHandler) GetJitter() int {
	h.jitterMu.RLock()
	defer h.jitterMu.RUnlock()
	return h.actionJitterMaxMs
}

// ReadOnlyInteractActions lists actions that should not have jitter applied.
var ReadOnlyInteractActions = map[string]bool{
	"list_interactive":          true,
	"get_text":                  true,
	"get_value":                 true,
	"get_attribute":             true,
	"query":                     true,
	"screenshot":                true,
	"list_states":               true,
	"state_list":                true,
	"get_readable":              true,
	"get_markdown":              true,
	"explore_page":              true,
	"run_a11y_and_export_sarif": true,
	"wait_for":                  true,
	"wait_for_stable":           true,
	"auto_dismiss_overlays":     true,
	"batch":                     true,
	"highlight":                 true,
	"subtitle":                  true,
	"clipboard_read":            true,
}

// ApplyJitter sleeps for a random duration up to maxMs if jitter is configured.
func (h *InteractActionHandler) ApplyJitter(action string) int {
	if ReadOnlyInteractActions[action] {
		return 0
	}
	h.jitterMu.RLock()
	maxMs := h.actionJitterMaxMs
	h.jitterMu.RUnlock()
	if maxMs <= 0 {
		return 0
	}
	jitterMs := 0
	if maxMs > 0 {
		jitterMs = rand.IntN(maxMs)
	}
	if jitterMs > 0 {
		time.Sleep(time.Duration(jitterMs) * time.Millisecond)
	}
	return jitterMs
}

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

// interact_action_handler.go — InteractActionHandler state, retry policy, jitter,
// and response enrichment applied on the way out.
// Why one file: these policies share the handler's mutex-protected lifecycle state
// and decorate every command response through the same boundary.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/elemindex"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
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
	elementIndexRegistry *elemindex.Registry
}

// NewInteractActionHandler creates a new InteractActionHandler with the given dependencies.
func NewInteractActionHandler(deps *Deps) *InteractActionHandler {
	return &InteractActionHandler{
		deps:                 deps,
		evidenceByCommand:    make(map[string]*commandEvidenceState),
		retryByCommand:       make(map[string]*commandRetryState),
		elementIndexRegistry: elemindex.New(),
	}
}

const maxRetryAttemptsPerStep = 2

type commandRetryState struct {
	Attempt             int
	MaxAttempts         int
	Action              string
	Strategy            string
	StrategyFingerprint string
	ChangedStrategy     bool
	PolicyViolation     string
	ParentCorrelationID string
	CreatedAt           time.Time
}

type retryTerminalDecision struct {
	Terminal bool
	Cause    string
}

func parseRetryParentCorrelationID(args json.RawMessage) string {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	mcp.LenientUnmarshal(args, &params)
	return strings.TrimSpace(params.CorrelationID)
}

func deriveRetryStrategy(action string, args json.RawMessage) (strategy string, fingerprint string) {
	var payload map[string]any
	mcp.LenientUnmarshal(args, &payload)

	fingerprintFields := map[string]any{
		"action": strings.ToLower(strings.TrimSpace(action)),
	}
	for _, key := range []string{
		"selector",
		"scope_selector",
		"scope_rect",
		"annotation_rect",
		"element_id",
		"index",
		"frame",
		"world",
		"text",
		"value",
		"wait_for",
	} {
		if value, ok := payload[key]; ok {
			fingerprintFields[key] = value
		}
	}
	fingerprint = stableMarshalForRetry(fingerprintFields)

	switch {
	case payload["element_id"] != nil:
		return "element_handle", fingerprint
	case payload["scope_selector"] != nil || payload["scope_rect"] != nil || payload["annotation_rect"] != nil:
		return "scoped_selector", fingerprint
	case payload["frame"] != nil:
		return "frame_targeted", fingerprint
	case payload["selector"] != nil:
		return "selector", fingerprint
	case payload["index"] != nil:
		return "indexed", fingerprint
	case payload["world"] != nil:
		return "world_switch", fingerprint
	default:
		return "default", fingerprint
	}
}

func stableMarshalForRetry(value map[string]any) string {
	if value == nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func (h *InteractActionHandler) armRetryContract(correlationID, action string, args json.RawMessage) {
	if h == nil || correlationID == "" {
		return
	}

	if action == "" {
		action = canonicalActionFromInteractArgs(args)
	}
	action = strings.ToLower(strings.TrimSpace(action))
	strategy, fingerprint := deriveRetryStrategy(action, args)
	parentCorrelationID := parseRetryParentCorrelationID(args)

	state := &commandRetryState{
		Attempt:             1,
		MaxAttempts:         maxRetryAttemptsPerStep,
		Action:              action,
		Strategy:            strategy,
		StrategyFingerprint: fingerprint,
		ChangedStrategy:     true,
		ParentCorrelationID: parentCorrelationID,
		CreatedAt:           time.Now(),
	}

	if parentCorrelationID != "" {
		if parent, ok := h.getRetryState(parentCorrelationID); ok {
			state.Attempt = parent.Attempt + 1
			if state.Attempt > state.MaxAttempts {
				state.Attempt = state.MaxAttempts
				state.PolicyViolation = "attempt_limit_exceeded"
			}
			state.ChangedStrategy = state.StrategyFingerprint != parent.StrategyFingerprint
			if !state.ChangedStrategy {
				state.PolicyViolation = "strategy_unchanged"
			}
		} else {
			state.Attempt = 2
			state.PolicyViolation = "parent_context_missing"
		}
	}

	h.storeRetryState(correlationID, state)
}

func (h *InteractActionHandler) getRetryState(correlationID string) (*commandRetryState, bool) {
	h.retryContractMu.Lock()
	defer h.retryContractMu.Unlock()
	state, ok := h.retryByCommand[correlationID]
	return state, ok
}

func (h *InteractActionHandler) storeRetryState(correlationID string, state *commandRetryState) {
	h.retryContractMu.Lock()
	defer h.retryContractMu.Unlock()

	if h.retryByCommand == nil {
		h.retryByCommand = make(map[string]*commandRetryState)
	}
	h.retryByCommand[correlationID] = state
	h.pruneRetryStatesLocked(2048)
}

func (h *InteractActionHandler) pruneRetryStatesLocked(maxEntries int) {
	if len(h.retryByCommand) <= maxEntries {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	for key, state := range h.retryByCommand {
		if oldestKey == "" || state.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = state.CreatedAt
		}
	}
	if oldestKey != "" {
		delete(h.retryByCommand, oldestKey)
	}
}

func deriveRetryReason(responseData map[string]any, fallback string) string {
	if responseData != nil {
		if code, ok := responseData["error_code"].(string); ok && strings.TrimSpace(code) != "" {
			return strings.TrimSpace(code)
		}
		if errorCode, ok := responseData["error"].(string); ok && strings.TrimSpace(errorCode) != "" {
			return strings.TrimSpace(errorCode)
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "unknown"
}

func (h *InteractActionHandler) AttachRetryContext(correlationID string, responseData map[string]any, status string, fallbackReason string) retryTerminalDecision {
	if h == nil || correlationID == "" || responseData == nil {
		return retryTerminalDecision{}
	}

	state, ok := h.getRetryState(correlationID)
	if !ok || state == nil {
		return retryTerminalDecision{}
	}

	reason := deriveRetryReason(responseData, fallbackReason)
	if strings.EqualFold(status, "complete") && reason == "unknown" {
		reason = "success"
	}

	retryContext := map[string]any{
		"attempt":          state.Attempt,
		"max_attempts":     state.MaxAttempts,
		"strategy":         state.Strategy,
		"changed_strategy": state.ChangedStrategy,
		"reason":           reason,
	}
	if state.ParentCorrelationID != "" {
		retryContext["parent_correlation_id"] = state.ParentCorrelationID
	}
	if state.PolicyViolation != "" {
		retryContext["policy_violation"] = state.PolicyViolation
	}

	decision := retryTerminalDecision{}
	failureStatus := status == "error" || status == "timeout" || status == "expired" || status == "cancelled"
	if failureStatus {
		if state.Attempt >= state.MaxAttempts {
			decision.Terminal = true
			decision.Cause = "max_attempts_reached"
		}
		if state.Attempt > 1 && !state.ChangedStrategy {
			decision.Terminal = true
			decision.Cause = "strategy_not_changed"
		}
	}

	retryContext["terminal_stop"] = decision.Terminal
	if decision.Cause != "" {
		retryContext["terminal_cause"] = decision.Cause
	}
	responseData["retry_context"] = retryContext

	if !failureStatus {
		return decision
	}

	if decision.Terminal {
		responseData["terminal"] = true
		responseData["retryable"] = false
		if _, exists := responseData["retry"]; !exists {
			responseData["retry"] = "Terminal after two attempts. Stop retrying this step and report evidence_summary."
		}
		responseData["evidence_summary"] = buildRetryEvidenceSummary(correlationID, reason, retryContext, responseData)
		return decision
	}

	if _, exists := responseData["retryable"]; !exists {
		responseData["retryable"] = true
	}
	if _, exists := responseData["retry"]; !exists {
		responseData["retry"] = "Retry once with a changed strategy (scope_selector/scope_rect/element_id/index/frame/world). If the second attempt fails, stop and report evidence_summary."
	}

	return decision
}

func buildRetryEvidenceSummary(correlationID, reason string, retryContext map[string]any, responseData map[string]any) map[string]any {
	summary := map[string]any{
		"correlation_id": correlationID,
		"failure_reason": reason,
		"next_action":    "Stop retries for this step and report this bundle.",
		"required": []string{
			"command_result",
			"screenshot",
			"scoped_list_interactive_output",
		},
	}

	if retryContext != nil {
		summary["retry_context"] = retryContext
	}
	if responseData != nil {
		if evidence, ok := responseData["evidence"]; ok {
			summary["captured_evidence"] = evidence
		}
		if url, ok := responseData["effective_url"].(string); ok && strings.TrimSpace(url) != "" {
			summary["url"] = url
		} else if url, ok := responseData["resolved_url"].(string); ok && strings.TrimSpace(url) != "" {
			summary["url"] = url
		}
	}
	return summary
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
	listArgs := marshalQueryParams(map[string]any{"what": "list_interactive", "visible_only": true})
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
	trace []act.WorkflowStep,
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

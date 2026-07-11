// Purpose: Implements run_a11y_and_export_sarif workflow and result payload extraction.
// Why: Isolates accessibility+SARIF workflow orchestration from form/navigation workflows.
// Docs: docs/features/feature/form-filling/index.md

package interacthandler

import (
	"encoding/json"
	"strings"
	"time"
)

// handleRunA11yAndExportSARIF runs accessibility audit then exports SARIF in one call.
// Gates (requirePilot, requireExtension, requireTabTracking) are applied by the delegated handlers.
func (h *InteractActionHandler) HandleRunA11yAndExportSARIF(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Scope  string `json:"scope,omitempty"`
		SaveTo string `json:"save_to,omitempty"`
		TabID  int    `json:"tab_id,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	trace := make([]WorkflowStep, 0, 2)
	workflowStart := time.Now()

	// Step 1: Run accessibility audit.
	a11yArgs := buildQueryParams(map[string]any{
		"what":   "accessibility",
		"scope":  params.Scope,
		"tab_id": params.TabID,
	})
	stepStart := time.Now()
	a11yResp := h.deps.ToolAnalyze(req, a11yArgs)
	trace = append(trace, WorkflowStep{
		Action:   "analyze_accessibility",
		Status:   responseStatus(a11yResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
	})
	if isErrorResponse(a11yResp) {
		return workflowResult(req, "run_a11y_and_export_sarif", trace, a11yResp, workflowStart)
	}

	// Step 2: Export as SARIF, reusing successful a11y payload to avoid a second blocking query.
	sarifParams := map[string]any{
		"scope":   params.Scope,
		"save_to": params.SaveTo,
	}
	if a11yResult := extractMCPResponseJSONPayload(a11yResp); len(a11yResult) > 0 {
		sarifParams["a11y_result"] = a11yResult
	}
	sarifArgs, _ := json.Marshal(sarifParams)
	stepStart = time.Now()
	sarifResp := h.deps.ToolExportSARIF(req, sarifArgs)
	trace = append(trace, WorkflowStep{
		Action:   "generate_sarif",
		Status:   responseStatus(sarifResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
	})

	return workflowResult(req, "run_a11y_and_export_sarif", trace, sarifResp, workflowStart)
}

// extractMCPResponseJSONPayload extracts JSON payload from first text block in MCP response.
func extractMCPResponseJSONPayload(resp JSONRPCResponse) json.RawMessage {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || len(result.Content) == 0 {
		return nil
	}

	text := strings.TrimSpace(result.Content[0].Text)
	jsonStart := strings.IndexAny(text, "{[")
	if jsonStart < 0 {
		return nil
	}
	payload := strings.TrimSpace(text[jsonStart:])
	if !json.Valid([]byte(payload)) {
		return nil
	}
	return json.RawMessage(payload)
}

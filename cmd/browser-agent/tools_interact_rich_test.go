// Purpose: Tests for interact rich response formatting.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_rich_test.go — TDD tests for Rich Action Results.
// Tests the daemon-side contract: analyze param parsing, forwarding to
// pending query, schema presence, and top-level field surfacing.
//
// Run: go test ./cmd/browser-agent -run "TestRichAction" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// analyze param: explicit parsing and forwarding
// ============================================

func TestRichAction_AnalyzeInPendingQueryParams(t *testing.T) {
	env := newInteractTestEnv(t)

	// Enable pilot so the request gets queued (not rejected at pilot check)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","analyze":true,"background":true}`)
	if !ok {
		t.Fatal("click with analyze:true should return result")
	}
	if result.IsError {
		t.Fatalf("click with analyze:true + pilot enabled should not error. Got: %s",
			result.Content[0].Text)
	}

	// Extract the queued pending query and verify analyze is in params
	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("Failed to parse pending query params: %v", err)
	}

	analyze, exists := params["analyze"]
	if !exists {
		t.Fatal("analyze field missing from pending query params — it was stripped during forwarding")
	}
	if analyze != true {
		t.Errorf("analyze = %v, want true", analyze)
	}
}

func TestRichAction_AnalyzeFalseNotForwarded(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","analyze":false,"background":true}`)
	if !ok {
		t.Fatal("click with analyze:false should return result")
	}
	if result.IsError {
		t.Fatalf("click with analyze:false + pilot enabled should not error. Got: %s",
			result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("Failed to parse pending query params: %v", err)
	}

	// analyze:false should either be absent or false — not true
	if analyze, exists := params["analyze"]; exists && analyze == true {
		t.Error("analyze:false should not be forwarded as true")
	}
}

func TestRichAction_AnalyzeOmittedByDefault(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok {
		t.Fatal("click without analyze should return result")
	}
	if result.IsError {
		t.Fatalf("click + pilot enabled should not error. Got: %s",
			result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("Failed to parse pending query params: %v", err)
	}

	// analyze should not be present when not specified
	if _, exists := params["analyze"]; exists {
		t.Error("analyze should not be in params when not specified by caller")
	}
}

func TestRichAction_AnalyzeOnNavigationAction(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// refresh doesn't use analyze (it always returns perf_diff)
	// but should not reject it either
	result, ok := env.callInteract(t, `{"what":"refresh","analyze":true,"background":true}`)
	if !ok {
		t.Fatal("refresh with analyze:true should return result")
	}
	if result.IsError {
		t.Fatalf("refresh with analyze:true + pilot enabled should not error. Got: %s",
			result.Content[0].Text)
	}
}

func TestRichAction_FrameSelectorInPendingQueryParams(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#submit","frame":"iframe[name='payment']","sync":false}`)
	if !ok {
		t.Fatal("click with frame selector should return result")
	}
	if result.IsError {
		t.Fatalf("click with frame selector should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["frame"].(string); !ok || got != "iframe[name='payment']" {
		t.Fatalf("frame selector not forwarded correctly, got %#v", params["frame"])
	}
}

func TestRichAction_FrameIndexInPendingQueryParams(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#submit","frame":0,"sync":false}`)
	if !ok {
		t.Fatal("click with frame index should return result")
	}
	if result.IsError {
		t.Fatalf("click with frame index should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query was created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["frame"].(float64); !ok || got != 0 {
		t.Fatalf("frame index not forwarded correctly, got %#v", params["frame"])
	}
}

// ============================================
// Schema: analyze param in tools/list
// ============================================

func TestRichAction_SchemaHasAnalyze(t *testing.T) {
	env := newInteractTestEnv(t)

	// Get tools list directly (not via embedded MCPHandler which has no toolHandler)
	tools := env.handler.ToolsList()

	// Find interact tool schema
	var interactSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in ToolsList()")
	}

	props, ok := interactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}

	analyzeParam, exists := props["analyze"]
	if !exists {
		t.Fatal("interact schema missing 'analyze' property")
	}

	analyzeMap, ok := analyzeParam.(map[string]any)
	if !ok {
		t.Fatal("analyze property is not an object")
	}

	if analyzeMap["type"] != "boolean" {
		t.Errorf("analyze.type = %v, want 'boolean'", analyzeMap["type"])
	}

	desc, ok := analyzeMap["description"].(string)
	if !ok || desc == "" {
		t.Fatal("analyze must have a description")
	}

	// Description must mention profiling/performance/timing
	lower := strings.ToLower(desc)
	if !strings.Contains(lower, "profil") &&
		!strings.Contains(lower, "performance") &&
		!strings.Contains(lower, "timing") &&
		!strings.Contains(lower, "breakdown") {
		t.Errorf("analyze description must mention profiling/performance/timing. Got: %q", desc)
	}
}

func TestRichAction_SchemaHasEvidence(t *testing.T) {
	env := newInteractTestEnv(t)
	tools := env.handler.ToolsList()

	var interactSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in ToolsList()")
	}

	props, ok := interactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}

	evidenceParam, exists := props["evidence"]
	if !exists {
		t.Fatal("interact schema missing 'evidence' property")
	}
	evidenceMap, ok := evidenceParam.(map[string]any)
	if !ok {
		t.Fatal("evidence property is not an object")
	}
	if evidenceMap["type"] != "string" {
		t.Fatalf("evidence.type = %v, want 'string'", evidenceMap["type"])
	}

	enumValues, ok := evidenceMap["enum"].([]string)
	if !ok {
		t.Fatalf("evidence.enum missing or wrong type: %T", evidenceMap["enum"])
	}
	want := map[string]bool{"off": true, "on_mutation": true, "always": true}
	for _, v := range enumValues {
		delete(want, v)
	}
	if len(want) != 0 {
		t.Fatalf("evidence enum missing values: %v", want)
	}
}

func TestRichAction_SchemaHasFrame(t *testing.T) {
	env := newInteractTestEnv(t)
	tools := env.handler.ToolsList()

	var interactSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in ToolsList()")
	}

	props, ok := interactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}

	frameParam, exists := props["frame"]
	if !exists {
		t.Fatal("interact schema missing 'frame' property")
	}
	frameMap, ok := frameParam.(map[string]any)
	if !ok {
		t.Fatal("frame property is not an object")
	}
	if frameMap["type"] != "string" {
		t.Fatal("frame property should be type string")
	}
}

func TestRichAction_SchemaDescriptionMentionsPerf(t *testing.T) {
	env := newInteractTestEnv(t)

	// Get tools list directly
	tools := env.handler.ToolsList()

	var desc string
	for _, tool := range tools {
		if tool.Name == "interact" {
			desc = tool.Description
			break
		}
	}

	if desc == "" {
		t.Fatal("interact tool not found or has empty description")
	}

	lower := strings.ToLower(desc)
	if !strings.Contains(lower, "perf") &&
		!strings.Contains(lower, "timing") &&
		!strings.Contains(lower, "diff") {
		t.Errorf("interact description should mention perf/timing/diff so AI discovers perf_diff. Got: %q", desc)
	}
}

// ============================================
// Correlation ID: DOM actions with analyze
// ============================================

func TestRichAction_CorrelationID_HasAction(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","analyze":true,"background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed with pilot enabled")
	}

	// Extract correlation_id from result
	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "correlation_id") {
		t.Fatal("Result must contain correlation_id")
	}
	if !strings.Contains(text, "dom_click_") {
		t.Errorf("Correlation ID should start with dom_click_. Got: %s", text)
	}
}

// ============================================
// Top-level surfacing: timing, dom_changes, analysis
// ============================================

func TestRichAction_AnalyzeFieldsSurfacedTopLevel(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Click with analyze:true
	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","analyze":true,"background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	// Simulate extension result with analyze enrichment fields
	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"timing": {"total_ms": 55, "js_blocking_ms": 12, "render_ms": 8},
		"dom_changes": {"added": 3, "removed": 0, "modified": 1, "summary": "3 added, 1 modified"},
		"analysis": "click completed in 55ms. 3 added, 1 modified."
	}`)
	env.capture.Queries().ApplyCommandResult(corrID, "complete", extensionResult, "")

	// Observe command_result
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// timing must be surfaced at top level
	timing, ok := responseData["timing"].(map[string]any)
	if !ok {
		t.Fatal("timing must be surfaced at top level of command_result response")
	}
	if totalMs, _ := timing["total_ms"].(float64); totalMs != 55 {
		t.Errorf("timing.total_ms = %v, want 55", totalMs)
	}

	// dom_changes must be surfaced at top level
	domChanges, ok := responseData["dom_changes"].(map[string]any)
	if !ok {
		t.Fatal("dom_changes must be surfaced at top level of command_result response")
	}
	if domChanges["summary"] != "3 added, 1 modified" {
		t.Errorf("dom_changes.summary = %v, want '3 added, 1 modified'", domChanges["summary"])
	}

	// analysis must be surfaced at top level
	analysis, ok := responseData["analysis"].(string)
	if !ok || analysis == "" {
		t.Fatal("analysis must be surfaced at top level of command_result response")
	}
	if !strings.Contains(analysis, "55ms") {
		t.Errorf("analysis = %q, should mention 55ms", analysis)
	}

	// Enriched fields are de-duplicated from nested result payloads.
	extResult, ok := responseData["result"].(map[string]any)
	if !ok {
		t.Fatal("result envelope must still exist")
	}
	if _, exists := extResult["timing"]; exists {
		t.Error("timing should be stripped from nested result after top-level enrichment")
	}
}

func TestRichAction_NoAnalyzeFieldsWhenAbsent(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	// Extension result WITHOUT analyze fields (compact mode)
	extensionResult := json.RawMessage(`{"success": true, "action": "click", "dom_summary": "1 added"}`)
	env.capture.Queries().ApplyCommandResult(corrID, "complete", extensionResult, "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// None of the analyze fields should be at top level
	if _, exists := responseData["timing"]; exists {
		t.Error("timing should NOT be at top level when extension doesn't provide it")
	}
	if _, exists := responseData["dom_changes"]; exists {
		t.Error("dom_changes should NOT be at top level when extension doesn't provide it")
	}
	if _, exists := responseData["analysis"]; exists {
		t.Error("analysis should NOT be at top level when extension doesn't provide it")
	}
}

func TestRichAction_MatchedDiagnosticsSurfacedTopLevel(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"text=Submit","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"matched": {
			"tag": "button",
			"role": "button",
			"classes": ["btn", "btn-primary"],
			"text_preview": "Submit",
			"selector": "text=Submit",
			"bbox": {"x": 64, "y": 120, "width": 128, "height": 36}
		}
	}`)
	env.capture.Queries().ApplyCommandResult(corrID, "complete", extensionResult, "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	matched, ok := responseData["matched"].(map[string]any)
	if !ok {
		t.Fatal("matched diagnostics must be surfaced at top level")
	}
	if matched["tag"] != "button" {
		t.Fatalf("matched.tag = %v, want button", matched["tag"])
	}
	if matched["text_preview"] != "Submit" {
		t.Fatalf("matched.text_preview = %v, want Submit", matched["text_preview"])
	}
	bbox, ok := matched["bbox"].(map[string]any)
	if !ok {
		t.Fatal("matched.bbox should be present")
	}
	if bbox["width"] != float64(128) {
		t.Fatalf("matched.bbox.width = %v, want 128", bbox["width"])
	}

	extResult, ok := responseData["result"].(map[string]any)
	if !ok {
		t.Fatal("result envelope must still exist")
	}
	if _, exists := extResult["matched"]; exists {
		t.Fatal("matched should be stripped from nested result after top-level enrichment")
	}
}

func TestRichAction_TargetContextSurfacedTopLevel(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","tab_id":77,"background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"resolved_tab_id": 77,
		"resolved_url": "https://example.com/form",
		"effective_tab_id": 77,
		"effective_url": "https://example.com/form",
		"target_context": {
			"source": "explicit_tab",
			"requested_tab_id": 77,
			"tracked_tab_id": null,
			"use_active_tab": false
		}
	}`)
	env.capture.Queries().ApplyCommandResult(corrID, "complete", extensionResult, "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if responseData["effective_tab_id"] != float64(77) {
		t.Fatalf("effective_tab_id = %v, want 77", responseData["effective_tab_id"])
	}
	if responseData["effective_url"] != "https://example.com/form" {
		t.Fatalf("effective_url = %v, want https://example.com/form", responseData["effective_url"])
	}

	// Routing-only context is stripped from successful responses to reduce tokens.
	if _, exists := responseData["target_context"]; exists {
		t.Fatal("target_context should be stripped from successful responses")
	}
	if _, exists := responseData["resolved_tab_id"]; exists {
		t.Fatal("resolved_tab_id should be omitted when URL did not change")
	}
	if _, exists := responseData["resolved_url"]; exists {
		t.Fatal("resolved_url should be omitted when URL did not change")
	}

	extResult, ok := responseData["result"].(map[string]any)
	if !ok {
		t.Fatal("result envelope missing")
	}
	for _, key := range []string{"target_context", "resolved_tab_id", "resolved_url", "effective_tab_id", "effective_url"} {
		if _, exists := extResult[key]; exists {
			t.Fatalf("%s should be stripped from nested result after enrichment", key)
		}
	}
}

// ============================================
// Command result passthrough: dom_summary, analyze fields
// ============================================

func TestRichAction_DomSummaryPassthrough(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Click with analyze:true
	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","analyze":true,"background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	// Simulate extension result with dom_summary and analyze fields
	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"dom_summary": "2 added, 1 modified",
		"timing": {"total_ms": 42},
		"dom_changes": {"added": 2, "removed": 0, "modified": 1, "summary": "2 added, 1 modified"},
		"analysis": "click completed in 42ms. 2 added, 1 modified."
	}`)
	env.capture.Queries().ApplyCommandResult(corrID, "complete", extensionResult, "")

	// Observe command_result — verify extension fields pass through
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Enriched fields should be surfaced top-level.
	if responseData["dom_summary"] != "2 added, 1 modified" {
		t.Errorf("dom_summary = %q, want '2 added, 1 modified'", responseData["dom_summary"])
	}
	if responseData["analysis"] != "click completed in 42ms. 2 added, 1 modified." {
		t.Errorf("analysis = %q, want 'click completed in 42ms. 2 added, 1 modified.'", responseData["analysis"])
	}
	timingTop, exists := responseData["timing"].(map[string]any)
	if !exists {
		t.Fatal("timing missing at top-level command result")
	}
	if totalMs, _ := timingTop["total_ms"].(float64); totalMs != 42 {
		t.Errorf("timing.total_ms = %v, want 42", totalMs)
	}

	// Nested result should keep core result payload while dropping enriched duplicates.
	extResult, ok := responseData["result"].(map[string]any)
	if !ok {
		t.Fatal("response should contain 'result' object with extension data")
	}
	for _, key := range []string{"dom_summary", "analysis", "timing"} {
		if _, exists := extResult[key]; exists {
			t.Fatalf("%s should be stripped from nested result after top-level enrichment", key)
		}
	}
}

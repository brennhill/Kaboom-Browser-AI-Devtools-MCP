// Purpose: Validate form validation summary compact responses.
// Why: Prevents silent regressions in form validation summary mode.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_inspect_test.go — Tests for form validation summary builder.
package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/inspect"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestBuildFormValidationSummary_Basic(t *testing.T) {
	t.Parallel()

	formsData := map[string]any{
		"forms": []any{
			map[string]any{"id": "login", "valid": true},
			map[string]any{"id": "search", "valid": false},
			map[string]any{"id": "signup", "valid": false},
		},
	}
	formsJSON, _ := json.Marshal(formsData)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "Form validation results\n" + string(formsJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	summarized := inspect.BuildFormValidationSummary(resp)

	var summaryResult mcp.MCPToolResult
	if err := json.Unmarshal(summarized.Result, &summaryResult); err != nil {
		t.Fatal(err)
	}

	// Parse the JSON from the summary text
	text := summaryResult.Content[0].Text
	idx := 0
	for i, ch := range text {
		if ch == '{' {
			idx = i
			break
		}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("failed to parse summary JSON: %v", err)
	}

	if data["total_forms"] != float64(3) {
		t.Errorf("total_forms = %v, want 3", data["total_forms"])
	}
	if data["valid"] != float64(1) {
		t.Errorf("valid = %v, want 1", data["valid"])
	}
	if data["invalid"] != float64(2) {
		t.Errorf("invalid = %v, want 2", data["invalid"])
	}
}

func TestBuildFormValidationSummary_ErrorResponse(t *testing.T) {
	t.Parallel()

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "error occurred"}},
		IsError: true,
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	// Should return original response unchanged
	summarized := inspect.BuildFormValidationSummary(resp)
	if string(summarized.Result) != string(resp.Result) {
		t.Error("error response should be returned unchanged")
	}
}

func TestBuildFormValidationSummary_EmptyTextBlock_NoPanic(t *testing.T) {
	t.Parallel()

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: ""}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	// Regression guard: empty text blocks must be ignored safely.
	summarized := inspect.BuildFormValidationSummary(resp)
	if string(summarized.Result) != string(resp.Result) {
		t.Error("expected unchanged response for empty text block")
	}
}

func TestExtractFormsList_Direct(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"forms": []any{map[string]any{"id": "f1"}},
	}
	forms := inspect.ExtractFormsList(data)
	if len(forms) != 1 {
		t.Errorf("expected 1 form, got %d", len(forms))
	}
}

func TestExtractFormsList_Nested(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"result": map[string]any{
			"forms": []any{map[string]any{"id": "f1"}},
		},
	}
	forms := inspect.ExtractFormsList(data)
	if len(forms) != 1 {
		t.Errorf("expected 1 form from nested result, got %d", len(forms))
	}
}

func TestExtractFormsList_NoForms(t *testing.T) {
	t.Parallel()
	data := map[string]any{"foo": "bar"}
	forms := inspect.ExtractFormsList(data)
	if forms != nil {
		t.Error("expected nil for data without forms")
	}
}

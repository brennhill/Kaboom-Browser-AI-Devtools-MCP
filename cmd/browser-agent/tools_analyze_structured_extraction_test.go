// Purpose: Validate analyze structured extraction modes (form_state, data_table).
// Why: Ensures agents can extract structured form/table data without fragile execute_js pipelines.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"strings"
	"testing"
)

func TestToolsAnalyzeFormState_DispatchesQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"form_state","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("form_state should queue successfully, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "form_state_") {
		t.Errorf("correlation_id should start with 'form_state_', got: %s", corr)
	}

	pq := lastPendingQuerySnapshot(cap.Queries())
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.Type != "form_state" {
		t.Errorf("pending query type = %q, want 'form_state'", pq.Type)
	}
}

func TestToolsAnalyzeDataTable_DispatchesQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"data_table","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("data_table should queue successfully, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "data_table_") {
		t.Errorf("correlation_id should start with 'data_table_', got: %s", corr)
	}

	pq := lastPendingQuerySnapshot(cap.Queries())
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.Type != "data_table" {
		t.Errorf("pending query type = %q, want 'data_table'", pq.Type)
	}
}

func TestToolsAnalyzeStructuredExtraction_InValidModes(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	modes := strings.Join(h.analyzeDispatcher.ValidModes(), ", ")
	if !strings.Contains(modes, "form_state") {
		t.Errorf("valid analyze modes should include 'form_state': %s", modes)
	}
	if !strings.Contains(modes, "data_table") {
		t.Errorf("valid analyze modes should include 'data_table': %s", modes)
	}
}

func TestToolsAnalyzeSchema_StructuredExtractionInWhatEnum(t *testing.T) {
	t.Parallel()
	_, _, _ = makeToolHandler(t)

	tools := toolSchemasForTest()
	var analyzeSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "analyze" {
			analyzeSchema = tool.InputSchema
			break
		}
	}
	if analyzeSchema == nil {
		t.Fatal("analyze tool not found in ToolsList()")
	}

	props, ok := analyzeSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("analyze schema missing properties")
	}
	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("analyze schema missing 'what' property")
	}
	enumValues, ok := whatProp["enum"].([]string)
	if !ok {
		t.Fatal("'what' property missing enum")
	}

	foundFormState := false
	foundDataTable := false
	for _, v := range enumValues {
		if v == "form_state" {
			foundFormState = true
		}
		if v == "data_table" {
			foundDataTable = true
		}
	}
	if !foundFormState {
		t.Errorf("'what' enum should include 'form_state', got: %v", enumValues)
	}
	if !foundDataTable {
		t.Errorf("'what' enum should include 'data_table', got: %v", enumValues)
	}
}

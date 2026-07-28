// tools_interact_state_queries_test.go — Tests interact state and query responses.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// interact(action:"save_state") — Response Fields
// ============================================

func TestToolsInteractSaveState_MissingSnapshotName(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"save_state"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("save_state without snapshot_name should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "snapshot_name") {
		t.Error("error should mention 'snapshot_name' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractSaveState_Success(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"save_state","snapshot_name":"test_save"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("save_state should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "snapshot_name", "state"} {
		if _, ok := data[field]; !ok {
			t.Errorf("save_state response missing field %q", field)
		}
	}
	if data["status"] != "saved" {
		t.Errorf("status = %v, want 'saved'", data["status"])
	}
	if data["snapshot_name"] != "test_save" {
		t.Errorf("snapshot_name = %v, want 'test_save'", data["snapshot_name"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"list_states") — Response Fields
// ============================================

func TestToolsInteractListStates_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"list_states"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("list_states should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"states", "count"} {
		if _, ok := data[field]; !ok {
			t.Errorf("list_states response missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"list_interactive") — Response Fields
// ============================================

func TestToolsInteractListInteractive_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"list_interactive"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("list_interactive should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "dom_list_") {
		t.Errorf("correlation_id should start with 'dom_list_', got: %s", corr)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query for list_interactive")
	}
	if !strings.Contains(string(pq.Params), `"action":"list_interactive"`) {
		t.Errorf("pending query params should include canonical action=list_interactive, got: %s", string(pq.Params))
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"get_text", structured:true) — Regression (#390)
// ============================================

func TestToolsInteractGetText_StructuredPassthrough(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"get_text","selector":".accordion","structured":true}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("get_text should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "dom_get_text_") {
		t.Errorf("correlation_id should start with 'dom_get_text_', got: %s", corr)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query for get_text")
	}
	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("pending query params should be valid JSON: %v", err)
	}
	if got, _ := params["action"].(string); got != "get_text" {
		t.Fatalf("pending query action = %#v, want get_text", params["action"])
	}
	if got, _ := params["structured"].(bool); !got {
		t.Fatalf("pending query should include structured=true, got: %#v", params["structured"])
	}
}

// ============================================
// validateDOMActionParams Tests
// ============================================

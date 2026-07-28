// Purpose: Tests for session tool handler dispatch.
// Docs: docs/features/feature/request-session-correlation/index.md

// tool_handler_test.go — Tests for tool-handler.go (SessionManager.HandleTool).
// Covers: diffSessionsParams parsing, capture/compare/list/delete dispatching,
// error paths for missing params, invalid JSON, unknown actions.
package session

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
)

// ============================================
// HandleTool: Invalid Input
// ============================================

func TestSessionHandleTool_InvalidJSON(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	_, err := sm.HandleTool(json.RawMessage(`{broken`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestSessionHandleTool_EmptyParams(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	_, err := sm.HandleTool(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("Expected error for empty params (no action)")
	}
}

func TestSessionHandleTool_UnknownAction(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	_, err := sm.HandleTool(json.RawMessage(`{"action":"explode"}`))
	if err == nil {
		t.Fatal("Expected error for unknown action")
	}
}

// ============================================
// HandleTool: Capture
// ============================================

func TestSessionHandleTool_CaptureSuccess(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		consoleErrors: []types.SnapshotError{
			{Type: "error", Message: "test-err", Count: 2},
		},
		consoleWarnings: []types.SnapshotError{
			{Type: "warning", Message: "deprecation", Count: 1},
		},
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/health", Status: 200},
		},
		pageURL: "http://localhost:3000/app",
	}
	sm := NewSessionManager(10, mock)

	params := json.RawMessage(`{"action":"capture","name":"my-snap"}`)
	result, err := sm.HandleTool(params)
	if err != nil {
		t.Fatalf("HandleTool capture failed: %v", err)
	}

	// Marshal + unmarshal to check JSON structure
	data, _ := json.Marshal(result)
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if resp["action"] != "captured" {
		t.Errorf("Expected action='captured', got %v", resp["action"])
	}
	if resp["name"] != "my-snap" {
		t.Errorf("Expected name='my-snap', got %v", resp["name"])
	}

	// Check snapshot sub-object
	snapshot, ok := resp["snapshot"].(map[string]any)
	if !ok {
		t.Fatal("Expected snapshot object in response")
	}
	if snapshot["page_url"] != "http://localhost:3000/app" {
		t.Errorf("Expected page_url, got %v", snapshot["page_url"])
	}
	// console_errors count
	if snapshot["console_errors"] != float64(1) {
		t.Errorf("Expected console_errors=1, got %v", snapshot["console_errors"])
	}
	if snapshot["console_warnings"] != float64(1) {
		t.Errorf("Expected console_warnings=1, got %v", snapshot["console_warnings"])
	}
	if snapshot["network_requests"] != float64(1) {
		t.Errorf("Expected network_requests=1, got %v", snapshot["network_requests"])
	}
}

func TestSessionHandleTool_CaptureMissingName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	_, err := sm.HandleTool(json.RawMessage(`{"action":"capture"}`))
	if err == nil {
		t.Fatal("Expected error for capture without name")
	}
}

func TestSessionHandleTool_CaptureWithURLFilter(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/data", Status: 200},
			{Method: "GET", URL: "/static/app.js", Status: 200},
		},
		pageURL: "http://localhost:3000",
	}
	sm := NewSessionManager(10, mock)

	params := json.RawMessage(`{"action":"capture","name":"filtered","url":"/api/"}`)
	_, err := sm.HandleTool(params)
	if err != nil {
		t.Fatalf("HandleTool capture with filter failed: %v", err)
	}

	// Verify the snapshot was stored
	list := sm.List()
	if len(list) != 1 {
		t.Fatalf("Expected 1 snapshot, got %d", len(list))
	}
	if list[0].Name != "filtered" {
		t.Errorf("Expected name 'filtered', got %q", list[0].Name)
	}
}

func TestSessionHandleTool_CaptureInvalidName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Reserved name
	_, err := sm.HandleTool(json.RawMessage(`{"action":"capture","name":"current"}`))
	if err == nil {
		t.Fatal("Expected error for reserved name 'current'")
	}
}

// ============================================
// HandleTool: Compare
// ============================================

func TestSessionHandleTool_CompareSuccess(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []types.SnapshotError{}
	sm.Capture("snap-a", "")

	mock.consoleErrors = []types.SnapshotError{{Type: "error", Message: "err", Count: 1}}
	sm.Capture("snap-b", "")

	params := json.RawMessage(`{"action":"compare","compare_a":"snap-a","compare_b":"snap-b"}`)
	result, err := sm.HandleTool(params)
	if err != nil {
		t.Fatalf("HandleTool compare failed: %v", err)
	}

	data, _ := json.Marshal(result)
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if resp["action"] != "compared" {
		t.Errorf("Expected action='compared', got %v", resp["action"])
	}
	if resp["a"] != "snap-a" {
		t.Errorf("Expected a='snap-a', got %v", resp["a"])
	}
	if resp["b"] != "snap-b" {
		t.Errorf("Expected b='snap-b', got %v", resp["b"])
	}
}

func TestSessionHandleTool_CompareMissingParams(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Missing compare_a and compare_b
	_, err := sm.HandleTool(json.RawMessage(`{"action":"compare"}`))
	if err == nil {
		t.Fatal("Expected error for compare without compare_a/compare_b")
	}

	// Missing compare_b only
	_, err = sm.HandleTool(json.RawMessage(`{"action":"compare","compare_a":"a"}`))
	if err == nil {
		t.Fatal("Expected error for compare without compare_b")
	}

	// Missing compare_a only
	_, err = sm.HandleTool(json.RawMessage(`{"action":"compare","compare_b":"b"}`))
	if err == nil {
		t.Fatal("Expected error for compare without compare_a")
	}
}

func TestSessionHandleTool_CompareNonExistentSnapshots(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	params := json.RawMessage(`{"action":"compare","compare_a":"missing-a","compare_b":"missing-b"}`)
	_, err := sm.HandleTool(params)
	if err == nil {
		t.Fatal("Expected error for non-existent snapshots")
	}
}

// ============================================
// HandleTool: List
// ============================================

func TestSessionHandleTool_ListEmpty(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	result, err := sm.HandleTool(json.RawMessage(`{"action":"list"}`))
	if err != nil {
		t.Fatalf("HandleTool list failed: %v", err)
	}

	data, _ := json.Marshal(result)
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if resp["action"] != "listed" {
		t.Errorf("Expected action='listed', got %v", resp["action"])
	}
	snapshots, ok := resp["snapshots"].([]any)
	if !ok {
		t.Fatal("Expected snapshots array")
	}
	if len(snapshots) != 0 {
		t.Errorf("Expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestSessionHandleTool_ListMultiple(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("alpha", "")
	sm.Capture("beta", "")
	sm.Capture("gamma", "")

	result, err := sm.HandleTool(json.RawMessage(`{"action":"list"}`))
	if err != nil {
		t.Fatalf("HandleTool list failed: %v", err)
	}

	data, _ := json.Marshal(result)
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	snapshots := resp["snapshots"].([]any)
	if len(snapshots) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(snapshots))
	}
}

// ============================================
// HandleTool: Delete
// ============================================

func TestSessionHandleTool_DeleteSuccess(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("to-delete", "")

	result, err := sm.HandleTool(json.RawMessage(`{"action":"delete","name":"to-delete"}`))
	if err != nil {
		t.Fatalf("HandleTool delete failed: %v", err)
	}

	data, _ := json.Marshal(result)
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if resp["action"] != "deleted" {
		t.Errorf("Expected action='deleted', got %v", resp["action"])
	}
	if resp["name"] != "to-delete" {
		t.Errorf("Expected name='to-delete', got %v", resp["name"])
	}

	// Verify actually deleted
	if len(sm.List()) != 0 {
		t.Error("Expected 0 snapshots after delete")
	}
}

func TestSessionHandleTool_DeleteMissingName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	_, err := sm.HandleTool(json.RawMessage(`{"action":"delete"}`))
	if err == nil {
		t.Fatal("Expected error for delete without name")
	}
}

func TestSessionHandleTool_DeleteNonExistent(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	_, err := sm.HandleTool(json.RawMessage(`{"action":"delete","name":"ghost"}`))
	if err == nil {
		t.Fatal("Expected error for deleting non-existent snapshot")
	}
}

// ============================================
// HandleTool: JSON Field Names (snake_case)
// ============================================

func TestSessionHandleTool_JSONFieldsAreSnakeCase(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		consoleErrors: []types.SnapshotError{{Type: "error", Message: "e", Count: 1}},
		pageURL:       "http://localhost:3000",
	}
	sm := NewSessionManager(10, mock)

	sm.Capture("a", "")
	sm.Capture("b", "")

	// Capture response check
	params := json.RawMessage(`{"action":"capture","name":"check-json"}`)
	result, _ := sm.HandleTool(params)
	data, _ := json.Marshal(result)
	jsonStr := string(data)

	// Verify snake_case keys exist in the response
	snakeCaseKeys := []string{"captured_at", "console_errors", "console_warnings", "network_requests", "page_url"}
	for _, key := range snakeCaseKeys {
		if !json.Valid(data) {
			t.Fatalf("Invalid JSON response")
		}
		// Parse to check key exists
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("json.Unmarshal error: %v", err)
		}
		snapshot, ok := parsed["snapshot"].(map[string]any)
		if !ok {
			continue
		}
		if _, exists := snapshot[key]; !exists {
			t.Errorf("Expected snake_case key %q in response: %s", key, jsonStr)
		}
	}

	// Compare response check
	compareParams := json.RawMessage(`{"action":"compare","compare_a":"a","compare_b":"b"}`)
	compareResult, _ := sm.HandleTool(compareParams)
	compareData, _ := json.Marshal(compareResult)
	var compareResp map[string]any
	if err := json.Unmarshal(compareData, &compareResp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// Check diff contains expected snake_case keys
	diff, ok := compareResp["diff"].(map[string]any)
	if ok {
		diffKeys := []string{"errors", "network", "performance", "summary"}
		for _, key := range diffKeys {
			if _, exists := diff[key]; !exists {
				t.Errorf("Expected key %q in diff response", key)
			}
		}
	}
}

// ============================================
// HandleTool: End-to-end Capture + Compare
// ============================================

func TestSessionHandleTool_CaptureCompareIntegration(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Capture "before" with no errors
	mock.consoleErrors = []types.SnapshotError{}
	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/data", Status: 200, Duration: 50},
	}
	sm.HandleTool(json.RawMessage(`{"action":"capture","name":"before"}`))

	// Capture "after" with errors
	mock.consoleErrors = []types.SnapshotError{
		{Type: "error", Message: "Something broke", Count: 1},
	}
	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/data", Status: 500, Duration: 200},
	}
	sm.HandleTool(json.RawMessage(`{"action":"capture","name":"after"}`))

	// Compare
	compareResult, err := sm.HandleTool(json.RawMessage(`{"action":"compare","compare_a":"before","compare_b":"after"}`))
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	data, _ := json.Marshal(compareResult)
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	summary, ok := resp["summary"].(map[string]any)
	if !ok {
		t.Fatal("Expected summary in response")
	}
	if summary["verdict"] != "regressed" {
		t.Errorf("Expected verdict='regressed', got %v", summary["verdict"])
	}
}

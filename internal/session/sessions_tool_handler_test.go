// sessions_tool_handler_test.go — Tests snapshot MCP tool action dispatch.
// Docs: docs/features/feature/historical-snapshots/index.md

package session

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestHandleDiffSessions_Capture(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		pageURL: "http://localhost:3000/test",
		consoleErrors: []types.SnapshotError{
			{Type: "console", Message: "test error", Count: 1},
		},
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/health", Status: 200},
		},
	}
	sm := NewSessionManager(10, mock)

	params := map[string]any{
		"action": "capture",
		"name":   "test-snap",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err != nil {
		t.Fatalf("HandleTool capture failed: %v", err)
	}

	resultJSON, _ := json.Marshal(result)
	var response map[string]any
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if response["action"] != "captured" {
		t.Errorf("Expected action 'captured', got %v", response["action"])
	}
	if response["name"] != "test-snap" {
		t.Errorf("Expected name 'test-snap', got %v", response["name"])
	}
}

func TestHandleDiffSessions_List(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("snap-1", "")
	sm.Capture("snap-2", "")

	params := map[string]any{"action": "list"}
	paramsJSON, _ := json.Marshal(params)

	result, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err != nil {
		t.Fatalf("HandleTool list failed: %v", err)
	}

	resultJSON, _ := json.Marshal(result)
	var response map[string]any
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if response["action"] != "listed" {
		t.Errorf("Expected action 'listed', got %v", response["action"])
	}
	snapshots, ok := response["snapshots"].([]any)
	if !ok {
		t.Fatal("Expected snapshots array in response")
	}
	if len(snapshots) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(snapshots))
	}
}

func TestHandleDiffSessions_Compare(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []types.SnapshotError{}
	sm.Capture("before", "")
	mock.consoleErrors = []types.SnapshotError{{Type: "console", Message: "err", Count: 1}}
	sm.Capture("after", "")

	params := map[string]any{
		"action":    "compare",
		"compare_a": "before",
		"compare_b": "after",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err != nil {
		t.Fatalf("HandleTool compare failed: %v", err)
	}

	resultJSON, _ := json.Marshal(result)
	var response map[string]any
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if response["action"] != "compared" {
		t.Errorf("Expected action 'compared', got %v", response["action"])
	}
}

func TestHandleDiffSessions_Delete(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("to-delete", "")

	params := map[string]any{
		"action": "delete",
		"name":   "to-delete",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err != nil {
		t.Fatalf("HandleTool delete failed: %v", err)
	}

	resultJSON, _ := json.Marshal(result)
	var response map[string]any
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if response["action"] != "deleted" {
		t.Errorf("Expected action 'deleted', got %v", response["action"])
	}

	list := sm.List()
	if len(list) != 0 {
		t.Errorf("Expected 0 snapshots after delete, got %d", len(list))
	}
}

func TestHandleDiffSessions_InvalidAction(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	params := map[string]any{"action": "invalid"}
	paramsJSON, _ := json.Marshal(params)

	_, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err == nil {
		t.Error("Expected error for invalid action")
	}
}

func TestHandleDiffSessions_MissingAction(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	params := map[string]any{}
	paramsJSON, _ := json.Marshal(params)

	_, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err == nil {
		t.Error("Expected error for missing action")
	}
}

func TestHandleDiffSessions_CaptureRequiresName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	params := map[string]any{"action": "capture"}
	paramsJSON, _ := json.Marshal(params)

	_, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err == nil {
		t.Error("Expected error when capture action has no name")
	}
}

func TestHandleDiffSessions_CompareRequiresParams(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	params := map[string]any{"action": "compare"}
	paramsJSON, _ := json.Marshal(params)

	_, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err == nil {
		t.Error("Expected error when compare has no compare_a/compare_b")
	}
}

func TestHandleDiffSessions_URLFilter(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		pageURL: "http://localhost:3000",
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200},
			{Method: "GET", URL: "/static/app.js", Status: 200},
		},
	}
	sm := NewSessionManager(10, mock)

	params := map[string]any{
		"action": "capture",
		"name":   "filtered",
		"url":    "/api/",
	}
	paramsJSON, _ := json.Marshal(params)

	sm.HandleTool(json.RawMessage(paramsJSON))

	list := sm.List()
	if len(list) != 1 {
		t.Fatalf("Expected 1 snapshot, got %d", len(list))
	}
}

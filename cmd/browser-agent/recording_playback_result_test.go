// Purpose: Tests for recording playback result formatting.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// recording_playback_result_test.go — Tests for buildPlaybackResult method.
package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// buildPlaybackResult
// ============================================

func newPlaybackTestEnv(t *testing.T) *ToolHandler {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "test-playback.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	return mcpHandler.toolHandler.(*ToolHandler)
}

func TestBuildPlaybackResult_AllActionsSucceeded(t *testing.T) {
	t.Parallel()
	handler := newPlaybackTestEnv(t)

	session := &capture.PlaybackSession{
		RecordingID:      "rec-001",
		StartedAt:        time.Now().Add(-500 * time.Millisecond),
		ActionsExecuted:  5,
		ActionsFailed:    0,
		Results:          make([]capture.PlaybackResult, 3),
		SelectorFailures: map[string]int{},
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`42`)}
	resp := handler.buildPlaybackResult(req, "rec-001", session)

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got %q", resp.JSONRPC)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result.IsError {
		t.Error("expected isError false for successful playback")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}

	// Parse the JSON from the text content
	text := result.Content[0].Text
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if data["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", data["status"])
	}
	if data["recording_id"] != "rec-001" {
		t.Errorf("expected recording_id 'rec-001', got %v", data["recording_id"])
	}
	if data["actions_executed"] != float64(5) {
		t.Errorf("expected actions_executed 5, got %v", data["actions_executed"])
	}
	if data["actions_failed"] != float64(0) {
		t.Errorf("expected actions_failed 0, got %v", data["actions_failed"])
	}
	if data["actions_total"] != float64(5) {
		t.Errorf("expected actions_total 5, got %v", data["actions_total"])
	}
	if data["results_count"] != float64(3) {
		t.Errorf("expected results_count 3, got %v", data["results_count"])
	}
}

func TestBuildPlaybackResult_PartialFailure(t *testing.T) {
	t.Parallel()
	handler := newPlaybackTestEnv(t)

	session := &capture.PlaybackSession{
		RecordingID:     "rec-002",
		StartedAt:       time.Now().Add(-1 * time.Second),
		ActionsExecuted: 3,
		ActionsFailed:   2,
		Results:         make([]capture.PlaybackResult, 5),
		SelectorFailures: map[string]int{
			"#submit-btn": 1,
			".nav-link":   1,
		},
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := handler.buildPlaybackResult(req, "rec-002", session)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	text := result.Content[0].Text
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if data["status"] != "partial" {
		t.Errorf("expected status 'partial' when actions failed, got %v", data["status"])
	}
	if data["actions_total"] != float64(5) {
		t.Errorf("expected actions_total 5 (3+2), got %v", data["actions_total"])
	}

	// Verify selector_failures is present
	sf, ok := data["selector_failures"].(map[string]any)
	if !ok {
		t.Fatalf("expected selector_failures to be object, got %T", data["selector_failures"])
	}
	if sf["#submit-btn"] != float64(1) {
		t.Errorf("expected #submit-btn failure count 1, got %v", sf["#submit-btn"])
	}
}

func TestBuildPlaybackResult_ZeroActions(t *testing.T) {
	t.Parallel()
	handler := newPlaybackTestEnv(t)

	session := &capture.PlaybackSession{
		RecordingID:      "rec-003",
		StartedAt:        time.Now(),
		ActionsExecuted:  0,
		ActionsFailed:    0,
		Results:          []capture.PlaybackResult{},
		SelectorFailures: map[string]int{},
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := handler.buildPlaybackResult(req, "rec-003", session)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	text := result.Content[0].Text
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if data["status"] != "ok" {
		t.Errorf("expected status 'ok' when no failures, got %v", data["status"])
	}
	if data["actions_total"] != float64(0) {
		t.Errorf("expected actions_total 0, got %v", data["actions_total"])
	}
	if data["results_count"] != float64(0) {
		t.Errorf("expected results_count 0, got %v", data["results_count"])
	}
}

func TestBuildPlaybackResult_DurationIsPositive(t *testing.T) {
	t.Parallel()
	handler := newPlaybackTestEnv(t)

	session := &capture.PlaybackSession{
		RecordingID:      "rec-004",
		StartedAt:        time.Now().Add(-100 * time.Millisecond),
		ActionsExecuted:  1,
		ActionsFailed:    0,
		Results:          make([]capture.PlaybackResult, 1),
		SelectorFailures: map[string]int{},
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := handler.buildPlaybackResult(req, "rec-004", session)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	text := result.Content[0].Text
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	durationMs, ok := data["duration_ms"].(float64)
	if !ok {
		t.Fatalf("expected duration_ms to be number, got %T", data["duration_ms"])
	}
	if durationMs < 0 {
		t.Errorf("expected positive duration_ms, got %v", durationMs)
	}
}

func TestBuildPlaybackResult_SnakeCaseFields(t *testing.T) {
	t.Parallel()
	handler := newPlaybackTestEnv(t)

	session := &capture.PlaybackSession{
		RecordingID:      "rec-005",
		StartedAt:        time.Now(),
		ActionsExecuted:  1,
		ActionsFailed:    0,
		Results:          make([]capture.PlaybackResult, 1),
		SelectorFailures: map[string]int{},
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := handler.buildPlaybackResult(req, "rec-005", session)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	text := result.Content[0].Text
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	expectedKeys := []string{
		"status", "recording_id", "actions_executed", "actions_failed",
		"actions_total", "duration_ms", "results_count", "selector_failures",
	}
	for _, key := range expectedKeys {
		if _, ok := data[key]; !ok {
			t.Errorf("expected snake_case key %q to be present", key)
		}
	}
}

func TestBuildPlaybackResult_MessageFormat(t *testing.T) {
	t.Parallel()
	handler := newPlaybackTestEnv(t)

	session := &capture.PlaybackSession{
		RecordingID:      "rec-006",
		StartedAt:        time.Now(),
		ActionsExecuted:  7,
		ActionsFailed:    3,
		Results:          make([]capture.PlaybackResult, 10),
		SelectorFailures: map[string]int{},
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := handler.buildPlaybackResult(req, "rec-006", session)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	text := result.Content[0].Text
	// The text should start with a summary line like "Playback complete: 7/10 actions executed"
	if len(text) == 0 {
		t.Fatal("expected non-empty text content")
	}
	// Summary should contain "Playback complete"
	if !strings.Contains(text, "Playback complete") {
		t.Errorf("expected text to contain 'Playback complete', got: %s", text[:min(len(text), 100)])
	}
}

// containsString removed — replaced by strings.Contains at all call sites.

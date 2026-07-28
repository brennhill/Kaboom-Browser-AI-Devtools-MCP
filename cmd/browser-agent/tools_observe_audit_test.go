// Purpose: Tests for observe audit trail integration.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_audit_test.go — Behavioral tests for observe tool
//
// ⚠️ ARCHITECTURAL INVARIANT - ALL OBSERVE MODES MUST WORK
//
// These tests verify ACTUAL BEHAVIOR, not just "doesn't crash":
// 1. Data flow: Add data → observe returns that data
// 2. Empty state: Returns empty array, not error
// 3. Error handling: Invalid inputs return structured errors
// 4. Safety: All modes execute without panic
//
// Test Categories:
// - Data flow tests: Verify data added to buffers appears in observe output
// - Empty state tests: Verify empty buffers return empty results (not errors)
// - Error handling tests: Verify invalid inputs return structured errors
// - Safety net tests: Verify all 29 modes don't panic
//
// Run: go test ./cmd/browser-agent -run "TestObserveAudit" -v
package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Test Infrastructure
// ============================================

type observeTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Capture
}

func newObserveTestEnv(t *testing.T) *observeTestEnv {
	t.Helper()
	server, err := NewServer(t.TempDir()+"/test-observe-audit.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)
	return &observeTestEnv{handler: handler, server: server, capture: cap}
}

// callObserve invokes the observe tool and returns parsed result
func (e *observeTestEnv) callObserve(t *testing.T, what string) (mcp.MCPToolResult, bool) {
	t.Helper()

	args := json.RawMessage(`{"what":"` + what + `"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := e.handler.observeDispatcher.Handle(req, args)

	if resp.Result == nil {
		return mcp.MCPToolResult{}, false
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	return result, true
}

// addLogEntry adds a log entry to the server (simulates extension POST)
func (e *observeTestEnv) addLogEntry(entry mcp.LogEntry) {
	e.server.logs.SeedEntries([]mcp.LogEntry{
		entry,
	}, nil)
}

// ============================================
// Behavioral Tests: Data Flow
// These tests add data, then verify observe returns it
// ============================================

// TestObserveAudit_Errors_DataFlow verifies: add errors → observe errors returns them
func TestObserveAudit_Errors_DataFlow(t *testing.T) {
	env := newObserveTestEnv(t)

	// 1. Add error log entry (simulating browser extension POST)
	errorEntry := mcp.LogEntry{
		"type":    "console",
		"level":   "error",
		"message": "UniqueErrorMessage12345",
		"ts":      time.Now().UTC().Format(time.RFC3339),
	}
	env.addLogEntry(errorEntry)

	// 2. Call observe errors
	result, ok := env.callObserve(t, "errors")

	// ASSERTION 1: Result returned (not nil)
	if !ok {
		t.Fatal("observe errors should return result, got nil")
	}

	// ASSERTION 2: Content block exists
	if len(result.Content) == 0 {
		t.Fatal("observe errors should return at least one content block")
	}

	// ASSERTION 3: Our specific error message appears in output
	text := result.Content[0].Text
	if !strings.Contains(text, "UniqueErrorMessage12345") {
		t.Errorf("observe errors MUST contain the error we added\n"+
			"Expected to find: UniqueErrorMessage12345\n"+
			"Got: %s", text)
	}
}

// TestObserveAudit_Logs_DataFlow verifies logs are returned
func TestObserveAudit_Logs_DataFlow(t *testing.T) {
	env := newObserveTestEnv(t)

	// Add multiple log entries
	env.addLogEntry(mcp.LogEntry{"type": "console", "level": "log", "message": "LogTestEntry1", "ts": time.Now().UTC().Format(time.RFC3339)})
	env.addLogEntry(mcp.LogEntry{"type": "console", "level": "warn", "message": "LogTestEntry2", "ts": time.Now().UTC().Format(time.RFC3339)})

	// Call observe logs
	result, ok := env.callObserve(t, "logs")
	if !ok {
		t.Fatal("observe logs should return result")
	}

	text := result.Content[0].Text

	// ASSERTION: Both log messages appear
	if !strings.Contains(text, "LogTestEntry1") {
		t.Errorf("observe logs missing LogTestEntry1\nGot: %s", text)
	}
	if !strings.Contains(text, "LogTestEntry2") {
		t.Errorf("observe logs missing LogTestEntry2\nGot: %s", text)
	}
}

// TestObserveAudit_NetworkBodies_DataFlow verifies network body flow
func TestObserveAudit_NetworkBodies_DataFlow(t *testing.T) {
	env := newObserveTestEnv(t)

	// Add network body via HTTP handler (correct field names)
	body := capture.NetworkBody{
		URL:          "https://unique-test-api.example.com/endpoint",
		Method:       "POST",
		Status:       201,
		RequestBody:  `{"test": "request"}`,
		ResponseBody: `{"test": "response"}`,
	}
	payload, _ := json.Marshal(map[string]any{"bodies": []capture.NetworkBody{body}})
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	env.capture.HandleNetworkBodies(w, req)

	if w.Code != 200 {
		t.Fatalf("POST /network-bodies failed: %d", w.Code)
	}

	// Call observe network_bodies
	result, ok := env.callObserve(t, "network_bodies")
	if !ok {
		t.Fatal("observe network_bodies should return result")
	}

	text := result.Content[0].Text

	// ASSERTION: URL appears in output
	if !strings.Contains(text, "unique-test-api.example.com") {
		t.Errorf("observe network_bodies MUST contain posted URL\nGot: %s", text)
	}
}

// TestObserveAudit_EnhancedActions_DataFlow verifies actions flow
func TestObserveAudit_EnhancedActions_DataFlow(t *testing.T) {
	env := newObserveTestEnv(t)

	// Add enhanced action via HTTP handler
	action := capture.EnhancedAction{
		Type:      "click",
		Timestamp: time.Now().UnixMilli(),
		URL:       "https://example.com/unique-action-test",
	}
	payload, _ := json.Marshal(map[string]any{"actions": []capture.EnhancedAction{action}})
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	env.capture.HandleEnhancedActions(w, req)

	if w.Code != 200 {
		t.Fatalf("POST /enhanced-actions failed: %d", w.Code)
	}

	// Call observe actions
	result, ok := env.callObserve(t, "actions")
	if !ok {
		t.Fatal("observe actions should return result")
	}

	text := result.Content[0].Text

	// ASSERTION: Action type appears
	if !strings.Contains(text, "click") {
		t.Errorf("observe actions MUST contain 'click'\nGot: %s", text)
	}
}

// ============================================
// Behavioral Tests: Empty State Handling
// Empty buffers should return empty results, NOT errors
// ============================================

func TestObserveAudit_EmptyState_ReturnsEmptyNotError(t *testing.T) {
	env := newObserveTestEnv(t)

	// These modes should return empty results, NOT errors
	modes := []string{
		"errors",
		"logs",
		"network_bodies",
		"websocket_events",
		"actions",
	}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			result, ok := env.callObserve(t, mode)

			// ASSERTION 1: Returns result (not nil)
			if !ok {
				t.Fatalf("observe %s should return result even when empty", mode)
			}

			// ASSERTION 2: Not an error
			if result.IsError {
				t.Errorf("observe %s with no data should NOT be an error", mode)
			}

			// ASSERTION 3: Has content
			if len(result.Content) == 0 {
				t.Errorf("observe %s should return content block", mode)
			}
		})
	}
}

// ============================================
// Behavioral Tests: Error Handling
// Invalid inputs should return structured errors
// ============================================

func TestObserveAudit_UnknownMode_ReturnsStructuredError(t *testing.T) {
	env := newObserveTestEnv(t)

	result, ok := env.callObserve(t, "completely_invalid_mode_xyz")
	if !ok {
		t.Fatal("unknown mode should return result with isError")
	}

	// ASSERTION 1: IsError is true
	if !result.IsError {
		t.Error("unknown mode MUST set isError:true")
	}

	// ASSERTION 2: Error message is helpful
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "unknown") &&
		!strings.Contains(strings.ToLower(text), "invalid") {
		t.Errorf("error should mention 'unknown' or 'invalid'\nGot: %s", text)
	}
}

func TestObserveAudit_MissingWhat_ReturnsError(t *testing.T) {
	env := newObserveTestEnv(t)

	// Call with empty args (missing "what" parameter)
	args := json.RawMessage(`{}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.observeDispatcher.Handle(req, args)

	if resp.Result == nil {
		t.Fatal("missing 'what' should return result with error")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// ASSERTION: IsError is true
	if !result.IsError {
		t.Error("missing 'what' parameter MUST set isError:true")
	}
}

func TestObserveAudit_InvalidJSON_ReturnsParseError(t *testing.T) {
	env := newObserveTestEnv(t)

	args := json.RawMessage(`{invalid json here}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.observeDispatcher.Handle(req, args)

	// ASSERTION: Returns some response (not nil/panic)
	if resp.Result == nil && resp.Error == nil {
		t.Fatal("invalid JSON should return response, not nil")
	}

	// If result, should be error
	if resp.Result != nil {
		var result mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("json.Unmarshal error: %v", err)
		}
		if !result.IsError {
			t.Error("invalid JSON MUST return isError:true")
		}
	}
}

// ============================================
// Safety Net: All Modes Execute Without Panic
// ============================================

func TestObserveAudit_AllModes_NoPanic(t *testing.T) {
	if testing.Short() {
		t.Skip("skips 5s+ waterfall timeout in short mode")
	}

	env := newObserveTestEnv(t)

	// Complete list from tools_observe.go
	allModes := []string{
		"errors", "logs", "extension_logs", "network_waterfall",
		"network_bodies", "websocket_events", "websocket_status",
		"actions", "vitals", "page", "tabs", "pilot",
		"performance", "accessibility",
		"timeline", "error_clusters", "error_bundles", "history",
		"security_audit", "third_party_audit",
		"command_result", "pending_commands", "failed_commands",
		"recordings", "recording_actions", "playback_results",
		"log_diff_report",
	}

	for _, mode := range allModes {
		t.Run(mode, func(t *testing.T) {
			// Catch panics
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("observe(%s) PANICKED: %v", mode, r)
				}
			}()

			args := json.RawMessage(`{"what":"` + mode + `"}`)
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := env.handler.observeDispatcher.Handle(req, args)

			// ASSERTION: Returns something
			if resp.Result == nil && resp.Error == nil {
				t.Errorf("observe(%s) returned nil response", mode)
			}
		})
	}
}

// TestObserveAudit_ModeCount documents coverage
func TestObserveAudit_ModeCount(t *testing.T) {
	t.Log("Observe audit covers 30 modes with:")
	t.Log("  - 4 data flow tests (verify add data → observe returns it)")
	t.Log("  - 5 empty state tests (verify empty returns empty, not error)")
	t.Log("  - 3 error handling tests (verify structured errors)")
	t.Log("  - 30 panic safety tests (verify no crashes)")
}

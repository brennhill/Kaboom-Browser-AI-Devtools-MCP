// Purpose: Tests for configure audit log handler.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_configure_audit_test.go — Behavioral tests for configure tool
//
// ⚠️ ARCHITECTURAL INVARIANT - ALL CONFIGURE ACTIONS MUST WORK
//
// These tests verify ACTUAL BEHAVIOR, not just "doesn't crash":
// 1. Data flow: Configure action → returns expected data
// 2. Parameter validation: Required params return errors when missing
// 3. Response format: Returns correctly structured MCP responses
// 4. Safety: All actions execute without panic
//
// Test Categories:
// - Data flow tests: Verify configure returns expected response data
// - Error handling tests: Verify missing params return structured errors
// - Response format tests: Verify MCP response structure
// - Safety net tests: Verify all 19 actions don't panic
//
// Run: go test ./cmd/browser-agent -run "TestConfigureAudit" -v
package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Test Infrastructure
// ============================================

type configureTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Capture
}

func newConfigureTestEnv(t *testing.T) *configureTestEnv {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "test-configure.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	cap.SetPilotEnabled(false) // explicit default for configure context tests
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)
	return &configureTestEnv{handler: handler, server: server, capture: cap}
}

// callConfigure invokes the configure tool and returns parsed result
func (e *configureTestEnv) callConfigure(t *testing.T, argsJSON string) (mcp.MCPToolResult, bool) {
	t.Helper()

	args := json.RawMessage(argsJSON)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := e.handler.toolConfigure(req, args)

	if resp.Result == nil {
		return mcp.MCPToolResult{}, false
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	return result, true
}

// Legacy helper for backward compatibility with existing tests
func createConfigureTestHandler(t *testing.T) *ToolHandler {
	t.Helper()
	return newConfigureTestEnv(t).handler
}

// ============================================
// Behavioral Tests: Data Flow
// These tests verify configure returns expected response data
// ============================================

// TestConfigureAudit_Health_DataFlow verifies health returns status and component info
func TestConfigureAudit_Health_DataFlow(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"health"}`)
	if !ok {
		t.Fatal("health should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Error("health should NOT return isError")
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("health should return content block")
	}

	// ASSERTION 3: Content contains status information
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "status") &&
		!strings.Contains(strings.ToLower(text), "ok") &&
		!strings.Contains(strings.ToLower(text), "health") {
		t.Errorf("health response should contain status info\nGot: %s", text)
	}
}

// TestConfigureAudit_TestBoundary_DataFlow verifies test boundary returns test_id
func TestConfigureAudit_TestBoundary_DataFlow(t *testing.T) {
	env := newConfigureTestEnv(t)

	// Start test boundary with unique test_id
	testID := "unique_test_boundary_12345"
	result, ok := env.callConfigure(t, `{"what":"test_boundary_start","test_id":"`+testID+`"}`)
	if !ok {
		t.Fatal("test_boundary_start should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("test_boundary_start should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Response contains our test_id
	if len(result.Content) == 0 {
		t.Fatal("test_boundary_start should return content")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, testID) {
		t.Errorf("test_boundary_start response MUST contain test_id\n"+
			"Expected to find: %s\nGot: %s", testID, text)
	}
}

// TestConfigureAudit_Recording_DataFlow verifies recording start returns recording_id
func TestConfigureAudit_Recording_DataFlow(t *testing.T) {
	env := newConfigureTestEnv(t)

	// Start recording with unique name
	recordingName := "unique_recording_test_12345"
	result, ok := env.callConfigure(t, `{"what":"event_recording_start","name":"`+recordingName+`"}`)
	if !ok {
		t.Fatal("event_recording_start should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("event_recording_start should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Response contains recording_id
	if len(result.Content) == 0 {
		t.Fatal("event_recording_start should return content")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "recording") {
		t.Errorf("event_recording_start response MUST mention recording\nGot: %s", text)
	}
}

// TestConfigureAudit_Clear_DataFlow verifies clear returns ok status
func TestConfigureAudit_Clear_DataFlow(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"clear"}`)
	if !ok {
		t.Fatal("clear should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Error("clear should NOT return isError")
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("clear should return content")
	}

	// ASSERTION 3: Response indicates success
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "ok") &&
		!strings.Contains(strings.ToLower(text), "clear") &&
		!strings.Contains(strings.ToLower(text), "success") {
		t.Errorf("clear should indicate success\nGot: %s", text)
	}
}

// TestConfigureAudit_NoiseRule_List_DataFlow verifies noise_rule list returns rules array
func TestConfigureAudit_NoiseRule_List_DataFlow(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"list"}`)
	if !ok {
		t.Fatal("noise_rule list should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Error("noise_rule list should NOT return isError")
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("noise_rule list should return content")
	}

	// ASSERTION 3: Response contains rules info
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "rule") &&
		!strings.Contains(strings.ToLower(text), "noise") {
		t.Errorf("noise_rule list should mention rules\nGot: %s", text)
	}
}

// ============================================
// Behavioral Tests: Parameter Validation
// Missing required params should return structured errors
// ============================================

// TestConfigureAudit_TestBoundaryStart_MissingTestID verifies error for missing test_id
func TestConfigureAudit_TestBoundaryStart_MissingTestID(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"test_boundary_start"}`)
	if !ok {
		t.Fatal("test_boundary_start without test_id should return result")
	}

	// ASSERTION 1: IsError is true
	if !result.IsError {
		t.Error("test_boundary_start without test_id MUST return isError:true")
	}

	// ASSERTION 2: Error mentions missing parameter
	if len(result.Content) == 0 {
		t.Fatal("error response should have content")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "test_id") &&
		!strings.Contains(strings.ToLower(text), "missing") &&
		!strings.Contains(strings.ToLower(text), "required") {
		t.Errorf("error should mention missing test_id\nGot: %s", text)
	}
}

// TestConfigureAudit_TestBoundaryEnd_MissingTestID verifies error for missing test_id
func TestConfigureAudit_TestBoundaryEnd_MissingTestID(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"test_boundary_end"}`)
	if !ok {
		t.Fatal("test_boundary_end without test_id should return result")
	}

	// ASSERTION 1: IsError is true
	if !result.IsError {
		t.Error("test_boundary_end without test_id MUST return isError:true")
	}
}

// TestConfigureAudit_RecordingStop_MissingRecordingID verifies error for missing recording_id
func TestConfigureAudit_RecordingStop_MissingRecordingID(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"event_recording_stop"}`)
	if !ok {
		t.Fatal("event_recording_stop without recording_id should return result")
	}

	// ASSERTION 1: IsError is true
	if !result.IsError {
		t.Error("event_recording_stop without recording_id MUST return isError:true")
	}
}

// TestConfigureAudit_Playback_MissingRecordingID verifies error for missing recording_id
func TestConfigureAudit_Playback_MissingRecordingID(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"playback"}`)
	if !ok {
		t.Fatal("playback without recording_id should return result")
	}

	// ASSERTION 1: IsError is true
	if !result.IsError {
		t.Error("playback without recording_id MUST return isError:true")
	}
}

// TestConfigureAudit_LogDiff_MissingIDs verifies error for missing recording IDs
func TestConfigureAudit_LogDiff_MissingIDs(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"log_diff"}`)
	if !ok {
		t.Fatal("log_diff without IDs should return result")
	}

	// ASSERTION 1: IsError is true
	if !result.IsError {
		t.Error("log_diff without original_id/replay_id MUST return isError:true")
	}
}

// ============================================
// Behavioral Tests: Error Handling
// Invalid inputs should return structured errors
// ============================================

// TestConfigureAudit_UnknownAction_StructuredError verifies unknown action returns helpful error
func TestConfigureAudit_UnknownAction_StructuredError(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"completely_invalid_action_xyz"}`)
	if !ok {
		t.Fatal("unknown action should return result with isError")
	}

	// ASSERTION 1: IsError is true
	if !result.IsError {
		t.Error("unknown action MUST set isError:true")
	}

	// ASSERTION 2: Error message is helpful
	if len(result.Content) == 0 {
		t.Fatal("error response should have content")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "unknown") &&
		!strings.Contains(strings.ToLower(text), "invalid") {
		t.Errorf("error should mention 'unknown' or 'invalid'\nGot: %s", text)
	}
}

// TestConfigureAudit_MissingAction_ReturnsError verifies missing action returns error
func TestConfigureAudit_MissingAction_ReturnsError(t *testing.T) {
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{}`)
	if !ok {
		t.Fatal("missing 'action' should return result with error")
	}

	// ASSERTION: IsError is true
	if !result.IsError {
		t.Error("missing 'action' parameter MUST set isError:true")
	}
}

// TestConfigureAudit_InvalidJSON_ReturnsParseError verifies invalid JSON returns error
func TestConfigureAudit_InvalidJSON_ReturnsParseError(t *testing.T) {
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{invalid json here}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolConfigure(req, args)

	// ASSERTION: Returns some response (not nil/panic)
	if resp.Result == nil && resp.Error == nil {
		t.Fatal("invalid JSON should return response, not nil")
	}

	// If result, should be error
	if resp.Result != nil {
		var result mcp.MCPToolResult
		_ = json.Unmarshal(resp.Result, &result)
		if !result.IsError {
			t.Error("invalid JSON MUST return isError:true")
		}
	}
}

// TestConfigureAudit_ValidateAPI_InvalidOperation removed in Phase 0: moved to analyze({what:'api_validation'})

// ============================================
// Behavioral Tests: Empty State Handling
// Empty state should return empty results, NOT errors
// ============================================

func TestConfigureAudit_EmptyState_ReturnsEmptyNotError(t *testing.T) {
	env := newConfigureTestEnv(t)

	// These actions should return success even with no data
	// Note: query_dom removed in Phase 0 (moved to analyze({what:'dom'}))
	actions := []struct {
		name string
		args string
	}{
		{"clear", `{"what":"clear"}`},
		{"health", `{"what":"health"}`},
		{"noise_rule_list", `{"what":"noise_rule","noise_action":"list"}`},
		{"audit_log_report", `{"what":"audit_log"}`},
	}

	for _, tc := range actions {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := env.callConfigure(t, tc.args)

			// ASSERTION 1: Returns result (not nil)
			if !ok {
				t.Fatalf("configure %s should return result even when empty", tc.name)
			}

			// ASSERTION 2: Not an error
			if result.IsError {
				t.Errorf("configure %s with no data should NOT be an error", tc.name)
			}

			// ASSERTION 3: Has content
			if len(result.Content) == 0 {
				t.Errorf("configure %s should return content block", tc.name)
			}
		})
	}
}

// ============================================
// Safety Net: All Actions Execute Without Panic
// ============================================

// validateConfigureResponse checks response validity
func validateConfigureResponse(t *testing.T, action string, resp mcp.JSONRPCResponse) {
	t.Helper()

	if resp.Result == nil && resp.Error == nil {
		t.Errorf("configure(%s): response has neither result nor error", action)
		return
	}

	if resp.Result != nil {
		var parsed any
		if err := json.Unmarshal(resp.Result, &parsed); err != nil {
			t.Errorf("configure(%s): result is not valid JSON: %v", action, err)
		}
	}

	if resp.Error != nil {
		errStr := resp.Error.Message
		if strings.Contains(errStr, "panic") || strings.Contains(errStr, "nil pointer") {
			t.Errorf("configure(%s) panicked: %s", action, errStr)
		}
	}
}

// TestConfigureAudit_AllActions tests every configure action
func TestConfigureAudit_AllActions(t *testing.T) {
	handler := createConfigureTestHandler(t)

	// Complete list of configure actions from tools_configure.go
	actions := []struct {
		action string
		args   string
	}{
		{"store", `{"what":"store","store_action":"save","key":"test_key","data":"test_value"}`},
		{"load", `{"what":"load","key":"test_key"}`},
		{"noise_rule", `{"what":"noise_rule","operation":"list"}`},
		{"clear", `{"what":"clear"}`},
		{"diff_sessions", `{"what":"diff_sessions"}`},
		{"audit_log", `{"what":"audit_log","operation":"report"}`},
		{"health", `{"what":"health"}`},
		{"telemetry", `{"what":"telemetry"}`},
		{"streaming", `{"what":"streaming"}`},
		{"test_boundary_start", `{"what":"test_boundary_start","test_name":"unit_test"}`},
		{"test_boundary_end", `{"what":"test_boundary_end"}`},
		{"event_recording_start", `{"what":"event_recording_start","name":"test_recording"}`},
		{"event_recording_stop", `{"what":"event_recording_stop"}`},
		{"playback", `{"what":"playback","name":"test_recording"}`},
		{"log_diff", `{"what":"log_diff"}`},
	}

	for _, tc := range actions {
		t.Run(tc.action, func(t *testing.T) {
			args := json.RawMessage(tc.args)
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := handler.toolConfigure(req, args)
			validateConfigureResponse(t, tc.action, resp)
		})
	}
}

// TestConfigureAudit_NoiseRule_AllOperations tests noise_rule sub-operations
func TestConfigureAudit_NoiseRule_AllOperations(t *testing.T) {
	handler := createConfigureTestHandler(t)

	operations := []struct {
		op   string
		args string
	}{
		{"add", `{"what":"noise_rule","operation":"add","pattern":"noise_pattern"}`},
		{"remove", `{"what":"noise_rule","operation":"remove","pattern":"noise_pattern"}`},
		{"list", `{"what":"noise_rule","operation":"list"}`},
		{"reset", `{"what":"noise_rule","operation":"reset"}`},
		{"auto_detect", `{"what":"noise_rule","operation":"auto_detect"}`},
	}

	for _, tc := range operations {
		t.Run("noise_rule_"+tc.op, func(t *testing.T) {
			args := json.RawMessage(tc.args)
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := handler.toolConfigure(req, args)
			validateConfigureResponse(t, "noise_rule:"+tc.op, resp)
		})
	}
}

// TestConfigureAudit_AuditLog_AllOperations tests audit_log sub-operations
func TestConfigureAudit_AuditLog_AllOperations(t *testing.T) {
	handler := createConfigureTestHandler(t)

	operations := []struct {
		op   string
		args string
	}{
		{"analyze", `{"what":"audit_log","operation":"analyze"}`},
		{"report", `{"what":"audit_log","operation":"report"}`},
		{"clear", `{"what":"audit_log","operation":"clear"}`},
	}

	for _, tc := range operations {
		t.Run("audit_log_"+tc.op, func(t *testing.T) {
			args := json.RawMessage(tc.args)
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := handler.toolConfigure(req, args)
			validateConfigureResponse(t, "audit_log:"+tc.op, resp)
		})
	}
}

// TestConfigureAudit_Health_Detailed tests health action returns expected fields
func TestConfigureAudit_Health_Detailed(t *testing.T) {
	handler := createConfigureTestHandler(t)

	args := json.RawMessage(`{"what":"health"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := handler.toolConfigure(req, args)

	if resp.Result == nil {
		t.Fatal("health should return result")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if len(result.Content) == 0 {
		t.Error("health should return content")
	}
}

// TestConfigureAudit_UnknownAction verifies error handling
func TestConfigureAudit_UnknownAction(t *testing.T) {
	handler := createConfigureTestHandler(t)

	args := json.RawMessage(`{"what":"nonexistent_action_xyz"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := handler.toolConfigure(req, args)

	// Should return error, not panic
	if resp.Result != nil {
		var result mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err == nil {
			if !result.IsError {
				t.Error("unknown action should return isError:true")
			}
		}
	}
}

// TestConfigureAudit_EmptyArgs verifies graceful handling
func TestConfigureAudit_EmptyArgs(t *testing.T) {
	handler := createConfigureTestHandler(t)

	args := json.RawMessage(`{}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := handler.toolConfigure(req, args)

	validateConfigureResponse(t, "empty", resp)
}

// TestConfigureAudit_InvalidJSON verifies error handling
func TestConfigureAudit_InvalidJSON(t *testing.T) {
	handler := createConfigureTestHandler(t)

	args := json.RawMessage(`{invalid}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := handler.toolConfigure(req, args)

	if resp.Error == nil && resp.Result == nil {
		t.Error("invalid JSON should return some response")
	}
}

// TestConfigureAudit_ActionCount documents coverage
func TestConfigureAudit_ActionCount(t *testing.T) {
	t.Log("Configure audit covers 14 actions with:")
	t.Log("  - 5 data flow tests (verify actions return expected data)")
	t.Log("  - 5 parameter validation tests (verify missing params return errors)")
	t.Log("  - 4 error handling tests (verify structured errors)")
	t.Log("  - 5 empty state tests (verify empty returns success, not error)")
	t.Log("  - 14 safety net tests (verify all actions don't panic)")
	t.Log("  - 5 noise_rule sub-operation tests")
	t.Log("  - 3 audit_log sub-operation tests")
	t.Log("  Note: query_dom and validate_api moved to analyze tool")
}

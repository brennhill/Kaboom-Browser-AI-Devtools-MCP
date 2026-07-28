// tools_configure_network_recording_handler_test.go — Tests configure-tool recording actions.
// Docs: docs/features/feature/backend-log-streaming/index.md
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Handler-level tests: canonical network-recording owner
// ============================================

func TestToolConfigureNetworkRecording_StartSuccess(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"operation":"start"}`)
	resp := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if status, _ := data["status"].(string); status != "recording" {
		t.Errorf("status = %q, want %q", status, "recording")
	}
	startedAt, _ := data["started_at"].(string)
	if startedAt == "" {
		t.Error("started_at should be present and non-empty")
	}
	// Verify started_at is valid RFC3339
	if _, err := time.Parse(time.RFC3339, startedAt); err != nil {
		t.Errorf("started_at %q is not valid RFC3339: %v", startedAt, err)
	}
}

func TestToolConfigureNetworkRecording_StartWithFilters(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"operation":"start","domain":"api.example.com","method":"POST"}`)
	resp := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if status, _ := data["status"].(string); status != "recording" {
		t.Errorf("status = %q, want %q", status, "recording")
	}
	if df, _ := data["domain_filter"].(string); df != "api.example.com" {
		t.Errorf("domain_filter = %q, want %q", df, "api.example.com")
	}
	if mf, _ := data["method_filter"].(string); mf != "POST" {
		t.Errorf("method_filter = %q, want %q", mf, "POST")
	}
}

func TestToolConfigureNetworkRecording_StartAlreadyActive(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"operation":"start"}`)

	// First start should succeed
	resp1 := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, args)
	result1 := parseToolResult(t, resp1)
	if result1.IsError {
		t.Fatalf("first start should succeed, got: %s", firstText(result1))
	}

	// Second start should fail
	resp2 := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, args)
	result2 := parseToolResult(t, resp2)
	if !result2.IsError {
		t.Fatal("second start should return isError:true")
	}
	text := firstText(result2)
	if !strings.Contains(text, "already active") {
		t.Errorf("error should mention 'already active', got: %s", text)
	}
	if !strings.Contains(text, "invalid_param") {
		t.Errorf("error code should be 'invalid_param', got: %s", text)
	}
}

func TestToolConfigureNetworkRecording_StopNotActive(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"operation":"stop"}`)
	resp := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("stop without active recording should return isError:true")
	}
	text := firstText(result)
	if !strings.Contains(text, "No active network recording") {
		t.Errorf("error should mention 'No active network recording', got: %s", text)
	}
	if !strings.Contains(text, "operation='start'") {
		t.Errorf("recovery action should suggest starting first, got: %s", text)
	}
}

func TestToolConfigureNetworkRecording_StopSuccess(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	// Start recording first
	startArgs := json.RawMessage(`{"operation":"start"}`)
	resp1 := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, startArgs)
	result1 := parseToolResult(t, resp1)
	if result1.IsError {
		t.Fatalf("start should succeed, got: %s", firstText(result1))
	}

	// Stop recording
	stopArgs := json.RawMessage(`{"operation":"stop"}`)
	resp2 := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, stopArgs)
	result2 := parseToolResult(t, resp2)
	if result2.IsError {
		t.Fatalf("stop should succeed, got: %s", firstText(result2))
	}

	data := extractResultJSON(t, result2)
	if status, _ := data["status"].(string); status != "stopped" {
		t.Errorf("status = %q, want %q", status, "stopped")
	}
	// count should be 0 since no network bodies in test capture
	if count, _ := data["count"].(float64); count != 0 {
		t.Errorf("count = %v, want 0", count)
	}
	// requests should be present (nil serializes as null, but the field exists)
	if _, hasDuration := data["duration_ms"]; !hasDuration {
		t.Error("response should include duration_ms")
	}
}

func TestToolConfigureNetworkRecording_StatusInactive(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"operation":"status"}`)
	resp := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("status should succeed, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if active, _ := data["active"].(bool); active {
		t.Error("active should be false when not recording")
	}
	// When inactive, started_at and duration_ms should NOT be present
	if _, hasStartedAt := data["started_at"]; hasStartedAt {
		t.Error("started_at should not be present when inactive")
	}
	if _, hasDuration := data["duration_ms"]; hasDuration {
		t.Error("duration_ms should not be present when inactive")
	}
}

func TestToolConfigureNetworkRecording_StatusActive(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	// Start recording with filters
	startArgs := json.RawMessage(`{"operation":"start","domain":"test.com","method":"GET"}`)
	resp1 := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, startArgs)
	result1 := parseToolResult(t, resp1)
	if result1.IsError {
		t.Fatalf("start should succeed, got: %s", firstText(result1))
	}

	// Query status
	statusArgs := json.RawMessage(`{"operation":"status"}`)
	resp2 := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, statusArgs)
	result2 := parseToolResult(t, resp2)
	if result2.IsError {
		t.Fatalf("status should succeed, got: %s", firstText(result2))
	}

	data := extractResultJSON(t, result2)
	if active, _ := data["active"].(bool); !active {
		t.Error("active should be true when recording")
	}
	startedAt, _ := data["started_at"].(string)
	if startedAt == "" {
		t.Error("started_at should be present when active")
	}
	if _, hasDuration := data["duration_ms"]; !hasDuration {
		t.Error("duration_ms should be present when active")
	}
	if df, _ := data["domain_filter"].(string); df != "test.com" {
		t.Errorf("domain_filter = %q, want %q", df, "test.com")
	}
	if mf, _ := data["method_filter"].(string); mf != "GET" {
		t.Errorf("method_filter = %q, want %q", mf, "GET")
	}
}

func TestToolConfigureNetworkRecording_UnknownOperation(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"operation":"restart"}`)
	resp := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown operation should return isError:true")
	}
	text := firstText(result)
	if !strings.Contains(text, "Unknown operation") {
		t.Errorf("error should mention 'Unknown operation', got: %s", text)
	}
	if !strings.Contains(text, "restart") {
		t.Errorf("error should echo back the unknown operation name, got: %s", text)
	}
	if !strings.Contains(text, "'start', 'stop', or 'status'") {
		t.Errorf("recovery action should list valid operations, got: %s", text)
	}
}

func TestToolConfigureNetworkRecording_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{bad json`)
	resp := netrecord.HandleNetworkRecording(h.capture.Telemetry(), h.networkRecording, req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	text := firstText(result)
	if !strings.Contains(text, "invalid_json") {
		t.Errorf("error code should contain 'invalid_json', got: %s", text)
	}
	if !strings.Contains(text, "Fix JSON syntax") {
		t.Errorf("error should include recovery action, got: %s", text)
	}
}

func TestToolConfigureNetworkRecording_ViaDispatch(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Start via dispatch
	resp1 := callConfigureRaw(h, `{"what":"network_recording","operation":"start","domain":"dispatch.example.com"}`)
	result1 := parseToolResult(t, resp1)
	if result1.IsError {
		t.Fatalf("dispatch start should succeed, got: %s", firstText(result1))
	}

	data1 := extractResultJSON(t, result1)
	if status, _ := data1["status"].(string); status != "recording" {
		t.Errorf("status = %q, want %q", status, "recording")
	}
	if df, _ := data1["domain_filter"].(string); df != "dispatch.example.com" {
		t.Errorf("domain_filter = %q, want %q", df, "dispatch.example.com")
	}

	// Status via dispatch
	resp2 := callConfigureRaw(h, `{"what":"network_recording","operation":"status"}`)
	result2 := parseToolResult(t, resp2)
	if result2.IsError {
		t.Fatalf("dispatch status should succeed, got: %s", firstText(result2))
	}

	data2 := extractResultJSON(t, result2)
	if active, _ := data2["active"].(bool); !active {
		t.Error("active should be true after start via dispatch")
	}

	// Stop via dispatch
	resp3 := callConfigureRaw(h, `{"what":"network_recording","operation":"stop"}`)
	result3 := parseToolResult(t, resp3)
	if result3.IsError {
		t.Fatalf("dispatch stop should succeed, got: %s", firstText(result3))
	}

	data3 := extractResultJSON(t, result3)
	if status, _ := data3["status"].(string); status != "stopped" {
		t.Errorf("status = %q, want %q", status, "stopped")
	}
}

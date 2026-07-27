// tools_configure_jitter_test.go — Tests for action jitter configuration.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Happy Path
// ============================================

func TestToolConfigureActionJitter_SetValue(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"action_jitter_ms":200}`)
	resp := toolconfigure.HandleActionJitter(h, req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if got, _ := data["action_jitter_ms"].(float64); got != 200 {
		t.Errorf("action_jitter_ms = %v, want 200", got)
	}
}

func TestToolConfigureActionJitter_ResponseContainsSummary(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"action_jitter_ms":100}`)
	resp := toolconfigure.HandleActionJitter(h, req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}
	if !strings.Contains(firstText(result), "Action jitter configured") {
		t.Errorf("response should contain summary text, got: %s", firstText(result))
	}
}

// ============================================
// Clamping
// ============================================

func TestToolConfigureActionJitter_NegativeClampedToZero(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"action_jitter_ms":-100}`)
	resp := toolconfigure.HandleActionJitter(h, req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if got, _ := data["action_jitter_ms"].(float64); got != 0 {
		t.Errorf("action_jitter_ms = %v, want 0 (clamped from negative)", got)
	}
}

func TestToolConfigureActionJitter_MaxClamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    string
		wantAct float64
	}{
		{
			name:    "action_jitter_ms capped at 5000",
			args:    `{"action_jitter_ms":9999}`,
			wantAct: 5000,
		},
		{
			name:    "exactly at max boundary",
			args:    `{"action_jitter_ms":5000}`,
			wantAct: 5000,
		},
		{
			name:    "one over max",
			args:    `{"action_jitter_ms":5001}`,
			wantAct: 5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h, _, _ := makeToolHandler(t)

			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := toolconfigure.HandleActionJitter(h, req, json.RawMessage(tt.args))

			result := parseToolResult(t, resp)
			if result.IsError {
				t.Fatalf("expected success, got error: %s", firstText(result))
			}

			data := extractResultJSON(t, result)
			if got, _ := data["action_jitter_ms"].(float64); got != tt.wantAct {
				t.Errorf("action_jitter_ms = %v, want %v", got, tt.wantAct)
			}
		})
	}
}

func TestToolConfigureActionJitter_ZeroPreserved(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"action_jitter_ms":0}`)
	resp := toolconfigure.HandleActionJitter(h, req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}
	data := extractResultJSON(t, result)
	if got, _ := data["action_jitter_ms"].(float64); got != 0 {
		t.Errorf("action_jitter_ms = %v, want 0", got)
	}
}

// ============================================
// Partial Update
// ============================================

func TestToolConfigureActionJitter_PartialUpdate(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	// Set initial value
	args1 := json.RawMessage(`{"action_jitter_ms":300}`)
	resp1 := toolconfigure.HandleActionJitter(h, req, args1)
	result1 := parseToolResult(t, resp1)
	if result1.IsError {
		t.Fatalf("initial set should succeed, got: %s", firstText(result1))
	}

	// Update with new value
	args2 := json.RawMessage(`{"action_jitter_ms":500}`)
	resp2 := toolconfigure.HandleActionJitter(h, req, args2)
	result2 := parseToolResult(t, resp2)
	if result2.IsError {
		t.Fatalf("update should succeed, got: %s", firstText(result2))
	}

	data := extractResultJSON(t, result2)
	if got, _ := data["action_jitter_ms"].(float64); got != 500 {
		t.Errorf("action_jitter_ms = %v, want 500", got)
	}
}

// ============================================
// No Params (read-only query)
// ============================================

func TestToolConfigureActionJitter_EmptyJSON_ReturnsCurrentValues(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	// Set known value first
	args1 := json.RawMessage(`{"action_jitter_ms":250}`)
	resp1 := toolconfigure.HandleActionJitter(h, req, args1)
	result1 := parseToolResult(t, resp1)
	if result1.IsError {
		t.Fatalf("initial set should succeed, got: %s", firstText(result1))
	}

	// Query with empty JSON — should return current value unchanged
	resp2 := toolconfigure.HandleActionJitter(h, req, json.RawMessage(`{}`))
	result2 := parseToolResult(t, resp2)
	if result2.IsError {
		t.Fatalf("empty JSON should succeed, got: %s", firstText(result2))
	}

	data := extractResultJSON(t, result2)
	if got, _ := data["action_jitter_ms"].(float64); got != 250 {
		t.Errorf("action_jitter_ms = %v, want 250 (unchanged)", got)
	}
}

func TestToolConfigureActionJitter_NilArgs_ReturnsDefaults(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := toolconfigure.HandleActionJitter(h, req, nil)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("nil args should succeed, got: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if got, _ := data["action_jitter_ms"].(float64); got != 0 {
		t.Errorf("action_jitter_ms = %v, want 0 (default)", got)
	}
}

// ============================================
// Error Handling
// ============================================

func TestToolConfigureActionJitter_InvalidJSON_LenientFallback(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := toolconfigure.HandleActionJitter(h, req, json.RawMessage(`{bad json`))

	// mcp.LenientUnmarshal silently ignores parse errors — invalid JSON is treated
	// as "no params" and returns current jitter value (not an error).
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatal("lenient unmarshal should not fail on invalid JSON")
	}
}

// ============================================
// JSON-RPC Response Structure
// ============================================

func TestToolConfigureActionJitter_ResponseID_Matches(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 42}
	args := json.RawMessage(`{"action_jitter_ms":100}`)
	resp := toolconfigure.HandleActionJitter(h, req, args)

	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want \"2.0\"", resp.JSONRPC)
	}
	if resp.ID != 42 {
		t.Errorf("ID = %v, want 42", resp.ID)
	}
}

// ============================================
// Integration via configure dispatch
// ============================================

func TestToolConfigureActionJitter_ViaDispatch(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"action_jitter","action_jitter_ms":350}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dispatch to action_jitter should succeed, got: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if got, _ := data["action_jitter_ms"].(float64); got != 350 {
		t.Errorf("action_jitter_ms = %v, want 350", got)
	}
}

// ============================================
// Unknown params are silently ignored
// ============================================

func TestToolConfigureActionJitter_UnknownParamsIgnored(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	// Unknown params should be ignored without error
	args := json.RawMessage(`{"action_jitter_ms":200,"unknown_param":999}`)
	resp := toolconfigure.HandleActionJitter(h, req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("unknown params should not cause error, got: %s", firstText(result))
	}
	data := extractResultJSON(t, result)
	if got, _ := data["action_jitter_ms"].(float64); got != 200 {
		t.Errorf("action_jitter_ms = %v, want 200", got)
	}
}

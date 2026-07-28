// Purpose: Tests for enum param validation in observe handlers.
// Why: Ensures invalid min_level, direction, classification, scope values return defaults with param_hint instead of errors.

package observe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func newValidationDeps() Deps {
	return (&mockTransientDeps{cap: capture.NewCapture()}).deps()
}

// extractJSON strips any text prefix before the first '{' or '['.
func extractJSON(text string) string {
	for i, ch := range text {
		if ch == '{' || ch == '[' {
			return text[i:]
		}
	}
	return text
}

// parseParamHint extracts the param_hint field from a non-error response.
func parseParamHint(t *testing.T, resp mcp.JSONRPCResponse) string {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected non-error response, got error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("no content in result")
	}
	jsonText := extractJSON(result.Content[0].Text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("unmarshal response JSON: %v", err)
	}
	hint, _ := data["param_hint"].(string)
	return hint
}

// ============================================
// min_level validation
// ============================================

func TestGetBrowserLogs_InvalidMinLevel_ReturnsDefaultWithHint(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"min_level":"critical"}`)

	resp := GetBrowserLogs(deps, req, args)
	hint := parseParamHint(t, resp)

	if hint == "" {
		t.Fatal("expected param_hint for invalid min_level")
	}
	if !strings.Contains(hint, "critical") {
		t.Errorf("hint should mention the invalid value, got: %s", hint)
	}
	if !strings.Contains(hint, "debug, log, info, warn, error") {
		t.Errorf("hint should list valid values, got: %s", hint)
	}
}

func TestGetBrowserLogs_ValidMinLevels_NoHint(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, level := range []string{"debug", "log", "info", "warn", "error", ""} {
		args, _ := json.Marshal(map[string]any{"min_level": level})
		resp := GetBrowserLogs(deps, req, args)
		hint := parseParamHint(t, resp)
		if hint != "" {
			t.Errorf("min_level=%q should not produce param_hint, got: %s", level, hint)
		}
	}
}

// ============================================
// direction validation
// ============================================

func TestGetWSEvents_InvalidDirection_ReturnsDefaultWithHint(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"direction":"both"}`)

	resp := GetWSEvents(deps, req, args)
	hint := parseParamHint(t, resp)

	if hint == "" {
		t.Fatal("expected param_hint for invalid direction")
	}
	if !strings.Contains(hint, "both") {
		t.Errorf("hint should mention the invalid value, got: %s", hint)
	}
	if !strings.Contains(hint, "incoming, outgoing") {
		t.Errorf("hint should list valid values, got: %s", hint)
	}
}

func TestGetWSEvents_ValidDirections_NoHint(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, dir := range []string{"incoming", "outgoing", ""} {
		args, _ := json.Marshal(map[string]any{"direction": dir})
		resp := GetWSEvents(deps, req, args)
		hint := parseParamHint(t, resp)
		if hint != "" {
			t.Errorf("direction=%q should not produce param_hint, got: %s", dir, hint)
		}
	}
}

// ============================================
// classification validation
// ============================================

func TestGetTransients_InvalidClassification_ReturnsDefaultWithHint(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"classification":"popup"}`)

	resp := GetTransients(deps, req, args)
	hint := parseParamHint(t, resp)

	if hint == "" {
		t.Fatal("expected param_hint for invalid classification")
	}
	if !strings.Contains(hint, "popup") {
		t.Errorf("hint should mention the invalid value, got: %s", hint)
	}
	if !strings.Contains(hint, "alert, toast, snackbar") {
		t.Errorf("hint should list valid values, got: %s", hint)
	}
}

func TestGetTransients_ValidClassifications_NoHint(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, cls := range []string{"alert", "toast", "snackbar", "notification", "tooltip", "banner", "flash", ""} {
		args, _ := json.Marshal(map[string]any{"classification": cls})
		resp := GetTransients(deps, req, args)
		hint := parseParamHint(t, resp)
		if hint != "" {
			t.Errorf("classification=%q should not produce param_hint, got: %s", cls, hint)
		}
	}
}

// ============================================
// error_bundles scope validation
// ============================================

func TestGetErrorBundles_InvalidScope_ReturnsDefaultWithHint(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"scope":"bogus"}`)

	resp := GetErrorBundles(deps, req, args)
	hint := parseParamHint(t, resp)

	if hint == "" {
		t.Fatal("expected param_hint for invalid scope in error_bundles")
	}
	if !strings.Contains(hint, "bogus") {
		t.Errorf("hint should mention the invalid value, got: %s", hint)
	}
}

func TestGetErrorBundles_ValidScopes_NoHint(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, scope := range []string{"current_page", "all", ""} {
		args, _ := json.Marshal(map[string]any{"scope": scope})
		resp := GetErrorBundles(deps, req, args)
		hint := parseParamHint(t, resp)
		if hint != "" {
			t.Errorf("scope=%q should not produce param_hint, got: %s", scope, hint)
		}
	}
}

// extractMCPResult parses the MCPToolResult from a response.
func extractMCPResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return result
}

// Purpose: Tests for tool error response formatting.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_errors_test.go — Tests for structured error retryable field and retry_after_ms.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestRootDoesNotReexportMCPErrorSurface(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(testFile), "*.go"))
	if err != nil {
		t.Fatalf("list root Go files: %v", err)
	}
	for _, forbidden := range []string{
		"ErrInvalidJSON =",
		"type StructuredError =",
		"func mcpStructuredError(",
		"func withParam(",
		"func withHint(",
		"func withAction(",
		"func withSelector(",
		"func withRetryable(",
		"func withRetryAfterMs(",
		"func withFinal(",
		"func withRecoveryToolCall(",
	} {
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read %s: %v", path, readErr)
			}
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s retains MCP compatibility facade %q", filepath.Base(path), forbidden)
			}
		}
	}
}

// ============================================
// Retryable Error Field Tests
// ============================================

func TestStructuredError_RetryableErrors_SerializeCorrectly(t *testing.T) {
	t.Parallel()

	result := mcp.StructuredErrorResponse(
		mcp.ErrExtTimeout, "Extension timed out", "Retry the command",
		mcp.WithRetryable(true), mcp.WithRetryAfterMs(1000),
	)

	se := extractStructuredErrorJSON(t, result)

	retryable, ok := se["retryable"].(bool)
	if !ok {
		t.Fatal("retryable field missing or not a bool")
	}
	if !retryable {
		t.Error("retryable should be true for mcp.ErrExtTimeout")
	}

	retryAfterMs, ok := se["retry_after_ms"].(float64)
	if !ok {
		t.Fatal("retry_after_ms field missing or not a number")
	}
	if retryAfterMs != 1000 {
		t.Errorf("retry_after_ms = %v, want 1000", retryAfterMs)
	}
}

func TestStructuredError_NonRetryableErrors_OmitRetryAfterMs(t *testing.T) {
	t.Parallel()

	result := mcp.StructuredErrorResponse(
		mcp.ErrInvalidParam, "Bad parameter", "Fix the parameter",
		mcp.WithRetryable(false),
	)

	se := extractStructuredErrorJSON(t, result)

	retryable, ok := se["retryable"].(bool)
	if !ok {
		t.Fatal("retryable field missing or not a bool")
	}
	if retryable {
		t.Error("retryable should be false for mcp.ErrInvalidParam")
	}

	if _, exists := se["retry_after_ms"]; exists {
		t.Error("retry_after_ms should not be present for non-retryable errors")
	}
}

func TestStructuredError_DefaultRetryable_IsFalse(t *testing.T) {
	t.Parallel()

	// No mcp.WithRetryable option — should default to false
	result := mcp.StructuredErrorResponse(
		mcp.ErrInternal, "Internal error", "Do not retry",
	)

	se := extractStructuredErrorJSON(t, result)

	// retryable should still be present (zero value = false)
	retryable, ok := se["retryable"].(bool)
	if !ok {
		t.Fatal("retryable field missing or not a bool")
	}
	if retryable {
		t.Error("retryable should default to false")
	}
}

func TestStructuredError_CanonicalRecoveryContractFields(t *testing.T) {
	t.Parallel()

	result := mcp.StructuredErrorResponse(
		mcp.ErrMissingParam, "Missing parameter", "Call interact with what=list_interactive",
	)

	se := extractStructuredErrorJSON(t, result)
	if se["error_code"] != mcp.ErrMissingParam {
		t.Fatalf("error_code = %v, want %q", se["error_code"], mcp.ErrMissingParam)
	}
	if se["recovery_playbook"] != "Call interact with what=list_interactive" {
		t.Fatalf("recovery_playbook = %v", se["recovery_playbook"])
	}
	if _, exists := se["error"]; exists {
		t.Fatalf("legacy field error should not be present: %v", se["error"])
	}
	if _, exists := se["retry"]; exists {
		t.Fatalf("legacy field retry should not be present: %v", se["retry"])
	}
}

// ============================================
// Action/Selector Context Tests
// ============================================

func TestStructuredError_ActionAndSelector_OmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	result := mcp.StructuredErrorResponse(
		mcp.ErrExtTimeout, "Extension timed out", "Retry the command",
	)

	se := extractStructuredErrorJSON(t, result)

	if _, exists := se["action"]; exists {
		t.Error("action should be omitted when not set")
	}
	if _, exists := se["selector"]; exists {
		t.Error("selector should be omitted when not set")
	}
}

func TestStructuredError_ActionAndSelector_PresentWhenSet(t *testing.T) {
	t.Parallel()

	result := mcp.StructuredErrorResponse(
		mcp.ErrNoData, "Extension not connected", "Check extension",
		mcp.WithAction("click"), mcp.WithSelector("#submit-btn"),
	)

	se := extractStructuredErrorJSON(t, result)

	action, ok := se["action"].(string)
	if !ok || action != "click" {
		t.Errorf("action = %v, want 'click'", se["action"])
	}

	selector, ok := se["selector"].(string)
	if !ok || selector != "#submit-btn" {
		t.Errorf("selector = %v, want '#submit-btn'", se["selector"])
	}
}

// ============================================
// Smoke Tests: Stream 4 — Diagnostic Hints in Gate Errors
// ============================================

func TestSmoke_RequireExtension_ErrorContainsDiagnosticHint(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireExtension(req)
	if !blocked {
		t.Fatal("expected requireExtension to block when extension is disconnected")
	}

	se := extractStructuredError(t, resp)
	if se.Hint == "" {
		t.Fatal("mcp.StructuredError.Hint should not be empty for extension gate error")
	}
	for _, expected := range []string{"extension=DISCONNECTED", "pilot=", "tracked_tab=", "csp="} {
		if !strings.Contains(se.Hint, expected) {
			t.Errorf("hint should contain %q, got: %s", expected, se.Hint)
		}
	}
}

func TestSmoke_RequirePilot_ErrorContainsDiagnosticHint(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetPilotEnabled(false)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requirePilot(req)
	if !blocked {
		t.Fatal("expected requirePilot to block when pilot is disabled")
	}

	se := extractStructuredError(t, resp)
	if se.Hint == "" {
		t.Fatal("mcp.StructuredError.Hint should not be empty for pilot gate error")
	}
	if !strings.Contains(se.Hint, "pilot=DISABLED") {
		t.Errorf("hint should contain 'pilot=DISABLED', got: %s", se.Hint)
	}
}

func TestSmoke_RequireTabTracking_ErrorContainsDiagnosticHint(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	// No tab tracking set

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireTabTracking(req)
	if !blocked {
		t.Fatal("expected requireTabTracking to block when no tab is tracked")
	}

	se := extractStructuredError(t, resp)
	if se.Hint == "" {
		t.Fatal("mcp.StructuredError.Hint should not be empty for tab tracking gate error")
	}
	if !strings.Contains(se.Hint, "tracked_tab=NONE") {
		t.Errorf("hint should contain 'tracked_tab=NONE', got: %s", se.Hint)
	}
}

func TestSmoke_RequireCSPClear_ErrorContainsDiagnosticHint(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireCSPClear(req, "main")
	if !blocked {
		t.Fatal("expected requireCSPClear to block world=main when CSP restricts script_exec")
	}

	se := extractStructuredError(t, resp)
	if se.Hint == "" {
		t.Fatal("mcp.StructuredError.Hint should not be empty for CSP gate error")
	}
	if !strings.Contains(se.Hint, "csp=RESTRICTED(script_exec)") {
		t.Errorf("hint should contain 'csp=RESTRICTED(script_exec)', got: %s", se.Hint)
	}
}

// Helpers: extractStructuredErrorJSON and extractJSONFromText are in tools_test_helpers_test.go.

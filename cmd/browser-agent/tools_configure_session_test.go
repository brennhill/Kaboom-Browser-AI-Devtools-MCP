// Purpose: Tests for configure session store/load operations.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_configure_session_test.go — Coverage tests for load session and test boundary handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// handleLoadSessionContext — 58% → 100%
// ============================================

func TestToolLoadSessionContext_WithStore(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"load"}`)
	if !ok {
		t.Fatal("load should return result")
	}
	if result.IsError {
		t.Fatalf("load should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
}

func TestToolLoadSessionContext_NilStore(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	// Force the canonical session handler's nil-store path.
	env.handler.sessionStoreImpl = nil
	env.handler.configureSessions = toolconfigure.NewSessionHandler(toolconfigure.SessionDeps{
		RequireStore:      env.handler.requireSessionStore,
		InvalidateSummary: env.handler.invalidateSummaryPref,
		SetActiveCodebase: env.server.SetActiveCodebase,
	}, nil, env.handler.sessionManager)

	args := json.RawMessage(`{"what":"load"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.configureSessions.Load(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("load with nil store should return isError:true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "not_initialized") && !strings.Contains(text, "not initialized") {
		t.Fatalf("error should mention not_initialized, got: %s", text)
	}
}

// ============================================
// toolConfigureTestBoundaryEnd — 63% → 100%
// ============================================

func TestToolConfigureTestBoundaryEnd_Success(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	// Must start a boundary first so end succeeds.
	startResult, startOK := env.callConfigure(t, `{"what":"test_boundary_start","test_id":"test-123"}`)
	if !startOK {
		t.Fatal("test_boundary_start should return result")
	}
	if startResult.IsError {
		t.Fatalf("test_boundary_start should not error, got: %s", startResult.Content[0].Text)
	}

	result, ok := env.callConfigure(t, `{"what":"test_boundary_end","test_id":"test-123"}`)
	if !ok {
		t.Fatal("test_boundary_end should return result")
	}
	if result.IsError {
		t.Fatalf("test_boundary_end should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
	if testID, _ := data["test_id"].(string); testID != "test-123" {
		t.Fatalf("test_id = %q, want test-123", testID)
	}
	if wasActive, _ := data["was_active"].(bool); !wasActive {
		t.Fatal("was_active should be true")
	}
}

func TestToolConfigureTestBoundaryEnd_MissingTestID(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"test_boundary_end"}`)
	if !ok {
		t.Fatal("test_boundary_end should return result")
	}
	if !result.IsError {
		t.Fatal("test_boundary_end without test_id should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "test_id") {
		t.Errorf("error should mention test_id, got: %s", text)
	}
}

func TestToolConfigureTestBoundaryEnd_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolConfigureTestBoundaryEnd(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

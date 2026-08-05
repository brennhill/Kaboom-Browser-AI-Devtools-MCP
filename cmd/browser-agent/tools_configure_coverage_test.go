// Purpose: Coverage-expansion tests for browser-agent edge cases and branch paths.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_configure_coverage_test.go — Coverage tests for configure sub-handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// toolconfigure.HandleClear — clear-mode coverage
// ============================================

func TestToolConfigureClear_AllBuffers(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"clear","buffer":"all"}`)
	if !ok {
		t.Fatal("clear all should return result")
	}
	if result.IsError {
		t.Fatalf("clear all should not error, got: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "clear") {
		t.Errorf("response should mention clear, got: %s", text)
	}
}

func TestToolConfigureClear_DefaultsToAll(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	// No buffer param — should default to "all"
	result, ok := env.callConfigure(t, `{"what":"clear"}`)
	if !ok {
		t.Fatal("clear (default) should return result")
	}
	if result.IsError {
		t.Fatalf("clear default should not error, got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureClear_Network(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"clear","buffer":"network"}`)
	if !ok {
		t.Fatal("clear network should return result")
	}
	if result.IsError {
		t.Fatalf("clear network should not error, got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureClear_WebSocket(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"clear","buffer":"websocket"}`)
	if !ok {
		t.Fatal("clear websocket should return result")
	}
	if result.IsError {
		t.Fatalf("clear websocket should not error, got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureClear_Actions(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"clear","buffer":"actions"}`)
	if !ok {
		t.Fatal("clear actions should return result")
	}
	if result.IsError {
		t.Fatalf("clear actions should not error, got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureClear_Logs(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"clear","buffer":"logs"}`)
	if !ok {
		t.Fatal("clear logs should return result")
	}
	if result.IsError {
		t.Fatalf("clear logs should not error, got: %s", result.Content[0].Text)
	}
}

func TestToolConfigureClear_UnknownBuffer(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"clear","buffer":"invalid_buffer"}`)
	if !ok {
		t.Fatal("clear invalid should return result")
	}
	if !result.IsError {
		t.Fatal("clear invalid buffer should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "invalid_buffer") {
		t.Errorf("error should mention invalid buffer name, got: %s", text)
	}
}

func TestToolConfigureClear_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := toolconfigure.HandleClear(toolconfigure.ClearTargets{
		Capture:  env.capture,
		Resetter: newRuntimeResetter(env.capture),
		ClearLogs: func() int {
			count := env.server.logs.EntryCount()
			env.server.logs.ClearEntries()
			return count
		},
		Inbox:       env.server.pushInbox,
		Annotations: env.handler.annotationStore,
	}, req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

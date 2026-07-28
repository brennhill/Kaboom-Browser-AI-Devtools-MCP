// Purpose: Tests for observe tool scope filtering.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_scope_test.go — Tests for scope filtering in errors/logs observe handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

func TestGetBrowserErrors_InvalidScope(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{"scope": "bogus"})
	resp := observe.GetBrowserErrors(buildObserveReadDeps(h), req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Error("did not expect hard error for invalid scope")
	}
	if !strings.Contains(result.Content[0].Text, `"param_hint":"Unknown scope bogus ignored`) {
		t.Errorf("expected scope param_hint, got: %s", result.Content[0].Text)
	}
}

func TestGetBrowserErrors_ValidScopes(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, scope := range []string{"current_page", "all", ""} {
		args, _ := json.Marshal(map[string]any{"scope": scope})
		resp := observe.GetBrowserErrors(buildObserveReadDeps(h), req, args)
		var result mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("scope=%q unmarshal: %v", scope, err)
		}
		if result.IsError {
			t.Errorf("scope=%q should not error", scope)
		}
	}
}

func TestGetBrowserLogs_InvalidScope(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{"scope": "invalid"})
	resp := observe.GetBrowserLogs(buildObserveReadDeps(h), req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Error("did not expect hard error for invalid scope")
	}
	if !strings.Contains(result.Content[0].Text, `"param_hint":"Unknown scope invalid ignored`) {
		t.Errorf("expected scope param_hint, got: %s", result.Content[0].Text)
	}
}

func TestGetBrowserLogs_ValidScopes(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, scope := range []string{"current_page", "all", ""} {
		args, _ := json.Marshal(map[string]any{"scope": scope})
		resp := observe.GetBrowserLogs(buildObserveReadDeps(h), req, args)
		var result mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("scope=%q unmarshal: %v", scope, err)
		}
		if result.IsError {
			t.Errorf("scope=%q should not error", scope)
		}
	}
}

func TestGetBrowserErrors_ScopeInResponse(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	args, _ := json.Marshal(map[string]any{"scope": "all"})
	resp := observe.GetBrowserErrors(buildObserveReadDeps(h), req, args)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, `"scope":"all"`) {
		t.Error("expected scope=all in response")
	}
}

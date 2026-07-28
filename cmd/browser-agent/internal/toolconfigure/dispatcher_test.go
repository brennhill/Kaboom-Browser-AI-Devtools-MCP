// dispatcher_test.go — Verifies canonical configure mode dispatch.

package toolconfigure

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestDispatcherRoutesConfiguredAction(t *testing.T) {
	called := false
	dispatcher := NewDispatcher(map[string]Handler{
		"health": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			called = true
			return mcp.Succeed(req, "healthy", map[string]any{"status": "ok"})
		},
	})

	response := dispatcher.Handle(
		mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1},
		json.RawMessage(`{"what":"health"}`),
	)

	if !called {
		t.Fatal("configured action was not called")
	}
	if response.Error != nil {
		t.Fatalf("unexpected dispatch error: %#v", response.Error)
	}
}

func TestDispatcherRejectsUnknownActionWithCanonicalList(t *testing.T) {
	dispatcher := NewDispatcher(map[string]Handler{
		"doctor": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			return mcp.Succeed(req, "doctor", nil)
		},
		"health": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			return mcp.Succeed(req, "health", nil)
		},
	})

	response := dispatcher.Handle(
		mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1},
		json.RawMessage(`{"what":"missing"}`),
	)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	if !result.IsError {
		t.Fatal("unknown action should fail")
	}
	encoded, _ := json.Marshal(response)
	if string(encoded) == "" || !containsAll(string(encoded), "doctor", "health") {
		t.Fatalf("response does not list canonical actions: %s", encoded)
	}
}

func containsAll(value string, values ...string) bool {
	for _, candidate := range values {
		if !strings.Contains(value, candidate) {
			return false
		}
	}
	return true
}

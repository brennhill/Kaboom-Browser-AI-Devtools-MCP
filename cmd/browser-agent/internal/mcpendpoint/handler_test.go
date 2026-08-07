// handler_test.go — Verifies MCP endpoint composition and backend routing.

package mcpendpoint

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpcall"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type endpointExecutor struct{}

func (endpointExecutor) HandleToolCall(request mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
	if name != "observe" {
		return mcp.JSONRPCResponse{}, false
	}
	return mcp.Succeed(request, "ok", map[string]any{"status": "ready"}), true
}

func TestHandlerRoutesConfiguredBackendAndNotifications(t *testing.T) {
	captured := capture.NewCapture()
	handler := New(Config{Version: "0.9.0", Runtime: appruntime.New("0.9.0")}, mcpcall.Backend{
		Executor: endpointExecutor{},
		Capture:  captured,
		Schemas: []mcp.MCPTool{{
			Name: "observe", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		}},
	})

	response := handler.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{}}`),
	})
	if response == nil || response.Error != nil {
		t.Fatalf("tool response = %#v", response)
	}
	if notification := handler.HandleRequest(mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, Method: "notifications/initialized"}); notification != nil {
		t.Fatalf("notification response = %#v, want nil", notification)
	}
}

func TestHandlerUsesConfiguredHostPolicies(t *testing.T) {
	warnings := []string{"host warning"}
	handler := New(Config{
		Version:       "0.9.0",
		Runtime:       appruntime.New("0.9.0"),
		AddWarning:    func(value string) { warnings = append(warnings, value) },
		DrainWarnings: func() []string { drained := warnings; warnings = nil; return drained },
		PendingAudit:  func() bool { return true },
	}, mcpcall.Backend{
		Executor: endpointExecutor{},
		Schemas: []mcp.MCPTool{{
			Name: "observe", InputSchema: map[string]any{
				"type": "object", "properties": map[string]any{"what": map[string]any{"type": "string"}},
			},
		}},
	})

	response := handler.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion, ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"unknown":true}}`),
	})
	if response == nil || response.Error != nil || len(warnings) != 0 {
		t.Fatalf("policy response = %#v warnings=%v", response, warnings)
	}
	for _, want := range []string{"host warning", "unknown parameter", "ACTION REQUIRED"} {
		if !strings.Contains(string(response.Result), want) {
			t.Errorf("response omitted %q: %s", want, response.Result)
		}
	}
}

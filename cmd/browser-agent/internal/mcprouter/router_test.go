// router_test.go — Tests deterministic JSON-RPC validation and method routing.

package mcprouter

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestHandleValidatesEnvelopeAndNotificationSemantics(t *testing.T) {
	var request mcp.JSONRPCRequest
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":true,"method":"ping"}`), &request); err != nil {
		t.Fatal(err)
	}
	invalidID := Handle(request, Config{})
	if invalidID == nil || invalidID.Error == nil || invalidID.Error.Code != -32600 || invalidID.ID != nil {
		t.Fatalf("invalid ID response = %#v", invalidID)
	}
	if response := Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", Method: "ping"}, Config{}); response != nil {
		t.Fatalf("notification response = %#v, want nil", response)
	}
	invalidVersion := Handle(mcp.JSONRPCRequest{JSONRPC: "1.0", ID: 1, Method: "ping"}, Config{})
	if invalidVersion == nil || invalidVersion.Error == nil || invalidVersion.Error.Code != -32600 {
		t.Fatalf("invalid version response = %#v", invalidVersion)
	}
}

func TestHandleRoutesStaticProtocolAndToolMethods(t *testing.T) {
	config := Config{
		Version: "v-test",
		Schemas: []mcp.MCPTool{{Name: "observe"}},
		ToolCall: func(request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{"called":true}`)}
		},
	}
	for _, method := range []string{"initialized", "ping", "prompts/list", "initialize", "tools/list", "resources/list", "resources/templates/list"} {
		response := Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: method, Method: method}, config)
		if response == nil || response.Error != nil || response.Result == nil {
			t.Fatalf("%s response = %#v", method, response)
		}
	}
	toolResponse := Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: json.RawMessage(`{"name":"observe","arguments":{}}`)}, config)
	if toolResponse == nil || string(toolResponse.Result) != `{"called":true}` {
		t.Fatalf("tools/call response = %#v", toolResponse)
	}
	unknown := Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 3, Method: "unknown"}, config)
	if unknown == nil || unknown.Error == nil || unknown.Error.Code != -32601 {
		t.Fatalf("unknown response = %#v", unknown)
	}
}

func TestHandleClampsDynamicResults(t *testing.T) {
	large := make([]byte, 1_100_000)
	for index := range large {
		large[index] = 'x'
	}
	raw := mcp.TextResponse(string(large))
	response := Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/call"}, Config{
		ToolCall: func(request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Result: raw}
		},
	})
	if response == nil || len(response.Result) >= len(raw) {
		t.Fatalf("clamped result size = %d, original = %d", len(response.Result), len(raw))
	}
}

// router_test.go — Tests deterministic JSON-RPC validation and method routing.

package mcprouter

import (
	"encoding/json"
	"strings"
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

func TestHandleNegotiatesVersionsAndRejectsInvalidVersionOrNullID(t *testing.T) {
	for clientVersion, expected := range map[string]string{
		"2024-11-05": "2024-11-05",
		"2025-06-18": "2025-06-18",
		"2099-01-01": "2025-06-18",
		"":           "2025-06-18",
	} {
		params := json.RawMessage(`{}`)
		if clientVersion != "" {
			params = json.RawMessage(`{"protocolVersion":"` + clientVersion + `"}`)
		}
		response := Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: params}, Config{Version: "v-test"})
		var result mcp.MCPInitializeResult
		if response == nil || response.Error != nil {
			t.Fatalf("initialize %q response = %#v", clientVersion, response)
		}
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatal(err)
		}
		if result.ProtocolVersion != expected {
			t.Fatalf("initialize %q negotiated %q, want %q", clientVersion, result.ProtocolVersion, expected)
		}
	}
	for _, version := range []string{"", "1.0"} {
		response := Handle(mcp.JSONRPCRequest{JSONRPC: version, ID: 1, Method: "ping"}, Config{})
		if response == nil || response.Error == nil || response.Error.Code != -32600 {
			t.Fatalf("JSON-RPC version %q response = %#v", version, response)
		}
	}
	var nullID mcp.JSONRPCRequest
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":null,"method":"ping"}`), &nullID); err != nil {
		t.Fatal(err)
	}
	response := Handle(nullID, Config{})
	if response == nil || response.Error == nil || response.Error.Code != -32600 || response.ID != nil {
		t.Fatalf("null ID response = %#v", response)
	}
}

// A size-backstop firing must reach the host. It means the mode that produced
// the response has no adequate limit of its own, and the clamp only ever said
// so in English inside the response body, where nothing could act on it.
func TestHandleReportsAnOversizedResponse(t *testing.T) {
	oversized, err := json.Marshal(mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: strings.Repeat("x", mcp.MaxResponseBytes+5000)}},
	})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	var gotMethod string
	var gotReport mcp.ClampReport
	response := Handle(
		mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"recordings"}}`)},
		Config{
			ToolCall: func(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
				return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: oversized}
			},
			OnOversizedResponse: func(method string, report mcp.ClampReport) {
				gotMethod, gotReport = method, report
			},
		},
	)
	if response == nil {
		t.Fatal("expected a response")
	}
	// "tools/call" would name every mode identically and tell an operator
	// nothing about which handler needs a limit.
	if gotMethod != "observe/recordings" {
		t.Errorf("reported subject = %q, want observe/recordings", gotMethod)
	}
	if !gotReport.Truncated || gotReport.OriginalBytes <= gotReport.LimitBytes {
		t.Errorf("report = %+v, want a truncation larger than the limit", gotReport)
	}
}

// A response within the limit must not raise the signal, or it becomes noise.
func TestHandleDoesNotReportResponsesWithinTheLimit(t *testing.T) {
	small, err := json.Marshal(mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: "ok"}}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	reported := false
	Handle(
		mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: "tools/call"},
		Config{
			ToolCall: func(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
				return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: small}
			},
			OnOversizedResponse: func(string, mcp.ClampReport) { reported = true },
		},
	)
	if reported {
		t.Error("a response within the limit must not be reported as oversized")
	}
}

// runtime_test.go — Tests for push capability negotiation and runtime state.

package pushapi

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

func TestExtractClientCapabilities_ClaudeCode(t *testing.T) {
	raw := json.RawMessage(`{
		"capabilities": {"sampling": {}},
		"clientInfo": {"name": "claude-code", "version": "1.0"}
	}`)
	caps := ExtractClientCapabilities(raw)
	if !caps.SupportsSampling {
		t.Fatal("should detect sampling support")
	}
	if caps.ClientName != "claude-code" {
		t.Fatalf("expected claude-code, got %s", caps.ClientName)
	}
}

func TestPushRuntimeFramingCallbackAndOutboundPayloads(t *testing.T) {
	var payloads [][]byte
	var framings []bridge.StdioFraming
	runtime := NewRuntime(func(payload []byte, framing bridge.StdioFraming) {
		payloads = append(payloads, append([]byte(nil), payload...))
		framings = append(framings, framing)
	})
	runtime.StoreFraming(bridge.StdioFramingContentLength)
	if runtime.Framing() != bridge.StdioFramingContentLength {
		t.Fatalf("framing = %v", runtime.Framing())
	}

	var changed push.ClientCapabilities
	runtime.OnCapabilitiesChange(func(caps push.ClientCapabilities) { changed = caps })
	wantCaps := push.ClientCapabilities{SupportsSampling: true, ClientName: "client"}
	runtime.SetCapabilities(wantCaps)
	if changed != wantCaps {
		t.Fatalf("callback caps = %+v", changed)
	}

	request := push.SamplingRequest{JSONRPC: "2.0", ID: 7, Method: "sampling/createMessage"}
	if err := runtime.SendSampling(request); err != nil {
		t.Fatal(err)
	}
	runtime.SendNotification("notifications/message", map[string]any{"message": "hello"})
	if len(payloads) != 2 || len(framings) != 2 {
		t.Fatalf("writes = %d payloads, %d framings", len(payloads), len(framings))
	}
	for index, payload := range payloads {
		if !json.Valid(payload) || framings[index] != bridge.StdioFramingContentLength {
			t.Fatalf("write %d = %s, %v", index, payload, framings[index])
		}
	}
}

func TestExtractClientCapabilitiesNullSampling(t *testing.T) {
	caps := ExtractClientCapabilities(json.RawMessage(`{"capabilities":{"sampling":null},"clientInfo":{"name":"codex"}}`))
	if caps.SupportsSampling || !caps.SupportsNotifications {
		t.Fatalf("capabilities = %+v", caps)
	}
}

func TestExtractClientCapabilities_NoSampling(t *testing.T) {
	raw := json.RawMessage(`{
		"capabilities": {},
		"clientInfo": {"name": "cursor"}
	}`)
	caps := ExtractClientCapabilities(raw)
	if caps.SupportsSampling {
		t.Fatal("should not detect sampling without field")
	}
	if caps.ClientName != "cursor" {
		t.Fatalf("expected cursor, got %s", caps.ClientName)
	}
}

func TestExtractClientCapabilities_Empty(t *testing.T) {
	caps := ExtractClientCapabilities(json.RawMessage(`{}`))
	if caps.SupportsSampling || caps.SupportsNotifications {
		t.Fatal("empty params should have no capabilities")
	}
}

func TestExtractClientCapabilities_Malformed(t *testing.T) {
	caps := ExtractClientCapabilities(json.RawMessage(`not json`))
	if caps.ClientName != "" {
		t.Fatal("malformed should return empty")
	}
}

func TestPushState_SetGet(t *testing.T) {
	runtime := NewRuntime(func([]byte, bridge.StdioFraming) {})
	runtime.SetCapabilities(ExtractClientCapabilities(json.RawMessage(`{
		"capabilities": {"sampling": {}},
		"clientInfo": {"name": "test-client"}
	}`)))

	caps := runtime.Capabilities()
	if caps.ClientName != "test-client" {
		t.Fatalf("expected test-client, got %s", caps.ClientName)
	}
	if !caps.SupportsSampling {
		t.Fatal("should support sampling")
	}
}

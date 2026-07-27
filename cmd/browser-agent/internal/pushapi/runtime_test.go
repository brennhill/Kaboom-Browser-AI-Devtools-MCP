// runtime_test.go — Tests for push capability negotiation and runtime state.

package pushapi

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
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

// routing_test.go — Verifies canonical-only tool routing.

package toolrouting

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func testRegistry() Registry[string] {
	return Registry[string]{
		Handlers: map[string]Handler[string]{
			"logs": func(owner string, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
				return mcp.Succeed(req, "handled by "+owner, map[string]any{"ok": true})
			},
		},
		Resolution: Resolution{ToolName: "observe", ValidModes: "logs"},
	}
}

func TestDispatchRequiresCanonicalWhat(t *testing.T) {
	response := Dispatch("router", mcp.JSONRPCRequest{ID: 1}, json.RawMessage(`{"action":"logs"}`), testRegistry())
	if !strings.Contains(string(response.Result), "Required parameter 'what' is missing") {
		t.Fatalf("selector alias should be rejected: %s", response.Result)
	}
}

func TestDispatchUsesCanonicalWhat(t *testing.T) {
	response := Dispatch("router", mcp.JSONRPCRequest{ID: 2}, json.RawMessage(`{"what":"logs"}`), testRegistry())
	if strings.Contains(string(response.Result), "is_error") {
		t.Fatalf("canonical selector should dispatch: %s", response.Result)
	}
}

func TestDispatchUnknownModeIncludesCapabilityRecovery(t *testing.T) {
	response := Dispatch("router", mcp.JSONRPCRequest{ID: 3}, json.RawMessage(`{"what":"network"}`), testRegistry())
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	for _, expected := range []string{"unknown_mode", "recovery_tool_call", "describe_capabilities", `"observe"`} {
		if !strings.Contains(text, expected) {
			t.Errorf("unknown-mode response missing %q: %s", expected, text)
		}
	}
}

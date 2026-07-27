// routing_test.go — Characterizes canonical tool routing and alias warnings.

package toolrouting

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestDispatchResolvesDeprecatedAliasAndWarns(t *testing.T) {
	registry := Registry[string]{
		Handlers: map[string]Handler[string]{
			"logs": func(owner string, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
				return mcp.Succeed(req, "handled by "+owner, map[string]any{"ok": true})
			},
		},
		AliasDefs:  []Alias{{JSONField: "action", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"}},
		Resolution: Resolution{ToolName: "observe", ValidModes: "logs"},
	}

	response := Dispatch("router", mcp.JSONRPCRequest{ID: 1}, json.RawMessage(`{"action":"logs"}`), registry)
	if !strings.Contains(string(response.Result), "deprecated") {
		t.Fatalf("alias warning missing from response: %s", response.Result)
	}
}

func TestDispatchRejectsConflictingCanonicalAndAliasModes(t *testing.T) {
	registry := Registry[struct{}]{
		Handlers:   map[string]Handler[struct{}]{},
		AliasDefs:  []Alias{{JSONField: "mode"}},
		Resolution: Resolution{ToolName: "analyze", ValidModes: "dom"},
	}

	response := Dispatch(struct{}{}, mcp.JSONRPCRequest{ID: 2}, json.RawMessage(`{"what":"dom","mode":"links"}`), registry)
	if !strings.Contains(string(response.Result), "Conflicting parameters") {
		t.Fatalf("conflict error missing from response: %s", response.Result)
	}
}

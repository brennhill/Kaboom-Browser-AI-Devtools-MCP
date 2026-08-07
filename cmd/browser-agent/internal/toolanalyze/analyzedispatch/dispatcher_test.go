// dispatcher_test.go — Tests canonical analyze-mode registration and routing.

package analyzedispatch

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestDispatcherRegistersNavigationPatterns(t *testing.T) {
	dispatcher := NewDispatcher(Config{})
	if !slices.Contains(dispatcher.ValidModes(), "navigation_patterns") {
		t.Fatalf("navigation_patterns missing from modes: %v", dispatcher.ValidModes())
	}
}

func TestDispatcherRoutesLinkHealthAndRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	called := false
	dispatcher := NewDispatcher(Config{Analyze: toolanalyze.Deps{
		EnqueuePendingQuery: func(_ mcp.JSONRPCRequest, query queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
			called = query.Type == "link_health"
			return mcp.JSONRPCResponse{}, false
		},
		MaybeWaitForCommand: func(req mcp.JSONRPCRequest, correlationID string, _ json.RawMessage, summary string) mcp.JSONRPCResponse {
			return mcp.Succeed(req, summary, map[string]any{"status": "queued", "correlation_id": correlationID})
		},
	}})
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	response := dispatcher.Handle(req, json.RawMessage(`{"what":"link_health","domain":"example.com"}`))
	if !called || response.Error != nil || !strings.Contains(string(response.Result), `correlation_id`) {
		t.Fatalf("link health dispatch = called:%t response:%s", called, response.Result)
	}
	for name, args := range map[string]json.RawMessage{
		"missing what": nil,
		"unknown mode": json.RawMessage(`{"what":"missing"}`),
		"invalid JSON": json.RawMessage(`{bad`),
	} {
		t.Run(name, func(t *testing.T) {
			response := dispatcher.Handle(req, args)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil || !result.IsError {
				t.Fatalf("response = %s, error=%v", response.Result, err)
			}
		})
	}
	for _, mode := range []string{"dom", "api_validation", "performance", "accessibility", "error_clusters", "navigation_patterns", "security_audit", "third_party_audit", "link_health"} {
		if !slices.Contains(dispatcher.ValidModes(), mode) {
			t.Errorf("analyze modes missing %q: %v", mode, dispatcher.ValidModes())
		}
	}
}

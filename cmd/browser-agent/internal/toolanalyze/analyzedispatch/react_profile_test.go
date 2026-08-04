// react_profile_test.go — Verifies opt-in React profile command routing.
// Docs: docs/features/feature/react-performance-profiling/index.md

package analyzedispatch

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestReactProfileQueuesLifecycleActions(t *testing.T) {
	for _, action := range []string{"start", "stop"} {
		t.Run(action, func(t *testing.T) {
			var queued queries.PendingQuery
			deps := toolanalyze.Deps{
				EnqueuePendingQuery: func(_ mcp.JSONRPCRequest, query queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
					queued = query
					return mcp.JSONRPCResponse{}, false
				},
				MaybeWaitForCommand: func(req mcp.JSONRPCRequest, _ string, _ json.RawMessage, _ string) mcp.JSONRPCResponse {
					return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"queued":true}`)}
				},
			}
			resp := HandleReactProfile(deps, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, json.RawMessage(`{"action":"`+action+`","tab_id":9}`))
			if resp.Error != nil || queued.Type != "react_profile" || queued.TabID != 9 {
				t.Fatalf("response=%+v queued=%+v", resp, queued)
			}
		})
	}
}

func TestReactProfileRejectsInvalidAction(t *testing.T) {
	resp := HandleReactProfile(toolanalyze.Deps{}, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, json.RawMessage(`{"action":"status"}`))
	if !bytes.Contains(resp.Result, []byte(`"isError":true`)) {
		t.Fatal("invalid action succeeded")
	}
}

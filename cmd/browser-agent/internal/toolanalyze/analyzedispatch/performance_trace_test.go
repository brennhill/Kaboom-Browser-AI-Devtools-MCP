// performance_trace_test.go — Verifies analyze CPU trace command routing.

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

func TestPerformanceTraceQueuesStartAndStop(t *testing.T) {
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
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
			args := json.RawMessage(`{"what":"performance_trace","action":"` + action + `","tab_id":17}`)
			resp := HandlePerformanceTrace(deps, req, args)
			if resp.Error != nil {
				t.Fatalf("unexpected error: %+v", resp.Error)
			}
			if queued.Type != "performance_trace" || queued.TabID != 17 {
				t.Fatalf("queued = %+v", queued)
			}
		})
	}
}

func TestPerformanceTraceRejectsInvalidAction(t *testing.T) {
	resp := HandlePerformanceTrace(toolanalyze.Deps{}, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, json.RawMessage(`{"action":"analyze"}`))
	if !bytes.Contains(resp.Result, []byte(`"isError":true`)) {
		t.Fatal("invalid action succeeded")
	}
}

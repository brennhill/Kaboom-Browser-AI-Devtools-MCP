// dom_test.go — Tests DOM inspection query construction.

package inspect

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

type fakeDeps struct {
	query queries.PendingQuery
	args  json.RawMessage
}

func (f *fakeDeps) EnqueuePendingQuery(_ mcp.JSONRPCRequest, query queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
	f.query = query
	return mcp.JSONRPCResponse{}, false
}

func (f *fakeDeps) MaybeWaitForCommand(req mcp.JSONRPCRequest, _ string, args json.RawMessage, summary string) mcp.JSONRPCResponse {
	f.args = args
	return mcp.Succeed(req, summary, map[string]any{"status": "queued"})
}

func (f *fakeDeps) deps() Deps {
	return Deps{
		EnqueuePendingQuery: f.EnqueuePendingQuery,
		MaybeWaitForCommand: f.MaybeWaitForCommand,
	}
}

func TestHandleDOMDefaultsSelector(t *testing.T) {
	t.Parallel()
	deps := &fakeDeps{}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := HandleDOM(deps.deps(), req, json.RawMessage(`{"tab_id":7}`))
	if response.Error != nil {
		t.Fatalf("response error = %v", response.Error)
	}
	if deps.query.Type != "dom" || deps.query.TabID != 7 {
		t.Fatalf("query = %#v", deps.query)
	}
	var args map[string]any
	if err := json.Unmarshal(deps.args, &args); err != nil {
		t.Fatal(err)
	}
	if args["selector"] != "*" {
		t.Fatalf("selector = %#v", args["selector"])
	}
}

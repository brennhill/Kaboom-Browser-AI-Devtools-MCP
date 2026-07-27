// handler_test.go — Tests quality-gate setup boundary behavior.

package qualitygates

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type fakeCodebase struct{ path string }

func (f fakeCodebase) GetActiveCodebase() string { return f.path }

func TestHandleRequiresActiveCodebase(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := Handle(fakeCodebase{}, req, nil)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected missing active codebase to fail")
	}
}

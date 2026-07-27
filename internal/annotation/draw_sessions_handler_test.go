// draw_sessions_handler_test.go — Tests persisted draw-session loading.

package annotation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestLoadDrawSessionRejectsTraversal(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := LoadDrawSession(NewStore(time.Minute), req, json.RawMessage(`{"file":"../secret.json"}`), t.TempDir(), nil)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected path traversal to fail")
	}
}

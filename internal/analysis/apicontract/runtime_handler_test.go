// runtime_handler_test.go — Tests API contract runtime MCP operations.

package apicontract

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestRuntimeHandleRejectsMissingOperation(t *testing.T) {
	t.Parallel()
	runtime := NewRuntime()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := runtime.Handle(req, json.RawMessage(`{}`), nil)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected missing operation to fail")
	}
}

func TestRuntimeClearAdvancesBodyOffset(t *testing.T) {
	t.Parallel()
	runtime := NewRuntime()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := runtime.Handle(req, json.RawMessage(`{"operation":"clear"}`), make([]types.NetworkBody, 3))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError || runtime.offset != 3 {
		t.Fatalf("is_error=%v offset=%d", result.IsError, runtime.offset)
	}
}

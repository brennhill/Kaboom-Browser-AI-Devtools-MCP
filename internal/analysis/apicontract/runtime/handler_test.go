// handler_test.go — Tests API contract runtime MCP operations.

package runtime

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestRuntimePackageRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("API contract runtime package has %d files; want at most 10 change-coupled owners", files)
	}
}

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

func TestRuntimeHandlesAnalyzeReportAndClearOperations(t *testing.T) {
	t.Parallel()
	runtime := NewRuntime()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	for operation, expected := range map[string]string{
		"analyze": `"operation":"analyze"`,
		"report":  `"operation":"report"`,
		"clear":   `"operation":"clear"`,
	} {
		t.Run(operation, func(t *testing.T) {
			response := runtime.Handle(req, json.RawMessage(`{"operation":"`+operation+`"}`), nil)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil {
				t.Fatal(err)
			}
			if result.IsError || len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, expected) {
				t.Fatalf("%s response = %+v", operation, result)
			}
		})
	}
}

func TestRuntimeRejectsMalformedAndUnknownOperations(t *testing.T) {
	t.Parallel()
	runtime := NewRuntime()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	for name, args := range map[string]json.RawMessage{
		"malformed": json.RawMessage(`{bad`),
		"unknown":   json.RawMessage(`{"operation":"invalid"}`),
	} {
		t.Run(name, func(t *testing.T) {
			response := runtime.Handle(req, args, nil)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil {
				t.Fatal(err)
			}
			if !result.IsError {
				t.Fatalf("%s unexpectedly succeeded: %+v", name, result)
			}
		})
	}
}

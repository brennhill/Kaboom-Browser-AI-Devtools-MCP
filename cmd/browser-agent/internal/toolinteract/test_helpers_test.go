// test_helpers_test.go — Test support for toolinteract package tests.
package toolinteract

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// newTestHandler creates a minimal InteractActionHandler for unit tests.
func newTestHandler() *InteractActionHandler {
	return NewInteractActionHandler(&Deps{})
}

// newTestToolHandler wraps newTestHandler for compatibility with moved tests.
func newTestToolHandler() *testToolHandlerShim {
	return &testToolHandlerShim{h: newTestHandler()}
}

// testToolHandlerShim wraps InteractActionHandler to provide the interactAction() pattern
// that existing tests expect.
type testToolHandlerShim struct {
	h *InteractActionHandler
}

func (s *testToolHandlerShim) interactAction() *InteractActionHandler {
	return s.h
}

// parseToolResult is a test helper that unmarshals a mcp.JSONRPCResponse result into mcp.MCPToolResult.
func parseToolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}
	return result
}

// firstText extracts the first text content from a mcp.MCPToolResult.
func firstText(result mcp.MCPToolResult) string {
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text
		}
	}
	return ""
}

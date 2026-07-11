// test_helpers_test.go — Test support for interacthandler package tests.
package interacthandler

import (
	"encoding/json"
	"testing"
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

// parseToolResult is a test helper that unmarshals a JSONRPCResponse result into MCPToolResult.
func parseToolResult(t *testing.T, resp JSONRPCResponse) MCPToolResult {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}
	return result
}

// firstText extracts the first text content from a MCPToolResult.
func firstText(result MCPToolResult) string {
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text
		}
	}
	return ""
}

// Purpose: Provides canonical observation handler fixtures shared across package tests.
// Why: Keeps cross-family response decoding and dependency setup DRY without production facades.
// Docs: docs/features/feature/observe/index.md

package testsupport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/screenshot"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Deps returns a complete deterministic dependency set for observation tests.
func Deps(captured *capture.Capture) core.Deps {
	return core.Deps{
		Capture:              captured,
		Commands:             commandsOf(captured),
		Screenshots:          newScreenshots(captured),
		LogEntries:           func() ([]types.LogEntry, []time.Time) { return nil, nil },
		LogTotalAdded:        func() int64 { return 0 },
		IsConsoleNoise:       func(types.LogEntry) bool { return false },
		ExecuteA11yQuery:     func(string, []string, any, bool) (json.RawMessage, error) { return nil, nil },
		DiagnosticHintString: func() string { return "doctor hint" },
	}
}

// newScreenshots wires the screenshot use case to a capture's command owner and tracking state.
func newScreenshots(captured *capture.Capture) *screenshot.Service {
	return screenshot.New(screenshot.Deps{
		Commands: captured.Commands(),
		TrackingEnabled: func() bool {
			enabled, _, _ := captured.Extension().GetTrackingStatus()
			return enabled
		},
	})
}

// DecodeToolResult decodes a JSON-RPC response's MCP tool result.
func DecodeToolResult(t *testing.T, response mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	return result
}

// ExtractMCPJSON parses the structured JSON following an MCP text summary.
func ExtractMCPJSON(t *testing.T, response mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	result := DecodeToolResult(t, response)
	if len(result.Content) == 0 {
		t.Fatal("no content blocks in response")
	}
	text := result.Content[0].Text
	separator := strings.Index(text, "\n")
	if separator < 0 {
		t.Fatalf("no JSON separator in response text: %s", text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[separator+1:]), &data); err != nil {
		t.Fatalf("decode response JSON: %v", err)
	}
	return data
}

// commandsOf returns the capture's command owner, or a nil runner for a nil capture.
func commandsOf(captured *capture.Capture) queries.QueryRunner {
	if captured == nil {
		return nil
	}
	return captured.Commands()
}

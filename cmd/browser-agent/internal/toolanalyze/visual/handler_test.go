// handler_test.go — Tests visual-regression analyze handlers.

package visual

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

type fakeDeps struct {
	stored persistence.SessionStoreArgs
}

func (f *fakeDeps) CaptureScreenshot(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	return mcp.Succeed(req, "Screenshot captured", map[string]any{"path": "/tmp/current.png"})
}

func (f *fakeDeps) GetTrackingStatus() (bool, int, string) {
	return true, 7, "https://example.test"
}

func (f *fakeDeps) HasSessionStore() bool { return true }

func (f *fakeDeps) HandleSessionStore(args persistence.SessionStoreArgs) (json.RawMessage, error) {
	f.stored = args
	return json.RawMessage(`{}`), nil
}

func TestSaveBaselinePersistsScreenshotMetadata(t *testing.T) {
	t.Parallel()
	deps := &fakeDeps{}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := SaveBaseline(deps, req, json.RawMessage(`{"name":"home"}`))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatal("expected baseline save to succeed")
	}
	if deps.stored.Namespace != "visual_baselines" || deps.stored.Key != "home" {
		t.Fatalf("stored args = %#v", deps.stored)
	}
}

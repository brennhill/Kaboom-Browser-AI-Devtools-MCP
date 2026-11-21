// page_screenshot_test.go — Tests the MCP adapter's mapping of screenshot use-case failures and results.

package page

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/screenshot"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/testsupport"
)

func TestScreenshotFailureMapsEveryKindToCanonicalMCPError(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	cases := map[screenshot.FailureKind][]string{
		screenshot.KindNotTracking:     {mcp.ErrNoData, "No tab is being tracked"},
		screenshot.KindInvalidFormat:   {mcp.ErrInvalidParam, "Invalid screenshot format: x"},
		screenshot.KindInvalidQuality:  {mcp.ErrInvalidParam, "Invalid quality: x (must be 1-100)"},
		screenshot.KindQueueFull:       {mcp.ErrQueueFull, "Command queue full: x"},
		screenshot.KindTimeout:         {mcp.ErrExtTimeout, "Screenshot capture timeout: x"},
		screenshot.KindMalformedResult: {mcp.ErrInvalidJSON, "Failed to parse screenshot result: x"},
		screenshot.KindExtensionError:  {mcp.ErrExtError, "Screenshot capture failed: x"},
	}
	for kind, want := range cases {
		resp := ScreenshotFailure(req, &screenshot.Failure{Kind: kind, Detail: "x"}, "hint")
		result := testsupport.DecodeToolResult(t, resp)
		if !result.IsError || !strings.Contains(string(resp.Result), want[0]) || !strings.Contains(result.Content[0].Text, want[1]) {
			t.Fatalf("%s mapped to %s", kind, resp.Result)
		}
	}
	if resp := ScreenshotFailure(req, errors.New("boom"), ""); !testsupport.DecodeToolResult(t, resp).IsError {
		t.Fatal("unclassified error must still fail")
	}
}

func TestScreenshotSuccessKeepsDataURLOutOfTextBlock(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 2}
	resp := screenshotSuccess(req, screenshot.Result{Fields: map[string]any{"path": "/p.png"}, DataURL: "data:image/png;base64,aGk="})
	result := testsupport.DecodeToolResult(t, resp)
	if len(result.Content) != 2 || result.Content[1].Type != "image" || result.Content[1].Data != "aGk=" {
		t.Fatalf("content = %+v", result.Content)
	}
	if strings.Contains(result.Content[0].Text, "base64") {
		t.Fatalf("text block duplicates image: %s", result.Content[0].Text)
	}
	raw, _ := json.Marshal(result.Content[0])
	if !strings.Contains(string(raw), "/p.png") {
		t.Fatalf("text block missing fields: %s", raw)
	}
}

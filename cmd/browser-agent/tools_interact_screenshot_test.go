// tools_interact_screenshot_test.go — Tests for include_screenshot on interact actions (#317).
// Validates that interact actions can optionally return a screenshot alongside the action result.
package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// TestInteract_IncludeScreenshot_Schema verifies the include_screenshot parameter
// is accepted by the interact schema without error.
func TestInteract_IncludeScreenshot_Schema(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com")
	env.capture.SimulateExtensionConnectForTest()

	// Send a click action with include_screenshot=true
	// The action will timeout since no extension is processing, but the schema should accept the param
	args := json.RawMessage(`{"what":"click","selector":"button","include_screenshot":true,"sync":false}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolInteract(req, args)

	result := parseToolResult(t, resp)
	// Should be queued (async mode), NOT an error about invalid param
	if result.IsError {
		text := result.Content[0].Text
		if strings.Contains(text, "include_screenshot") {
			t.Fatalf("include_screenshot should be accepted as a valid parameter, got error: %s", text)
		}
	}
}

// TestInteract_IncludeScreenshot_AppendsImageBlock verifies that when
// include_screenshot=true is set on an interact action, a screenshot is
// captured after the action and included as an inline image content block.
func TestInteract_IncludeScreenshot_AppendsImageBlock(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com")
	env.capture.SimulateExtensionConnectForTest()

	args := json.RawMessage(`{"what":"click","selector":"button","include_screenshot":true}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = env.handler.toolInteract(req, args)
		close(done)
	}()

	// Wait for the DOM action query to be created and complete it
	var domQueryID string
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		pending := env.capture.Queries().GetPendingQueries()
		for _, q := range pending {
			if q.Type == "dom_action" {
				domQueryID = q.CorrelationID
				break
			}
		}
		if domQueryID != "" {
			break
		}
	}
	if domQueryID == "" {
		t.Fatal("no dom_action query found in pending queries")
	}

	// Complete the DOM action
	actionResult, _ := json.Marshal(map[string]any{
		"success": true,
		"message": "Clicked button",
	})
	env.capture.Queries().ApplyCommandResult(domQueryID, "complete", actionResult, "")

	// Wait for the screenshot query to be created (triggered after action completion)
	var screenshotQueryID string
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		pending := env.capture.Queries().GetPendingQueries()
		for _, q := range pending {
			if q.Type == "screenshot" {
				screenshotQueryID = q.ID
				break
			}
		}
		if screenshotQueryID != "" {
			break
		}
	}
	if screenshotQueryID == "" {
		// The test response may have already returned if screenshot wasn't triggered.
		// Check if we need to wait longer or if the implementation is missing.
		select {
		case <-done:
			// Response came back - check if it has an image block
			var result mcp.MCPToolResult
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}
			t.Fatalf("no screenshot query was created after action completion. Result blocks: %d", len(result.Content))
		case <-time.After(2 * time.Second):
			t.Fatal("no screenshot query found and handler still blocking")
		}
	}

	// Complete the screenshot query with fake image data
	fakeImageData := []byte("fake-screenshot-after-click")
	base64Data := base64.StdEncoding.EncodeToString(fakeImageData)
	screenshotResult, _ := json.Marshal(map[string]any{
		"filename": "example.com-20240101-120001.jpg",
		"path":     "/tmp/screenshots/example.com-20240101-120001.jpg",
		"data_url": "data:image/jpeg;base64," + base64Data,
	})
	env.capture.Queries().SetQueryResult(screenshotQueryID, screenshotResult)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("toolInteract timed out")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Should have text block (action result) + image block (screenshot)
	var hasImageBlock bool
	for _, block := range result.Content {
		if block.Type == "image" {
			hasImageBlock = true
			if block.Data != base64Data {
				t.Errorf("image data mismatch")
			}
			if block.MimeType != "image/jpeg" {
				t.Errorf("image mimeType = %q, want 'image/jpeg'", block.MimeType)
			}
		}
	}
	if !hasImageBlock {
		t.Fatalf("expected an image content block in response, got %d blocks: types=%v",
			len(result.Content), contentBlockTypes(result.Content))
	}
}

// TestInteract_IncludeScreenshot_DefaultFalse verifies that when include_screenshot
// is not set, no screenshot is captured after the action.
func TestInteract_IncludeScreenshot_DefaultFalse(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com")
	env.capture.SimulateExtensionConnectForTest()

	args := json.RawMessage(`{"what":"click","selector":"button"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = env.handler.toolInteract(req, args)
		close(done)
	}()

	// Wait for the DOM action query
	var domQueryID string
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		pending := env.capture.Queries().GetPendingQueries()
		for _, q := range pending {
			if q.Type == "dom_action" {
				domQueryID = q.CorrelationID
				break
			}
		}
		if domQueryID != "" {
			break
		}
	}
	if domQueryID == "" {
		t.Fatal("no dom_action query found")
	}

	// Complete the DOM action
	actionResult, _ := json.Marshal(map[string]any{"success": true})
	env.capture.Queries().ApplyCommandResult(domQueryID, "complete", actionResult, "")

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("toolInteract timed out")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Should NOT have any image blocks
	for _, block := range result.Content {
		if block.Type == "image" {
			t.Fatal("should not have image block when include_screenshot is not set")
		}
	}
}

// contentBlockTypes returns the types of all content blocks for diagnostic output.
func contentBlockTypes(blocks []mcp.MCPContentBlock) []string {
	types := make([]string, len(blocks))
	for i, b := range blocks {
		types[i] = b.Type
	}
	return types
}

// Ensure unused imports don't cause compilation errors.
var _ = queries.PendingQuery{}

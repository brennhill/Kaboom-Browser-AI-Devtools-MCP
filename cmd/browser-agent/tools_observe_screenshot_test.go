// tools_observe_screenshot_test.go — Tests for inline screenshot in observe response (#339).
// Validates that observe(what="screenshot") returns both file path text AND inline base64 image.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// TestGetScreenshot_InlineImageInResponse verifies that a successful screenshot
// returns both a text content block (file path info) and an image content block
// (base64-encoded inline image).
func TestGetScreenshot_InlineImageInResponse(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	capturefixture.Track(env.capture, 1, "https://example.com")

	// Simulate extension returning screenshot result with data_url
	fakeImageData := []byte("fake-png-image-data-for-test")
	base64Data := base64.StdEncoding.EncodeToString(fakeImageData)
	screenshotResult := map[string]any{
		"filename": "example.com-20240101-120000.jpg",
		"path":     "/tmp/screenshots/example.com-20240101-120000.jpg",
		"data_url": "data:image/jpeg;base64," + base64Data,
	}

	// Launch GetScreenshot in a goroutine since it blocks on WaitForResult
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"screenshot"}`)

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = observe.GetScreenshot(buildObserveReadDeps(env.handler), req, args)
		close(done)
	}()

	// Wait for the pending query to be created, then set the result
	var queryID string
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		pending := env.capture.Queries().GetPendingQueries()
		for _, q := range pending {
			if q.Type == "screenshot" {
				queryID = q.ID
				break
			}
		}
		if queryID != "" {
			break
		}
	}
	if queryID == "" {
		t.Fatal("no screenshot query found in pending queries")
	}

	resultJSON, _ := json.Marshal(screenshotResult)
	env.capture.Queries().SetQueryResult(queryID, resultJSON)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetScreenshot timed out")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Should have at least 2 content blocks: text + image
	if len(result.Content) < 2 {
		t.Fatalf("expected at least 2 content blocks (text + image), got %d: %+v", len(result.Content), result.Content)
	}

	// First block should be text with screenshot info
	if result.Content[0].Type != "text" {
		t.Errorf("first content block type = %q, want 'text'", result.Content[0].Type)
	}
	if !strings.Contains(result.Content[0].Text, "Screenshot captured") {
		t.Errorf("first content block should contain 'Screenshot captured', got: %s", result.Content[0].Text)
	}

	// Find the image content block
	var imageBlock *mcp.MCPContentBlock
	for i := range result.Content {
		if result.Content[i].Type == "image" {
			imageBlock = &result.Content[i]
			break
		}
	}
	if imageBlock == nil {
		t.Fatal("expected an image content block in response")
	}

	// Verify image block fields
	if imageBlock.Data != base64Data {
		t.Errorf("image data = %q, want %q", imageBlock.Data, base64Data)
	}
	if imageBlock.MimeType != "image/jpeg" {
		t.Errorf("image mimeType = %q, want 'image/jpeg'", imageBlock.MimeType)
	}

	// Verify base64 data is valid
	decoded, err := base64.StdEncoding.DecodeString(imageBlock.Data)
	if err != nil {
		t.Fatalf("image data is not valid base64: %v", err)
	}
	if string(decoded) != string(fakeImageData) {
		t.Errorf("decoded image data = %q, want %q", string(decoded), string(fakeImageData))
	}
}

// TestGetScreenshot_InlineImage_PNGFormat verifies that PNG format screenshots
// use the correct mimeType.
func TestGetScreenshot_InlineImage_PNGFormat(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	capturefixture.Track(env.capture, 1, "https://example.com")

	fakeImageData := []byte("fake-png-data")
	base64Data := base64.StdEncoding.EncodeToString(fakeImageData)
	screenshotResult := map[string]any{
		"filename": "example.com-20240101-120000.png",
		"path":     "/tmp/screenshots/example.com-20240101-120000.png",
		"data_url": "data:image/png;base64," + base64Data,
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"screenshot","format":"png"}`)

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = observe.GetScreenshot(buildObserveReadDeps(env.handler), req, args)
		close(done)
	}()

	var queryID string
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		pending := env.capture.Queries().GetPendingQueries()
		for _, q := range pending {
			if q.Type == "screenshot" {
				queryID = q.ID
				break
			}
		}
		if queryID != "" {
			break
		}
	}
	if queryID == "" {
		t.Fatal("no screenshot query found")
	}

	resultJSON, _ := json.Marshal(screenshotResult)
	env.capture.Queries().SetQueryResult(queryID, resultJSON)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetScreenshot timed out")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	var imageBlock *mcp.MCPContentBlock
	for i := range result.Content {
		if result.Content[i].Type == "image" {
			imageBlock = &result.Content[i]
			break
		}
	}
	if imageBlock == nil {
		t.Fatal("expected an image content block")
	}
	if imageBlock.MimeType != "image/png" {
		t.Errorf("mimeType = %q, want 'image/png'", imageBlock.MimeType)
	}
}

// TestGetScreenshot_NoDataURL_StillReturnsTextResult verifies backward compatibility:
// when the extension result does not include data_url, only the text block is returned.
func TestGetScreenshot_NoDataURL_StillReturnsTextResult(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	capturefixture.Track(env.capture, 1, "https://example.com")

	screenshotResult := map[string]any{
		"filename": "example.com-20240101-120000.jpg",
		"path":     "/tmp/screenshots/example.com-20240101-120000.jpg",
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"screenshot"}`)

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = observe.GetScreenshot(buildObserveReadDeps(env.handler), req, args)
		close(done)
	}()

	var queryID string
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		pending := env.capture.Queries().GetPendingQueries()
		for _, q := range pending {
			if q.Type == "screenshot" {
				queryID = q.ID
				break
			}
		}
		if queryID != "" {
			break
		}
	}
	if queryID == "" {
		t.Fatal("no screenshot query found")
	}

	resultJSON, _ := json.Marshal(screenshotResult)
	env.capture.Queries().SetQueryResult(queryID, resultJSON)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetScreenshot timed out")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Should have exactly 1 content block (text only, no image)
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block (text only), got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content block type = %q, want 'text'", result.Content[0].Type)
	}
}

func TestGetScreenshot_SaveTo_WritesFileAndReturnsPath(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	capturefixture.Track(env.capture, 1, "https://example.com")

	fakeImageData := []byte("save-to-test-image")
	base64Data := base64.StdEncoding.EncodeToString(fakeImageData)
	screenshotResult := map[string]any{
		"filename": "example.com-20240101-120000.png",
		"path":     "/tmp/screenshots/example.com-20240101-120000.png",
		"data_url": "data:image/png;base64," + base64Data,
	}

	savePath := filepath.Join(t.TempDir(), "captures", "manual", "audit-shot.png")
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"screenshot","save_to":"` + savePath + `"}`)

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = observe.GetScreenshot(buildObserveReadDeps(env.handler), req, args)
		close(done)
	}()

	var queryID string
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		pending := env.capture.Queries().GetPendingQueries()
		for _, q := range pending {
			if q.Type == "screenshot" {
				queryID = q.ID
				break
			}
		}
		if queryID != "" {
			break
		}
	}
	if queryID == "" {
		t.Fatal("no screenshot query found")
	}

	resultJSON, _ := json.Marshal(screenshotResult)
	env.capture.Queries().SetQueryResult(queryID, resultJSON)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetScreenshot timed out")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(result.Content) == 0 || result.Content[0].Type != "text" {
		t.Fatalf("expected text response block, got: %+v", result.Content)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("failed to parse response json: %v", err)
	}

	if got, want := responseData["save_to"], savePath; got != want {
		t.Fatalf("save_to = %v, want %q", got, want)
	}

	written, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("expected screenshot file at save_to path: %v", err)
	}
	if string(written) != string(fakeImageData) {
		t.Fatalf("written file bytes mismatch: got %q want %q", string(written), string(fakeImageData))
	}
}

func TestGetScreenshot_SaveTo_InvalidExtensionReturnsSaveError(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	capturefixture.Track(env.capture, 1, "https://example.com")

	fakeImageData := []byte("save-to-test-image")
	base64Data := base64.StdEncoding.EncodeToString(fakeImageData)
	screenshotResult := map[string]any{
		"filename": "example.com-20240101-120000.png",
		"path":     "/tmp/screenshots/example.com-20240101-120000.png",
		"data_url": "data:image/png;base64," + base64Data,
	}

	invalidPath := filepath.Join(t.TempDir(), "captures", "manual", "audit-shot.txt")
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"screenshot","save_to":"` + invalidPath + `"}`)

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = observe.GetScreenshot(buildObserveReadDeps(env.handler), req, args)
		close(done)
	}()

	var queryID string
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		pending := env.capture.Queries().GetPendingQueries()
		for _, q := range pending {
			if q.Type == "screenshot" {
				queryID = q.ID
				break
			}
		}
		if queryID != "" {
			break
		}
	}
	if queryID == "" {
		t.Fatal("no screenshot query found")
	}

	resultJSON, _ := json.Marshal(screenshotResult)
	env.capture.Queries().SetQueryResult(queryID, resultJSON)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetScreenshot timed out")
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(result.Content) == 0 || result.Content[0].Type != "text" {
		t.Fatalf("expected text response block, got: %+v", result.Content)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("failed to parse response json: %v", err)
	}

	saveErr, ok := responseData["save_to_error"].(string)
	if !ok || saveErr == "" {
		t.Fatalf("expected save_to_error in response, got: %v", responseData["save_to_error"])
	}
	if !strings.Contains(saveErr, ".png") && !strings.Contains(saveErr, ".jpg") {
		t.Fatalf("save_to_error should mention valid extensions, got: %q", saveErr)
	}

	if _, err := os.Stat(invalidPath); !os.IsNotExist(err) {
		t.Fatalf("invalid save_to path should not create file: stat err = %v", err)
	}
}

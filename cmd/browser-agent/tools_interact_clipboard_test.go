// tools_interact_clipboard_test.go — Tests for clipboard read/write interact actions.
//
// Tests verify parameter validation and pilot gating for clipboard_read and clipboard_write.
//
// Run: go test ./cmd/browser-agent -run "TestClipboard" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Parameter Validation: clipboard_write missing text
// ============================================

func TestClipboard_Write_MissingText(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"what":"clipboard_write"}`)
	resp := h.toolInteract(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("clipboard_write without text MUST return isError:true")
	}

	text := strings.ToLower(firstText(result))
	if !strings.Contains(text, "text") {
		t.Errorf("error should mention missing 'text' parameter\nGot: %s", firstText(result))
	}
}

// ============================================
// Parameter Validation: clipboard_write invalid JSON
// ============================================

func TestClipboard_Write_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	// The interact dispatcher parses the top-level "what" first, then delegates
	// to interactAction().handleClipboardWrite which re-parses args. We need to pass valid
	// top-level JSON so the dispatcher can route, but the clipboard handler
	// receives the full args. Since the dispatcher needs "what", we call
	// handleClipboardWrite directly with broken JSON for the inner parse.
	args := json.RawMessage(`{bad json`)
	resp := h.interactAction().HandleClipboardWrite(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("clipboard_write with invalid JSON MUST return isError:true")
	}

	text := firstText(result)
	if !strings.Contains(text, "invalid_json") {
		t.Errorf("error code should contain 'invalid_json'\nGot: %s", text)
	}
	if !strings.Contains(text, "Fix JSON syntax") {
		t.Errorf("error should include recovery action\nGot: %s", text)
	}
}

// ============================================
// Pilot Gating: clipboard_read
// ============================================

func TestClipboard_Read_PilotGating(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"what":"clipboard_read"}`)
	resp := h.toolInteract(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("clipboard_read with pilot disabled MUST return isError:true")
	}

	text := strings.ToLower(firstText(result))
	if !strings.Contains(text, "pilot") {
		t.Errorf("clipboard_read error should mention pilot\nGot: %s", firstText(result))
	}
}

// ============================================
// Pilot Gating: clipboard_write
// ============================================

func TestClipboard_Write_PilotGating(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"what":"clipboard_write","text":"hello"}`)
	resp := h.toolInteract(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("clipboard_write with pilot disabled MUST return isError:true")
	}

	text := strings.ToLower(firstText(result))
	if !strings.Contains(text, "pilot") {
		t.Errorf("clipboard_write error should mention pilot\nGot: %s", firstText(result))
	}
}

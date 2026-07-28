// Purpose: Tests for generate HAR export output.
// Docs: docs/features/feature/test-generation/index.md

// tools_generate_har_test.go — MCP integration tests for HAR export.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/export"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func setupHARTestHandler(t *testing.T) *ToolHandler {
	t.Helper()
	srv, err := NewServer(t.TempDir()+"/test-har-export.jsonl", 10)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(srv, cap)
	return mcpHandler.toolHandler.(*ToolHandler)
}

func TestToolExportHAR_ReturnsHARJSON(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)
	handler.capture.Telemetry().AddNetworkBodiesForTest([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200, Duration: 50},
	})

	args, _ := json.Marshal(map[string]any{})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := toolgenerate.HandleExportHAR(handler, req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}

	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var harLog export.HARLog
	if err := json.Unmarshal([]byte(jsonPart), &harLog); err != nil {
		t.Fatalf("response text is not valid HAR JSON: %v", err)
	}
	if len(harLog.Log.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(harLog.Log.Entries))
	}
}

func TestToolExportHAR_SaveToFile(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)
	handler.capture.Telemetry().AddNetworkBodiesForTest([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200},
	})

	tmpDir := filepath.Join(".tmp-har-export", strings.ReplaceAll(t.Name(), "/", "_"))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})
	tmpFile := filepath.Join(tmpDir, "test-tool-export.har")

	args, _ := json.Marshal(map[string]any{"save_to": tmpFile})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call"}
	resp := toolgenerate.HandleExportHAR(handler, req, args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var summary export.HARExportResult
	if err := json.Unmarshal([]byte(jsonPart), &summary); err != nil {
		t.Fatalf("response text is not valid summary JSON: %v", err)
	}
	if summary.SavedTo != tmpFile {
		t.Errorf("expected saved_to %s, got %s", tmpFile, summary.SavedTo)
	}
	if summary.EntriesCount != 1 {
		t.Errorf("expected entries_count 1, got %d", summary.EntriesCount)
	}
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("expected file to exist")
	}
}

func TestToolExportHAR_Filters(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)
	handler.capture.Telemetry().AddNetworkBodiesForTest([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200},
		{Timestamp: "2026-01-23T10:30:01.000Z", Method: "POST", URL: "https://example.com/api", Status: 500},
		{Timestamp: "2026-01-23T10:30:02.000Z", Method: "GET", URL: "https://example.com/static", Status: 200},
	})

	args, _ := json.Marshal(map[string]any{"method": "POST", "status_min": 400})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call"}
	resp := toolgenerate.HandleExportHAR(handler, req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var harLog export.HARLog
	if err := json.Unmarshal([]byte(jsonPart), &harLog); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if len(harLog.Log.Entries) != 1 {
		t.Fatalf("expected 1 entry with filters, got %d", len(harLog.Log.Entries))
	}
	if harLog.Log.Entries[0].Request.Method != "POST" {
		t.Errorf("expected POST, got %s", harLog.Log.Entries[0].Request.Method)
	}
}

func TestToolExportHAR_PathTraversal(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)

	args, _ := json.Marshal(map[string]any{"save_to": "../../etc/passwd"})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`5`), Method: "tools/call"}
	resp := toolgenerate.HandleExportHAR(handler, req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error response for path traversal")
	}
}

func TestToolExportHAR_EmptyCapture(t *testing.T) {
	t.Parallel()
	handler := setupHARTestHandler(t)

	args, _ := json.Marshal(map[string]any{})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
	resp := toolgenerate.HandleExportHAR(handler, req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected no error for empty HAR export, got: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	jsonPart := text
	if lines := strings.SplitN(text, "\n", 2); len(lines) == 2 {
		jsonPart = lines[1]
	}
	var harLog export.HARLog
	if err := json.Unmarshal([]byte(jsonPart), &harLog); err != nil {
		t.Fatalf("Expected valid HAR JSON, got parse error: %v", err)
	}
	if len(harLog.Log.Entries) != 0 {
		t.Errorf("Expected 0 entries in empty HAR, got %d", len(harLog.Log.Entries))
	}
}

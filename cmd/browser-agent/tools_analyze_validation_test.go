// Purpose: Tests for analyze tool input validation.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	az "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/analyze"
)

func newAnalyzeValidationHandler(t *testing.T) *ToolHandler {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "analyze-validation.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(func() {
		server.logs.Shutdown(2 * time.Second)
	})
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	return mcpHandler.tools.Executor.(*ToolHandler)
}

func decodeToolResult(t *testing.T, raw json.RawMessage) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("json.Unmarshal(MCPToolResult) error = %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatalf("result has no content: %+v", result)
	}
	return result
}

func decodeToolJSONPayload(t *testing.T, result mcp.MCPToolResult) map[string]any {
	t.Helper()
	text := result.Content[0].Text
	idx := strings.IndexByte(text, '\n')
	if idx < 0 || idx+1 >= len(text) {
		t.Fatalf("tool result missing JSON payload: %q", text)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text[idx+1:]), &payload); err != nil {
		t.Fatalf("json.Unmarshal(payload) error = %v", err)
	}
	return payload
}

func TestToolValidateLinksValidationErrors(t *testing.T) {
	_ = newAnalyzeValidationHandler(t)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	t.Run("invalid JSON", func(t *testing.T) {
		resp := toolanalyze.HandleLinkValidation(req, json.RawMessage(`{invalid`), version)
		result := decodeToolResult(t, resp.Result)
		if !result.IsError {
			t.Fatalf("expected isError=true, got %+v", result)
		}
		if !strings.Contains(strings.ToLower(result.Content[0].Text), "invalid json") {
			t.Fatalf("unexpected error text: %q", result.Content[0].Text)
		}
	})

	t.Run("missing urls", func(t *testing.T) {
		resp := toolanalyze.HandleLinkValidation(req, json.RawMessage(`{}`), version)
		result := decodeToolResult(t, resp.Result)
		if !result.IsError {
			t.Fatalf("expected isError=true, got %+v", result)
		}
		if !strings.Contains(result.Content[0].Text, "urls") {
			t.Fatalf("error text should mention urls: %q", result.Content[0].Text)
		}
	})

	t.Run("no valid http urls", func(t *testing.T) {
		resp := toolanalyze.HandleLinkValidation(req, json.RawMessage(`{"urls":["ftp://x","javascript:alert(1)"]}`), version)
		result := decodeToolResult(t, resp.Result)
		if !result.IsError {
			t.Fatalf("expected isError=true, got %+v", result)
		}
		if !strings.Contains(strings.ToLower(result.Content[0].Text), "http") {
			t.Fatalf("error text should mention http/https urls: %q", result.Content[0].Text)
		}
	})
}

func TestToolValidateLinksExecutesAndReturnsResults(t *testing.T) {
	_ = newAnalyzeValidationHandler(t)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}

	// timeout_ms and max_workers intentionally out-of-bounds to exercise clamping.
	args := json.RawMessage(`{"urls":["http://127.0.0.1:1"],"timeout_ms":5,"max_workers":999}`)
	resp := toolanalyze.HandleLinkValidation(req, args, version)
	result := decodeToolResult(t, resp.Result)
	if result.IsError {
		t.Fatalf("expected success result, got error: %+v", result)
	}

	payload := decodeToolJSONPayload(t, result)
	if got, _ := payload["status"].(string); got != "completed" {
		t.Fatalf("status = %q, want completed", got)
	}
	if got, _ := payload["total"].(float64); int(got) != 1 {
		t.Fatalf("total = %v, want 1", payload["total"])
	}

	results, ok := payload["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("results = %#v, want single result", payload["results"])
	}
	entry, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("result entry type = %T, want map", results[0])
	}
	if got, _ := entry["code"].(string); got != "broken" {
		t.Fatalf("result code = %q, want broken", got)
	}
	if got, _ := entry["status"].(float64); int(got) != 0 {
		t.Fatalf("result status = %v, want 0 for transport error", entry["status"])
	}
	if errText, _ := entry["error"].(string); !strings.Contains(errText, "ssrf_blocked") {
		t.Fatalf("expected SSRF-blocked error, got %q", errText)
	}
}

func TestValidateLinksServerSideAndSingleLinkPrivateIP(t *testing.T) {
	_ = newAnalyzeValidationHandler(t) // keep to verify handler creation still works

	results := az.ValidateLinksServerSide([]string{
		"http://127.0.0.1:1",
		"https://127.0.0.1:1",
	}, 1000, 1, version)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for i, res := range results {
		if res.Code != "broken" {
			t.Fatalf("result[%d].Code = %q, want broken", i, res.Code)
		}
		if !strings.Contains(res.Error, "ssrf_blocked") {
			t.Fatalf("result[%d].Error = %q, want ssrf_blocked", i, res.Error)
		}
	}

	single := az.ValidateSingleLinkWithClient(az.NewLinkValidationClient(time.Second), "http://127.0.0.1:80", version)
	if single.Code != "broken" || single.Status != 0 {
		t.Fatalf("single link result = %+v, want broken with status 0", single)
	}
	if !strings.Contains(single.Error, "ssrf_blocked") {
		t.Fatalf("single link error = %q, want ssrf_blocked", single.Error)
	}
}

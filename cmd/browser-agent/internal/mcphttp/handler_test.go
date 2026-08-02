// handler_test.go — Tests MCP-over-HTTP request parsing and context capture.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package mcphttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestNewRequestContextReadsKaboomHeaders(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "http://localhost/mcp", nil)
	req.Header.Set("X-Kaboom-Ext-Session", "session-123")
	req.Header.Set("X-Kaboom-Client", "kaboom-extension/1.2.3")
	req.Header.Set("X-Kaboom-Extension-Version", "1.2.3")
	req.Header.Set("Authorization", "Bearer secret-token")

	ctx := newRequestContext(req, "9.9.9")

	if ctx.extSessionID != "session-123" {
		t.Fatalf("extSessionID = %q, want session-123", ctx.extSessionID)
	}
	if ctx.clientID != "kaboom-extension/1.2.3" {
		t.Fatalf("clientID = %q, want kaboom-extension/1.2.3", ctx.clientID)
	}
	if got := ctx.headers["Authorization"]; got != "[REDACTED]" {
		t.Fatalf("Authorization header = %q, want [REDACTED]", got)
	}
	if got := ctx.headers["X-Kaboom-Extension-Version"]; got != "1.2.3" {
		t.Fatalf("X-Kaboom-Extension-Version = %q, want 1.2.3", got)
	}
}

func TestServeHTTPControlCharactersRemainValidJSON(t *testing.T) {
	t.Parallel()
	handler := New(Config{
		Version:     "test",
		MaxBodySize: 4096,
		HandleRequest: func(req mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
			response := mcp.Succeed(req, "Page info", map[string]any{
				"url":   "https://example.test/path\x00next",
				"title": "before\bafter\nnext\tcolumn",
			})
			return &response
		},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call"}`,
	))
	request.Header.Set("Content-Type", "application/json")

	handler.ServeHTTP(recorder, request)

	body := recorder.Body.Bytes()
	if !json.Valid(body) {
		t.Fatalf("HTTP response contains invalid JSON: %q", body)
	}
	var response mcp.JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode JSON-RPC response: %v", err)
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode MCP result: %v", err)
	}
	parts := strings.SplitN(result.Content[0].Text, "\n", 2)
	if len(parts) != 2 || !json.Valid([]byte(parts[1])) {
		t.Fatalf("nested page payload contains invalid JSON: %q", result.Content[0].Text)
	}
}

func TestServeHTTPReplacesInvalidRawResultWithProtocolError(t *testing.T) {
	t.Parallel()
	handler := New(Config{
		Version:     "test",
		MaxBodySize: 4096,
		HandleRequest: func(req mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
			return &mcp.JSONRPCResponse{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      req.ID,
				Result:  json.RawMessage("{\"title\":\"before\x00after\"}"),
			}
		},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":7,"method":"tools/call"}`,
	))
	request.Header.Set("Content-Type", "application/json")

	handler.ServeHTTP(recorder, request)

	body := recorder.Body.Bytes()
	if !json.Valid(body) {
		t.Fatalf("serialization failure must still return valid JSON: %q", body)
	}
	var response mcp.JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode JSON-RPC response: %v", err)
	}
	if response.Error == nil || response.Error.Code != -32603 {
		t.Fatalf("response error = %+v, want internal serialization error", response.Error)
	}
	if response.ID != float64(7) {
		t.Fatalf("response id = %#v, want 7", response.ID)
	}
}

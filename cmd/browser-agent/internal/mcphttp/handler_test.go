// handler_test.go — Tests MCP-over-HTTP request parsing and context capture.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package mcphttp

import (
	"encoding/json"
	"errors"
	"io"
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

func TestServeHTTPNotificationAndResponseFraming(t *testing.T) {
	t.Parallel()
	handler := New(Config{
		Version:     "test",
		MaxBodySize: 4096,
		HandleRequest: func(request mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
			if !request.HasID() {
				return nil
			}
			response := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{}`)}
			return &response
		},
	})

	notification := httptest.NewRecorder()
	handler.ServeHTTP(notification, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	)))
	if notification.Code != http.StatusNoContent || notification.Body.Len() != 0 {
		t.Fatalf("notification status/body = %d/%q", notification.Code, notification.Body.String())
	}

	request := httptest.NewRecorder()
	handler.ServeHTTP(request, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
	)))
	body := request.Body.Bytes()
	if request.Code != http.StatusOK || len(body) == 0 || body[len(body)-1] != '\n' {
		t.Fatalf("request status/body = %d/%q", request.Code, body)
	}
	if len(body) > 1 && body[len(body)-2] == '\n' {
		t.Fatalf("response has double newline: %q", body)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestServeHTTPRejectsInvalidTransportInputs(t *testing.T) {
	t.Parallel()
	handler := New(Config{
		Version:     "test",
		MaxBodySize: 4096,
		HandleRequest: func(request mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
			response := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{}`)}
			return &response
		},
	})
	assertProtocolError := func(recorder *httptest.ResponseRecorder, code int) {
		t.Helper()
		var response mcp.JSONRPCResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Error == nil || response.Error.Code != code {
			t.Fatalf("response = %#v, want protocol error %d", response, code)
		}
	}

	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if method.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d", method.Code)
	}

	malformed := httptest.NewRecorder()
	handler.ServeHTTP(malformed, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0",`)))
	assertProtocolError(malformed, -32700)

	nonJSON := httptest.NewRecorder()
	nonJSONRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	nonJSONRequest.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(nonJSON, nonJSONRequest)
	assertProtocolError(nonJSON, -32700)

	readFailure := httptest.NewRecorder()
	readFailureRequest := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	readFailureRequest.Body = io.NopCloser(failingReader{})
	handler.ServeHTTP(readFailure, readFailureRequest)
	var readResponse mcp.JSONRPCResponse
	if err := json.Unmarshal(readFailure.Body.Bytes(), &readResponse); err != nil {
		t.Fatal(err)
	}
	if readResponse.Error == nil || readResponse.Error.Code != -32700 || readResponse.ID != nil {
		t.Fatalf("read failure response = %#v", readResponse)
	}

	for _, contentType := range []string{"", "application/json", "application/json; charset=utf-8"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"ping"}`))
		if contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("content type %q status = %d", contentType, recorder.Code)
		}
	}
}

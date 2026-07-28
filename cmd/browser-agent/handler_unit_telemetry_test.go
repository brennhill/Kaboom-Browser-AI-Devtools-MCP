// Purpose: Unit tests for browser-agent handler telemetry logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// handler_unit_telemetry_test.go — MCP handler tests for passive telemetry, HTTP transport,
// protocol negotiation, notification detection, and content-type validation.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// failReader always returns an error on Read.
type failReader struct{}

func (failReader) Read(_ []byte) (int, error) { return 0, errors.New("boom") }

// ioNopCloser is a local substitute for io.NopCloser to avoid pulling in io for one test path.
type ioNopCloser struct {
	Reader interface {
		Read([]byte) (int, error)
	}
}

func (n ioNopCloser) Read(p []byte) (int, error) { return n.Reader.Read(p) }
func (n ioNopCloser) Close() error               { return nil }

func TestMCPHandler_PassiveTelemetrySummaryDeltas(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	cap := capture.NewCapture()
	cap.Extension().SetTrackingStatusForTest(42, "https://tracked.test")

	// Seed baseline data before first call; first response should still report zero deltas.
	srv.logs.AddEntries([]types.LogEntry{{"level": "error", "message": "baseline error"}})
	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{Method: "GET", URL: "https://api.test/ok", Status: 200},
		{Method: "GET", URL: "https://api.test/fail", Status: 500},
	})
	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{Event: "message", ID: "ws-1"}})
	cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click", Timestamp: time.Now().UnixMilli()}})

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     cap,
		limiter: testLimiter{allowed: true},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcp.TextResponse("ok"),
			}, true
		},
	})
	req := mcp.JSONRPCRequest{
		JSONRPC:  "2.0",
		ID:       1,
		Method:   "tools/call",
		ClientID: "client-a",
		Params:   json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	}
	resp1 := h.HandleRequest(req)
	if resp1 == nil || resp1.Error != nil {
		t.Fatalf("first tools/call response = %+v, want success", resp1)
	}
	if changed := mustTelemetryChanged(t, resp1.Result); changed {
		t.Fatal("first call telemetry_changed = true, want false")
	}
	if _, ok := telemetrySummaryIfPresent(t, resp1.Result); ok {
		t.Fatal("first call should omit telemetry_summary in auto mode when nothing changed")
	}

	// Add new activity between calls; second response should report these deltas.
	srv.logs.AddEntries([]types.LogEntry{
		{"level": "error", "message": "TypeError"},
		{"level": "info", "message": "noise"},
	})
	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{Method: "GET", URL: "https://api.test/ok2", Status: 204},
		{Method: "GET", URL: "https://api.test/fail2", Status: 503},
	})
	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Event: "message", ID: "ws-2"},
		{Event: "message", ID: "ws-3"},
	})
	cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "type", Timestamp: time.Now().UnixMilli()}})

	req.ID = 2
	resp2 := h.HandleRequest(req)
	if resp2 == nil || resp2.Error != nil {
		t.Fatalf("second tools/call response = %+v, want success", resp2)
	}
	if changed := mustTelemetryChanged(t, resp2.Result); !changed {
		t.Fatal("second call telemetry_changed = false, want true")
	}
	summary2 := mustTelemetrySummary(t, resp2.Result)
	if got := mustTelemetryInt(t, summary2, "new_errors_since_last_call"); got != 1 {
		t.Fatalf("second call new_errors_since_last_call = %d, want 1", got)
	}
	if got := mustTelemetryInt(t, summary2, "new_network_requests_since_last_call"); got != 2 {
		t.Fatalf("second call new_network_requests_since_last_call = %d, want 2", got)
	}
	if got := mustTelemetryInt(t, summary2, "new_network_errors_since_last_call"); got != 1 {
		t.Fatalf("second call new_network_errors_since_last_call = %d, want 1", got)
	}
	if got := mustTelemetryInt(t, summary2, "new_websocket_events_since_last_call"); got != 2 {
		t.Fatalf("second call new_websocket_events_since_last_call = %d, want 2", got)
	}
	if got := mustTelemetryInt(t, summary2, "new_actions_since_last_call"); got != 1 {
		t.Fatalf("second call new_actions_since_last_call = %d, want 1", got)
	}
	if got, _ := summary2["trigger_tool"].(string); got != "observe" {
		t.Fatalf("trigger_tool = %q, want observe", got)
	}
}

func TestMCPHandler_PassiveTelemetryIsPerClient(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry-per-client.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	cap := capture.NewCapture()

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     cap,
		limiter: testLimiter{allowed: true},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcp.TextResponse("ok"),
			}, true
		},
	})
	reqA := mcp.JSONRPCRequest{
		JSONRPC:  "2.0",
		ID:       1,
		Method:   "tools/call",
		ClientID: "client-a",
		Params:   json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	}
	respA1 := h.HandleRequest(reqA)
	if respA1 == nil || respA1.Error != nil {
		t.Fatalf("client-a first call response = %+v, want success", respA1)
	}
	if changed := mustTelemetryChanged(t, respA1.Result); changed {
		t.Fatal("client-a first call telemetry_changed = true, want false")
	}
	if _, ok := telemetrySummaryIfPresent(t, respA1.Result); ok {
		t.Fatal("client-a first call should omit telemetry_summary in auto mode")
	}

	srv.logs.AddEntries([]types.LogEntry{{"level": "error", "message": "new error"}})

	reqA.ID = 2
	respA2 := h.HandleRequest(reqA)
	if respA2 == nil || respA2.Error != nil {
		t.Fatalf("client-a second call response = %+v, want success", respA2)
	}
	if changed := mustTelemetryChanged(t, respA2.Result); !changed {
		t.Fatal("client-a second call telemetry_changed = false, want true")
	}
	summaryA2 := mustTelemetrySummary(t, respA2.Result)
	if got := mustTelemetryInt(t, summaryA2, "new_errors_since_last_call"); got != 1 {
		t.Fatalf("client-a new_errors_since_last_call = %d, want 1", got)
	}

	reqB := mcp.JSONRPCRequest{
		JSONRPC:  "2.0",
		ID:       3,
		Method:   "tools/call",
		ClientID: "client-b",
		Params:   json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	}
	respB1 := h.HandleRequest(reqB)
	if respB1 == nil || respB1.Error != nil {
		t.Fatalf("client-b first call response = %+v, want success", respB1)
	}
	if changed := mustTelemetryChanged(t, respB1.Result); changed {
		t.Fatal("client-b first call telemetry_changed = true, want false")
	}
	if _, ok := telemetrySummaryIfPresent(t, respB1.Result); ok {
		t.Fatal("client-b first call should omit telemetry_summary in auto mode")
	}
}

func TestMCPHandler_PassiveTelemetryModeFullIncludesSummaryWithoutChanges(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry-full.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	srv.logs.SetTelemetryMode(telemetryModeFull)

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.TextResponse("ok")}, true
		},
	})

	resp := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}
	if changed := mustTelemetryChanged(t, resp.Result); changed {
		t.Fatal("telemetry_changed = true, want false on first call")
	}
	summary := mustTelemetrySummary(t, resp.Result)
	if got := mustTelemetryInt(t, summary, "new_errors_since_last_call"); got != 0 {
		t.Fatalf("new_errors_since_last_call = %d, want 0", got)
	}
}

func TestMCPHandler_PassiveTelemetryModeOffSuppressesTelemetryMetadata(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry-off.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	srv.logs.SetTelemetryMode(telemetryModeOff)

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.TextResponse("ok")}, true
		},
	})

	resp := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal(MCPToolResult) error = %v", err)
	}
	if result.Metadata != nil {
		if _, ok := result.Metadata["telemetry_changed"]; ok {
			t.Fatal("telemetry_changed should be omitted in off mode")
		}
		if _, ok := result.Metadata["telemetry_summary"]; ok {
			t.Fatal("telemetry_summary should be omitted in off mode")
		}
	}
}

func TestMCPHandler_PassiveTelemetryModePerCallOverride(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry-override.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	srv.logs.SetTelemetryMode(telemetryModeFull)

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.TextResponse("ok")}, true
		},
	})

	resp := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"logs","telemetry_mode":"off"}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal(MCPToolResult) error = %v", err)
	}
	if result.Metadata != nil {
		if _, ok := result.Metadata["telemetry_summary"]; ok {
			t.Fatal("telemetry_summary should be suppressed by per-call telemetry_mode=off")
		}
		if _, ok := result.Metadata["telemetry_changed"]; ok {
			t.Fatal("telemetry_changed should be suppressed by per-call telemetry_mode=off")
		}
	}
}

func TestMCPHandlerHandleHTTP(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(nil, "v-http")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"ok":true}`),
			}, true
		},
	})
	transport := newMCPHTTPHandler(h)

	getReq := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	getRR := httptest.NewRecorder()
	transport.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /mcp status = %d, want %d", getRR.Code, http.StatusMethodNotAllowed)
	}

	parseReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,`))
	parseRR := httptest.NewRecorder()
	transport.ServeHTTP(parseRR, parseReq)
	var parseResp mcp.JSONRPCResponse
	if err := json.Unmarshal(parseRR.Body.Bytes(), &parseResp); err != nil {
		t.Fatalf("json.Unmarshal(parse error response) error = %v", err)
	}
	if parseResp.Error == nil || parseResp.Error.Code != -32700 {
		t.Fatalf("parse error response = %+v, want parse error code -32700", parseResp)
	}

	notifyReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`))
	notifyRR := httptest.NewRecorder()
	transport.ServeHTTP(notifyRR, notifyReq)
	if notifyRR.Code != http.StatusNoContent {
		t.Fatalf("notification status = %d, want %d", notifyRR.Code, http.StatusNoContent)
	}

	callReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":99,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`))
	callRR := httptest.NewRecorder()
	transport.ServeHTTP(callRR, callReq)
	var callResp mcp.JSONRPCResponse
	if err := json.Unmarshal(callRR.Body.Bytes(), &callResp); err != nil {
		t.Fatalf("json.Unmarshal(call response) error = %v", err)
	}
	if callResp.Error != nil {
		t.Fatalf("tools/call HTTP response has error: %+v", callResp.Error)
	}

	readErrReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBuffer(nil))
	readErrReq.Body = ioNopCloser{Reader: failReader{}}
	readErrRR := httptest.NewRecorder()
	transport.ServeHTTP(readErrRR, readErrReq)
	var readErrResp mcp.JSONRPCResponse
	if err := json.Unmarshal(readErrRR.Body.Bytes(), &readErrResp); err != nil {
		t.Fatalf("json.Unmarshal(read error response) error = %v", err)
	}
	if readErrResp.Error == nil || readErrResp.Error.Code != -32700 {
		t.Fatalf("read error response = %+v, want code -32700", readErrResp)
	}
	// G1: Read errors must have null ID per JSON-RPC 2.0 spec section 5
	if readErrResp.ID != nil {
		t.Fatalf("read error id = %v, want null (JSON-RPC 2.0 spec: parse/read errors must have null id)", readErrResp.ID)
	}
}

func TestHandleRequest_RejectsInvalidJSONRPCVersion(t *testing.T) {
	t.Parallel()
	h := NewMCPHandler(nil, "v-test")

	// jsonrpc: "1.0" should be rejected
	resp := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "1.0", ID: 1, Method: "ping"})
	if resp == nil || resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("expected -32600 Invalid Request for jsonrpc 1.0, got %+v", resp)
	}

	// Empty jsonrpc should be rejected
	resp2 := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "", ID: 2, Method: "ping"})
	if resp2 == nil || resp2.Error == nil || resp2.Error.Code != -32600 {
		t.Fatalf("expected -32600 Invalid Request for empty jsonrpc, got %+v", resp2)
	}

	// jsonrpc: "2.0" should be accepted
	resp3 := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 3, Method: "ping"})
	if resp3 == nil || resp3.Error != nil {
		t.Fatalf("expected success for jsonrpc 2.0, got %+v", resp3)
	}
}

func TestHandleHTTP_RejectsNonJSONContentType(t *testing.T) {
	t.Parallel()
	h := NewMCPHandler(nil, "v-test")
	transport := newMCPHTTPHandler(h)

	// text/plain should be rejected
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	transport.ServeHTTP(rr, req)

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("expected -32700 for non-JSON Content-Type, got %+v", resp)
	}

	// No Content-Type header should be accepted (lenient)
	req2 := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"ping"}`))
	rr2 := httptest.NewRecorder()
	transport.ServeHTTP(rr2, req2)

	var resp2 mcp.JSONRPCResponse
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if resp2.Error != nil {
		t.Fatalf("empty Content-Type should be accepted, got error: %+v", resp2.Error)
	}

	// application/json should be accepted
	req3 := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"ping"}`))
	req3.Header.Set("Content-Type", "application/json")
	rr3 := httptest.NewRecorder()
	transport.ServeHTTP(rr3, req3)

	var resp3 mcp.JSONRPCResponse
	if err := json.Unmarshal(rr3.Body.Bytes(), &resp3); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if resp3.Error != nil {
		t.Fatalf("application/json should be accepted, got error: %+v", resp3.Error)
	}

	// application/json; charset=utf-8 should be accepted
	req4 := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"ping"}`))
	req4.Header.Set("Content-Type", "application/json; charset=utf-8")
	rr4 := httptest.NewRecorder()
	transport.ServeHTTP(rr4, req4)

	var resp4 mcp.JSONRPCResponse
	if err := json.Unmarshal(rr4.Body.Bytes(), &resp4); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if resp4.Error != nil {
		t.Fatalf("application/json; charset=utf-8 should be accepted, got error: %+v", resp4.Error)
	}
}

func TestHandleInitialize_NegotiatesProtocolVersion(t *testing.T) {
	t.Parallel()
	h := NewMCPHandler(nil, "v-test")

	tests := []struct {
		name            string
		clientVersion   string
		expectedVersion string
	}{
		{"echoes 2024-11-05", "2024-11-05", "2024-11-05"},
		{"echoes 2025-06-18", "2025-06-18", "2025-06-18"},
		{"unknown version falls back to latest", "2099-01-01", "2025-06-18"},
		{"empty version falls back to latest", "", "2025-06-18"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := fmt.Sprintf(`{"protocolVersion":"%s"}`, tt.clientVersion)
			if tt.clientVersion == "" {
				params = `{}`
			}
			resp := h.HandleRequest(mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params:  json.RawMessage(params),
			})
			if resp == nil || resp.Error != nil {
				t.Fatalf("initialize response = %+v, want success", resp)
			}
			data := mustDecodeJSON[mcp.MCPInitializeResult](t, resp.Result)
			if data.ProtocolVersion != tt.expectedVersion {
				t.Fatalf("ProtocolVersion = %q, want %q", data.ProtocolVersion, tt.expectedVersion)
			}
		})
	}
}

func TestHandleRequest_NotificationDetection(t *testing.T) {
	t.Parallel()
	h := NewMCPHandler(nil, "v-test")

	// G7: Per JSON-RPC 2.0, a Notification is a Request without an "id" member.
	// req.ID == nil is the sole notification check.

	// Case 1: nil ID, non-notification method -> notification (no response)
	if resp := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: nil, Method: "some/method"}); resp != nil {
		t.Fatalf("nil ID should be treated as notification, got response: %+v", resp)
	}

	// Case 2: nil ID, notifications/ prefix -> notification (no response)
	if resp := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: nil, Method: "notifications/initialized"}); resp != nil {
		t.Fatalf("nil ID with notifications/ prefix should be notification, got response: %+v", resp)
	}

	// Case 3: non-nil ID, notifications/ prefix -> NOT a notification (should get response)
	resp := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "notifications/initialized"})
	if resp == nil {
		t.Fatal("non-nil ID with notifications/ prefix should NOT be treated as notification — should get response")
	}
}

func TestMCPHandlerHandleHTTP_ReadErrorUsesNullID(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(nil, "v-http")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
	})
	transport := newMCPHTTPHandler(h)

	readErrReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBuffer(nil))
	readErrReq.Body = ioNopCloser{Reader: failReader{}}
	readErrRR := httptest.NewRecorder()
	transport.ServeHTTP(readErrRR, readErrReq)

	var readErrResp mcp.JSONRPCResponse
	if err := json.Unmarshal(readErrRR.Body.Bytes(), &readErrResp); err != nil {
		t.Fatalf("json.Unmarshal(read error response) error = %v", err)
	}
	if readErrResp.Error == nil || readErrResp.Error.Code != -32700 {
		t.Fatalf("read error response = %+v, want code -32700", readErrResp)
	}
	if readErrResp.ID != nil {
		t.Fatalf("read error response id = %v, want null", readErrResp.ID)
	}
}

func TestMCPHandlerHandleHTTP_IDNullIsInvalidRequest(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(nil, "v-http")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
	})
	transport := newMCPHTTPHandler(h)

	nullIDReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":null,"method":"ping","params":{}}`))
	nullIDRR := httptest.NewRecorder()
	transport.ServeHTTP(nullIDRR, nullIDReq)
	if nullIDRR.Code != http.StatusOK {
		t.Fatalf("id:null request status = %d, want %d", nullIDRR.Code, http.StatusOK)
	}

	var nullIDResp mcp.JSONRPCResponse
	if err := json.Unmarshal(nullIDRR.Body.Bytes(), &nullIDResp); err != nil {
		t.Fatalf("json.Unmarshal(id:null response) error = %v", err)
	}
	if nullIDResp.Error == nil || nullIDResp.Error.Code != -32600 {
		t.Fatalf("id:null response = %+v, want invalid request -32600", nullIDResp)
	}
	if nullIDResp.ID != nil {
		t.Fatalf("id:null response id = %v, want null", nullIDResp.ID)
	}
}

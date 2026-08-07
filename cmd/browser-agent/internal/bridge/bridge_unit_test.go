// Purpose: Unit tests for browser-agent bridge logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// bridge_unit_test.go — Unit tests for bridge helper functions.
package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// NOTE: Tests that redirect os.Stdout cannot use t.Parallel().

func TestSendBridgeError(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	testRunner.sendBridgeError(42, -32603, "test error message", bridge.StdioFramingLine)

	os.Stdout = origStdout
	_ = w.Close()

	output, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatalf("failed to read pipe: %v", readErr)
	}

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("sendBridgeError output not valid JSON: %v; got: %q", err, string(output))
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc=2.0, got %q", resp.JSONRPC)
	}
	if resp.Error == nil {
		t.Fatal("expected error field to be set")
	}
	if resp.Error.Code != -32603 {
		t.Fatalf("expected code=-32603, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "test error message" {
		t.Fatalf("expected message=%q, got %q", "test error message", resp.Error.Message)
	}
}

func TestSendStartupErrorWritesJSONRPCError(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = w
	SendStartupError("boom")
	os.Stdout = original
	_ = w.Close()
	output, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	var response mcp.JSONRPCResponse
	if err := json.Unmarshal(output, &response); err != nil {
		t.Fatalf("startup output is not JSON-RPC: %v; output=%q", err, output)
	}
	if response.ID != "startup" || response.Error == nil || response.Error.Code != -32603 || !strings.Contains(response.Error.Message, "boom") {
		t.Fatalf("startup response = %#v", response)
	}
}

func TestSendToolError(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	testRunner.sendToolErrorWithOptions("req-1", "Server is starting up. Please retry.", bridge.StdioFramingLine, bridgeToolErrorOptions{})

	os.Stdout = origStdout
	_ = w.Close()

	output, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatalf("failed to read pipe: %v", readErr)
	}

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("sendToolError output not valid JSON: %v; got: %q", err, string(output))
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc=2.0, got %q", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Fatal("sendToolError should not set JSON-RPC error field (it's a soft error)")
	}
	if resp.Result == nil {
		t.Fatal("expected result field to be set")
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result["isError"] != true {
		t.Fatalf("expected isError=true, got %v", result["isError"])
	}
	if result["status"] != "error" {
		t.Fatalf("expected status=error, got %v", result["status"])
	}
	if result["subsystem"] != "bridge" {
		t.Fatalf("expected subsystem=bridge, got %v", result["subsystem"])
	}
	if result["retryable"] != false {
		t.Fatalf("expected retryable=false default, got %v", result["retryable"])
	}
	if result["error_code"] != "bridge_tool_error" {
		t.Fatalf("expected error_code=bridge_tool_error default, got %v", result["error_code"])
	}
	if result["correlation_id"] != "req-1" {
		t.Fatalf("expected correlation_id=req-1, got %v", result["correlation_id"])
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected content array, got %v", result["content"])
	}
	firstItem := content[0].(map[string]any)
	if firstItem["text"] != "Server is starting up. Please retry." {
		t.Fatalf("unexpected text: %v", firstItem["text"])
	}
}

func TestHandleDaemonNotReady_StartingIncludesStructuredRetryEnvelope(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	output := captureBridgeIO(t, "", func() {
		req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: "req-1", Method: "tools/call"}
		testRunner.handleDaemonNotReady(req, "starting", func() {}, bridge.StdioFramingLine)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 1 {
		t.Fatalf("response count = %d, want 1", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("expected soft tool error, got protocol error: %+v", responses[0].Error)
	}

	var result map[string]any
	if err := json.Unmarshal(responses[0].Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result["error_code"] != "daemon_starting" {
		t.Fatalf("error_code = %v, want daemon_starting", result["error_code"])
	}
	if result["subsystem"] != "bridge_startup" {
		t.Fatalf("subsystem = %v, want bridge_startup", result["subsystem"])
	}
	if result["reason"] != "daemon_starting" {
		t.Fatalf("reason = %v, want daemon_starting", result["reason"])
	}
	retryable, ok := result["retryable"].(bool)
	if !ok || !retryable {
		t.Fatalf("retryable = %v (ok=%v), want true", result["retryable"], ok)
	}
	if ms, ok := result["retry_after_ms"].(float64); !ok || int(ms) != 2000 {
		t.Fatalf("retry_after_ms = %v, want 2000", result["retry_after_ms"])
	}
	if result["correlation_id"] != "req-1" {
		t.Fatalf("correlation_id = %v, want req-1", result["correlation_id"])
	}
}

func TestBridgeForwardRequest_ToolsCallConnectionErrorReturnsSoftErrorEnvelope(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	var wg sync.WaitGroup
	wg.Add(1)
	signal := func() { wg.Done() }

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	line := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"page"}}}`)

	go func() {
		testRunner.bridgeForwardRequest(&http.Client{}, "http://127.0.0.1:1/mcp", req, line, 300*time.Millisecond, nil, signal, bridge.StdioFramingLine)
	}()

	wg.Wait()
	os.Stdout = origStdout
	_ = w.Close()

	output, _ := io.ReadAll(r)
	_ = r.Close()

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("bridgeForwardRequest output not valid JSON: %v; got: %q", err, string(output))
	}
	if resp.Error != nil {
		t.Fatalf("expected soft tool error result, got protocol error: %+v", resp.Error)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result["isError"] != true {
		t.Fatalf("isError = %v, want true", result["isError"])
	}
	if result["error_code"] != "bridge_connection_error" {
		t.Fatalf("error_code = %v, want bridge_connection_error", result["error_code"])
	}
	if result["subsystem"] != "bridge_http_forwarder" {
		t.Fatalf("subsystem = %v, want bridge_http_forwarder", result["subsystem"])
	}
	retryable, ok := result["retryable"].(bool)
	if !ok || !retryable {
		t.Fatalf("retryable = %v (ok=%v), want true", result["retryable"], ok)
	}
	if result["correlation_id"] != "1" {
		t.Fatalf("correlation_id = %v, want 1", result["correlation_id"])
	}
}

func TestBridgeForwardRequest_ToolsCallNoContentReturnsSoftErrorEnvelope(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	output := captureBridgeIO(t, "", func() {
		req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
		line := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"page"}}}`)
		testRunner.bridgeForwardRequest(client, "http://unit.test/mcp", req, line, time.Second, nil, func() {}, bridge.StdioFramingLine)
	})

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v; output=%q", err, output)
	}
	if resp.Error != nil {
		t.Fatalf("expected soft error result, got protocol error: %+v", resp.Error)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result["error_code"] != "bridge_unexpected_no_content" {
		t.Fatalf("error_code = %v, want bridge_unexpected_no_content", result["error_code"])
	}
}

func TestBridgeForwardRequest_ToolsCallEmptyBodyReturnsSoftErrorEnvelope(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("   ")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	output := captureBridgeIO(t, "", func() {
		req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
		line := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"page"}}}`)
		testRunner.bridgeForwardRequest(client, "http://unit.test/mcp", req, line, time.Second, nil, func() {}, bridge.StdioFramingLine)
	})

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v; output=%q", err, output)
	}
	if resp.Error != nil {
		t.Fatalf("expected soft error result, got protocol error: %+v", resp.Error)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result["error_code"] != "bridge_empty_response" {
		t.Fatalf("error_code = %v, want bridge_empty_response", result["error_code"])
	}
}

func TestBridgeForwardRequest_ToolsCallInvalidJSONBodyReturnsSoftErrorEnvelope(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{not-valid-json`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	output := captureBridgeIO(t, "", func() {
		req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call"}
		line := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"page"}}}`)
		testRunner.bridgeForwardRequest(client, "http://unit.test/mcp", req, line, time.Second, nil, func() {}, bridge.StdioFramingLine)
	})

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v; output=%q", err, output)
	}
	if resp.Error != nil {
		t.Fatalf("expected soft error result, got protocol error: %+v", resp.Error)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result["error_code"] != "bridge_invalid_response" {
		t.Fatalf("error_code = %v, want bridge_invalid_response", result["error_code"])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBridgeRequestIDString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		id   any
		want string
	}{
		{name: "string", id: "req-1", want: "req-1"},
		{name: "raw-number", id: json.RawMessage(`123`), want: "123"},
		{name: "raw-string", id: json.RawMessage(`"abc"`), want: "abc"},
		{name: "float", id: 42.5, want: "42.5"},
		{name: "nil", id: nil, want: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := bridgeRequestIDString(tc.id); got != tc.want {
				t.Fatalf("bridgeRequestIDString(%v) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}

// TestBridgeForwardRequest_LargeBodyRead verifies that bridgeForwardRequest
// reads the full response body even when the server writes it with a delay.
// This is a regression test for a bug where bridgeDoHTTP created a context
// with defer cancel(), which canceled before the caller read resp.Body.
func TestBridgeForwardRequest_LargeBodyRead(t *testing.T) {
	// Build a large JSON-RPC response (~20KB) to exceed any internal buffering
	largePayload := strings.Repeat("key:value,", 5000)
	rpcEnvelope := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": largePayload + "done:true",
				},
			},
			"isError": false,
		},
	}
	rpcBytes, err := json.Marshal(rpcEnvelope)
	if err != nil {
		t.Fatalf("json.Marshal rpcEnvelope: %v", err)
	}
	rpcResponse := string(rpcBytes)
	headersFlushed := make(chan struct{})
	releaseBody := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hold the body behind an explicit barrier after flushing headers. This
		// deterministically reproduces a response whose body outlives the request
		// setup scope without guessing at scheduler timing.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(headersFlushed)
		<-releaseBody
		_, _ = w.Write([]byte(rpcResponse))
	}))
	defer srv.Close()

	// Capture stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	var wg sync.WaitGroup
	wg.Add(1)
	signalCalled := false
	signal := func() {
		signalCalled = true
		wg.Done()
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	line := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"configure","arguments":{"what":"health"}}}`)

	go func() {
		testRunner.bridgeForwardRequest(&http.Client{}, srv.URL, req, line, 5*time.Second, nil, signal, bridge.StdioFramingLine)
	}()

	<-headersFlushed
	close(releaseBody)
	wg.Wait()
	os.Stdout = origStdout
	_ = w.Close()

	output, _ := io.ReadAll(r)
	_ = r.Close()

	if !signalCalled {
		t.Fatal("signal was never called")
	}

	// Verify we got the full response, not a "Failed to read response: context canceled" error
	outputStr := string(output)
	if strings.Contains(outputStr, "context canceled") {
		t.Fatalf("got context canceled error — body read failed: %s", outputStr)
	}
	if strings.Contains(outputStr, "Failed to read response") {
		t.Fatalf("got body read error: %s", outputStr)
	}
	// "done":true is inside a JSON text field, so it appears escaped as \"done\":true
	if !strings.Contains(outputStr, `done`) {
		t.Fatalf("response body was truncated or missing: %s", outputStr[:min(len(outputStr), 200)])
	}
	if len(outputStr) < len(rpcResponse)/2 {
		t.Fatalf("response body appears truncated: got %d bytes, expected ~%d", len(outputStr), len(rpcResponse))
	}
}

func TestDaemonStateMarkFailedAndReadyAreIdempotent(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}

	state.markFailed("first")

	done := make(chan struct{})
	go func() {
		defer close(done)
		state.markFailed("second")
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second markFailed call blocked or panicked")
	}

	state.mu.Lock()
	if !state.failed {
		t.Fatal("expected failed=true after markFailed")
	}
	state.mu.Unlock()

	// markReady should also be safe when invoked repeatedly.
	state.mu.Lock()
	state.resetSignalsLocked()
	state.ready = false
	state.failed = false
	state.err = ""
	state.mu.Unlock()

	state.markReady()
	state.markReady()

	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.ready {
		t.Fatal("expected ready=true after markReady")
	}
	if state.failed {
		t.Fatal("expected failed=false after markReady")
	}
}

func TestCheckDaemonStatus_StartupGraceWaitsForReadySignal(t *testing.T) {
	oldGrace := daemonStartupGracePeriod
	daemonStartupGracePeriod = 120 * time.Millisecond
	defer func() { daemonStartupGracePeriod = oldGrace }()

	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}

	// Deliver the same readiness edge that a concurrent daemon startup emits.
	// The status check must consume the already-available signal without waiting
	// for the grace timer.
	close(state.readyCh)

	status := checkDaemonStatus(state, mcp.JSONRPCRequest{Method: "tools/call"}, 7890)
	if status != "" {
		t.Fatalf("checkDaemonStatus() = %q, want empty status after ready signal", status)
	}
}

func TestCheckDaemonStatus_StartupGraceTimeoutReturnsStarting(t *testing.T) {
	oldGrace := daemonStartupGracePeriod
	daemonStartupGracePeriod = 60 * time.Millisecond
	defer func() { daemonStartupGracePeriod = oldGrace }()

	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}

	start := time.Now()
	status := checkDaemonStatus(state, mcp.JSONRPCRequest{Method: "tools/call"}, 7890)
	elapsed := time.Since(start)

	if status != "starting" {
		t.Fatalf("checkDaemonStatus() = %q, want starting", status)
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("startup grace wait too short: %v, want >= 40ms", elapsed)
	}
}

func TestIsIgnorableStdoutSyncError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "einval", err: syscall.EINVAL, want: true},
		{name: "ebadf", err: syscall.EBADF, want: true},
		{name: "pathErrorEbadf", err: &os.PathError{Op: "sync", Path: "/dev/stdout", Err: syscall.EBADF}, want: true},
		{name: "other", err: fmt.Errorf("boom"), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsIgnorableStdoutSyncError(tc.err); got != tc.want {
				t.Fatalf("IsIgnorableStdoutSyncError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func captureMCPTransportOutput(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	previous := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = previous }()

	run()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return string(output)
}

func TestWriteMCPPayloadNormalizesLineFraming(t *testing.T) {
	output := captureMCPTransportOutput(t, func() {
		WriteMCPPayload([]byte(" \n\t"+`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`+"\n\n "), bridge.StdioFramingLine)
	})
	if !strings.HasSuffix(output, "\n") || strings.HasSuffix(output, "\n\n") {
		t.Fatalf("line framing must have exactly one trailing newline: %q", output)
	}
	trimmed := strings.TrimSuffix(output, "\n")
	if !json.Valid([]byte(trimmed)) || strings.TrimSpace(trimmed) != trimmed {
		t.Fatalf("line framing did not normalize JSON payload: %q", output)
	}
}

func TestWriteMCPPayloadUsesNormalizedContentLength(t *testing.T) {
	output := captureMCPTransportOutput(t, func() {
		WriteMCPPayload([]byte(" \n\t"+`{"jsonrpc":"2.0","id":9,"result":{"ok":true}}`+"\n\n "), bridge.StdioFramingContentLength)
	})
	parts := strings.SplitN(output, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("content-length output = %q", output)
	}
	lengthText := strings.TrimPrefix(strings.SplitN(parts[0], "\r\n", 2)[0], "Content-Length: ")
	reportedLength, err := strconv.Atoi(lengthText)
	if err != nil {
		t.Fatalf("content length %q: %v", lengthText, err)
	}
	if reportedLength != len(parts[1]) || !json.Valid([]byte(parts[1])) || strings.TrimSpace(parts[1]) != parts[1] {
		t.Fatalf("content-length framing mismatch: header=%d body=%q", reportedLength, parts[1])
	}
}

func TestWriteMCPPayloadReplacesInvalidJSON(t *testing.T) {
	output := captureMCPTransportOutput(t, func() {
		WriteMCPPayload([]byte("not-json"), bridge.StdioFramingLine)
	})
	var response mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(strings.TrimSuffix(output, "\n")), &response); err != nil {
		t.Fatalf("decode fallback response: %v; output=%q", err, output)
	}
	if response.JSONRPC != mcp.JSONRPCVersion || response.ID != nil || response.Error == nil || response.Error.Code != -32603 {
		t.Fatalf("fallback response = %#v", response)
	}
}

func TestExtractToolAction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		method     string
		params     string
		wantTool   string
		wantAction string
	}{
		{name: "configure restart", method: "tools/call", params: `{"name":"configure","arguments":{"what":"restart"}}`, wantTool: "configure", wantAction: "restart"},
		{name: "observe", method: "tools/call", params: `{"name":"observe","arguments":{"what":"errors"}}`, wantTool: "observe", wantAction: "errors"},
		{name: "missing action", method: "tools/call", params: `{"name":"configure","arguments":{"buffer":"all"}}`, wantTool: "configure"},
		{name: "non tool call", method: "initialize", params: `{}`},
		{name: "invalid params", method: "tools/call", params: `{not json}`},
		{name: "missing arguments", method: "tools/call", params: `{"name":"configure"}`, wantTool: "configure"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tool, action := ExtractToolAction(mcp.JSONRPCRequest{
				JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: test.method, Params: json.RawMessage(test.params),
			})
			if tool != test.wantTool || action != test.wantAction {
				t.Fatalf("ExtractToolAction() = (%q, %q), want (%q, %q)", tool, action, test.wantTool, test.wantAction)
			}
		})
	}
}

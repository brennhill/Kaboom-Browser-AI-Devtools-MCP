// Purpose: Unit tests for browser-agent bridge fastpath logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func contentLengthFrame(payload string) string {
	return fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(payload), payload)
}

func contentTypeFirstFrame(payload string) string {
	return fmt.Sprintf("Content-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(payload), payload)
}

func captureBridgeIO(t *testing.T, input string, fn func()) string {
	t.Helper()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdin) error = %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}

	oldIn := os.Stdin
	oldOut := os.Stdout
	os.Stdin = inR
	os.Stdout = outW

	_, _ = io.WriteString(inW, input)
	_ = inW.Close()

	fn()

	os.Stdin = oldIn
	os.Stdout = oldOut
	_ = inR.Close()
	_ = outW.Close()

	out, err := io.ReadAll(outR)
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	_ = outR.Close()
	return string(out)
}

func captureBridgeIOWithStderr(t *testing.T, input string, fn func()) (string, string) {
	t.Helper()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdin) error = %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stderr) error = %v", err)
	}

	oldIn := os.Stdin
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdin = inR
	os.Stdout = outW
	os.Stderr = errW

	_, _ = io.WriteString(inW, input)
	_ = inW.Close()

	fn()

	os.Stdin = oldIn
	os.Stdout = oldOut
	os.Stderr = oldErr
	_ = inR.Close()
	_ = outW.Close()
	_ = errW.Close()

	stdout, readErr := io.ReadAll(outR)
	if readErr != nil {
		t.Fatalf("ReadAll(stdout) error = %v", readErr)
	}
	_ = outR.Close()
	stderr, readErr := io.ReadAll(errR)
	if readErr != nil {
		t.Fatalf("ReadAll(stderr) error = %v", readErr)
	}
	_ = errR.Close()

	return string(stdout), string(stderr)
}

func parseJSONLines(t *testing.T, output string) []mcp.JSONRPCResponse {
	t.Helper()
	var responses []mcp.JSONRPCResponse
	sc := bufio.NewScanner(strings.NewReader(output))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
		responses = append(responses, resp)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	return responses
}

func parseFramedJSONResponses(t *testing.T, output string) []mcp.JSONRPCResponse {
	t.Helper()
	remaining := output
	responses := make([]mcp.JSONRPCResponse, 0, 4)

	for strings.TrimSpace(remaining) != "" {
		parts := strings.SplitN(remaining, "\r\n\r\n", 2)
		if len(parts) != 2 {
			t.Fatalf("output missing Content-Length framing boundary: %q", remaining)
		}
		headers := strings.Split(parts[0], "\r\n")
		contentLength := -1
		for _, header := range headers {
			h := strings.TrimSpace(header)
			if strings.HasPrefix(strings.ToLower(h), "content-length:") {
				p := strings.SplitN(h, ":", 2)
				if len(p) != 2 {
					t.Fatalf("invalid Content-Length header format: %q", h)
				}
				n, err := strconv.Atoi(strings.TrimSpace(p[1]))
				if err != nil || n < 0 {
					t.Fatalf("invalid Content-Length header %q: %v", h, err)
				}
				contentLength = n
				break
			}
		}
		if contentLength < 0 {
			t.Fatalf("framed output missing Content-Length header: %q", parts[0])
		}

		bodyAndRest := parts[1]
		if len(bodyAndRest) < contentLength {
			t.Fatalf("framed output body shorter than Content-Length (%d): %q", contentLength, bodyAndRest)
		}
		body := bodyAndRest[:contentLength]
		remaining = bodyAndRest[contentLength:]

		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &resp); err != nil {
			t.Fatalf("invalid framed JSON response body %q: %v", body, err)
		}
		responses = append(responses, resp)
	}

	return responses
}

func parseMCPResponses(t *testing.T, output string) []mcp.JSONRPCResponse {
	t.Helper()
	trimmed := strings.TrimSpace(output)
	if strings.HasPrefix(strings.ToLower(trimmed), "content-length:") {
		return parseFramedJSONResponses(t, output)
	}
	return parseJSONLines(t, output)
}

func parseFramedJSONResponse(t *testing.T, output string) mcp.JSONRPCResponse {
	t.Helper()
	parts := strings.SplitN(output, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("output missing Content-Length framing: %q", output)
	}
	if !strings.HasPrefix(strings.ToLower(parts[0]), "content-length:") {
		t.Fatalf("framed output missing Content-Length header: %q", parts[0])
	}
	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(parts[1])), &resp); err != nil {
		t.Fatalf("invalid framed JSON response: %v", err)
	}
	return resp
}

func TestBridgeFastPathCoreMethods(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	resetFastPathResourceReadCounters()
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"kaboom://capabilities"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"unknown/method","params":{}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`,
	}, "\n") + "\n"

	output := captureBridgeIO(t, input, func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 7 {
		t.Fatalf("response count = %d, want 7", len(responses))
	}

	// Parse error response.
	if responses[0].Error == nil || responses[0].Error.Code != -32700 {
		t.Fatalf("parse-error response = %+v, want JSON parse error", responses[0])
	}

	// initialize response should include protocolVersion.
	if responses[1].Error != nil {
		t.Fatalf("initialize response has error: %+v", responses[1].Error)
	}
	var initResult map[string]any
	if err := json.Unmarshal(responses[1].Result, &initResult); err != nil {
		t.Fatalf("initialize result unmarshal error = %v", err)
	}
	if initResult["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocolVersion = %v, want 2025-06-18", initResult["protocolVersion"])
	}

	// tools/list fast path.
	var toolsResult map[string]any
	if err := json.Unmarshal(responses[2].Result, &toolsResult); err != nil {
		t.Fatalf("tools/list result unmarshal error = %v", err)
	}
	if _, ok := toolsResult["tools"]; !ok {
		t.Fatalf("tools/list result missing tools: %v", toolsResult)
	}

	// resources/read should be served by fast path while daemon is still starting.
	var resourcesReadResult map[string]any
	if err := json.Unmarshal(responses[4].Result, &resourcesReadResult); err != nil {
		t.Fatalf("resources/read result unmarshal error = %v", err)
	}
	contents, _ := resourcesReadResult["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("resources/read contents = %v, want one content item", resourcesReadResult["contents"])
	}

	// unknown method should return method-not-found protocol error.
	if responses[5].Error == nil || responses[5].Error.Code != -32601 {
		t.Fatalf("unknown method response = %+v, want -32601", responses[5])
	}

	// tools/call while not ready should be a soft tool error.
	if responses[6].Error != nil {
		t.Fatalf("tools/call startup response should be soft error, got protocol error %+v", responses[6].Error)
	}
	var startupResult map[string]any
	if err := json.Unmarshal(responses[6].Result, &startupResult); err != nil {
		t.Fatalf("startup result unmarshal error = %v", err)
	}
	if startupResult["isError"] != true {
		t.Fatalf("startup result isError = %v, want true", startupResult["isError"])
	}

	success, failure := SnapshotFastPathResourceReadCounters()
	if success != 1 || failure != 0 {
		t.Fatalf("fast-path resources/read counters = (%d,%d), want (1,0)", success, failure)
	}
}

func TestBridgeFastPath_ContentLengthInputProducesContentLengthOutput(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := contentLengthFrame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)

	output := captureBridgeIO(t, input, func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	resp := parseFramedJSONResponse(t, output)
	if resp.Error != nil {
		t.Fatalf("initialize response has error: %+v", resp.Error)
	}
	var initResult map[string]any
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		t.Fatalf("initialize result unmarshal error = %v", err)
	}
	if initResult["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocolVersion = %v, want 2025-06-18", initResult["protocolVersion"])
	}
}

func TestBridgeFastPathResourcesReadCanonicalizesPlaybookAliases(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	resetFastPathResourceReadCounters()
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"kaboom://playbook/security"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"kaboom://playbook/security_audit/quick"}}`,
	}, "\n") + "\n"

	output := captureBridgeIO(t, input, func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	for i, resp := range responses {
		if resp.Error != nil {
			t.Fatalf("response[%d] error = %+v, want success", i, resp.Error)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("response[%d] unmarshal error = %v", i, err)
		}
		contents, _ := result["contents"].([]any)
		if len(contents) != 1 {
			t.Fatalf("response[%d] contents = %v, want one content", i, result["contents"])
		}
		content, _ := contents[0].(map[string]any)
		if content["uri"] != "kaboom://playbook/security/quick" {
			t.Fatalf("response[%d] content uri = %v, want kaboom://playbook/security/quick", i, content["uri"])
		}
	}

	success, failure := SnapshotFastPathResourceReadCounters()
	if success != 2 || failure != 0 {
		t.Fatalf("fast-path resources/read counters = (%d,%d), want (2,0)", success, failure)
	}
}

func TestBridgeFastPathResourcesReadFailureTelemetry(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	resetFastPathResourceReadCounters()
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"kaboom://playbook/nonexistent/quick"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":[]}`,
	}, "\n") + "\n"

	output := captureBridgeIO(t, input, func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})
	responses := parseJSONLines(t, output)
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	if responses[0].Error == nil || responses[0].Error.Code != -32002 {
		t.Fatalf("response[0] error = %+v, want -32002", responses[0].Error)
	}
	if responses[1].Error == nil || responses[1].Error.Code != -32602 {
		t.Fatalf("response[1] error = %+v, want -32602", responses[1].Error)
	}

	success, failure := SnapshotFastPathResourceReadCounters()
	if success != 0 || failure != 2 {
		t.Fatalf("fast-path resources/read counters = (%d,%d), want (0,2)", success, failure)
	}
}

func TestBridgeFastPathResourcesReadTelemetryPersistsToStateLogs(t *testing.T) {
	// Do not run in parallel; test redirects process stdio and env.
	resetFastPathResourceReadCounters()
	t.Setenv(statecfg.StateDirEnv, t.TempDir())

	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"kaboom://capabilities"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"kaboom://playbook/nonexistent/quick"}}`,
	}, "\n") + "\n"

	_ = captureBridgeIO(t, input, func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	path, err := FastPathResourceReadLogPath()
	if err != nil {
		t.Fatalf("FastPathResourceReadLogPath() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("telemetry log lines = %d, want 2; log=%q", len(lines), string(data))
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first telemetry line parse error = %v", err)
	}
	if first["event"] != "bridge_fastpath_resources_read" {
		t.Fatalf("first telemetry event = %v, want bridge_fastpath_resources_read", first["event"])
	}
	if first["success"] != true {
		t.Fatalf("first telemetry success = %v, want true", first["success"])
	}

	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("second telemetry line parse error = %v", err)
	}
	if second["success"] != false {
		t.Fatalf("second telemetry success = %v, want false", second["success"])
	}
	if code, ok := second["error_code"].(float64); !ok || int(code) != -32002 {
		t.Fatalf("second telemetry error_code = %v, want -32002", second["error_code"])
	}
}

func TestBridgeFastPathResourcesReadTelemetry(t *testing.T) {
	// Do not run in parallel; test redirects process stdio and uses Setenv.
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	resetFastPathCounters()

	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"kaboom://capabilities"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"kaboom://playbook/not-a-thing/quick"}}`,
	}, "\n") + "\n"

	output := captureBridgeIO(t, input, func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 3 {
		t.Fatalf("response count = %d, want 3", len(responses))
	}
	if responses[1].Error != nil {
		t.Fatalf("resources/read success response has protocol error: %+v", responses[1].Error)
	}
	if responses[2].Error == nil || responses[2].Error.Code != -32002 {
		t.Fatalf("resources/read miss response = %+v, want -32002", responses[2])
	}

	path, err := FastPathTelemetryLogPath()
	if err != nil {
		t.Fatalf("FastPathTelemetryLogPath error = %v", err)
	}
	summary := summarizeFastPathTelemetryLog(path, 100)
	if summary.total < 3 {
		t.Fatalf("telemetry total = %d, want >= 3", summary.total)
	}
	if summary.methods["resources/read"] < 2 {
		t.Fatalf("resources/read telemetry count = %d, want >= 2", summary.methods["resources/read"])
	}
	if summary.failure < 1 {
		t.Fatalf("failure count = %d, want >= 1", summary.failure)
	}
	if summary.errorCodes[-32002] < 1 {
		t.Fatalf("error code -32002 count = %d, want >= 1", summary.errorCodes[-32002])
	}
}

func TestBridgeFastPathFramedInitializeAndToolsList(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"codex","version":"1"}}}`
	toolsReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	input := contentLengthFrame(initReq) + contentLengthFrame(toolsReq)

	setStderrSink(io.Discard)
	stdout, stderr := captureBridgeIOWithStderr(t, input, func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})
	setStderrSink(os.Stderr)

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("framed startup emitted stderr output: %q", stderr)
	}
	if strings.Contains(stdout, `"code":-32700`) {
		t.Fatalf("framed startup emitted parse error: %q", stdout)
	}

	responses := parseMCPResponses(t, stdout)
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("initialize returned protocol error: %+v", responses[0].Error)
	}
	if responses[1].Error != nil {
		t.Fatalf("tools/list returned protocol error: %+v", responses[1].Error)
	}
}

func TestBridgeFastPathFramedInitializeAndToolsList_ContentTypeFirstHeaders(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"codex","version":"1"}}}`
	toolsReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	input := contentTypeFirstFrame(initReq) + contentTypeFirstFrame(toolsReq)

	setStderrSink(io.Discard)
	stdout, stderr := captureBridgeIOWithStderr(t, input, func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})
	setStderrSink(os.Stderr)

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("framed startup emitted stderr output: %q", stderr)
	}
	if strings.Contains(stdout, `"code":-32700`) {
		t.Fatalf("framed startup emitted parse error: %q", stdout)
	}

	responses := parseMCPResponses(t, stdout)
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("initialize returned protocol error: %+v", responses[0].Error)
	}
	if responses[1].Error != nil {
		t.Fatalf("tools/list returned protocol error: %+v", responses[1].Error)
	}
}

func TestBridgeFastPathNotificationWithoutIDHasNoResponse(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	output := captureBridgeIO(t, `{"jsonrpc":"2.0","method":"ping","params":{}}`+"\n", func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 0 {
		t.Fatalf("notification should not emit response, got %d responses: %q", len(responses), output)
	}
}

func TestBridgeFastPathNullIDReturnsInvalidRequest(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	output := captureBridgeIO(t, `{"jsonrpc":"2.0","id":null,"method":"ping","params":{}}`+"\n", func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})

	responses := parseJSONLines(t, output)
	if len(responses) != 1 {
		t.Fatalf("expected one response, got %d: %q", len(responses), output)
	}
	if responses[0].Error == nil || responses[0].Error.Code != -32600 {
		t.Fatalf("null-id response = %+v, want invalid request -32600", responses[0])
	}
	if responses[0].ID != nil {
		t.Fatalf("null-id response id = %v, want null", responses[0].ID)
	}
}

func TestBridgeFastPathFailedDaemonMessage(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &daemonState{runner: testRunner,
		failed:   true,
		err:      "bind: address already in use",
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}

	output := captureBridgeIO(t, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`+"\n", func() {
		testRunner.StdioToHTTPFast("http://127.0.0.1:1/mcp", state, 7890)
	})
	responses := parseJSONLines(t, output)
	if len(responses) != 1 {
		t.Fatalf("response count = %d, want 1", len(responses))
	}

	var result map[string]any
	if err := json.Unmarshal(responses[0].Result, &result); err != nil {
		t.Fatalf("unmarshal soft error result: %v", err)
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("content = %v, want non-empty", result["content"])
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	if !strings.Contains(text, "Port may be in use") {
		t.Fatalf("daemon failure message = %q, expected port suggestion", text)
	}
}

func TestBridgeServerHealthHelpers(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: mux}
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_ = srv.Serve(ln)
	}()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	if !testRunner.IsServerRunning(port) {
		t.Fatalf("testRunner.IsServerRunning(%d) = false, want true", port)
	}
	if !testRunner.WaitForServer(port, 750*time.Millisecond) {
		t.Fatalf("testRunner.WaitForServer(%d) = false, want true", port)
	}

	_ = srv.Close()
	<-serveDone
	if testRunner.IsServerRunning(port) {
		t.Fatalf("testRunner.IsServerRunning(%d) = true after shutdown, want false", port)
	}
}

func TestCheckDaemonStatus_HealsReadyFlagFromHealth(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok","name":"`+testRunner.identity.ServerName+`","version":"1.0.0"}`)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	state := &daemonState{runner: testRunner,
		port:     port,
		ready:    false,
		failed:   true,
		err:      "stale failure",
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	req := mcp.JSONRPCRequest{Method: "tools/call"}

	status := checkDaemonStatus(state, req, 7890)
	if status != "" {
		t.Fatalf("checkDaemonStatus() = %q, want empty status", status)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.ready {
		t.Fatal("expected state.ready to be healed to true")
	}
	if state.failed {
		t.Fatal("expected state.failed to be cleared")
	}
	if state.err != "" {
		t.Fatalf("expected state.err to be cleared, got %q", state.err)
	}
}

func TestRunningServerVersionCompatible(t *testing.T) {
	oldVersion := testRunner.identity.Version
	testRunner.identity.Version = "9.9.9"
	t.Cleanup(func() { testRunner.identity.Version = oldVersion })
	healthVersion := "9.9.9"

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok","name":"`+testRunner.identity.ServerName+`","version":"`+healthVersion+`"}`)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	compatible, gotVersion, gotService := testRunner.runningServerVersionCompatible(port)
	if !compatible || gotVersion != "9.9.9" || gotService != testRunner.identity.ServerName {
		t.Fatalf("testRunner.runningServerVersionCompatible() = (%v, %q, %q), want (true, %q, %q)", compatible, gotVersion, gotService, "9.9.9", testRunner.identity.ServerName)
	}

	healthVersion = "1.0.0"
	compatible, gotVersion, gotService = testRunner.runningServerVersionCompatible(port)
	if compatible || gotVersion != "1.0.0" || gotService != testRunner.identity.ServerName {
		t.Fatalf("testRunner.runningServerVersionCompatible() = (%v, %q, %q), want (false, %q, %q)", compatible, gotVersion, gotService, "1.0.0", testRunner.identity.ServerName)
	}
}

func TestFlushStdoutNoPanic(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error = %v", err)
	}
	oldOut := os.Stdout
	os.Stdout = outW
	defer func() {
		os.Stdout = oldOut
		_ = outW.Close()
		_ = outR.Close()
	}()

	FlushStdout()
}

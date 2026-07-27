// runner_test.go — Connect-mode lifecycle and MCP forwarding contracts.

package connectmode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type serverState struct {
	mu              sync.Mutex
	registerCalls   int
	unregisterCalls int
	mcpCalls        int
	lastClient      string
	registerBody    string
}

func testPort(t *testing.T, rawURL string) int {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("strconv.Atoi(port) error = %v", err)
	}
	return port
}

func TestRunnerHappyPath(t *testing.T) {
	state := &serverState{}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/clients", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		state.mu.Lock()
		state.registerCalls++
		state.lastClient = r.Header.Get("X-Kaboom-Client")
		state.registerBody = string(body)
		state.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var req mcp.JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		state.mu.Lock()
		state.mcpCalls++
		state.lastClient = r.Header.Get("X-Kaboom-Client")
		state.mu.Unlock()
		_ = json.NewEncoder(w).Encode(mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: json.RawMessage(`{"ok":true}`),
		})
	})
	mux.HandleFunc("/clients/client-1", func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		state.unregisterCalls++
		state.lastClient = r.Header.Get("X-Kaboom-Client")
		state.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var output bytes.Buffer
	var diagnostics bytes.Buffer
	runner := New(Deps{
		Input:      strings.NewReader(`{"jsonrpc":"2.0","id":99,"method":"ping","params":{}}` + "\n"),
		HTTPClient: server.Client(),
		Diagnosticf: func(format string, args ...any) {
			_, _ = fmt.Fprintf(&diagnostics, format, args...)
		},
		WriteMCP: func(payload []byte, framing bridge.StdioFraming) {
			if framing != bridge.StdioFramingLine {
				t.Fatalf("framing = %v, want line", framing)
			}
			output.Write(payload)
			output.WriteByte('\n')
		},
		Exit: func(code int) { t.Fatalf("unexpected exit %d", code) },
	})
	runner.Run(testPort(t, server.URL), "client-1", "/tmp/project")

	var response mcp.JSONRPCResponse
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &response); err != nil {
		t.Fatalf("output is not valid JSON-RPC: %v", err)
	}
	if response.Error != nil {
		t.Fatalf("unexpected protocol error: %+v", response.Error)
	}
	if got := diagnostics.String(); !strings.Contains(got, "Connected to") || !strings.Contains(got, "Disconnected from") {
		t.Fatalf("diagnostics missing lifecycle messages: %q", got)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.registerCalls != 1 || state.unregisterCalls != 1 || state.mcpCalls != 1 {
		t.Fatalf("calls register=%d unregister=%d mcp=%d", state.registerCalls, state.unregisterCalls, state.mcpCalls)
	}
	if state.lastClient != "client-1" || !strings.Contains(state.registerBody, `"cwd":"/tmp/project"`) {
		t.Fatalf("client=%q registration=%q", state.lastClient, state.registerBody)
	}
}

func TestRunnerHealthFailureExitsWithoutForwarding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	exitCode := 0
	writes := 0
	runner := New(Deps{
		Input:      strings.NewReader(`{"jsonrpc":"2.0","id":1}` + "\n"),
		HTTPClient: server.Client(),
		WriteMCP:   func([]byte, bridge.StdioFraming) { writes++ },
		Exit:       func(code int) { exitCode = code },
	})
	runner.Run(testPort(t, server.URL), "client", "/tmp")
	if exitCode != 1 || writes != 0 {
		t.Fatalf("exit=%d writes=%d, want exit=1 writes=0", exitCode, writes)
	}
}

func TestSendMCPError(t *testing.T) {
	var output []byte
	runner := New(Deps{WriteMCP: func(payload []byte, _ bridge.StdioFraming) {
		output = append([]byte(nil), payload...)
	}})
	runner.sendMCPError(1, -32600, "Invalid Request")

	var response mcp.JSONRPCResponse
	if err := json.Unmarshal(output, &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Error == nil || response.Error.Code != -32600 || response.Error.Message != "Invalid Request" {
		t.Fatalf("response error = %+v", response.Error)
	}
}

func TestExtractRequestID(t *testing.T) {
	tests := []struct {
		input string
		want  any
	}{
		{`{"jsonrpc":"2.0","id":42,"method":"test"}`, float64(42)},
		{`{"jsonrpc":"2.0","id":"req-1","method":"test"}`, "req-1"},
		{`{"jsonrpc":"2.0","id":null}`, nil},
		{`not json`, nil},
	}
	for _, test := range tests {
		if got := extractRequestID(test.input); got != test.want {
			t.Errorf("extractRequestID(%q) = %v, want %v", test.input, got, test.want)
		}
	}
}

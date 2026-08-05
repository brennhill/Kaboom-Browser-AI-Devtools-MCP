// cli_coverage_extra_test.go — Additional unit tests raising coverage of the CLI run flow,
// daemon bootstrap, flag-spec parsing, and transport encode/decode helpers.
// Deterministic: uses fake RuntimeConfig callbacks and local httptest servers; no real spawns.

package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/cli/parser"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestCLIPackageRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("cli package has %d files; want at most 10 change-coupled owners", files)
	}
}

func TestCliParseFlag_Missing(t *testing.T) {
	t.Parallel()

	val, remaining := CLIParseFlag([]string{"--other", "x"}, "--url")
	if val != "" {
		t.Fatalf("missing flag should return empty, got %q", val)
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining should be unchanged, len = %d", len(remaining))
	}
}

func TestParseConfigureFixtureRestoreTransactionID(t *testing.T) {
	t.Parallel()
	args, err := parser.ParseConfigureArgs("qa_fixture", []string{"--fixture-action", "restore", "--transaction-id", "transaction_1"})
	if err != nil {
		t.Fatal(err)
	}
	if args["what"] != "qa_fixture" || args["fixture_action"] != "restore" || args["transaction_id"] != "transaction_1" {
		t.Fatalf("ParseConfigureArgs() = %#v", args)
	}
}

// serverPort extracts the numeric port from an httptest server URL.
func serverPort(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", rawURL, err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("strconv.Atoi(%q) error = %v", u.Port(), err)
	}
	return p
}

// runRC builds a RuntimeConfig whose daemon is reported as already running so
// Run/EnsureDaemon never spawn a process.
func runRC(port int, output *bytes.Buffer) RuntimeConfig {
	return RuntimeConfig{
		DefaultPort:        port,
		MaxPostBodySize:    10 * 1024 * 1024,
		DiagnosticOutput:   output,
		IsServerRunning:    func(int) bool { return true },
		WaitForServer:      func(int, time.Duration) bool { return true },
		DaemonProcessArgv0: func(exePath string) string { return exePath },
	}
}

// mcpEchoServer returns an httptest server that replies with a successful tool result.
func mcpEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcp.JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: `{"total":0}`}}}
		resultJSON, _ := json.Marshal(result)
		resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// --- Run flow ---

func TestRun_ObserveSuccess(t *testing.T) {
	t.Setenv("KABOOM_PORT", "")
	t.Setenv("KABOOM_FORMAT", "")

	srv := mcpEchoServer(t)
	defer srv.Close()
	var output bytes.Buffer
	rc := runRC(serverPort(t, srv.URL), &output)
	code := Run([]string{"observe", "errors", "--limit", "10"}, rc)
	if code != 0 {
		t.Fatalf("Run() = %d, want 0", code)
	}
}

func TestRun_AnalyzeAccessibilityTimeoutBranch(t *testing.T) {
	t.Setenv("KABOOM_PORT", "")
	t.Setenv("KABOOM_FORMAT", "")

	srv := mcpEchoServer(t)
	defer srv.Close()
	var output bytes.Buffer
	rc := runRC(serverPort(t, srv.URL), &output)
	code := Run([]string{"analyze", "accessibility"}, rc)
	if code != 0 {
		t.Fatalf("Run() = %d, want 0", code)
	}
}

func TestRun_ObserveCommandResultTimeoutBranch(t *testing.T) {
	t.Setenv("KABOOM_PORT", "")
	t.Setenv("KABOOM_FORMAT", "")

	srv := mcpEchoServer(t)
	defer srv.Close()
	var output bytes.Buffer
	rc := runRC(serverPort(t, srv.URL), &output)
	code := Run([]string{"observe", "command-result", "--correlation-id", "abc"}, rc)
	if code != 0 {
		t.Fatalf("Run() = %d, want 0", code)
	}
}

func TestRun_UsageError(t *testing.T) {
	t.Setenv("KABOOM_PORT", "")
	t.Setenv("KABOOM_FORMAT", "")

	var output bytes.Buffer
	rc := runRC(testDefaultPort, &output)
	code := Run([]string{"observe"}, rc)
	if code != 2 {
		t.Fatalf("Run() = %d, want 2 for usage error", code)
	}
	if !strings.Contains(output.String(), "Usage: kaboom") {
		t.Fatalf("Run() diagnostic output = %q, want usage", output.String())
	}
}

func TestRun_ParseError(t *testing.T) {
	t.Setenv("KABOOM_PORT", "")
	t.Setenv("KABOOM_FORMAT", "")

	var output bytes.Buffer
	rc := runRC(testDefaultPort, &output)
	// interact click without a targeting param fails ParseCLIArgs.
	code := Run([]string{"interact", "click"}, rc)
	if code != 2 {
		t.Fatalf("Run() = %d, want 2 for parse error", code)
	}
}

func TestRun_CallToolError(t *testing.T) {
	t.Setenv("KABOOM_PORT", "")
	t.Setenv("KABOOM_FORMAT", "")

	// A port with nothing listening: IsServerRunning is faked true but CallTool fails.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	var output bytes.Buffer
	rc := runRC(port, &output)
	code := Run([]string{"observe", "errors"}, rc)
	if code != 1 {
		t.Fatalf("Run() = %d, want 1 for CallTool failure", code)
	}
}

// --- EnsureDaemon ---

func TestEnsureDaemon_AlreadyRunning(t *testing.T) {
	t.Parallel()

	rc := RuntimeConfig{IsServerRunning: func(int) bool { return true }}
	baseURL, err := EnsureDaemon(7890, rc)
	if err != nil {
		t.Fatalf("EnsureDaemon() error = %v", err)
	}
	if baseURL != "http://127.0.0.1:7890" {
		t.Fatalf("EnsureDaemon() = %q, want http://127.0.0.1:7890", baseURL)
	}
}

// --- FormatResult format routing ---

func TestFormatResult_JSONAndCSVFormats(t *testing.T) {
	result := &mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: `{"count":3}`}}}

	for _, format := range []string{"json", "csv"} {
		var output bytes.Buffer
		code := FormatResult(&output, format, "observe", "errors", result)
		if code != 0 {
			t.Fatalf("FormatResult(%q) = %d, want 0", format, code)
		}
	}
}

// --- Transport encode/decode ---

func TestBuildToolCallBody_MarshalError(t *testing.T) {
	t.Parallel()

	// A channel cannot be JSON-marshaled, forcing the params-marshal error path.
	if _, err := BuildToolCallBody("observe", map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("BuildToolCallBody() error = nil, want marshal error")
	}
}

func TestParseToolCallResponse_Errors(t *testing.T) {
	t.Parallel()

	if _, err := ParseToolCallResponse([]byte("{not json")); err == nil {
		t.Fatal("expected error for invalid top-level JSON")
	}

	// Valid JSON-RPC envelope but result is a number, not an MCPToolResult object.
	if _, err := ParseToolCallResponse([]byte(`{"jsonrpc":"2.0","result":123}`)); err == nil {
		t.Fatal("expected error for non-object result")
	}

	res, err := ParseToolCallResponse([]byte(`{"jsonrpc":"2.0","result":{"content":[{"type":"text","text":"ok"}]}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || len(res.Content) != 1 || res.Content[0].Text != "ok" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

// --- Tool parser error propagation ---

func TestToolParsers_UnknownFlagPropagatesError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fn   func() (map[string]any, error)
	}{
		{"observe", func() (map[string]any, error) { return parser.ParseObserveArgs("errors", []string{"--nope", "x"}) }},
		{"analyze", func() (map[string]any, error) { return parser.ParseAnalyzeArgs("dom", []string{"--nope", "x"}) }},
		{"generate", func() (map[string]any, error) { return parser.ParseGenerateArgs("har", []string{"--nope", "x"}) }},
		{"configure", func() (map[string]any, error) { return parser.ParseConfigureArgs("health", []string{"--nope", "x"}) }},
		{"interact", func() (map[string]any, error) { return parser.ParseInteractArgs("navigate", []string{"--nope", "x"}) }},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := tc.fn(); err == nil {
				t.Fatalf("%s parser error = nil, want error for unknown flag", tc.name)
			}
		})
	}
}

// --- ValidateInteractArgs upload branch ---

func TestValidateInteractArgs_UploadRequiresTarget(t *testing.T) {
	t.Parallel()

	if _, err := parser.ParseInteractArgs("upload", nil); err == nil {
		t.Fatal("expected error for upload without a target")
	}
	if _, err := parser.ParseInteractArgs("upload", []string{"--api-endpoint", "https://x.test/api"}); err != nil {
		t.Fatalf("unexpected error for upload with api-endpoint: %v", err)
	}
}

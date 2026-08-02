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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestParseConfigureFixtureRestoreTransactionID(t *testing.T) {
	t.Parallel()
	args, err := ParseConfigureArgs("qa_fixture", []string{"--fixture-action", "restore", "--transaction-id", "transaction_1"})
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

// --- ParseFlagsBySpec ---

func TestParseFlagsBySpec_KindsAndErrors(t *testing.T) {
	t.Parallel()

	specs := map[string]CLIFlagSpec{
		"--str":  {MCPKey: "str", Kind: FlagString},
		"--int":  {MCPKey: "int", Kind: FlagInt},
		"--json": {MCPKey: "json", Kind: FlagJSON},
		"--list": {MCPKey: "list", Kind: FlagStringList},
		"--jos":  {MCPKey: "jos", Kind: FlagJSONOrString},
		"--ios":  {MCPKey: "ios", Kind: FlagIntOrString},
		"--bool": {MCPKey: "bool", Kind: FlagBool},
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"unknown flag", []string{"--nope"}, true},
		{"missing string value", []string{"--str"}, true},
		{"string value is another flag", []string{"--str", "--bool"}, true},
		{"invalid int", []string{"--int", "abc"}, true},
		{"missing int value", []string{"--int"}, true},
		{"invalid json", []string{"--json", "{bad"}, true},
		{"missing json value", []string{"--json"}, true},
		{"missing list value", []string{"--list"}, true},
		{"missing jos value", []string{"--jos"}, true},
		{"missing ios value", []string{"--ios"}, true},
		{"valid string", []string{"--str", "hello"}, false},
		{"valid int", []string{"--int", "42"}, false},
		{"valid json object", []string{"--json", `{"a":1}`}, false},
		{"valid list", []string{"--list", "a,b,c"}, false},
		{"jos plain string", []string{"--jos", "plain"}, false},
		{"jos json array", []string{"--jos", `[1,2]`}, false},
		{"ios integer", []string{"--ios", "42"}, false},
		{"ios string", []string{"--ios", "top"}, false},
		{"bool flag", []string{"--bool"}, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseFlagsBySpec(tc.args, specs)
			if tc.wantErr && err == nil {
				t.Fatalf("ParseFlagsBySpec(%v) error = nil, want error", tc.args)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ParseFlagsBySpec(%v) error = %v, want nil", tc.args, err)
			}
		})
	}
}

func TestParseFlagsBySpec_UnsupportedKind(t *testing.T) {
	t.Parallel()

	specs := map[string]CLIFlagSpec{"--x": {MCPKey: "x", Kind: CLIFlagKind(99)}}
	if _, err := ParseFlagsBySpec([]string{"--x"}, specs); err == nil {
		t.Fatal("ParseFlagsBySpec() error = nil, want error for unsupported kind")
	}
}

func TestParseFlagsBySpec_ValuesMapCorrectly(t *testing.T) {
	t.Parallel()

	specs := map[string]CLIFlagSpec{
		"--ios": {MCPKey: "frame", Kind: FlagIntOrString},
	}
	out, err := ParseFlagsBySpec([]string{"--ios", "5"}, specs)
	if err != nil {
		t.Fatalf("ParseFlagsBySpec() error = %v", err)
	}
	if out["frame"] != 5 {
		t.Fatalf("frame = %v (%T), want int 5", out["frame"], out["frame"])
	}

	out, err = ParseFlagsBySpec([]string{"--ios", "main"}, specs)
	if err != nil {
		t.Fatalf("ParseFlagsBySpec() error = %v", err)
	}
	if out["frame"] != "main" {
		t.Fatalf("frame = %v, want string main", out["frame"])
	}
}

// --- ParseCSVList edge ---

func TestParseCSVList_AllEmptyYieldsEmptySlice(t *testing.T) {
	t.Parallel()

	got := ParseCSVList("  ,  , ")
	if len(got) != 0 {
		t.Fatalf("ParseCSVList() = %v, want empty slice", got)
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
		{"observe", func() (map[string]any, error) { return ParseObserveArgs("errors", []string{"--nope", "x"}) }},
		{"analyze", func() (map[string]any, error) { return ParseAnalyzeArgs("dom", []string{"--nope", "x"}) }},
		{"generate", func() (map[string]any, error) { return ParseGenerateArgs("har", []string{"--nope", "x"}) }},
		{"configure", func() (map[string]any, error) { return ParseConfigureArgs("health", []string{"--nope", "x"}) }},
		{"interact", func() (map[string]any, error) { return ParseInteractArgs("navigate", []string{"--nope", "x"}) }},
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

	if _, err := ParseInteractArgs("upload", nil); err == nil {
		t.Fatal("expected error for upload without a target")
	}
	if _, err := ParseInteractArgs("upload", []string{"--api-endpoint", "https://x.test/api"}); err != nil {
		t.Fatalf("unexpected error for upload with api-endpoint: %v", err)
	}
}

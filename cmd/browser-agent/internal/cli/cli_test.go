// cli_test.go — Tests for CLI mode: argument parsing, output formatting, and end-to-end flow.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

const testDefaultPort = 7890

// --- IsCLIMode tests ---

func TestIsCLIMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"observe tool", []string{"observe", "errors"}, true},
		{"analyze tool", []string{"analyze", "dom"}, true},
		{"generate tool", []string{"generate", "har"}, true},
		{"configure tool", []string{"configure", "health"}, true},
		{"interact tool", []string{"interact", "click", "--selector", "#btn"}, true},
		{"flag --version", []string{"--version"}, false},
		{"flag --help", []string{"--help"}, false},
		{"flag --port", []string{"--port", "8080"}, false},
		{"flag --daemon", []string{"--daemon"}, false},
		{"flag --bridge", []string{"--bridge"}, false},
		{"flag --stop", []string{"--stop"}, false},
		{"flag --force", []string{"--force"}, false},
		{"flag --doctor", []string{"--doctor"}, false},
		{"flag --fastpath-min-samples", []string{"--fastpath-min-samples", "20"}, false},
		{"flag --fastpath-max-failure-ratio", []string{"--fastpath-max-failure-ratio", "0.05"}, false},
		{"flag --connect", []string{"--connect"}, false},
		{"empty args", []string{}, false},
		{"unknown word", []string{"foobar"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsCLIMode(tt.args)
			if got != tt.want {
				t.Errorf("IsCLIMode(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// --- ResolveCLIConfig tests ---

func testRC() RuntimeConfig {
	return RuntimeConfig{DefaultPort: testDefaultPort}
}

func TestResolveCLIConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg, remaining := ResolveCLIConfig([]string{"observe", "errors"}, testRC())
	if cfg.Port != testDefaultPort {
		t.Errorf("expected port %d, got %d", testDefaultPort, cfg.Port)
	}
	if cfg.Format != "human" {
		t.Errorf("expected format 'human', got %q", cfg.Format)
	}
	if cfg.Timeout != 15000 {
		t.Errorf("expected timeout 15000, got %d", cfg.Timeout)
	}
	if len(remaining) != 2 || remaining[0] != "observe" || remaining[1] != "errors" {
		t.Errorf("expected remaining [observe errors], got %v", remaining)
	}
}

func TestResolveCLIConfigFlagOverrides(t *testing.T) {
	t.Parallel()

	cfg, remaining := ResolveCLIConfig([]string{"--port", "9999", "--format", "json", "--timeout", "10000", "observe", "errors"}, testRC())
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.Format != "json" {
		t.Errorf("expected format 'json', got %q", cfg.Format)
	}
	if cfg.Timeout != 10000 {
		t.Errorf("expected timeout 10000, got %d", cfg.Timeout)
	}
	if len(remaining) != 2 || remaining[0] != "observe" || remaining[1] != "errors" {
		t.Errorf("expected remaining [observe errors], got %v", remaining)
	}
}

func TestResolveCLIConfigEnvOverrides(t *testing.T) {
	t.Setenv("KABOOM_PORT", "8888")
	t.Setenv("KABOOM_FORMAT", "csv")

	cfg, _ := ResolveCLIConfig([]string{"observe", "errors"}, testRC())
	if cfg.Port != 8888 {
		t.Errorf("expected port 8888, got %d", cfg.Port)
	}
	if cfg.Format != "csv" {
		t.Errorf("expected format 'csv', got %q", cfg.Format)
	}
}

func TestResolveCLIConfigFlagBeatsEnv(t *testing.T) {
	t.Setenv("KABOOM_PORT", "8888")

	cfg, _ := ResolveCLIConfig([]string{"--port", "9999", "observe", "errors"}, testRC())
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999 (flag beats env), got %d", cfg.Port)
	}
}

// --- NormalizeAction tests ---

func TestNormalizeAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"click", "click"},
		{"get-text", "get_text"},
		{"get-value", "get_value"},
		{"network-waterfall", "network_waterfall"},
		{"network-bodies", "network_bodies"},
		{"noise-rule", "noise_rule"},
		{"execute-js", "execute_js"},
		{"new-tab", "new_tab"},
		{"key-press", "key_press"},
		{"scroll-to", "scroll_to"},
		{"wait-for", "wait_for"},
		{"set-attribute", "set_attribute"},
		{"get-attribute", "get_attribute"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := NormalizeAction(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeAction(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- ParseObserveArgs tests ---

func TestParseObserveArgsErrors(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseObserveArgs("errors", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["what"] != "errors" {
		t.Errorf("expected what 'errors', got %v", mcpArgs["what"])
	}
}

func TestParseObserveArgsWithLimit(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseObserveArgs("errors", []string{"--limit", "50"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["limit"] != 50 {
		t.Errorf("expected limit 50, got %v", mcpArgs["limit"])
	}
}

func TestParseObserveArgsLogs(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseObserveArgs("logs", []string{"--min-level", "warn", "--limit", "100"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["what"] != "logs" {
		t.Errorf("expected what 'logs', got %v", mcpArgs["what"])
	}
	if mcpArgs["min_level"] != "warn" {
		t.Errorf("expected min_level 'warn', got %v", mcpArgs["min_level"])
	}
	if mcpArgs["limit"] != 100 {
		t.Errorf("expected limit 100, got %v", mcpArgs["limit"])
	}
}

func TestParseObserveArgsNetworkWaterfall(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseObserveArgs("network_waterfall", []string{"--url", "api.example.com", "--status-min", "400"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["url"] != "api.example.com" {
		t.Errorf("expected url 'api.example.com', got %v", mcpArgs["url"])
	}
	if mcpArgs["status_min"] != 400 {
		t.Errorf("expected status_min 400, got %v", mcpArgs["status_min"])
	}
}

func TestParseObserveArgsAccessibility(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseObserveArgs("accessibility", []string{"--scope", "form"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["scope"] != "form" {
		t.Errorf("expected scope 'form', got %v", mcpArgs["scope"])
	}
}

func TestParseObserveArgsWebsocket(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseObserveArgs("websocket_events", []string{"--connection-id", "ws1", "--direction", "sent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["connection_id"] != "ws1" {
		t.Errorf("expected connection_id 'ws1', got %v", mcpArgs["connection_id"])
	}
	if mcpArgs["direction"] != "sent" {
		t.Errorf("expected direction 'sent', got %v", mcpArgs["direction"])
	}
}

// --- ParseGenerateArgs tests ---

func TestParseGenerateArgsHAR(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseGenerateArgs("har", []string{"--save-to", "out.har"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["what"] != "har" {
		t.Errorf("expected format 'har', got %v", mcpArgs["what"])
	}
	if mcpArgs["save_to"] != "out.har" {
		t.Errorf("expected save_to 'out.har', got %v", mcpArgs["save_to"])
	}
}

func TestParseGenerateArgsTest(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseGenerateArgs("test", []string{"--test-name", "my_test", "--assert-network", "--assert-no-errors"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["test_name"] != "my_test" {
		t.Errorf("expected test_name 'my_test', got %v", mcpArgs["test_name"])
	}
	if mcpArgs["assert_network"] != true {
		t.Errorf("expected assert_network true, got %v", mcpArgs["assert_network"])
	}
	if mcpArgs["assert_no_errors"] != true {
		t.Errorf("expected assert_no_errors true, got %v", mcpArgs["assert_no_errors"])
	}
}

func TestParseGenerateArgsCSP(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseGenerateArgs("csp", []string{"--mode", "strict"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["mode"] != "strict" {
		t.Errorf("expected mode 'strict', got %v", mcpArgs["mode"])
	}
}

func TestParseGenerateArgsReproduction(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseGenerateArgs("reproduction", []string{"--error-message", "timeout", "--last-n", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["error_message"] != "timeout" {
		t.Errorf("expected error_message 'timeout', got %v", mcpArgs["error_message"])
	}
	if mcpArgs["last_n"] != 5 {
		t.Errorf("expected last_n 5, got %v", mcpArgs["last_n"])
	}
}

// --- ParseConfigureArgs tests ---

func TestParseConfigureArgsHealth(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseConfigureArgs("health", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["what"] != "health" {
		t.Errorf("expected action 'health', got %v", mcpArgs["what"])
	}
}

func TestParseConfigureArgsClear(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseConfigureArgs("clear", []string{"--buffer", "network"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["buffer"] != "network" {
		t.Errorf("expected buffer 'network', got %v", mcpArgs["buffer"])
	}
}

func TestParseConfigureArgsNoiseRule(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseConfigureArgs("noise_rule", []string{"--noise-action", "add", "--pattern", "*.png", "--reason", "images"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["noise_action"] != "add" {
		t.Errorf("expected noise_action 'add', got %v", mcpArgs["noise_action"])
	}
	if mcpArgs["pattern"] != "*.png" {
		t.Errorf("expected pattern '*.png', got %v", mcpArgs["pattern"])
	}
}

func TestParseConfigureArgsStore(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseConfigureArgs("store", []string{"--key", "session", "--data", `{"id":"123"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["key"] != "session" {
		t.Errorf("expected key 'session', got %v", mcpArgs["key"])
	}
	data, ok := mcpArgs["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T: %v", mcpArgs["data"], mcpArgs["data"])
	}
	if data["id"] != "123" {
		t.Errorf("expected data.id '123', got %v", data["id"])
	}
}

// --- ParseInteractArgs tests ---

func TestParseInteractArgsClick(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseInteractArgs("click", []string{"--selector", "#btn"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["what"] != "click" {
		t.Errorf("expected action 'click', got %v", mcpArgs["what"])
	}
	if mcpArgs["selector"] != "#btn" {
		t.Errorf("expected selector '#btn', got %v", mcpArgs["selector"])
	}
}

func TestParseInteractArgsClickMissingSelector(t *testing.T) {
	t.Parallel()

	_, err := ParseInteractArgs("click", nil)
	if err == nil {
		t.Fatal("expected error for missing selector")
	}
	if !strings.Contains(err.Error(), "selector") {
		t.Errorf("expected error about missing selector, got: %v", err)
	}
}

func TestParseInteractArgsNavigate(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseInteractArgs("navigate", []string{"--url", "https://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["url"] != "https://example.com" {
		t.Errorf("expected url 'https://example.com', got %v", mcpArgs["url"])
	}
	if mcpArgs["what"] != "navigate" {
		t.Errorf("expected what 'navigate', got %v", mcpArgs["what"])
	}
}

func TestParseInteractArgsNavigateMissingURL(t *testing.T) {
	t.Parallel()

	_, err := ParseInteractArgs("navigate", nil)
	if err == nil {
		t.Fatal("expected error for missing url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("expected error about missing url, got: %v", err)
	}
}

func TestParseInteractArgsType(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseInteractArgs("type", []string{"--selector", "#input", "--text", "hello", "--clear"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["selector"] != "#input" {
		t.Errorf("expected selector '#input', got %v", mcpArgs["selector"])
	}
	if mcpArgs["text"] != "hello" {
		t.Errorf("expected text 'hello', got %v", mcpArgs["text"])
	}
	if mcpArgs["clear"] != true {
		t.Errorf("expected clear true, got %v", mcpArgs["clear"])
	}
}

func TestParseInteractArgsScrollToDirection(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseInteractArgs("scroll_to", []string{"--selector", "#modal-body", "--direction", "bottom"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["selector"] != "#modal-body" {
		t.Errorf("expected selector '#modal-body', got %v", mcpArgs["selector"])
	}
	if mcpArgs["direction"] != "bottom" {
		t.Errorf("expected direction 'bottom', got %v", mcpArgs["direction"])
	}
}

func TestParseInteractArgsGetTextStructured(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseInteractArgs("get_text", []string{"--selector", ".accordion", "--structured"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["selector"] != ".accordion" {
		t.Errorf("expected selector '.accordion', got %v", mcpArgs["selector"])
	}
	if mcpArgs["structured"] != true {
		t.Errorf("expected structured true, got %v", mcpArgs["structured"])
	}
}

func TestParseInteractArgsExecuteJS(t *testing.T) {
	t.Parallel()

	mcpArgs, err := ParseInteractArgs("execute_js", []string{"--script", "document.title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["script"] != "document.title" {
		t.Errorf("expected script 'document.title', got %v", mcpArgs["script"])
	}
}

func TestParseInteractArgsExecuteJSMissingScript(t *testing.T) {
	t.Parallel()

	_, err := ParseInteractArgs("execute_js", nil)
	if err == nil {
		t.Fatal("expected error for missing script")
	}
	if !strings.Contains(err.Error(), "script") {
		t.Errorf("expected error about missing script, got: %v", err)
	}
}

func TestParseInteractArgsKebabCase(t *testing.T) {
	t.Parallel()

	action := NormalizeAction("get-text")
	mcpArgs, err := ParseInteractArgs(action, []string{"--selector", ".content"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["what"] != "get_text" {
		t.Errorf("expected action 'get_text', got %v", mcpArgs["what"])
	}
}

// --- ParseCLIArgs dispatch tests ---

func TestParseCLIArgsDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tool    string
		action  string
		args    []string
		wantKey string
		wantVal any
		wantErr bool
	}{
		{"observe errors", "observe", "errors", nil, "what", "errors", false},
		{"analyze dom", "analyze", "dom", nil, "what", "dom", false},
		{"generate har", "generate", "har", nil, "what", "har", false},
		{"configure health", "configure", "health", nil, "what", "health", false},
		{"interact click", "interact", "click", []string{"--selector", "#btn"}, "what", "click", false},
		{"interact new-tab kebab", "interact", "new-tab", []string{"--url", "https://example.com"}, "what", "new_tab", false},
		{"unknown tool", "foobar", "x", nil, "", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcpArgs, err := ParseCLIArgs(tt.tool, tt.action, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mcpArgs[tt.wantKey] != tt.wantVal {
				t.Errorf("expected %s=%v, got %v", tt.wantKey, tt.wantVal, mcpArgs[tt.wantKey])
			}
		})
	}
}

// --- Format tests ---

func TestFormatHuman(t *testing.T) {
	t.Parallel()

	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: `{"entries":[],"total":0}`}},
	}
	cliRes := BuildCLIResult("observe", "errors", result)

	var buf bytes.Buffer
	err := FormatHuman(&buf, cliRes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[OK]") {
		t.Errorf("expected [OK] in output, got: %s", out)
	}
	if !strings.Contains(out, "observe") {
		t.Errorf("expected 'observe' in output, got: %s", out)
	}
}

func TestFormatHumanError(t *testing.T) {
	t.Parallel()

	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "Connection refused"}},
		IsError: true,
	}
	cliRes := BuildCLIResult("observe", "errors", result)

	var buf bytes.Buffer
	err := FormatHuman(&buf, cliRes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[Error]") {
		t.Errorf("expected [Error] in output, got: %s", out)
	}
}

func TestFormatJSON(t *testing.T) {
	t.Parallel()

	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: `{"status":"ok"}`}},
	}
	cliRes := BuildCLIResult("configure", "health", result)

	var buf bytes.Buffer
	err := FormatJSON(&buf, cliRes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["success"] != true {
		t.Errorf("expected success=true, got %v", parsed["success"])
	}
	if parsed["tool"] != "configure" {
		t.Errorf("expected tool=configure, got %v", parsed["tool"])
	}
}

func TestFormatCSV(t *testing.T) {
	t.Parallel()

	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: `{"count":5}`}},
	}
	cliRes := BuildCLIResult("observe", "errors", result)

	var buf bytes.Buffer
	err := FormatCSV(&buf, cliRes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header + data row, got %d lines", len(lines))
	}
	// Header should contain success, tool, action
	if !strings.Contains(lines[0], "success") {
		t.Errorf("expected 'success' in CSV header, got: %s", lines[0])
	}
}

// --- End-to-end tests with mock HTTP server ---

func TestEndToEndWithMockServer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcp.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", 400)
			return
		}

		result := mcp.MCPToolResult{
			Content: []mcp.MCPContentBlock{{Type: "text", Text: `{"entries":[],"total":0}`}},
		}
		resultJSON, _ := json.Marshal(result)
		resp := mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mcpArgs := map[string]any{"what": "errors"}
	result, err := CallTool(server.URL, "observe", mcpArgs, 5000, 10*1024*1024)
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}
	if result.IsError {
		t.Error("expected success, got isError")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content blocks")
	}
	if result.Content[0].Text != `{"entries":[],"total":0}` {
		t.Errorf("unexpected text: %s", result.Content[0].Text)
	}
}

func TestEndToEndToolError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcp.JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		result := mcp.MCPToolResult{
			Content: []mcp.MCPContentBlock{{Type: "text", Text: "No data available"}},
			IsError: true,
		}
		resultJSON, _ := json.Marshal(result)
		resp := mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mcpArgs := map[string]any{"what": "errors"}
	result, err := CallTool(server.URL, "observe", mcpArgs, 5000, 10*1024*1024)
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true")
	}
}

func TestEndToEndJSONRPCError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcp.JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.JSONRPCError{Code: -32601, Message: "Method not found"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mcpArgs := map[string]any{"what": "errors"}
	_, err := CallTool(server.URL, "observe", mcpArgs, 5000, 10*1024*1024)
	if err == nil {
		t.Fatal("expected error for JSON-RPC error response")
	}
	if !strings.Contains(err.Error(), "Method not found") {
		t.Errorf("expected 'Method not found' in error, got: %v", err)
	}
}

func TestEndToEndServerNotRunning(t *testing.T) {
	t.Parallel()

	_, err := CallTool("http://127.0.0.1:0", "observe", map[string]any{"what": "errors"}, 1000, 10*1024*1024)
	if err == nil {
		t.Fatal("expected error when server is not running")
	}
}

func TestEndToEndHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", 500)
	}))
	defer server.Close()

	_, err := CallTool(server.URL, "observe", map[string]any{"what": "errors"}, 5000, 10*1024*1024)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

// --- FormatResult exit code tests ---

func TestFormatResultExitCodeSuccess(t *testing.T) {
	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "ok"}},
	}
	var output bytes.Buffer
	code := FormatResult(&output, "human", "observe", "errors", result)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output.String(), "ok") {
		t.Fatalf("FormatResult output = %q, want result text", output.String())
	}
}

func TestFormatResultExitCodeError(t *testing.T) {
	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "failed"}},
		IsError: true,
	}
	var output bytes.Buffer
	code := FormatResult(&output, "human", "observe", "errors", result)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output.String(), "failed") {
		t.Fatalf("FormatResult output = %q, want error text", output.String())
	}
}

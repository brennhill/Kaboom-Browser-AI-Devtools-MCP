// handlers_coverage_test.go — Unit tests for the generate-local MCP artifact handlers.
// Exercises param parsing, no-data vs with-data branches, validation and error paths
// for each generate artifact handler using a fake Deps backed by real capture stores.

package toolgenerate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/export/har"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ---------------------------------------------------------------------------
// Test fakes
// ---------------------------------------------------------------------------

type fakeGenerateDeps struct {
	cap          *capture.Capture
	annStore     *annotation.Store
	version      string
	extConnected bool
	a11yResult   json.RawMessage
	a11yErr      error
}

type fakeTestGenerator struct{}

func (fakeTestGenerator) HandleGenerateTestFromContext(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	return mcp.Succeed(req, "context test", map[string]any{"status": "ok"})
}

func (fakeTestGenerator) HandleGenerateTestHeal(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	return mcp.Succeed(req, "healed test", map[string]any{"status": "ok"})
}

func (fakeTestGenerator) HandleGenerateTestClassify(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	return mcp.Succeed(req, "classified test", map[string]any{"status": "ok"})
}

func (f *fakeGenerateDeps) deps() Deps {
	return Deps{
		Capture:         f.cap,
		AnnotationStore: f.annStore,
		Version:         f.version,
		IsExtensionConnected: func() bool {
			return f.extConnected
		},
		ExecuteA11yQuery: func(_ string, _ []string, _ any, _ bool) (json.RawMessage, error) {
			return f.a11yResult, f.a11yErr
		},
	}
}

func newGenDeps() *fakeGenerateDeps {
	return &fakeGenerateDeps{
		cap:      capture.NewCapture(),
		annStore: annotation.NewStore(1 * time.Hour),
		version:  "9.9.9",
	}
}

func genReq() mcp.JSONRPCRequest { return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1} }

// parseResult decodes an MCP tool result into (isError, text).
func parseResult(t *testing.T, resp mcp.JSONRPCResponse) (bool, string) {
	t.Helper()
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("failed to unmarshal result: %v (raw=%s)", err, string(resp.Result))
	}
	text := ""
	if len(r.Content) > 0 {
		text = r.Content[0].Text
	}
	return r.IsError, text
}

func parseResultJSON[T any](t *testing.T, response mcp.JSONRPCResponse) T {
	t.Helper()
	isError, text := parseResult(t, response)
	if isError {
		t.Fatalf("unexpected tool error: %s", text)
	}
	if _, payload, found := strings.Cut(text, "\n"); found {
		text = payload
	}
	var result T
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("invalid result JSON: %v (text=%s)", err, text)
	}
	return result
}

func TestDispatcherRoutesEveryGenerateModeAndRejectsInvalidRequests(t *testing.T) {
	d := newGenDeps()
	dispatcher := NewDispatcher(d.deps(), fakeTestGenerator{})
	for _, args := range []json.RawMessage{nil, json.RawMessage(`{bad`), json.RawMessage(`{"what":"not_a_mode"}`)} {
		if isError, _ := parseResult(t, dispatcher.Handle(genReq(), args)); !isError {
			t.Errorf("invalid generate request %q succeeded", args)
		}
	}
	for _, mode := range []string{
		"reproduction", "test", "pr_summary", "sarif", "har", "csp", "sri",
		"visual_test", "annotation_report", "annotation_issues",
		"test_from_context", "test_heal", "test_classify",
	} {
		response := dispatcher.Handle(genReq(), json.RawMessage(`{"what":"`+mode+`"}`))
		if len(response.Result) == 0 {
			t.Errorf("generate mode %q returned no result", mode)
		}
	}
}

// ---------------------------------------------------------------------------
// HandleExportHAR
// ---------------------------------------------------------------------------

func TestHandleExportHAR(t *testing.T) {
	t.Run("no bodies yields empty export", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleExportHAR(d.deps(), genReq(), json.RawMessage(`{}`))
		result := parseResultJSON[har.HARLog](t, resp)
		if len(result.Log.Entries) != 0 {
			t.Fatalf("empty HAR entries = %#v", result.Log.Entries)
		}
	})

	t.Run("with bodies", func(t *testing.T) {
		d := newGenDeps()
		d.cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
			{URL: "https://example.com/api", Method: "GET", Status: 200},
		})
		resp := HandleExportHAR(d.deps(), genReq(), json.RawMessage(`{"method":"GET"}`))
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("HAR export with bodies should succeed: %s", text)
		}
	})

	t.Run("unsafe save_to path errors", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleExportHAR(d.deps(), genReq(), json.RawMessage(`{"save_to":"../escape.har"}`))
		if isErr, _ := parseResult(t, resp); !isErr {
			t.Fatal("unsafe save_to path should error")
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleExportHAR(d.deps(), genReq(), json.RawMessage(`{bad`))
		if isErr, _ := parseResult(t, resp); !isErr {
			t.Fatal("invalid JSON should error")
		}
	})
}

func TestHandleExportHARPreservesOutputFiltersAndFileContract(t *testing.T) {
	d := newGenDeps()
	d.cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{Timestamp: "2026-01-23T10:30:00.000Z", Method: "GET", URL: "https://example.com/api", Status: 200},
		{Timestamp: "2026-01-23T10:30:01.000Z", Method: "POST", URL: "https://example.com/api", Status: 500},
	})

	filtered := parseResultJSON[har.HARLog](t, HandleExportHAR(d.deps(), genReq(), json.RawMessage(`{"method":"POST","status_min":400}`)))
	if len(filtered.Log.Entries) != 1 || filtered.Log.Entries[0].Request.Method != "POST" {
		t.Fatalf("filtered entries = %#v", filtered.Log.Entries)
	}

	directory := filepath.Join(".tmp-har-export", strings.ReplaceAll(t.Name(), "/", "_"))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Errorf("remove HAR test directory: %v", err)
		}
	})
	path := filepath.Join(directory, "owner.har")
	summary := parseResultJSON[har.HARExportResult](t, HandleExportHAR(d.deps(), genReq(), json.RawMessage(`{"save_to":"`+path+`"}`)))
	if summary.SavedTo != path || summary.EntriesCount != 2 {
		t.Fatalf("save summary = %#v", summary)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved HAR unavailable: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HandlePRSummary
// ---------------------------------------------------------------------------

func TestHandlePRSummary(t *testing.T) {
	t.Run("no activity", func(t *testing.T) {
		d := newGenDeps()
		result := parseResultJSON[map[string]any](t, HandlePRSummary(d.deps(), genReq(), nil))
		if result["reason"] != "no_activity_captured" {
			t.Fatalf("empty summary = %#v", result)
		}
	})

	t.Run("with activity", func(t *testing.T) {
		d := newGenDeps()
		capturefixture.Track(d.cap, 1, "https://example.com/dash")
		d.cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{
			{Type: "click", Timestamp: time.Now().UnixMilli()},
			{Type: "click", Timestamp: time.Now().UnixMilli()},
			{Type: "navigate", Timestamp: time.Now().UnixMilli()},
		})
		d.cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
			{URL: "https://example.com/ok", Method: "GET", Status: 200},
			{URL: "https://example.com/bad", Method: "GET", Status: 500},
		})
		d.cap.ExtensionLogs().Add([]types.ExtensionLog{
			{Level: "error", Message: "boom"},
			{Level: "info", Message: "fine"},
		})
		result := parseResultJSON[map[string]any](t, HandlePRSummary(d.deps(), genReq(), nil))
		stats, _ := result["stats"].(map[string]any)
		summary, _ := result["summary"].(string)
		if stats["actions"] != float64(3) || !strings.Contains(summary, "## Session Summary") {
			t.Fatalf("activity summary = %#v", result)
		}
		for _, field := range []string{"commands_completed", "commands_failed", "console_errors", "network_errors", "network_captured"} {
			if _, exists := stats[field]; !exists {
				t.Errorf("summary stats missing %q: %#v", field, stats)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// HandleExportSARIF
// ---------------------------------------------------------------------------

func TestHandleExportSARIF(t *testing.T) {
	t.Run("precomputed a11y result", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleExportSARIF(d.deps(), genReq(), json.RawMessage(`{"a11y_result":{"violations":[]}}`))
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("SARIF export should succeed: %s", text)
		}
	})

	t.Run("not connected uses empty a11y", func(t *testing.T) {
		d := newGenDeps()
		d.extConnected = false
		resp := HandleExportSARIF(d.deps(), genReq(), json.RawMessage(`{"scope":"page"}`))
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("SARIF export (empty) should succeed: %s", text)
		}
	})

	t.Run("connected runs a11y query", func(t *testing.T) {
		d := newGenDeps()
		d.extConnected = true
		d.a11yResult = json.RawMessage(`{"violations":[]}`)
		resp := HandleExportSARIF(d.deps(), genReq(), json.RawMessage(`{"include_passes":true}`))
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("SARIF export via query should succeed: %s", text)
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleExportSARIF(d.deps(), genReq(), json.RawMessage(`{bad`))
		if isErr, _ := parseResult(t, resp); !isErr {
			t.Fatal("invalid JSON should error")
		}
	})
}

// ---------------------------------------------------------------------------
// HandleGenerateCSP
// ---------------------------------------------------------------------------

func TestHandleGenerateCSP(t *testing.T) {
	t.Run("no bodies unavailable", func(t *testing.T) {
		d := newGenDeps()
		result := parseResultJSON[map[string]any](t, HandleGenerateCSP(d.deps(), genReq(), json.RawMessage(`{}`)))
		if result["status"] != "unavailable" || result["mode"] != "moderate" {
			t.Fatalf("empty CSP result = %#v", result)
		}
	})

	t.Run("invalid mode errors", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleGenerateCSP(d.deps(), genReq(), json.RawMessage(`{"mode":"nuclear"}`))
		if isErr, _ := parseResult(t, resp); !isErr {
			t.Fatal("invalid mode should error")
		}
	})

	t.Run("valid modes with bodies", func(t *testing.T) {
		for _, mode := range []string{"strict", "moderate", "report_only", ""} {
			d := newGenDeps()
			d.cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
				{URL: "https://cdn.example.com/app.js", Method: "GET", Status: 200},
			})
			args := `{"mode":"` + mode + `"}`
			if mode == "" {
				args = `{}`
			}
			resp := HandleGenerateCSP(d.deps(), genReq(), json.RawMessage(args))
			if isErr, text := parseResult(t, resp); isErr {
				t.Fatalf("CSP mode %q should succeed: %s", mode, text)
			}
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleGenerateCSP(d.deps(), genReq(), json.RawMessage(`{bad`))
		if isErr, _ := parseResult(t, resp); !isErr {
			t.Fatal("invalid JSON should error")
		}
	})
}

func TestHandleGenerateTestExplainsEmptyCapture(t *testing.T) {
	d := newGenDeps()
	result := parseResultJSON[map[string]any](t, HandleGenerateTest(d.deps(), genReq(), json.RawMessage(`{}`)))
	if result["reason"] != "no_actions_captured" {
		t.Fatalf("empty test result = %#v", result)
	}
}

func TestHandleGenerateCSPPreservesPolicyContract(t *testing.T) {
	d := newGenDeps()
	d.cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", Method: "GET", Status: 200},
		{URL: "https://cdn.example.com/style.css", ContentType: "text/css", Method: "GET", Status: 200},
		{URL: "https://api.example.com/data", ContentType: "application/json", Method: "GET", Status: 200},
		{URL: "https://cdn.example.com/logo.png", ContentType: "image/png", Method: "GET", Status: 200},
	})
	result := parseResultJSON[map[string]any](t, HandleGenerateCSP(d.deps(), genReq(), json.RawMessage(`{"mode":"strict"}`)))
	policy, _ := result["policy"].(string)
	if result["status"] != "ok" || !strings.Contains(policy, "default-src") || !strings.Contains(policy, "'self'") {
		t.Fatalf("CSP result = %#v", result)
	}
	if result["origins_observed"] != float64(4) || result["directives"] == nil {
		t.Fatalf("CSP attribution = %#v", result)
	}
}

// ---------------------------------------------------------------------------
// HandleGenerateSRI
// ---------------------------------------------------------------------------

func TestHandleGenerateSRI(t *testing.T) {
	t.Run("no bodies unavailable", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleGenerateSRI(d.deps(), genReq(), json.RawMessage(`{}`))
		if isErr, _ := parseResult(t, resp); isErr {
			t.Fatal("unavailable should be success response")
		}
	})

	t.Run("with bodies", func(t *testing.T) {
		d := newGenDeps()
		capturefixture.Track(d.cap, 1, "https://example.com")
		d.cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
			{URL: "https://cdn.example.com/lib.js", Method: "GET", Status: 200, ContentType: "application/javascript"},
		})
		resp := HandleGenerateSRI(d.deps(), genReq(), json.RawMessage(`{}`))
		// Result may be success or a structured error depending on hash availability;
		// either way the response must be well-formed.
		parseResult(t, resp)
	})
}

// ---------------------------------------------------------------------------
// HandleGenerateTest
// ---------------------------------------------------------------------------

func TestHandleGenerateTest(t *testing.T) {
	t.Run("no actions captured", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleGenerateTest(d.deps(), genReq(), json.RawMessage(`{}`))
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("generate test should succeed with hint: %s", text)
		}
	})

	t.Run("with actions", func(t *testing.T) {
		d := newGenDeps()
		d.cap.Telemetry().AddEnhancedActions([]types.EnhancedAction{
			{Type: "navigate", ToURL: "https://example.com", Timestamp: time.Now().UnixMilli()},
			{Type: "click", Selectors: map[string]any{"css": "#btn"}, Timestamp: time.Now().UnixMilli()},
		})
		result := parseResultJSON[map[string]any](t, HandleGenerateTest(d.deps(), genReq(), json.RawMessage(`{"test_name":"login","last_n":10}`)))
		metadata, _ := result["metadata"].(map[string]any)
		script, _ := result["script"].(string)
		if result["test_name"] != "login" || result["action_count"] != float64(2) || metadata["actions_available"] != float64(2) {
			t.Fatalf("generated test metadata = %#v", result)
		}
		if !strings.Contains(script, "import { test, expect }") || !strings.Contains(script, "test.describe('login'") {
			t.Fatalf("generated Playwright script = %q", script)
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleGenerateTest(d.deps(), genReq(), json.RawMessage(`{bad`))
		if isErr, _ := parseResult(t, resp); !isErr {
			t.Fatal("invalid JSON should error")
		}
	})
}

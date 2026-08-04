// handlers_coverage_test.go — Unit tests for the generate-local MCP artifact handlers.
// Exercises param parsing, no-data vs with-data branches, validation and error paths
// for each generate artifact handler using a fake Deps backed by real capture stores.

package toolgenerate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
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

// ---------------------------------------------------------------------------
// HandleExportHAR
// ---------------------------------------------------------------------------

func TestHandleExportHAR(t *testing.T) {
	t.Run("no bodies yields empty export", func(t *testing.T) {
		d := newGenDeps()
		resp := HandleExportHAR(d.deps(), genReq(), json.RawMessage(`{}`))
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("empty HAR export should succeed: %s", text)
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

// ---------------------------------------------------------------------------
// HandlePRSummary
// ---------------------------------------------------------------------------

func TestHandlePRSummary(t *testing.T) {
	t.Run("no activity", func(t *testing.T) {
		d := newGenDeps()
		resp := HandlePRSummary(d.deps(), genReq(), nil)
		isErr, text := parseResult(t, resp)
		if isErr {
			t.Fatalf("pr summary should succeed: %s", text)
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
		resp := HandlePRSummary(d.deps(), genReq(), nil)
		isErr, text := parseResult(t, resp)
		if isErr {
			t.Fatalf("pr summary should succeed: %s", text)
		}
		if len(text) == 0 {
			t.Error("expected summary text")
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
		resp := HandleGenerateCSP(d.deps(), genReq(), json.RawMessage(`{}`))
		if isErr, _ := parseResult(t, resp); isErr {
			t.Fatal("unavailable should be a success response, not error")
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
		resp := HandleGenerateTest(d.deps(), genReq(), json.RawMessage(`{"test_name":"login","last_n":10}`))
		if isErr, text := parseResult(t, resp); isErr {
			t.Fatalf("generate test with actions should succeed: %s", text)
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

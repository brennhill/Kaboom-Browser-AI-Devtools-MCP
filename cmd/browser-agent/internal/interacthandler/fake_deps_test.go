// fake_deps_test.go — Shared fully-populated fake Deps harness for interacthandler tests.
// Purpose: Provide a deterministic, dependency-injected Deps so handlers can be exercised
// end-to-end without real chrome/network/IO. Reuses the real capture.Store (no external I/O).
package interacthandler

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// fakeState captures interactions with the injected Deps and exposes behavior toggles.
type fakeState struct {
	mu sync.Mutex

	cap *capture.Store

	// Guard toggles — when true, the corresponding guard blocks the command.
	blockPilot   bool
	blockExt     bool
	blockTab     bool
	blockCSP     bool
	blockSession bool
	blockEnqueue bool

	// Captured interactions.
	enqueued        []queries.PendingQuery
	recordedActions []string
	drawStarted     int

	// Pluggable overrides (nil => default behavior).
	waitFn     func(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse
	interactFn func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
	screenshot func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
	pageInfo   func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
	analyzeFn  func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
	sarifFn    func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
	enrichFn   func(resp JSONRPCResponse, req JSONRPCRequest, tabID int) JSONRPCResponse
	redaction  RedactionEngine
	listenPort int
	evidenceFn func(clientID string) EvidenceShot
}

// newFakeState builds a fakeState with pilot enabled, a tracked tab, and a connected extension.
func newFakeState() *fakeState {
	c := capture.NewCapture()
	c.SetPilotEnabled(true)
	c.SetTrackingStatusForTest(1, "https://example.com/page")
	c.SimulateExtensionConnectForTest()
	return &fakeState{cap: c, listenPort: 7890}
}

func (fs *fakeState) record(action string) {
	fs.mu.Lock()
	fs.recordedActions = append(fs.recordedActions, action)
	fs.mu.Unlock()
}

func (fs *fakeState) enqueue(q queries.PendingQuery) {
	fs.mu.Lock()
	fs.enqueued = append(fs.enqueued, q)
	fs.mu.Unlock()
}

func (fs *fakeState) enqueuedCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return len(fs.enqueued)
}

func (fs *fakeState) recordedCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return len(fs.recordedActions)
}

func guardFn(block *bool, code, msg string) GuardCheck {
	return func(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool) {
		if *block {
			return fail(req, code, msg, "adjust and retry", opts...), true
		}
		return JSONRPCResponse{}, false
	}
}

// deps builds a fully-populated *Deps wired to this fakeState.
func (fs *fakeState) deps() *Deps {
	return &Deps{
		RequirePilot:       guardFn(&fs.blockPilot, ErrCodePilotDisabled, "Pilot mode disabled"),
		RequireExtension:   guardFn(&fs.blockExt, ErrNotInitialized, "Extension not connected"),
		RequireTabTracking: guardFn(&fs.blockTab, ErrNotInitialized, "Tab tracking not active"),
		RequireCSPClear: func(req JSONRPCRequest, world string) (JSONRPCResponse, bool) {
			if fs.blockCSP {
				return fail(req, ErrInvalidParam, "CSP restricted for world "+world, "use isolated world"), true
			}
			return JSONRPCResponse{}, false
		},

		EnqueuePendingQuery: func(req JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (JSONRPCResponse, bool) {
			fs.enqueue(query)
			if fs.blockEnqueue {
				return fail(req, ErrQueueFull, "Queue full", "retry later"), true
			}
			return JSONRPCResponse{}, false
		},

		MaybeWaitForCommand: func(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse {
			if fs.waitFn != nil {
				return fs.waitFn(req, correlationID, args, queuedSummary)
			}
			return succeed(req, queuedSummary, map[string]any{
				"status":         "complete",
				"correlation_id": correlationID,
			})
		},

		Capture: func() *capture.Store { return fs.cap },

		RecordAIAction:           func(action, url string, extra map[string]any) { fs.record(action) },
		RecordAIEnhancedAction:   func(action capture.EnhancedAction) { fs.record("enhanced") },
		RecordDOMPrimitiveAction: func(action, selector, text, value string) { fs.record("dom:" + action) },

		ToolInteract: func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			if fs.interactFn != nil {
				return fs.interactFn(req, args)
			}
			return succeed(req, "nested interact", map[string]any{"status": "complete"})
		},
		ToolAnalyze: func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			if fs.analyzeFn != nil {
				return fs.analyzeFn(req, args)
			}
			return succeed(req, "analyze", map[string]any{"issues": []any{}})
		},
		ToolExportSARIF: func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			if fs.sarifFn != nil {
				return fs.sarifFn(req, args)
			}
			return succeed(req, "sarif", map[string]any{"status": "exported"})
		},

		EnrichNavigateResponse: func(resp JSONRPCResponse, req JSONRPCRequest, tabID int) JSONRPCResponse {
			if fs.enrichFn != nil {
				return fs.enrichFn(resp, req, tabID)
			}
			return resp
		},
		InjectCSPBlockedActions: func(resp JSONRPCResponse) JSONRPCResponse { return resp },

		GetScreenshot: func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			if fs.screenshot != nil {
				return fs.screenshot(req, args)
			}
			result := MCPToolResult{Content: []MCPContentBlock{
				{Type: "text", Text: "screenshot"},
				{Type: "image", Data: "QUJD", MimeType: "image/png"},
			}}
			return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: safeMarshal(result, "{}")}
		},
		GetPageInfo: func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			if fs.pageInfo != nil {
				return fs.pageInfo(req, args)
			}
			return succeed(req, "page", map[string]any{
				"url": "https://example.com/page", "title": "Example", "tab_id": 1,
			})
		},

		MarkDrawStarted: func() { fs.mu.Lock(); fs.drawStarted++; fs.mu.Unlock() },
		GetListenPort:   func() int { return fs.listenPort },

		DefaultEvidenceCapture: func(clientID string) EvidenceShot {
			if fs.evidenceFn != nil {
				return fs.evidenceFn(clientID)
			}
			return EvidenceShot{Path: "/tmp/evidence-" + clientID + ".png", Filename: "evidence.png"}
		},

		RequireSessionStore: func(req JSONRPCRequest) (JSONRPCResponse, bool) {
			if fs.blockSession {
				return fail(req, ErrNotInitialized, "Session store unavailable", "enable persistence"), true
			}
			return JSONRPCResponse{}, false
		},
		DiagnosticHint:     func() func(*StructuredError) { return withHint("diagnostic hint") },
		GetRedactionEngine: func() RedactionEngine { return fs.redaction },
		GetCommandResult: func(correlationID string) (*queries.CommandResult, bool) {
			return fs.cap.GetCommandResult(correlationID)
		},

		ReplayMu: &sync.Mutex{},
	}
}

// newFakeHandler returns an InteractActionHandler wired to a fresh fakeState.
func newFakeHandler(t *testing.T) (*InteractActionHandler, *fakeState) {
	t.Helper()
	fs := newFakeState()
	return NewInteractActionHandler(fs.deps()), fs
}

// testReq returns a standard JSON-RPC request for interact handler tests.
func testReq() JSONRPCRequest {
	return JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: float64(1), ClientID: "client-test"}
}

// assertErr fails the test unless resp is an MCP error whose text contains codeSubstr.
func assertErr(t *testing.T, resp JSONRPCResponse, codeSubstr string) {
	t.Helper()
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatalf("expected error response, got success: %s", firstText(result))
	}
	if codeSubstr != "" && !contains(firstText(result), codeSubstr) {
		t.Fatalf("expected error text to contain %q, got: %s", codeSubstr, firstText(result))
	}
}

// assertOK fails the test unless resp is a non-error MCP result.
func assertOK(t *testing.T, resp JSONRPCResponse) MCPToolResult {
	t.Helper()
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success response, got error: %s", firstText(result))
	}
	return result
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

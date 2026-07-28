// fake_deps_test.go — Shared fully-populated fake Deps harness for toolinteract tests.
// Purpose: Provide a deterministic, dependency-injected Deps so handlers can be exercised
// end-to-end without real chrome/network/IO. Reuses the real capture.Capture (no external I/O).
package toolinteract

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestDepsDoesNotReexportMCPProtocolSurface(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "deps.go"))
	if err != nil {
		t.Fatalf("read deps.go: %v", err)
	}
	for _, forbidden := range []string{
		"type JSONRPCRequest =",
		"type JSONRPCResponse =",
		"type MCPToolResult =",
		"type MCPContentBlock =",
		"type StructuredError =",
		"const JSONRPCVersion =",
		"ErrInvalidJSON =",
		"func withParam(",
		"func withHint(",
		"func withAction(",
		"func withSelector(",
		"func withRetryable(",
		"func withRetryAfterMs(",
		"func withFinal(",
		"func withRecoveryToolCall(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("deps.go retains MCP compatibility facade %q", forbidden)
		}
	}
}

// fakeState captures interactions with the injected Deps and exposes behavior toggles.
type fakeState struct {
	mu sync.Mutex

	cap *capture.Capture

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
	waitFn     func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse
	interactFn func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse
	screenshot func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse
	pageInfo   func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse
	analyzeFn  func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse
	sarifFn    func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse
	enrichFn   func(resp mcp.JSONRPCResponse, req mcp.JSONRPCRequest, tabID int) mcp.JSONRPCResponse
	redaction  RedactionEngine
	listenPort int
	evidenceFn func(clientID string) EvidenceShot
}

// newFakeState builds a fakeState with pilot enabled, a tracked tab, and a connected extension.
func newFakeState() *fakeState {
	c := capture.NewCapture()
	c.Extension().SetPilotEnabled(true)
	c.Extension().SetTrackingStatusForTest(1, "https://example.com/page")
	c.Extension().SimulateExtensionConnectForTest()
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

func guardFn(block *bool, code, msg string) toolguard.Check {
	return func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
		if *block {
			return mcp.Fail(req, code, msg, "adjust and retry", opts...), true
		}
		return mcp.JSONRPCResponse{}, false
	}
}

// deps builds a fully-populated *Deps wired to this fakeState.
func (fs *fakeState) deps() *Deps {
	return &Deps{
		RequirePilot:       guardFn(&fs.blockPilot, mcp.ErrCodePilotDisabled, "Pilot mode disabled"),
		RequireExtension:   guardFn(&fs.blockExt, mcp.ErrNotInitialized, "Extension not connected"),
		RequireTabTracking: guardFn(&fs.blockTab, mcp.ErrNotInitialized, "Tab tracking not active"),
		RequireCSPClear: func(req mcp.JSONRPCRequest, world string) (mcp.JSONRPCResponse, bool) {
			if fs.blockCSP {
				return mcp.Fail(req, mcp.ErrInvalidParam, "CSP restricted for world "+world, "use isolated world"), true
			}
			return mcp.JSONRPCResponse{}, false
		},

		EnqueuePendingQuery: func(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
			fs.enqueue(query)
			if fs.blockEnqueue {
				return mcp.Fail(req, mcp.ErrQueueFull, "Queue full", "retry later"), true
			}
			return mcp.JSONRPCResponse{}, false
		},

		MaybeWaitForCommand: func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
			if fs.waitFn != nil {
				return fs.waitFn(req, correlationID, args, queuedSummary)
			}
			return mcp.Succeed(req, queuedSummary, map[string]any{
				"status":         "complete",
				"correlation_id": correlationID,
			})
		},

		Capture: func() *capture.Capture { return fs.cap },

		RecordAIAction:           func(action, url string, extra map[string]any) { fs.record(action) },
		RecordAIEnhancedAction:   func(action types.EnhancedAction) { fs.record("enhanced") },
		RecordDOMPrimitiveAction: func(action, selector, text, value string) { fs.record("dom:" + action) },

		ToolInteract: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			if fs.interactFn != nil {
				return fs.interactFn(req, args)
			}
			return mcp.Succeed(req, "nested interact", map[string]any{"status": "complete"})
		},
		ToolAnalyze: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			if fs.analyzeFn != nil {
				return fs.analyzeFn(req, args)
			}
			return mcp.Succeed(req, "analyze", map[string]any{"issues": []any{}})
		},
		ToolExportSARIF: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			if fs.sarifFn != nil {
				return fs.sarifFn(req, args)
			}
			return mcp.Succeed(req, "sarif", map[string]any{"status": "exported"})
		},

		EnrichNavigateResponse: func(resp mcp.JSONRPCResponse, req mcp.JSONRPCRequest, tabID int) mcp.JSONRPCResponse {
			if fs.enrichFn != nil {
				return fs.enrichFn(resp, req, tabID)
			}
			return resp
		},
		InjectCSPBlockedActions: func(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse { return resp },

		GetScreenshot: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			if fs.screenshot != nil {
				return fs.screenshot(req, args)
			}
			result := mcp.MCPToolResult{Content: []mcp.MCPContentBlock{
				{Type: "text", Text: "screenshot"},
				{Type: "image", Data: "QUJD", MimeType: "image/png"},
			}}
			return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: mcp.SafeMarshal(result, "{}")}
		},
		GetPageInfo: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			if fs.pageInfo != nil {
				return fs.pageInfo(req, args)
			}
			return mcp.Succeed(req, "page", map[string]any{
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

		RequireSessionStore: func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
			if fs.blockSession {
				return mcp.Fail(req, mcp.ErrNotInitialized, "Session store unavailable", "enable persistence"), true
			}
			return mcp.JSONRPCResponse{}, false
		},
		DiagnosticHint:     func() func(*mcp.StructuredError) { return mcp.WithHint("diagnostic hint") },
		GetRedactionEngine: func() RedactionEngine { return fs.redaction },
		GetCommandResult: func(correlationID string) (*queries.CommandResult, bool) {
			return fs.cap.Queries().GetCommandResult(correlationID)
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
func testReq() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: float64(1), ClientID: "client-test"}
}

// assertErr fails the test unless resp is an MCP error whose text contains codeSubstr.
func assertErr(t *testing.T, resp mcp.JSONRPCResponse, codeSubstr string) {
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
func assertOK(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
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

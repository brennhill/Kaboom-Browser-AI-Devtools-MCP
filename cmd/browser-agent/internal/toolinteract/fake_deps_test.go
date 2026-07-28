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

func TestActionOwnersDoNotReexportMCPProtocolSurface(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "action_owners.go"))
	if err != nil {
		t.Fatalf("read action_owners.go: %v", err)
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
		"type RedactionEngine interface",
		"GetRedactionEngine func()",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("action_owners.go retains MCP compatibility facade %q", forbidden)
		}
	}
}

func TestActionFamiliesHaveNoBroadHandlerOrDependencyBag(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(testFile), "*.go"))
	if err != nil {
		t.Fatalf("list toolinteract sources: %v", err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, forbidden := range []string{
			"type Deps struct",
			"type InteractActionHandler struct",
			"NewInteractActionHandler",
			"NewUploadInteractHandler",
			"NewStateInteractHandler",
		} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s retains broad interact surface %q", filepath.Base(path), forbidden)
			}
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
	listenPort int
	evidenceFn func(clientID string) EvidenceShot
}

type fakeCapabilities struct {
	RequirePilot, RequireExtension, RequireTabTracking toolguard.Check
	RequireCSPClear                                    func(mcp.JSONRPCRequest, string) (mcp.JSONRPCResponse, bool)
	EnqueuePendingQuery                                func(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool)
	MaybeWaitForCommand                                func(mcp.JSONRPCRequest, string, json.RawMessage, string) mcp.JSONRPCResponse
	Capture                                            func() *capture.Capture
	RecordAIAction                                     func(string, string, map[string]any)
	RecordAIEnhancedAction                             func(types.EnhancedAction)
	RecordDOMPrimitiveAction                           func(string, string, string, string)
	ToolInteract, ToolAnalyze, ToolExportSARIF         func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	InjectCSPBlockedActions                            func(mcp.JSONRPCResponse) mcp.JSONRPCResponse
	GetScreenshot, GetPageInfo                         func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	MarkDrawStarted                                    func()
	GetListenPort                                      func() int
	DefaultEvidenceCapture                             func(string) EvidenceShot
	RequireSessionStore                                func(mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool)
	DiagnosticHint                                     func() func(*mcp.StructuredError)
	Redact                                             func(map[string]any) map[string]any
	GetCommandResult                                   func(string) (*queries.CommandResult, bool)
	ReplayMu                                           *sync.Mutex
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

func (fs *fakeState) enqueuedSnapshot() []queries.PendingQuery {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]queries.PendingQuery(nil), fs.enqueued...)
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
func (fs *fakeState) deps() *fakeCapabilities {
	return &fakeCapabilities{
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
			if query.Type == "page_summary" {
				_, _ = fs.cap.Queries().CreatePendingQueryWithTimeout(query, timeout, req.ClientID)
				fs.cap.Queries().ApplyCommandResult(
					query.CorrelationID,
					"complete",
					json.RawMessage(`{"main_content_preview":"Example content"}`),
					"",
				)
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
		DiagnosticHint: func() func(*mcp.StructuredError) { return mcp.WithHint("diagnostic hint") },
		Redact:         func(data map[string]any) map[string]any { return data },
		GetCommandResult: func(correlationID string) (*queries.CommandResult, bool) {
			return fs.cap.Queries().GetCommandResult(correlationID)
		},

		ReplayMu: &sync.Mutex{},
	}
}

type fakeActionOwners struct {
	runtime  *ActionRuntime
	dom      *DOMActions
	browser  *BrowserActions
	page     *PageActions
	workflow *WorkflowActions
	storage  *StorageActions
	batch    *BatchActions
}

func newFakeActionOwners(t *testing.T) (*fakeActionOwners, *fakeState) {
	t.Helper()
	fs := newFakeState()
	deps := fs.deps()
	runtime := NewActionRuntime(RuntimeDeps{
		RequireCSPClear: deps.RequireCSPClear, EnqueuePendingQuery: deps.EnqueuePendingQuery,
		MaybeWaitForCommand: deps.MaybeWaitForCommand, RecordAIAction: deps.RecordAIAction,
		DefaultEvidenceCapture: deps.DefaultEvidenceCapture,
	})
	dom := NewDOMActions(runtime, DOMDeps{deps.RequirePilot, deps.RequireExtension, deps.RequireTabTracking, deps.RecordDOMPrimitiveAction})
	storage := NewStorageActions(runtime, StorageDeps{deps.RequirePilot, deps.RequireExtension, deps.RequireTabTracking})
	page := NewPageActions(runtime, dom, storage, PageDeps{
		deps.RequirePilot, deps.RequireExtension, deps.RequireTabTracking, deps.Capture,
		deps.EnqueuePendingQuery, deps.RecordAIAction, deps.MarkDrawStarted, deps.GetScreenshot, deps.GetPageInfo,
	})
	browser := NewBrowserActions(runtime, page, BrowserDeps{
		deps.RequirePilot, deps.RequireExtension, deps.RequireTabTracking, deps.Capture,
		deps.InjectCSPBlockedActions, deps.GetListenPort,
	})
	workflow := NewWorkflowActions(runtime, dom, browser, page, WorkflowDeps{deps.Capture, deps.ToolAnalyze, deps.ToolExportSARIF})
	batch := NewBatchActions(runtime, BatchDeps{
		deps.RequirePilot, deps.RequireExtension, deps.Capture, deps.RecordAIAction, deps.ToolInteract, deps.ReplayMu,
	})
	return &fakeActionOwners{runtime, dom, browser, page, workflow, storage, batch}, fs
}

func newFakeDOMActions(t *testing.T) (*DOMActions, *fakeState) {
	o, s := newFakeActionOwners(t)
	return o.dom, s
}
func newFakeBrowserActions(t *testing.T) (*BrowserActions, *fakeState) {
	o, s := newFakeActionOwners(t)
	return o.browser, s
}
func newFakePageActions(t *testing.T) (*PageActions, *fakeState) {
	o, s := newFakeActionOwners(t)
	return o.page, s
}
func newFakeWorkflowActions(t *testing.T) (*WorkflowActions, *fakeState) {
	o, s := newFakeActionOwners(t)
	return o.workflow, s
}
func newFakeStorageActions(t *testing.T) (*StorageActions, *fakeState) {
	o, s := newFakeActionOwners(t)
	return o.storage, s
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

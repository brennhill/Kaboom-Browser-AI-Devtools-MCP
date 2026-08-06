// action_runtime_test.go — Shared action runtime, evidence, and constructed dependency tests.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSecureJitterStaysWithinRequestedRange(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		got := secureJitter(7)
		if got < 0 || got >= 7 {
			t.Fatalf("secureJitter(7) = %d", got)
		}
	}
	if got := secureJitter(0); got != 0 {
		t.Fatalf("secureJitter(0) = %d", got)
	}
}

func TestRetryContractStopsUnchangedSecondAttempt(t *testing.T) {
	runtime := NewActionRuntime(RuntimeDeps{})
	firstArgs := json.RawMessage(`{"what":"click","selector":"#save"}`)
	runtime.armRetryContract("first", "", firstArgs)
	runtime.armRetryContract("second", "click", json.RawMessage(`{"selector":"#save","correlation_id":"first"}`))

	data := map[string]any{"error_code": "element_not_found", "effective_url": "https://example.test"}
	decision := runtime.AttachRetryContext("second", data, "error", "")
	if !decision.Terminal || decision.Cause != "strategy_not_changed" {
		t.Fatalf("decision = %+v", decision)
	}
	if data["terminal"] != true || data["retryable"] != false || data["evidence_summary"] == nil {
		t.Fatalf("terminal response = %#v", data)
	}
	context, ok := data["retry_context"].(map[string]any)
	if !ok || context["attempt"] != 2 || context["policy_violation"] != "strategy_unchanged" {
		t.Fatalf("retry context = %#v", data["retry_context"])
	}
}

func TestRetryContractMissingParentAndAttemptLimit(t *testing.T) {
	runtime := NewActionRuntime(RuntimeDeps{})
	runtime.armRetryContract("orphan", "click", json.RawMessage(`{"element_id":"x","correlation_id":"missing"}`))
	orphan, _ := runtime.getRetryState("orphan")
	if orphan.Attempt != 2 || orphan.PolicyViolation != "parent_context_missing" {
		t.Fatalf("orphan = %+v", orphan)
	}

	runtime.armRetryContract("third", "click", json.RawMessage(`{"scope_selector":"main","correlation_id":"orphan"}`))
	third, _ := runtime.getRetryState("third")
	if third.Attempt != maxRetryAttemptsPerStep || third.PolicyViolation != "attempt_limit_exceeded" {
		t.Fatalf("third = %+v", third)
	}
	data := map[string]any{"error": " timeout "}
	decision := runtime.AttachRetryContext("third", data, "timeout", "")
	if !decision.Terminal || decision.Cause != "max_attempts_reached" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestRetryHelpersCoverStrategiesAndPruning(t *testing.T) {
	for _, tc := range []struct {
		args string
		want string
	}{
		{args: `{}`, want: "default"},
		{args: `{"element_id":"e"}`, want: "element_handle"},
		{args: `{"scope_rect":{"x":1}}`, want: "scoped_selector"},
		{args: `{"frame":"main"}`, want: "frame_targeted"},
		{args: `{"selector":"button"}`, want: "selector"},
		{args: `{"index":1}`, want: "indexed"},
		{args: `{"world":"isolated"}`, want: "world_switch"},
	} {
		strategy, fingerprint := deriveRetryStrategy("click", json.RawMessage(tc.args))
		if strategy != tc.want || !strings.Contains(fingerprint, `"action":"click"`) {
			t.Fatalf("%s => %q %q", tc.args, strategy, fingerprint)
		}
	}
	if stableMarshalForRetry(nil) != "" {
		t.Fatal("nil retry map was marshaled")
	}

	runtime := NewActionRuntime(RuntimeDeps{})
	runtime.retryByCommand = map[string]*commandRetryState{
		"old": {CreatedAt: time.Unix(1, 0)},
		"new": {CreatedAt: time.Unix(2, 0)},
	}
	runtime.pruneRetryStatesLocked(1)
	if _, exists := runtime.retryByCommand["old"]; exists {
		t.Fatal("old retry state was not pruned")
	}
	if decision := runtime.AttachRetryContext("missing", map[string]any{}, "error", "fallback"); decision.Terminal {
		t.Fatal("missing retry state became terminal")
	}
	if decision := (*ActionRuntime)(nil).AttachRetryContext("x", map[string]any{}, "error", "fallback"); decision.Terminal {
		t.Fatal("nil runtime became terminal")
	}
}

func TestQueuedResponseRecognitionEdgeCases(t *testing.T) {
	queued := mcp.Succeed(testReq(), "queued", map[string]any{"status": "queued"})
	if !isResponseQueued(queued) {
		t.Fatal("queued response not recognized")
	}
	for _, response := range []mcp.JSONRPCResponse{
		{},
		{Result: json.RawMessage(`bad`)},
		{Result: mustRuntimeJSON(t, mcp.MCPToolResult{})},
		mcp.SucceedText(testReq(), "not json"),
		mcp.Succeed(testReq(), "complete", map[string]any{"status": "complete"}),
	} {
		if isResponseQueued(response) {
			t.Fatalf("false queued response: %s", response.Result)
		}
	}
}

func mustRuntimeJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

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
	capturefixture.SetPilot(c, true)
	capturefixture.Track(c, 1, "https://example.com/page")
	capturefixture.Connect(c)
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
	workflow := NewWorkflowActions(runtime, dom, browser, page, WorkflowDeps{
		Capture: deps.Capture, ToolAnalyze: deps.ToolAnalyze,
		ToolExportSARIF: deps.ToolExportSARIF, Now: time.Now,
	})
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

func TestEvidenceArgumentAndEnvironmentParsing(t *testing.T) {
	for _, tc := range []struct {
		args string
		want evidenceMode
		ok   bool
	}{
		{args: `{}`, want: evidenceModeOff, ok: true},
		{args: `{"evidence":" ALWAYS "}`, want: evidenceModeAlways, ok: true},
		{args: `{"evidence":"on_mutation"}`, want: evidenceModeOnMutation, ok: true},
		{args: `{"evidence":"sometimes"}`, want: evidenceModeOff, ok: false},
	} {
		got, err := ParseEvidenceMode(json.RawMessage(tc.args))
		if (err == nil) != tc.ok || got != tc.want {
			t.Fatalf("ParseEvidenceMode(%s) = %q, %v", tc.args, got, err)
		}
	}
	t.Setenv(evidenceRetryEnv, "bad")
	if got := evidenceRetryCount(); got != 1 {
		t.Fatalf("bad retry = %d", got)
	}
	t.Setenv(evidenceRetryEnv, "-3")
	if got := evidenceRetryCount(); got != 0 {
		t.Fatalf("low retry = %d", got)
	}
	t.Setenv(evidenceRetryEnv, "99")
	if got := evidenceRetryCount(); got != 3 {
		t.Fatalf("high retry = %d", got)
	}
	t.Setenv(evidenceMaxCapturesEnv, "1")
	if got := evidenceMaxCapturesPerCommand(); got != 1 {
		t.Fatalf("capture max = %d", got)
	}
	if got := canonicalActionFromInteractArgs(json.RawMessage(`{"action":" CLICK "}`)); got != "click" {
		t.Fatalf("canonical action = %q", got)
	}
	if !isMutationAction(" Navigate ") || isMutationAction("get_text") {
		t.Fatal("mutation classification mismatch")
	}
}

func TestCaptureEvidencePreconditions(t *testing.T) {
	if got := CaptureEvidence(nil, "client"); got.Error != "capture_not_initialized" {
		t.Fatalf("nil capture = %+v", got)
	}
	store := capture.NewCapture()
	if got := CaptureEvidence(store, "client"); got.Error != "no_tracked_tab" {
		t.Fatalf("untracked capture = %+v", got)
	}
}

func TestCaptureEvidenceResultContracts(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    EvidenceShot
	}{
		{name: "success", payload: `{"path":"/tmp/shot.png","filename":"shot.png"}`, want: EvidenceShot{Path: "/tmp/shot.png", Filename: "shot.png"}},
		{name: "extension error", payload: `{"error":"capture denied"}`, want: EvidenceShot{Error: "capture denied"}},
		{name: "missing path", payload: `{"filename":"shot.png"}`, want: EvidenceShot{Filename: "shot.png", Error: "screenshot_missing_path"}},
		{name: "invalid JSON", payload: `{bad`, want: EvidenceShot{Error: "screenshot_parse_error:"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := capture.NewCapture()
			t.Cleanup(store.Close)
			store.Extension().UpdateTrackedTab(7, "https://example.test", "Example")
			done := make(chan EvidenceShot, 1)
			go func() { done <- CaptureEvidence(store, "client") }()
			store.Queries().WaitForPendingQueries(time.Second)
			queryID := ""
			for _, query := range store.Queries().GetPendingQueries() {
				if query.Type == "screenshot" {
					queryID = query.ID
					break
				}
			}
			if queryID == "" {
				t.Fatal("screenshot query was not queued")
			}
			store.Queries().SetQueryResult(queryID, json.RawMessage(tc.payload))
			select {
			case got := <-done:
				if tc.name == "invalid JSON" {
					if len(got.Error) < len(tc.want.Error) || got.Error[:len(tc.want.Error)] != tc.want.Error {
						t.Fatalf("shot = %+v", got)
					}
				} else if got != tc.want {
					t.Fatalf("shot = %+v, want %+v", got, tc.want)
				}
			case <-time.After(time.Second):
				t.Fatal("CaptureEvidence did not return")
			}
		})
	}
}

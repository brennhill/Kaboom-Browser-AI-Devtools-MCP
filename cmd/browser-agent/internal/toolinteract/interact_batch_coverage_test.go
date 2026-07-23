// interact_batch_coverage_test.go — Behavioural tests for batch execution, response
// enrichment helpers, the command builder, package helpers, and composable side effects.
//
// All package-scope identifiers in this file are prefixed `batchcov` to stay collision-free
// with the other test files in this shared package.

package toolinteract

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================================================
// Fixtures
// ============================================================================

// batchcovAIAction records one RecordAIAction call.
type batchcovAIAction struct {
	action string
	url    string
	extra  map[string]any
}

// batchcovDOMAction records one RecordDOMPrimitiveAction call.
type batchcovDOMAction struct {
	action, selector, text, value string
}

// batchcovWait records one MaybeWaitForCommand call.
type batchcovWait struct {
	correlationID string
	args          string
	queuedSummary string
}

// batchcovEnqueue records one EnqueuePendingQuery call.
type batchcovEnqueue struct {
	query   queries.PendingQuery
	timeout time.Duration
}

// batchcovRec is the shared observation sink for the fake Deps.
type batchcovRec struct {
	mu        sync.Mutex
	enqueued  []batchcovEnqueue
	aiActions []batchcovAIAction
	domActs   []batchcovDOMAction
	waits     []batchcovWait
	toolArgs  []string
}

func (r *batchcovRec) snapshotEnqueued() []batchcovEnqueue {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]batchcovEnqueue, len(r.enqueued))
	copy(out, r.enqueued)
	return out
}

func (r *batchcovRec) lastEnqueued(t *testing.T) queries.PendingQuery {
	t.Helper()
	all := r.snapshotEnqueued()
	if len(all) == 0 {
		t.Fatal("expected at least one enqueued query, got none")
	}
	return all[len(all)-1].query
}

func (r *batchcovRec) snapshotToolArgs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.toolArgs))
	copy(out, r.toolArgs)
	return out
}

// batchcovPassGuard is a guard that never blocks.
func batchcovPassGuard(_ JSONRPCRequest, _ ...func(*StructuredError)) (JSONRPCResponse, bool) {
	return JSONRPCResponse{}, false
}

// batchcovBlockGuard returns a guard that always blocks with the given message,
// applying any StructuredError opts it is handed (so guardsWithOpts is observable).
func batchcovBlockGuard(message string) GuardCheck {
	return func(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool) {
		return fail(req, ErrCodePilotDisabled, message, "Enable pilot mode", opts...), true
	}
}

// batchcovDeps builds a fully-wired fake Deps whose defaults all succeed.
// Individual tests overwrite the fields they care about.
func batchcovDeps() (*Deps, *batchcovRec) {
	rec := &batchcovRec{}
	deps := &Deps{
		// Per-handler replay mutex so parallel batch tests never contend on the
		// package-global ReplayMu.
		ReplayMu:           &sync.Mutex{},
		RequirePilot:       batchcovPassGuard,
		RequireExtension:   batchcovPassGuard,
		RequireTabTracking: batchcovPassGuard,
		RequireCSPClear: func(_ mcp.JSONRPCRequest, _ string) (mcp.JSONRPCResponse, bool) {
			return JSONRPCResponse{}, false
		},
		EnqueuePendingQuery: func(_ mcp.JSONRPCRequest, q queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
			rec.mu.Lock()
			rec.enqueued = append(rec.enqueued, batchcovEnqueue{query: q, timeout: timeout})
			rec.mu.Unlock()
			return JSONRPCResponse{}, false
		},
		MaybeWaitForCommand: func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
			rec.mu.Lock()
			rec.waits = append(rec.waits, batchcovWait{correlationID: correlationID, args: string(args), queuedSummary: queuedSummary})
			rec.mu.Unlock()
			return succeed(req, queuedSummary, map[string]any{"status": "queued", "correlation_id": correlationID})
		},
		RecordAIAction: func(action, url string, extra map[string]any) {
			rec.mu.Lock()
			rec.aiActions = append(rec.aiActions, batchcovAIAction{action: action, url: url, extra: extra})
			rec.mu.Unlock()
		},
		RecordDOMPrimitiveAction: func(action, selector, text, value string) {
			rec.mu.Lock()
			rec.domActs = append(rec.domActs, batchcovDOMAction{action, selector, text, value})
			rec.mu.Unlock()
		},
		ToolInteract: func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			rec.mu.Lock()
			rec.toolArgs = append(rec.toolArgs, string(args))
			rec.mu.Unlock()
			return succeed(req, "Step done", map[string]any{"status": "ok"})
		},
	}
	return deps, rec
}

func batchcovHandler(t *testing.T) (*InteractActionHandler, *batchcovRec) {
	t.Helper()
	deps, rec := batchcovDeps()
	return NewInteractActionHandler(deps), rec
}

func batchcovReq() JSONRPCRequest {
	return JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: float64(7), ClientID: "batchcov-client"}
}

// batchcovResult unmarshals a JSONRPCResponse into an MCPToolResult.
func batchcovResult(t *testing.T, resp JSONRPCResponse) MCPToolResult {
	t.Helper()
	var out MCPToolResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatalf("unmarshal tool result: %v (raw=%s)", err, string(resp.Result))
	}
	return out
}

// batchcovPayload extracts the JSON object that follows the summary line of a
// succeed()/fail() response body.
func batchcovPayload(t *testing.T, resp JSONRPCResponse) map[string]any {
	t.Helper()
	result := batchcovResult(t, resp)
	if len(result.Content) == 0 {
		t.Fatalf("response has no content blocks: %s", string(resp.Result))
	}
	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatalf("no JSON payload in response text: %q", text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse payload %q: %v", text[idx:], err)
	}
	return data
}

// batchcovJSONMap parses raw JSON into a map, failing the test on error.
func batchcovJSONMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse JSON %q: %v", string(raw), err)
	}
	return out
}

// batchcovToolResp wraps a pre-built MCPToolResult as a JSONRPCResponse.
func batchcovToolResp(t *testing.T, result MCPToolResult) JSONRPCResponse {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal MCPToolResult: %v", err)
	}
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: raw}
}

// batchcovTextResp builds a response with a single text content block.
func batchcovTextResp(t *testing.T, text string) JSONRPCResponse {
	t.Helper()
	return batchcovToolResp(t, MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: text}}})
}

// ============================================================================
// helpers.go
// ============================================================================

func TestSucceed_EmbedsSummaryLineThenCompactJSON(t *testing.T) {
	t.Parallel()
	resp := succeed(batchcovReq(), "Batch execution", map[string]any{"status": "ok", "count": 2})

	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", resp.JSONRPC)
	}
	if resp.ID != float64(7) {
		t.Errorf("id = %v, want 7 (request id must be echoed)", resp.ID)
	}
	result := batchcovResult(t, resp)
	if result.IsError {
		t.Error("succeed() must not set isError")
	}
	if len(result.Content) != 1 || result.Content[0].Type != "text" {
		t.Fatalf("content = %+v, want exactly one text block", result.Content)
	}
	text := result.Content[0].Text
	if !strings.HasPrefix(text, "Batch execution\n") {
		t.Fatalf("text = %q, want summary line first", text)
	}
	data := batchcovJSONMap(t, []byte(strings.TrimPrefix(text, "Batch execution\n")))
	if data["status"] != "ok" || data["count"] != float64(2) {
		t.Errorf("payload = %v, want status=ok count=2", data)
	}
}

func TestFail_ProducesStructuredErrorWithAppliedOptions(t *testing.T) {
	t.Parallel()
	resp := fail(batchcovReq(), ErrInvalidParam, "bad selector", "Fix the selector",
		withParam("selector"), withHint("try #id"), withAction("click"), withSelector("#nope"))

	result := batchcovResult(t, resp)
	if !result.IsError {
		t.Fatal("fail() must set isError=true so the LLM sees the failure")
	}
	text := result.Content[0].Text
	if !strings.HasPrefix(text, "Error: invalid_param — Fix the selector\n") {
		t.Fatalf("text header = %q, want 'Error: <code> — <playbook>' first line", text)
	}
	data := batchcovPayload(t, resp)
	for field, want := range map[string]string{
		"error_code":        "invalid_param",
		"message":           "bad selector",
		"recovery_playbook": "Fix the selector",
		"param":             "selector",
		"hint":              "try #id",
		"action":            "click",
		"selector":          "#nope",
	} {
		if data[field] != want {
			t.Errorf("%s = %v, want %q", field, data[field], want)
		}
	}
}

func TestParseArgs_StopsOnMalformedJSONAndPopulatesOnValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     string
		wantStop bool
		wantWhat string
	}{
		{name: "valid object", args: `{"what":"click"}`, wantStop: false, wantWhat: "click"},
		{name: "unknown fields ignored", args: `{"what":"type","zzz":1}`, wantStop: false, wantWhat: "type"},
		{name: "truncated json", args: `{"what":`, wantStop: true},
		{name: "wrong type for field", args: `{"what":42}`, wantStop: true},
		{name: "empty args", args: ``, wantStop: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var v struct {
				What string `json:"what"`
			}
			resp, stop := parseArgs(batchcovReq(), json.RawMessage(tc.args), &v)
			if stop != tc.wantStop {
				t.Fatalf("stop = %v, want %v", stop, tc.wantStop)
			}
			if !stop {
				if v.What != tc.wantWhat {
					t.Errorf("what = %q, want %q", v.What, tc.wantWhat)
				}
				if resp.Result != nil {
					t.Errorf("non-stopping parseArgs must return a zero response, got %s", string(resp.Result))
				}
				return
			}
			data := batchcovPayload(t, resp)
			if data["error_code"] != "invalid_json" {
				t.Errorf("error_code = %v, want invalid_json", data["error_code"])
			}
			if msg, _ := data["message"].(string); !strings.HasPrefix(msg, "Invalid JSON arguments: ") {
				t.Errorf("message = %q, want it to carry the decoder error", msg)
			}
		})
	}
}

func TestRequireString_RejectsOnlyEmptyValues(t *testing.T) {
	t.Parallel()
	if _, blocked := requireString(batchcovReq(), "value", "url", "hint"); blocked {
		t.Fatal("non-empty value must not be rejected")
	}
	// A whitespace-only string is deliberately accepted — requireString checks
	// emptiness, not blankness. Pinning this stops a silent trim being added.
	if _, blocked := requireString(batchcovReq(), "   ", "url", "hint"); blocked {
		t.Fatal("whitespace-only value is currently accepted; behaviour changed")
	}
	resp, blocked := requireString(batchcovReq(), "", "url", "Pass a url")
	if !blocked {
		t.Fatal("empty value must be rejected")
	}
	data := batchcovPayload(t, resp)
	if data["error_code"] != "missing_param" || data["param"] != "url" {
		t.Errorf("error = %v, want missing_param/param=url", data)
	}
	if data["message"] != "Required parameter 'url' is missing" {
		t.Errorf("message = %v", data["message"])
	}
	if data["recovery_playbook"] != "Pass a url" {
		t.Errorf("recovery_playbook = %v, want the caller hint", data["recovery_playbook"])
	}
}

func TestBuildQueryParams_MarshalsMapAndFallsBackToEmptyObject(t *testing.T) {
	t.Parallel()
	got := buildQueryParams(map[string]any{"action": "click", "tab_id": 3})
	data := batchcovJSONMap(t, got)
	if data["action"] != "click" || data["tab_id"] != float64(3) {
		t.Errorf("params = %v, want action=click tab_id=3", data)
	}

	// A channel is unmarshalable; the helper must degrade to "{}" rather than
	// emit nil, which would make PendingQuery.Params invalid JSON downstream.
	fallback := buildQueryParams(map[string]any{"bad": make(chan int)})
	if string(fallback) != "{}" {
		t.Errorf("fallback = %q, want {}", string(fallback))
	}
}

func TestLenientUnmarshal_LeavesTargetUntouchedOnBadInput(t *testing.T) {
	t.Parallel()
	type payload struct {
		N int `json:"n"`
	}
	var v payload
	lenientUnmarshal(nil, &v)
	if v.N != 0 {
		t.Errorf("nil args must be a no-op, got %+v", v)
	}
	lenientUnmarshal(json.RawMessage(`{"n":`), &v)
	if v.N != 0 {
		t.Errorf("malformed args must leave the value untouched, got %+v", v)
	}
	lenientUnmarshal(json.RawMessage(`{"n":5}`), &v)
	if v.N != 5 {
		t.Errorf("n = %d, want 5", v.N)
	}
}

func TestMutateToolResult_AppliesMutationAndSkipsUnparseableResults(t *testing.T) {
	t.Parallel()
	resp := batchcovTextResp(t, "hello")
	mutated := mutateToolResult(resp, func(r *MCPToolResult) {
		r.IsError = true
		r.Content = append(r.Content, MCPContentBlock{Type: "text", Text: "world"})
	})
	result := batchcovResult(t, mutated)
	if !result.IsError {
		t.Error("mutation to IsError was not persisted")
	}
	if len(result.Content) != 2 || result.Content[1].Text != "world" {
		t.Errorf("content = %+v, want the appended block", result.Content)
	}

	called := false
	garbage := JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: json.RawMessage(`not json`)}
	out := mutateToolResult(garbage, func(*MCPToolResult) { called = true })
	if called {
		t.Error("mutation callback must not run when the result cannot be parsed")
	}
	if string(out.Result) != `not json` {
		t.Errorf("result = %q, want the original bytes preserved", string(out.Result))
	}
}

func TestNewCorrelationID_IsPrefixedAndUniqueUnderConcurrency(t *testing.T) {
	t.Parallel()
	const workers, perWorker = 8, 250

	var mu sync.Mutex
	seen := make(map[string]bool, workers*perWorker)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]string, 0, perWorker)
			for i := 0; i < perWorker; i++ {
				local = append(local, newCorrelationID("dom_click"))
			}
			mu.Lock()
			defer mu.Unlock()
			for _, id := range local {
				seen[id] = true
			}
		}()
	}
	wg.Wait()

	if len(seen) != workers*perWorker {
		// Collisions here mean two in-flight commands would share a correlation ID
		// and their results would overwrite each other.
		t.Fatalf("unique IDs = %d, want %d", len(seen), workers*perWorker)
	}
	for id := range seen {
		if !strings.HasPrefix(id, "dom_click_") {
			t.Fatalf("id %q lost its prefix", id)
		}
		if strings.Count(id, "_") != 3 {
			t.Fatalf("id %q does not match prefix_timestamp_random shape", id)
		}
	}
}

func TestCheckGuards_ShortCircuitsAtFirstBlocker(t *testing.T) {
	t.Parallel()
	var ran []string
	mark := func(name string, block bool) GuardCheck {
		return func(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool) {
			ran = append(ran, name)
			if block {
				return fail(req, ErrCodePilotDisabled, name+" blocked", "retry", opts...), true
			}
			return JSONRPCResponse{}, false
		}
	}

	resp, blocked := checkGuards(batchcovReq(), mark("a", false), mark("b", true), mark("c", false))
	if !blocked {
		t.Fatal("expected blocked=true")
	}
	if strings.Join(ran, ",") != "a,b" {
		t.Errorf("guards ran = %v, want a,b (c must not run after a blocker)", ran)
	}
	if got := batchcovPayload(t, resp)["message"]; got != "b blocked" {
		t.Errorf("message = %v, want the blocking guard's message", got)
	}

	ran = nil
	if _, blocked := checkGuards(batchcovReq(), mark("a", false), mark("b", false)); blocked {
		t.Error("no guard blocked, want blocked=false")
	}
	if len(ran) != 2 {
		t.Errorf("guards ran = %v, want both", ran)
	}
}

func TestCheckGuardsWithOpts_PassesOptionsThroughToGuardErrors(t *testing.T) {
	t.Parallel()
	opts := []func(*StructuredError){withAction("click"), withSelector("#submit")}
	resp, blocked := checkGuardsWithOpts(batchcovReq(), opts, batchcovPassGuard, batchcovBlockGuard("pilot off"))
	if !blocked {
		t.Fatal("expected the second guard to block")
	}
	data := batchcovPayload(t, resp)
	if data["action"] != "click" || data["selector"] != "#submit" {
		t.Errorf("guard error = %v, want action/selector context carried through", data)
	}

	if _, blocked := checkGuardsWithOpts(batchcovReq(), opts, batchcovPassGuard); blocked {
		t.Error("expected blocked=false when no guard blocks")
	}
}

// ============================================================================
// interact_batch.go — pure helpers
// ============================================================================

func TestForceReplayAsyncInteractStep_PinsSyncAndWaitToFalse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		in        string
		unchanged bool
		keep      map[string]any
	}{
		{name: "adds both flags when absent", in: `{"what":"click","selector":"#a"}`, keep: map[string]any{"what": "click", "selector": "#a"}},
		{name: "overrides caller-supplied true", in: `{"what":"click","sync":true,"wait":true}`, keep: map[string]any{"what": "click"}},
		{name: "leaves already-false flags alone", in: `{"what":"click","sync":false,"wait":false}`, keep: map[string]any{"what": "click"}},
		{name: "malformed json passes through", in: `{"what":`, unchanged: true},
		{name: "json array passes through", in: `["click"]`, unchanged: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := forceReplayAsyncInteractStep(json.RawMessage(tc.in))
			if tc.unchanged {
				if string(got) != tc.in {
					t.Fatalf("got %q, want the input returned verbatim", string(got))
				}
				return
			}
			data := batchcovJSONMap(t, got)
			// If either flag leaked through as true the nested interact call would
			// block on MaybeWaitForCommand and the batch would serialise on it.
			if data["sync"] != false {
				t.Errorf("sync = %v, want false", data["sync"])
			}
			if data["wait"] != false {
				t.Errorf("wait = %v, want false", data["wait"])
			}
			for k, want := range tc.keep {
				if data[k] != want {
					t.Errorf("%s = %v, want %v", k, data[k], want)
				}
			}
		})
	}
}

func TestStripComposableScreenshotFromStep_RemovesOnlyThatKey(t *testing.T) {
	t.Parallel()
	got := StripComposableScreenshotFromStep(json.RawMessage(`{"what":"click","include_screenshot":true,"selector":"#a"}`))
	data := batchcovJSONMap(t, got)
	if _, present := data["include_screenshot"]; present {
		t.Error("include_screenshot must be stripped: per-step screenshots are discarded by the aggregate response")
	}
	if data["what"] != "click" || data["selector"] != "#a" {
		t.Errorf("other keys lost: %v", data)
	}

	// Absent key: byte-identical passthrough (no needless re-marshal).
	orig := `{"what":"click"}`
	if got := StripComposableScreenshotFromStep(json.RawMessage(orig)); string(got) != orig {
		t.Errorf("got %q, want %q unchanged", string(got), orig)
	}
	bad := `{"what":`
	if got := StripComposableScreenshotFromStep(json.RawMessage(bad)); string(got) != bad {
		t.Errorf("malformed input: got %q, want passthrough", string(got))
	}
	// include_screenshot=false must be stripped too — the key's presence is what matters.
	got = StripComposableScreenshotFromStep(json.RawMessage(`{"include_screenshot":false}`))
	if _, present := batchcovJSONMap(t, got)["include_screenshot"]; present {
		t.Error("include_screenshot:false must also be stripped")
	}
}

func TestExtractCorrelationIDFromToolResponse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp JSONRPCResponse
		want string
	}{
		{name: "nil result", resp: JSONRPCResponse{}, want: ""},
		{name: "no content blocks", resp: JSONRPCResponse{Result: json.RawMessage(`{"content":[]}`)}, want: ""},
		{name: "plain json body", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"{\"correlation_id\":\"abc_1\"}"}]}`)}, want: "abc_1"},
		{name: "summary line stripped", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"Click queued\n{\"correlation_id\":\"abc_2\"}"}]}`)}, want: "abc_2"},
		{name: "empty correlation id ignored", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"{\"correlation_id\":\"\"}"}]}`)}, want: ""},
		{name: "non-string correlation id ignored", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"{\"correlation_id\":42}"}]}`)}, want: ""},
		{name: "image block skipped, later text used", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"image","data":"xx"},{"type":"text","text":"{\"correlation_id\":\"abc_3\"}"}]}`)}, want: "abc_3"},
		{name: "unparseable block skipped, later text used", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"not json"},{"type":"text","text":"{\"correlation_id\":\"abc_4\"}"}]}`)}, want: "abc_4"},
		{name: "result not a tool result", resp: JSONRPCResponse{Result: json.RawMessage(`"oops"`)}, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := extractCorrelationIDFromToolResponse(tc.resp); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractErrorMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp JSONRPCResponse
		want string
	}{
		{name: "nil result", resp: JSONRPCResponse{}, want: ""},
		{name: "message field of a pure json body", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"{\"message\":\"element not found\"}"}]}`)}, want: "element not found"},
		{name: "json without message falls back to raw text", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"{\"status\":\"error\"}"}]}`)}, want: `{"status":"error"}`},
		{name: "non-json text returned verbatim", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"boom"}]}`)}, want: "boom"},
		{name: "empty and non-text blocks skipped", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":""},{"type":"image","data":"x"},{"type":"text","text":"second"}]}`)}, want: "second"},
		{name: "no content", resp: JSONRPCResponse{Result: json.RawMessage(`{"content":[]}`)}, want: ""},
		{name: "unparseable result", resp: JSONRPCResponse{Result: json.RawMessage(`nope`)}, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := extractErrorMessage(tc.resp); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestExtractErrorMessage_DoesNotUnwrapStandardFailResponses documents that a
// response built by fail() ("Error: code — playbook\n{json}") is NOT reduced to
// its message field: the leading summary line makes the whole block non-JSON, so
// the raw text is returned. Batch step errors therefore carry the full banner.
func TestExtractErrorMessage_DoesNotUnwrapStandardFailResponses(t *testing.T) {
	t.Parallel()
	resp := fail(batchcovReq(), ErrInvalidParam, "selector required", "Add a selector", withParam("selector"))
	got := extractErrorMessage(resp)
	if got == "selector required" {
		t.Fatal("behaviour changed: the summary line is now stripped before parsing")
	}
	if !strings.HasPrefix(got, "Error: invalid_param — Add a selector\n") {
		t.Errorf("got %q, want the raw formatted error text", got)
	}
}

// ============================================================================
// interact_batch.go — HandleBatch
// ============================================================================

// batchcovBatchArgs marshals batch params, embedding raw step JSON verbatim.
func batchcovBatchArgs(t *testing.T, steps []string, extra map[string]any) json.RawMessage {
	t.Helper()
	raw := make([]json.RawMessage, 0, len(steps))
	for _, s := range steps {
		raw = append(raw, json.RawMessage(s))
	}
	payload := map[string]any{"steps": raw}
	for k, v := range extra {
		payload[k] = v
	}
	out, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal batch args: %v", err)
	}
	return out
}

func TestBatch_RejectsInvalidStepLists(t *testing.T) {
	t.Parallel()
	fifty1 := make([]string, 51)
	for i := range fifty1 {
		fifty1[i] = `{"what":"click"}`
	}
	tests := []struct {
		name        string
		args        json.RawMessage
		wantCode    string
		wantMessage string
	}{
		{
			name:        "no steps key",
			args:        json.RawMessage(`{}`),
			wantCode:    ErrInvalidParam,
			wantMessage: "Steps must be a non-empty array",
		},
		{
			name:        "empty steps array",
			args:        json.RawMessage(`{"steps":[]}`),
			wantCode:    ErrInvalidParam,
			wantMessage: "Steps must be a non-empty array",
		},
		{
			name:        "one over the 50-step cap",
			args:        batchcovBatchArgs(t, fifty1, nil),
			wantCode:    ErrInvalidParam,
			wantMessage: "Steps exceeds maximum of 50",
		},
		{
			name:        "step without what or action",
			args:        json.RawMessage(`{"steps":[{"what":"click"},{"selector":"#a"}]}`),
			wantCode:    ErrInvalidParam,
			wantMessage: "Step[1] missing required 'what' field",
		},
		{
			name:        "step that is not an object",
			args:        json.RawMessage(`{"steps":["click"]}`),
			wantCode:    ErrInvalidParam,
			wantMessage: "Step[0] missing required 'what' field",
		},
		{
			name:        "malformed args",
			args:        json.RawMessage(`{"steps":`),
			wantCode:    ErrInvalidJSON,
			wantMessage: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, rec := batchcovHandler(t)
			resp := h.HandleBatch(batchcovReq(), tc.args)
			data := batchcovPayload(t, resp)
			if data["error_code"] != tc.wantCode {
				t.Errorf("error_code = %v, want %v", data["error_code"], tc.wantCode)
			}
			if tc.wantMessage != "" && data["message"] != tc.wantMessage {
				t.Errorf("message = %v, want %q", data["message"], tc.wantMessage)
			}
			if got := len(rec.snapshotToolArgs()); got != 0 {
				t.Errorf("%d steps dispatched, want 0 on a validation failure", got)
			}
		})
	}
}

func TestBatch_StepWithActionInsteadOfWhatIsAccepted(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	resp := h.HandleBatch(batchcovReq(), json.RawMessage(`{"steps":[{"action":"scroll"}]}`))
	data := batchcovPayload(t, resp)
	if data["status"] != "ok" {
		t.Fatalf("status = %v, want ok (legacy 'action' key must satisfy the what requirement): %v", data["status"], data)
	}
	if len(rec.snapshotToolArgs()) != 1 {
		t.Fatalf("dispatched %d steps, want 1", len(rec.snapshotToolArgs()))
	}
	results, _ := data["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %v, want one entry", data["results"])
	}
	step := results[0].(map[string]any)
	if step["action"] != "scroll" {
		t.Errorf("result action = %v, want scroll (falls back to the 'action' key)", step["action"])
	}
}

func TestBatch_GuardBlockHappensBeforeAnyStepRuns(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	h.deps.RequireExtension = batchcovBlockGuard("extension not connected")

	resp := h.HandleBatch(batchcovReq(), json.RawMessage(`{"steps":[{"what":"click"},{"what":"type"}]}`))
	data := batchcovPayload(t, resp)
	if data["message"] != "extension not connected" {
		t.Errorf("message = %v, want the guard message", data["message"])
	}
	if got := len(rec.snapshotToolArgs()); got != 0 {
		t.Errorf("%d steps dispatched, want 0 — guards must fail fast (#9.R3.9)", got)
	}
	if got := len(rec.aiActions); got != 0 {
		t.Errorf("%d audit records written, want 0 when guards block", got)
	}
}

func TestBatch_RejectsConcurrentBatchWhileReplayMutexHeld(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	var mu sync.Mutex
	h.deps.ReplayMu = &mu
	mu.Lock()
	defer mu.Unlock()

	resp := h.HandleBatch(batchcovReq(), json.RawMessage(`{"steps":[{"what":"click"}]}`))
	data := batchcovPayload(t, resp)
	if data["message"] != "Another batch or sequence is currently executing" {
		t.Fatalf("message = %v, want the concurrency rejection", data["message"])
	}
	if data["error_code"] != ErrInvalidParam {
		t.Errorf("error_code = %v, want invalid_param", data["error_code"])
	}
	if got := len(rec.snapshotToolArgs()); got != 0 {
		t.Errorf("%d steps dispatched, want 0 while another batch holds the lock", got)
	}
}

func TestBatch_ReleasesReplayMutexAfterCompletion(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	var mu sync.Mutex
	h.deps.ReplayMu = &mu

	h.HandleBatch(batchcovReq(), json.RawMessage(`{"steps":[{"what":"click"}]}`))
	if !mu.TryLock() {
		t.Fatal("replay mutex still held after HandleBatch returned")
	}
	mu.Unlock()
}

func TestBatch_AllStepsSucceed_ReportsOkAndAuditsOnce(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	steps := []string{`{"what":"click","selector":"#a"}`, `{"what":"type","text":"hi"}`, `{"what":"scroll"}`}
	resp := h.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, nil))

	if batchcovResult(t, resp).IsError {
		t.Fatal("a fully successful batch must not be an error response")
	}
	data := batchcovPayload(t, resp)
	for field, want := range map[string]any{
		"status":         "ok",
		"steps_executed": float64(3),
		"steps_failed":   float64(0),
		"steps_queued":   float64(0),
		"steps_total":    float64(3),
	} {
		if data[field] != want {
			t.Errorf("%s = %v, want %v", field, data[field], want)
		}
	}
	if msg, _ := data["message"].(string); !strings.HasPrefix(msg, "Batch executed: 3/3 steps in ") {
		t.Errorf("message = %q", msg)
	}
	results, _ := data["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("results = %v, want 3 entries", data["results"])
	}
	wantActions := []string{"click", "type", "scroll"}
	for i, r := range results {
		step := r.(map[string]any)
		if step["step_index"] != float64(i) {
			t.Errorf("results[%d].step_index = %v, want %d", i, step["step_index"], i)
		}
		if step["action"] != wantActions[i] {
			t.Errorf("results[%d].action = %v, want %s", i, step["action"], wantActions[i])
		}
		if step["status"] != "ok" {
			t.Errorf("results[%d].status = %v, want ok", i, step["status"])
		}
	}

	if len(rec.aiActions) != 1 {
		t.Fatalf("audit records = %d, want exactly 1 for the batch itself", len(rec.aiActions))
	}
	if rec.aiActions[0].action != "batch" || rec.aiActions[0].extra["steps"] != 3 {
		t.Errorf("audit record = %+v, want action=batch steps=3", rec.aiActions[0])
	}
}

func TestBatch_StopsOnFirstFailureWhenContinueOnErrorFalse(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	h.deps.ToolInteract = func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		rec.mu.Lock()
		n := len(rec.toolArgs)
		rec.toolArgs = append(rec.toolArgs, string(args))
		rec.mu.Unlock()
		if n == 1 {
			return fail(req, ErrInvalidParam, "selector not found", "Use list_interactive")
		}
		return succeed(req, "Step done", map[string]any{"status": "ok"})
	}

	steps := []string{`{"what":"click"}`, `{"what":"type"}`, `{"what":"scroll"}`}
	resp := h.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, map[string]any{"continue_on_error": false}))
	data := batchcovPayload(t, resp)

	if got := len(rec.snapshotToolArgs()); got != 2 {
		t.Fatalf("dispatched %d steps, want 2 — step 3 must not run after the failure", got)
	}
	if data["status"] != "error" {
		t.Errorf("status = %v, want error", data["status"])
	}
	if data["message"] != "Batch failed at step 2/3" {
		t.Errorf("message = %v, want 'Batch failed at step 2/3'", data["message"])
	}
	if data["steps_executed"] != float64(2) || data["steps_failed"] != float64(1) {
		t.Errorf("executed/failed = %v/%v, want 2/1", data["steps_executed"], data["steps_failed"])
	}
	results, _ := data["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results = %v, want 2 entries", data["results"])
	}
	failed := results[1].(map[string]any)
	if failed["status"] != "error" {
		t.Errorf("failing step status = %v, want error", failed["status"])
	}
	if msg, _ := failed["error"].(string); !strings.Contains(msg, "selector not found") {
		t.Errorf("failing step error = %q, want it to carry the tool error", msg)
	}
}

func TestBatch_ContinuesPastFailuresByDefault(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	h.deps.ToolInteract = func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		rec.mu.Lock()
		n := len(rec.toolArgs)
		rec.toolArgs = append(rec.toolArgs, string(args))
		rec.mu.Unlock()
		if n == 0 {
			return fail(req, ErrInvalidParam, "boom", "retry")
		}
		return succeed(req, "Step done", map[string]any{"status": "ok"})
	}

	steps := []string{`{"what":"click"}`, `{"what":"type"}`, `{"what":"scroll"}`}
	resp := h.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, nil))
	data := batchcovPayload(t, resp)

	if got := len(rec.snapshotToolArgs()); got != 3 {
		t.Fatalf("dispatched %d steps, want all 3 (continue_on_error defaults to true)", got)
	}
	if data["status"] != "partial" {
		t.Errorf("status = %v, want partial", data["status"])
	}
	if data["message"] != "Batch partially executed: 2/3 steps succeeded, 1 failed" {
		t.Errorf("message = %v", data["message"])
	}
	if data["steps_executed"] != float64(3) || data["steps_failed"] != float64(1) {
		t.Errorf("executed/failed = %v/%v, want 3/1", data["steps_executed"], data["steps_failed"])
	}
}

func TestBatch_AllStepsFail_ReportsErrorStatus(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	h.deps.ToolInteract = func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return fail(req, ErrExtError, "extension exploded", "retry")
	}

	steps := []string{`{"what":"click"}`, `{"what":"type"}`}
	resp := h.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, nil))
	data := batchcovPayload(t, resp)

	if data["status"] != "error" {
		t.Errorf("status = %v, want error", data["status"])
	}
	if data["message"] != "Batch failed: all 2 executed steps had errors" {
		t.Errorf("message = %v", data["message"])
	}
	if data["steps_failed"] != float64(2) {
		t.Errorf("steps_failed = %v, want 2", data["steps_failed"])
	}
}

func TestBatch_StopAfterStepCapsExecutionButNotTotal(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	steps := []string{`{"what":"a"}`, `{"what":"b"}`, `{"what":"c"}`, `{"what":"d"}`}
	resp := h.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, map[string]any{"stop_after_step": 2}))
	data := batchcovPayload(t, resp)

	if got := len(rec.snapshotToolArgs()); got != 2 {
		t.Fatalf("dispatched %d steps, want 2", got)
	}
	if data["steps_executed"] != float64(2) {
		t.Errorf("steps_executed = %v, want 2", data["steps_executed"])
	}
	if data["steps_total"] != float64(4) {
		t.Errorf("steps_total = %v, want 4 (the declared list length, not the cap)", data["steps_total"])
	}
	if msg, _ := data["message"].(string); !strings.HasPrefix(msg, "Batch executed: 2/4 steps in ") {
		t.Errorf("message = %q", msg)
	}

	// A stop_after_step at or beyond the list length is a no-op.
	h2, rec2 := batchcovHandler(t)
	h2.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, map[string]any{"stop_after_step": 9}))
	if got := len(rec2.snapshotToolArgs()); got != 4 {
		t.Errorf("dispatched %d steps with stop_after_step=9, want all 4", got)
	}
}

func TestBatch_RewritesStepArgsToAsyncAndDropsScreenshots(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	steps := []string{`{"what":"click","selector":"#a","include_screenshot":true,"sync":true,"wait":true}`}
	h.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, nil))

	args := rec.snapshotToolArgs()
	if len(args) != 1 {
		t.Fatalf("dispatched %d steps, want 1", len(args))
	}
	data := batchcovJSONMap(t, []byte(args[0]))
	if _, present := data["include_screenshot"]; present {
		t.Error("include_screenshot must be stripped before dispatch (#9.2.2)")
	}
	if data["sync"] != false || data["wait"] != false {
		t.Errorf("sync=%v wait=%v, want both false so the nested call does not block", data["sync"], data["wait"])
	}
	if data["selector"] != "#a" || data["what"] != "click" {
		t.Errorf("step payload lost fields: %v", data)
	}
}

func TestBatch_CorrelationLifecycleDrivesStepStatuses(t *testing.T) {
	t.Parallel()
	store := capture.NewCapture()
	defer store.Close()

	// Long registration TTL so nothing expires mid-test; the batch's own
	// step_timeout_ms is what bounds each WaitForCommand call.
	const ttl = time.Minute
	store.RegisterCommand("corr_ok", "q0", ttl)
	store.CompleteCommand("corr_ok", nil, "")
	store.RegisterCommand("corr_err", "q1", ttl)
	store.ApplyCommandResult("corr_err", "error", nil, "click intercepted")
	store.RegisterCommand("corr_err_blank", "q2", ttl)
	store.ApplyCommandResult("corr_err_blank", "timeout", nil, "")
	store.RegisterCommand("corr_pending", "q4", ttl) // never resolved

	corrIDs := []string{"corr_ok", "corr_err", "corr_err_blank", "corr_unregistered", "corr_pending"}

	h, rec := batchcovHandler(t)
	h.deps.Capture = func() *capture.Store { return store }
	h.deps.ToolInteract = func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		rec.mu.Lock()
		n := len(rec.toolArgs)
		rec.toolArgs = append(rec.toolArgs, string(args))
		rec.mu.Unlock()
		return succeed(req, "queued", map[string]any{"status": "queued", "correlation_id": corrIDs[n]})
	}

	steps := []string{`{"what":"a"}`, `{"what":"b"}`, `{"what":"c"}`, `{"what":"d"}`, `{"what":"e"}`}
	resp := h.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, map[string]any{"step_timeout_ms": 1}))
	data := batchcovPayload(t, resp)

	results, _ := data["results"].([]any)
	if len(results) != 5 {
		t.Fatalf("results = %v, want 5 entries", data["results"])
	}
	type want struct{ status, errText, corr string }
	wants := []want{
		{status: "ok", corr: "corr_ok"},
		{status: "error", errText: "click intercepted", corr: "corr_err"},
		{status: "error", errText: "command failed with status timeout", corr: "corr_err_blank"},
		{status: "queued", corr: "corr_unregistered"},
		{status: "queued", corr: "corr_pending"},
	}
	for i, w := range wants {
		step := results[i].(map[string]any)
		if step["status"] != w.status {
			t.Errorf("results[%d].status = %v, want %s", i, step["status"], w.status)
		}
		if step["correlation_id"] != w.corr {
			t.Errorf("results[%d].correlation_id = %v, want %s", i, step["correlation_id"], w.corr)
		}
		got, _ := step["error"].(string)
		if got != w.errText {
			t.Errorf("results[%d].error = %q, want %q", i, got, w.errText)
		}
	}

	if data["steps_failed"] != float64(2) || data["steps_queued"] != float64(2) {
		t.Errorf("failed/queued = %v/%v, want 2/2", data["steps_failed"], data["steps_queued"])
	}
	if data["status"] != "partial" {
		t.Errorf("status = %v, want partial (queued and failed both present)", data["status"])
	}
	if data["message"] != "Batch executed with failures: 2 queued, 2 failed" {
		t.Errorf("message = %v", data["message"])
	}
}

func TestBatch_AllQueuedReportsQueuedStatus(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	store := capture.NewCapture()
	defer store.Close()
	h.deps.Capture = func() *capture.Store { return store }
	h.deps.ToolInteract = func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		rec.mu.Lock()
		rec.toolArgs = append(rec.toolArgs, string(args))
		rec.mu.Unlock()
		return succeed(req, "queued", map[string]any{"correlation_id": "never_registered"})
	}

	steps := []string{`{"what":"a"}`, `{"what":"b"}`}
	resp := h.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, map[string]any{"step_timeout_ms": 1}))
	data := batchcovPayload(t, resp)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want queued", data["status"])
	}
	if data["message"] != "Batch queued: 2/2 steps still running" {
		t.Errorf("message = %v", data["message"])
	}
}

// TestBatch_ContinueOnErrorFalseDoesNotStopOnExtensionSideFailure documents a real
// defect: the `break` for continue_on_error=false lives inside the
// isErrorResponse(stepResp) branch only. A step whose *tool* response was a normal
// "queued" success but whose extension command resolved to "error" is counted as
// failed yet the batch keeps going. Fixing that should flip this assertion to 1.
func TestBatch_ContinueOnErrorFalseDoesNotStopOnExtensionSideFailure(t *testing.T) {
	t.Parallel()
	store := capture.NewCapture()
	defer store.Close()
	store.RegisterCommand("corr_boom", "q0", time.Minute)
	store.ApplyCommandResult("corr_boom", "error", nil, "element detached")

	h, rec := batchcovHandler(t)
	h.deps.Capture = func() *capture.Store { return store }
	h.deps.ToolInteract = func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		rec.mu.Lock()
		n := len(rec.toolArgs)
		rec.toolArgs = append(rec.toolArgs, string(args))
		rec.mu.Unlock()
		if n == 0 {
			return succeed(req, "queued", map[string]any{"correlation_id": "corr_boom"})
		}
		return succeed(req, "Step done", map[string]any{"status": "ok"})
	}

	steps := []string{`{"what":"a"}`, `{"what":"b"}`}
	resp := h.HandleBatch(batchcovReq(), batchcovBatchArgs(t, steps, map[string]any{
		"continue_on_error": false,
		"step_timeout_ms":   1,
	}))

	if got := len(rec.snapshotToolArgs()); got != 2 {
		t.Fatalf("dispatched %d steps, want 2 — this test pins the CURRENT (buggy) behaviour", got)
	}
	data := batchcovPayload(t, resp)
	if data["steps_failed"] != float64(1) {
		t.Errorf("steps_failed = %v, want 1", data["steps_failed"])
	}
	if data["status"] != "partial" {
		t.Errorf("status = %v, want partial under current behaviour", data["status"])
	}
}

// ============================================================================
// interact_command_builder.go
// ============================================================================

func TestCommandBuilder_RequiresQueryType(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	resp, corr := h.newCommand("highlight").
		correlationPrefix("highlight").
		executeWithCorrelation(batchcovReq(), json.RawMessage(`{}`))

	data := batchcovPayload(t, resp)
	if data["error_code"] != ErrMissingParam {
		t.Errorf("error_code = %v, want missing_param", data["error_code"])
	}
	if data["message"] != "commandBuilder: queryType is required" {
		t.Errorf("message = %v", data["message"])
	}
	if corr != "" {
		t.Errorf("correlation id = %q, want empty when validation fails before correlation", corr)
	}
	if len(rec.snapshotEnqueued()) != 0 {
		t.Error("nothing may be enqueued when queryType is missing")
	}
}

func TestCommandBuilder_CorrelationPrefixFallsBackToBuilderName(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	_, corr := h.newCommand("scroll_page").
		queryType("dom_action").
		executeWithCorrelation(batchcovReq(), json.RawMessage(`{}`))
	if !strings.HasPrefix(corr, "scroll_page_") {
		t.Fatalf("correlation id = %q, want the builder name as prefix", corr)
	}

	_, corr = h.newCommand("scroll_page").
		correlationPrefix("dom_scroll").
		queryType("dom_action").
		executeWithCorrelation(batchcovReq(), json.RawMessage(`{}`))
	if !strings.HasPrefix(corr, "dom_scroll_") {
		t.Fatalf("correlation id = %q, want the explicit prefix to win", corr)
	}
}

func TestCommandBuilder_GuardsShortCircuitBeforeEnqueue(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	resp, corr := h.newCommand("click").
		queryType("dom_action").
		guards(batchcovPassGuard, batchcovBlockGuard("pilot disabled"), batchcovPassGuard).
		executeWithCorrelation(batchcovReq(), json.RawMessage(`{}`))

	if corr != "" {
		t.Errorf("correlation id = %q, want empty when a guard blocks", corr)
	}
	if got := batchcovPayload(t, resp)["message"]; got != "pilot disabled" {
		t.Errorf("message = %v, want the guard message", got)
	}
	if len(rec.snapshotEnqueued()) != 0 {
		t.Error("guard block must prevent enqueue")
	}
	if len(rec.waits) != 0 {
		t.Error("guard block must prevent MaybeWaitForCommand")
	}
}

func TestCommandBuilder_GuardOptsReachGuardErrorPayload(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	resp, _ := h.newCommand("click").
		queryType("dom_action").
		guardsWithOpts(domActionContextOptions("click", "#submit"), batchcovBlockGuard("pilot disabled")).
		executeWithCorrelation(batchcovReq(), json.RawMessage(`{}`))

	data := batchcovPayload(t, resp)
	if data["action"] != "click" || data["selector"] != "#submit" {
		t.Errorf("guard error = %v, want action/selector context attached", data)
	}
}

func TestCommandBuilder_CSPGuardRunsAfterOtherGuards(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	var sawWorld string
	h.deps.RequireCSPClear = func(req mcp.JSONRPCRequest, world string) (mcp.JSONRPCResponse, bool) {
		sawWorld = world
		return fail(req, ErrInvalidParam, "page CSP blocks main-world eval", "Use world=isolated"), true
	}

	resp, corr := h.newCommand("execute_js").
		queryType("execute").
		guards(batchcovPassGuard).
		cspGuard("main").
		executeWithCorrelation(batchcovReq(), json.RawMessage(`{}`))

	if sawWorld != "main" {
		t.Errorf("csp guard world = %q, want main", sawWorld)
	}
	if corr != "" {
		t.Errorf("correlation id = %q, want empty when CSP blocks", corr)
	}
	if got := batchcovPayload(t, resp)["message"]; got != "page CSP blocks main-world eval" {
		t.Errorf("message = %v", got)
	}
	if len(rec.snapshotEnqueued()) != 0 {
		t.Error("CSP block must prevent enqueue")
	}

	// An empty cspWorld must skip the guard entirely.
	h2, _ := batchcovHandler(t)
	called := false
	h2.deps.RequireCSPClear = func(_ mcp.JSONRPCRequest, _ string) (mcp.JSONRPCResponse, bool) {
		called = true
		return JSONRPCResponse{}, false
	}
	h2.newCommand("execute_js").queryType("execute").execute(batchcovReq(), json.RawMessage(`{}`))
	if called {
		t.Error("RequireCSPClear must not run when cspGuard was never configured")
	}
}

func TestCommandBuilder_EnqueuesQueryWithConfiguredFields(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	_, corr := h.newCommand("highlight").
		correlationPrefix("highlight").
		queryType("dom_action").
		buildParams(map[string]any{"action": "highlight", "selector": "#hero"}).
		tabID(42).
		executeWithCorrelation(batchcovReq(), json.RawMessage(`{"selector":"#hero"}`))

	all := rec.snapshotEnqueued()
	if len(all) != 1 {
		t.Fatalf("enqueued %d queries, want 1", len(all))
	}
	q := all[0].query
	if q.Type != "dom_action" {
		t.Errorf("query type = %q, want dom_action", q.Type)
	}
	if q.TabID != 42 {
		t.Errorf("tab id = %d, want 42", q.TabID)
	}
	if q.CorrelationID != corr {
		t.Errorf("query correlation %q != returned %q", q.CorrelationID, corr)
	}
	params := batchcovJSONMap(t, q.Params)
	if params["action"] != "highlight" || params["selector"] != "#hero" {
		t.Errorf("params = %v, want the buildParams map", params)
	}
	if all[0].timeout != queries.AsyncCommandTimeout {
		t.Errorf("timeout = %v, want the AsyncCommandTimeout default", all[0].timeout)
	}
}

func TestCommandBuilder_ExplicitTimeoutOverridesDefault(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	b := h.newCommand("slow").queryType("execute")
	// No fluent setter exists for qTimeout; set it directly to pin the
	// "zero means AsyncCommandTimeout" fallback as a fallback, not a constant.
	b.qTimeout = 2500 * time.Millisecond
	b.execute(batchcovReq(), json.RawMessage(`{}`))

	all := rec.snapshotEnqueued()
	if len(all) != 1 {
		t.Fatalf("enqueued %d queries, want 1", len(all))
	}
	if all[0].timeout != 2500*time.Millisecond {
		t.Errorf("timeout = %v, want 2.5s", all[0].timeout)
	}
}

func TestCommandBuilder_FallsBackToWaitArgsWhenQueryParamsUnset(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	waitArgs := json.RawMessage(`{"selector":"#a","what":"click"}`)
	h.newCommand("click").queryType("dom_action").execute(batchcovReq(), waitArgs)

	q := rec.lastEnqueued(t)
	if string(q.Params) != string(waitArgs) {
		t.Errorf("params = %s, want the original waitArgs %s", string(q.Params), string(waitArgs))
	}

	// queryParams wins when supplied.
	h2, rec2 := batchcovHandler(t)
	h2.newCommand("click").
		queryType("dom_action").
		queryParams(json.RawMessage(`{"action":"click"}`)).
		execute(batchcovReq(), waitArgs)
	if got := string(rec2.lastEnqueued(t).Params); got != `{"action":"click"}` {
		t.Errorf("params = %s, want the explicit queryParams", got)
	}
}

func TestCommandBuilder_PreEnqueueSeesCorrelationIDBeforeEnqueue(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	var seenCorr string
	var enqueuedAtCallback int
	_, corr := h.newCommand("perf").
		correlationPrefix("perf").
		queryType("execute").
		preEnqueue(func(correlationID string) {
			seenCorr = correlationID
			enqueuedAtCallback = len(rec.snapshotEnqueued())
		}).
		executeWithCorrelation(batchcovReq(), json.RawMessage(`{}`))

	if seenCorr != corr || corr == "" {
		t.Errorf("preEnqueue saw %q, execute returned %q", seenCorr, corr)
	}
	if enqueuedAtCallback != 0 {
		t.Error("preEnqueue must run before the query is enqueued")
	}
}

func TestCommandBuilder_EnqueueFailureSkipsRecordingAndWait(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	postRan := false
	h.deps.EnqueuePendingQuery = func(req mcp.JSONRPCRequest, _ queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
		return fail(req, ErrQueueFull, "command queue is full", "Retry shortly"), true
	}

	resp, corr := h.newCommand("click").
		queryType("dom_action").
		recordAction("dom_click", "https://example.test/", map[string]any{"selector": "#a"}).
		postEnqueue(func() { postRan = true }).
		executeWithCorrelation(batchcovReq(), json.RawMessage(`{}`))

	if got := batchcovPayload(t, resp)["error_code"]; got != ErrQueueFull {
		t.Errorf("error_code = %v, want queue_full", got)
	}
	if corr == "" {
		// The caller still needs the ID to correlate the failure with armed evidence.
		t.Error("correlation id must still be returned when enqueue is rejected")
	}
	if len(rec.aiActions) != 0 {
		t.Errorf("AI action recorded despite enqueue failure: %+v", rec.aiActions)
	}
	if postRan {
		t.Error("postEnqueue must not run when enqueue is rejected")
	}
	if len(rec.waits) != 0 {
		t.Error("MaybeWaitForCommand must not run when enqueue is rejected")
	}
}

func TestCommandBuilder_RecordsActionThenPostEnqueueThenWaits(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	var order []string
	base := h.deps.RecordAIAction
	h.deps.RecordAIAction = func(action, url string, extra map[string]any) {
		order = append(order, "record")
		base(action, url, extra)
	}
	prevWait := h.deps.MaybeWaitForCommand
	h.deps.MaybeWaitForCommand = func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
		order = append(order, "wait")
		return prevWait(req, correlationID, args, queuedSummary)
	}

	waitArgs := json.RawMessage(`{"selector":"#a"}`)
	resp, corr := h.newCommand("click").
		queryType("dom_action").
		recordAction("dom_click", "https://example.test/", map[string]any{"selector": "#a"}).
		postEnqueue(func() { order = append(order, "post") }).
		queuedMessage("click queued").
		executeWithCorrelation(batchcovReq(), waitArgs)

	if strings.Join(order, ",") != "record,post,wait" {
		t.Errorf("call order = %v, want record,post,wait", order)
	}
	if len(rec.aiActions) != 1 {
		t.Fatalf("AI actions = %+v, want 1", rec.aiActions)
	}
	got := rec.aiActions[0]
	if got.action != "dom_click" || got.url != "https://example.test/" || got.extra["selector"] != "#a" {
		t.Errorf("recorded action = %+v", got)
	}
	if len(rec.waits) != 1 {
		t.Fatalf("waits = %+v, want 1", rec.waits)
	}
	w := rec.waits[0]
	if w.correlationID != corr {
		t.Errorf("wait correlation = %q, want %q", w.correlationID, corr)
	}
	if w.queuedSummary != "click queued" {
		t.Errorf("queued summary = %q, want 'click queued'", w.queuedSummary)
	}
	if w.args != string(waitArgs) {
		t.Errorf("wait args = %q, want the original args %q", w.args, string(waitArgs))
	}
	if got := batchcovPayload(t, resp)["correlation_id"]; got != corr {
		t.Errorf("response correlation_id = %v, want %q", got, corr)
	}
}

// TestBatch_FallsBackToPackageGlobalReplayMutex covers the nil-Deps.ReplayMu path.
// Deliberately NOT parallel: it takes the process-wide ReplayMu.
func TestBatch_FallsBackToPackageGlobalReplayMutex(t *testing.T) {
	h, rec := batchcovHandler(t)
	h.deps.ReplayMu = nil

	resp := h.HandleBatch(batchcovReq(), json.RawMessage(`{"steps":[{"what":"click"}]}`))
	data := batchcovPayload(t, resp)
	if data["status"] != "ok" {
		t.Fatalf("status = %v, want ok when falling back to the global mutex: %v", data["status"], data)
	}
	if len(rec.snapshotToolArgs()) != 1 {
		t.Errorf("dispatched %d steps, want 1", len(rec.snapshotToolArgs()))
	}
	if !ReplayMu.TryLock() {
		t.Fatal("package-global ReplayMu still held after HandleBatch returned")
	}
	ReplayMu.Unlock()
}

// ============================================================================
// interact_response_helpers.go
// ============================================================================

func TestIsResponseQueued(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp JSONRPCResponse
		want bool
	}{
		{name: "nil result", resp: JSONRPCResponse{}, want: false},
		{name: "empty content", resp: JSONRPCResponse{Result: json.RawMessage(`{"content":[]}`)}, want: false},
		{name: "unparseable result", resp: JSONRPCResponse{Result: json.RawMessage(`garbage`)}, want: false},
		{name: "status queued in bare json", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"{\"status\":\"queued\"}"}]}`)}, want: true},
		{name: "status queued after summary line", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"Click queued\n{\"status\":\"queued\"}"}]}`)}, want: true},
		{name: "status ok", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"{\"status\":\"ok\"}"}]}`)}, want: false},
		{name: "status not a string", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"text","text":"{\"status\":1}"}]}`)}, want: false},
		{name: "queued found in a later block", resp: JSONRPCResponse{Result: json.RawMessage(
			`{"content":[{"type":"image","data":"x"},{"type":"text","text":"nope"},{"type":"text","text":"{\"status\":\"queued\"}"}]}`)}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isResponseQueued(tc.resp); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAppendScreenshotToResponse_AppendsFirstImageBlockOnly(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	h.deps.GetScreenshot = func(_ mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return batchcovToolResp(t, MCPToolResult{Content: []MCPContentBlock{
			{Type: "text", Text: "screenshot captured"},
			{Type: "image", Data: "AAAA", MimeType: "image/png"},
			{Type: "image", Data: "BBBB", MimeType: "image/png"},
		}})
	}

	base := batchcovTextResp(t, "Click ok")
	out := h.AppendScreenshotToResponse(base, batchcovReq())
	result := batchcovResult(t, out)
	if len(result.Content) != 2 {
		t.Fatalf("content = %+v, want the original block plus exactly one image", result.Content)
	}
	if result.Content[0].Text != "Click ok" {
		t.Errorf("original block mutated: %+v", result.Content[0])
	}
	img := result.Content[1]
	if img.Type != "image" || img.Data != "AAAA" || img.MimeType != "image/png" {
		t.Errorf("appended block = %+v, want the first image block verbatim", img)
	}
}

func TestAppendScreenshotToResponse_LeavesResponseUnchangedWhenNoUsableImage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		screenshot func(t *testing.T) JSONRPCResponse
	}{
		{
			name:       "screenshot result is unparseable",
			screenshot: func(*testing.T) JSONRPCResponse { return JSONRPCResponse{Result: json.RawMessage(`nope`)} },
		},
		{
			name: "screenshot has no image block",
			screenshot: func(t *testing.T) JSONRPCResponse {
				return batchcovToolResp(t, MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: "no camera"}}})
			},
		},
		{
			name: "image block has empty data",
			screenshot: func(t *testing.T) JSONRPCResponse {
				return batchcovToolResp(t, MCPToolResult{Content: []MCPContentBlock{{Type: "image", Data: ""}}})
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, _ := batchcovHandler(t)
			shot := tc.screenshot(t)
			h.deps.GetScreenshot = func(_ mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse { return shot }

			base := batchcovTextResp(t, "Click ok")
			out := h.AppendScreenshotToResponse(base, batchcovReq())
			if string(out.Result) != string(base.Result) {
				t.Errorf("result = %s, want unchanged %s", string(out.Result), string(base.Result))
			}
		})
	}
}

func TestAppendInteractiveToResponse_AppendsElementsSection(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	const listText = "list_interactive results\n{\"elements\":[{\"index\":0,\"selector\":\"#go\"}]}"
	h.deps.MaybeWaitForCommand = func(_ mcp.JSONRPCRequest, _ string, _ json.RawMessage, _ string) mcp.JSONRPCResponse {
		return batchcovTextResp(t, listText)
	}

	base := batchcovTextResp(t, "Click ok")
	out := h.AppendInteractiveToResponse(base, batchcovReq())
	result := batchcovResult(t, out)
	if len(result.Content) != 2 {
		t.Fatalf("content = %+v, want two blocks", result.Content)
	}
	if result.Content[1].Type != "text" {
		t.Errorf("appended block type = %q, want text", result.Content[1].Type)
	}
	appended := result.Content[1].Text
	if !strings.HasPrefix(appended, "\n--- Interactive Elements ---\nlist_interactive results\n") {
		t.Fatalf("appended text = %q, want the section header then the list block", appended)
	}
	// HandleListInteractive annotates the payload with index metadata before this
	// helper sees it; the appended section must carry that annotation through.
	body := appended[strings.Index(appended, "{"):]
	data := batchcovJSONMap(t, []byte(body))
	elems, _ := data["elements"].([]any)
	if len(elems) != 1 || elems[0].(map[string]any)["selector"] != "#go" {
		t.Errorf("elements = %v, want the single #go element", data["elements"])
	}
	if gen, _ := data["index_generation"].(string); !strings.HasPrefix(gen, "dom_list_") {
		t.Errorf("index_generation = %v, want the list correlation id", data["index_generation"])
	}
}

func TestAppendInteractiveToResponse_SkipsWhenListInteractiveFails(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	// A blocked guard makes HandleListInteractive return an isError result; the
	// primary response must survive untouched rather than gain a broken section.
	h.deps.RequirePilot = batchcovBlockGuard("pilot disabled")

	base := batchcovTextResp(t, "Click ok")
	out := h.AppendInteractiveToResponse(base, batchcovReq())
	if string(out.Result) != string(base.Result) {
		t.Errorf("result = %s, want unchanged", string(out.Result))
	}
}

func TestReadPageContext_ExtractsOnlyNonEmptyFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		page    string
		isError bool
		wantOK  bool
		want    map[string]any
	}{
		{
			name:   "url title and tab_id after summary line",
			page:   "Page info\n{\"url\":\"https://a.test/\",\"title\":\"A\",\"tab_id\":12}",
			wantOK: true,
			want:   map[string]any{"url": "https://a.test/", "title": "A", "tab_id": float64(12)},
		},
		{
			name:   "empty strings are dropped",
			page:   "{\"url\":\"\",\"title\":\"Only title\"}",
			wantOK: true,
			want:   map[string]any{"title": "Only title"},
		},
		{
			name:   "tab_id zero is still reported",
			page:   "{\"tab_id\":0}",
			wantOK: true,
			want:   map[string]any{"tab_id": float64(0)},
		},
		{name: "no recognised fields", page: "{\"foo\":\"bar\"}", wantOK: false},
		{name: "unparseable text", page: "not json at all", wantOK: false},
		{name: "error result", page: "{\"url\":\"https://a.test/\"}", isError: true, wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, _ := batchcovHandler(t)
			h.deps.GetPageInfo = func(_ mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
				return batchcovToolResp(t, MCPToolResult{
					Content: []MCPContentBlock{{Type: "text", Text: tc.page}},
					IsError: tc.isError,
				})
			}

			got, ok := h.readPageContext(batchcovReq())
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (got %v)", ok, tc.wantOK, got)
			}
			if !tc.wantOK {
				if got != nil {
					t.Errorf("context = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("context = %v, want %v", got, tc.want)
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("%s = %v, want %v", k, got[k], want)
				}
			}
		})
	}
}

func TestAppendPageContextToResponse_AddsMetadataAndTextBlock(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	h.deps.GetPageInfo = func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return succeed(req, "Page", map[string]any{"url": "https://a.test/x", "title": "X", "tab_id": 3})
	}

	base := batchcovTextResp(t, "Click ok")
	out := h.AppendPageContextToResponse(base, batchcovReq())
	result := batchcovResult(t, out)

	if len(result.Content) != 2 {
		t.Fatalf("content = %+v, want two blocks", result.Content)
	}
	if !strings.HasPrefix(result.Content[1].Text, "\n--- Page Context ---\n") {
		t.Fatalf("appended block = %q", result.Content[1].Text)
	}
	inline := batchcovJSONMap(t, []byte(strings.TrimPrefix(result.Content[1].Text, "\n--- Page Context ---\n")))
	if inline["url"] != "https://a.test/x" || inline["title"] != "X" || inline["tab_id"] != float64(3) {
		t.Errorf("inline context = %v", inline)
	}
	// Metadata matters: MCP clients read page_context structurally, not from prose.
	meta, ok := result.Metadata["page_context"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %v, want a page_context object", result.Metadata)
	}
	if meta["url"] != "https://a.test/x" || meta["tab_id"] != float64(3) {
		t.Errorf("metadata page_context = %v", meta)
	}
}

func TestAppendPageContextToResponse_UnchangedWhenContextUnavailable(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	h.deps.GetPageInfo = func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return succeed(req, "Page", map[string]any{"unrelated": true})
	}
	base := batchcovTextResp(t, "Click ok")
	if out := h.AppendPageContextToResponse(base, batchcovReq()); string(out.Result) != string(base.Result) {
		t.Errorf("result = %s, want unchanged", string(out.Result))
	}

	// An unparseable primary response must also survive untouched.
	h.deps.GetPageInfo = func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return succeed(req, "Page", map[string]any{"url": "https://a.test/"})
	}
	broken := JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: json.RawMessage(`not-json`)}
	if out := h.AppendPageContextToResponse(broken, batchcovReq()); string(out.Result) != "not-json" {
		t.Errorf("result = %s, want the original bytes", string(out.Result))
	}
}

func TestAppendWorkflowTraceToResponse_AddsEnvelopeToMetadataOnly(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	start := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	trace := []WorkflowStep{
		{Action: "navigate", Status: "success", TimingMs: 120},
		{Action: "click", Status: "error", TimingMs: -5, Detail: "not found"},
	}

	base := batchcovTextResp(t, "Workflow done")
	out := h.AppendWorkflowTraceToResponse(base, "fill_form", trace, start, "failed")
	result := batchcovResult(t, out)

	if len(result.Content) != 1 || result.Content[0].Text != "Workflow done" {
		t.Errorf("content was modified: %+v (trace belongs in metadata)", result.Content)
	}
	if result.Metadata["trace_id"] != "workflow_fill_form_1704164645000000000" {
		t.Errorf("trace_id = %v, want a workflow_<name>_<startNanos> id", result.Metadata["trace_id"])
	}
	env, ok := result.Metadata["workflow_trace"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %v, want a workflow_trace object", result.Metadata)
	}
	if env["workflow"] != "fill_form" || env["status"] != "failed" {
		t.Errorf("envelope = %v, want workflow=fill_form status=failed", env)
	}
	stages, _ := env["stages"].([]any)
	if len(stages) != 2 {
		t.Fatalf("stages = %v, want 2", env["stages"])
	}
	s0 := stages[0].(map[string]any)
	if s0["stage"] != "navigate" || s0["duration_ms"] != float64(120) {
		t.Errorf("stage 0 = %v", s0)
	}
	s1 := stages[1].(map[string]any)
	if s1["duration_ms"] != float64(0) {
		t.Errorf("stage 1 duration = %v, want a negative timing clamped to 0", s1["duration_ms"])
	}
	if s1["error"] != "not found" {
		t.Errorf("stage 1 error = %v, want the failing step's detail", s1["error"])
	}
}

func TestAppendWorkflowTraceToResponse_UnchangedOnUnparseableResult(t *testing.T) {
	t.Parallel()
	h, _ := batchcovHandler(t)
	broken := JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: json.RawMessage(`{{{`)}
	out := h.AppendWorkflowTraceToResponse(broken, "nav", nil, time.Now(), "ok")
	if string(out.Result) != "{{{" {
		t.Errorf("result = %s, want the original bytes preserved", string(out.Result))
	}
}

// ============================================================================
// interact_composable.go
// ============================================================================

func TestHandleWaitForStable_InjectsDefaultsWithoutClobberingCallerValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		args          json.RawMessage
		wantStability float64
		wantTimeout   float64
		wantTabID     int
	}{
		{name: "nil args", args: nil, wantStability: 500, wantTimeout: 5000},
		{name: "empty object", args: json.RawMessage(`{}`), wantStability: 500, wantTimeout: 5000},
		{name: "zero values treated as unset", args: json.RawMessage(`{"stability_ms":0,"timeout_ms":0}`), wantStability: 500, wantTimeout: 5000},
		{name: "negative values treated as unset", args: json.RawMessage(`{"stability_ms":-1,"timeout_ms":-9}`), wantStability: 500, wantTimeout: 5000},
		{name: "explicit values preserved", args: json.RawMessage(`{"stability_ms":250,"timeout_ms":9000,"tab_id":4}`), wantStability: 250, wantTimeout: 9000, wantTabID: 4},
		{name: "malformed args fall back to defaults", args: json.RawMessage(`{"stability_ms":`), wantStability: 500, wantTimeout: 5000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, rec := batchcovHandler(t)
			resp := h.HandleWaitForStable(batchcovReq(), tc.args)
			if batchcovResult(t, resp).IsError {
				t.Fatalf("unexpected error response: %s", string(resp.Result))
			}
			q := rec.lastEnqueued(t)
			if q.Type != "dom_action" {
				t.Errorf("query type = %q, want dom_action", q.Type)
			}
			if q.TabID != tc.wantTabID {
				t.Errorf("tab id = %d, want %d", q.TabID, tc.wantTabID)
			}
			params := batchcovJSONMap(t, q.Params)
			if params["action"] != "wait_for_stable" {
				t.Errorf("action = %v, want wait_for_stable", params["action"])
			}
			if params["stability_ms"] != tc.wantStability {
				t.Errorf("stability_ms = %v, want %v", params["stability_ms"], tc.wantStability)
			}
			if params["timeout_ms"] != tc.wantTimeout {
				t.Errorf("timeout_ms = %v, want %v", params["timeout_ms"], tc.wantTimeout)
			}
			if !strings.HasPrefix(q.CorrelationID, "dom_wait_for_stable_") {
				t.Errorf("correlation id = %q, want a dom_wait_for_stable prefix", q.CorrelationID)
			}
		})
	}
}

func TestHandleAutoDismissOverlays_DispatchesDomActionWithoutSelector(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	// auto_dismiss_overlays is selector-optional; a selector requirement here would
	// break the standalone consent-banner path.
	resp := h.HandleAutoDismissOverlays(batchcovReq(), json.RawMessage(`{"tab_id":8}`))
	if batchcovResult(t, resp).IsError {
		t.Fatalf("unexpected error response: %s", string(resp.Result))
	}
	q := rec.lastEnqueued(t)
	if q.Type != "dom_action" || q.TabID != 8 {
		t.Errorf("query = %+v, want dom_action on tab 8", q)
	}
	if got := batchcovJSONMap(t, q.Params)["action"]; got != "auto_dismiss_overlays" {
		t.Errorf("action = %v, want auto_dismiss_overlays", got)
	}
	if len(rec.domActs) != 1 || rec.domActs[0].action != "auto_dismiss_overlays" {
		t.Errorf("recorded DOM actions = %+v, want one auto_dismiss_overlays entry", rec.domActs)
	}
}

func TestQueueComposableSideEffects_EnqueueExpectedQueries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		queue      func(h *InteractActionHandler)
		wantType   string
		wantPrefix string
		wantParams map[string]any
	}{
		{
			name:       "auto dismiss overlays",
			queue:      func(h *InteractActionHandler) { h.QueueComposableAutoDismiss(batchcovReq()) },
			wantType:   "dom_action",
			wantPrefix: "dom_auto_dismiss_overlays_",
			wantParams: map[string]any{"action": "auto_dismiss_overlays"},
		},
		{
			name:       "action diff",
			queue:      func(h *InteractActionHandler) { h.QueueComposableActionDiff(batchcovReq()) },
			wantType:   "dom_action",
			wantPrefix: "dom_action_diff_",
			wantParams: map[string]any{"action": "action_diff", "timeout_ms": float64(3000)},
		},
		{
			name:       "wait for stable with explicit stability",
			queue:      func(h *InteractActionHandler) { h.QueueComposableWaitForStable(batchcovReq(), 250) },
			wantType:   "dom_action",
			wantPrefix: "dom_wait_for_stable_",
			wantParams: map[string]any{"action": "wait_for_stable", "stability_ms": float64(250), "timeout_ms": float64(5000)},
		},
		{
			name:       "wait for stable defaults non-positive stability",
			queue:      func(h *InteractActionHandler) { h.QueueComposableWaitForStable(batchcovReq(), 0) },
			wantType:   "dom_action",
			wantPrefix: "dom_wait_for_stable_",
			wantParams: map[string]any{"action": "wait_for_stable", "stability_ms": float64(500), "timeout_ms": float64(5000)},
		},
		{
			name:       "subtitle",
			queue:      func(h *InteractActionHandler) { h.QueueComposableSubtitle(batchcovReq(), "Filling the form") },
			wantType:   "subtitle",
			wantPrefix: "subtitle_",
			wantParams: map[string]any{"text": "Filling the form"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, rec := batchcovHandler(t)
			tc.queue(h)

			all := rec.snapshotEnqueued()
			if len(all) != 1 {
				t.Fatalf("enqueued %d queries, want exactly 1", len(all))
			}
			q := all[0].query
			if q.Type != tc.wantType {
				t.Errorf("query type = %q, want %q", q.Type, tc.wantType)
			}
			if !strings.HasPrefix(q.CorrelationID, tc.wantPrefix) {
				t.Errorf("correlation id = %q, want prefix %q", q.CorrelationID, tc.wantPrefix)
			}
			if q.TabID != 0 {
				t.Errorf("tab id = %d, want 0 (side effects target the active tab)", q.TabID)
			}
			if all[0].timeout != queries.AsyncCommandTimeout {
				t.Errorf("timeout = %v, want AsyncCommandTimeout", all[0].timeout)
			}
			params := batchcovJSONMap(t, q.Params)
			if len(params) != len(tc.wantParams) {
				t.Errorf("params = %v, want exactly %v", params, tc.wantParams)
			}
			for k, want := range tc.wantParams {
				if params[k] != want {
					t.Errorf("params[%s] = %v, want %v", k, params[k], want)
				}
			}
		})
	}
}

func TestQueueComposableSideEffects_SwallowBlockedEnqueue(t *testing.T) {
	t.Parallel()
	h, rec := batchcovHandler(t)
	// Side effects are fire-and-forget: a rejected enqueue must not panic or
	// surface, because the primary action's response is what the caller sees.
	h.deps.EnqueuePendingQuery = func(req mcp.JSONRPCRequest, q queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
		rec.mu.Lock()
		rec.enqueued = append(rec.enqueued, batchcovEnqueue{query: q, timeout: timeout})
		rec.mu.Unlock()
		return fail(req, ErrQueueFull, "queue full", "Retry"), true
	}

	h.QueueComposableAutoDismiss(batchcovReq())
	h.QueueComposableActionDiff(batchcovReq())
	h.QueueComposableWaitForStable(batchcovReq(), 100)
	h.QueueComposableSubtitle(batchcovReq(), "hi")

	if got := len(rec.snapshotEnqueued()); got != 4 {
		t.Fatalf("enqueue attempts = %d, want 4", got)
	}
}

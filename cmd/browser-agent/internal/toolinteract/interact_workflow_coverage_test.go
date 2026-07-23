// interact_workflow_coverage_test.go — Behavioural tests for the interact_workflow_* handlers.
// Covers: navigate_and_wait_for, fill_form, fill_form_and_submit, run_a11y_and_export_sarif,
// navigate_and_document. All package-scope identifiers are prefixed `wfcov`.

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

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// wfcovCall records one command that reached EnqueuePendingQuery.
type wfcovCall struct {
	QueryType     string
	Action        string
	Params        map[string]any
	CorrelationID string
	TabID         int
}

// wfcovStr returns a string param from the recorded call ("" when absent).
func (c wfcovCall) wfcovStr(key string) string {
	if v, ok := c.Params[key].(string); ok {
		return v
	}
	return ""
}

// wfcovNum returns a numeric param from the recorded call (found=false when absent).
func (c wfcovCall) wfcovNum(key string) (float64, bool) {
	switch v := c.Params[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

// wfcovHarness wires a fully-stubbed Deps so workflow handlers run end to end
// without a browser, network, or the main package.
type wfcovHarness struct {
	t   *testing.T
	h   *InteractActionHandler
	cap *capture.Store

	mu         sync.Mutex
	calls      []wfcovCall
	byCorr     map[string]wfcovCall
	domRecords []string

	// Hooks — set before invoking a handler.
	reply        func(call wfcovCall) (JSONRPCResponse, bool)
	enqueueBlock func(call wfcovCall) (JSONRPCResponse, bool)
	delay        map[string]time.Duration

	analyzeArgs []json.RawMessage
	sarifArgs   []json.RawMessage
	analyze     func(args json.RawMessage) JSONRPCResponse
	sarif       func(args json.RawMessage) JSONRPCResponse

	pageInfo     func() JSONRPCResponse
	enrichCount  int
	enrichTabIDs []int
	enrich       func(resp JSONRPCResponse) JSONRPCResponse
}

func wfcovNewHarness(t *testing.T) *wfcovHarness {
	t.Helper()
	hs := &wfcovHarness{
		t:      t,
		cap:    capture.NewCapture(),
		byCorr: map[string]wfcovCall{},
		delay:  map[string]time.Duration{},
	}
	pass := func(JSONRPCRequest, ...func(*StructuredError)) (JSONRPCResponse, bool) {
		return JSONRPCResponse{}, false
	}
	deps := &Deps{
		RequirePilot:       pass,
		RequireExtension:   pass,
		RequireTabTracking: pass,
		Capture:            func() *capture.Store { return hs.cap },
		EnqueuePendingQuery: func(req JSONRPCRequest, q queries.PendingQuery, _ time.Duration) (JSONRPCResponse, bool) {
			call := wfcovCall{
				QueryType:     q.Type,
				CorrelationID: q.CorrelationID,
				TabID:         q.TabID,
				Params:        map[string]any{},
			}
			_ = json.Unmarshal(q.Params, &call.Params)
			call.Action, _ = call.Params["action"].(string)
			hs.mu.Lock()
			hs.calls = append(hs.calls, call)
			hs.byCorr[q.CorrelationID] = call
			block := hs.enqueueBlock
			hs.mu.Unlock()
			if block != nil {
				return block(call)
			}
			return JSONRPCResponse{}, false
		},
		MaybeWaitForCommand: func(req JSONRPCRequest, correlationID string, _ json.RawMessage, _ string) JSONRPCResponse {
			hs.mu.Lock()
			call := hs.byCorr[correlationID]
			reply := hs.reply
			d := hs.delay[call.Action]
			hs.mu.Unlock()
			if d > 0 {
				// Fixture latency: simulates a slow extension round-trip so the
				// workflow's own timeout budget can be exercised. Not a wait.
				time.Sleep(d)
			}
			if reply != nil {
				if resp, ok := reply(call); ok {
					return resp
				}
			}
			return wfcovOK(req, call.Action)
		},
		RecordAIAction: func(string, string, map[string]any) {},
		RecordDOMPrimitiveAction: func(action, selector, text, value string) {
			hs.mu.Lock()
			hs.domRecords = append(hs.domRecords, action+"|"+selector+"|"+text+"|"+value)
			hs.mu.Unlock()
		},
		EnrichNavigateResponse: func(resp JSONRPCResponse, _ JSONRPCRequest, tabID int) JSONRPCResponse {
			hs.mu.Lock()
			hs.enrichCount++
			hs.enrichTabIDs = append(hs.enrichTabIDs, tabID)
			fn := hs.enrich
			hs.mu.Unlock()
			if fn != nil {
				return fn(resp)
			}
			return resp
		},
		InjectCSPBlockedActions: func(resp JSONRPCResponse) JSONRPCResponse { return resp },
		ToolAnalyze: func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			hs.mu.Lock()
			hs.analyzeArgs = append(hs.analyzeArgs, args)
			fn := hs.analyze
			hs.mu.Unlock()
			if fn != nil {
				return fn(args)
			}
			return wfcovOK(req, "analyze")
		},
		ToolExportSARIF: func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			hs.mu.Lock()
			hs.sarifArgs = append(hs.sarifArgs, args)
			fn := hs.sarif
			hs.mu.Unlock()
			if fn != nil {
				return fn(args)
			}
			return wfcovOK(req, "sarif")
		},
		GetPageInfo: func(req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
			hs.mu.Lock()
			fn := hs.pageInfo
			hs.mu.Unlock()
			if fn != nil {
				return fn()
			}
			return fail(req, ErrNoData, "no page info", "n/a")
		},
	}
	hs.h = NewInteractActionHandler(deps)
	return hs
}

func (hs *wfcovHarness) wfcovActions() []string {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	out := make([]string, 0, len(hs.calls))
	for _, c := range hs.calls {
		out = append(out, c.Action)
	}
	return out
}

func (hs *wfcovHarness) wfcovCallAt(i int) wfcovCall {
	hs.t.Helper()
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if i >= len(hs.calls) {
		hs.t.Fatalf("no enqueued command at index %d (have %d: %v)", i, len(hs.calls), hs.calls)
	}
	return hs.calls[i]
}

// ---------------------------------------------------------------------------
// Response fixtures + payload readers
// ---------------------------------------------------------------------------

func wfcovReq() JSONRPCRequest {
	return JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: float64(1), ClientID: "wfcov-client"}
}

func wfcovOK(req JSONRPCRequest, action string) JSONRPCResponse {
	return succeed(req, action+" ok", map[string]any{"status": "ok", "action": action})
}

func wfcovErr(req JSONRPCRequest, msg string) JSONRPCResponse {
	return fail(req, ErrExtError, msg, "retry")
}

// wfcovPending builds a non-final (async correlation pending) success response.
func wfcovPending(req JSONRPCRequest, correlationID string) JSONRPCResponse {
	return succeed(req, "click still processing", map[string]any{
		"status":         "still_processing",
		"final":          false,
		"correlation_id": correlationID,
	})
}

// wfcovPayload parses the JSON object that follows the summary line of the
// first text content block.
func wfcovPayload(t *testing.T, resp JSONRPCResponse) map[string]any {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v (raw=%s)", err, string(resp.Result))
	}
	if len(result.Content) == 0 {
		t.Fatalf("response has no content blocks: %s", string(resp.Result))
	}
	text := result.Content[0].Text
	nl := strings.Index(text, "\n")
	if nl < 0 {
		t.Fatalf("no JSON payload after summary line: %q", text)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text[nl+1:]), &out); err != nil {
		t.Fatalf("parse payload %q: %v", text[nl+1:], err)
	}
	return out
}

func wfcovIsError(t *testing.T, resp JSONRPCResponse) bool {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	return result.IsError
}

func wfcovMetadata(t *testing.T, resp JSONRPCResponse) map[string]any {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	return result.Metadata
}

// wfcovTraceOf reads the workflow trace steps out of a workflowResult payload.
func wfcovTraceOf(t *testing.T, resp JSONRPCResponse) []map[string]any {
	t.Helper()
	payload := wfcovPayload(t, resp)
	raw, ok := payload["trace"].([]any)
	if !ok {
		t.Fatalf("payload has no trace array: %v", payload)
	}
	out := make([]map[string]any, 0, len(raw))
	for _, e := range raw {
		m, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("trace entry is not an object: %v", e)
		}
		out = append(out, m)
	}
	return out
}

// wfcovStepNames flattens a trace to "action=status" pairs for readable asserts.
func wfcovStepNames(steps []map[string]any) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, wfcovMapStr(s, "action")+"="+wfcovMapStr(s, "status"))
	}
	return out
}

func wfcovMapStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// wfcovEnvelopeStages reads the normalized stage envelope written into MCP
// metadata by AppendWorkflowTraceToResponse.
func wfcovEnvelopeStages(t *testing.T, resp JSONRPCResponse) (status string, stages []string) {
	t.Helper()
	meta := wfcovMetadata(t, resp)
	if meta == nil {
		t.Fatalf("response carries no metadata: %s", string(resp.Result))
	}
	env, ok := meta["workflow_trace"].(map[string]any)
	if !ok {
		t.Fatalf("metadata has no workflow_trace: %v", meta)
	}
	status = wfcovMapStr(env, "status")
	raw, _ := env["stages"].([]any)
	for _, e := range raw {
		m, _ := e.(map[string]any)
		stages = append(stages, wfcovMapStr(m, "stage")+"="+wfcovMapStr(m, "status"))
	}
	return status, stages
}

func wfcovJoin(items []string) string { return strings.Join(items, ",") }

// ---------------------------------------------------------------------------
// navigate_and_wait_for (interact_workflow_navigate.go)
// ---------------------------------------------------------------------------

func TestNavigateAndWaitFor_RejectsMalformedArgs(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleNavigateAndWaitFor(wfcovReq(), json.RawMessage(`{"url":`))

	if !wfcovIsError(t, resp) {
		t.Fatal("malformed JSON must produce an error response")
	}
	payload := wfcovPayload(t, resp)
	if got := wfcovMapStr(payload, "error_code"); got != ErrInvalidJSON {
		t.Fatalf("error_code = %q, want %q", got, ErrInvalidJSON)
	}
	if len(hs.wfcovActions()) != 0 {
		t.Fatalf("no command may be enqueued for malformed args, got %v", hs.wfcovActions())
	}
}

func TestNavigateAndWaitFor_RequiresURLAndSelector(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		args      string
		wantParam string
	}{
		{"missing url", `{"wait_for":"#main"}`, "url"},
		{"missing wait_for", `{"url":"https://example.test/"}`, "wait_for"},
		{"blank url", `{"url":"","wait_for":"#main"}`, "url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := wfcovNewHarness(t)
			resp := hs.h.HandleNavigateAndWaitFor(wfcovReq(), json.RawMessage(tc.args))
			if !wfcovIsError(t, resp) {
				t.Fatal("expected an error response")
			}
			payload := wfcovPayload(t, resp)
			if got := wfcovMapStr(payload, "error_code"); got != ErrMissingParam {
				t.Fatalf("error_code = %q, want %q", got, ErrMissingParam)
			}
			if got := wfcovMapStr(payload, "param"); got != tc.wantParam {
				t.Fatalf("param = %q, want %q", got, tc.wantParam)
			}
			if len(hs.wfcovActions()) != 0 {
				t.Fatalf("validation must run before dispatch, got %v", hs.wfcovActions())
			}
		})
	}
}

func TestNavigateAndWaitFor_DispatchesNavigateThenWaitWithTabScope(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleNavigateAndWaitFor(wfcovReq(),
		json.RawMessage(`{"url":"https://example.test/app","wait_for":"#ready","tab_id":42}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("workflow should succeed: %s", string(resp.Result))
	}
	if got := wfcovJoin(hs.wfcovActions()); got != "navigate,wait_for" {
		t.Fatalf("dispatched actions = %q, want \"navigate,wait_for\"", got)
	}

	nav := hs.wfcovCallAt(0)
	if nav.QueryType != "browser_action" {
		t.Errorf("navigate query type = %q, want browser_action", nav.QueryType)
	}
	if nav.wfcovStr("url") != "https://example.test/app" {
		t.Errorf("navigate url = %q", nav.wfcovStr("url"))
	}
	if nav.TabID != 42 {
		t.Errorf("navigate tab_id = %d, want 42", nav.TabID)
	}

	wait := hs.wfcovCallAt(1)
	if wait.QueryType != "dom_action" {
		t.Errorf("wait query type = %q, want dom_action", wait.QueryType)
	}
	if wait.wfcovStr("selector") != "#ready" {
		t.Errorf("wait selector = %q, want #ready", wait.wfcovStr("selector"))
	}
	if wait.TabID != 42 {
		t.Errorf("wait tab_id = %d, want 42", wait.TabID)
	}

	steps := wfcovTraceOf(t, resp)
	if got := wfcovJoin(wfcovStepNames(steps)); got != "navigate=success,wait_for=success" {
		t.Fatalf("trace = %q", got)
	}
	if got := wfcovMapStr(steps[0], "detail"); got != "https://example.test/app" {
		t.Errorf("navigate step detail = %q, want the target URL", got)
	}
	if got := wfcovMapStr(steps[1], "detail"); got != "#ready" {
		t.Errorf("wait step detail = %q, want the selector", got)
	}
	payload := wfcovPayload(t, resp)
	if wfcovMapStr(payload, "status") != "success" {
		t.Errorf("workflow status = %v, want success", payload["status"])
	}
	if payload["steps"] != float64(2) || payload["successful"] != float64(2) {
		t.Errorf("steps/successful = %v/%v, want 2/2", payload["steps"], payload["successful"])
	}
}

func TestNavigateAndWaitFor_FailedNavigationSkipsSelectorWait(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "navigate" {
			return wfcovErr(wfcovReq(), "tab crashed"), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleNavigateAndWaitFor(wfcovReq(),
		json.RawMessage(`{"url":"https://example.test/","wait_for":"#ready"}`))

	// Regression guard: waiting for a selector on a page that never loaded burns
	// the caller's whole timeout for no reason.
	if got := wfcovJoin(hs.wfcovActions()); got != "navigate" {
		t.Fatalf("dispatched actions = %q, want only \"navigate\"", got)
	}
	if !wfcovIsError(t, resp) {
		t.Fatal("workflow must report isError when navigation fails")
	}
	steps := wfcovTraceOf(t, resp)
	if got := wfcovJoin(wfcovStepNames(steps)); got != "navigate=error" {
		t.Fatalf("trace = %q", got)
	}
	payload := wfcovPayload(t, resp)
	if wfcovMapStr(payload, "status") != "failed" {
		t.Errorf("status = %v, want failed", payload["status"])
	}
	if detail := wfcovMapStr(payload, "error_detail"); !strings.Contains(detail, "tab crashed") {
		t.Errorf("error_detail = %q, want the underlying failure text", detail)
	}
}

func TestNavigateAndWaitFor_SelectorTimeoutMarksWorkflowFailed(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "wait_for" {
			return wfcovErr(wfcovReq(), "selector never appeared"), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleNavigateAndWaitFor(wfcovReq(),
		json.RawMessage(`{"url":"https://example.test/","wait_for":"#ready","include_content":true}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("selector timeout must fail the workflow")
	}
	if got := wfcovJoin(wfcovStepNames(wfcovTraceOf(t, resp))); got != "navigate=success,wait_for=error" {
		t.Fatalf("trace = %q", got)
	}
	// include_content must not run once the wait step failed.
	if hs.enrichCount != 0 {
		t.Errorf("EnrichNavigateResponse called %d times after a failed wait, want 0", hs.enrichCount)
	}
}

func TestNavigateAndWaitFor_IncludeContentEnrichesNavigateResponseOnce(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.enrich = func(resp JSONRPCResponse) JSONRPCResponse {
		return succeed(wfcovReq(), "navigate ok", map[string]any{"status": "ok", "content": "hello world"})
	}

	resp := hs.h.HandleNavigateAndWaitFor(wfcovReq(),
		json.RawMessage(`{"url":"https://example.test/","wait_for":"#ready","include_content":true,"tab_id":7}`))

	// The nested navigate handler also honours include_content; the workflow must
	// not pass it down, or the page content would be fetched twice.
	if hs.enrichCount != 1 {
		t.Fatalf("EnrichNavigateResponse called %d times, want exactly 1", hs.enrichCount)
	}
	if hs.enrichTabIDs[0] != 7 {
		t.Errorf("enrich tab_id = %d, want 7", hs.enrichTabIDs[0])
	}
	steps := wfcovTraceOf(t, resp)
	if got := wfcovJoin(wfcovStepNames(steps)); got != "navigate=success,wait_for=success,get_content=success" {
		t.Fatalf("trace = %q", got)
	}
}

func TestNavigateAndWaitFor_TimeoutBudgetFlowsIntoSelectorWait(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		args    string
		wantMin float64
		wantMax float64
	}{
		{"default is 15s", `{"url":"https://e.test/","wait_for":"#a"}`, 14_000, 15_000},
		{"explicit budget is forwarded", `{"url":"https://e.test/","wait_for":"#a","timeout_ms":9000}`, 8_000, 9_000},
		{"tiny budget clamps to 1s floor", `{"url":"https://e.test/","wait_for":"#a","timeout_ms":50}`, 1_000, 1_000},
		{"negative budget falls back to default", `{"url":"https://e.test/","wait_for":"#a","timeout_ms":-5}`, 14_000, 15_000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := wfcovNewHarness(t)
			hs.h.HandleNavigateAndWaitFor(wfcovReq(), json.RawMessage(tc.args))
			wait := hs.wfcovCallAt(1)
			got, ok := wait.wfcovNum("timeout_ms")
			if !ok {
				t.Fatalf("wait_for command carries no timeout_ms: %v", wait.Params)
			}
			if got < tc.wantMin || got > tc.wantMax {
				t.Fatalf("wait timeout_ms = %v, want within [%v,%v]", got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// run_a11y_and_export_sarif (interact_workflow_a11y_sarif.go)
// ---------------------------------------------------------------------------

func TestRunA11yAndExportSARIF_RejectsMalformedArgs(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleRunA11yAndExportSARIF(wfcovReq(), json.RawMessage(`{"scope":}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("malformed JSON must produce an error response")
	}
	if got := wfcovMapStr(wfcovPayload(t, resp), "error_code"); got != ErrInvalidJSON {
		t.Fatalf("error_code = %q, want %q", got, ErrInvalidJSON)
	}
	if len(hs.analyzeArgs) != 0 {
		t.Fatalf("analyze must not run on malformed args, got %d calls", len(hs.analyzeArgs))
	}
}

func TestRunA11yAndExportSARIF_ReusesAuditPayloadForExport(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.analyze = func(json.RawMessage) JSONRPCResponse {
		return succeed(wfcovReq(), "accessibility audit", map[string]any{"violations": []any{"color-contrast"}})
	}

	resp := hs.h.HandleRunA11yAndExportSARIF(wfcovReq(),
		json.RawMessage(`{"scope":"main","save_to":"/tmp/wfcov.sarif","tab_id":3}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("workflow should succeed: %s", string(resp.Result))
	}
	if len(hs.analyzeArgs) != 1 || len(hs.sarifArgs) != 1 {
		t.Fatalf("analyze/sarif call counts = %d/%d, want 1/1", len(hs.analyzeArgs), len(hs.sarifArgs))
	}

	var a11y map[string]any
	if err := json.Unmarshal(hs.analyzeArgs[0], &a11y); err != nil {
		t.Fatalf("analyze args not JSON: %v", err)
	}
	if a11y["what"] != "accessibility" || a11y["scope"] != "main" || a11y["tab_id"] != float64(3) {
		t.Fatalf("analyze args = %v, want what=accessibility scope=main tab_id=3", a11y)
	}

	var sarif map[string]any
	if err := json.Unmarshal(hs.sarifArgs[0], &sarif); err != nil {
		t.Fatalf("sarif args not JSON: %v", err)
	}
	if sarif["scope"] != "main" || sarif["save_to"] != "/tmp/wfcov.sarif" {
		t.Fatalf("sarif args = %v, want scope/save_to forwarded", sarif)
	}
	// The audit payload is threaded into the export so the extension is not
	// queried a second time for the same accessibility results.
	reused, ok := sarif["a11y_result"].(map[string]any)
	if !ok {
		t.Fatalf("sarif args carry no a11y_result: %v", sarif)
	}
	if v, _ := reused["violations"].([]any); len(v) != 1 || v[0] != "color-contrast" {
		t.Fatalf("a11y_result = %v, want the audit violations verbatim", reused)
	}

	if got := wfcovJoin(wfcovStepNames(wfcovTraceOf(t, resp))); got != "analyze_accessibility=success,generate_sarif=success" {
		t.Fatalf("trace = %q", got)
	}
}

func TestRunA11yAndExportSARIF_FailedAuditSkipsExport(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.analyze = func(json.RawMessage) JSONRPCResponse {
		return wfcovErr(wfcovReq(), "accessibility engine unavailable")
	}

	resp := hs.h.HandleRunA11yAndExportSARIF(wfcovReq(), json.RawMessage(`{}`))

	if len(hs.sarifArgs) != 0 {
		t.Fatalf("SARIF export must not run after a failed audit, got %d calls", len(hs.sarifArgs))
	}
	if !wfcovIsError(t, resp) {
		t.Fatal("workflow must report isError")
	}
	if got := wfcovJoin(wfcovStepNames(wfcovTraceOf(t, resp))); got != "analyze_accessibility=error" {
		t.Fatalf("trace = %q", got)
	}
}

func TestRunA11yAndExportSARIF_ExportFailurePreservesAuditStep(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.sarif = func(json.RawMessage) JSONRPCResponse {
		return wfcovErr(wfcovReq(), "cannot write sarif file")
	}

	resp := hs.h.HandleRunA11yAndExportSARIF(wfcovReq(), json.RawMessage(`{}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("export failure must fail the workflow")
	}
	steps := wfcovTraceOf(t, resp)
	if got := wfcovJoin(wfcovStepNames(steps)); got != "analyze_accessibility=success,generate_sarif=error" {
		t.Fatalf("trace = %q", got)
	}
	payload := wfcovPayload(t, resp)
	if payload["successful"] != float64(1) {
		t.Errorf("successful = %v, want 1 (the audit still succeeded)", payload["successful"])
	}
	if detail := wfcovMapStr(payload, "error_detail"); !strings.Contains(detail, "cannot write sarif file") {
		t.Errorf("error_detail = %q, want the export failure text", detail)
	}
}

func TestRunA11yAndExportSARIF_OmitsA11yResultWhenAuditHasNoJSON(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
	}{
		{"plain prose", "accessibility audit finished with no findings"},
		{"trailing garbage after JSON", `summary
{"violations":[]} <-- trailing`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := wfcovNewHarness(t)
			hs.analyze = func(json.RawMessage) JSONRPCResponse {
				result := MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: tc.text}}}
				raw, _ := json.Marshal(result)
				return JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: raw}
			}

			hs.h.HandleRunA11yAndExportSARIF(wfcovReq(), json.RawMessage(`{}`))

			var sarif map[string]any
			if err := json.Unmarshal(hs.sarifArgs[0], &sarif); err != nil {
				t.Fatalf("sarif args not JSON: %v", err)
			}
			if _, present := sarif["a11y_result"]; present {
				t.Fatalf("a11y_result must be omitted when the audit payload is not valid JSON: %v", sarif)
			}
		})
	}
}

func TestExtractMCPResponseJSONPayload_Shapes(t *testing.T) {
	t.Parallel()
	mkResp := func(blocks ...MCPContentBlock) JSONRPCResponse {
		raw, _ := json.Marshal(MCPToolResult{Content: blocks})
		return JSONRPCResponse{JSONRPC: JSONRPCVersion, Result: raw}
	}
	text := func(s string) MCPContentBlock { return MCPContentBlock{Type: "text", Text: s} }

	cases := []struct {
		name string
		resp JSONRPCResponse
		want string
	}{
		{"object after summary line", mkResp(text("summary\n{\"a\":1}")), `{"a":1}`},
		{"array payload", mkResp(text("summary\n[1,2]")), `[1,2]`},
		{"leading prose on same line", mkResp(text("found: {\"a\":1}")), `{"a":1}`},
		{"no braces at all", mkResp(text("nothing here")), ""},
		{"invalid json after brace", mkResp(text("summary\n{oops}")), ""},
		{"no content blocks", mkResp(), ""},
		{"unparseable result", JSONRPCResponse{Result: json.RawMessage(`not-json`)}, ""},
		{"nil result", JSONRPCResponse{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractMCPResponseJSONPayload(tc.resp)
			if string(got) != tc.want {
				t.Fatalf("payload = %q, want %q", string(got), tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// fill_form / fill_form_and_submit (interact_workflow_forms.go)
// ---------------------------------------------------------------------------

func TestFillForm_RejectsEmptyAndMalformedInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     string
		wantCode string
	}{
		{"malformed json", `{"fields":`, ErrInvalidJSON},
		{"fields omitted", `{}`, ErrMissingParam},
		{"fields empty array", `{"fields":[]}`, ErrMissingParam},
		{"fields null", `{"fields":null}`, ErrMissingParam},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := wfcovNewHarness(t)
			resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(tc.args))
			if !wfcovIsError(t, resp) {
				t.Fatal("expected an error response")
			}
			if got := wfcovMapStr(wfcovPayload(t, resp), "error_code"); got != tc.wantCode {
				t.Fatalf("error_code = %q, want %q", got, tc.wantCode)
			}
			if len(hs.wfcovActions()) != 0 {
				t.Fatalf("no command may be dispatched, got %v", hs.wfcovActions())
			}
		})
	}
}

func TestFillForm_TypesEveryFieldWithClearAndTabScope(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(`{
		"fields":[{"selector":"#email","value":"a@b.test"},{"selector":"#pw","value":"hunter2"}],
		"tab_id":11}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("workflow should succeed: %s", string(resp.Result))
	}
	if got := wfcovJoin(hs.wfcovActions()); got != "type,type" {
		t.Fatalf("dispatched actions = %q, want \"type,type\"", got)
	}
	first := hs.wfcovCallAt(0)
	if first.wfcovStr("selector") != "#email" || first.wfcovStr("text") != "a@b.test" {
		t.Errorf("field 0 args = %v", first.Params)
	}
	if first.Params["clear"] != true {
		t.Errorf("clear = %v, want true — fill_form must replace existing values, not append", first.Params["clear"])
	}
	if first.TabID != 11 {
		t.Errorf("field 0 tab_id = %d, want 11", first.TabID)
	}
	if second := hs.wfcovCallAt(1); second.wfcovStr("text") != "hunter2" {
		t.Errorf("field 1 text = %q", second.wfcovStr("text"))
	}

	steps := wfcovTraceOf(t, resp)
	if got := wfcovJoin(wfcovStepNames(steps)); got != "type[0]=success,type[1]=success" {
		t.Fatalf("trace = %q", got)
	}
	if got := wfcovMapStr(steps[1], "detail"); got != "#pw" {
		t.Errorf("step detail = %q, want the field selector", got)
	}
	payload := wfcovPayload(t, resp)
	if payload["steps"] != float64(2) || wfcovMapStr(payload, "workflow") != "fill_form" {
		t.Errorf("payload steps/workflow = %v/%v", payload["steps"], payload["workflow"])
	}
}

func TestFillForm_FieldWithoutSelectorOrIndexAbortsRemainingFields(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(`{"fields":[
		{"selector":"#ok","value":"v"},
		{"value":"orphan"},
		{"selector":"#never","value":"v"}]}`))

	if got := wfcovJoin(hs.wfcovActions()); got != "type" {
		t.Fatalf("dispatched actions = %q, want only the first field", got)
	}
	if !wfcovIsError(t, resp) {
		t.Fatal("workflow must report isError")
	}
	steps := wfcovTraceOf(t, resp)
	if got := wfcovJoin(wfcovStepNames(steps)); got != "type[0]=success,type[1]=error" {
		t.Fatalf("trace = %q", got)
	}
	if got := wfcovMapStr(steps[1], "detail"); got != "Missing selector and index" {
		t.Errorf("bad-field step detail = %q", got)
	}
	if detail := wfcovMapStr(wfcovPayload(t, resp), "error_detail"); !strings.Contains(detail, "Field 1 missing") {
		t.Errorf("error_detail = %q, want the offending field index", detail)
	}
}

func TestFillForm_IndexAddressedFieldResolvesThroughElementIndex(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	// index 0 is a legitimate address; a naive zero-check would treat it as absent.
	hs.h.elementIndexRegistry.store("wfcov-client", 0, "gen_1", map[int]string{0: "#first-input"})

	resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(`{"fields":[{"index":0,"value":"typed"}]}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("index-addressed field must be accepted: %s", string(resp.Result))
	}
	call := hs.wfcovCallAt(0)
	if got, ok := call.wfcovNum("index"); !ok || got != 0 {
		t.Fatalf("index param = %v (present=%v), want 0", got, ok)
	}
	if got := call.wfcovStr("selector"); got != "#first-input" {
		t.Errorf("selector = %q, want the index resolved against the element registry", got)
	}
	if got := wfcovMapStr(wfcovTraceOf(t, resp)[0], "detail"); got != "index:0" {
		t.Errorf("step detail = %q, want \"index:0\"", got)
	}
}

func TestFillForm_UnknownFieldIndexFailsWithRefreshGuidance(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(`{"fields":[{"index":4,"value":"typed"}]}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("an index with no element registry entry must fail")
	}
	if len(hs.wfcovActions()) != 0 {
		t.Fatalf("nothing may be dispatched for an unresolvable index, got %v", hs.wfcovActions())
	}
	detail := wfcovMapStr(wfcovPayload(t, resp), "error_detail")
	if !strings.Contains(detail, "Element index 4 not found") || !strings.Contains(detail, "list_interactive") {
		t.Fatalf("error_detail = %q, want it to name the index and point at list_interactive", detail)
	}
}

func TestFillForm_FallsBackToSelectForNonTypeableElement(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "type" {
			return succeed(wfcovReq(), "type rejected", map[string]any{"error": "not_typeable"}), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(`{"fields":[{"selector":"#country","value":"NZ"}],"tab_id":4}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("select fallback should succeed: %s", string(resp.Result))
	}
	if got := wfcovJoin(hs.wfcovActions()); got != "type,select" {
		t.Fatalf("dispatched actions = %q, want \"type,select\"", got)
	}
	sel := hs.wfcovCallAt(1)
	if sel.wfcovStr("value") != "NZ" || sel.wfcovStr("selector") != "#country" || sel.TabID != 4 {
		t.Fatalf("select command args = %v (tab=%d)", sel.Params, sel.TabID)
	}
	// The trace names the action actually used, so agents can tell a <select> apart.
	if got := wfcovJoin(wfcovStepNames(wfcovTraceOf(t, resp))); got != "select[0]=success" {
		t.Fatalf("trace = %q", got)
	}
}

func TestFillForm_SelectFallbackFailurePropagates(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		switch call.Action {
		case "type":
			return succeed(wfcovReq(), "type rejected", map[string]any{"error": "not_typeable"}), true
		case "select":
			return wfcovErr(wfcovReq(), "no option with that value"), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(`{"fields":[{"selector":"#country","value":"XX"}]}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("a failing select fallback must fail the workflow")
	}
	if got := wfcovJoin(wfcovStepNames(wfcovTraceOf(t, resp))); got != "select[0]=error" {
		t.Fatalf("trace = %q", got)
	}
}

// TestFillForm_EmptyValueIsRejectedByTypeValidation documents current behaviour:
// fill_form cannot be used to blank out a field, because the underlying type
// primitive requires a non-empty 'text'. See final report.
func TestFillForm_EmptyValueIsRejectedByTypeValidation(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(`{"fields":[{"selector":"#note","value":""}]}`))

	if !wfcovIsError(t, resp) {
		t.Fatalf("expected the empty value to be rejected, got: %s", string(resp.Result))
	}
	if len(hs.wfcovActions()) != 0 {
		t.Fatalf("validation happens before dispatch, got %v", hs.wfcovActions())
	}
	if detail := wfcovMapStr(wfcovPayload(t, resp), "error_detail"); !strings.Contains(detail, "'text' is missing") {
		t.Fatalf("error_detail = %q, want the missing-text validation message", detail)
	}
}

func TestFillFormAndSubmit_RequiresASubmitTarget(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleFillFormAndSubmit(wfcovReq(),
		json.RawMessage(`{"fields":[{"selector":"#a","value":"v"}]}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("missing submit target must be rejected")
	}
	payload := wfcovPayload(t, resp)
	if got := wfcovMapStr(payload, "error_code"); got != ErrMissingParam {
		t.Fatalf("error_code = %q, want %q", got, ErrMissingParam)
	}
	if got := wfcovMapStr(payload, "param"); got != "submit_selector" {
		t.Fatalf("param = %q, want submit_selector", got)
	}
	if len(hs.wfcovActions()) != 0 {
		t.Fatalf("fields must not be typed before the submit target is validated, got %v", hs.wfcovActions())
	}
}

func TestFillFormAndSubmit_SubmitIndexZeroIsAValidTarget(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.h.elementIndexRegistry.store("wfcov-client", 0, "gen_1", map[int]string{0: "#submit-btn"})

	resp := hs.h.HandleFillFormAndSubmit(wfcovReq(),
		json.RawMessage(`{"fields":[{"selector":"#a","value":"v"}],"submit_index":0}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("submit_index 0 must be accepted: %s", string(resp.Result))
	}
	if got := wfcovJoin(hs.wfcovActions()); got != "type,click" {
		t.Fatalf("dispatched actions = %q, want \"type,click\"", got)
	}
	click := hs.wfcovCallAt(1)
	if got, ok := click.wfcovNum("index"); !ok || got != 0 {
		t.Fatalf("click index = %v (present=%v), want 0", got, ok)
	}
	if got := click.wfcovStr("selector"); got != "#submit-btn" {
		t.Errorf("submit selector = %q, want the index resolved against the element registry", got)
	}
}

func TestFillFormAndSubmit_ClicksSubmitAfterAllFields(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleFillFormAndSubmit(wfcovReq(), json.RawMessage(`{
		"fields":[{"selector":"#u","value":"bob"},{"selector":"#p","value":"pw"}],
		"submit_selector":"button[type=submit]","tab_id":5}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("workflow should succeed: %s", string(resp.Result))
	}
	if got := wfcovJoin(hs.wfcovActions()); got != "type,type,click" {
		t.Fatalf("dispatched actions = %q", got)
	}
	click := hs.wfcovCallAt(2)
	if click.wfcovStr("selector") != "button[type=submit]" || click.TabID != 5 {
		t.Fatalf("click args = %v (tab=%d)", click.Params, click.TabID)
	}
	steps := wfcovTraceOf(t, resp)
	if got := wfcovJoin(wfcovStepNames(steps)); got != "type[0]=success,type[1]=success,click_submit=success" {
		t.Fatalf("trace = %q", got)
	}
	if got := wfcovMapStr(steps[2], "detail"); got != "button[type=submit]" {
		t.Errorf("submit step detail = %q", got)
	}
}

func TestFillFormAndSubmit_FieldFailureNeverSubmitsTheForm(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.wfcovStr("selector") == "#p" {
			return wfcovErr(wfcovReq(), "element detached"), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleFillFormAndSubmit(wfcovReq(), json.RawMessage(`{
		"fields":[{"selector":"#u","value":"bob"},{"selector":"#p","value":"pw"}],
		"submit_selector":"#go"}`))

	// Submitting a half-filled form is destructive; this must never happen.
	for _, a := range hs.wfcovActions() {
		if a == "click" {
			t.Fatalf("submit was clicked despite a failed field: %v", hs.wfcovActions())
		}
	}
	if !wfcovIsError(t, resp) {
		t.Fatal("workflow must report isError")
	}
	if got := wfcovJoin(wfcovStepNames(wfcovTraceOf(t, resp))); got != "type[0]=success,type[1]=error" {
		t.Fatalf("trace = %q", got)
	}
}

func TestFillFormAndSubmit_SubmitClickFailureIsReported(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "click" {
			return wfcovErr(wfcovReq(), "submit button is disabled"), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleFillFormAndSubmit(wfcovReq(), json.RawMessage(`{
		"fields":[{"selector":"#u","value":"bob"}],"submit_selector":"#go"}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("submit failure must fail the workflow")
	}
	payload := wfcovPayload(t, resp)
	if payload["successful"] != float64(1) || payload["steps"] != float64(2) {
		t.Errorf("successful/steps = %v/%v, want 1/2", payload["successful"], payload["steps"])
	}
	if detail := wfcovMapStr(payload, "error_detail"); !strings.Contains(detail, "submit button is disabled") {
		t.Errorf("error_detail = %q", detail)
	}
}

func TestIsNotTypeableError_OnlyMatchesExtensionPayloadMarker(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		resp JSONRPCResponse
		want bool
	}{
		{"marker in result payload", JSONRPCResponse{Result: json.RawMessage(`{"error":"not_typeable"}`)}, true},
		{"unrelated payload", JSONRPCResponse{Result: json.RawMessage(`{"status":"ok"}`)}, false},
		{"nil result", JSONRPCResponse{}, false},
		{
			"transport error outranks payload text",
			JSONRPCResponse{
				Error:  &mcp.JSONRPCError{Code: -32000, Message: "not_typeable"},
				Result: json.RawMessage(`{"error":"not_typeable"}`),
			},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsNotTypeableError(tc.resp); got != tc.want {
				t.Fatalf("IsNotTypeableError = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// navigate_and_document (interact_workflow_navigate_document.go)
// ---------------------------------------------------------------------------

// wfcovPageInfo builds an observe(what="page") style response.
func wfcovPageInfo(url, title string, tabID int) JSONRPCResponse {
	return succeed(wfcovReq(), "page", map[string]any{"url": url, "title": title, "tab_id": tabID})
}

func TestNavigateAndDocument_RejectsMalformedArgs(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"selector"`))

	if !wfcovIsError(t, resp) {
		t.Fatal("malformed JSON must produce an error response")
	}
	if got := wfcovMapStr(wfcovPayload(t, resp), "error_code"); got != ErrInvalidJSON {
		t.Fatalf("error_code = %q, want %q", got, ErrInvalidJSON)
	}
	if len(hs.wfcovActions()) != 0 {
		t.Fatalf("no command may be dispatched, got %v", hs.wfcovActions())
	}
}

func TestNavigateAndDocument_TabIDMustMatchTheTrackedTab(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		trackedTab int // 0 = tracking never enabled
		argTabID   int
		wantFrag   string
	}{
		{"tracking disabled", 0, 9, "requires an actively tracked tab"},
		{"different tab tracked", 3, 9, "requires tracked tab_id=3; got tab_id=9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := wfcovNewHarness(t)
			if tc.trackedTab > 0 {
				hs.cap.UpdateTrackedTab(tc.trackedTab, "https://tracked.test/", "tracked")
			}
			args, _ := json.Marshal(map[string]any{"selector": "#go", "tab_id": tc.argTabID})

			resp := hs.h.HandleNavigateAndDocument(wfcovReq(), args)

			if !wfcovIsError(t, resp) {
				t.Fatal("tab mismatch must be rejected")
			}
			payload := wfcovPayload(t, resp)
			if got := wfcovMapStr(payload, "message"); !strings.Contains(got, tc.wantFrag) {
				t.Fatalf("message = %q, want it to contain %q", got, tc.wantFrag)
			}
			if got := wfcovMapStr(payload, "param"); got != "tab_id" {
				t.Errorf("param = %q, want tab_id", got)
			}
			if len(hs.wfcovActions()) != 0 {
				t.Fatalf("click must not run when the tab is wrong, got %v", hs.wfcovActions())
			}
			status, stages := wfcovEnvelopeStages(t, resp)
			if status != "failed" || wfcovJoin(stages) != "validate_tab=error" {
				t.Fatalf("envelope status/stages = %q/%q", status, wfcovJoin(stages))
			}
		})
	}
}

func TestNavigateAndDocument_ClickFailureStopsBeforeStabilityWait(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "click" {
			return wfcovErr(wfcovReq(), "no element matches #go"), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"selector":"#go"}`))

	if got := wfcovJoin(hs.wfcovActions()); got != "click" {
		t.Fatalf("dispatched actions = %q, want only \"click\"", got)
	}
	if !wfcovIsError(t, resp) {
		t.Fatal("click failure must surface as an error")
	}
	status, stages := wfcovEnvelopeStages(t, resp)
	if status != "failed" || wfcovJoin(stages) != "validate_tab=success,click=error" {
		t.Fatalf("envelope status/stages = %q/%q", status, wfcovJoin(stages))
	}
}

func TestNavigateAndDocument_MissingSelectorFailsAtClickValidation(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"wait_for_stable":false}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("click without a target must fail")
	}
	payload := wfcovPayload(t, resp)
	if got := wfcovMapStr(payload, "error_code"); got != ErrMissingParam {
		t.Fatalf("error_code = %q, want %q", got, ErrMissingParam)
	}
	if len(hs.wfcovActions()) != 0 {
		t.Fatalf("nothing may be dispatched, got %v", hs.wfcovActions())
	}
}

func TestNavigateAndDocument_PendingClickReturnsForPollingWithoutWaiting(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.cap.UpdateTrackedTab(1, "https://before.test/", "before")
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "click" {
			return wfcovPending(wfcovReq(), call.CorrelationID), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"selector":"#go","timeout_ms":30000}`))

	// A queued click must not be followed by a blocking URL/stability wait —
	// the caller polls the correlation ID instead.
	if got := wfcovJoin(hs.wfcovActions()); got != "click" {
		t.Fatalf("dispatched actions = %q, want only \"click\"", got)
	}
	if wfcovIsError(t, resp) {
		t.Fatalf("a pending click is not an error: %s", string(resp.Result))
	}
	status, stages := wfcovEnvelopeStages(t, resp)
	if status != "pending" {
		t.Fatalf("envelope status = %q, want pending", status)
	}
	if got := wfcovJoin(stages); got != "validate_tab=success,click=success" {
		t.Fatalf("stages = %q", got)
	}
	if payload := wfcovPayload(t, resp); payload["correlation_id"] == nil {
		t.Errorf("pending response must keep the correlation_id for polling: %v", payload)
	}
}

func TestNavigateAndDocument_URLChangeIsDetectedAndRecorded(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.cap.UpdateTrackedTab(1, "https://before.test/page", "before")
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "click" {
			hs.cap.UpdateTrackedTab(1, "https://after.test/next", "after")
		}
		return JSONRPCResponse{}, false
	}
	hs.pageInfo = func() JSONRPCResponse { return wfcovPageInfo("https://after.test/next", "After", 1) }

	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"selector":"#go","timeout_ms":5000}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("workflow should succeed: %s", string(resp.Result))
	}
	if got := wfcovJoin(hs.wfcovActions()); got != "click,wait_for_stable" {
		t.Fatalf("dispatched actions = %q", got)
	}
	status, stages := wfcovEnvelopeStages(t, resp)
	if status != "success" {
		t.Fatalf("envelope status = %q, want success", status)
	}
	want := "validate_tab=success,click=success,wait_for_url_change=success,wait_for_stable=success"
	if got := wfcovJoin(stages); got != want {
		t.Fatalf("stages = %q, want %q", got, want)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	last := result.Content[len(result.Content)-1].Text
	if !strings.Contains(last, "--- Page Context ---") || !strings.Contains(last, "https://after.test/next") {
		t.Fatalf("final content block should carry post-navigation page context, got %q", last)
	}
	pageCtx, ok := result.Metadata["page_context"].(map[string]any)
	if !ok || pageCtx["title"] != "After" {
		t.Fatalf("metadata page_context = %v, want the post-navigation title", result.Metadata)
	}
}

func TestNavigateAndDocument_URLNeverChangesReportsTimeout(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.cap.UpdateTrackedTab(1, "https://static.test/", "static")

	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"selector":"#go","timeout_ms":250}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("an unchanged URL must fail the workflow")
	}
	payload := wfcovPayload(t, resp)
	if got := wfcovMapStr(payload, "error_code"); got != ErrExtTimeout {
		t.Fatalf("error_code = %q, want %q", got, ErrExtTimeout)
	}
	if got := wfcovMapStr(payload, "message"); !strings.Contains(got, "URL did not change") {
		t.Fatalf("message = %q", got)
	}
	if got := wfcovMapStr(payload, "param"); got != "wait_for_url_change" {
		t.Errorf("param = %q, want wait_for_url_change", got)
	}
	// wait_for_stable must not run once the navigation guard failed.
	if got := wfcovJoin(hs.wfcovActions()); got != "click" {
		t.Fatalf("dispatched actions = %q, want only \"click\"", got)
	}
}

func TestNavigateAndDocument_URLWaitSkippedWithoutBaselineURL(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	// No tracked tab and no page info: there is no baseline to compare against.
	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"selector":"#go"}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("a missing baseline URL must not fail the workflow: %s", string(resp.Result))
	}
	_, stages := wfcovEnvelopeStages(t, resp)
	want := "validate_tab=success,click=success,wait_for_url_change=skipped,wait_for_stable=success"
	if got := wfcovJoin(stages); got != want {
		t.Fatalf("stages = %q, want %q", got, want)
	}
}

func TestNavigateAndDocument_BaselineURLFallsBackToPageContext(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	// Capture has no tracked URL, so the baseline comes from observe(page).
	var clicked bool
	hs.pageInfo = func() JSONRPCResponse {
		if clicked {
			return wfcovPageInfo("https://second.test/", "Second", 1)
		}
		return wfcovPageInfo("https://first.test/", "First", 1)
	}
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "click" {
			clicked = true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"selector":"#go","timeout_ms":5000}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("workflow should succeed: %s", string(resp.Result))
	}
	_, stages := wfcovEnvelopeStages(t, resp)
	if !strings.Contains(wfcovJoin(stages), "wait_for_url_change=success") {
		t.Fatalf("stages = %q, want the URL wait to succeed off the page-context baseline", wfcovJoin(stages))
	}
}

func TestNavigateAndDocument_WaitStagesCanBeDisabled(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.cap.UpdateTrackedTab(1, "https://static.test/", "static")

	resp := hs.h.HandleNavigateAndDocument(wfcovReq(),
		json.RawMessage(`{"selector":"#go","wait_for_url_change":false,"wait_for_stable":false}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("workflow should succeed: %s", string(resp.Result))
	}
	if got := wfcovJoin(hs.wfcovActions()); got != "click" {
		t.Fatalf("dispatched actions = %q, want only \"click\"", got)
	}
	_, stages := wfcovEnvelopeStages(t, resp)
	if got := wfcovJoin(stages); got != "validate_tab=success,click=success,wait_for_stable=skipped" {
		t.Fatalf("stages = %q", got)
	}
}

func TestNavigateAndDocument_StabilityDefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		args          string
		wantStability float64
		wantTimeoutHi float64
	}{
		{"defaults injected", `{"selector":"#go","wait_for_url_change":false}`, 500, 5000},
		{"stability override", `{"selector":"#go","wait_for_url_change":false,"stability_ms":1200}`, 1200, 5000},
		{"remaining budget forwarded", `{"selector":"#go","wait_for_url_change":false,"timeout_ms":8000}`, 500, 8000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := wfcovNewHarness(t)
			hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(tc.args))

			wait := hs.wfcovCallAt(1)
			if wait.Action != "wait_for_stable" {
				t.Fatalf("second command = %q, want wait_for_stable", wait.Action)
			}
			if got, _ := wait.wfcovNum("stability_ms"); got != tc.wantStability {
				t.Fatalf("stability_ms = %v, want %v", got, tc.wantStability)
			}
			got, ok := wait.wfcovNum("timeout_ms")
			if !ok {
				t.Fatalf("wait_for_stable carries no timeout_ms: %v", wait.Params)
			}
			if got > tc.wantTimeoutHi || got < tc.wantTimeoutHi-2000 {
				t.Fatalf("timeout_ms = %v, want just under %v", got, tc.wantTimeoutHi)
			}
		})
	}
}

func TestNavigateAndDocument_StabilityFailurePropagates(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "wait_for_stable" {
			return wfcovErr(wfcovReq(), "DOM never settled"), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"selector":"#go"}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("a failed stability wait must fail the workflow")
	}
	status, stages := wfcovEnvelopeStages(t, resp)
	if status != "failed" {
		t.Fatalf("envelope status = %q, want failed", status)
	}
	if !strings.HasSuffix(wfcovJoin(stages), "wait_for_stable=error") {
		t.Fatalf("stages = %q", wfcovJoin(stages))
	}
}

func TestNavigateAndDocument_ExhaustedTimeoutBudgetIsReportedPerStage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		args      string
		wantStage string
	}{
		{"before url wait", `{"selector":"#go","timeout_ms":5}`, "wait_for_url_change"},
		{"before stability wait", `{"selector":"#go","timeout_ms":5,"wait_for_url_change":false}`, "wait_for_stable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := wfcovNewHarness(t)
			hs.cap.UpdateTrackedTab(1, "https://static.test/", "static")
			// Fixture latency: a slow click consumes the whole workflow budget.
			hs.delay["click"] = 40 * time.Millisecond

			resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(tc.args))

			if !wfcovIsError(t, resp) {
				t.Fatal("an exhausted budget must fail the workflow")
			}
			payload := wfcovPayload(t, resp)
			if got := wfcovMapStr(payload, "error_code"); got != ErrExtTimeout {
				t.Fatalf("error_code = %q, want %q", got, ErrExtTimeout)
			}
			wantMsg := "timeout_ms exhausted before " + tc.wantStage + " stage"
			if got := wfcovMapStr(payload, "message"); got != wantMsg {
				t.Fatalf("message = %q, want %q", got, wantMsg)
			}
			if got := wfcovMapStr(payload, "param"); got != "timeout_ms" {
				t.Errorf("param = %q, want timeout_ms", got)
			}
			_, stages := wfcovEnvelopeStages(t, resp)
			if !strings.HasSuffix(wfcovJoin(stages), tc.wantStage+"=error") {
				t.Fatalf("stages = %q, want the failure attributed to %q", wfcovJoin(stages), tc.wantStage)
			}
		})
	}
}

func TestFilterNavigateAndDocumentClickArgs_KeepsOnlyClickFields(t *testing.T) {
	t.Parallel()
	in := json.RawMessage(`{
		"selector":"#go","index":2,"nth":1,"x":10,"y":20,"tab_id":3,"frame":"main",
		"timeout_ms":900,"reason":"navigate","element_id":"e1","index_generation":"g1",
		"scope_selector":"#form","scope_rect":{"x":0},"annotation_rect":{"y":0},
		"stability_ms":1500,"wait_for_stable":true,"wait_for_url_change":false,"bogus":"drop me"}`)

	var got map[string]any
	if err := json.Unmarshal(filterNavigateAndDocumentClickArgs(in), &got); err != nil {
		t.Fatalf("filtered args are not JSON: %v", err)
	}

	for _, key := range []string{"selector", "index", "nth", "x", "y", "tab_id", "frame",
		"timeout_ms", "reason", "element_id", "index_generation", "scope_selector",
		"scope_rect", "annotation_rect"} {
		if _, ok := got[key]; !ok {
			t.Errorf("click arg %q was dropped", key)
		}
	}
	// Workflow-only knobs must not leak into the click payload sent to the page.
	for _, key := range []string{"stability_ms", "wait_for_stable", "wait_for_url_change", "bogus"} {
		if _, ok := got[key]; ok {
			t.Errorf("non-click arg %q leaked into the click payload", key)
		}
	}
	if len(got) != 14 {
		t.Errorf("filtered arg count = %d, want 14", len(got))
	}
}

func TestFilterNavigateAndDocumentClickArgs_PassesThroughUnusableInput(t *testing.T) {
	t.Parallel()
	for _, in := range []string{`not json`, `null`, `[1,2]`} {
		got := filterNavigateAndDocumentClickArgs(json.RawMessage(in))
		if string(got) != in {
			t.Errorf("input %q was rewritten to %q; unusable input must pass through untouched", in, string(got))
		}
	}
}

func TestRemainingNavigateAndDocumentTimeoutMs_Budget(t *testing.T) {
	t.Parallel()
	if _, ok := remainingNavigateAndDocumentTimeoutMs(time.Now(), 0); ok {
		t.Error("a zero total budget must report exhausted")
	}
	if _, ok := remainingNavigateAndDocumentTimeoutMs(time.Now(), -1); ok {
		t.Error("a negative total budget must report exhausted")
	}
	if _, ok := remainingNavigateAndDocumentTimeoutMs(time.Now().Add(-2*time.Second), 1000); ok {
		t.Error("a budget already overrun must report exhausted")
	}
	got, ok := remainingNavigateAndDocumentTimeoutMs(time.Now(), 5000)
	if !ok || got > 5000 || got < 4000 {
		t.Errorf("remaining = %d (ok=%v), want just under 5000", got, ok)
	}
}

func TestValidateNavigateAndDocumentTab_UntargetedCallSkipsTrackingCheck(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	for _, tabID := range []int{0, -1} {
		if _, blocked := hs.h.validateNavigateAndDocumentTab(wfcovReq(), tabID); blocked {
			t.Errorf("tab_id=%d must not require an active tracked tab", tabID)
		}
	}
	hs.cap.UpdateTrackedTab(8, "https://x.test/", "x")
	if _, blocked := hs.h.validateNavigateAndDocumentTab(wfcovReq(), 8); blocked {
		t.Error("a tab_id equal to the tracked tab must be allowed")
	}
}

func TestFillFormAndSubmit_RejectsEmptyAndMalformedInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     string
		wantCode string
	}{
		{"malformed json", `{"fields":[}`, ErrInvalidJSON},
		{"fields empty", `{"fields":[],"submit_selector":"#go"}`, ErrMissingParam},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hs := wfcovNewHarness(t)
			resp := hs.h.HandleFillFormAndSubmit(wfcovReq(), json.RawMessage(tc.args))
			if !wfcovIsError(t, resp) {
				t.Fatal("expected an error response")
			}
			if got := wfcovMapStr(wfcovPayload(t, resp), "error_code"); got != tc.wantCode {
				t.Fatalf("error_code = %q, want %q", got, tc.wantCode)
			}
			if len(hs.wfcovActions()) != 0 {
				t.Fatalf("no command may be dispatched, got %v", hs.wfcovActions())
			}
		})
	}
}

func TestFillForm_SelectFallbackKeepsIndexAddressing(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.h.elementIndexRegistry.store("wfcov-client", 0, "gen_1", map[int]string{2: "#country"})
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "type" {
			return succeed(wfcovReq(), "type rejected", map[string]any{"error": "not_typeable"}), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(`{"fields":[{"index":2,"value":"NZ"}]}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("select fallback should succeed: %s", string(resp.Result))
	}
	sel := hs.wfcovCallAt(1)
	if sel.Action != "select" {
		t.Fatalf("second command = %q, want select", sel.Action)
	}
	// The fallback must re-address the same element, not fall back to a bare selector-less call.
	if got, ok := sel.wfcovNum("index"); !ok || got != 2 {
		t.Fatalf("select index = %v (present=%v), want 2", got, ok)
	}
	if got := sel.wfcovStr("value"); got != "NZ" {
		t.Errorf("select value = %q, want NZ", got)
	}
}

func TestNavigateAndDocument_OmittedTimeoutStillWaitsForURLChange(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.cap.UpdateTrackedTab(1, "https://before.test/", "before")
	hs.reply = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.Action == "click" {
			hs.cap.UpdateTrackedTab(1, "https://after.test/", "after")
		}
		return JSONRPCResponse{}, false
	}

	// timeout_ms omitted: the URL wait falls back to its own default budget and
	// no per-stage budget is forwarded to wait_for_stable.
	resp := hs.h.HandleNavigateAndDocument(wfcovReq(), json.RawMessage(`{"selector":"#go"}`))

	if wfcovIsError(t, resp) {
		t.Fatalf("workflow should succeed: %s", string(resp.Result))
	}
	_, stages := wfcovEnvelopeStages(t, resp)
	want := "validate_tab=success,click=success,wait_for_url_change=success,wait_for_stable=success"
	if got := wfcovJoin(stages); got != want {
		t.Fatalf("stages = %q, want %q", got, want)
	}
	if got, _ := hs.wfcovCallAt(1).wfcovNum("timeout_ms"); got != 5000 {
		t.Errorf("wait_for_stable timeout_ms = %v, want the primitive default 5000", got)
	}
}

func TestFillForm_QueueRejectionStopsWorkflowAndSkipsReproductionRecord(t *testing.T) {
	t.Parallel()
	hs := wfcovNewHarness(t)
	hs.enqueueBlock = func(call wfcovCall) (JSONRPCResponse, bool) {
		if call.wfcovStr("selector") == "#second" {
			return fail(wfcovReq(), ErrQueueFull, "command queue is full", "retry later"), true
		}
		return JSONRPCResponse{}, false
	}

	resp := hs.h.HandleFillForm(wfcovReq(), json.RawMessage(`{"fields":[
		{"selector":"#first","value":"a"},
		{"selector":"#second","value":"b"},
		{"selector":"#third","value":"c"}]}`))

	if !wfcovIsError(t, resp) {
		t.Fatal("a rejected enqueue must fail the workflow")
	}
	if got := wfcovJoin(hs.wfcovActions()); got != "type,type" {
		t.Fatalf("enqueue attempts = %q, want the workflow to stop after the rejection", got)
	}
	if got := wfcovJoin(wfcovStepNames(wfcovTraceOf(t, resp))); got != "type[0]=success,type[1]=error" {
		t.Fatalf("trace = %q", got)
	}
	// A command that never reached the queue must not be replayed by the
	// reproduction recorder.
	if len(hs.domRecords) != 1 || hs.domRecords[0] != "type|#first|a|" {
		t.Fatalf("recorded DOM primitives = %v, want only the accepted field", hs.domRecords)
	}
}

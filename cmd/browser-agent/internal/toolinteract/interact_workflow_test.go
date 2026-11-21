// interact_workflow_test.go — Tests for form, navigate, a11y workflows and page/screenshot response enrichment.
package toolinteract

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/a11yaudit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/pageinfo"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/screenshot"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestHandleFillForm_Success(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	args := `{"fields":[{"selector":"#name","value":"Ada"},{"selector":"#email","value":"a@b.co"}]}`
	assertOK(t, h.HandleFillForm(testReq(), json.RawMessage(args)))
}

func TestHandleFillForm_EmptyFields(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleFillForm(testReq(), json.RawMessage(`{"fields":[]}`)), mcp.ErrMissingParam)
}

func TestHandleFillForm_InvalidJSON(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleFillForm(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestHandleFillForm_FieldMissingSelectorAndIndex(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleFillForm(testReq(), json.RawMessage(`{"fields":[{"value":"x"}]}`)), mcp.ErrMissingParam)
}

func TestHandleFillForm_ByIndex_NotInRegistry(t *testing.T) {
	// Without a prior list_interactive to populate the element-index registry,
	// resolving field index 0 returns an invalid_param "call list_interactive first" error.
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleFillForm(testReq(), json.RawMessage(`{"fields":[{"index":0,"value":"x"}]}`)), mcp.ErrInvalidParam)
}

func TestHandleFillFormAndSubmit_Success(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	args := `{"fields":[{"selector":"#name","value":"Ada"}],"submit_selector":"#go"}`
	assertOK(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(args)))
}

func TestHandleFillFormAndSubmit_BySubmitIndex_NotInRegistry(t *testing.T) {
	// submit_index requires a populated element-index registry (list_interactive first);
	// without it, the submit step returns an invalid_param error.
	h, _ := newFakeWorkflowActions(t)
	args := `{"fields":[{"selector":"#name","value":"Ada"}],"submit_index":2}`
	assertErr(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(args)), mcp.ErrInvalidParam)
}

func TestHandleFillFormAndSubmit_MissingSubmit(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	args := `{"fields":[{"selector":"#name","value":"Ada"}]}`
	assertErr(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(args)), mcp.ErrMissingParam)
}

func TestHandleFillFormAndSubmit_EmptyFields(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(`{"fields":[],"submit_selector":"#go"}`)), mcp.ErrMissingParam)
}

func TestHandleFillFormAndSubmit_InvalidJSON(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestHandleFillForm_SelectFallback(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	// First type returns not_typeable, forcing a select fallback.
	call := 0
	fs.waitFn = func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
		call++
		if call == 1 {
			return mcp.Succeed(req, "type", map[string]any{"status": "complete", "error": "not_typeable"})
		}
		return mcp.Succeed(req, queuedSummary, map[string]any{"status": "complete"})
	}
	// Note: not_typeable is detected via response body substring, not IsError.
	resp := h.HandleFillForm(testReq(), json.RawMessage(`{"fields":[{"selector":"#s","value":"opt"}]}`))
	assertOK(t, resp)
}

func TestHandleNavigateAndWaitFor_Success(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	args := `{"url":"https://example.org","wait_for":"#ready"}`
	assertOK(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(args)))
}

func TestHandleNavigateAndWaitFor_MissingURL(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(`{"wait_for":"#x"}`)), mcp.ErrMissingParam)
}

func TestHandleNavigateAndWaitFor_MissingWaitFor(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(`{"url":"https://x.io"}`)), mcp.ErrMissingParam)
}

func TestHandleNavigateAndWaitFor_InvalidJSON(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestHandleNavigateAndWaitFor_IncludeContent(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	args := `{"url":"https://example.org","wait_for":"#ready","include_content":true}`
	assertOK(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(args)))
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) < 3 || enqueued[len(enqueued)-1].Type != "page_summary" {
		t.Fatalf("enqueued = %#v, want final page_summary enrichment", enqueued)
	}
}

func TestHandleNavigateAndDocument_WaitsDisabled(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	args := `{"selector":"#link","wait_for_url_change":false,"wait_for_stable":false}`
	assertOK(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(args)))
}

func TestHandleNavigateAndDocument_InvalidJSON(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestHandleNavigateAndDocument_TabMismatch(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	// tracked tab is 1; request tab_id 2 should mismatch.
	capturefixture.Track(fs.cap, 1, "https://example.com/page")
	args := `{"selector":"#link","tab_id":2,"wait_for_url_change":false,"wait_for_stable":false}`
	assertErr(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(args)), mcp.ErrInvalidParam)
}

func TestHandleNavigateAndDocument_ClickError(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	fs.waitFn = func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
		return mcp.Fail(req, mcp.ErrExtError, "click failed", "retry")
	}
	args := `{"selector":"#link","wait_for_url_change":false,"wait_for_stable":false}`
	assertErr(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(args)), mcp.ErrExtError)
}

func TestHandleNavigateAndDocument_NoResultAfterNavigationContinues(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	capturefixture.Track(fs.cap, 1, "https://example.test/before")
	noResult := mcp.Fail(testReq(), mcp.ErrExtError, "Command failed: no_result", "retry")
	if !clickLostToNavigation(noResult) {
		t.Fatalf("clickLostToNavigation did not classify response: %+v", noResult)
	}
	fs.waitFn = func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
		return noResult
	}
	fs.waitTrackedURLFn = func(beforeURL string, timeout time.Duration) (string, bool) {
		if beforeURL != "https://example.test/before" || timeout != 5*time.Second {
			t.Fatalf("wait args = %q, %s", beforeURL, timeout)
		}
		capturefixture.Track(fs.cap, 1, "https://example.test/after")
		return "https://example.test/after", true
	}

	args := `{"selector":"#link","wait_for_url_change":true,"wait_for_stable":false}`
	assertOK(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(args)))
}

func TestHandleNavigateAndDocument_URLChangeTimeoutUsesOwnerBoundary(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	capturefixture.Track(fs.cap, 1, "https://example.test/before")
	waits := 0
	fs.waitTrackedURLFn = func(beforeURL string, timeout time.Duration) (string, bool) {
		waits++
		if beforeURL != "https://example.test/before" || timeout != 75*time.Millisecond {
			t.Fatalf("wait args = %q, %s", beforeURL, timeout)
		}
		return beforeURL, false
	}

	resp := h.HandleNavigateAndDocument(testReq(), json.RawMessage(
		`{"selector":"#link","timeout_ms":75,"wait_for_url_change":true,"wait_for_stable":false}`,
	))
	assertErr(t, resp, mcp.ErrExtTimeout)
	if waits != 1 {
		t.Fatalf("tracking waits = %d, want 1", waits)
	}
}

func TestHandleNavigateAndDocument_TabIDRequiresTracking(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	resp := h.HandleNavigateAndDocument(testReq(), json.RawMessage(
		`{"selector":"#link","tab_id":42,"wait_for_url_change":false,"wait_for_stable":false}`,
	))
	assertErr(t, resp, mcp.ErrInvalidParam)
}

func TestNavigateAndDocumentWaitStable_SuccessAppendsTrace(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	trace := make([]act.WorkflowStep, 0, 1)
	resp, stop := h.navigateAndDocumentWaitStable(testReq(), navigateAndDocumentParams{StabilityMs: 250}, time.Now(), &trace)
	if stop {
		t.Fatalf("expected success, got error: %s", resp.Result)
	}
	if len(trace) != 1 || trace[0].Action != "wait_for_stable" || trace[0].Status != "success" {
		t.Fatalf("trace = %#v", trace)
	}
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) != 1 || enqueued[0].Type != "dom_action" {
		t.Fatalf("enqueued = %#v", enqueued)
	}
	if !strings.Contains(string(enqueued[0].Params), `"stability_ms":250`) {
		t.Fatalf("wait params = %s", enqueued[0].Params)
	}
}

func TestNavigateAndDocumentWaitStable_NoTimeoutSkipsBudgetCheck(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	// TimeoutMs == 0 must skip the remaining-budget calculation entirely,
	// even with a workflowStart far in the past.
	trace := make([]act.WorkflowStep, 0, 1)
	resp, stop := h.navigateAndDocumentWaitStable(testReq(), navigateAndDocumentParams{StabilityMs: 100}, time.Now().Add(-time.Hour), &trace)
	if stop {
		t.Fatalf("zero timeout must not trip the budget check: %s", resp.Result)
	}
	if len(trace) != 1 || trace[0].Status != "success" {
		t.Fatalf("trace = %#v", trace)
	}
}

func TestNavigateAndDocumentWaitStable_TimeoutBudgetExceeded(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	trace := make([]act.WorkflowStep, 0, 1)
	resp, stop := h.navigateAndDocumentWaitStable(testReq(),
		navigateAndDocumentParams{TimeoutMs: 5000, StabilityMs: 250},
		time.Now().Add(-time.Minute), &trace)
	if !stop {
		t.Fatal("exhausted budget must stop the workflow")
	}
	assertErr(t, resp, mcp.ErrExtTimeout)
	if len(trace) != 1 || trace[0].Status != "error" || trace[0].Action != "wait_for_stable" {
		t.Fatalf("trace = %#v", trace)
	}
}

func TestNavigateAndDocumentWaitStable_ErrorResponseStopsWorkflow(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	fs.waitFn = func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
		return mcp.Fail(req, mcp.ErrExtTimeout, "stability timed out", "increase stability_ms")
	}
	trace := make([]act.WorkflowStep, 0, 1)
	resp, stop := h.navigateAndDocumentWaitStable(testReq(), navigateAndDocumentParams{StabilityMs: 250}, time.Now(), &trace)
	if !stop {
		t.Fatal("wait_for_stable failure must stop the workflow")
	}
	assertErr(t, resp, mcp.ErrExtTimeout)
	if len(trace) != 1 || trace[0].Status != "error" {
		t.Fatalf("trace = %#v", trace)
	}
}

func TestHandleNavigateAndDocument_WaitStableEnabled(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	args := `{"selector":"#link","wait_for_url_change":false,"wait_for_stable":true}`
	assertOK(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(args)))
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) != 2 || enqueued[1].Type != "dom_action" {
		t.Fatalf("enqueued = %#v, want click then wait_for_stable", enqueued)
	}
	if !strings.Contains(string(enqueued[1].Params), `"action":"wait_for_stable"`) {
		t.Fatalf("wait_for_stable params = %s", enqueued[1].Params)
	}
}

func TestNavigateAndDocumentTimeoutBudgetExceededNamesStage(t *testing.T) {
	resp := navigateAndDocumentTimeoutBudgetExceeded(testReq(), "wait_for_stable")
	result := parseToolResult(t, resp)
	if !result.IsError || !contains(firstText(result), "wait_for_stable stage") {
		t.Fatalf("budget response = %s", firstText(result))
	}
}

func TestHandleRunA11yAndExportSARIF_Success(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertOK(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{"scope":"page"}`)))
}

func TestHandleRunA11yAndExportSARIFReusesSingleAuditPayload(t *testing.T) {
	h, state := newFakeWorkflowActions(t)
	auditCalls := 0
	state.auditFn = func(scope string) (a11yaudit.Result, error) {
		auditCalls++
		if scope != "body" {
			t.Fatalf("audit scope = %q", scope)
		}
		return a11yaudit.Result{Data: map[string]any{"violations": []any{map[string]any{"id": "color-contrast"}}}}, nil
	}
	state.sarifFn = func(a11yResult json.RawMessage, scope, saveTo string) (map[string]any, error) {
		if scope != "body" || saveTo != "/tmp/audit.sarif" || !strings.Contains(string(a11yResult), "color-contrast") {
			t.Fatalf("SARIF input = %s scope=%q saveTo=%q", a11yResult, scope, saveTo)
		}
		return map[string]any{"status": "exported"}, nil
	}
	assertOK(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{"scope":"body","save_to":"/tmp/audit.sarif","tab_id":42}`)))
	if auditCalls != 1 {
		t.Fatalf("audit calls = %d, want 1", auditCalls)
	}
}

func TestHandleRunA11yAndExportSARIF_A11yError(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	fs.auditFn = func(string) (a11yaudit.Result, error) { return a11yaudit.Result{}, errors.New("audit failed") }
	assertErr(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{}`)), mcp.ErrExtError)
}

func TestHandleRunA11yAndExportSARIF_SARIFError(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	fs.sarifFn = func(json.RawMessage, string, string) (map[string]any, error) {
		return nil, errors.New("SARIF export failed: bad")
	}
	assertErr(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{}`)), mcp.ErrNoData)
}

func TestHandleRunA11yAndExportSARIF_InvalidJSON(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestWorkflowFieldLabel(t *testing.T) {
	idx := 3
	if workflowFieldLabel(act.FormField{Index: &idx}) != "index:3" {
		t.Fatal("expected index label")
	}
	if workflowFieldLabel(act.FormField{Selector: "#x"}) != "#x" {
		t.Fatal("expected selector label")
	}
}

func TestIsNotTypeableError(t *testing.T) {
	yes := mcp.JSONRPCResponse{Result: json.RawMessage(`{"content":[{"type":"text","text":"not_typeable"}]}`)}
	if !IsNotTypeableError(yes) {
		t.Fatal("expected not_typeable detected")
	}
	no := mcp.Succeed(testReq(), "ok", map[string]any{"status": "complete"})
	if IsNotTypeableError(no) {
		t.Fatal("expected no not_typeable for clean response")
	}
}

func TestFilterNavigateAndDocumentClickArgs(t *testing.T) {
	out := filterNavigateAndDocumentClickArgs(json.RawMessage(`{"selector":"#a","stability_ms":5,"x":1}`))
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if _, ok := m["selector"]; !ok {
		t.Fatal("expected selector retained")
	}
	if _, ok := m["stability_ms"]; ok {
		t.Fatal("expected stability_ms filtered out")
	}
}

func TestRemainingNavigateAndDocumentTimeoutMs(t *testing.T) {
	now := time.Unix(100, 0)
	if _, ok := remainingNavigateAndDocumentTimeoutMs(now, 0, now); ok {
		t.Fatal("expected not-ok for zero total")
	}
	if ms, ok := remainingNavigateAndDocumentTimeoutMs(now, 5000, now.Add(time.Second)); !ok || ms != 4000 {
		t.Fatalf("expected positive remaining, got %d ok=%v", ms, ok)
	}
}

func TestBuildRetryEvidenceSummaryPrefersEffectiveURL(t *testing.T) {
	retryContext := map[string]any{"attempt": 2, "terminal_stop": true}
	evidence := map[string]any{"screenshot": "shot.png"}
	summary := buildRetryEvidenceSummary(
		"corr-7",
		"element_not_found",
		retryContext,
		map[string]any{
			"evidence":      evidence,
			"effective_url": "https://effective.example.test",
			"resolved_url":  "https://resolved.example.test",
		},
	)
	if summary["correlation_id"] != "corr-7" ||
		summary["failure_reason"] != "element_not_found" ||
		summary["url"] != "https://effective.example.test" {
		t.Fatalf("summary = %#v", summary)
	}
	if summary["captured_evidence"] == nil || summary["retry_context"] == nil {
		t.Fatalf("summary omitted evidence: %#v", summary)
	}

	resolved := buildRetryEvidenceSummary("", "", nil, map[string]any{
		"effective_url": " ",
		"resolved_url":  "https://resolved.example.test",
	})
	if resolved["url"] != "https://resolved.example.test" {
		t.Fatalf("resolved fallback = %#v", resolved)
	}
}

func TestAppendScreenshotToResponseAddsFirstImageBlock(t *testing.T) {
	h, fs := newFakePageActions(t)
	fs.screenshot = func(screenshot.Options) (screenshot.Result, error) {
		return screenshot.Result{Fields: map[string]any{"path": "/s.jpg"}, DataURL: "data:image/jpeg;base64,ZmFrZS1pbWFnZQ=="}, nil
	}

	got := h.AppendScreenshotToResponse(mcp.SucceedText(testReq(), "clicked"), testReq())
	result := parseToolResult(t, got)
	if len(result.Content) != 2 || result.Content[1].Type != "image" ||
		result.Content[1].Data != "ZmFrZS1pbWFnZQ==" || result.Content[1].MimeType != "image/jpeg" {
		t.Fatalf("enriched screenshot result = %#v", result.Content)
	}
}

func TestAppendScreenshotToResponseKeepsBaseWithoutUsableImage(t *testing.T) {
	h, fs := newFakePageActions(t)
	base := mcp.SucceedText(testReq(), "clicked")
	for _, outcome := range []struct {
		result screenshot.Result
		err    error
	}{
		{err: &screenshot.Failure{Kind: screenshot.KindTimeout, Detail: "slow"}},
		{result: screenshot.Result{Fields: map[string]any{"path": "/s.jpg"}}},
		{result: screenshot.Result{Fields: map[string]any{}, DataURL: "not-a-data-url"}},
	} {
		fs.screenshot = func(screenshot.Options) (screenshot.Result, error) { return outcome.result, outcome.err }
		got := h.AppendScreenshotToResponse(base, testReq())
		if string(got.Result) != string(base.Result) {
			t.Fatalf("unusable screenshot changed base: %s", got.Result)
		}
	}
}

func trackingOf(store *capture.Capture) func() bool {
	return func() bool {
		enabled, _, _ := store.Extension().GetTrackingStatus()
		return enabled
	}
}

// diagnosticRecorder is a statediag.Reporter/Resolver fake.
type diagnosticRecorder struct {
	mu       sync.Mutex
	reported []statediag.Diagnostic
	resolved []string
}

func (r *diagnosticRecorder) Report(d statediag.Diagnostic) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reported = append(r.reported, d)
}

func (r *diagnosticRecorder) Resolve(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolved = append(r.resolved, name)
}

func TestAppendScreenshotFailureReportsDiagnosticAndKeepsAction(t *testing.T) {
	h, fs := newFakePageActions(t)
	base := mcp.SucceedText(testReq(), "clicked")
	fs.screenshot = func(screenshot.Options) (screenshot.Result, error) {
		return screenshot.Result{}, &screenshot.Failure{Kind: screenshot.KindTimeout, Detail: "slow"}
	}
	if got := h.AppendScreenshotToResponse(base, testReq()); string(got.Result) != string(base.Result) {
		t.Fatalf("failed screenshot changed the successful action response: %s", got.Result)
	}
	if len(fs.diagnostics.reported) != 1 {
		t.Fatalf("reported diagnostics = %+v, want one", fs.diagnostics.reported)
	}
	reported := fs.diagnostics.reported[0]
	if reported.Name != includeScreenshotDiagnostic || !strings.Contains(reported.Detail, "timeout") || reported.Fix == "" {
		t.Fatalf("diagnostic = %+v", reported)
	}

	fs.screenshot = nil
	h.AppendScreenshotToResponse(base, testReq())
	if len(fs.diagnostics.resolved) != 1 || fs.diagnostics.resolved[0] != includeScreenshotDiagnostic {
		t.Fatalf("resolved = %v, want the diagnostic resolved after a successful capture", fs.diagnostics.resolved)
	}
}

func TestAppendPageContextUsesTypedPageInfo(t *testing.T) {
	h, fs := newFakePageActions(t)
	base := mcp.SucceedText(testReq(), "clicked")
	fs.pageInfo = func() pageinfo.Info { return pageinfo.Info{} }
	if got := h.AppendPageContextToResponse(base, testReq()); string(got.Result) != string(base.Result) {
		t.Fatalf("unknown page changed the response: %s", got.Result)
	}
	fs.pageInfo = func() pageinfo.Info { return pageinfo.Info{URL: "https://typed.test", TabID: 9} }
	result := parseToolResult(t, h.AppendPageContextToResponse(base, testReq()))
	last := result.Content[len(result.Content)-1].Text
	if !strings.Contains(last, `"url":"https://typed.test"`) || !strings.Contains(last, `"tab_id":9`) || strings.Contains(last, "title") {
		t.Fatalf("page context block = %q", last)
	}
}

// interact_workflow_test.go — Tests for form, navigate, and a11y workflow handlers.
package toolinteract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
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

func TestHandleRunA11yAndExportSARIF_Success(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertOK(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{"scope":"page"}`)))
}

func TestHandleRunA11yAndExportSARIFReusesSingleAnalyzePayload(t *testing.T) {
	h, state := newFakeWorkflowActions(t)
	analyzeCalls := 0
	state.analyzeFn = func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		analyzeCalls++
		var input map[string]any
		if err := json.Unmarshal(args, &input); err != nil {
			t.Fatalf("decode analyze args: %v", err)
		}
		if input["what"] != "accessibility" || input["scope"] != "body" || input["tab_id"] != float64(42) {
			t.Fatalf("analyze args = %#v", input)
		}
		return mcp.Succeed(req, "audit", map[string]any{"violations": []any{map[string]any{"id": "color-contrast"}}})
	}
	state.sarifFn = func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		var input struct {
			Scope      string          `json:"scope"`
			SaveTo     string          `json:"save_to"`
			A11yResult json.RawMessage `json:"a11y_result"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			t.Fatalf("decode SARIF args: %v", err)
		}
		if input.Scope != "body" || input.SaveTo != "/tmp/audit.sarif" || !strings.Contains(string(input.A11yResult), "color-contrast") {
			t.Fatalf("SARIF args = %#v", input)
		}
		return mcp.Succeed(req, "sarif", map[string]any{"status": "exported"})
	}
	assertOK(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{"scope":"body","save_to":"/tmp/audit.sarif","tab_id":42}`)))
	if analyzeCalls != 1 {
		t.Fatalf("analyze calls = %d, want 1", analyzeCalls)
	}
}

func TestHandleRunA11yAndExportSARIF_A11yError(t *testing.T) {
	h, fs := newFakeWorkflowActions(t)
	fs.analyzeFn = func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return mcp.Fail(req, mcp.ErrExtError, "audit failed", "retry")
	}
	assertErr(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{}`)), mcp.ErrExtError)
}

func TestHandleRunA11yAndExportSARIF_InvalidJSON(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertErr(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestExtractMCPResponseJSONPayload(t *testing.T) {
	resp := mcp.Succeed(testReq(), "summary", map[string]any{"k": "v"})
	payload := extractMCPResponseJSONPayload(resp)
	if payload == nil || !contains(string(payload), "\"k\":\"v\"") {
		t.Fatalf("expected extracted payload, got %s", payload)
	}
	// Non-JSON content returns nil.
	plain := mcp.JSONRPCResponse{Result: json.RawMessage(`{"content":[{"type":"text","text":"no json here"}]}`)}
	if extractMCPResponseJSONPayload(plain) != nil {
		t.Fatal("expected nil for text without JSON")
	}
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

// Element listings were sized for completeness rather than for a model's
// context budget: ~279 bytes per element, against ~22 for the equivalent line
// from chrome-devtools-mcp. Most of the difference is data no caller reads when
// it is choosing what to click — a bounding box, a landmark tag, an overlay
// flag. Lean by default, everything on request.
func TestProjectInteractiveElementsIsLeanByDefault(t *testing.T) {
	full := map[string]any{
		"elements": []any{
			map[string]any{
				"bbox":          map[string]any{"x": 20.0, "y": 10.0, "width": 94.0, "height": 28.0},
				"element_id":    "el_1",
				"element_type":  "link",
				"in_overlay":    true,
				"index":         0.0,
				"label":         "Kaboom",
				"landmark_tag":  "header",
				"landmark_role": "banner",
				"selector":      "text=Kaboom:nth-match(1)",
				"tag":           "a",
				"visible":       true,
			},
		},
	}

	lean := projectElementCollections(full, false)
	elements, _ := lean["elements"].([]any)
	if len(elements) != 1 {
		t.Fatalf("projection dropped the collection: %+v", lean)
	}
	got, _ := elements[0].(map[string]any)

	for _, keep := range []string{"element_id", "element_type", "label", "selector", "index"} {
		if _, ok := got[keep]; !ok {
			t.Errorf("lean projection must keep %q — an agent targets with it", keep)
		}
	}
	for _, drop := range []string{"bbox", "landmark_tag", "landmark_role", "in_overlay", "tag"} {
		if _, ok := got[drop]; ok {
			t.Errorf("lean projection must drop %q by default", drop)
		}
	}
	// visible=true is the overwhelmingly common case and says nothing; only the
	// exception is worth bytes.
	if _, ok := got["visible"]; ok {
		t.Error("lean projection must omit visible when the element is visible")
	}
}

func TestProjectInteractiveElementsKeepsEverythingWhenVerbose(t *testing.T) {
	full := map[string]any{
		"elements": []any{
			map[string]any{"element_id": "el_1", "bbox": map[string]any{"x": 1.0}, "landmark_tag": "header", "visible": true},
		},
	}
	verbose := projectElementCollections(full, true)
	got := verbose["elements"].([]any)[0].(map[string]any)
	for _, keep := range []string{"element_id", "bbox", "landmark_tag", "visible"} {
		if _, ok := got[keep]; !ok {
			t.Errorf("verbose must keep %q", keep)
		}
	}
}

// A hidden element is the exception the caller needs to know about.
func TestProjectInteractiveElementsKeepsVisibleWhenFalse(t *testing.T) {
	full := map[string]any{
		"elements": []any{map[string]any{"element_id": "el_1", "visible": false}},
	}
	got := projectElementCollections(full, false)["elements"].([]any)[0].(map[string]any)
	if visible, ok := got["visible"].(bool); !ok || visible {
		t.Fatalf("visible=false must survive the lean projection, got %+v", got)
	}
}

// explore_page names its collection differently, and its menu enrichment runs
// before the projection and depends on bbox — so the projection must apply to
// both collections and must not run until enrichment is done.
func TestProjectInteractiveElementsCoversExploreCollection(t *testing.T) {
	full := map[string]any{
		"interactive_elements": []any{
			map[string]any{"element_id": "el_9", "bbox": map[string]any{"x": 1.0}, "label": "Go"},
		},
	}
	got := projectElementCollections(full, false)["interactive_elements"].([]any)[0].(map[string]any)
	if _, ok := got["bbox"]; ok {
		t.Error("explore_page's interactive_elements must be projected too")
	}
	if got["label"] != "Go" {
		t.Errorf("label must survive, got %+v", got)
	}
}

// The real response is a lifecycle envelope with the payload under result, so a
// projection that only walks the top level does nothing in production while
// passing against a flat fixture. It did exactly that until a live measurement
// showed 10,622 bytes unchanged.
func TestProjectInteractiveElementsReachesIntoTheLifecycleEnvelope(t *testing.T) {
	enveloped := map[string]any{
		"correlation_id":   "dom_list_1",
		"lifecycle_status": "complete",
		"result": map[string]any{
			"elements": []any{
				map[string]any{"element_id": "el_1", "label": "Kaboom", "bbox": map[string]any{"x": 20.0}, "tag": "a"},
			},
		},
	}
	got := projectElementCollections(enveloped, false)
	inner := got["result"].(map[string]any)["elements"].([]any)[0].(map[string]any)
	if _, ok := inner["bbox"]; ok {
		t.Error("the projection must reach into result, where every real payload lives")
	}
	if inner["label"] != "Kaboom" {
		t.Errorf("label must survive inside the envelope, got %+v", inner)
	}
}

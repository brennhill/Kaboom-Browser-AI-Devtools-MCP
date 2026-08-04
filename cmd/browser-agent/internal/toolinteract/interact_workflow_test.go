// interact_workflow_test.go — Tests for form, navigate, and a11y workflow handlers.
package toolinteract

import (
	"encoding/json"
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
		capturefixture.Track(fs.cap, 1, "https://example.test/after")
		return noResult
	}

	args := `{"selector":"#link","wait_for_url_change":true,"wait_for_stable":false}`
	assertOK(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(args)))
}

func TestHandleRunA11yAndExportSARIF_Success(t *testing.T) {
	h, _ := newFakeWorkflowActions(t)
	assertOK(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{"scope":"page"}`)))
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
	if _, ok := remainingNavigateAndDocumentTimeoutMs(time.Now(), 0); ok {
		t.Fatal("expected not-ok for zero total")
	}
	if ms, ok := remainingNavigateAndDocumentTimeoutMs(time.Now(), 5000); !ok || ms <= 0 {
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

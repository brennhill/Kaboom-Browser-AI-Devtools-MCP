// interact_workflow_test.go — Tests for form, navigate, and a11y workflow handlers.
package toolinteract

import (
	"encoding/json"
	"testing"
	"time"

	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

func TestHandleFillForm_Success(t *testing.T) {
	h, _ := newFakeHandler(t)
	args := `{"fields":[{"selector":"#name","value":"Ada"},{"selector":"#email","value":"a@b.co"}]}`
	assertOK(t, h.HandleFillForm(testReq(), json.RawMessage(args)))
}

func TestHandleFillForm_EmptyFields(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleFillForm(testReq(), json.RawMessage(`{"fields":[]}`)), ErrMissingParam)
}

func TestHandleFillForm_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleFillForm(testReq(), json.RawMessage(`bad`)), ErrInvalidJSON)
}

func TestHandleFillForm_FieldMissingSelectorAndIndex(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleFillForm(testReq(), json.RawMessage(`{"fields":[{"value":"x"}]}`)), ErrMissingParam)
}

func TestHandleFillForm_ByIndex_NotInRegistry(t *testing.T) {
	// Without a prior list_interactive to populate the element-index registry,
	// resolving field index 0 returns an invalid_param "call list_interactive first" error.
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleFillForm(testReq(), json.RawMessage(`{"fields":[{"index":0,"value":"x"}]}`)), ErrInvalidParam)
}

func TestHandleFillFormAndSubmit_Success(t *testing.T) {
	h, _ := newFakeHandler(t)
	args := `{"fields":[{"selector":"#name","value":"Ada"}],"submit_selector":"#go"}`
	assertOK(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(args)))
}

func TestHandleFillFormAndSubmit_BySubmitIndex_NotInRegistry(t *testing.T) {
	// submit_index requires a populated element-index registry (list_interactive first);
	// without it, the submit step returns an invalid_param error.
	h, _ := newFakeHandler(t)
	args := `{"fields":[{"selector":"#name","value":"Ada"}],"submit_index":2}`
	assertErr(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(args)), ErrInvalidParam)
}

func TestHandleFillFormAndSubmit_MissingSubmit(t *testing.T) {
	h, _ := newFakeHandler(t)
	args := `{"fields":[{"selector":"#name","value":"Ada"}]}`
	assertErr(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(args)), ErrMissingParam)
}

func TestHandleFillFormAndSubmit_EmptyFields(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(`{"fields":[],"submit_selector":"#go"}`)), ErrMissingParam)
}

func TestHandleFillFormAndSubmit_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleFillFormAndSubmit(testReq(), json.RawMessage(`bad`)), ErrInvalidJSON)
}

func TestHandleFillForm_SelectFallback(t *testing.T) {
	h, fs := newFakeHandler(t)
	// First type returns not_typeable, forcing a select fallback.
	call := 0
	fs.waitFn = func(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse {
		call++
		if call == 1 {
			return succeed(req, "type", map[string]any{"status": "complete", "error": "not_typeable"})
		}
		return succeed(req, queuedSummary, map[string]any{"status": "complete"})
	}
	// Note: not_typeable is detected via response body substring, not IsError.
	resp := h.HandleFillForm(testReq(), json.RawMessage(`{"fields":[{"selector":"#s","value":"opt"}]}`))
	assertOK(t, resp)
}

func TestHandleNavigateAndWaitFor_Success(t *testing.T) {
	h, _ := newFakeHandler(t)
	args := `{"url":"https://example.org","wait_for":"#ready"}`
	assertOK(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(args)))
}

func TestHandleNavigateAndWaitFor_MissingURL(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(`{"wait_for":"#x"}`)), ErrMissingParam)
}

func TestHandleNavigateAndWaitFor_MissingWaitFor(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(`{"url":"https://x.io"}`)), ErrMissingParam)
}

func TestHandleNavigateAndWaitFor_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(`bad`)), ErrInvalidJSON)
}

func TestHandleNavigateAndWaitFor_IncludeContent(t *testing.T) {
	h, fs := newFakeHandler(t)
	enriched := false
	fs.enrichFn = func(resp JSONRPCResponse, req JSONRPCRequest, tabID int) JSONRPCResponse {
		enriched = true
		return resp
	}
	args := `{"url":"https://example.org","wait_for":"#ready","include_content":true}`
	assertOK(t, h.HandleNavigateAndWaitFor(testReq(), json.RawMessage(args)))
	if !enriched {
		t.Fatal("expected enrichment on include_content")
	}
}

func TestHandleNavigateAndDocument_WaitsDisabled(t *testing.T) {
	h, _ := newFakeHandler(t)
	args := `{"selector":"#link","wait_for_url_change":false,"wait_for_stable":false}`
	assertOK(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(args)))
}

func TestHandleNavigateAndDocument_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(`bad`)), ErrInvalidJSON)
}

func TestHandleNavigateAndDocument_TabMismatch(t *testing.T) {
	h, fs := newFakeHandler(t)
	// tracked tab is 1; request tab_id 2 should mismatch.
	fs.cap.SetTrackingStatusForTest(1, "https://example.com/page")
	args := `{"selector":"#link","tab_id":2,"wait_for_url_change":false,"wait_for_stable":false}`
	assertErr(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(args)), ErrInvalidParam)
}

func TestHandleNavigateAndDocument_ClickError(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.waitFn = func(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse {
		return fail(req, ErrExtError, "click failed", "retry")
	}
	args := `{"selector":"#link","wait_for_url_change":false,"wait_for_stable":false}`
	assertErr(t, h.HandleNavigateAndDocument(testReq(), json.RawMessage(args)), ErrExtError)
}

func TestHandleRunA11yAndExportSARIF_Success(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{"scope":"page"}`)))
}

func TestHandleRunA11yAndExportSARIF_A11yError(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.analyzeFn = func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fail(req, ErrExtError, "audit failed", "retry")
	}
	assertErr(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`{}`)), ErrExtError)
}

func TestHandleRunA11yAndExportSARIF_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleRunA11yAndExportSARIF(testReq(), json.RawMessage(`bad`)), ErrInvalidJSON)
}

func TestExtractMCPResponseJSONPayload(t *testing.T) {
	resp := succeed(testReq(), "summary", map[string]any{"k": "v"})
	payload := extractMCPResponseJSONPayload(resp)
	if payload == nil || !contains(string(payload), "\"k\":\"v\"") {
		t.Fatalf("expected extracted payload, got %s", payload)
	}
	// Non-JSON content returns nil.
	plain := JSONRPCResponse{Result: json.RawMessage(`{"content":[{"type":"text","text":"no json here"}]}`)}
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
	yes := JSONRPCResponse{Result: json.RawMessage(`{"content":[{"type":"text","text":"not_typeable"}]}`)}
	if !IsNotTypeableError(yes) {
		t.Fatal("expected not_typeable detected")
	}
	no := succeed(testReq(), "ok", map[string]any{"status": "complete"})
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

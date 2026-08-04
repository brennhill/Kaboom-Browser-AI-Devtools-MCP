// interact_browser_actions_test.go — Tests for navigation, tab, script, and util browser actions.
package toolinteract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestBrowserActionsHandleIsTheCanonicalActionBoundary(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertOK(t, h.Handle("subtitle", testReq(), json.RawMessage(`{"text":"hello"}`)))
	assertErr(t, h.Handle("unknown", testReq(), json.RawMessage(`{}`)), mcp.ErrInvalidParam)
}

func TestHandleNavigate_Success(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	resp := h.Handle("navigate", testReq(), json.RawMessage(`{"url":"https://example.org"}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueued query, got %d", fs.enqueuedCount())
	}
	if fs.recordedCount() != 1 {
		t.Fatalf("expected 1 recorded action, got %d", fs.recordedCount())
	}
}

func TestHandleNavigate_MissingURL(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	resp := h.Handle("navigate", testReq(), json.RawMessage(`{}`))
	assertErr(t, resp, mcp.ErrMissingParam)
}

func TestHandleNavigate_InvalidJSON(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	resp := h.Handle("navigate", testReq(), json.RawMessage(`{bad`))
	assertErr(t, resp, mcp.ErrInvalidJSON)
}

func TestHandleNavigate_PilotBlocked(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	fs.blockPilot = true
	resp := h.Handle("navigate", testReq(), json.RawMessage(`{"url":"https://example.org"}`))
	assertErr(t, resp, mcp.ErrCodePilotDisabled)
	if fs.enqueuedCount() != 0 {
		t.Fatalf("blocked guard should not enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleNavigate_IncludeContent(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	resp := h.Handle("navigate", testReq(), json.RawMessage(`{"url":"https://example.org","include_content":true}`))
	assertOK(t, resp)
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) != 2 || enqueued[1].Type != "page_summary" {
		t.Fatalf("enqueued = %#v, want navigate followed by page_summary", enqueued)
	}
	var summaryParams map[string]any
	if err := json.Unmarshal(enqueued[1].Params, &summaryParams); err != nil {
		t.Fatalf("decode page_summary params: %v", err)
	}
	if _, hasScript := summaryParams["script"]; hasScript {
		t.Fatal("page_summary enrichment must not use execute script")
	}
	if summaryParams["timeout_ms"] != float64(4000) {
		t.Fatalf("timeout_ms = %v, want 4000", summaryParams["timeout_ms"])
	}
}

func TestHandleNavigate_InsecureURLRejected(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	// security mode is Normal by default => insecure prefix rejected.
	resp := h.Handle("navigate", testReq(), json.RawMessage(`{"url":"kaboom-insecure://http://internal"}`))
	assertErr(t, resp, mcp.ErrInvalidParam)
}

func TestHandleRefresh_Success(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	resp := h.Handle("refresh", testReq(), json.RawMessage(`{}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleRefresh_TabBlocked(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	fs.blockTab = true
	resp := h.Handle("refresh", testReq(), json.RawMessage(`{}`))
	assertErr(t, resp, mcp.ErrNotInitialized)
}

func TestHandleBackForward(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	assertOK(t, h.Handle("back", testReq(), json.RawMessage(`{}`)))
	assertOK(t, h.Handle("forward", testReq(), json.RawMessage(`{}`)))
	if fs.enqueuedCount() != 2 {
		t.Fatalf("expected 2 enqueues, got %d", fs.enqueuedCount())
	}
}

func TestHandleNewTab_Success(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	resp := h.Handle("new_tab", testReq(), json.RawMessage(`{"url":"https://example.org"}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleNewTab_NoURL(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	// No URL is valid for new_tab (opens blank).
	assertOK(t, h.Handle("new_tab", testReq(), json.RawMessage(`{}`)))
}

func TestHandleNewTab_InvalidJSON(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("new_tab", testReq(), json.RawMessage(`nope`)), mcp.ErrInvalidJSON)
}

func TestHandleSwitchTab_MissingTarget(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	resp := h.Handle("switch_tab", testReq(), json.RawMessage(`{}`))
	assertErr(t, resp, mcp.ErrMissingParam)
}

func TestHandleSwitchTab_NegativeIndex(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	resp := h.Handle("switch_tab", testReq(), json.RawMessage(`{"tab_index":-1}`))
	assertErr(t, resp, mcp.ErrInvalidParam)
}

func TestHandleSwitchTab_Success(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	resp := h.Handle("switch_tab", testReq(), json.RawMessage(`{"tab_id":5}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleSwitchTab_AppliesTrackingOnComplete(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	// Complete the switch_tab command so applySwitchTabTracking updates the tracked tab.
	fs.waitFn = func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
		fs.cap.Queries().RegisterCommand(correlationID, correlationID, time.Minute)
		fs.cap.Queries().ApplyCommandResult(correlationID, "complete", json.RawMessage(`{"success":true,"tab_id":42,"url":"https://switched.example","title":"Switched"}`), "")
		return mcp.Succeed(req, queuedSummary, map[string]any{"status": "complete", "correlation_id": correlationID})
	}
	resp := h.Handle("switch_tab", testReq(), json.RawMessage(`{"tab_id":42}`))
	assertOK(t, resp)
	_, tabID, tabURL := fs.cap.Extension().GetTrackingStatus()
	if tabID != 42 || tabURL != "https://switched.example" {
		t.Fatalf("tracked tab not updated: tab=%d url=%s", tabID, tabURL)
	}
}

func TestHandleActivateAndCloseTab(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	assertOK(t, h.Handle("activate_tab", testReq(), json.RawMessage(`{}`)))
	assertOK(t, h.Handle("close_tab", testReq(), json.RawMessage(`{"tab_id":3}`)))
	if fs.enqueuedCount() != 2 {
		t.Fatalf("expected 2 enqueues, got %d", fs.enqueuedCount())
	}
}

func TestHandleCloseTab_InvalidJSON(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("close_tab", testReq(), json.RawMessage(`x`)), mcp.ErrInvalidJSON)
}

func TestHandleHighlight_Success(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertOK(t, h.Handle("highlight", testReq(), json.RawMessage(`{"selector":"#btn"}`)))
}

func TestHandleHighlight_MissingSelector(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("highlight", testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestHandleExecuteJS_Success(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertOK(t, h.Handle("execute_js", testReq(), json.RawMessage(`{"script":"1+1"}`)))
}

func TestHandleExecuteJS_MissingScript(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("execute_js", testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestHandleExecuteJS_InvalidWorld(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("execute_js", testReq(), json.RawMessage(`{"script":"1","world":"moon"}`)), mcp.ErrInvalidParam)
}

func TestHandleExecuteJS_MainWorldCSPBlocked(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	fs.blockCSP = true
	resp := h.Handle("execute_js", testReq(), json.RawMessage(`{"script":"1","world":"main"}`))
	assertErr(t, resp, mcp.ErrInvalidParam)
}

func TestHandleSubtitle_SetAndClear(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertOK(t, h.Handle("subtitle", testReq(), json.RawMessage(`{"text":"hello"}`)))
	assertOK(t, h.Handle("subtitle", testReq(), json.RawMessage(`{"text":""}`)))
}

func TestHandleSubtitle_MissingText(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("subtitle", testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestResolveNavigateURL_PassThrough(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	got, err := h.resolveNavigateURL("https://example.com/x")
	if err != nil || got != "https://example.com/x" {
		t.Fatalf("plain url should pass through, got %q err=%v", got, err)
	}
}

func TestResolveNavigateURL_InsecureRequiresMode(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	_, err := h.resolveNavigateURL("kaboom-insecure://http://internal.host/path")
	if err == nil {
		t.Fatal("expected error when security mode is not insecure_proxy")
	}
}

func TestResolveNavigateURL_InsecureProxyRewrite(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	fs.cap.Extension().SetSecurityMode(capture.SecurityModeInsecureProxy, nil)
	got, err := h.resolveNavigateURL("kaboom-insecure://http://internal.host/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(got, "127.0.0.1:7890/insecure-proxy?target=") {
		t.Fatalf("expected rewritten proxy URL, got %q", got)
	}
}

func TestResolveNavigateURL_InsecureEmptyTarget(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	fs.cap.Extension().SetSecurityMode(capture.SecurityModeInsecureProxy, nil)
	if _, err := h.resolveNavigateURL("kaboom-insecure://"); err == nil {
		t.Fatal("expected error for empty insecure target")
	}
}

func TestResolveNavigateURL_InsecureNonHTTPScheme(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	fs.cap.Extension().SetSecurityMode(capture.SecurityModeInsecureProxy, nil)
	if _, err := h.resolveNavigateURL("kaboom-insecure://ftp://internal.host"); err == nil {
		t.Fatal("expected error for non-http insecure target scheme")
	}
}

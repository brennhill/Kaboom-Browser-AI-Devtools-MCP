// interact_browser_actions_test.go — Tests for navigation, tab, script, and util browser actions.
package toolinteract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestHandleNavigate_Success(t *testing.T) {
	h, fs := newFakeHandler(t)
	resp := h.HandleBrowserActionNavigateImpl(testReq(), json.RawMessage(`{"url":"https://example.org"}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueued query, got %d", fs.enqueuedCount())
	}
	if fs.recordedCount() != 1 {
		t.Fatalf("expected 1 recorded action, got %d", fs.recordedCount())
	}
}

func TestHandleNavigate_MissingURL(t *testing.T) {
	h, _ := newFakeHandler(t)
	resp := h.HandleBrowserActionNavigateImpl(testReq(), json.RawMessage(`{}`))
	assertErr(t, resp, mcp.ErrMissingParam)
}

func TestHandleNavigate_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	resp := h.HandleBrowserActionNavigateImpl(testReq(), json.RawMessage(`{bad`))
	assertErr(t, resp, mcp.ErrInvalidJSON)
}

func TestHandleNavigate_PilotBlocked(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.blockPilot = true
	resp := h.HandleBrowserActionNavigateImpl(testReq(), json.RawMessage(`{"url":"https://example.org"}`))
	assertErr(t, resp, mcp.ErrCodePilotDisabled)
	if fs.enqueuedCount() != 0 {
		t.Fatalf("blocked guard should not enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleNavigate_IncludeContent(t *testing.T) {
	h, fs := newFakeHandler(t)
	enriched := false
	fs.enrichFn = func(resp mcp.JSONRPCResponse, req mcp.JSONRPCRequest, tabID int) mcp.JSONRPCResponse {
		enriched = true
		return resp
	}
	resp := h.HandleBrowserActionNavigateImpl(testReq(), json.RawMessage(`{"url":"https://example.org","include_content":true}`))
	assertOK(t, resp)
	if !enriched {
		t.Fatal("expected EnrichNavigateResponse to be called")
	}
}

func TestHandleNavigate_InsecureURLRejected(t *testing.T) {
	h, _ := newFakeHandler(t)
	// security mode is Normal by default => insecure prefix rejected.
	resp := h.HandleBrowserActionNavigateImpl(testReq(), json.RawMessage(`{"url":"kaboom-insecure://http://internal"}`))
	assertErr(t, resp, mcp.ErrInvalidParam)
}

func TestHandleRefresh_Success(t *testing.T) {
	h, fs := newFakeHandler(t)
	resp := h.HandleBrowserActionRefreshImpl(testReq(), json.RawMessage(`{}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleRefresh_TabBlocked(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.blockTab = true
	resp := h.HandleBrowserActionRefreshImpl(testReq(), json.RawMessage(`{}`))
	assertErr(t, resp, mcp.ErrNotInitialized)
}

func TestHandleBackForward(t *testing.T) {
	h, fs := newFakeHandler(t)
	assertOK(t, h.HandleBrowserActionBackImpl(testReq(), json.RawMessage(`{}`)))
	assertOK(t, h.HandleBrowserActionForwardImpl(testReq(), json.RawMessage(`{}`)))
	if fs.enqueuedCount() != 2 {
		t.Fatalf("expected 2 enqueues, got %d", fs.enqueuedCount())
	}
}

func TestHandleNewTab_Success(t *testing.T) {
	h, fs := newFakeHandler(t)
	resp := h.HandleBrowserActionNewTabImpl(testReq(), json.RawMessage(`{"url":"https://example.org"}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleNewTab_NoURL(t *testing.T) {
	h, _ := newFakeHandler(t)
	// No URL is valid for new_tab (opens blank).
	assertOK(t, h.HandleBrowserActionNewTabImpl(testReq(), json.RawMessage(`{}`)))
}

func TestHandleNewTab_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleBrowserActionNewTabImpl(testReq(), json.RawMessage(`nope`)), mcp.ErrInvalidJSON)
}

func TestHandleSwitchTab_MissingTarget(t *testing.T) {
	h, _ := newFakeHandler(t)
	resp := h.HandleBrowserActionSwitchTabImpl(testReq(), json.RawMessage(`{}`))
	assertErr(t, resp, mcp.ErrMissingParam)
}

func TestHandleSwitchTab_NegativeIndex(t *testing.T) {
	h, _ := newFakeHandler(t)
	resp := h.HandleBrowserActionSwitchTabImpl(testReq(), json.RawMessage(`{"tab_index":-1}`))
	assertErr(t, resp, mcp.ErrInvalidParam)
}

func TestHandleSwitchTab_Success(t *testing.T) {
	h, fs := newFakeHandler(t)
	resp := h.HandleBrowserActionSwitchTabImpl(testReq(), json.RawMessage(`{"tab_id":5}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleSwitchTab_AppliesTrackingOnComplete(t *testing.T) {
	h, fs := newFakeHandler(t)
	// Complete the switch_tab command so ApplySwitchTabTracking updates the tracked tab.
	fs.waitFn = func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
		fs.cap.Queries().RegisterCommand(correlationID, correlationID, time.Minute)
		fs.cap.Queries().ApplyCommandResult(correlationID, "complete", json.RawMessage(`{"success":true,"tab_id":42,"url":"https://switched.example","title":"Switched"}`), "")
		return mcp.Succeed(req, queuedSummary, map[string]any{"status": "complete", "correlation_id": correlationID})
	}
	resp := h.HandleBrowserActionSwitchTabImpl(testReq(), json.RawMessage(`{"tab_id":42}`))
	assertOK(t, resp)
	_, tabID, tabURL := fs.cap.GetTrackingStatus()
	if tabID != 42 || tabURL != "https://switched.example" {
		t.Fatalf("tracked tab not updated: tab=%d url=%s", tabID, tabURL)
	}
}

func TestHandleActivateAndCloseTab(t *testing.T) {
	h, fs := newFakeHandler(t)
	assertOK(t, h.HandleActivateTabImpl(testReq(), json.RawMessage(`{}`)))
	assertOK(t, h.HandleBrowserActionCloseTabImpl(testReq(), json.RawMessage(`{"tab_id":3}`)))
	if fs.enqueuedCount() != 2 {
		t.Fatalf("expected 2 enqueues, got %d", fs.enqueuedCount())
	}
}

func TestHandleCloseTab_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleBrowserActionCloseTabImpl(testReq(), json.RawMessage(`x`)), mcp.ErrInvalidJSON)
}

func TestHandleHighlight_Success(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleHighlightImpl(testReq(), json.RawMessage(`{"selector":"#btn"}`)))
}

func TestHandleHighlight_MissingSelector(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleHighlightImpl(testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestHandleExecuteJS_Success(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleExecuteJSImpl(testReq(), json.RawMessage(`{"script":"1+1"}`)))
}

func TestHandleExecuteJS_MissingScript(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleExecuteJSImpl(testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestHandleExecuteJS_InvalidWorld(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleExecuteJSImpl(testReq(), json.RawMessage(`{"script":"1","world":"moon"}`)), mcp.ErrInvalidParam)
}

func TestHandleExecuteJS_MainWorldCSPBlocked(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.blockCSP = true
	resp := h.HandleExecuteJSImpl(testReq(), json.RawMessage(`{"script":"1","world":"main"}`))
	assertErr(t, resp, mcp.ErrInvalidParam)
}

func TestHandleSubtitle_SetAndClear(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleSubtitleImpl(testReq(), json.RawMessage(`{"text":"hello"}`)))
	assertOK(t, h.HandleSubtitleImpl(testReq(), json.RawMessage(`{"text":""}`)))
}

func TestHandleSubtitle_MissingText(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleSubtitleImpl(testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestResolveNavigateURL_PassThrough(t *testing.T) {
	h, _ := newFakeHandler(t)
	got, err := h.ResolveNavigateURLImpl("https://example.com/x")
	if err != nil || got != "https://example.com/x" {
		t.Fatalf("plain url should pass through, got %q err=%v", got, err)
	}
}

func TestResolveNavigateURL_InsecureRequiresMode(t *testing.T) {
	h, _ := newFakeHandler(t)
	_, err := h.ResolveNavigateURLImpl("kaboom-insecure://http://internal.host/path")
	if err == nil {
		t.Fatal("expected error when security mode is not insecure_proxy")
	}
}

func TestResolveNavigateURL_InsecureProxyRewrite(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)
	got, err := h.ResolveNavigateURLImpl("kaboom-insecure://http://internal.host/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(got, "127.0.0.1:7890/insecure-proxy?target=") {
		t.Fatalf("expected rewritten proxy URL, got %q", got)
	}
}

func TestResolveNavigateURL_InsecureEmptyTarget(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)
	if _, err := h.ResolveNavigateURLImpl("kaboom-insecure://"); err == nil {
		t.Fatal("expected error for empty insecure target")
	}
}

func TestResolveNavigateURL_InsecureNonHTTPScheme(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.cap.SetSecurityMode(capture.SecurityModeInsecureProxy, nil)
	if _, err := h.ResolveNavigateURLImpl("kaboom-insecure://ftp://internal.host"); err == nil {
		t.Fatal("expected error for non-http insecure target scheme")
	}
}

// interact_page_test.go — Tests for the page-level interact actions in
// interact_page.go (content extraction, composable steps, clipboard, draw) plus the
// list_interactive response shaping in interact_elements.go.
package toolinteract

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"strings"
	"testing"
)

func TestHandleGetReadable(t *testing.T) {
	h, fs := newFakePageActions(t)
	assertOK(t, h.HandleGetReadable(testReq(), json.RawMessage(`{}`)))
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleGetMarkdown(t *testing.T) {
	h, _ := newFakePageActions(t)
	assertOK(t, h.HandleGetMarkdown(testReq(), json.RawMessage(`{"timeout_ms":40000}`)))
}

func TestHandleContentExtraction_TabBlocked(t *testing.T) {
	h, fs := newFakePageActions(t)
	fs.blockTab = true
	assertErr(t, h.HandleContentExtraction(testReq(), json.RawMessage(`{}`), "get_readable", "readable"), mcp.ErrNotInitialized)
}

func TestHandleWaitForStable(t *testing.T) {
	h, _ := newFakePageActions(t)
	assertOK(t, h.HandleWaitForStable(testReq(), json.RawMessage(`{}`)))
}

func TestHandleWaitForStable_WithParams(t *testing.T) {
	h, _ := newFakePageActions(t)
	assertOK(t, h.HandleWaitForStable(testReq(), json.RawMessage(`{"stability_ms":300,"timeout_ms":2000}`)))
}

func TestHandleAutoDismissOverlays(t *testing.T) {
	h, _ := newFakePageActions(t)
	assertOK(t, h.HandleAutoDismissOverlays(testReq(), json.RawMessage(`{}`)))
}

func TestQueueComposableHelpers(t *testing.T) {
	h, fs := newFakePageActions(t)
	h.QueueComposableAutoDismiss(testReq())
	h.QueueComposableActionDiff(testReq())
	h.QueueComposableWaitForStable(testReq(), 0)
	h.QueueComposableWaitForStable(testReq(), 250)
	h.QueueComposableSubtitle(testReq(), "hello")
	if fs.enqueuedCount() != 5 {
		t.Fatalf("expected 5 composable enqueues, got %d", fs.enqueuedCount())
	}
}

func TestHandleClipboardRead(t *testing.T) {
	h, fs := newFakePageActions(t)
	assertOK(t, h.HandleClipboardRead(testReq(), json.RawMessage(`{}`)))
	// records clipboard_read on success.
	if fs.recordedCount() != 1 {
		t.Fatalf("expected 1 recorded action, got %d", fs.recordedCount())
	}
}

func TestHandleClipboardWrite(t *testing.T) {
	h, _ := newFakePageActions(t)
	assertOK(t, h.HandleClipboardWrite(testReq(), json.RawMessage(`{"text":"copy me"}`)))
}

func TestHandleClipboardWrite_MissingText(t *testing.T) {
	h, _ := newFakePageActions(t)
	assertErr(t, h.HandleClipboardWrite(testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestHandleClipboardWrite_InvalidJSON(t *testing.T) {
	h, _ := newFakePageActions(t)
	assertErr(t, h.HandleClipboardWrite(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestHandleClipboardRead_PilotBlockedNoRecord(t *testing.T) {
	h, fs := newFakePageActions(t)
	fs.blockPilot = true
	assertErr(t, h.HandleClipboardRead(testReq(), json.RawMessage(`{}`)), mcp.ErrCodePilotDisabled)
	if fs.recordedCount() != 0 {
		t.Fatalf("blocked clipboard read should not record, got %d", fs.recordedCount())
	}
}

func TestHandleDrawModeStart_Success(t *testing.T) {
	h, fs := newFakePageActions(t)
	resp := h.HandleDrawModeStart(testReq(), json.RawMessage(`{"annot_session":"s1"}`))
	assertOK(t, resp)
	if fs.drawStarted != 1 {
		t.Fatalf("expected MarkDrawStarted called once, got %d", fs.drawStarted)
	}
}

func TestHandleDrawModeStart_NoArgs(t *testing.T) {
	h, _ := newFakePageActions(t)
	assertOK(t, h.HandleDrawModeStart(testReq(), nil))
}

func TestHandleDrawModeStart_ExtensionBlocked(t *testing.T) {
	h, fs := newFakePageActions(t)
	fs.blockExt = true
	assertErr(t, h.HandleDrawModeStart(testReq(), json.RawMessage(`{}`)), mcp.ErrNotInitialized)
	if fs.drawStarted != 0 {
		t.Fatalf("blocked draw should not mark started, got %d", fs.drawStarted)
	}
}

func TestHandleDrawModeStart_TabBlocked(t *testing.T) {
	h, fs := newFakePageActions(t)
	fs.blockTab = true
	assertErr(t, h.HandleDrawModeStart(testReq(), json.RawMessage(`{}`)), mcp.ErrNotInitialized)
}

func TestHandleListInteractive_Success(t *testing.T) {
	h, fs := newFakePageActions(t)
	// Return a response with elements so index metadata is built.
	fs.waitFn = func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
		return mcp.Succeed(req, "list_interactive results", map[string]any{
			"elements": []any{
				map[string]any{"index": float64(0), "selector": "#a", "tag": "input"},
				map[string]any{"index": float64(1), "selector": "#b", "tag": "button"},
			},
		})
	}
	resp := h.dom.HandleListInteractive(testReq(), json.RawMessage(`{}`))
	result := assertOK(t, resp)
	// index_generation should be annotated into the response text.
	if !contains(firstText(result), "index_generation") {
		t.Fatalf("expected index_generation annotation, got: %s", firstText(result))
	}
	// The element index should now resolve.
	sel, ok, _, _ := h.dom.resolveIndexToSelector("client-test", 0, 1, "")
	if !ok || sel != "#b" {
		t.Fatalf("expected #b resolved, got %q ok=%v", sel, ok)
	}
}

func TestHandleListInteractive_Truncation(t *testing.T) {
	h, fs := newFakePageActions(t)
	fs.waitFn = func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
		elems := make([]any, 8)
		for i := range elems {
			elems[i] = map[string]any{"index": float64(i), "selector": "#e", "tag": "div"}
		}
		return mcp.Succeed(req, "list_interactive results", map[string]any{"elements": elems})
	}
	resp := h.dom.HandleListInteractive(testReq(), json.RawMessage(`{"limit":3}`))
	result := assertOK(t, resp)
	if !contains(firstText(result), "\"truncated\":true") {
		t.Fatalf("expected truncated marker, got: %s", firstText(result))
	}
}

func TestSetNestedElements_TopLevel(t *testing.T) {
	data := map[string]any{"elements": []any{1, 2, 3}}
	setNestedElements(data, []any{9})
	if len(data["elements"].([]any)) != 1 {
		t.Fatal("expected top-level elements replaced")
	}
}

func TestSetNestedElements_Nested(t *testing.T) {
	data := map[string]any{"result": map[string]any{"elements": []any{1, 2}}}
	setNestedElements(data, []any{9})
	inner := data["result"].(map[string]any)["elements"].([]any)
	if len(inner) != 1 {
		t.Fatal("expected nested elements replaced")
	}
}

func TestEnrichExploreWithMenusSeparatesNavigation(t *testing.T) {
	payload := map[string]any{
		"interactive_count": float64(4),
		"interactive_elements": []any{
			map[string]any{"index": float64(0), "label": "Home", "tag": "a", "href": "/", "landmark_tag": "nav", "visible": true, "bbox": map[string]any{"x": float64(10), "y": float64(10), "width": float64(50), "height": float64(20)}},
			map[string]any{"index": float64(1), "label": "Docs", "tag": "a", "href": "/docs", "landmark_tag": "nav", "visible": true, "bbox": map[string]any{"x": float64(70), "y": float64(10), "width": float64(50), "height": float64(20)}},
			map[string]any{"index": float64(2), "label": "Save", "tag": "button", "role": "button", "visible": false},
			"unparsed",
		},
	}
	resp := mcp.Succeed(testReq(), "Explore page", payload)
	enriched := enrichExploreWithMenus(resp)
	result := parseToolResult(t, enriched)
	text := firstText(result)
	if !strings.Contains(text, `"site_menus"`) || !strings.Contains(text, `"interactive_count":2`) {
		t.Fatalf("enriched response = %s", text)
	}
	if strings.Contains(text, `"label":"Home"`) && strings.Contains(text, `"interactive_elements":[{"bbox"`) {
		t.Fatalf("menu elements remained in interactive list: %s", text)
	}
}

func TestEnrichExploreWithMenusLeavesUnusableResultsAlone(t *testing.T) {
	responses := []mcp.JSONRPCResponse{
		{Result: json.RawMessage(`not-json`)},
		mcp.SucceedText(testReq(), "no JSON here"),
		mcp.SucceedText(testReq(), "prefix\n{bad"),
		mcp.Succeed(testReq(), "Explore", map[string]any{"interactive_elements": []any{}}),
	}
	for _, response := range responses {
		before := string(response.Result)
		if got := enrichExploreWithMenus(response); string(got.Result) != before {
			t.Fatalf("response changed: %s -> %s", before, got.Result)
		}
	}
}

func TestAppendInteractiveToResponseBestEffortFailures(t *testing.T) {
	h, fs := newFakePageActions(t)
	base := mcp.JSONRPCResponse{Result: json.RawMessage(`bad`)}
	fs.waitFn = func(req mcp.JSONRPCRequest, _ string, _ json.RawMessage, _ string) mcp.JSONRPCResponse {
		return mcp.SucceedText(req, "button")
	}
	if got := h.AppendInteractiveToResponse(base, testReq()); string(got.Result) != "bad" {
		t.Fatalf("malformed base changed: %s", got.Result)
	}

	fs.waitFn = func(req mcp.JSONRPCRequest, _ string, _ json.RawMessage, _ string) mcp.JSONRPCResponse {
		return mcp.Fail(req, "blocked", "blocked", "retry")
	}
	base = mcp.SucceedText(testReq(), "complete")
	if got := h.AppendInteractiveToResponse(base, testReq()); string(got.Result) != string(base.Result) {
		t.Fatalf("error list response changed base: %s", got.Result)
	}
}

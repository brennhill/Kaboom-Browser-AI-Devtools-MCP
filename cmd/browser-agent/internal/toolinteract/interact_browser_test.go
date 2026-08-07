// interact_browser_test.go — Browser, page, and browser-state action contract tests.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"strings"
	"testing"
	"time"
)

func TestBrowserActionsHandleIsTheCanonicalActionBoundary(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertOK(t, h.Handle("subtitle", testReq(), json.RawMessage(`{"text":"hello"}`)))
	assertErr(t, h.Handle("unknown", testReq(), json.RawMessage(`{}`)), mcp.ErrInvalidParam)
}

func TestHandleNavigate_Success(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	resp := h.Handle("navigate", testReq(), json.RawMessage(`{"url":"https://example.org"}`))
	result := assertOK(t, resp)
	if !contains(firstText(result), "correlation_id") || !contains(firstText(result), `"status":"complete"`) {
		t.Fatalf("navigate response = %s", firstText(result))
	}
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) != 1 || enqueued[0].Type != "browser_action" {
		t.Fatalf("navigate enqueue = %#v", enqueued)
	}
	var params map[string]any
	if err := json.Unmarshal(enqueued[0].Params, &params); err != nil {
		t.Fatalf("decode navigate params: %v", err)
	}
	if params["action"] != "navigate" {
		t.Fatalf("navigate action = %#v", params["action"])
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
	h, fs := newFakeBrowserActions(t)
	assertOK(t, h.Handle("highlight", testReq(), json.RawMessage(`{"selector":"#btn","tab_id":99}`)))
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) != 1 || enqueued[0].Type != "highlight" || enqueued[0].TabID != 99 {
		t.Fatalf("highlight enqueue = %#v", enqueued)
	}
}

func TestHandleHighlight_MissingSelector(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("highlight", testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestHandleHighlight_InvalidJSONAndPilotBlock(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	assertErr(t, h.Handle("highlight", testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
	fs.blockPilot = true
	assertErr(t, h.Handle("highlight", testReq(), json.RawMessage(`{"selector":"#btn"}`)), mcp.ErrCodePilotDisabled)
}

func TestHandleExecuteJS_Success(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	result := assertOK(t, h.Handle("execute_js", testReq(), json.RawMessage(`{"script":"1+1"}`)))
	if !contains(firstText(result), "correlation_id") || !contains(firstText(result), `"status":"complete"`) {
		t.Fatalf("execute_js response = %s", firstText(result))
	}
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) != 1 || enqueued[0].Type != "execute" {
		t.Fatalf("execute_js enqueue = %#v", enqueued)
	}
}

func TestHandleExecuteJS_InvalidJSON(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("execute_js", testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
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
	h, fs := newFakeBrowserActions(t)
	assertOK(t, h.Handle("subtitle", testReq(), json.RawMessage(`{"text":"hello"}`)))
	cleared := assertOK(t, h.Handle("subtitle", testReq(), json.RawMessage(`{"text":""}`)))
	if !strings.Contains(strings.ToLower(firstText(cleared)), "clear") {
		t.Fatalf("subtitle clear response = %s", firstText(cleared))
	}
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) != 2 || enqueued[0].Type != "subtitle" || enqueued[1].Type != "subtitle" {
		t.Fatalf("subtitle enqueues = %#v", enqueued)
	}
}

func TestHandleSubtitle_MissingText(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("subtitle", testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestHandleSubtitle_InvalidJSON(t *testing.T) {
	h, _ := newFakeBrowserActions(t)
	assertErr(t, h.Handle("subtitle", testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestHandleContentExtractionRoutesDedicatedQueries(t *testing.T) {
	for _, testCase := range []struct {
		name, queryType, label string
		args                   json.RawMessage
		wantTimeout, wantTab   int
	}{
		{name: "readable defaults", queryType: "get_readable", label: "readable", args: json.RawMessage(`{}`), wantTimeout: 10000},
		{name: "markdown custom", queryType: "get_markdown", label: "markdown", args: json.RawMessage(`{"timeout_ms":7000,"tab_id":88}`), wantTimeout: 7000, wantTab: 88},
		{name: "page summary clamps", queryType: "page_summary", label: "page_summary", args: json.RawMessage(`{"timeout_ms":60000,"tab_id":77}`), wantTimeout: 30000, wantTab: 77},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h, fs := newFakePageActions(t)
			assertOK(t, h.HandleContentExtraction(testReq(), testCase.args, testCase.queryType, testCase.label))
			enqueued := fs.enqueuedSnapshot()
			if len(enqueued) != 1 || enqueued[0].Type != testCase.queryType || enqueued[0].TabID != testCase.wantTab {
				t.Fatalf("enqueued = %#v", enqueued)
			}
			var params map[string]any
			if err := json.Unmarshal(enqueued[0].Params, &params); err != nil {
				t.Fatalf("decode params: %v", err)
			}
			if params["timeout_ms"] != float64(testCase.wantTimeout) {
				t.Fatalf("timeout_ms = %v, want %d", params["timeout_ms"], testCase.wantTimeout)
			}
			if _, exists := params["script"]; exists {
				t.Fatalf("%s must not route through execute script", testCase.queryType)
			}
		})
	}
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

func TestHandleClipboardWrite_PilotBlockedNoRecord(t *testing.T) {
	h, fs := newFakePageActions(t)
	fs.blockPilot = true
	assertErr(t, h.HandleClipboardWrite(testReq(), json.RawMessage(`{"text":"copy me"}`)), mcp.ErrCodePilotDisabled)
	if fs.recordedCount() != 0 {
		t.Fatalf("blocked clipboard write should not record, got %d", fs.recordedCount())
	}
}

func TestHandleDrawModeStart_Success(t *testing.T) {
	h, fs := newFakePageActions(t)
	resp := h.HandleDrawModeStart(testReq(), json.RawMessage(`{"annot_session":"s1","tab_id":42}`))
	result := assertOK(t, resp)
	if !contains(firstText(result), "correlation_id") {
		t.Fatalf("draw response = %s", firstText(result))
	}
	if fs.drawStarted != 1 {
		t.Fatalf("expected MarkDrawStarted called once, got %d", fs.drawStarted)
	}
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) != 1 || enqueued[0].Type != "draw_mode" || enqueued[0].TabID != 42 {
		t.Fatalf("draw enqueue = %#v", enqueued)
	}
	var params map[string]string
	if err := json.Unmarshal(enqueued[0].Params, &params); err != nil {
		t.Fatalf("decode draw params: %v", err)
	}
	if params["action"] != "start" || params["annot_session"] != "s1" {
		t.Fatalf("draw params = %#v", params)
	}
}

func TestHandleDrawModeStart_NoArgs(t *testing.T) {
	h, _ := newFakePageActions(t)
	assertOK(t, h.HandleDrawModeStart(testReq(), nil))
}

func TestHandleDrawModeStart_InvalidJSON(t *testing.T) {
	h, fs := newFakePageActions(t)
	assertErr(t, h.HandleDrawModeStart(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
	if fs.enqueuedCount() != 0 || fs.drawStarted != 0 {
		t.Fatalf("malformed draw mutated state: enqueued=%d started=%d", fs.enqueuedCount(), fs.drawStarted)
	}
}

func TestHandleDrawModeStart_PilotBlocked(t *testing.T) {
	h, fs := newFakePageActions(t)
	fs.blockPilot = true
	assertErr(t, h.HandleDrawModeStart(testReq(), json.RawMessage(`{}`)), mcp.ErrCodePilotDisabled)
	if fs.enqueuedCount() != 0 || fs.drawStarted != 0 {
		t.Fatalf("pilot-blocked draw mutated state: enqueued=%d started=%d", fs.enqueuedCount(), fs.drawStarted)
	}
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
	resp := h.dom.HandleListInteractive(testReq(), json.RawMessage(`{"tab_id":42}`))
	result := assertOK(t, resp)
	// index_generation should be annotated into the response text.
	if !contains(firstText(result), "index_generation") {
		t.Fatalf("expected index_generation annotation, got: %s", firstText(result))
	}
	// The element index should now resolve.
	sel, ok, _, _ := h.dom.resolveIndexToSelector("client-test", 42, 1, "")
	if !ok || sel != "#b" {
		t.Fatalf("expected #b resolved, got %q ok=%v", sel, ok)
	}
	enqueued := fs.enqueuedSnapshot()
	if len(enqueued) != 1 || enqueued[0].Type != "dom_action" || enqueued[0].TabID != 42 {
		t.Fatalf("list_interactive enqueue = %#v", enqueued)
	}
}

func TestHandleListInteractive_InvalidJSONAndPilotBlock(t *testing.T) {
	h, fs := newFakePageActions(t)
	assertErr(t, h.dom.HandleListInteractive(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
	fs.blockPilot = true
	assertErr(t, h.dom.HandleListInteractive(testReq(), json.RawMessage(`{}`)), mcp.ErrCodePilotDisabled)
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

func TestHandleSetStorage_Success(t *testing.T) {
	h, fs := newFakeStorageActions(t)
	resp := h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k","value":"v"}`))
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleSetStorage_InvalidType(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"cookies","key":"k","value":"v"}`)), mcp.ErrInvalidParam)
}

func TestHandleSetStorage_MissingKey(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","value":"v"}`)), mcp.ErrMissingParam)
}

func TestHandleSetStorage_MissingValue(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k"}`)), mcp.ErrMissingParam)
}

func TestHandleSetStorage_InvalidJSON(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestHandleSetStorage_InvalidWorld(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage","key":"k","value":"v","world":"moon"}`)), mcp.ErrInvalidParam)
}

func TestHandleDeleteStorage_Success(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleDeleteStorage(testReq(), json.RawMessage(`{"storage_type":"sessionStorage","key":"k"}`)))
}

func TestHandleDeleteStorage_MissingKey(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleDeleteStorage(testReq(), json.RawMessage(`{"storage_type":"sessionStorage"}`)), mcp.ErrMissingParam)
}

func TestHandleDeleteStorage_InvalidType(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleDeleteStorage(testReq(), json.RawMessage(`{"storage_type":"bogus","key":"k"}`)), mcp.ErrInvalidParam)
}

func TestHandleClearStorage_Success(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleClearStorage(testReq(), json.RawMessage(`{"storage_type":"localStorage"}`)))
}

func TestHandleClearStorage_InvalidType(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleClearStorage(testReq(), json.RawMessage(`{"storage_type":"bogus"}`)), mcp.ErrInvalidParam)
}

func TestHandleClearStorage_InvalidJSON(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleClearStorage(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestStorageAndCookieActionsPreserveSharedExecutionTarget(t *testing.T) {
	tests := []struct {
		name string
		args json.RawMessage
		run  func(*StorageActions, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	}{
		{
			name: "set",
			args: json.RawMessage(`{"storage_type":"localStorage","key":"k","value":"v","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleSetStorage,
		},
		{
			name: "delete",
			args: json.RawMessage(`{"storage_type":"localStorage","key":"k","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleDeleteStorage,
		},
		{
			name: "clear",
			args: json.RawMessage(`{"storage_type":"localStorage","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleClearStorage,
		},
		{
			name: "set cookie",
			args: json.RawMessage(`{"name":"sid","value":"abc","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleSetCookie,
		},
		{
			name: "delete cookie",
			args: json.RawMessage(`{"name":"sid","tab_id":42,"timeout_ms":1234,"world":"isolated"}`),
			run:  (*StorageActions).HandleDeleteCookie,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, fs := newFakeStorageActions(t)
			assertOK(t, tt.run(h, testReq(), tt.args))

			queued := fs.enqueuedSnapshot()
			if len(queued) != 1 {
				t.Fatalf("queued commands = %d, want 1", len(queued))
			}
			if queued[0].TabID != 42 {
				t.Fatalf("tab_id = %d, want 42", queued[0].TabID)
			}
			var params map[string]any
			if err := json.Unmarshal(queued[0].Params, &params); err != nil {
				t.Fatalf("decode queued params: %v", err)
			}
			if params["timeout_ms"] != float64(1234) {
				t.Fatalf("timeout_ms = %v, want 1234", params["timeout_ms"])
			}
			if params["world"] != "isolated" {
				t.Fatalf("world = %v, want isolated", params["world"])
			}
		})
	}
}

func TestHandleSetCookie_Success(t *testing.T) {
	h, fs := newFakeStorageActions(t)
	assertOK(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"name":"sid","value":"abc","domain":"example.com","path":"/app"}`)))
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleSetCookie_DefaultPath(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"name":"sid","value":"abc"}`)))
}

func TestHandleSetCookie_MissingName(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"value":"abc"}`)), mcp.ErrMissingParam)
}

func TestHandleSetCookie_MissingValue(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleSetCookie(testReq(), json.RawMessage(`{"name":"sid"}`)), mcp.ErrMissingParam)
}

func TestHandleDeleteCookie_Success(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleDeleteCookie(testReq(), json.RawMessage(`{"name":"sid","domain":"example.com","path":"/app"}`)))
}

func TestHandleDeleteCookie_DefaultPath(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertOK(t, h.HandleDeleteCookie(testReq(), json.RawMessage(`{"name":"sid"}`)))
}

func TestHandleDeleteCookie_MissingName(t *testing.T) {
	h, _ := newFakeStorageActions(t)
	assertErr(t, h.HandleDeleteCookie(testReq(), json.RawMessage(`{}`)), mcp.ErrMissingParam)
}

func TestValidateStorageType(t *testing.T) {
	expr, _, ok := validateStorageType(testReq(), "localStorage")
	if !ok || expr != "localStorage" {
		t.Fatalf("expected localStorage valid, got %q ok=%v", expr, ok)
	}
	if _, _, ok := validateStorageType(testReq(), "nope"); ok {
		t.Fatal("expected invalid type rejected")
	}
}

func TestJSQuote(t *testing.T) {
	if jsQuote("a\"b") != `"a\"b"` {
		t.Fatalf("unexpected jsQuote output: %s", jsQuote("a\"b"))
	}
}

func TestHandleNavigate_InsecureURLRejected(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	resp := h.Handle("navigate", testReq(), json.RawMessage(`{"url":"kaboom-insecure://http://internal"}`))
	assertErr(t, resp, mcp.ErrInvalidParam)
	if !contains(firstText(parseToolResult(t, resp)), "security_mode") || fs.enqueuedCount() != 0 {
		t.Fatalf("insecure rejection = %s; enqueued=%d", firstText(parseToolResult(t, resp)), fs.enqueuedCount())
	}
}

func TestHandleInsecureProxyRewritesBrowserTargets(t *testing.T) {
	for _, tc := range []struct{ action, target, encoded string }{
		{"navigate", "https://example.com/path?q=1#frag", "https%3A%2F%2Fexample.com%2Fpath%3Fq%3D1%23frag"},
		{"new_tab", "https://example.org", "https%3A%2F%2Fexample.org"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			h, fs := newFakeBrowserActions(t)
			fs.cap.Extension().SetSecurityMode(syncruntime.SecurityModeInsecureProxy, nil)
			args := json.RawMessage(`{"url":"kaboom-insecure://` + tc.target + `"}`)
			assertOK(t, h.Handle(tc.action, testReq(), args))
			enqueued := fs.enqueuedSnapshot()
			if len(enqueued) != 1 || enqueued[0].Type != "browser_action" {
				t.Fatalf("insecure browser enqueue = %#v", enqueued)
			}
			var params map[string]any
			if err := json.Unmarshal(enqueued[0].Params, &params); err != nil {
				t.Fatalf("decode insecure browser params: %v", err)
			}
			urlValue, _ := params["url"].(string)
			if !strings.Contains(urlValue, "http://127.0.0.1:7890/insecure-proxy?target=") || !strings.Contains(urlValue, tc.encoded) {
				t.Fatalf("rewritten browser URL = %q", urlValue)
			}
		})
	}
}

func TestResolveNavigateURLContracts(t *testing.T) {
	h, fs := newFakeBrowserActions(t)
	got, err := h.resolveNavigateURL("https://example.com/x")
	if err != nil || got != "https://example.com/x" {
		t.Fatalf("plain URL = %q, %v", got, err)
	}
	if _, err := h.resolveNavigateURL("kaboom-insecure://http://internal.host/path"); err == nil {
		t.Fatal("insecure URL accepted in normal mode")
	}
	fs.cap.Extension().SetSecurityMode(syncruntime.SecurityModeInsecureProxy, nil)
	got, err = h.resolveNavigateURL("kaboom-insecure://http://internal.host/path")
	if err != nil || !contains(got, "127.0.0.1:7890/insecure-proxy?target=") {
		t.Fatalf("proxy rewrite = %q, %v", got, err)
	}
	for _, target := range []string{"kaboom-insecure://", "kaboom-insecure://ftp://internal.host"} {
		if _, err := h.resolveNavigateURL(target); err == nil {
			t.Fatalf("invalid insecure target accepted: %q", target)
		}
	}
}

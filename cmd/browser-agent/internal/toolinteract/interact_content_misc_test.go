// interact_content_misc_test.go — Tests for content extraction, composable, clipboard, draw, list.
package toolinteract

import (
	"encoding/json"
	"testing"
)

func TestHandleGetReadable(t *testing.T) {
	h, fs := newFakeHandler(t)
	assertOK(t, h.HandleGetReadable(testReq(), json.RawMessage(`{}`)))
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
}

func TestHandleGetMarkdown(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleGetMarkdown(testReq(), json.RawMessage(`{"timeout_ms":40000}`)))
}

func TestHandleContentExtraction_TabBlocked(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.blockTab = true
	assertErr(t, h.HandleContentExtraction(testReq(), json.RawMessage(`{}`), "get_readable", "readable"), ErrNotInitialized)
}

func TestHandleWaitForStable(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleWaitForStable(testReq(), json.RawMessage(`{}`)))
}

func TestHandleWaitForStable_WithParams(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleWaitForStable(testReq(), json.RawMessage(`{"stability_ms":300,"timeout_ms":2000}`)))
}

func TestHandleAutoDismissOverlays(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleAutoDismissOverlays(testReq(), json.RawMessage(`{}`)))
}

func TestQueueComposableHelpers(t *testing.T) {
	h, fs := newFakeHandler(t)
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
	h, fs := newFakeHandler(t)
	assertOK(t, h.HandleClipboardRead(testReq(), json.RawMessage(`{}`)))
	// records clipboard_read on success.
	if fs.recordedCount() != 1 {
		t.Fatalf("expected 1 recorded action, got %d", fs.recordedCount())
	}
}

func TestHandleClipboardWrite(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleClipboardWrite(testReq(), json.RawMessage(`{"text":"copy me"}`)))
}

func TestHandleClipboardWrite_MissingText(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleClipboardWrite(testReq(), json.RawMessage(`{}`)), ErrMissingParam)
}

func TestHandleClipboardWrite_InvalidJSON(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertErr(t, h.HandleClipboardWrite(testReq(), json.RawMessage(`bad`)), ErrInvalidJSON)
}

func TestHandleClipboardRead_PilotBlockedNoRecord(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.blockPilot = true
	assertErr(t, h.HandleClipboardRead(testReq(), json.RawMessage(`{}`)), ErrCodePilotDisabled)
	if fs.recordedCount() != 0 {
		t.Fatalf("blocked clipboard read should not record, got %d", fs.recordedCount())
	}
}

func TestHandleDrawModeStart_Success(t *testing.T) {
	h, fs := newFakeHandler(t)
	resp := h.HandleDrawModeStart(testReq(), json.RawMessage(`{"annot_session":"s1"}`))
	assertOK(t, resp)
	if fs.drawStarted != 1 {
		t.Fatalf("expected MarkDrawStarted called once, got %d", fs.drawStarted)
	}
}

func TestHandleDrawModeStart_NoArgs(t *testing.T) {
	h, _ := newFakeHandler(t)
	assertOK(t, h.HandleDrawModeStart(testReq(), nil))
}

func TestHandleDrawModeStart_ExtensionBlocked(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.blockExt = true
	assertErr(t, h.HandleDrawModeStart(testReq(), json.RawMessage(`{}`)), ErrNotInitialized)
	if fs.drawStarted != 0 {
		t.Fatalf("blocked draw should not mark started, got %d", fs.drawStarted)
	}
}

func TestHandleDrawModeStart_TabBlocked(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.blockTab = true
	assertErr(t, h.HandleDrawModeStart(testReq(), json.RawMessage(`{}`)), ErrNotInitialized)
}

func TestHandleListInteractive_Success(t *testing.T) {
	h, fs := newFakeHandler(t)
	// Return a response with elements so index metadata is built.
	fs.waitFn = func(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse {
		return succeed(req, "list_interactive results", map[string]any{
			"elements": []any{
				map[string]any{"index": float64(0), "selector": "#a", "tag": "input"},
				map[string]any{"index": float64(1), "selector": "#b", "tag": "button"},
			},
		})
	}
	resp := h.HandleListInteractive(testReq(), json.RawMessage(`{}`))
	result := assertOK(t, resp)
	// index_generation should be annotated into the response text.
	if !contains(firstText(result), "index_generation") {
		t.Fatalf("expected index_generation annotation, got: %s", firstText(result))
	}
	// The element index should now resolve.
	sel, ok, _, _ := h.resolveIndexToSelector("client-test", 0, 1, "")
	if !ok || sel != "#b" {
		t.Fatalf("expected #b resolved, got %q ok=%v", sel, ok)
	}
}

func TestHandleListInteractive_Truncation(t *testing.T) {
	h, fs := newFakeHandler(t)
	fs.waitFn = func(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse {
		elems := make([]any, 8)
		for i := range elems {
			elems[i] = map[string]any{"index": float64(i), "selector": "#e", "tag": "div"}
		}
		return succeed(req, "list_interactive results", map[string]any{"elements": elems})
	}
	resp := h.HandleListInteractive(testReq(), json.RawMessage(`{"limit":3}`))
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

func TestFormatIndexGenerationConflict(t *testing.T) {
	if !contains(formatIndexGenerationConflict("", ""), "generation mismatch") {
		t.Fatal("expected generic message")
	}
	if !contains(formatIndexGenerationConflict("old", "new"), "old") {
		t.Fatal("expected message to include expected generation")
	}
}

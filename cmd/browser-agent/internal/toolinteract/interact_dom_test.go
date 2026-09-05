// interact_dom_test.go — DOM primitives, element indexes, and response contract tests.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/elemindex"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/pagescripts"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"strings"
	"testing"
)

func TestHandleDOMPrimitive_ClickSuccess(t *testing.T) {
	h, fs := newFakeDOMActions(t)
	resp := h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"selector":"#go","action":"click"}`), "click")
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", fs.enqueuedCount())
	}
	// RecordDOMPrimitiveAction runs post-enqueue.
	if fs.recordedCount() != 1 {
		t.Fatalf("expected 1 recorded dom action, got %d", fs.recordedCount())
	}
	fs.mu.Lock()
	queryType := fs.enqueued[0].Type
	fs.mu.Unlock()
	if queryType != "dom_action" {
		t.Fatalf("plain click query type = %q, want dom_action", queryType)
	}
}

func TestHandleDOMPrimitive_PreservesStructuredQueryFlag(t *testing.T) {
	h, fs := newFakeDOMActions(t)
	assertOK(t, h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"selector":".accordion","structured":true}`), "get_text"))
	queued := fs.enqueuedSnapshot()
	if len(queued) != 1 || queued[0].Type != "dom_action" || !strings.HasPrefix(queued[0].CorrelationID, "dom_get_text_") {
		t.Fatalf("structured get_text enqueue = %#v", queued)
	}
	var params map[string]any
	if err := json.Unmarshal(queued[0].Params, &params); err != nil {
		t.Fatalf("decode get_text params: %v", err)
	}
	if params["action"] != "get_text" || params["structured"] != true {
		t.Fatalf("structured get_text params = %#v", params)
	}
}

func TestHandleDOMPrimitive_InvalidJSON(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	assertErr(t, h.HandleDOMPrimitive(testReq(), json.RawMessage(`{bad`), "click"), mcp.ErrInvalidJSON)
}

func TestHandleDOMPrimitive_MissingSelector(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	// click requires a selector/element_id/index.
	assertErr(t, h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"action":"click"}`), "click"), mcp.ErrMissingParam)
}

func TestHandleDOMPrimitive_TypeRequiresText(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	// type action requires text field.
	assertErr(t, h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"selector":"#i","action":"type"}`), "type"), mcp.ErrMissingParam)
}

func TestHandleDOMPrimitive_TypeSuccess(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	assertOK(t, h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"selector":"#i","text":"hi","action":"type"}`), "type"))
}

func TestHandleDOMPrimitive_SelectorOptionalActions(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	// open_composer doesn't require a selector.
	assertOK(t, h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"action":"open_composer"}`), "open_composer"))
}

func TestHandleDOMPrimitiveActionFamilyContracts(t *testing.T) {
	t.Parallel()
	invalid := []struct {
		action, args, missing string
	}{
		{"click", `{}`, "selector"},
		{"check", `{}`, "selector"},
		{"get_text", `{}`, "selector"},
		{"get_value", `{}`, "selector"},
		{"focus", `{}`, "selector"},
		{"scroll_to", `{}`, "selector"},
		{"hover", `{}`, "selector"},
		{"wait_for", `{}`, "selector"},
		{"type", `{"selector":"input"}`, "text"},
		{"paste", `{"selector":"input"}`, "text"},
		{"paste", `{"text":"hello"}`, "selector"},
		{"select", `{"selector":"select"}`, "value"},
		{"get_attribute", `{"selector":"a"}`, "name"},
		{"set_attribute", `{"selector":"div"}`, "name"},
	}
	for _, testCase := range invalid {
		t.Run("reject "+testCase.action, func(t *testing.T) {
			h, _ := newFakeDOMActions(t)
			response := h.HandleDOMPrimitive(testReq(), json.RawMessage(testCase.args), testCase.action)
			assertErr(t, response, mcp.ErrMissingParam)
			if !strings.Contains(firstText(parseToolResult(t, response)), testCase.missing) {
				t.Fatalf("%s error omitted %q: %s", testCase.action, testCase.missing, firstText(parseToolResult(t, response)))
			}
		})
	}

	valid := []struct{ action, args string }{
		{"click", `{"selector":"#btn"}`},
		{"type", `{"selector":"input","text":"hello"}`},
		{"select", `{"selector":"select","value":"one"}`},
		{"check", `{"selector":"input"}`},
		{"get_text", `{"selector":"div"}`},
		{"get_value", `{"selector":"input"}`},
		{"get_attribute", `{"selector":"a","name":"href"}`},
		{"set_attribute", `{"selector":"div","name":"data-test","value":"1"}`},
		{"focus", `{"selector":"input"}`},
		{"scroll_to", `{"selector":"footer"}`},
		{"hover", `{"selector":"a"}`},
		{"paste", `{"selector":"input","text":"hello"}`},
		{"wait_for", `{"selector":"spinner"}`},
		{"key_press", `{"selector":"input","text":"Enter"}`},
		{"open_composer", `{}`},
		{"submit_active_composer", `{}`},
		{"confirm_top_dialog", `{}`},
		{"dismiss_top_overlay", `{}`},
	}
	for _, testCase := range valid {
		t.Run("queue "+testCase.action, func(t *testing.T) {
			h, state := newFakeDOMActions(t)
			assertOK(t, h.HandleDOMPrimitive(testReq(), json.RawMessage(testCase.args), testCase.action))
			queued := state.enqueuedSnapshot()
			if len(queued) != 1 || queued[0].Type != "dom_action" || !strings.HasPrefix(queued[0].CorrelationID, "dom_") || !strings.Contains(string(queued[0].Params), `"action":"`+testCase.action+`"`) {
				t.Fatalf("%s enqueue = %#v", testCase.action, queued)
			}
			blocked, blockedState := newFakeDOMActions(t)
			blockedState.blockPilot = true
			assertErr(t, blocked.HandleDOMPrimitive(testReq(), json.RawMessage(testCase.args), testCase.action), mcp.ErrCodePilotDisabled)
		})
	}
}

func TestHandleDOMPrimitive_ClickWithCoordsRoutesCDP(t *testing.T) {
	h, fs := newFakeDOMActions(t)
	resp := h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"x":10,"y":20,"action":"click"}`), "click")
	assertOK(t, resp)
	if fs.enqueuedCount() != 1 {
		t.Fatalf("expected 1 enqueue via CDP click, got %d", fs.enqueuedCount())
	}
	// The enqueued query should be a cdp_action.
	fs.mu.Lock()
	qType := fs.enqueued[0].Type
	fs.mu.Unlock()
	if qType != "cdp_action" {
		t.Fatalf("expected cdp_action query, got %q", qType)
	}
}

func TestHandleDOMPrimitive_IndexResolvesToSelector(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	h.elementIndexRegistry.Store("client-test", 0, "gen_1", map[int]elemindex.Target{2: {Selector: "#resolved"}})
	resp := h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"index":2,"action":"click"}`), "click")
	assertOK(t, resp)
}

func TestHandleDOMPrimitive_IndexNotFound(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	resp := h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"index":9,"action":"click"}`), "click")
	assertErr(t, resp, mcp.ErrInvalidParam)
}

func TestHandleHardwareClick_Success(t *testing.T) {
	h, fs := newFakeDOMActions(t)
	response := h.HandleHardwareClick(testReq(), json.RawMessage(`{"x":5,"y":6}`))
	assertOK(t, response)
	if !strings.Contains(string(response.Result), "cdp_click_") {
		t.Fatalf("hardware click response missing correlation prefix: %s", response.Result)
	}
	fs.mu.Lock()
	queryType := fs.enqueued[0].Type
	fs.mu.Unlock()
	if queryType != "cdp_action" {
		t.Fatalf("hardware click query type = %q, want cdp_action", queryType)
	}
}

func TestHandleHardwareClick_MissingX(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	assertErr(t, h.HandleHardwareClick(testReq(), json.RawMessage(`{"y":6}`)), mcp.ErrMissingParam)
}

func TestHandleHardwareClick_MissingY(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	assertErr(t, h.HandleHardwareClick(testReq(), json.RawMessage(`{"x":6}`)), mcp.ErrMissingParam)
}

func TestHandleHardwareClick_InvalidJSON(t *testing.T) {
	h, _ := newFakeDOMActions(t)
	assertErr(t, h.HandleHardwareClick(testReq(), json.RawMessage(`bad`)), mcp.ErrInvalidJSON)
}

func TestHandleCDPClick_PilotBlocked(t *testing.T) {
	h, fs := newFakeDOMActions(t)
	fs.blockPilot = true
	assertErr(t, h.HandleCDPClick(testReq(), json.RawMessage(`{}`), "hardware_click", cdpClickTarget{X: 1, Y: 2}), mcp.ErrCodePilotDisabled)
}

func TestNormalizeDOMActionArgs_SetsAction(t *testing.T) {
	out := normalizeDOMActionArgs(json.RawMessage(`{"selector":"#a"}`), "click")
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["action"] != "click" {
		t.Fatalf("expected action=click, got %v", m["action"])
	}
}

func TestNormalizeDOMActionArgs_NearToScopeRect(t *testing.T) {
	out := normalizeDOMActionArgs(json.RawMessage(`{"near_x":100,"near_y":200,"near_radius":10}`), "click")
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	rect, ok := m["scope_rect"].(map[string]any)
	if !ok {
		t.Fatalf("expected scope_rect derived from near_*, got %v", m["scope_rect"])
	}
	if rect["x"].(float64) != 90 || rect["width"].(float64) != 20 {
		t.Fatalf("unexpected rect: %v", rect)
	}
}

func TestNormalizeDOMActionArgs_PreservesScopeRect(t *testing.T) {
	out := normalizeDOMActionArgs(json.RawMessage(`{"scope_rect":{"x":1}}`), "click")
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if _, ok := m["scope_rect"]; !ok {
		t.Fatal("expected canonical scope_rect")
	}
}

func TestNormalizeDOMActionArgs_InvalidJSONFallsBack(t *testing.T) {
	out := normalizeDOMActionArgs(json.RawMessage(`not json`), "click")
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["action"] != "click" {
		t.Fatalf("expected action injected on invalid json path, got %v", m)
	}
}

func TestToFloat64(t *testing.T) {
	cases := []struct {
		in   any
		want float64
		ok   bool
	}{
		{float64(3.5), 3.5, true},
		{int(7), 7, true},
		{json.Number("2.5"), 2.5, true},
		{"nope", 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		got, ok := toFloat64(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("toFloat64(%v) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestValidateWaitForConditions(t *testing.T) {
	// No conditions => error.
	if _, failed := validateWaitForConditions(testReq(), "wait_for", DOMPrimitiveParams{}); !failed {
		t.Error("expected failure with no conditions")
	}
	// Single selector => ok.
	if _, failed := validateWaitForConditions(testReq(), "wait_for", DOMPrimitiveParams{Selector: "#a"}); failed {
		t.Error("expected ok with single selector condition")
	}
	// Mutually exclusive => error.
	if _, failed := validateWaitForConditions(testReq(), "wait_for", DOMPrimitiveParams{Selector: "#a", Text: "x"}); !failed {
		t.Error("expected failure with multiple conditions")
	}
	// absent without selector => error.
	if _, failed := validateWaitForConditions(testReq(), "wait_for", DOMPrimitiveParams{Absent: true}); !failed {
		t.Error("expected failure with absent and no selector")
	}
	// Non wait_for action => ok.
	if _, failed := validateWaitForConditions(testReq(), "click", DOMPrimitiveParams{}); failed {
		t.Error("expected no validation for non-wait_for action")
	}
}

func TestValidateDOMActionParams(t *testing.T) {
	tests := []struct {
		name, action, text, value, attribute string
		wantFailure                          bool
	}{
		{"click", "click", "", "", "", false},
		{"check", "check", "", "", "", false},
		{"focus", "focus", "", "", "", false},
		{"scroll", "scroll_to", "", "", "", false},
		{"wait", "wait_for", "", "", "", false},
		{"key", "key_press", "", "", "", false},
		{"type missing", "type", "", "", "", true},
		{"type", "type", "hello", "", "", false},
		{"select missing", "select", "", "", "", true},
		{"select", "select", "", "opt", "", false},
		{"attribute missing", "get_attribute", "", "", "", true},
		{"attribute", "get_attribute", "", "", "href", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, failed := ValidateDOMActionParams(testReq(), tc.action, tc.text, tc.value, tc.attribute)
			if failed != tc.wantFailure {
				t.Fatalf("ValidateDOMActionParams(%q) failed=%v, want %v", tc.action, failed, tc.wantFailure)
			}
		})
	}
}

func TestUpdateArgsFields(t *testing.T) {
	out := updateArgsFields(json.RawMessage(`{"a":1}`), map[string]any{"selector": "#new"})
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["selector"] != "#new" || m["a"] != float64(1) {
		t.Fatalf("expected selector injected beside existing args, got %v", m)
	}
	// A ref resolves to a point rather than a selector, so both fields must survive one call.
	out = updateArgsFields(json.RawMessage(`{"a":1}`), map[string]any{"x": 12.5, "y": 30.0})
	_ = json.Unmarshal(out, &m)
	if m["x"] != 12.5 || m["y"] != float64(30) {
		t.Fatalf("expected x and y injected, got %v", m)
	}
	// Invalid JSON returns args unchanged.
	same := updateArgsFields(json.RawMessage(`bad`), map[string]any{"selector": "#x"})
	if string(same) != "bad" {
		t.Fatalf("expected unchanged on invalid json, got %s", same)
	}
}

func TestParseDOMPrimitiveParams(t *testing.T) {
	p, err := ParseDOMPrimitiveParams(json.RawMessage(`{"selector":"#a","tab_id":3,"nth":-1,"direction":"bottom","structured":true}`))
	if err != nil || p.Selector != "#a" || p.TabID != 3 || p.Nth == nil || *p.Nth != -1 || p.Direction != "bottom" || !p.Structured {
		t.Fatalf("unexpected parse: %+v err=%v", p, err)
	}
	if _, err := ParseDOMPrimitiveParams(json.RawMessage(`bad`)); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := ParseDOMPrimitiveParams(json.RawMessage(`{"nth":1.5}`)); err == nil {
		t.Fatal("expected fractional nth to be rejected")
	}
}

func TestWaitForConditions_URLContains(t *testing.T) {
	if _, failed := validateWaitForConditions(testReq(), "wait_for", DOMPrimitiveParams{URLContains: "/done"}); failed {
		t.Error("expected url_contains-only wait to pass validation")
	}
}

func TestResolveIndexToSelector_Empty(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	_, ok, _, _ := h.resolveIndexToTarget("client-a", 0, 0, "")
	if ok {
		t.Error("expected not found on empty store")
	}
}

func TestResolveIndexToSelector_AfterBuild(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	// Manually populate the store
	h.elementIndexRegistry.Store("client-a", 0, "gen_1", map[int]elemindex.Target{
		0: {Selector: "#email"},
		1: {Selector: "#password"},
		2: {Selector: "button[type=submit]"},
	})

	target, ok, _, _ := h.resolveIndexToTarget("client-a", 0, 1, "")
	if !ok || target.Selector != "#password" {
		t.Errorf("expected #password, got %q (ok=%v)", target.Selector, ok)
	}

	_, ok, _, _ = h.resolveIndexToTarget("client-a", 0, 99, "")
	if ok {
		t.Error("expected not found for missing index")
	}
}

func TestResolveIndexToSelector_ScopedByClientAndTab(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	h.elementIndexRegistry.Store("client-a", 0, "gen_a", map[int]elemindex.Target{1: {Selector: "#a"}})
	h.elementIndexRegistry.Store("client-b", 0, "gen_b", map[int]elemindex.Target{1: {Selector: "#b"}})
	h.elementIndexRegistry.Store("client-a", 9, "gen_a9", map[int]elemindex.Target{1: {Selector: "#a9"}})

	target, ok, _, _ := h.resolveIndexToTarget("client-a", 0, 1, "")
	if !ok || target.Selector != "#a" {
		t.Fatalf("client-a/tab0 selector=%q ok=%v, want #a/true", target.Selector, ok)
	}
	target, ok, _, _ = h.resolveIndexToTarget("client-b", 0, 1, "")
	if !ok || target.Selector != "#b" {
		t.Fatalf("client-b/tab0 selector=%q ok=%v, want #b/true", target.Selector, ok)
	}
	target, ok, _, _ = h.resolveIndexToTarget("client-a", 9, 1, "")
	if !ok || target.Selector != "#a9" {
		t.Fatalf("client-a/tab9 selector=%q ok=%v, want #a9/true", target.Selector, ok)
	}
}

func TestResolveIndexToSelector_GenerationMismatch(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	h.elementIndexRegistry.Store("client-a", 0, "gen_new", map[int]elemindex.Target{1: {Selector: "#a"}})

	_, ok, stale, latest := h.resolveIndexToTarget("client-a", 0, 1, "gen_old")
	if ok {
		t.Fatal("expected no selector on generation mismatch")
	}
	if !stale {
		t.Fatal("expected stale=true on generation mismatch")
	}
	if latest != "gen_new" {
		t.Fatalf("latest generation=%q, want gen_new", latest)
	}
}

func TestHandleDOMPrimitive_IndexGenerationMismatch(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	h.elementIndexRegistry.Store("client-a", 7, "gen_new", map[int]elemindex.Target{1: {Selector: "#submit"}})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), ClientID: "client-a"}
	resp := h.HandleDOMPrimitive(req, json.RawMessage(`{"index":1,"tab_id":7,"index_generation":"gen_old"}`), "click")
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatalf("expected error response, got: %s", firstText(result))
	}
	text := strings.ToLower(firstText(result))
	if !strings.Contains(text, "index_generation") || !strings.Contains(text, "mismatch") {
		t.Fatalf("expected index_generation mismatch guidance, got: %s", firstText(result))
	}
}

func TestBuildElementIndexFromResponse_ValidElements(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	// Simulate a list_interactive response with elements in the JSON
	elemJSON, _ := json.Marshal(map[string]any{"elements": []any{
		map[string]any{"index": float64(0), "selector": "#name", "tag": "input"},
		map[string]any{"index": float64(1), "selector": ".btn-submit", "tag": "button"},
	}})
	resultJSON, _ := json.Marshal(mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "list_interactive results\n" + string(elemJSON)}},
	})
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.buildElementIndexFromResponse("client-a", 0, "gen_1", resp)

	target, ok, _, _ := h.resolveIndexToTarget("client-a", 0, 0, "")
	if !ok || target.Selector != "#name" {
		t.Errorf("index 0: expected #name, got %q (ok=%v)", target.Selector, ok)
	}

	target, ok, _, _ = h.resolveIndexToTarget("client-a", 0, 1, "")
	if !ok || target.Selector != ".btn-submit" {
		t.Errorf("index 1: expected .btn-submit, got %q (ok=%v)", target.Selector, ok)
	}
}

func TestBuildElementIndexFromResponse_ErrorResponse(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	// Pre-populate store
	h.elementIndexRegistry.Store("client-a", 0, "gen_1", map[int]elemindex.Target{0: {Selector: "old"}})

	// Error response should not clear the store
	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "error"}},
		IsError: true,
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.buildElementIndexFromResponse("client-a", 0, "gen_2", resp)

	target, ok, _, _ := h.resolveIndexToTarget("client-a", 0, 0, "")
	if !ok || target.Selector != "old" {
		t.Errorf("error response should not clear store, got %q (ok=%v)", target.Selector, ok)
	}
}

func TestAnnotateListInteractiveIndexMetadata_PreservesPrefixAndSkipsMalformedBlocks(t *testing.T) {
	t.Parallel()

	result := mcp.MCPToolResult{Content: []mcp.MCPContentBlock{
		{Type: "text", Text: "not JSON"},
		{Type: "text", Text: "list_interactive results\n{\"elements\":[]}"},
	}}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	annotated := annotateListInteractiveIndexMetadata(resp, 7, "gen_7")
	var got mcp.MCPToolResult
	if err := json.Unmarshal(annotated.Result, &got); err != nil {
		t.Fatalf("decode annotated result: %v", err)
	}
	const prefix = "list_interactive results\n"
	if !strings.HasPrefix(got.Content[1].Text, prefix) {
		t.Fatalf("result prefix changed: %q", got.Content[1].Text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(got.Content[1].Text, prefix)), &data); err != nil {
		t.Fatalf("decode annotated payload: %v", err)
	}
	if data["index_generation"] != "gen_7" || data["index_scope_tab_id"] != float64(7) {
		t.Fatalf("metadata = %#v", data)
	}
}

// ============================================
// List Interactive Limit/Truncation Tests
// ============================================

func TestTruncateListInteractiveResponse_Basic(t *testing.T) {
	t.Parallel()

	elements := make([]any, 20)
	for i := range elements {
		elements[i] = map[string]any{"index": float64(i), "selector": "#el-" + string(rune('a'+i)), "tag": "div"}
	}
	elemData := map[string]any{"elements": elements}
	elemJSON, _ := json.Marshal(elemData)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "list_interactive results\n" + string(elemJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	truncated := truncateListInteractiveResponse(resp, 5)

	// Parse truncated response
	var truncResult mcp.MCPToolResult
	if err := json.Unmarshal(truncated.Result, &truncResult); err != nil {
		t.Fatal(err)
	}

	text := truncResult.Content[0].Text
	idx := 0
	for i, ch := range text {
		if ch == '{' {
			idx = i
			break
		}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse truncated JSON: %v", err)
	}

	elems, ok := data["elements"].([]any)
	if !ok {
		t.Fatal("elements not found in truncated response")
	}
	if len(elems) != 5 {
		t.Errorf("expected 5 elements, got %d", len(elems))
	}
	if data["total"] != float64(20) {
		t.Errorf("total = %v, want 20", data["total"])
	}
	if data["truncated"] != true {
		t.Errorf("truncated = %v, want true", data["truncated"])
	}
}

func TestTruncateListInteractiveResponse_NoTruncationNeeded(t *testing.T) {
	t.Parallel()

	elemData := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#a"},
			map[string]any{"index": float64(1), "selector": "#b"},
		},
	}
	elemJSON, _ := json.Marshal(elemData)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: string(elemJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	truncated := truncateListInteractiveResponse(resp, 10)

	// Should be unchanged
	if string(truncated.Result) != string(resp.Result) {
		t.Error("response should be unchanged when limit > element count")
	}
}

func TestTruncateListInteractiveResponse_ErrorResponse(t *testing.T) {
	t.Parallel()

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "error"}},
		IsError: true,
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	truncated := truncateListInteractiveResponse(resp, 5)
	if string(truncated.Result) != string(resp.Result) {
		t.Error("error response should be unchanged")
	}
}

// newTestHandler creates a minimal DOM owner for element-index tests.
func newTestHandler() *DOMActions {
	return NewDOMActions(NewActionRuntime(RuntimeDeps{}), DOMDeps{})
}

// parseToolResult is a test helper that unmarshals a mcp.JSONRPCResponse result into mcp.MCPToolResult.
func parseToolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}
	return result
}

// firstText extracts the first text content from a mcp.MCPToolResult.
func firstText(result mcp.MCPToolResult) string {
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text
		}
	}
	return ""
}

// The clipboard read must queue the embedded page script verbatim. An inline copy
// would drift from the Node fixtures that prove the bounded permission, focus,
// navigation, and context-destruction outcomes.
func TestClipboardReadQueuesTheBoundedPageScript(t *testing.T) {
	handler, state := newFakePageActions(t)
	assertOK(t, handler.HandleClipboardRead(testReq(), json.RawMessage(`{}`)))

	queued := state.enqueuedSnapshot()
	if len(queued) != 1 || queued[0].Type != "execute" || !strings.HasPrefix(queued[0].CorrelationID, "exec_") {
		t.Fatalf("clipboard read enqueue = %#v", queued)
	}
	var params map[string]any
	if err := json.Unmarshal(queued[0].Params, &params); err != nil {
		t.Fatalf("decode clipboard read params: %v", err)
	}
	if params["script"] != pagescripts.ClipboardRead {
		t.Fatalf("clipboard read must queue the embedded page script, got %v", params["script"])
	}
	if params["world"] != "main" || params["reason"] != "clipboard_read" {
		t.Fatalf("clipboard read routing = %#v", params)
	}
	// The page script's own deadline must fit inside the executor budget, or the
	// generic execution_timeout wins the race and the classification is lost.
	timeout, ok := params["timeout_ms"].(float64)
	if !ok || timeout <= 2000 {
		t.Fatalf("clipboard read timeout_ms = %v, want a budget above the page deadline", params["timeout_ms"])
	}
}

func TestClipboardReadNeverEnqueuesWhenGuardsBlock(t *testing.T) {
	handler, state := newFakePageActions(t)
	state.blockPilot = true
	assertErr(t, handler.HandleClipboardRead(testReq(), json.RawMessage(`{}`)), mcp.ErrCodePilotDisabled)
	if state.enqueuedCount() != 0 {
		t.Fatalf("blocked clipboard read enqueued %d queries", state.enqueuedCount())
	}
	if state.recordedCount() != 0 {
		t.Fatalf("blocked clipboard read recorded %d AI actions", state.recordedCount())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Accessibility refs share the element index's generation rule (kaboom-05ue.10).
// ─────────────────────────────────────────────────────────────────────────────

// A ref taken before a re-render must be refused after one, not resolved against the new
// snapshot. Both snapshots hold backend id 412 at different coordinates — Chrome reusing a
// backendNodeId — so without the generation check the stale ref would click 900,900.
func TestHandleDOMPrimitive_StaleRefIsRefusedFreshRefClicks(t *testing.T) {
	h, fs := newFakeDOMActions(t)
	h.elementIndexRegistry.Store("client-test", 0, "gen_old", map[int]elemindex.Target{
		0: {AXBackendID: 412, Role: "button", Name: "Add to cart", CenterX: 100, CenterY: 200, HasCenter: true},
	})

	// Control: quoting the generation the ref was issued under DOES click, at the candidate's
	// own coordinates and through the coordinate click path.
	assertOK(t, h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"ref":"ax_412","index_generation":"gen_old"}`), "click"))
	queued := fs.enqueuedSnapshot()
	if len(queued) != 1 || queued[0].Type != "cdp_action" {
		t.Fatalf("fresh ref enqueue = %#v, want one cdp_action", queued)
	}
	var params map[string]any
	if json.Unmarshal(queued[0].Params, &params) != nil || params["x"] != float64(100) || params["y"] != float64(200) {
		t.Fatalf("fresh ref click params = %s, want a click at (100,200)", queued[0].Params)
	}

	// The page re-renders. backendNodeId 412 now names a different control.
	h.elementIndexRegistry.Store("client-test", 0, "gen_new", map[int]elemindex.Target{
		0: {AXBackendID: 412, Role: "button", Name: "Delete account", CenterX: 900, CenterY: 900, HasCenter: true},
	})

	resp := h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"ref":"ax_412","index_generation":"gen_old"}`), "click")
	assertErr(t, resp, mcp.ErrInvalidParam)
	text := firstText(parseToolResult(t, resp))
	if !strings.Contains(text, "generation mismatch") || !strings.Contains(text, "gen_new") {
		t.Fatalf("stale ref refusal = %s, want a generation conflict naming gen_new", text)
	}
	if fs.enqueuedCount() != 1 {
		t.Fatalf("stale ref enqueued %d commands; a refused ref must dispatch nothing", fs.enqueuedCount())
	}
}

func TestHandleDOMPrimitive_RefWithNoActionablePointIsRefused(t *testing.T) {
	h, fs := newFakeDOMActions(t)
	h.elementIndexRegistry.Store("client-test", 0, "gen_1", map[int]elemindex.Target{
		0: {AXBackendID: 412, CenterX: 10, CenterY: 20, HasCenter: true},
		// find could not read this candidate's box, so it has no point to click.
		1: {AXBackendID: 413, Role: "button", Name: "Collapsed"},
	})

	// Control: ax_412 IS in this snapshot, has a point, and clicks.
	assertOK(t, h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"ref":"ax_412"}`), "click"))

	for _, ref := range []string{"ax_999", "ax_413"} {
		resp := h.HandleDOMPrimitive(testReq(), json.RawMessage(`{"ref":"`+ref+`"}`), "click")
		assertErr(t, resp, mcp.ErrInvalidParam)
		if !strings.Contains(firstText(parseToolResult(t, resp)), ref) {
			t.Fatalf("refusal for %s = %s, want the ref named", ref, firstText(parseToolResult(t, resp)))
		}
	}
	if fs.enqueuedCount() != 1 {
		t.Fatalf("enqueued %d commands, want only the control click", fs.enqueuedCount())
	}
}

// find must publish its candidates into the same generation-stamped index, or its refs stay
// a second handle space with nothing to catch a stale one.
func TestHandleFind_StampsCandidatesIntoTheElementIndex(t *testing.T) {
	h, fs := newFakePageActions(t)
	fs.waitFn = func(req mcp.JSONRPCRequest, _ string, _ json.RawMessage, _ string) mcp.JSONRPCResponse {
		return mcp.Succeed(req, "find results", map[string]any{
			"success": true, "action": "find", "match_count": 1,
			"candidates": []any{map[string]any{
				"ref": "ax_412", "role": "button", "name": "Add to cart",
				"x": float64(300), "y": float64(400), "width": float64(0), "height": float64(0),
			}},
		})
	}

	text := firstText(assertOK(t, h.HandleFind(testReq(), json.RawMessage(`{"query":"add to cart"}`))))
	payloadStart := strings.Index(text, "{")
	var data map[string]any
	if payloadStart < 0 || json.Unmarshal([]byte(text[payloadStart:]), &data) != nil {
		t.Fatalf("find response carries no JSON payload: %s", text)
	}
	generation, _ := data["index_generation"].(string)
	candidates, _ := data["candidates"].([]any)
	if generation == "" || len(candidates) != 1 {
		t.Fatalf("find payload = %#v, want one candidate and an index_generation", data)
	}
	if first, _ := candidates[0].(map[string]any); first["index"] != float64(0) {
		t.Fatalf("candidate 0 = %#v, want index 0 so the caller can quote it", first)
	}

	target, ok, stale, _ := h.dom.elementIndexRegistry.ResolveRef("client-test", 0, "ax_412", generation)
	if !ok || stale {
		t.Fatalf("ResolveRef after find = ok=%v stale=%v, want the candidate resolved", ok, stale)
	}
	if target.CenterX != 300 || target.CenterY != 400 || target.Name != "Add to cart" {
		t.Fatalf("stamped target = %+v, want the candidate find returned", target)
	}
	// Control for the refusal: the SAME ref against a generation find never issued is refused.
	if _, ok, stale, _ := h.dom.elementIndexRegistry.ResolveRef("client-test", 0, "ax_412", "gen_never"); ok || !stale {
		t.Fatalf("ref under a foreign generation = ok=%v stale=%v, want refused as stale", ok, stale)
	}
}

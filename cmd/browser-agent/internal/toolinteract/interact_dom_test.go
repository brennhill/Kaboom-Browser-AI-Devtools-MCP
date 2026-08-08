// interact_dom_test.go — DOM primitives, element indexes, and response contract tests.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/pagescripts"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
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
	h.elementIndexRegistry.Store("client-test", 0, "gen_1", map[int]string{2: "#resolved"})
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
	assertErr(t, h.HandleCDPClick(testReq(), json.RawMessage(`{}`), "hardware_click", 1, 2, 0), mcp.ErrCodePilotDisabled)
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

func TestUpdateArgsSelector(t *testing.T) {
	out := updateArgsSelector(json.RawMessage(`{"a":1}`), "#new")
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["selector"] != "#new" {
		t.Fatalf("expected selector injected, got %v", m["selector"])
	}
	// Invalid JSON returns args unchanged.
	same := updateArgsSelector(json.RawMessage(`bad`), "#x")
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
	_, ok, _, _ := h.resolveIndexToSelector("client-a", 0, 0, "")
	if ok {
		t.Error("expected not found on empty store")
	}
}

func TestResolveIndexToSelector_AfterBuild(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	// Manually populate the store
	h.elementIndexRegistry.Store("client-a", 0, "gen_1", map[int]string{
		0: "#email",
		1: "#password",
		2: "button[type=submit]",
	})

	sel, ok, _, _ := h.resolveIndexToSelector("client-a", 0, 1, "")
	if !ok || sel != "#password" {
		t.Errorf("expected #password, got %q (ok=%v)", sel, ok)
	}

	_, ok, _, _ = h.resolveIndexToSelector("client-a", 0, 99, "")
	if ok {
		t.Error("expected not found for missing index")
	}
}

func TestResolveIndexToSelector_ScopedByClientAndTab(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	h.elementIndexRegistry.Store("client-a", 0, "gen_a", map[int]string{1: "#a"})
	h.elementIndexRegistry.Store("client-b", 0, "gen_b", map[int]string{1: "#b"})
	h.elementIndexRegistry.Store("client-a", 9, "gen_a9", map[int]string{1: "#a9"})

	sel, ok, _, _ := h.resolveIndexToSelector("client-a", 0, 1, "")
	if !ok || sel != "#a" {
		t.Fatalf("client-a/tab0 selector=%q ok=%v, want #a/true", sel, ok)
	}
	sel, ok, _, _ = h.resolveIndexToSelector("client-b", 0, 1, "")
	if !ok || sel != "#b" {
		t.Fatalf("client-b/tab0 selector=%q ok=%v, want #b/true", sel, ok)
	}
	sel, ok, _, _ = h.resolveIndexToSelector("client-a", 9, 1, "")
	if !ok || sel != "#a9" {
		t.Fatalf("client-a/tab9 selector=%q ok=%v, want #a9/true", sel, ok)
	}
}

func TestResolveIndexToSelector_GenerationMismatch(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	h.elementIndexRegistry.Store("client-a", 0, "gen_new", map[int]string{1: "#a"})

	_, ok, stale, latest := h.resolveIndexToSelector("client-a", 0, 1, "gen_old")
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
	h.elementIndexRegistry.Store("client-a", 7, "gen_new", map[int]string{1: "#submit"})

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
	elemData := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#name", "tag": "input"},
			map[string]any{"index": float64(1), "selector": ".btn-submit", "tag": "button"},
			map[string]any{"index": float64(2), "selector": "", "tag": "div"}, // empty selector, should be skipped
		},
	}
	elemJSON, _ := json.Marshal(elemData)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "list_interactive results\n" + string(elemJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.buildElementIndexFromResponse("client-a", 0, "gen_1", resp)

	sel, ok, _, _ := h.resolveIndexToSelector("client-a", 0, 0, "")
	if !ok || sel != "#name" {
		t.Errorf("index 0: expected #name, got %q (ok=%v)", sel, ok)
	}

	sel, ok, _, _ = h.resolveIndexToSelector("client-a", 0, 1, "")
	if !ok || sel != ".btn-submit" {
		t.Errorf("index 1: expected .btn-submit, got %q (ok=%v)", sel, ok)
	}

	// Index 2 had empty selector, should not be stored
	_, ok, _, _ = h.resolveIndexToSelector("client-a", 0, 2, "")
	if ok {
		t.Error("index 2 with empty selector should not be stored")
	}
}

func TestBuildElementIndexFromResponse_NestedResult(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	// Elements nested under result.result.elements
	nestedData := map[string]any{
		"result": map[string]any{
			"result": map[string]any{
				"elements": []any{
					map[string]any{"index": float64(0), "selector": "a.link"},
				},
			},
		},
	}
	nestedJSON, _ := json.Marshal(nestedData)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: string(nestedJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.buildElementIndexFromResponse("client-a", 0, "gen_1", resp)

	sel, ok, _, _ := h.resolveIndexToSelector("client-a", 0, 0, "")
	if !ok || sel != "a.link" {
		t.Errorf("expected a.link from nested result, got %q (ok=%v)", sel, ok)
	}
}

func TestBuildElementIndexFromResponse_ErrorResponse(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	// Pre-populate store
	h.elementIndexRegistry.Store("client-a", 0, "gen_1", map[int]string{0: "old"})

	// Error response should not clear the store
	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "error"}},
		IsError: true,
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.buildElementIndexFromResponse("client-a", 0, "gen_2", resp)

	sel, ok, _, _ := h.resolveIndexToSelector("client-a", 0, 0, "")
	if !ok || sel != "old" {
		t.Errorf("error response should not clear store, got %q (ok=%v)", sel, ok)
	}
}

func TestExtractElementList_Direct(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#a"},
		},
	}
	elems := act.ExtractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestExtractElementList_Nested(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"result": map[string]any{
			"elements": []any{
				map[string]any{"index": float64(0), "selector": "#b"},
			},
		},
	}
	elems := act.ExtractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element from nested, got %d", len(elems))
	}
}

func TestExtractElementList_NoElements(t *testing.T) {
	t.Parallel()
	data := map[string]any{"foo": "bar"}
	elems := act.ExtractElementList(data)
	if elems != nil {
		t.Error("expected nil for data without elements")
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

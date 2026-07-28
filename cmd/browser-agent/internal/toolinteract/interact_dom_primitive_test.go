// interact_dom_primitive_test.go — Tests for DOM primitive dispatch, CDP click, and validation.
package toolinteract

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
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
	h, _ := newFakeDOMActions(t)
	assertOK(t, h.HandleHardwareClick(testReq(), json.RawMessage(`{"x":5,"y":6}`)))
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
	// select requires value.
	if _, failed := ValidateDOMActionParams(testReq(), "select", "", "", ""); !failed {
		t.Error("expected select to require value")
	}
	if _, failed := ValidateDOMActionParams(testReq(), "select", "", "opt", ""); failed {
		t.Error("expected select with value to pass")
	}
	// Unknown action has no required params.
	if _, failed := ValidateDOMActionParams(testReq(), "click", "", "", ""); failed {
		t.Error("expected click to have no required params")
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
	p, err := ParseDOMPrimitiveParams(json.RawMessage(`{"selector":"#a","tab_id":3}`))
	if err != nil || p.Selector != "#a" || p.TabID != 3 {
		t.Fatalf("unexpected parse: %+v err=%v", p, err)
	}
	if _, err := ParseDOMPrimitiveParams(json.RawMessage(`bad`)); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestWaitForConditions_URLContains(t *testing.T) {
	if _, failed := validateWaitForConditions(testReq(), "wait_for", DOMPrimitiveParams{URLContains: "/done"}); failed {
		t.Error("expected url_contains-only wait to pass validation")
	}
}

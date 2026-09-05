// gestures_test.go — Pointer gesture and clipped-capture contracts (kaboom-05ue.5).
//
// Every rule here stops a call that would otherwise reach the page and do nothing visible:
// a one-point drag path drags nothing, a scroll_at with no delta scrolls zero pixels, and a
// right_click with neither selector nor coordinate has no target at all. Each of those would
// come back as a success.
// Docs: docs/features/feature/interact-explore/index.md

package contracts_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema/interact"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// track marks a tab as tracked so the gestures reach the queue instead of the tracking gate.
func track(fixture *gateFixture) {
	capturefixture.Track(fixture.capture, 42, "https://example.test")
}

func TestValidateGestureRejectsCallsThatWouldSilentlyDoNothing(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		action string
		target act.GestureTarget
		param  string
	}{
		{name: "drag with no route", action: "drag", param: "drag_path"},
		{name: "drag with one point", action: "drag", target: act.GestureTarget{PathPoints: 1}, param: "drag_path"},
		{name: "hover_at with no coordinate", action: "hover_at", param: "x"},
		{name: "hover_at with only x", action: "hover_at", target: act.GestureTarget{HasX: true}, param: "x"},
		{name: "scroll_at with no delta", action: "scroll_at", target: act.GestureTarget{HasX: true, HasY: true}, param: "delta_y"},
		{name: "right_click with no target at all", action: "right_click", param: "selector"},
		{name: "double_click with half a coordinate", action: "double_click", target: act.GestureTarget{HasX: true}, param: "selector"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			problem := act.ValidateGesture(testCase.action, testCase.target)
			if problem == nil {
				t.Fatalf("%s accepted a call that cannot act", testCase.action)
			}
			if problem.Param != testCase.param {
				t.Fatalf("param = %q, want %q", problem.Param, testCase.param)
			}
			if problem.Retry == "" {
				t.Fatal("a rejection must say what to send instead")
			}
		})
	}
}

func TestValidateGestureAcceptsEveryWayOfNamingATarget(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		action string
		target act.GestureTarget
	}{
		{name: "drag route", action: "drag", target: act.GestureTarget{PathPoints: 2}},
		{name: "right_click by selector", action: "right_click", target: act.GestureTarget{Selector: "#row"}},
		{name: "right_click by element handle", action: "right_click", target: act.GestureTarget{ElementID: "e7"}},
		{name: "right_click by index", action: "right_click", target: act.GestureTarget{HasIndex: true}},
		{name: "triple_click by coordinate", action: "triple_click", target: act.GestureTarget{HasX: true, HasY: true}},
		{name: "hover_at by coordinate", action: "hover_at", target: act.GestureTarget{HasX: true, HasY: true}},
		{name: "scroll_at with a vertical delta", action: "scroll_at", target: act.GestureTarget{HasX: true, HasY: true, HasDeltaY: true}},
		{name: "scroll_at with a horizontal delta", action: "scroll_at", target: act.GestureTarget{HasX: true, HasY: true, HasDeltaX: true}},
		{name: "an action that is not a gesture", action: "click", target: act.GestureTarget{}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if problem := act.ValidateGesture(testCase.action, testCase.target); problem != nil {
				t.Fatalf("rejected a valid call: %+v", problem)
			}
		})
	}
}

func TestValidateZoomRegionRejectsRectanglesChromeWouldReturnEmpty(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		target act.GestureTarget
		param  string
	}{
		{name: "no origin", target: act.GestureTarget{Width: 10, Height: 10}, param: "x"},
		{name: "zero width", target: act.GestureTarget{HasX: true, HasY: true, Height: 10}, param: "width"},
		{name: "negative height", target: act.GestureTarget{HasX: true, HasY: true, Width: 10, Height: -4}, param: "width"},
		{name: "scale beyond the renderer cap", target: act.GestureTarget{HasX: true, HasY: true, Width: 10, Height: 10, Scale: 9}, param: "scale"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			problem := act.ValidateZoomRegion(testCase.target)
			if problem == nil || problem.Param != testCase.param {
				t.Fatalf("problem = %+v, want param %q", problem, testCase.param)
			}
		})
	}

	valid := act.GestureTarget{HasX: true, HasY: true, Width: 320, Height: 200, Scale: 2}
	if problem := act.ValidateZoomRegion(valid); problem != nil {
		t.Fatalf("rejected a valid clip: %+v", problem)
	}
}

func TestExtractCapturedDataURLLiftsTheImageOutOfTheTextBlock(t *testing.T) {
	t.Parallel()

	nested := map[string]any{
		"status": "complete",
		"result": map[string]any{"path": "/tmp/a.png", "data_url": "data:image/png;base64,AAAA"},
	}
	if got := act.ExtractCapturedDataURL(nested); got != "data:image/png;base64,AAAA" {
		t.Fatalf("data_url = %q", got)
	}
	inner, _ := nested["result"].(map[string]any)
	if _, still := inner["data_url"]; still {
		t.Fatal("the base64 payload must be removed from the text block, not duplicated into it")
	}
	if inner["path"] != "/tmp/a.png" {
		t.Fatal("lifting the image must leave the rest of the result intact")
	}
	if act.ExtractCapturedDataURL(map[string]any{"status": "queued"}) != "" {
		t.Fatal("a queued response carries no image")
	}
}

// pointerGestures is the surface this file covers; the daemon must route every one of them.
var pointerGestures = []string{"drag", "right_click", "double_click", "triple_click", "hover_at", "scroll_at"}

func TestGestureActionsAreDispatchedAsDOMActions(t *testing.T) {
	t.Parallel()

	for _, action := range pointerGestures {
		if !act.DOMPrimitiveActions[action] {
			t.Errorf("%q is not routed to the DOM primitive handler, so the dispatcher has no entry for it", action)
		}
		// A gesture the rules do not recognise would be waved through with no target at all.
		if act.ValidateGesture(action, act.GestureTarget{}) == nil {
			t.Errorf("%q is not covered by the gesture rules, so an empty call would be accepted", action)
		}
	}
}

func TestEveryGestureActionIsInTheInteractSchema(t *testing.T) {
	t.Parallel()

	named := map[string]bool{}
	for _, spec := range interact.ActionSpecs() {
		named[spec.Name] = true
	}
	for _, action := range []string{"drag", "right_click", "double_click", "triple_click", "hover_at", "scroll_at", "zoom_region"} {
		if !named[action] {
			t.Errorf("interact schema has no action spec for %q — describe_capabilities and the what enum would both omit it", action)
		}
	}
}

func TestGestureHandlerRejectsBeforeQueuingAnything(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		action string
		args   string
		text   string
	}{
		{name: "drag without a route", action: "drag", args: `{}`, text: "drag_path"},
		{name: "drag with a single point", action: "drag", args: `{"drag_path":[{"x":1,"y":2}]}`, text: "drag_path"},
		{name: "hover_at without coordinates", action: "hover_at", args: `{}`, text: "x"},
		{name: "scroll_at without a delta", action: "scroll_at", args: `{"x":10,"y":20}`, text: "delta"},
		{name: "right_click without a target", action: "right_click", args: `{}`, text: "selector"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newGateFixture()
			connect(fixture)
			_, message := errorCode(t, fixture.dom.HandleDOMPrimitive(request(), json.RawMessage(testCase.args), testCase.action))
			if !strings.Contains(message, testCase.text) {
				t.Fatalf("message = %q, want it to name %q", message, testCase.text)
			}
			if len(fixture.queued) != 0 {
				t.Fatalf("a rejected gesture still queued %d command(s)", len(fixture.queued))
			}
		})
	}
}

func TestGestureHandlerQueuesACoordinateAddressedGesture(t *testing.T) {
	fixture := newGateFixture()
	connect(fixture)
	track(fixture)

	response := fixture.dom.HandleDOMPrimitive(request(), json.RawMessage(`{"x":120,"y":300,"delta_y":400}`), "scroll_at")
	if got := result(t, response); got.IsError {
		t.Fatalf("coordinate-addressed scroll_at was rejected: %#v", got)
	}
	if len(fixture.queued) != 1 {
		t.Fatalf("queued %d commands, want 1", len(fixture.queued))
	}
	query := fixture.queued[0]
	if query.Type != "dom_action" {
		t.Fatalf("query type = %q, want dom_action", query.Type)
	}
	var params map[string]any
	if err := json.Unmarshal(query.Params, &params); err != nil {
		t.Fatalf("decode query params: %v", err)
	}
	if params["action"] != "scroll_at" || params["delta_y"] != float64(400) {
		t.Fatalf("query params = %#v", params)
	}
}

func TestDragQueuesItsWholeRoute(t *testing.T) {
	fixture := newGateFixture()
	connect(fixture)
	track(fixture)

	args := `{"drag_path":[{"x":10,"y":10},{"x":60,"y":10},{"x":60,"y":80}],"modifiers":["shift"]}`
	if got := result(t, fixture.dom.HandleDOMPrimitive(request(), json.RawMessage(args), "drag")); got.IsError {
		t.Fatalf("drag rejected: %#v", got)
	}
	var params map[string]any
	if err := json.Unmarshal(fixture.queued[0].Params, &params); err != nil {
		t.Fatalf("decode query params: %v", err)
	}
	route, _ := params["drag_path"].([]any)
	if len(route) != 3 {
		t.Fatalf("drag_path lost waypoints: %#v", params["drag_path"])
	}
	modifiers, _ := params["modifiers"].([]any)
	if len(modifiers) != 1 || modifiers[0] != "shift" {
		t.Fatalf("modifiers = %#v — a dropped modifier is a different gesture", params["modifiers"])
	}
}

func TestCoordinateClickCarriesItsModifiers(t *testing.T) {
	fixture := newGateFixture()
	connect(fixture)
	track(fixture)

	args := `{"x":40,"y":50,"modifiers":["ctrl"]}`
	if got := result(t, fixture.dom.HandleDOMPrimitive(request(), json.RawMessage(args), "click")); got.IsError {
		t.Fatalf("coordinate click rejected: %#v", got)
	}
	if len(fixture.queued) != 1 || fixture.queued[0].Type != "cdp_action" {
		t.Fatalf("queued = %#v", fixture.queued)
	}
	var params map[string]any
	if err := json.Unmarshal(fixture.queued[0].Params, &params); err != nil {
		t.Fatalf("decode query params: %v", err)
	}
	modifiers, _ := params["modifiers"].([]any)
	// A dropped ctrl turns "open in a background tab" into "navigate in place", reported as success.
	if len(modifiers) != 1 || modifiers[0] != "ctrl" {
		t.Fatalf("modifiers = %#v, want [ctrl]", params["modifiers"])
	}
}

func TestZoomRegionRejectsAnUnreadableClipBeforeAttachingTheDebugger(t *testing.T) {
	for _, testCase := range []struct {
		name string
		args string
		text string
	}{
		{name: "no rectangle", args: `{"x":0,"y":0}`, text: "width"},
		{name: "no origin", args: `{"width":100,"height":100}`, text: "x"},
		{name: "scale beyond the cap", args: `{"x":0,"y":0,"width":10,"height":10,"scale":9}`, text: "scale"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newGateFixture()
			connect(fixture)
			track(fixture)
			code, message := errorCode(t, fixture.browser.Handle("zoom_region", request(), json.RawMessage(testCase.args)))
			if code != mcp.ErrMissingParam && code != mcp.ErrInvalidParam {
				t.Fatalf("code = %q", code)
			}
			if !strings.Contains(message, testCase.text) {
				t.Fatalf("message = %q, want it to name %q", message, testCase.text)
			}
		})
	}
}

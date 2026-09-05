// targeting_test.go — Contracts for naming WHERE an interact action acts (kaboom-05ue.8).
//
// One action family now covers all three ways of naming a target: a selector, a ref from
// find, and a viewport coordinate read off a screenshot. Every rule tested here stops a call
// that would otherwise act somewhere the caller did not point at and report success — two
// targets in one call with the loser dropped in silence, half a coordinate, or a point that
// is not on the screen at all and that Chrome clamps to the nearest edge.
// Docs: docs/features/feature/interact-explore/index.md

package contracts_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema/interact"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// coordinateActions is the surface that now accepts x/y as an alternative to a selector.
var coordinateActions = []string{"click", "right_click", "double_click", "triple_click", "hover_at", "scroll_at"}

func TestValidateTargetingRejectsCallsThatNameMoreThanOneTarget(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		action string
		target act.GestureTarget
		names  []string
	}{
		{
			name: "selector and coordinate", action: "click",
			target: act.GestureTarget{Selector: "#buy", HasX: true, HasY: true, X: 40, Y: 50},
			names:  []string{"selector", "#buy", "40", "50"},
		},
		{
			name: "ref and coordinate", action: "click",
			target: act.GestureTarget{Ref: "e12", HasX: true, HasY: true, X: 40, Y: 50},
			names:  []string{"ref", "e12", "40", "50"},
		},
		{
			name: "selector and ref", action: "type",
			target: act.GestureTarget{Selector: "#email", Ref: "e12"},
			names:  []string{"selector", "#email", "ref", "e12"},
		},
		{
			name: "element handle and coordinate", action: "right_click",
			target: act.GestureTarget{ElementID: "e7", HasX: true, HasY: true, X: 1, Y: 2},
			names:  []string{"element_id", "e7"},
		},
		{
			name: "index and coordinate", action: "double_click",
			target: act.GestureTarget{HasIndex: true, HasX: true, HasY: true, X: 1, Y: 2},
			names:  []string{"index"},
		},
		{
			name: "coordinate on a gesture that also got a ref", action: "scroll_at",
			target: act.GestureTarget{Ref: "e3", HasX: true, HasY: true, X: 9, Y: 9, HasDeltaY: true},
			names:  []string{"ref", "e3"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			problem := act.ValidateTargeting(testCase.action, testCase.target)
			if problem == nil {
				t.Fatalf("%s accepted two targets in one call and would have silently picked one", testCase.action)
			}
			for _, want := range testCase.names {
				if !strings.Contains(problem.Message, want) {
					t.Fatalf("message = %q, want it to name %q so the caller knows which targets collided", problem.Message, want)
				}
			}
			if problem.Retry == "" {
				t.Fatal("a rejection must say what to send instead")
			}
			if problem.Missing {
				t.Fatal("a conflict is a wrong parameter, not a missing one — the code must be ErrInvalidParam")
			}
		})
	}
}

func TestValidateTargetingAcceptsExactlyOneTarget(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		action string
		target act.GestureTarget
	}{
		{name: "selector alone", action: "click", target: act.GestureTarget{Selector: "#buy"}},
		{name: "ref alone", action: "click", target: act.GestureTarget{Ref: "e12"}},
		{name: "coordinate alone", action: "click", target: act.GestureTarget{HasX: true, HasY: true, X: 40, Y: 50}},
		{name: "origin is inside the viewport", action: "click", target: act.GestureTarget{HasX: true, HasY: true}},
		{name: "coordinate on hover_at", action: "hover_at", target: act.GestureTarget{HasX: true, HasY: true, X: 12, Y: 9}},
		{name: "no target at all is another rule's business", action: "click", target: act.GestureTarget{}},
		{name: "x/y on an action that cannot be aimed at a point", action: "type", target: act.GestureTarget{Selector: "#email", HasX: true, HasY: true}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if problem := act.ValidateTargeting(testCase.action, testCase.target); problem != nil {
				t.Fatalf("rejected a call that names one target: %+v", problem)
			}
		})
	}
}

func TestValidateTargetingRejectsHalfACoordinate(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		target act.GestureTarget
		param  string
	}{
		{name: "x without y", target: act.GestureTarget{HasX: true, X: 40}, param: "y"},
		{name: "y without x", target: act.GestureTarget{HasY: true, Y: 50}, param: "x"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			problem := act.ValidateTargeting("click", testCase.target)
			if problem == nil {
				t.Fatal("half a coordinate is not a point; accepting it clicks the selector path with a stray argument")
			}
			if problem.Param != testCase.param {
				t.Fatalf("param = %q, want %q", problem.Param, testCase.param)
			}
			if !problem.Missing {
				t.Fatal("an absent half of a coordinate is a missing parameter")
			}
		})
	}
}

func TestValidateTargetingRejectsAPointThatIsNotOnTheScreen(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		target act.GestureTarget
		param  string
	}{
		{name: "left of the viewport", target: act.GestureTarget{HasX: true, HasY: true, X: -1, Y: 50}, param: "x"},
		{name: "above the viewport", target: act.GestureTarget{HasX: true, HasY: true, X: 10, Y: -20}, param: "y"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			problem := act.ValidateTargeting("click", testCase.target)
			if problem == nil {
				t.Fatal("a negative viewport coordinate is off screen; Chrome clamps it and clicks the edge instead")
			}
			if problem.Param != testCase.param {
				t.Fatalf("param = %q, want %q", problem.Param, testCase.param)
			}
			if !strings.Contains(problem.Message, "outside the viewport") {
				t.Fatalf("message = %q, want it to say the point is outside the viewport", problem.Message)
			}
		})
	}
}

func TestTargetingConflictIsRejectedBeforeAnythingIsQueued(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		action string
		args   string
		text   string
	}{
		{name: "click by selector and coordinate", action: "click", args: `{"selector":"#buy","x":40,"y":50}`, text: "selector"},
		{name: "click by ref and coordinate", action: "click", args: `{"ref":"e12","x":40,"y":50}`, text: "ref"},
		{name: "click by selector and ref", action: "click", args: `{"selector":"#buy","ref":"e12"}`, text: "ref"},
		{name: "hover_at by ref and coordinate", action: "hover_at", args: `{"ref":"e12","x":40,"y":50}`, text: "ref"},
		{name: "scroll_at by selector and coordinate", action: "scroll_at", args: `{"selector":"#pane","x":40,"y":50,"delta_y":100}`, text: "selector"},
		{name: "click at a negative coordinate", action: "click", args: `{"x":-5,"y":50}`, text: "outside the viewport"},
		{name: "click with only an x", action: "click", args: `{"x":40}`, text: "y"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newGateFixture()
			connect(fixture)
			track(fixture)
			code, message := errorCode(t, fixture.dom.HandleDOMPrimitive(request(), json.RawMessage(testCase.args), testCase.action))
			if code != mcp.ErrInvalidParam && code != mcp.ErrMissingParam {
				t.Fatalf("code = %q, want a parameter error", code)
			}
			if !strings.Contains(message, testCase.text) {
				t.Fatalf("message = %q, want it to name %q", message, testCase.text)
			}
			if len(fixture.queued) != 0 {
				t.Fatalf("a call with an ambiguous target still queued %d command(s)", len(fixture.queued))
			}
		})
	}
}

func TestCoordinateTargetingReachesTheSameActionFamilyAsSelectorTargeting(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		action    string
		args      string
		queryType string
	}{
		{name: "click at a point", action: "click", args: `{"x":40,"y":50}`, queryType: "cdp_action"},
		{name: "click by selector", action: "click", args: `{"selector":"#buy"}`, queryType: "dom_action"},
		{name: "hover_at a point", action: "hover_at", args: `{"x":40,"y":50}`, queryType: "dom_action"},
		{name: "scroll_at a point", action: "scroll_at", args: `{"x":40,"y":50,"delta_y":300}`, queryType: "dom_action"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newGateFixture()
			connect(fixture)
			track(fixture)
			if got := result(t, fixture.dom.HandleDOMPrimitive(request(), json.RawMessage(testCase.args), testCase.action)); got.IsError {
				t.Fatalf("%s was rejected: %#v", testCase.action, got)
			}
			if len(fixture.queued) != 1 || fixture.queued[0].Type != testCase.queryType {
				t.Fatalf("queued = %#v, want one %s query", fixture.queued, testCase.queryType)
			}
		})
	}
}

// TestHardwareClickIsGoneFromEverySurface holds the deletion in place. A coordinate click is
// `click` with x/y; an action named after the mechanism that delivers it would be a second way
// to say the same thing, and the two would drift.
func TestHardwareClickIsGoneFromEverySurface(t *testing.T) {
	t.Parallel()

	for _, spec := range interact.ActionSpecs() {
		if spec.Name == "hardware_click" {
			t.Fatal("interact still exposes hardware_click; coordinate clicks belong to click")
		}
	}
	if act.DOMPrimitiveActions["hardware_click"] {
		t.Fatal("hardware_click is still routed to the DOM primitive handler")
	}
}

// TestEveryCoordinateActionDocumentsXAndY keeps the schema honest about the folded surface:
// an agent that reads a screenshot's coordinate_frame must be able to see, from the tool
// schema alone, which actions take the point it computed.
func TestEveryCoordinateActionDocumentsXAndY(t *testing.T) {
	t.Parallel()

	specs := map[string]interact.ActionSpec{}
	for _, spec := range interact.ActionSpecs() {
		specs[spec.Name] = spec
	}
	for _, action := range coordinateActions {
		spec, ok := specs[action]
		if !ok {
			t.Fatalf("interact schema has no action spec for %q", action)
		}
		named := map[string]bool{}
		for _, param := range append(append([]string{}, spec.Required...), spec.Optional...) {
			named[param] = true
		}
		if !named["x"] || !named["y"] {
			t.Errorf("%q does not name x and y in its schema, so an agent holding a screenshot coordinate cannot tell it accepts one", action)
		}
	}
}

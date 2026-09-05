// Purpose: The rule that decides WHERE an interact action acts when a call can name its target
// three different ways — a CSS/semantic selector, a ref from find, or a viewport coordinate read
// off a screenshot.
// Why: The handlers used to resolve targets in a fixed order and drop the losers in silence. A
// click carrying a selector AND x/y clicked the coordinate; a click carrying a ref AND a selector
// ignored the ref. Either way the agent was told the call succeeded and never learned which of
// the two things it had actually hit. A coordinate outside the viewport was worse: Chrome does
// not reject an out-of-range Input.dispatchMouseEvent, it clamps the point to the nearest edge,
// so the agent read "clicked at (1900, 400)" for a click that landed on the right border.
// Docs: docs/features/feature/interact-explore/index.md

package interact

import (
	"fmt"
	"math"
	"strings"
)

// coordinateTargetable reports the actions that accept a viewport point as an ALTERNATIVE to an
// element. These are the actions a screenshot's coordinate_frame feeds: an agent reads a pixel
// off the image, maps it through image_to_viewport, and passes the result here.
//
// `drag` is absent on purpose: its target is drag_path, not x/y, and it is validated by
// validateDrag. Actions outside this set ignore x/y entirely, so a stray coordinate on one of
// them is a spurious argument rather than a second target.
func coordinateTargetable(action string) bool {
	switch action {
	case "click", "right_click", "double_click", "triple_click", "hover_at", "scroll_at":
		return true
	}
	return false
}

// namedTarget is one way a call said where to act, rendered for the error message.
type namedTarget struct {
	param string
	shown string
}

// namedTargets lists every target channel the call filled in.
//
// selector, element_id and index are ONE channel: they are three spellings of "this element in
// the DOM", and the handlers resolve index and element_id into a selector before acting. ref is
// its own channel because it comes from the accessibility tree and resolves to a point, not a
// selector. x/y is its own channel and counts only when BOTH halves are present — half a
// coordinate is not a target, it is a typo, and it is reported as one below.
func namedTargets(action string, target GestureTarget) []namedTarget {
	var named []namedTarget
	switch {
	case target.Selector != "":
		named = append(named, namedTarget{param: "selector", shown: fmt.Sprintf("selector=%q", target.Selector)})
	case target.ElementID != "":
		named = append(named, namedTarget{param: "element_id", shown: fmt.Sprintf("element_id=%q", target.ElementID)})
	case target.HasIndex:
		named = append(named, namedTarget{param: "index", shown: "index from list_interactive"})
	}
	if target.Ref != "" {
		named = append(named, namedTarget{param: "ref", shown: fmt.Sprintf("ref=%q", target.Ref)})
	}
	if coordinateTargetable(action) && target.HasX && target.HasY {
		named = append(named, namedTarget{
			param: "x",
			shown: fmt.Sprintf("x/y=(%g, %g)", target.X, target.Y),
		})
	}
	return named
}

// ValidateTargeting reports why a call cannot say where it means to act, or nil when it can.
//
// Three failures, all of which used to be silent:
//   - two targets in one call, where the resolution order decided which one won;
//   - half a coordinate, which fell through to the selector rule and blamed a missing selector;
//   - a point off the screen, which Chrome clamps rather than refuses.
func ValidateTargeting(action string, target GestureTarget) *gestureParamError {
	if named := namedTargets(action, target); len(named) > 1 {
		return conflictingTargets(action, named)
	}
	if problem := halfACoordinate(action, target); problem != nil {
		return problem
	}
	if !target.HasX || !target.HasY {
		return nil
	}
	return ValidateViewportPoint(action, target.X, target.Y)
}

// ValidateViewportPoint reports why a viewport coordinate cannot be acted on, or nil.
//
// Exported separately because a ref resolves INTO a point: find reports an element's centre and
// the handler acts on it, so the same rule has to hold for a point the caller supplied and for a
// point the daemon computed. Actions that cannot be aimed at a point are not judged by it.
//
// The daemon knows only where the viewport starts, not how big it is — the extension holds the
// far edges (see src/background/dom/viewport-bounds.ts) because only the page can measure them.
func ValidateViewportPoint(action string, x, y float64) *gestureParamError {
	if !coordinateTargetable(action) {
		return nil
	}
	param, value := "", 0.0
	switch {
	case !finite(x) || x < 0:
		param, value = "x", x
	case !finite(y) || y < 0:
		param, value = "y", y
	default:
		return nil
	}
	return &gestureParamError{
		Param: param,
		Message: fmt.Sprintf(
			"%s was aimed at %s=%g, which is outside the viewport: viewport coordinates are CSS pixels measured from the top-left of the visible area, so they start at (0, 0) and grow right and down.",
			action, param, value),
		Retry: coordinateRetry,
	}
}

// halfACoordinate reports an x with no y, or a y with no x.
//
// Left alone, such a call falls through to the selector rule and is rejected for a missing
// selector — which sends the caller to fix the wrong parameter.
func halfACoordinate(action string, target GestureTarget) *gestureParamError {
	if !coordinateTargetable(action) || target.HasX == target.HasY {
		return nil
	}
	missing, supplied := "y", "x"
	if target.HasY {
		missing, supplied = "x", "y"
	}
	return &gestureParamError{
		Param:   missing,
		Missing: true,
		Message: fmt.Sprintf("%s was given %s but no %s; a viewport point needs both.", action, supplied, missing),
		Retry:   coordinateRetry,
	}
}

func conflictingTargets(action string, named []namedTarget) *gestureParamError {
	shown := make([]string, 0, len(named))
	for _, target := range named {
		shown = append(shown, target.shown)
	}
	return &gestureParamError{
		Param: named[0].param,
		Message: fmt.Sprintf(
			"%s was given more than one target: %s. A call names exactly one, and choosing for you would act somewhere you did not point at.",
			action, strings.Join(shown, " and ")),
		Retry: "Send exactly one target: 'selector' (CSS or semantic), 'ref' from find, 'element_id'/'index' from list_interactive, or 'x' and 'y' viewport coordinates. Drop the others.",
	}
}

// coordinateRetry names where a valid coordinate comes from. It is one string because the three
// coordinate failures should send the caller to the same place, and a screenshot's
// coordinate_frame is the only thing in the product that produces a point these actions accept.
const coordinateRetry = "Take a screenshot with observe, then map a pixel through its coordinate_frame: " +
	"x = image_x*image_to_viewport.scale_x + offset_x, y = image_y*scale_y + offset_y. " +
	"Those are viewport CSS pixels, which is exactly what x and y take. " +
	"To act on an element instead, send 'selector', 'ref' or 'element_id' and no coordinate."

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

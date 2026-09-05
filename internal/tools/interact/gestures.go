// Purpose: Target and parameter rules for the pointer gesture actions (drag, right/double/triple
// click, hover_at, scroll_at) and the clipped zoom_region capture.
// Why: These actions accept a coordinate where the older DOM actions demand a selector, so the
// shared "selector or element_id or index" rule would reject a correct call. Keeping the rules
// here as pure functions means they are testable without an MCP request or a browser.
// Docs: docs/features/feature/interact-explore/index.md

package interact

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// isPointerGesture reports the hardware-level pointer gestures. The extension dispatches each
// through CDP when it can and falls back to synthetic DOM events when it cannot; either way the
// daemon routes them as ordinary dom_action queries, so DOMPrimitiveActions lists them too.
func isPointerGesture(action string) bool {
	switch action {
	case "drag", "right_click", "double_click", "triple_click", "hover_at", "scroll_at":
		return true
	}
	return false
}

// isCoordinateOnlyGesture reports the gestures that address a viewport pixel and never an
// element. `hover` already covers element hover and `scroll_to` already covers element
// scrolling, so these two exist precisely for the case where there is no element to name —
// a canvas, a map tile, a PDF pane.
func isCoordinateOnlyGesture(action string) bool {
	return action == "hover_at" || action == "scroll_at"
}

// GestureTarget is everything a gesture call said about where and how to act.
//
// HasX/HasY and X/Y are both carried: a point at (0, 0) is a legal target, so "supplied" and
// "zero" have to stay distinguishable, and the targeting rules need the values themselves to say
// which coordinate was out of bounds.
type GestureTarget struct {
	Selector   string
	ElementID  string
	Ref        string
	HasIndex   bool
	HasX       bool
	HasY       bool
	X          float64
	Y          float64
	PathPoints int
	HasDeltaX  bool
	HasDeltaY  bool
	Width      float64
	Height     float64
	Scale      float64
}

// gestureParamError is a validation failure expressed without MCP types, so the rules can be
// unit-tested and the handler owns the response shape. The type is package-private and the
// fields are not: a caller reads Param/Message/Retry off the returned pointer, and never has
// to name the type.
type gestureParamError struct {
	// Param is the argument the caller should fix.
	Param string
	// Message says what is wrong; Retry says what to send instead.
	Message string
	Retry   string
	// Missing is true when the parameter was absent rather than malformed, which selects
	// ErrMissingParam over ErrInvalidParam at the call site.
	Missing bool
}

// ValidateGesture reports why a pointer gesture cannot be dispatched, or nil when it can.
//
// Every check here catches something that would otherwise fail inside the page with a message
// that blames the page rather than the call: a one-point drag path silently drags nothing, and a
// scroll_at with no delta dispatches a wheel event of zero pixels and reports success.
func ValidateGesture(action string, target GestureTarget) *gestureParamError {
	if action == "drag" {
		return validateDrag(target)
	}
	if isCoordinateOnlyGesture(action) {
		return validateCoordinateGesture(action, target)
	}
	if !isPointerGesture(action) {
		return nil
	}
	return validateClickGesture(action, target)
}

func validateDrag(target GestureTarget) *gestureParamError {
	if target.PathPoints >= 2 {
		return nil
	}
	return &gestureParamError{
		Param:   "drag_path",
		Missing: target.PathPoints == 0,
		Message: "drag requires 'drag_path' with at least 2 points",
		// No JSON braces in this prose: the MCP text block is "Error: code — retry\n<json>",
		// and every reader that locates the payload by its first brace would stop inside the
		// example instead of at the structured error.
		Retry: "Send drag_path as an ordered list of at least two x/y points — the route to drag along, not just the endpoints. Intermediate points make the drag follow a curve. It is drag_path, not path: 'path' is the cookie path.",
	}
}

func validateCoordinateGesture(action string, target GestureTarget) *gestureParamError {
	if !target.HasX || !target.HasY {
		return &gestureParamError{
			Param:   "x",
			Missing: true,
			Message: action + " requires viewport coordinates 'x' and 'y'",
			Retry:   "Add x and y (pixels from the top-left of the viewport). To act on an element instead, use hover or scroll_to with a selector.",
		}
	}
	if action == "scroll_at" && !target.HasDeltaX && !target.HasDeltaY {
		return &gestureParamError{
			Param:   "delta_y",
			Missing: true,
			Message: "scroll_at requires 'delta_x' or 'delta_y'",
			Retry:   "Add delta_y (positive scrolls down, negative up) or delta_x for horizontal scrolling.",
		}
	}
	return nil
}

func validateClickGesture(action string, target GestureTarget) *gestureParamError {
	if target.Selector != "" || target.ElementID != "" || target.HasIndex || (target.HasX && target.HasY) {
		return nil
	}
	return &gestureParamError{
		Param:   "selector",
		Missing: true,
		Message: "Required parameter 'selector', 'element_id', 'index', or 'x'/'y' is missing for " + action,
		Retry:   "Add 'selector' (CSS or semantic), 'element_id'/'index' from list_interactive, or x/y viewport coordinates.",
	}
}

// ValidateZoomRegion reports why a clipped capture cannot be taken, or nil when it can.
//
// Chrome answers Page.captureScreenshot with a zero-size or absurd clip by returning an image
// nobody can read rather than by failing, so the rectangle is checked before it is sent.
func ValidateZoomRegion(target GestureTarget) *gestureParamError {
	if !target.HasX || !target.HasY {
		return &gestureParamError{
			Param:   "x",
			Missing: true,
			Message: "zoom_region requires 'x' and 'y' — the top-left corner of the region to capture",
			Retry:   "Add x and y (pixels from the top-left of the viewport), plus width and height.",
		}
	}
	if target.Width <= 0 || target.Height <= 0 {
		return &gestureParamError{
			Param:   "width",
			Missing: target.Width == 0 && target.Height == 0,
			Message: "zoom_region requires 'width' and 'height' greater than 0",
			Retry:   "Add width and height in pixels — the size of the region to capture.",
		}
	}
	if target.Scale < 0 || target.Scale > maxZoomScale {
		return &gestureParamError{
			Param:   "scale",
			Message: "zoom_region 'scale' must be between 0 and 4",
			Retry:   "Use scale 2 to render the region at twice its on-screen size, or omit it for 1:1.",
		}
	}
	return nil
}

// GestureParamFailure turns a rule violation into the MCP failure both gesture handlers return,
// so a missing drag_path and a missing zoom width are reported with the same shape.
func GestureParamFailure(req mcp.JSONRPCRequest, action string, problem *gestureParamError) mcp.JSONRPCResponse {
	code := mcp.ErrInvalidParam
	if problem.Missing {
		code = mcp.ErrMissingParam
	}
	return mcp.Fail(req, code, problem.Message, problem.Retry, mcp.WithParam(problem.Param), mcp.WithAction(action))
}

// maxZoomScale caps the supersampling factor. Above this the capture costs more memory in the
// renderer than the extra legibility is worth, and Chrome starts refusing the clip outright.
const maxZoomScale = 4.0

// ExtractCapturedDataURL removes the base64 image from a query result and returns it, walking
// the nesting the async command lifecycle adds around the extension's payload.
//
// It is removed rather than copied because the result also becomes the response's TEXT block: a
// megabyte of base64 would be spent twice, once as an image Claude can see and once as characters
// it cannot read. Returns "" when the response carries no image, which is the normal shape of a
// background/queued call.
func ExtractCapturedDataURL(data map[string]any) string {
	if data == nil {
		return ""
	}
	if dataURL, ok := data["data_url"].(string); ok && dataURL != "" {
		delete(data, "data_url")
		return dataURL
	}
	nested, ok := data["result"].(map[string]any)
	if !ok {
		return ""
	}
	return ExtractCapturedDataURL(nested)
}

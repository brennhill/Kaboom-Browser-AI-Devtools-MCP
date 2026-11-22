// wire_screenshot.go — Defines the coordinate frame that ties a returned screenshot's pixels to the coordinates an action accepts.
//
// PURPOSE: close the loop between what the agent sees and where it can act. A
// screenshot with no frame of reference cannot be clicked: the image is in device
// pixels, `click` takes CSS pixels relative to the viewport, and on a retina
// display those differ by 2x — which arrives as random misclicks rather than as an
// error.
//
// CONTRACT: the page reports MEASURED quantities and no assumptions. The scale is
// derived from the image the caller is actually holding divided by the CSS extent
// it covers, never from an assumed device pixel ratio, because the capture may
// come from Page.captureScreenshot, from chrome.tabs.captureVisibleTab, or from a
// crop of either, and browser zoom moves the ratio underneath all three. A frame
// that cannot be measured is absent, and an absent frame is honest; a guessed one
// is a misclick with a number attached.

package screenshotframe

import "fmt"

// Capture kinds. Each names what CSS region the image covers, which is what
// decides the mapping.
const (
	// CaptureViewport is the visible viewport, origin at the viewport's top-left.
	CaptureViewport = "viewport"
	// CaptureFullPage is the whole document, so the image origin is the document
	// origin and a point in it is only clickable after scrolling to it.
	CaptureFullPage = "full_page"
	// CaptureElement is a crop of the viewport capture around one element.
	CaptureElement = "element"
	// CaptureRegion is a caller-chosen rectangle, optionally supersampled.
	CaptureRegion = "region"
)

// CaptureKinds returns the recognized capture kinds.
//
// A function rather than a package var: the set is fixed, and a mutable global
// here is a shared surface any caller could edit.
func CaptureKinds() []string {
	return []string{CaptureViewport, CaptureFullPage, CaptureElement, CaptureRegion}
}

// scaleAgreementTolerance is how far the horizontal and vertical scales may differ
// before the image is treated as distorted rather than merely rounded. One pixel
// of rounding on a 1280-wide image is 0.08%; anything past 1% means the image is
// not a uniform scaling of the region it claims to cover, and a caller reading a
// coordinate off it would land somewhere else.
const scaleAgreementTolerance = 0.01

// WireImageToViewport is the affine map from a pixel in the returned image to the
// CSS-pixel coordinate that click, hover_at and scroll_at accept:
//
//	viewport_x = image_x*scale_x + offset_x
//	viewport_y = image_y*scale_y + offset_y
//
// Both scales are reported because a capture is not guaranteed to be uniform: a
// clipped capture whose height was clamped to Chrome's maximum texture size is
// squashed on one axis only, and a single scale would place every click in the
// lower half of such an image too high.
type WireImageToViewport struct {
	ScaleX  float64 `json:"scale_x"`
	ScaleY  float64 `json:"scale_y"`
	OffsetX float64 `json:"offset_x"`
	OffsetY float64 `json:"offset_y"`
}

// WireImageRect is a rectangle in image pixels.
type WireImageRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// WireCoordinateFrame is everything needed to read a coordinate off a screenshot
// and act on it.
type WireCoordinateFrame struct {
	// Capture is one of CaptureKinds().
	Capture string `json:"capture"`

	// ImageWidth and ImageHeight are the returned image's real pixel dimensions,
	// measured by decoding it rather than predicted from the viewport times a
	// device pixel ratio.
	ImageWidth  float64 `json:"image_width"`
	ImageHeight float64 `json:"image_height"`

	// ViewportWidth and ViewportHeight are the visible viewport in CSS pixels.
	ViewportWidth  float64 `json:"viewport_width"`
	ViewportHeight float64 `json:"viewport_height"`

	// ScrollX and ScrollY are the document scroll offset at capture time. A
	// full-page image's pixel (0,0) is the document origin, so these are what
	// turn a point in it into a viewport coordinate.
	ScrollX float64 `json:"scroll_x"`
	ScrollY float64 `json:"scroll_y"`

	// DocumentWidth and DocumentHeight are the scrollable extent, so a caller can
	// tell how much of the page the image left out.
	DocumentWidth  float64 `json:"document_width"`
	DocumentHeight float64 `json:"document_height"`

	// DevicePixelRatio is what the page reports for window.devicePixelRatio. It is
	// context, NOT the mapping: use ImageToViewport, which is measured. The two
	// disagree under browser zoom and whenever a capture path rescaled the image.
	DevicePixelRatio float64 `json:"device_pixel_ratio"`

	// Clipped is true when the image shows less than the whole document, so a
	// caller knows a target it cannot find may simply be off-image.
	Clipped bool `json:"clipped"`

	// ImageToViewport maps image pixels to clickable CSS coordinates.
	ImageToViewport WireImageToViewport `json:"image_to_viewport"`

	// ViewportBoundsInImage is the part of the image that is on screen right now,
	// in image pixels. For a viewport capture it is the whole image. For a
	// full-page capture it is the scrolled window, and a point outside it needs a
	// scroll before it can be clicked.
	ViewportBoundsInImage WireImageRect `json:"viewport_bounds_in_image"`

	// Note states the mapping in words so a caller need not have read this file.
	// It is filled in by WithNote on the daemon side rather than by the page, so
	// the sentence and the arithmetic it describes have one source.
	Note string `json:"note,omitempty"`
}

// mappingNote is the sentence attached to a frame whose image origin is the
// viewport: the mapping is complete on its own.
const mappingNote = "Read a target's pixel (image_x, image_y) off the image, then act at " +
	"x = image_x*image_to_viewport.scale_x + offset_x, y = image_y*scale_y + offset_y. " +
	"Those are CSS pixels relative to the viewport, which is what click, hover_at and scroll_at take."

// documentOriginNote is the sentence attached to a frame whose image covers more
// than the viewport. The arithmetic is the same; what changes is that the answer
// may name a point that is not on screen, and clicking it without scrolling first
// would hit whatever happens to be at those viewport coordinates instead.
const documentOriginNote = mappingNote +
	" This image covers the whole document, so a point outside viewport_bounds_in_image is NOT on screen: " +
	"scroll to it before acting, or the coordinate lands on whatever is currently at that position."

// WithNote returns the frame carrying the sentence that belongs on it.
func (f WireCoordinateFrame) WithNote() WireCoordinateFrame {
	if f.Capture == CaptureFullPage {
		f.Note = documentOriginNote
		return f
	}
	f.Note = mappingNote
	return f
}

// viewportPoint maps a pixel in the returned image to the CSS-pixel coordinate an
// action accepts. It is the executable form of Note, and the tests hold the two to
// the same answer.
func (f WireCoordinateFrame) viewportPoint(imageX, imageY float64) (x, y float64) {
	return imageX*f.ImageToViewport.ScaleX + f.ImageToViewport.OffsetX,
		imageY*f.ImageToViewport.ScaleY + f.ImageToViewport.OffsetY
}

// onScreen reports whether a pixel of the image is inside the visible viewport, so
// a caller can tell "act now" from "scroll first" without doing the arithmetic.
func (f WireCoordinateFrame) onScreen(imageX, imageY float64) bool {
	x, y := f.viewportPoint(imageX, imageY)
	return x >= 0 && y >= 0 && x <= f.ViewportWidth && y <= f.ViewportHeight
}

// Validate reports why a frame cannot be trusted to place a click.
//
// Every check here is a way to be wrong silently. A zero scale sends every
// coordinate to the offset; a negative image size makes the bounds meaningless; an
// unknown capture kind means the caller cannot know whether the origin is the
// viewport or the document, and those differ by the scroll offset. A frame that
// fails this is dropped rather than shipped, because a caller acting on a bad
// frame gets a plausible-looking coordinate and clicks the wrong element.
func (f WireCoordinateFrame) Validate() error {
	if !known(f.Capture) {
		return fmt.Errorf("unknown capture kind %q (want one of %v)", f.Capture, CaptureKinds())
	}
	if f.ImageWidth <= 0 || f.ImageHeight <= 0 {
		return fmt.Errorf("image is %gx%g; a frame cannot describe an image with no pixels", f.ImageWidth, f.ImageHeight)
	}
	if f.ViewportWidth <= 0 || f.ViewportHeight <= 0 {
		return fmt.Errorf("viewport is %gx%g CSS px; without it no image pixel has a clickable coordinate",
			f.ViewportWidth, f.ViewportHeight)
	}
	if f.ImageToViewport.ScaleX <= 0 || f.ImageToViewport.ScaleY <= 0 {
		return fmt.Errorf("scale is %g x %g; a non-positive scale maps every pixel of the image to one point",
			f.ImageToViewport.ScaleX, f.ImageToViewport.ScaleY)
	}
	return nil
}

// uniform reports whether the two scales agree closely enough that a caller may
// use either one for both axes. It is not part of Validate: a non-uniform frame is
// still correct, it just cannot be simplified.
func (f WireCoordinateFrame) uniform() bool {
	sx, sy := f.ImageToViewport.ScaleX, f.ImageToViewport.ScaleY
	if sx <= 0 || sy <= 0 {
		return false
	}
	larger := sx
	if sy > larger {
		larger = sy
	}
	return abs(sx-sy)/larger <= scaleAgreementTolerance
}

func known(kind string) bool {
	for _, candidate := range CaptureKinds() {
		if candidate == kind {
			return true
		}
	}
	return false
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

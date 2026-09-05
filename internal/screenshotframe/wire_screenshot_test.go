// wire_screenshot_test.go — Holds the coordinate frame to the one promise it makes:
// a pixel read off the returned image maps to the coordinate that hits the element.

package screenshotframe

import (
	"math"
	"strings"
	"testing"
)

// retinaViewport is the case the bead names: a 1280x720 CSS viewport photographed
// at devicePixelRatio 2, so the image is 2560x1440 and a caller using image pixels
// directly would click at twice the intended coordinate.
func retinaViewport() WireCoordinateFrame {
	return WireCoordinateFrame{
		Capture:               CaptureViewport,
		ImageWidth:            2560,
		ImageHeight:           1440,
		ViewportWidth:         1280,
		ViewportHeight:        720,
		DocumentWidth:         1280,
		DocumentHeight:        4000,
		DevicePixelRatio:      2,
		Clipped:               true,
		ImageToViewport:       WireImageToViewport{ScaleX: 0.5, ScaleY: 0.5},
		ViewportBoundsInImage: WireImageRect{Width: 2560, Height: 1440},
	}
}

// scrolledFullPage is a whole-document capture taken while the page was scrolled
// 900 CSS px down. Pixel (0,0) of the image is the document origin, which is 900px
// above the top of the screen.
func scrolledFullPage() WireCoordinateFrame {
	return WireCoordinateFrame{
		Capture:               CaptureFullPage,
		ImageWidth:            1280,
		ImageHeight:           4000,
		ViewportWidth:         1280,
		ViewportHeight:        720,
		ScrollX:               0,
		ScrollY:               900,
		DocumentWidth:         1280,
		DocumentHeight:        4000,
		DevicePixelRatio:      2,
		Clipped:               false,
		ImageToViewport:       WireImageToViewport{ScaleX: 1, ScaleY: 1, OffsetY: -900},
		ViewportBoundsInImage: WireImageRect{Y: 900, Width: 1280, Height: 720},
	}
}

func closeTo(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %g, want %g", label, got, want)
	}
}

func TestViewportPointUndoesTheDevicePixelRatio(t *testing.T) {
	t.Parallel()
	frame := retinaViewport()

	// A button drawn at CSS (400, 300) appears at image pixel (800, 600) on a 2x
	// display. Reading that pixel back must land on the button, not at (800, 600)
	// CSS, which on this viewport is a different element entirely.
	x, y := frame.viewportPoint(800, 600)
	closeTo(t, "x", x, 400)
	closeTo(t, "y", y, 300)

	// Discriminating control: the identity mapping a caller would use without a
	// frame gives a different, wrong answer. Without this the assertion above would
	// also hold for a frame whose scale was 1.
	if gotX, _ := (WireCoordinateFrame{ImageToViewport: WireImageToViewport{ScaleX: 1, ScaleY: 1}}).viewportPoint(800, 600); gotX == x {
		t.Fatal("control: an unscaled frame must not agree with the retina frame, or the test proves nothing")
	}
}

func TestViewportPointOfAFullPageImageSubtractsTheScroll(t *testing.T) {
	t.Parallel()
	frame := scrolledFullPage()

	// A heading at document y=1000 is 100px below the top of the screen while
	// scrolled to 900.
	_, y := frame.viewportPoint(0, 1000)
	closeTo(t, "y", y, 100)
	if !frame.onScreen(0, 1000) {
		t.Error("document y=1000 is inside the scrolled window and must read as on screen")
	}

	// A heading at document y=100 is 800px ABOVE the screen. The arithmetic still
	// answers, and the answer is negative — which is the whole point: acting on it
	// without scrolling would click at y=-800, i.e. nothing.
	_, above := frame.viewportPoint(0, 100)
	closeTo(t, "y above the fold", above, -800)
	if frame.onScreen(0, 100) {
		t.Error("document y=100 is scrolled off the top and must NOT read as on screen")
	}
}

func TestOnScreenAgreesWithViewportBoundsInImage(t *testing.T) {
	t.Parallel()
	frame := scrolledFullPage()
	bounds := frame.ViewportBoundsInImage

	// Every corner of the reported bounds must be on screen, and a pixel just
	// outside must not be. The two fields are separate answers to the same
	// question, and a caller that trusts one while the other disagrees misclicks.
	corners := [][2]float64{
		{bounds.X, bounds.Y},
		{bounds.X + bounds.Width, bounds.Y},
		{bounds.X, bounds.Y + bounds.Height},
		{bounds.X + bounds.Width, bounds.Y + bounds.Height},
	}
	for _, corner := range corners {
		if !frame.onScreen(corner[0], corner[1]) {
			t.Errorf("corner %v of viewport_bounds_in_image reads as off screen", corner)
		}
	}
	if frame.onScreen(bounds.X, bounds.Y+bounds.Height+1) {
		t.Error("a pixel below the reported bounds must read as off screen")
	}
}

func TestNoteMatchesTheArithmetic(t *testing.T) {
	t.Parallel()
	// The note is what a caller reads instead of this file, so it must describe the
	// function that is actually applied. Both names appear in it, and the full-page
	// note additionally warns that the answer may be off screen — which is the case
	// TestViewportPointOfAFullPageImageSubtractsTheScroll proves is real.
	viewportNote := retinaViewport().WithNote().Note
	for _, term := range []string{"scale_x", "offset_x", "CSS pixels"} {
		if !strings.Contains(viewportNote, term) {
			t.Errorf("viewport note omits %q: %s", term, viewportNote)
		}
	}
	fullPageNote := scrolledFullPage().WithNote().Note
	if !strings.Contains(fullPageNote, "scroll to it") {
		t.Errorf("full-page note does not tell the caller to scroll: %s", fullPageNote)
	}
	if viewportNote == fullPageNote {
		t.Error("a full-page image and a viewport image do not carry the same risk and must not carry the same note")
	}
}

func TestValidateRejectsFramesThatWouldMisplaceAClick(t *testing.T) {
	t.Parallel()

	// Control: the good frame passes, so a rejection below is the specific defect
	// and not a Validate that refuses everything.
	if err := retinaViewport().Validate(); err != nil {
		t.Fatalf("control: a measured retina frame must validate, got %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(*WireCoordinateFrame)
		wantSub string
	}{
		{"zero scale collapses every pixel to one point", func(f *WireCoordinateFrame) {
			f.ImageToViewport.ScaleX = 0
		}, "non-positive scale"},
		{"negative scale mirrors the image", func(f *WireCoordinateFrame) {
			f.ImageToViewport.ScaleY = -0.5
		}, "non-positive scale"},
		{"an image with no pixels has no coordinates", func(f *WireCoordinateFrame) {
			f.ImageWidth = 0
		}, "no pixels"},
		{"without a viewport nothing is clickable", func(f *WireCoordinateFrame) {
			f.ViewportHeight = 0
		}, "no image pixel has a clickable coordinate"},
		{"an unknown kind hides whether the origin is the document", func(f *WireCoordinateFrame) {
			f.Capture = "thumbnail"
		}, "unknown capture kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			frame := retinaViewport()
			tc.mutate(&frame)
			err := frame.Validate()
			if err == nil {
				t.Fatalf("frame %+v validated; it would have placed clicks silently wrong", frame)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not explain the defect (want substring %q)", err, tc.wantSub)
			}
		})
	}
}

func TestUniformSeparatesRoundingFromDistortion(t *testing.T) {
	t.Parallel()

	// Rounding: a 1281 CSS px viewport photographed into 2560 image px gives
	// slightly different scales on the two axes, and that is still one image.
	rounded := retinaViewport()
	rounded.ImageToViewport.ScaleY = 0.5004
	if !rounded.uniform() {
		t.Error("sub-pixel rounding must not read as distortion")
	}

	// Distortion: a capture whose height was clamped to Chrome's texture limit is
	// squashed on one axis. A caller reusing scale_x for y would put every click in
	// the lower half of the image too high.
	clamped := retinaViewport()
	clamped.ImageToViewport.ScaleY = 1.9
	if clamped.uniform() {
		t.Error("a 3.8x difference between the axes must read as non-uniform")
	}
	// The frame is still valid — non-uniform is not broken, just not simplifiable.
	if err := clamped.Validate(); err != nil {
		t.Errorf("a non-uniform frame is still usable via two scales, got %v", err)
	}
}

// screenshot_frame_test.go — Proves the coordinate frame reaches observe intact, and
// that an unusable one is refused rather than forwarded.

package mediaapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/screenshotframe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// postScreenshotBody uploads one screenshot and returns the query result observe
// would read.
func postScreenshotBody(t *testing.T, queryID string, extra map[string]any) map[string]any {
	t.Helper()
	t.Setenv(state.StateDirEnv, t.TempDir())
	captured := capture.NewCapture()
	t.Cleanup(captured.Close)
	handler := New(captured, nil, nil)

	body := map[string]any{
		"data_url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("pixels")),
		"url":      "https://example.test/page",
		"query_id": queryID,
	}
	for k, v := range extra {
		body[k] = v
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/screenshots", strings.NewReader(string(encoded)))
	req.Header.Set("X-Kaboom-Client", "frame-test")
	handler.HandleScreenshot(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d; body=%s", recorder.Code, recorder.Body.String())
	}

	raw, ok := captured.Queries().TakeQueryResult(queryID)
	if !ok {
		t.Fatalf("query %q was never resolved; observe would time out", queryID)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("query result is not JSON: %v (%s)", err, raw)
	}
	return result
}

func usableFrame() map[string]any {
	return map[string]any{
		"capture":            screenshotframe.CaptureViewport,
		"image_width":        2560,
		"image_height":       1440,
		"viewport_width":     1280,
		"viewport_height":    720,
		"document_width":     1280,
		"document_height":    4000,
		"device_pixel_ratio": 2,
		"clipped":            true,
		"image_to_viewport":  map[string]any{"scale_x": 0.5, "scale_y": 0.5, "offset_x": 0, "offset_y": 0},
		"viewport_bounds_in_image": map[string]any{
			"x": 0, "y": 0, "width": 2560, "height": 1440,
		},
	}
}

func TestUsableFrameReachesObserveWithItsNote(t *testing.T) {
	result := postScreenshotBody(t, "query-good", map[string]any{"coordinate_frame": usableFrame()})

	frame, ok := result["coordinate_frame"].(map[string]any)
	if !ok {
		t.Fatalf("coordinate_frame missing from the query result; observe would return an unaddressable image: %#v", result)
	}
	mapping, ok := frame["image_to_viewport"].(map[string]any)
	if !ok {
		t.Fatalf("image_to_viewport missing: %#v", frame)
	}
	if scale, _ := mapping["scale_x"].(float64); scale != 0.5 {
		t.Errorf("scale_x = %v, want 0.5 — a retina image would be clicked at twice the intended coordinate", mapping["scale_x"])
	}

	// The note is attached here rather than by the extension, so the sentence and
	// the arithmetic have one source. Its absence would leave a caller to guess the
	// mapping from field names.
	if note, _ := frame["note"].(string); !strings.Contains(note, "scale_x") {
		t.Errorf("note does not state the mapping: %q", note)
	}
	if _, present := result["coordinate_frame_error"]; present {
		t.Errorf("a usable frame must not also report an error: %#v", result["coordinate_frame_error"])
	}
}

func TestUnusableFrameIsRefusedNotForwarded(t *testing.T) {
	broken := usableFrame()
	broken["image_to_viewport"] = map[string]any{"scale_x": 0, "scale_y": 0, "offset_x": 0, "offset_y": 0}

	result := postScreenshotBody(t, "query-broken", map[string]any{"coordinate_frame": broken})

	// A zero scale maps every pixel of the image to the same point. Forwarded, it
	// looks exactly like a good frame — every coordinate read off the screenshot
	// would resolve to (0,0) and click whatever is in the top-left corner.
	if _, present := result["coordinate_frame"]; present {
		t.Fatalf("a frame with a zero scale was forwarded: %#v", result["coordinate_frame"])
	}
	reason, _ := result["coordinate_frame_error"].(string)
	if !strings.Contains(reason, "non-positive scale") {
		t.Errorf("coordinate_frame_error = %q; it must name the defect", reason)
	}
}

func TestExtensionReportedAbsenceIsPassedThrough(t *testing.T) {
	result := postScreenshotBody(t, "query-absent", map[string]any{
		"coordinate_frame_error": "viewport_metrics_unavailable",
	})

	if _, present := result["coordinate_frame"]; present {
		t.Fatal("no frame was uploaded, so none may appear in the result")
	}
	if reason, _ := result["coordinate_frame_error"].(string); reason != "viewport_metrics_unavailable" {
		t.Errorf("coordinate_frame_error = %q, want the extension's own reason", reason)
	}
}

func TestScreenshotWithoutAFrameStillDelivers(t *testing.T) {
	// Control: the image is the primary product. A capture that could not be
	// measured must still reach the caller, or a browser where the metrics probe is
	// blocked would return no screenshot at all.
	result := postScreenshotBody(t, "query-plain", nil)

	if data, _ := result["data_url"].(string); !strings.HasPrefix(data, "data:image/jpeg;base64,") {
		t.Fatalf("data_url missing from an unframed screenshot: %#v", result)
	}
	for _, key := range []string{"coordinate_frame", "coordinate_frame_error"} {
		if _, present := result[key]; present {
			t.Errorf("%s appeared for an upload that carried neither: %#v", key, result[key])
		}
	}
}

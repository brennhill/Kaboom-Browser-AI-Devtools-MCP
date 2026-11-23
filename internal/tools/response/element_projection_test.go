// element_projection_test.go — Contracts for the lean element projection.
package toolresp

import "testing"

// Element listings were sized for completeness rather than for a model's
// context budget: ~279 bytes per element, against ~22 for the equivalent line
// from chrome-devtools-mcp. Most of the difference is data no caller reads when
// it is choosing what to click — a bounding box, a landmark tag, an overlay
// flag. Lean by default, everything on request.
func TestProjectInteractiveElementsIsLeanByDefault(t *testing.T) {
	full := map[string]any{
		"elements": []any{
			map[string]any{
				"bbox":          map[string]any{"x": 20.0, "y": 10.0, "width": 94.0, "height": 28.0},
				"element_id":    "el_1",
				"element_type":  "link",
				"in_overlay":    true,
				"index":         0.0,
				"label":         "Kaboom",
				"landmark_tag":  "header",
				"landmark_role": "banner",
				"selector":      "text=Kaboom:nth-match(1)",
				"tag":           "a",
				"visible":       true,
			},
		},
	}

	lean := projectElementCollections(full, false)
	elements, _ := lean["elements"].([]any)
	if len(elements) != 1 {
		t.Fatalf("projection dropped the collection: %+v", lean)
	}
	got, _ := elements[0].(map[string]any)

	for _, keep := range []string{"element_id", "element_type", "label", "selector", "index"} {
		if _, ok := got[keep]; !ok {
			t.Errorf("lean projection must keep %q — an agent targets with it", keep)
		}
	}
	for _, drop := range []string{"bbox", "landmark_tag", "landmark_role", "in_overlay", "tag"} {
		if _, ok := got[drop]; ok {
			t.Errorf("lean projection must drop %q by default", drop)
		}
	}
	// visible=true is the overwhelmingly common case and says nothing; only the
	// exception is worth bytes.
	if _, ok := got["visible"]; ok {
		t.Error("lean projection must omit visible when the element is visible")
	}
}

func TestProjectInteractiveElementsKeepsEverythingWhenVerbose(t *testing.T) {
	full := map[string]any{
		"elements": []any{
			map[string]any{"element_id": "el_1", "bbox": map[string]any{"x": 1.0}, "landmark_tag": "header", "visible": true},
		},
	}
	verbose := projectElementCollections(full, true)
	got := verbose["elements"].([]any)[0].(map[string]any)
	for _, keep := range []string{"element_id", "bbox", "landmark_tag", "visible"} {
		if _, ok := got[keep]; !ok {
			t.Errorf("verbose must keep %q", keep)
		}
	}
}

// A hidden element is the exception the caller needs to know about.
func TestProjectInteractiveElementsKeepsVisibleWhenFalse(t *testing.T) {
	full := map[string]any{
		"elements": []any{map[string]any{"element_id": "el_1", "visible": false}},
	}
	got := projectElementCollections(full, false)["elements"].([]any)[0].(map[string]any)
	if visible, ok := got["visible"].(bool); !ok || visible {
		t.Fatalf("visible=false must survive the lean projection, got %+v", got)
	}
}

// explore_page names its collection differently, and its menu enrichment runs
// before the projection and depends on bbox — so the projection must apply to
// both collections and must not run until enrichment is done.
func TestProjectInteractiveElementsCoversExploreCollection(t *testing.T) {
	full := map[string]any{
		"interactive_elements": []any{
			map[string]any{"element_id": "el_9", "bbox": map[string]any{"x": 1.0}, "label": "Go"},
		},
	}
	got := projectElementCollections(full, false)["interactive_elements"].([]any)[0].(map[string]any)
	if _, ok := got["bbox"]; ok {
		t.Error("explore_page's interactive_elements must be projected too")
	}
	if got["label"] != "Go" {
		t.Errorf("label must survive, got %+v", got)
	}
}

// The real response is a lifecycle envelope with the payload under result, so a
// projection that only walks the top level does nothing in production while
// passing against a flat fixture. It did exactly that until a live measurement
// showed 10,622 bytes unchanged.
func TestProjectInteractiveElementsReachesIntoTheLifecycleEnvelope(t *testing.T) {
	enveloped := map[string]any{
		"correlation_id":   "dom_list_1",
		"lifecycle_status": "complete",
		"result": map[string]any{
			"elements": []any{
				map[string]any{"element_id": "el_1", "label": "Kaboom", "bbox": map[string]any{"x": 20.0}, "tag": "a"},
			},
		},
	}
	got := projectElementCollections(enveloped, false)
	inner := got["result"].(map[string]any)["elements"].([]any)[0].(map[string]any)
	if _, ok := inner["bbox"]; ok {
		t.Error("the projection must reach into result, where every real payload lives")
	}
	if inner["label"] != "Kaboom" {
		t.Errorf("label must survive inside the envelope, got %+v", inner)
	}
}

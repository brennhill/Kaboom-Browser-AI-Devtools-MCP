// snapshot_test.go — Tests for building a snapshot from each discovery action's payload.

package elemindex

import "testing"

func TestTargetsFromElements(t *testing.T) {
	t.Parallel()
	targets := TargetsFromElements([]any{
		map[string]any{"index": float64(0), "selector": "#name", "element_type": "input", "label": "Full name"},
		map[string]any{"index": float64(1), "selector": ".btn-submit", "element_type": "button", "label": "Submit"},
		// No selector: nothing could act on this index, so it must not become a handle.
		map[string]any{"index": float64(2), "selector": "", "element_type": "div"},
		"not an element object",
	})

	if len(targets) != 2 {
		t.Fatalf("targets = %#v, want the two addressable elements", targets)
	}
	if targets[0].Selector != "#name" || targets[0].Role != "input" || targets[0].Name != "Full name" {
		t.Fatalf("index 0 = %+v", targets[0])
	}
	if _, present := targets[2]; present {
		t.Fatal("an element with no selector must not be stored: its index would name nothing")
	}
	// A DOM-scan target has no accessibility id, so it must not answer to a ref.
	if targets[1].AXBackendID != 0 {
		t.Fatalf("index 1 = %+v, want no accessibility handle", targets[1])
	}
}

func TestTargetsFromCandidates(t *testing.T) {
	t.Parallel()
	targets := TargetsFromCandidates([]any{
		map[string]any{
			"ref": "ax_412", "role": "button", "name": "Add to cart",
			"x": float64(100), "y": float64(200), "width": float64(80), "height": float64(40),
		},
		map[string]any{"ref": "ax_413", "role": "link", "name": "Add to cart and checkout"},
		// A candidate whose ref could not be issued is not addressable.
		map[string]any{"ref": "", "role": "button", "name": "Nameless"},
		"not a candidate object",
	})

	if len(targets) != 2 {
		t.Fatalf("targets = %#v, want the two candidates with usable refs", targets)
	}
	// Rank order is the index: position 0 in the response must be index 0 in the snapshot.
	first := targets[0]
	if first.AXBackendID != 412 || first.Role != "button" || first.Name != "Add to cart" {
		t.Fatalf("index 0 = %+v", first)
	}
	// The click point is the centre of the box find resolved, not its top-left corner.
	if !first.HasCenter || first.CenterX != 140 || first.CenterY != 220 {
		t.Fatalf("index 0 centre = (%v,%v) has=%v, want (140,220) true", first.CenterX, first.CenterY, first.HasCenter)
	}
	if first.Selector != "" {
		t.Fatalf("an accessibility candidate must carry no selector, got %q", first.Selector)
	}

	// Control for the assertion below: candidate 0 DID resolve a centre, so a missing centre
	// on candidate 1 reflects its absent geometry rather than the conversion doing nothing.
	second := targets[1]
	if second.AXBackendID != 413 {
		t.Fatalf("index 1 = %+v, want backend id 413", second)
	}
	if second.HasCenter || second.CenterX != 0 || second.CenterY != 0 {
		t.Fatalf("index 1 centre = (%v,%v) has=%v; a candidate with no box must report no centre — clicking 0,0 would hit the page corner", second.CenterX, second.CenterY, second.HasCenter)
	}
}

func TestTargetsFromCandidates_ZeroSizedBoxStillHasACentre(t *testing.T) {
	t.Parallel()
	// x and y present with no width/height is a real point, not missing geometry. Treating it
	// as absent would drop a collapsed-but-clickable control.
	targets := TargetsFromCandidates([]any{
		map[string]any{"ref": "ax_5", "x": float64(0), "y": float64(0)},
	})
	if !targets[0].HasCenter || targets[0].CenterX != 0 || targets[0].CenterY != 0 {
		t.Fatalf("target = %+v, want a centre at 0,0", targets[0])
	}
}

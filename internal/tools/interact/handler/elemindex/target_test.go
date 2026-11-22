// target_test.go — Tests for the accessibility handle space and its staleness invariant.

package elemindex

import "testing"

func TestParseRef(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		ref     string
		want    int
		wantOK  bool
		comment string
	}{
		{"ax_42", 42, true, "plain handle"},
		{"  ax_42  ", 42, true, "surrounding whitespace"},
		{"ax_0", 0, false, "Chrome never issues backendNodeId 0"},
		{"ax_-1", 0, false, "negative id"},
		{"ax_", 0, false, "no id"},
		{"ax_abc", 0, false, "non-numeric id"},
		{"42", 0, false, "no prefix"},
		{"", 0, false, "empty"},
		{"#submit", 0, false, "a CSS selector is not a ref"},
	} {
		got, ok := parseRef(tc.ref)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("parseRef(%q) = (%d,%v), want (%d,%v) — %s", tc.ref, got, ok, tc.want, tc.wantOK, tc.comment)
		}
	}
}

// The invariant this change exists for: an accessibility ref taken before a re-render must
// be REFUSED after one, not resolved against the new snapshot.
//
// The stale snapshot and the fresh one deliberately BOTH hold backend id 42 at different
// coordinates — that is Chrome reusing a backendNodeId after destroying the node it named.
// Without the generation check the stale ref resolves to (900,900), the coordinates of a
// different control, and the agent clicks it with no error.
func TestResolveRef_StaleGenerationIsRefused(t *testing.T) {
	t.Parallel()
	r := New()
	r.Store("client-a", 3, "gen_old", map[int]Target{
		0: {AXBackendID: 42, Role: "button", Name: "Add to cart", CenterX: 100, CenterY: 200, HasCenter: true},
	})

	// Control: the same call against the generation it was issued under DOES resolve, so
	// the refusal below cannot pass merely because ResolveRef never finds anything.
	fresh, ok, stale, _ := r.ResolveRef("client-a", 3, "ax_42", "gen_old")
	if !ok || stale || fresh.CenterX != 100 || fresh.CenterY != 200 || fresh.Name != "Add to cart" {
		t.Fatalf("fresh ResolveRef = (%+v, ok=%v, stale=%v), want the Add to cart target at 100,200", fresh, ok, stale)
	}

	// The page re-renders. backendNodeId 42 now names a different control.
	r.Store("client-a", 3, "gen_new", map[int]Target{
		0: {AXBackendID: 42, Role: "button", Name: "Delete account", CenterX: 900, CenterY: 900, HasCenter: true},
	})

	got, ok, stale, latest := r.ResolveRef("client-a", 3, "ax_42", "gen_old")
	if ok {
		t.Fatalf("stale ref resolved to %+v; it must be refused", got)
	}
	if got.CenterX != 0 || got.CenterY != 0 || got.Name != "" {
		t.Fatalf("refused ResolveRef returned target data %+v, want the zero Target", got)
	}
	if !stale {
		t.Fatal("ResolveRef did not report the generation as stale")
	}
	if latest != "gen_new" {
		t.Fatalf("latest generation = %q, want gen_new", latest)
	}
}

func TestResolveRef_UnknownRefIsNotFoundNotStale(t *testing.T) {
	t.Parallel()
	r := New()
	r.Store("client-a", 0, "gen_1", map[int]Target{0: {AXBackendID: 42}})

	// Control: ax_42 IS resolvable in this snapshot.
	if _, ok, _, _ := r.ResolveRef("client-a", 0, "ax_42", "gen_1"); !ok {
		t.Fatal("control: ax_42 must resolve in the snapshot that holds it")
	}
	_, ok, stale, latest := r.ResolveRef("client-a", 0, "ax_99", "gen_1")
	if ok {
		t.Fatal("ax_99 is not in the snapshot; it must not resolve")
	}
	if stale {
		t.Fatal("a ref that is simply absent must not be reported as a generation conflict")
	}
	if latest != "gen_1" {
		t.Fatalf("latest generation = %q, want gen_1", latest)
	}
}

func TestResolveRef_MalformedRefIsRejected(t *testing.T) {
	t.Parallel()
	r := New()
	r.Store("client-a", 0, "gen_1", map[int]Target{0: {AXBackendID: 42, Selector: "#submit"}})

	// Control: a well-formed ref against the same snapshot resolves.
	if _, ok, _, _ := r.ResolveRef("client-a", 0, "ax_42", ""); !ok {
		t.Fatal("control: ax_42 must resolve")
	}
	// A selector is not a ref. Resolving it would let "#submit" address whichever target
	// happened to be first in the map.
	if _, ok, _, _ := r.ResolveRef("client-a", 0, "#submit", ""); ok {
		t.Fatal("a CSS selector must not resolve as an accessibility ref")
	}
}

func TestResolveRef_ScopedByClientAndTab(t *testing.T) {
	t.Parallel()
	r := New()
	r.Store("client-a", 3, "gen_a", map[int]Target{0: {AXBackendID: 42, Name: "A"}})
	r.Store("client-b", 3, "gen_b", map[int]Target{0: {AXBackendID: 42, Name: "B"}})

	got, ok, _, _ := r.ResolveRef("client-a", 3, "ax_42", "")
	if !ok || got.Name != "A" {
		t.Fatalf("client-a ResolveRef = (%+v, %v), want name A", got, ok)
	}
	got, ok, _, _ = r.ResolveRef("client-b", 3, "ax_42", "")
	if !ok || got.Name != "B" {
		t.Fatalf("client-b ResolveRef = (%+v, %v), want name B", got, ok)
	}
	// A ref issued for tab 3 must not answer for tab 4: same handle, different page.
	if _, ok, _, _ := r.ResolveRef("client-a", 4, "ax_42", ""); ok {
		t.Fatal("ax_42 from tab 3 must not resolve against tab 4")
	}
}

func TestResolveRef_NilReceiverIsSafe(t *testing.T) {
	t.Parallel()
	var r *Registry
	if _, ok, _, _ := r.ResolveRef("client-a", 0, "ax_42", ""); ok {
		t.Error("ResolveRef on a nil Registry must report not found")
	}
}

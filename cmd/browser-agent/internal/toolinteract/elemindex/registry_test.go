// registry_test.go — Tests for the scoped element index and its staleness invariant.

package elemindex

import (
	"strings"
	"testing"
)

func TestResolve_EmptyRegistry(t *testing.T) {
	t.Parallel()
	r := New()
	if _, ok, _, _ := r.Resolve("client-a", 0, 0, ""); ok {
		t.Error("expected not found on empty registry")
	}
}

func TestResolve_NilReceiverIsSafe(t *testing.T) {
	t.Parallel()
	var r *Registry
	if gen := r.Store("client-a", 0, "gen", map[int]string{1: "#a"}); gen != "" {
		t.Errorf("Store on nil Registry = %q, want empty", gen)
	}
	if _, ok, _, _ := r.Resolve("client-a", 0, 1, ""); ok {
		t.Error("Resolve on nil Registry should report not found")
	}
}

func TestStore_GeneratesGenerationWhenEmpty(t *testing.T) {
	t.Parallel()
	r := New()
	gen := r.Store("client-a", 0, "", map[int]string{1: "#a"})
	if !strings.HasPrefix(gen, "idx_") {
		t.Fatalf("Store(generation=\"\") = %q, want a generated idx_ stamp", gen)
	}
	// The generated stamp must be usable as a Resolve comparand.
	if sel, ok, stale, _ := r.Resolve("client-a", 0, 1, gen); !ok || stale || sel != "#a" {
		t.Fatalf("Resolve with generated stamp = (%q,%v,%v), want (#a,true,false)", sel, ok, stale)
	}
}

func TestStore_ClonesCallerMap(t *testing.T) {
	t.Parallel()
	r := New()
	selectors := map[int]string{1: "#a"}
	r.Store("client-a", 0, "gen_1", selectors)
	selectors[1] = "#mutated-after-store"
	if sel, _, _, _ := r.Resolve("client-a", 0, 1, "gen_1"); sel != "#a" {
		t.Fatalf("Resolve = %q, want #a — Store must not alias the caller's map", sel)
	}
}

func TestResolve_ScopedByClientAndTab(t *testing.T) {
	t.Parallel()
	r := New()
	r.Store("client-a", 0, "gen_a", map[int]string{1: "#a"})
	r.Store("client-b", 0, "gen_b", map[int]string{1: "#b"})
	r.Store("client-a", 9, "gen_a9", map[int]string{1: "#a9"})

	for _, tc := range []struct {
		clientID string
		tabID    int
		want     string
	}{
		{"client-a", 0, "#a"},
		{"client-b", 0, "#b"},
		{"client-a", 9, "#a9"},
	} {
		if sel, ok, _, _ := r.Resolve(tc.clientID, tc.tabID, 1, ""); !ok || sel != tc.want {
			t.Errorf("Resolve(%q, %d) = %q ok=%v, want %q", tc.clientID, tc.tabID, sel, ok, tc.want)
		}
	}
}

func TestResolve_BlankClientIDNormalizesToSameScope(t *testing.T) {
	t.Parallel()
	r := New()
	r.Store("  ", 0, "gen_1", map[int]string{1: "#a"})
	if sel, ok, _, _ := r.Resolve("", 0, 1, ""); !ok || sel != "#a" {
		t.Fatalf("Resolve(\"\") = %q ok=%v, want #a — blank ids must share one scope", sel, ok)
	}
}

// The invariant this package exists for: an index quoted against an older
// generation must be refused, not silently answered from the newer snapshot.
func TestResolve_StaleGenerationIsRefused(t *testing.T) {
	t.Parallel()
	r := New()
	r.Store("client-a", 0, "gen_old", map[int]string{1: "#old"})
	r.Store("client-a", 0, "gen_new", map[int]string{1: "#new"})

	sel, ok, stale, latest := r.Resolve("client-a", 0, 1, "gen_old")
	if ok || sel != "" {
		t.Fatalf("Resolve with stale generation = %q ok=%v, want refused", sel, ok)
	}
	if !stale {
		t.Fatal("Resolve did not report the generation as stale")
	}
	if latest != "gen_new" {
		t.Fatalf("latest generation = %q, want gen_new", latest)
	}
}

func TestResolve_EmptyGenerationSkipsStalenessCheck(t *testing.T) {
	t.Parallel()
	r := New()
	r.Store("client-a", 0, "gen_new", map[int]string{1: "#new"})
	sel, ok, stale, _ := r.Resolve("client-a", 0, 1, "")
	if !ok || stale || sel != "#new" {
		t.Fatalf("Resolve(generation=\"\") = (%q,%v,%v), want (#new,true,false)", sel, ok, stale)
	}
}

func TestFormatGenerationConflict(t *testing.T) {
	t.Parallel()
	if !strings.Contains(FormatGenerationConflict("", ""), "generation mismatch") {
		t.Fatal("expected generic message")
	}
	msg := FormatGenerationConflict("old", "new")
	if !strings.Contains(msg, "old") || !strings.Contains(msg, "new") {
		t.Fatalf("FormatGenerationConflict(old,new) = %q, want both generations named", msg)
	}
}

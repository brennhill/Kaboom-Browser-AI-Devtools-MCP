// store_test.go — Verifies bounded enhanced-action ownership and navigation signaling.
package actionstore

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestStoreOwnsNestedInputAndReturnsDetachedEvidence(t *testing.T) {
	store := New(2)
	actions := []types.EnhancedAction{{
		Type: "click", Selectors: map[string]any{
			"aria": map[string]any{"role": "button"},
		},
		TestIDs: []string{"test-1"},
	}}
	store.Add(actions, time.Unix(1700000000, 0))
	actions[0].Selectors["aria"].(map[string]any)["role"] = "mutated"
	actions[0].TestIDs[0] = "mutated"

	snapshot := store.Snapshot()
	if snapshot.Actions[0].Selectors["aria"].(map[string]any)["role"] != "button" || snapshot.Actions[0].TestIDs[0] != "test-1" {
		t.Fatalf("retained action aliases caller input: %+v", snapshot.Actions[0])
	}
	snapshot.Actions[0].Selectors["aria"].(map[string]any)["role"] = "snapshot-mutated"
	if store.Snapshot().Actions[0].Selectors["aria"].(map[string]any)["role"] != "button" {
		t.Fatal("retained action aliases returned evidence")
	}
}

func TestStoreTracksNavigationEvictionStatsAndClear(t *testing.T) {
	store := New(2)
	now := time.Now().Add(-time.Second)
	if navigated := store.Add([]types.EnhancedAction{{Type: "click"}, {Type: "navigation"}, {Type: "input"}}, now); !navigated {
		t.Fatal("Add did not report retained navigation input")
	}
	stats := store.Stats()
	if stats.Count != 2 || stats.Capacity != 2 || stats.TotalAdded != 3 || stats.Pressure.Dropped != 1 {
		t.Fatalf("Stats() = %+v, want count=2 capacity=2 total=3 dropped=1", stats)
	}
	if stats.Pressure.OldestAge < time.Second/2 {
		t.Fatalf("oldest age = %v, want positive age", stats.Pressure.OldestAge)
	}
	if allocations := testing.AllocsPerRun(1000, func() { _ = store.Stats() }); allocations != 0 {
		t.Fatalf("Stats allocated %.2f times per call, want 0", allocations)
	}
	if removed := store.Clear(); removed != 2 {
		t.Fatalf("Clear() = %d, want 2", removed)
	}
	cleared := store.Stats()
	if cleared.Count != 0 || cleared.TotalAdded != 0 || cleared.Pressure.Dropped != 1 {
		t.Fatalf("cleared Stats() = %+v, want empty/reset total/preserved drops", cleared)
	}
}

func TestStoreDetachesLocatorAndEnvironmentPointers(t *testing.T) {
	store := New(4)
	action := types.EnhancedAction{
		Type:     "click",
		AX:       &types.WireAXLocator{Ref: "ax_1", Role: "button", Name: "Save"},
		Viewport: &types.WireViewportLocator{X: 10, Y: 20},
		Environment: &types.WireEnvironmentPin{
			Clock:      &types.WireClockPin{EpochMs: 1000, TimezoneID: "UTC"},
			Viewport:   &types.WireViewportPin{Width: 800, Height: 600},
			Unpinned:   []string{"network"},
			RandomSeed: "seed",
		},
	}
	store.Add([]types.EnhancedAction{action}, time.Now())

	// Sharing these pointers would let a later ingest rewrite evidence a caller already
	// holds, so a generated test would describe a locator or a pin that never applied.
	action.AX.Name = "mutated"
	action.Viewport.X = 999
	action.Environment.Clock.TimezoneID = "mutated"
	action.Environment.Unpinned[0] = "mutated"

	retained := store.Snapshot().Actions[0]
	if retained.AX.Name != "Save" {
		t.Errorf("retained AX name = %q, want %q", retained.AX.Name, "Save")
	}
	if retained.Viewport.X != 10 {
		t.Errorf("retained viewport x = %d, want 10", retained.Viewport.X)
	}
	if retained.Environment.Clock.TimezoneID != "UTC" {
		t.Errorf("retained timezone = %q, want UTC", retained.Environment.Clock.TimezoneID)
	}
	if retained.Environment.Unpinned[0] != "network" {
		t.Errorf("retained unpinned[0] = %q, want network", retained.Environment.Unpinned[0])
	}
}

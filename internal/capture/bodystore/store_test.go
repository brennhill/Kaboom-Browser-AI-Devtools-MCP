// store_test.go — Verifies bounded network-body ownership and pressure accounting.
package bodystore

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestStoreOwnsMutableInputAndReturnsDetachedSnapshots(t *testing.T) {
	store := New(2, 1<<20)
	now := time.Unix(1700000000, 0).UTC()
	bodies := []types.NetworkBody{{
		URL:             "https://example.test",
		ResponseHeaders: map[string]string{"x-test": "original"},
		TestIDs:         []string{"test-1"},
	}}
	store.Add(bodies, now)
	bodies[0].ResponseHeaders["x-test"] = "caller-mutated"
	bodies[0].TestIDs[0] = "caller-mutated"

	snapshot := store.Snapshot()
	if snapshot.Bodies[0].ResponseHeaders["x-test"] != "original" || snapshot.Bodies[0].TestIDs[0] != "test-1" {
		t.Fatalf("retained body aliases caller input: %+v", snapshot.Bodies[0])
	}
	snapshot.Bodies[0].ResponseHeaders["x-test"] = "snapshot-mutated"
	snapshot.Bodies[0].TestIDs[0] = "snapshot-mutated"
	again := store.Snapshot()
	if again.Bodies[0].ResponseHeaders["x-test"] != "original" || again.Bodies[0].TestIDs[0] != "test-1" {
		t.Fatalf("retained body aliases returned snapshot: %+v", again.Bodies[0])
	}
	if len(snapshot.Timestamps) != 1 || !snapshot.Timestamps[0].Equal(now) {
		t.Fatalf("timestamps = %v, want [%v]", snapshot.Timestamps, now)
	}
}

func TestStoreTracksEvictionErrorsMemoryAndClear(t *testing.T) {
	store := New(2, 1<<20)
	now := time.Now().Add(-time.Second)
	store.Add([]types.NetworkBody{
		{URL: "a", Status: 200, RequestBody: "a"},
		{URL: "b", Status: 500, ResponseBody: "bb"},
		{URL: "c", Status: 404, ResponseBody: "ccc"},
	}, now)
	snapshot := store.Snapshot()
	if snapshot.TotalAdded != 3 || snapshot.ErrorTotalAdded != 2 {
		t.Fatalf("totals = (%d, %d), want (3, 2)", snapshot.TotalAdded, snapshot.ErrorTotalAdded)
	}
	if snapshot.Pressure.Size != 2 || snapshot.Pressure.Capacity != 2 || snapshot.Pressure.Dropped != 1 {
		t.Fatalf("pressure = %+v, want size=2 capacity=2 dropped=1", snapshot.Pressure)
	}
	if snapshot.MemoryBytes <= 0 {
		t.Fatalf("memory = %d, want positive", snapshot.MemoryBytes)
	}
	if removed := store.Clear(); removed != 2 {
		t.Fatalf("Clear() = %d, want 2", removed)
	}
	cleared := store.Snapshot()
	if len(cleared.Bodies) != 0 || cleared.TotalAdded != 0 || cleared.ErrorTotalAdded != 0 || cleared.MemoryBytes != 0 {
		t.Fatalf("cleared snapshot = %+v", cleared)
	}
	if cleared.Pressure.Dropped != 1 {
		t.Fatalf("Clear reset cumulative dropped count: %+v", cleared.Pressure)
	}
}

func TestStoreEvictsInOnePassForMemoryPressure(t *testing.T) {
	store := New(4, 1)
	store.Add([]types.NetworkBody{{URL: "a", ResponseBody: "payload"}}, time.Now())
	snapshot := store.Snapshot()
	if len(snapshot.Bodies) != 0 || snapshot.MemoryBytes != 0 || snapshot.Pressure.Dropped != 1 {
		t.Fatalf("memory-pressure snapshot = %+v, want empty with one drop", snapshot)
	}
}

func TestStoreStatsDoNotCopyRetainedBodies(t *testing.T) {
	store := New(2, 1<<20)
	store.Add([]types.NetworkBody{{URL: "a", ResponseBody: "payload"}}, time.Now())
	allocations := testing.AllocsPerRun(1000, func() {
		stats := store.Stats()
		if stats.Count != 1 || stats.TotalAdded != 1 {
			panic("unexpected stats")
		}
	})
	if allocations != 0 {
		t.Fatalf("Stats allocated %.2f times per call, want 0", allocations)
	}
}

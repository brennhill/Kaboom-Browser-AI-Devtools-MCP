// store_test.go — Verifies bounded waterfall retention and detached snapshots.
package waterfallstore

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestStoreAddTagsAndEvicts(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	store := New(2)
	store.addAt([]types.NetworkWaterfallEntry{
		{URL: "https://a.test/a"},
		{URL: "https://a.test/b"},
		{URL: "https://a.test/c"},
	}, "https://app.local/dashboard", now)

	entries := store.Entries()
	if len(entries) != 2 || entries[0].URL != "https://a.test/b" || entries[1].URL != "https://a.test/c" {
		t.Fatalf("bounded entries = %+v, want b then c", entries)
	}
	for i, entry := range entries {
		if entry.PageURL != "https://app.local/dashboard" || !entry.Timestamp.Equal(now) {
			t.Fatalf("entry[%d] tagging = %+v, want page and controlled timestamp", i, entry)
		}
	}
	if got := store.Pressure(); got.Size != 2 || got.Capacity != 2 || got.Dropped != 1 {
		t.Fatalf("Pressure() = %+v, want size=2 capacity=2 dropped=1", got)
	}
}

func TestStoreEntriesDetachedAndClear(t *testing.T) {
	now := time.Unix(1700000100, 0).UTC()
	store := New(2)
	store.addAt([]types.NetworkWaterfallEntry{{URL: "https://a.test/a"}}, "https://app.local", now)

	snapshot := store.Entries()
	snapshot[0].URL = "mutated"
	if got := store.Entries()[0].URL; got != "https://a.test/a" {
		t.Fatalf("store mutated through snapshot; URL=%q", got)
	}
	if removed := store.Clear(); removed != 1 {
		t.Fatalf("Clear() = %d, want 1", removed)
	}
	if entries := store.Entries(); len(entries) != 0 {
		t.Fatalf("Entries() after Clear = %+v, want empty", entries)
	}
}

func TestStoreDoesNotMutateCallerEntries(t *testing.T) {
	store := New(1)
	entries := []types.NetworkWaterfallEntry{{
		URL:            "https://a.test/a",
		ServerTiming:   []types.WireServerTiming{{Name: "db"}},
		InitiatorStack: []string{"caller"},
	}}
	store.Add(entries, "https://app.local")
	if entries[0].PageURL != "" || !entries[0].Timestamp.IsZero() {
		t.Fatalf("Add mutated caller entry: %+v", entries[0])
	}
	entries[0].ServerTiming[0].Name = "mutated"
	entries[0].InitiatorStack[0] = "mutated"
	snapshot := store.Entries()
	if snapshot[0].ServerTiming[0].Name != "db" || snapshot[0].InitiatorStack[0] != "caller" {
		t.Fatalf("stored entry aliases caller slices: %+v", snapshot[0])
	}
	snapshot[0].ServerTiming[0].Name = "snapshot-mutated"
	snapshot[0].InitiatorStack[0] = "snapshot-mutated"
	again := store.Entries()
	if again[0].ServerTiming[0].Name != "db" || again[0].InitiatorStack[0] != "caller" {
		t.Fatalf("snapshot aliases retained slices: %+v", again[0])
	}
}

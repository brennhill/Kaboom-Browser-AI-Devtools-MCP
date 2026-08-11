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

// A page's resource-timing table is a snapshot, not a stream: every read of
// observe(network_waterfall) re-queries the page and re-sends the whole table.
// Appending it unconditionally meant reading the data corrupted it — measured
// live at 32 stored entries for 8 real requests, growing on every poll, with
// real requests evicted from the ring by copies of themselves.
func TestStoreIngestingTheSameSnapshotTwiceIsIdempotent(t *testing.T) {
	now := time.Unix(1700000200, 0).UTC()
	snapshot := []types.NetworkWaterfallEntry{
		{Name: "https://a.test/404", URL: "https://a.test/404", StartTime: 10.5, ResponseEnd: 20.5},
		{Name: "https://a.test/500", URL: "https://a.test/500", StartTime: 11.5, ResponseEnd: 21.5},
	}
	store := New(100)
	store.addAt(snapshot, "https://app.local/page", now)
	store.addAt(snapshot, "https://app.local/page", now.Add(3*time.Second))
	store.addAt(snapshot, "https://app.local/page", now.Add(6*time.Second))

	if got := len(store.Entries()); got != 2 {
		t.Fatalf("re-ingesting an unchanged snapshot stored %d entries, want 2", got)
	}
}

func TestStoreKeepsGenuinelyNewRequestsToTheSameURL(t *testing.T) {
	now := time.Unix(1700000300, 0).UTC()
	store := New(100)
	// Same URL fetched twice: distinct startTime makes these distinct requests,
	// and dedup must not collapse them.
	store.addAt([]types.NetworkWaterfallEntry{
		{Name: "https://a.test/poll", URL: "https://a.test/poll", StartTime: 10, ResponseEnd: 12},
	}, "https://app.local/page", now)
	store.addAt([]types.NetworkWaterfallEntry{
		{Name: "https://a.test/poll", URL: "https://a.test/poll", StartTime: 10, ResponseEnd: 12},
		{Name: "https://a.test/poll", URL: "https://a.test/poll", StartTime: 99, ResponseEnd: 101},
	}, "https://app.local/page", now.Add(time.Second))

	entries := store.Entries()
	if len(entries) != 2 {
		t.Fatalf("stored %d entries, want 2 (one deduped, one genuinely new)", len(entries))
	}
	if entries[1].StartTime != 99 {
		t.Fatalf("newest entry StartTime = %v, want the new request at 99", entries[1].StartTime)
	}
}

func TestStoreSeparatesIdenticalTimingsAcrossPages(t *testing.T) {
	now := time.Unix(1700000400, 0).UTC()
	store := New(100)
	// startTime is relative to each page's own timeOrigin, so two pages can
	// legitimately report the same URL at the same offset.
	entry := []types.NetworkWaterfallEntry{
		{Name: "https://cdn.test/app.js", URL: "https://cdn.test/app.js", StartTime: 5, ResponseEnd: 9},
	}
	store.addAt(entry, "https://app.local/one", now)
	store.addAt(entry, "https://app.local/two", now)

	if got := len(store.Entries()); got != 2 {
		t.Fatalf("stored %d entries, want 2 — identical timings on different pages are different requests", got)
	}
}

func TestStoreDedupSurvivesEviction(t *testing.T) {
	now := time.Unix(1700000500, 0).UTC()
	store := New(2)
	store.addAt([]types.NetworkWaterfallEntry{
		{Name: "a", URL: "a", StartTime: 1},
		{Name: "b", URL: "b", StartTime: 2},
		{Name: "c", URL: "c", StartTime: 3},
	}, "https://app.local", now)
	// 'a' has been evicted. Re-ingesting the same snapshot must not resurrect it
	// as a new entry and push out the live ones.
	store.addAt([]types.NetworkWaterfallEntry{
		{Name: "a", URL: "a", StartTime: 1},
		{Name: "b", URL: "b", StartTime: 2},
		{Name: "c", URL: "c", StartTime: 3},
	}, "https://app.local", now.Add(time.Second))

	entries := store.Entries()
	if len(entries) != 2 || entries[0].URL != "b" || entries[1].URL != "c" {
		t.Fatalf("entries = %+v, want b then c retained", entries)
	}
}

func TestStoreClearAllowsTheSameSnapshotToRepopulate(t *testing.T) {
	now := time.Unix(1700000600, 0).UTC()
	snapshot := []types.NetworkWaterfallEntry{
		{Name: "https://a.test/x", URL: "https://a.test/x", StartTime: 4, ResponseEnd: 8},
	}
	store := New(10)
	store.addAt(snapshot, "https://app.local", now)
	store.Clear()
	// A clear must not become a mute: the next snapshot is the same one, and it
	// has to come back rather than being suppressed as already seen.
	store.addAt(snapshot, "https://app.local", now.Add(time.Second))

	if got := len(store.Entries()); got != 1 {
		t.Fatalf("after clear the store held %d entries, want 1 repopulated", got)
	}
}

func TestStoreReloadOfTheSamePageIsNotSuppressed(t *testing.T) {
	now := time.Unix(1700000700, 0).UTC()
	store := New(10)
	// First load reaches startTime 900.
	store.addAt([]types.NetworkWaterfallEntry{
		{Name: "app.js", URL: "https://a.test/app.js", StartTime: 5, ResponseEnd: 9},
		{Name: "late.js", URL: "https://a.test/late.js", StartTime: 900, ResponseEnd: 950},
	}, "https://app.local/page", now)

	// Reload: same URL, timeOrigin restarts, so the clock appears to go
	// backwards. These are new requests despite colliding with the first load.
	store.addAt([]types.NetworkWaterfallEntry{
		{Name: "app.js", URL: "https://a.test/app.js", StartTime: 5, ResponseEnd: 9},
	}, "https://app.local/page", now.Add(time.Minute))

	if got := len(store.Entries()); got != 3 {
		t.Fatalf("stored %d entries, want 3 — a reload's requests are not duplicates", got)
	}
}

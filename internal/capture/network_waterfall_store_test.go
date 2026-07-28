package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

func TestNetworkWaterfallStore_AddTagsAndEvicts(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	store := newNetworkWaterfallStore(2)

	store.addAt([]types.NetworkWaterfallEntry{
		{URL: "https://a.test/a"},
		{URL: "https://a.test/b"},
		{URL: "https://a.test/c"},
	}, "https://app.local/dashboard", now)

	entries := store.Entries()
	if got := len(entries); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if entries[0].URL != "https://a.test/b" || entries[1].URL != "https://a.test/c" {
		t.Fatalf("eviction kept unexpected entries: %+v", entries)
	}
	for i, entry := range entries {
		if entry.PageURL != "https://app.local/dashboard" {
			t.Fatalf("entry[%d] page_url = %q, want %q", i, entry.PageURL, "https://app.local/dashboard")
		}
		if !entry.Timestamp.Equal(now) {
			t.Fatalf("entry[%d] timestamp = %v, want %v", i, entry.Timestamp, now)
		}
	}
}

func TestNetworkWaterfallStore_EntriesDetached(t *testing.T) {
	now := time.Unix(1700000100, 0).UTC()
	store := newNetworkWaterfallStore(2)
	store.addAt([]types.NetworkWaterfallEntry{{URL: "https://a.test/a"}}, "https://app.local", now)

	snap := store.Entries()
	if len(snap) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snap))
	}
	snap[0].URL = "mutated"

	if got := store.Entries()[0].URL; got != "https://a.test/a" {
		t.Fatalf("buffer mutated through snapshot; URL=%q", got)
	}
}

func TestNetworkWaterfallStore_Clear(t *testing.T) {
	now := time.Unix(1700000200, 0).UTC()
	store := newNetworkWaterfallStore(3)
	store.addAt([]types.NetworkWaterfallEntry{
		{URL: "https://a.test/a"},
		{URL: "https://a.test/b"},
	}, "https://app.local", now)

	removed := store.Clear()
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if got := len(store.Entries()); got != 0 {
		t.Fatalf("count after clear = %d, want 0", got)
	}
}

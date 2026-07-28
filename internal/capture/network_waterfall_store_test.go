package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

func TestNetworkWaterfallBuffer_AppendEntriesTagsAndEvicts(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	buf := NetworkWaterfallBuffer{
		entries:  make([]types.NetworkWaterfallEntry, 0, 2),
		capacity: 2,
	}

	buf.appendEntries([]types.NetworkWaterfallEntry{
		{URL: "https://a.test/a"},
		{URL: "https://a.test/b"},
		{URL: "https://a.test/c"},
	}, "https://app.local/dashboard", now)

	if got := buf.count(); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if buf.entries[0].URL != "https://a.test/b" || buf.entries[1].URL != "https://a.test/c" {
		t.Fatalf("eviction kept unexpected entries: %+v", buf.entries)
	}
	for i, entry := range buf.entries {
		if entry.PageURL != "https://app.local/dashboard" {
			t.Fatalf("entry[%d] page_url = %q, want %q", i, entry.PageURL, "https://app.local/dashboard")
		}
		if !entry.Timestamp.Equal(now) {
			t.Fatalf("entry[%d] timestamp = %v, want %v", i, entry.Timestamp, now)
		}
	}
}

func TestNetworkWaterfallBuffer_SnapshotDetached(t *testing.T) {
	now := time.Unix(1700000100, 0).UTC()
	buf := NetworkWaterfallBuffer{
		entries:  make([]types.NetworkWaterfallEntry, 0, 2),
		capacity: 2,
	}
	buf.appendEntries([]types.NetworkWaterfallEntry{{URL: "https://a.test/a"}}, "https://app.local", now)

	snap := buf.snapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snap))
	}
	snap[0].URL = "mutated"

	if got := buf.entries[0].URL; got != "https://a.test/a" {
		t.Fatalf("buffer mutated through snapshot; URL=%q", got)
	}
}

func TestNetworkWaterfallBuffer_Clear(t *testing.T) {
	now := time.Unix(1700000200, 0).UTC()
	buf := NetworkWaterfallBuffer{
		entries:  make([]types.NetworkWaterfallEntry, 0, 3),
		capacity: 3,
	}
	buf.appendEntries([]types.NetworkWaterfallEntry{
		{URL: "https://a.test/a"},
		{URL: "https://a.test/b"},
	}, "https://app.local", now)

	removed := buf.clear()
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if got := buf.count(); got != 0 {
		t.Fatalf("count after clear = %d, want 0", got)
	}
	if cap(buf.entries) != 3 {
		t.Fatalf("entries cap = %d, want 3", cap(buf.entries))
	}
}

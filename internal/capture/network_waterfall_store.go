// Purpose: Encapsulates network-waterfall ring-buffer operations behind focused store methods.
// Why: Keeps append/eviction/copy/clear behavior out of Capture methods during god-object decomposition.
// Docs: docs/features/feature/observe/index.md

package capture

import "time"

// appendEntries appends entries, annotates each one with page URL/timestamp, and enforces capacity.
func (b *NetworkWaterfallBuffer) appendEntries(entries []NetworkWaterfallEntry, pageURL string, now time.Time) {
	for i := range entries {
		entries[i].PageURL = pageURL
		entries[i].Timestamp = now
		b.entries = append(b.entries, entries[i])
	}

	if len(b.entries) <= b.capacity {
		return
	}
	kept := make([]NetworkWaterfallEntry, b.capacity)
	copy(kept, b.entries[len(b.entries)-b.capacity:])
	b.entries = kept
}

// count returns the number of buffered waterfall entries.
func (b *NetworkWaterfallBuffer) count() int {
	return len(b.entries)
}

// snapshot returns a detached copy of buffered entries.
func (b *NetworkWaterfallBuffer) snapshot() []NetworkWaterfallEntry {
	out := make([]NetworkWaterfallEntry, len(b.entries))
	copy(out, b.entries)
	return out
}

// clear removes all entries and returns the number removed.
func (b *NetworkWaterfallBuffer) clear() int {
	count := len(b.entries)
	b.entries = make([]NetworkWaterfallEntry, 0, b.capacity)
	return count
}

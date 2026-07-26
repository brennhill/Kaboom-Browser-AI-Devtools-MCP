// seed.go — Direct in-memory window seeding for tests in other packages.
// Why: cmd/browser-agent tests build windows the ingest path cannot express
// (backdated add-times, entries with no add-time at all) to exercise data-age
// and audit logic. Those tests live outside this package, so an unexported
// export_test.go hook cannot reach them.

package logstore

import "time"

// SeedEntries appends entries to the in-memory window and, when addedAt is
// non-nil, to the parallel add-time slice.
//
// It deliberately bypasses everything AddEntries does besides the append —
// no file queueing, no window trimming, no counters, no onEntries callback —
// so a test can construct an exact window state. Production ingest must go
// through AddEntries.
func (ls *Store) SeedEntries(entries []Entry, addedAt []time.Time) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.entries = append(ls.entries, entries...)
	if addedAt != nil {
		ls.logAddedAt = append(ls.logAddedAt, addedAt...)
	}
}

// SeedTotalAdded bumps the monotonic total-added counter by n.
// Test support only; see SeedEntries.
func (ls *Store) SeedTotalAdded(n int64) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.logTotalAdded += n
}

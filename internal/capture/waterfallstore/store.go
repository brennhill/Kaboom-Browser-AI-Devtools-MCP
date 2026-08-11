// store.go — Owns bounded browser resource-timing retention.
// Docs: docs/features/feature/backend-log-streaming/index.md

// Package waterfallstore owns resource-timing tagging, bounded retention,
// pressure accounting, detached snapshots, and synchronization.
package waterfallstore

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/pressure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/ringstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

const defaultCapacity = 1000

// identity distinguishes one browser request from another.
//
// A page's resource table is keyed by its own timeOrigin, so StartTime plus the
// resource name identifies a request uniquely within a single page load. Server
// receive time is deliberately excluded: the same request re-reported later is
// still the same request.
type identity struct {
	name        string
	url         string
	startTime   float64
	responseEnd float64
}

func identityOf(entry types.NetworkWaterfallEntry) identity {
	return identity{name: entry.Name, url: entry.URL, startTime: entry.StartTime, responseEnd: entry.ResponseEnd}
}

// seenCapacityFactor bounds the dedup index relative to retention, so a page
// with pathologically many resources degrades to duplicate-tolerant rather than
// growing memory without limit.
const seenCapacityFactor = 10

// Store retains browser resource timings independently of other telemetry.
type Store struct {
	mu      sync.RWMutex
	entries ringstore.Store[types.NetworkWaterfallEntry]
	dropped int64
	deduped int64

	// Dedup index for the page currently being observed. The browser reports a
	// snapshot, not a stream — observe(network_waterfall) re-queries the page
	// and receives the entire resource table on every single read — so ingest
	// has to be idempotent or reading the data corrupts it.
	seenPage     string
	seen         map[identity]struct{}
	seenMaxStart float64
}

// New creates a waterfall store with the requested bounded capacity.
func New(capacity int) *Store {
	return &Store{entries: ringstore.New[types.NetworkWaterfallEntry](capacity)}
}

// NewDefault creates the production waterfall store.
func NewDefault() *Store { return New(defaultCapacity) }

// Add tags and appends resource timings at server receive time.
func (s *Store) Add(entries []types.NetworkWaterfallEntry, pageURL string) {
	s.addAt(entries, pageURL, time.Now())
}

func (s *Store) addAt(entries []types.NetworkWaterfallEntry, pageURL string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resetDedupIfNewPageLoad(pageURL, entries)

	for i := range entries {
		entry := cloneEntry(entries[i])
		entry.PageURL = pageURL

		key := identityOf(entry)
		if _, duplicate := s.seen[key]; duplicate {
			s.deduped++
			continue
		}
		if len(s.seen) < s.entries.Capacity()*seenCapacityFactor {
			s.seen[key] = struct{}{}
		}
		if entry.StartTime > s.seenMaxStart {
			s.seenMaxStart = entry.StartTime
		}

		entry.Timestamp = now
		if _, overwritten := s.entries.Push(entry); overwritten {
			s.dropped++
		}
	}
}

// resetDedupIfNewPageLoad drops the dedup index when the incoming snapshot
// belongs to a different page load than the one it was built from.
//
// Two signals mean a new load. A different page URL is the obvious one. The
// subtler one is a reload of the same URL: timeOrigin restarts, so StartTime
// restarts near zero. Within a single load the clock only moves forward, so a
// snapshot whose newest entry predates what we have already recorded can only
// mean the page reloaded — and without this the reloaded page's early requests
// would collide with the previous load's and be discarded as duplicates.
func (s *Store) resetDedupIfNewPageLoad(pageURL string, entries []types.NetworkWaterfallEntry) {
	maxStart := 0.0
	for i := range entries {
		if entries[i].StartTime > maxStart {
			maxStart = entries[i].StartTime
		}
	}

	reloaded := len(entries) > 0 && maxStart < s.seenMaxStart
	if s.seen == nil || pageURL != s.seenPage || reloaded {
		s.seen = make(map[identity]struct{}, len(entries))
		s.seenPage = pageURL
		s.seenMaxStart = 0
	}
}

// Pressure returns bounded waterfall retention metrics.
func (s *Store) Pressure() pressure.Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := pressure.Stats{
		Size:     s.entries.Len(),
		Capacity: s.entries.Capacity(),
		Dropped:  s.dropped,
	}
	if s.entries.Len() == 0 {
		return stats
	}
	stats.OldestAge = time.Since(s.entries.At(0).Timestamp)
	if stats.OldestAge < 0 {
		stats.OldestAge = 0
	}
	return stats
}

// Entries returns a detached snapshot of resource timings.
func (s *Store) Entries() []types.NetworkWaterfallEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := s.entries.Snapshot()
	for i := range entries {
		entries[i] = cloneEntry(entries[i])
	}
	return entries
}

func cloneEntry(entry types.NetworkWaterfallEntry) types.NetworkWaterfallEntry {
	entry.ServerTiming = append([]types.WireServerTiming(nil), entry.ServerTiming...)
	entry.InitiatorStack = append([]string(nil), entry.InitiatorStack...)
	return entry
}

// Clear removes all resource timings and returns the removed count.
func (s *Store) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := s.entries.Len()
	s.entries.Clear()
	// The dedup index has to go with the entries. Keeping it would suppress the
	// very next snapshot as "already seen" and leave the buffer permanently
	// empty — a clear must not be a mute.
	s.seen = nil
	s.seenPage = ""
	s.seenMaxStart = 0
	return count
}

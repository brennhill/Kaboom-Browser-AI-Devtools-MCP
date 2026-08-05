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

// Store retains browser resource timings independently of other telemetry.
type Store struct {
	mu      sync.RWMutex
	entries ringstore.Store[types.NetworkWaterfallEntry]
	dropped int64
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
	for i := range entries {
		entry := cloneEntry(entries[i])
		entry.PageURL = pageURL
		entry.Timestamp = now
		if _, overwritten := s.entries.Push(entry); overwritten {
			s.dropped++
		}
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
	return count
}

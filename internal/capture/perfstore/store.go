// store.go — Owns bounded performance snapshots, samples, and action baselines.
// Docs: docs/features/feature/performance-audit/index.md
// Docs: docs/features/feature/operational-observability/index.md

package perfstore

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/pressure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

const (
	maxSnapshots = 100
	maxSamples   = 100
	maxBefore    = 50
)

// Pressure describes retained page snapshots and active pre-action baselines.
type Pressure struct {
	Snapshots       pressure.Stats `json:"snapshots"`
	Samples         pressure.Stats `json:"samples"`
	BeforeSnapshots pressure.Stats `json:"before_snapshots"`
}

// Store owns independently synchronized, bounded performance retention.
type Store struct {
	mu              sync.RWMutex
	now             func() time.Time
	snapshots       map[string]performance.PerformanceSnapshot
	snapshotOrder   []string
	samples         []performance.PerformanceSnapshot
	sampleAdded     []time.Time
	beforeSnapshots map[string]performance.PerformanceSnapshot
	snapshotAdded   map[string]time.Time
	beforeOrder     []string
	beforeAdded     map[string]time.Time
	snapshotDropped int64
	sampleDropped   int64
	beforeDropped   int64
}

// New constructs an empty production performance store.
func New() *Store { return newStore(time.Now) }

func newStore(now func() time.Time) *Store {
	return &Store{
		now:             now,
		snapshots:       make(map[string]performance.PerformanceSnapshot),
		snapshotOrder:   make([]string, 0, maxSnapshots),
		samples:         make([]performance.PerformanceSnapshot, 0, maxSamples),
		sampleAdded:     make([]time.Time, 0, maxSamples),
		beforeSnapshots: make(map[string]performance.PerformanceSnapshot),
		snapshotAdded:   make(map[string]time.Time),
		beforeAdded:     make(map[string]time.Time),
	}
}

// Add stores URL-keyed snapshots and chronological repeated-run samples.
func (s *Store) Add(snapshots []performance.PerformanceSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for _, snapshot := range snapshots {
		if snapshot.URL == "" {
			continue
		}
		if _, exists := s.snapshots[snapshot.URL]; !exists {
			s.snapshotOrder = append(s.snapshotOrder, snapshot.URL)
			s.snapshotAdded[snapshot.URL] = now
		}
		s.snapshots[snapshot.URL] = performance.CloneSnapshot(snapshot)
		s.samples = append(s.samples, performance.CloneSnapshot(snapshot))
		s.sampleAdded = append(s.sampleAdded, now)
	}
	s.enforceCapacityLocked()
}

func (s *Store) enforceCapacityLocked() {
	if overflow := len(s.samples) - maxSamples; overflow > 0 {
		s.sampleDropped += int64(overflow)
		s.samples = append([]performance.PerformanceSnapshot(nil), s.samples[overflow:]...)
		s.sampleAdded = append([]time.Time(nil), s.sampleAdded[overflow:]...)
	}
	if overflow := len(s.snapshotOrder) - maxSnapshots; overflow > 0 {
		s.snapshotDropped += int64(overflow)
		for _, key := range s.snapshotOrder[:overflow] {
			delete(s.snapshots, key)
			delete(s.snapshotAdded, key)
		}
		s.snapshotOrder = append([]string(nil), s.snapshotOrder[overflow:]...)
	}
}

// Entries returns detached URL snapshots in first-seen order.
func (s *Store) Entries() []performance.PerformanceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]performance.PerformanceSnapshot, 0, len(s.snapshotOrder))
	for _, key := range s.snapshotOrder {
		entries = append(entries, performance.CloneSnapshot(s.snapshots[key]))
	}
	return entries
}

// Samples returns detached chronological repeated-run samples.
func (s *Store) Samples() []performance.PerformanceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]performance.PerformanceSnapshot, len(s.samples))
	for index, snapshot := range s.samples {
		entries[index] = performance.CloneSnapshot(snapshot)
	}
	return entries
}

// ByURL returns a detached snapshot stored for one URL.
func (s *Store) ByURL(url string) (performance.PerformanceSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, exists := s.snapshots[url]
	return performance.CloneSnapshot(snapshot), exists
}

// StoreBefore stores a bounded pre-action snapshot for later diffing.
func (s *Store) StoreBefore(correlationID string, snapshot performance.PerformanceSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.beforeSnapshots[correlationID]; !exists {
		s.beforeOrder = append(s.beforeOrder, correlationID)
		s.beforeAdded[correlationID] = s.now()
	}
	s.beforeSnapshots[correlationID] = performance.CloneSnapshot(snapshot)
	if len(s.beforeOrder) <= maxBefore {
		return
	}
	oldest := s.beforeOrder[0]
	s.beforeOrder = append([]string(nil), s.beforeOrder[1:]...)
	delete(s.beforeSnapshots, oldest)
	delete(s.beforeAdded, oldest)
	s.beforeDropped++
}

// TakeBefore consumes and returns a detached pre-action snapshot.
func (s *Store) TakeBefore(correlationID string) (performance.PerformanceSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, exists := s.beforeSnapshots[correlationID]
	if !exists {
		return performance.PerformanceSnapshot{}, false
	}
	delete(s.beforeSnapshots, correlationID)
	delete(s.beforeAdded, correlationID)
	for index, key := range s.beforeOrder {
		if key == correlationID {
			s.beforeOrder = append(s.beforeOrder[:index], s.beforeOrder[index+1:]...)
			break
		}
	}
	return performance.CloneSnapshot(snapshot), true
}

// Pressure returns bounded retention metrics using the store clock.
func (s *Store) Pressure() Pressure {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	return Pressure{
		Snapshots:       pressureForKeys(s.snapshotOrder, s.snapshotAdded, s.snapshotDropped, maxSnapshots, now),
		Samples:         pressureForTimes(s.sampleAdded, s.sampleDropped, maxSamples, now),
		BeforeSnapshots: pressureForKeys(s.beforeOrder, s.beforeAdded, s.beforeDropped, maxBefore, now),
	}
}

func pressureForTimes(added []time.Time, dropped int64, capacity int, now time.Time) pressure.Stats {
	stats := pressure.Stats{Size: len(added), Capacity: capacity, Dropped: dropped}
	if len(added) > 0 {
		stats.OldestAge = nonnegativeAge(now, added[0])
	}
	return stats
}

func pressureForKeys(order []string, added map[string]time.Time, dropped int64, capacity int, now time.Time) pressure.Stats {
	stats := pressure.Stats{Size: len(order), Capacity: capacity, Dropped: dropped}
	if len(order) > 0 {
		stats.OldestAge = nonnegativeAge(now, added[order[0]])
	}
	return stats
}

func nonnegativeAge(now, added time.Time) time.Duration {
	age := now.Sub(added)
	if age < 0 {
		return 0
	}
	return age
}

// Clear removes retained snapshots while preserving cumulative drop evidence.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = make(map[string]performance.PerformanceSnapshot)
	s.snapshotOrder = make([]string, 0, maxSnapshots)
	s.samples = make([]performance.PerformanceSnapshot, 0, maxSamples)
	s.sampleAdded = make([]time.Time, 0, maxSamples)
	s.beforeSnapshots = make(map[string]performance.PerformanceSnapshot)
	s.snapshotAdded = make(map[string]time.Time)
	s.beforeOrder = nil
	s.beforeAdded = make(map[string]time.Time)
}

// store.go — Owns coherent WebSocket event retention and connection state.
// Docs: docs/features/feature/observe/index.md

package wsconn

import (
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/pressure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/ringstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

const eventOverhead = int64(200)

type eventEntry struct {
	event   types.WebSocketEvent
	addedAt time.Time
}

// Snapshot is detached WebSocket evidence with its ingestion timestamps.
type Snapshot struct {
	Events     []types.WebSocketEvent
	Timestamps []time.Time
}

// State is allocation-free WebSocket retention and connection metadata.
type State struct {
	Count           int
	Capacity        int
	TotalAdded      int64
	MemoryBytes     int64
	ConnectionCount int
	Pressure        pressure.Stats
}

// ClearCounts reports both state families removed by one coherent reset.
type ClearCounts struct {
	Events      int
	Connections int
}

// Store owns WebSocket events and the connection state derived from them.
type Store struct {
	mu          sync.RWMutex
	events      ringstore.Store[eventEntry]
	connections Tracker
	totalAdded  int64
	memoryBytes int64
	dropped     int64
	memoryLimit int64
}

// NewStore creates a WebSocket owner with explicit entry and memory limits.
func NewStore(capacity int, memoryLimit int64) *Store {
	if memoryLimit < 0 {
		memoryLimit = 0
	}
	return &Store{
		events:      ringstore.New[eventEntry](capacity),
		connections: NewTracker(),
		memoryLimit: memoryLimit,
	}
}

// Add takes ownership of events and updates derived connection state atomically.
func (s *Store) Add(events []types.WebSocketEvent, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalAdded += int64(len(events))
	for i := range events {
		event := cloneEvent(events[i])
		s.connections.TrackEvent(event)
		evicted, overwritten := s.events.Push(eventEntry{event: event, addedAt: now})
		if overwritten {
			s.dropped++
			s.memoryBytes -= eventMemory(evicted.event)
		}
		s.memoryBytes += eventMemory(event)
	}
	s.evictForMemory()
}

func (s *Store) evictForMemory() {
	excess := s.memoryBytes - s.memoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < s.events.Len() && excess > 0 {
		entryBytes := eventMemory(s.events.At(drop).event)
		excess -= entryBytes
		s.memoryBytes -= entryBytes
		drop++
	}
	s.events.DropOldest(drop)
	s.dropped += int64(drop)
}

// Events returns newest-first detached events matching the requested filter.
func (s *Store) Events(filter types.WebSocketEventFilter) []types.WebSocketEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > s.events.Len() {
		limit = s.events.Len()
	}
	filtered := make([]types.WebSocketEvent, 0, limit)
	for i := s.events.Len() - 1; i >= 0; i-- {
		entry := s.events.At(i)
		if !matchesFilter(entry.event, filter) {
			continue
		}
		filtered = append(filtered, cloneEvent(entry.event))
		if len(filtered) >= limit {
			break
		}
	}
	return filtered
}

// Snapshot returns all retained events and timestamps in FIFO order.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := Snapshot{
		Events:     make([]types.WebSocketEvent, s.events.Len()),
		Timestamps: make([]time.Time, s.events.Len()),
	}
	for i := range snapshot.Events {
		entry := s.events.At(i)
		snapshot.Events[i] = cloneEvent(entry.event)
		snapshot.Timestamps[i] = entry.addedAt
	}
	return snapshot
}

// Stats returns allocation-free metadata for the current instant.
func (s *Store) Stats() State { return s.statsAt(time.Now()) }

func (s *Store) statsAt(now time.Time) State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := State{
		Count:           s.events.Len(),
		Capacity:        s.events.Capacity(),
		TotalAdded:      s.totalAdded,
		MemoryBytes:     s.memoryBytes,
		ConnectionCount: s.connections.Count(),
		Pressure: pressure.Stats{
			Size:     s.events.Len(),
			Capacity: s.events.Capacity(),
			Dropped:  s.dropped,
		},
	}
	if s.events.Len() > 0 {
		state.Pressure.OldestAge = now.Sub(s.events.At(0).addedAt)
		if state.Pressure.OldestAge < 0 {
			state.Pressure.OldestAge = 0
		}
	}
	return state
}

// Status returns a detached projection of derived connection state.
func (s *Store) Status(filter types.WebSocketStatusFilter) types.WebSocketStatusResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connections.Status(filter)
}

// Clear atomically resets retained events and derived connection state.
func (s *Store) Clear() ClearCounts {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := ClearCounts{Events: s.events.Len(), Connections: s.connections.Count()}
	s.events.Clear()
	s.connections.Clear()
	s.totalAdded = 0
	s.memoryBytes = 0
	return counts
}

func matchesFilter(event types.WebSocketEvent, filter types.WebSocketEventFilter) bool {
	if filter.ConnectionID != "" && event.ID != filter.ConnectionID {
		return false
	}
	if filter.URLFilter != "" && !strings.Contains(event.URL, filter.URLFilter) {
		return false
	}
	if filter.Direction != "" && event.Direction != filter.Direction {
		return false
	}
	if filter.TestID != "" && !containsTestID(event.TestIDs, filter.TestID) {
		return false
	}
	return true
}

func containsTestID(testIDs []string, target string) bool {
	for _, testID := range testIDs {
		if testID == target {
			return true
		}
	}
	return false
}

func cloneEvent(event types.WebSocketEvent) types.WebSocketEvent {
	if event.Sampled != nil {
		sampled := *event.Sampled
		event.Sampled = &sampled
	}
	event.TestIDs = append([]string(nil), event.TestIDs...)
	return event
}

func eventMemory(event types.WebSocketEvent) int64 {
	return int64(len(event.Data)) + eventOverhead
}

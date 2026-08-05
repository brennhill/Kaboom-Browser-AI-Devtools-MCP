// store.go — Owns bounded captured HTTP request/response bodies.
// Docs: docs/features/feature/backend-log-streaming/index.md

// Package bodystore owns network-body synchronization, retention, memory
// pressure, counters, cloning, snapshots, and clearing.
package bodystore

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/pressure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/ringstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

const (
	defaultCapacity    = 100
	defaultMemoryLimit = int64(8 * 1024 * 1024)
	entryOverhead      = int64(300)
)

type entry struct {
	body    types.NetworkBody
	addedAt time.Time
}

// Snapshot is a detached, internally consistent network-body view.
type Snapshot struct {
	Bodies          []types.NetworkBody
	Timestamps      []time.Time
	TotalAdded      int64
	ErrorTotalAdded int64
	MemoryBytes     int64
	Pressure        pressure.Stats
}

// State is allocation-free body-store metadata for hot diagnostic paths.
type State struct {
	Count           int
	Capacity        int
	TotalAdded      int64
	ErrorTotalAdded int64
	MemoryBytes     int64
	Pressure        pressure.Stats
}

// Store owns captured network-body state and synchronization.
type Store struct {
	mu              sync.RWMutex
	entries         ringstore.Store[entry]
	totalAdded      int64
	errorTotalAdded int64
	memoryBytes     int64
	dropped         int64
	memoryLimit     int64
}

// New creates a store with explicit entry and memory limits.
func New(capacity int, memoryLimit int64) *Store {
	if memoryLimit < 0 {
		memoryLimit = 0
	}
	return &Store{
		entries:     ringstore.New[entry](capacity),
		memoryLimit: memoryLimit,
	}
}

// NewDefault creates the production network-body store.
func NewDefault() *Store { return New(defaultCapacity, defaultMemoryLimit) }

// Add takes ownership of bodies at the supplied ingestion time.
func (s *Store) Add(bodies []types.NetworkBody, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalAdded += int64(len(bodies))
	for i := range bodies {
		if bodies[i].Status >= 400 {
			s.errorTotalAdded++
		}
		body := cloneBody(bodies[i])
		evicted, overwritten := s.entries.Push(entry{body: body, addedAt: now})
		if overwritten {
			s.dropped++
			s.memoryBytes -= bodyMemory(evicted.body)
		}
		s.memoryBytes += bodyMemory(body)
	}
	s.evictForMemory()
}

func (s *Store) evictForMemory() {
	excess := s.memoryBytes - s.memoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < s.entries.Len() && excess > 0 {
		entryBytes := bodyMemory(s.entries.At(drop).body)
		excess -= entryBytes
		s.memoryBytes -= entryBytes
		drop++
	}
	s.entries.DropOldest(drop)
	s.dropped += int64(drop)
}

// Snapshot returns one detached view under the store lock.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := s.stateLocked(time.Now())
	result := Snapshot{
		Bodies:          make([]types.NetworkBody, s.entries.Len()),
		Timestamps:      make([]time.Time, s.entries.Len()),
		TotalAdded:      state.TotalAdded,
		ErrorTotalAdded: state.ErrorTotalAdded,
		MemoryBytes:     state.MemoryBytes,
		Pressure:        state.Pressure,
	}
	for i := range result.Bodies {
		retained := s.entries.At(i)
		result.Bodies[i] = cloneBody(retained.body)
		result.Timestamps[i] = retained.addedAt
	}
	return result
}

// Stats returns allocation-free counters and pressure metadata.
func (s *Store) Stats() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stateLocked(time.Now())
}

func (s *Store) stateLocked(now time.Time) State {
	state := State{
		Count:           s.entries.Len(),
		Capacity:        s.entries.Capacity(),
		TotalAdded:      s.totalAdded,
		ErrorTotalAdded: s.errorTotalAdded,
		MemoryBytes:     s.memoryBytes,
		Pressure: pressure.Stats{
			Size:     s.entries.Len(),
			Capacity: s.entries.Capacity(),
			Dropped:  s.dropped,
		},
	}
	if s.entries.Len() > 0 {
		state.Pressure.OldestAge = now.Sub(s.entries.At(0).addedAt)
		if state.Pressure.OldestAge < 0 {
			state.Pressure.OldestAge = 0
		}
	}
	return state
}

// Clear removes retained bodies and resets current-session totals.
func (s *Store) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := s.entries.Len()
	s.entries.Clear()
	s.totalAdded = 0
	s.errorTotalAdded = 0
	s.memoryBytes = 0
	return count
}

func cloneBody(body types.NetworkBody) types.NetworkBody {
	if body.ResponseHeaders != nil {
		headers := make(map[string]string, len(body.ResponseHeaders))
		for key, value := range body.ResponseHeaders {
			headers[key] = value
		}
		body.ResponseHeaders = headers
	}
	body.TestIDs = append([]string(nil), body.TestIDs...)
	return body
}

func bodyMemory(body types.NetworkBody) int64 {
	return int64(len(body.RequestBody)+len(body.ResponseBody)) + entryOverhead
}

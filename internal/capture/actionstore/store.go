// store.go — Owns bounded enhanced browser actions.
// Docs: docs/features/feature/backend-log-streaming/index.md

// Package actionstore owns enhanced-action synchronization, deep cloning,
// bounded retention, navigation signaling, pressure, snapshots, and clearing.
package actionstore

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/pressure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/ringstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

const defaultCapacity = 1000

type entry struct {
	action  types.EnhancedAction
	addedAt time.Time
}

// Snapshot is detached enhanced-action evidence with its ingestion timestamps.
type Snapshot struct {
	Actions    []types.EnhancedAction
	Timestamps []time.Time
}

// State is allocation-free enhanced-action metadata.
type State struct {
	Count      int
	Capacity   int
	TotalAdded int64
	Pressure   pressure.Stats
}

// Store owns enhanced-action state and synchronization.
type Store struct {
	mu         sync.RWMutex
	entries    ringstore.Store[entry]
	totalAdded int64
	dropped    int64
}

// New creates an action store with the requested bounded capacity.
func New(capacity int) *Store {
	return &Store{entries: ringstore.New[entry](capacity)}
}

// NewDefault creates the production action store.
func NewDefault() *Store { return New(defaultCapacity) }

// Add takes ownership of actions and reports whether input included navigation.
func (s *Store) Add(actions []types.EnhancedAction, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalAdded += int64(len(actions))
	navigated := false
	for i := range actions {
		if _, overwritten := s.entries.Push(entry{action: cloneAction(actions[i]), addedAt: now}); overwritten {
			s.dropped++
		}
		if actions[i].Type == "navigation" {
			navigated = true
		}
	}
	return navigated
}

// Snapshot returns detached actions and timestamps under one read lock.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := Snapshot{
		Actions:    make([]types.EnhancedAction, s.entries.Len()),
		Timestamps: make([]time.Time, s.entries.Len()),
	}
	for i := range snapshot.Actions {
		retained := s.entries.At(i)
		snapshot.Actions[i] = cloneAction(retained.action)
		snapshot.Timestamps[i] = retained.addedAt
	}
	return snapshot
}

// Stats returns allocation-free counters and pressure metadata.
func (s *Store) Stats() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := State{
		Count:      s.entries.Len(),
		Capacity:   s.entries.Capacity(),
		TotalAdded: s.totalAdded,
		Pressure: pressure.Stats{
			Size:     s.entries.Len(),
			Capacity: s.entries.Capacity(),
			Dropped:  s.dropped,
		},
	}
	if s.entries.Len() > 0 {
		state.Pressure.OldestAge = time.Since(s.entries.At(0).addedAt)
		if state.Pressure.OldestAge < 0 {
			state.Pressure.OldestAge = 0
		}
	}
	return state
}

// Clear removes retained actions and resets the current-session total.
func (s *Store) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := s.entries.Len()
	s.entries.Clear()
	s.totalAdded = 0
	return count
}

func cloneAction(action types.EnhancedAction) types.EnhancedAction {
	if action.Selectors != nil {
		selectors := make(map[string]any, len(action.Selectors))
		for key, value := range action.Selectors {
			selectors[key] = cloneSelector(value)
		}
		action.Selectors = selectors
	}
	action.TestIDs = append([]string(nil), action.TestIDs...)
	return action
}

func cloneSelector(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clone := make(map[string]any, len(typed))
		for key, child := range typed {
			clone[key] = cloneSelector(child)
		}
		return clone
	case []any:
		clone := make([]any, len(typed))
		for index, child := range typed {
			clone[index] = cloneSelector(child)
		}
		return clone
	case map[string]string:
		clone := make(map[string]string, len(typed))
		for key, child := range typed {
			clone[key] = child
		}
		return clone
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

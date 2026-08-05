// Purpose: Owns named snapshot state, capture contracts, CRUD, and bounded eviction.
// Docs: docs/features/feature/request-session-correlation/index.md

// snapshot-manager.go — SessionManager struct and snapshot management.
// NewSessionManager, Capture, captureCurrentState, List, Delete functions.
// Package session manages named snapshots of browser state and diff_sessions.
package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

const (
	maxSnapshotNameLen    = 50
	maxConsolePerSnapshot = 50
	maxNetworkPerSnapshot = 100
	reservedSnapshotName  = "current"
)

// CaptureStateReader supplies the live state needed to create a snapshot.
type CaptureStateReader interface {
	GetConsoleErrors() []types.SnapshotError
	GetConsoleWarnings() []types.SnapshotError
	GetNetworkRequests() []types.SnapshotNetworkRequest
	GetWSConnections() []types.SnapshotWSConnection
	GetPerformance() *performance.PerformanceSnapshot
	GetCurrentPageURL() string
}

// SnapshotListEntry is the detached list representation of a snapshot.
type SnapshotListEntry struct {
	Name       string    `json:"name"`
	CapturedAt time.Time `json:"captured_at"`
	PageURL    string    `json:"page_url"`
	ErrorCount int       `json:"error_count"`
}

// SessionManager manages named session snapshots.
type SessionManager struct {
	mu      sync.RWMutex
	snaps   map[string]*types.NamedSnapshot
	order   []string
	maxSize int
	reader  CaptureStateReader
}

// NewSessionManager creates a new SessionManager with given max snapshot count.
func NewSessionManager(maxSnapshots int, reader CaptureStateReader) *SessionManager {
	if maxSnapshots <= 0 {
		maxSnapshots = 10
	}
	return &SessionManager{
		snaps:   make(map[string]*types.NamedSnapshot),
		order:   make([]string, 0),
		maxSize: maxSnapshots,
		reader:  reader,
	}
}

func (sm *SessionManager) validateName(name string) error {
	if name == "" {
		return fmt.Errorf("snapshot name cannot be empty")
	}
	if name == reservedSnapshotName {
		return fmt.Errorf("snapshot name %q is reserved", reservedSnapshotName)
	}
	if len(name) > maxSnapshotNameLen {
		return fmt.Errorf("snapshot name exceeds %d characters", maxSnapshotNameLen)
	}
	return nil
}

func (sm *SessionManager) removeFromOrder(name string) {
	for i, existing := range sm.order {
		if existing != name {
			continue
		}
		order := make([]string, len(sm.order)-1)
		copy(order, sm.order[:i])
		copy(order[i:], sm.order[i+1:])
		sm.order = order
		return
	}
}

// Capture stores the current state as a named snapshot.
func (sm *SessionManager) Capture(name, urlFilter string) (*types.NamedSnapshot, error) {
	if err := sm.validateName(name); err != nil {
		return nil, err
	}

	snapshot := sm.captureCurrentState(name, urlFilter)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// If name already exists, overwrite (remove from order, re-add at end)
	if _, exists := sm.snaps[name]; exists {
		sm.removeFromOrder(name)
	} else {
		// Evict oldest if at capacity
		for len(sm.order) >= sm.maxSize {
			oldest := sm.order[0]
			delete(sm.snaps, oldest)
			newOrder := make([]string, len(sm.order)-1)
			copy(newOrder, sm.order[1:])
			sm.order = newOrder
		}
	}

	sm.snaps[name] = snapshot
	sm.order = append(sm.order, name)

	return snapshot, nil
}

// captureCurrentState reads the current state from reader and builds a snapshot.
func (sm *SessionManager) captureCurrentState(name, urlFilter string) *types.NamedSnapshot {
	errors := sm.reader.GetConsoleErrors()
	warnings := sm.reader.GetConsoleWarnings()
	network := sm.reader.GetNetworkRequests()
	ws := sm.reader.GetWSConnections()
	perf := sm.reader.GetPerformance()
	var perfSamples []performance.PerformanceSnapshot
	if sampleReader, ok := sm.reader.(interface {
		GetPerformanceSamples() []performance.PerformanceSnapshot
	}); ok {
		for _, sample := range sampleReader.GetPerformanceSamples() {
			if perf == nil || perf.URL == "" || sample.URL == perf.URL {
				perfSamples = append(perfSamples, sample)
			}
		}
	}
	if len(perfSamples) == 0 && perf != nil {
		perfSamples = append(perfSamples, *perf)
	}
	pageURL := sm.reader.GetCurrentPageURL()

	// Apply URL filter to network requests
	if urlFilter != "" {
		filtered := make([]types.SnapshotNetworkRequest, 0, len(network))
		for _, req := range network {
			if strings.Contains(req.URL, urlFilter) {
				filtered = append(filtered, req)
			}
		}
		network = filtered
	}

	// Apply limits
	if len(errors) > maxConsolePerSnapshot {
		errors = errors[:maxConsolePerSnapshot]
	}
	if len(warnings) > maxConsolePerSnapshot {
		warnings = warnings[:maxConsolePerSnapshot]
	}
	if len(network) > maxNetworkPerSnapshot {
		network = network[:maxNetworkPerSnapshot]
	}

	// Deep copy performance snapshot if present
	var perfCopy *performance.PerformanceSnapshot
	if perf != nil {
		p := performance.CloneSnapshot(*perf)
		perfCopy = &p
	}

	return &types.NamedSnapshot{
		Name:                 name,
		CapturedAt:           time.Now(),
		URLFilter:            urlFilter,
		PageURL:              pageURL,
		ConsoleErrors:        errors,
		ConsoleWarnings:      warnings,
		NetworkRequests:      network,
		WebSocketConnections: ws,
		Performance:          perfCopy,
		PerformanceSamples:   clonePerformanceSamples(perfSamples),
	}
}

func clonePerformanceSamples(samples []performance.PerformanceSnapshot) []performance.PerformanceSnapshot {
	clones := make([]performance.PerformanceSnapshot, len(samples))
	for index, sample := range samples {
		clones[index] = performance.CloneSnapshot(sample)
	}
	return clones
}

// List returns all stored snapshots in insertion order.
func (sm *SessionManager) List() []SnapshotListEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entries := make([]SnapshotListEntry, 0, len(sm.order))
	for _, name := range sm.order {
		snap := sm.snaps[name]
		entries = append(entries, SnapshotListEntry{
			Name:       snap.Name,
			CapturedAt: snap.CapturedAt,
			PageURL:    snap.PageURL,
			ErrorCount: len(snap.ConsoleErrors),
		})
	}
	return entries
}

// Delete removes a named snapshot.
func (sm *SessionManager) Delete(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.snaps[name]; !exists {
		return fmt.Errorf("snapshot %q not found", name)
	}

	delete(sm.snaps, name)
	sm.removeFromOrder(name)
	return nil
}

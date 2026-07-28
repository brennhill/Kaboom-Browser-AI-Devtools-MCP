// Purpose: Implements in-memory log entry mutation and read access routines.
// Why: Keeps entry lifecycle logic separate from file I/O and HTTP response concerns.
// Docs: docs/features/feature/backend-log-streaming/index.md

package server

import (
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// addEntries adds new entries and rotates if needed.
func (s *Server) addEntries(newEntries []types.LogEntry) int {
	type addEntriesPlan struct {
		entriesToSave []types.LogEntry
		appendOnly    []types.LogEntry
		callback      func([]types.LogEntry)
	}
	plan := func() addEntriesPlan {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.logTotalAdded += int64(len(newEntries))
		now := time.Now()
		for range newEntries {
			s.logAddedAt = append(s.logAddedAt, now)
		}
		s.entries = append(s.entries, newEntries...)

		// Rotate if needed — copy to new slice to allow GC of evicted entries.
		rotated := len(s.entries) > s.maxEntries
		if rotated {
			kept := make([]types.LogEntry, s.maxEntries)
			copy(kept, s.entries[len(s.entries)-s.maxEntries:])
			s.entries = kept
			keptAt := make([]time.Time, s.maxEntries)
			copy(keptAt, s.logAddedAt[len(s.logAddedAt)-s.maxEntries:])
			s.logAddedAt = keptAt
		}

		result := addEntriesPlan{
			callback: s.onEntries,
		}
		// Snapshot data for file I/O outside the lock.
		if rotated {
			result.entriesToSave = make([]types.LogEntry, len(s.entries))
			copy(result.entriesToSave, s.entries)
		} else {
			result.appendOnly = make([]types.LogEntry, len(newEntries))
			copy(result.appendOnly, newEntries)
		}
		return result
	}()

	// File I/O outside lock.
	if len(plan.entriesToSave) > 0 {
		if err := s.saveEntriesCopy(plan.entriesToSave); err != nil {
			fmt.Fprintf(os.Stderr, "[Kaboom] Error saving entries: %v\n", err)
		}
	} else {
		if err := s.appendToFile(plan.appendOnly); err != nil {
			fmt.Fprintf(os.Stderr, "[Kaboom] Error saving entries: %v\n", err)
		}
	}

	// Notify listeners outside the lock (e.g., cluster manager).
	if plan.callback != nil {
		plan.callback(newEntries)
	}

	return len(newEntries)
}

// clearEntries removes all entries.
func (s *Server) clearEntries() {
	logFile := func() string {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.entries = nil
		s.logAddedAt = nil
		return s.logFile
	}()
	// Write empty file outside lock.
	// #nosec G306 -- log files are owner-only (0600) for privacy
	if logFile != "" {
		if err := os.WriteFile(logFile, []byte{}, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "[Kaboom] Error clearing log file: %v\n", err)
		}
	}
}

// getEntryCount returns current entry count.
func (s *Server) getEntryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// getEntries returns a copy of all entries.
func (s *Server) getEntries() []types.LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]types.LogEntry, len(s.entries))
	copy(result, s.entries)
	return result
}

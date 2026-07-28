// Purpose: Provides thread-safe log snapshot accessor APIs over server log buffers.
// Why: Exposes read-only server log state for analysis without leaking synchronization details.
// Docs: docs/features/feature/backend-log-streaming/index.md

package server

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// LogSnapshot represents a snapshot of log state at a moment in time.
// Safe to pass across package boundaries without exposing internal synchronization.
type LogSnapshot struct {
	Entries       []types.LogEntry // Copy of log entries
	TotalAdded    int64            // Monotonic total added ever
	EntryCount    int              // Current entry count
	LastAddedAt   time.Time        // When the last entry was added (zero if no entries)
	OldestAddedAt time.Time        // When the oldest entry was added (zero if no entries)
}

// GetLogSnapshot returns a thread-safe snapshot of the log state.
// Does not block on external callbacks (onEntries).
func (s *Server) GetLogSnapshot() LogSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := LogSnapshot{
		TotalAdded: s.logTotalAdded,
		EntryCount: len(s.entries),
	}

	// Copy entries
	if len(s.entries) > 0 {
		snapshot.Entries = make([]types.LogEntry, len(s.entries))
		copy(snapshot.Entries, s.entries)

		// Timestamp bounds
		if len(s.logAddedAt) > 0 {
			snapshot.OldestAddedAt = s.logAddedAt[0]
			snapshot.LastAddedAt = s.logAddedAt[len(s.logAddedAt)-1]
		}
	}

	return snapshot
}

// GetLogCount returns the current number of entries (thread-safe).
func (s *Server) GetLogCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// GetLogTotalAdded returns the monotonic counter of total entries ever added (thread-safe).
func (s *Server) GetLogTotalAdded() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logTotalAdded
}

// GetLogEntries returns a copy of the current log entries (thread-safe).
func (s *Server) GetLogEntries() []types.LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		return nil
	}

	copy := make([]types.LogEntry, len(s.entries))
	for i, entry := range s.entries {
		// Create a new map copy for each entry
		entryCopy := make(types.LogEntry)
		for k, v := range entry {
			entryCopy[k] = v
		}
		copy[i] = entryCopy
	}
	return copy
}

// GetOldestLogTime returns the timestamp of the oldest entry, or zero if no entries (thread-safe).
func (s *Server) GetOldestLogTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.logAddedAt) == 0 {
		return time.Time{}
	}
	return s.logAddedAt[0]
}

// GetNewestLogTime returns the timestamp of the newest entry, or zero if no entries (thread-safe).
func (s *Server) GetNewestLogTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.logAddedAt) == 0 {
		return time.Time{}
	}
	return s.logAddedAt[len(s.logAddedAt)-1]
}

// GetLogTimestamps returns a copy of the logAddedAt timestamps (thread-safe).
// Used for checkpoint-based time-series queries.
func (s *Server) GetLogTimestamps() []time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.logAddedAt) == 0 {
		return nil
	}

	copy := make([]time.Time, len(s.logAddedAt))
	for i, t := range s.logAddedAt {
		copy[i] = t
	}
	return copy
}

// LogReader is an interface for reading log data from Server.
// Packages that need to analyze log state can depend on this interface
// instead of the Server struct directly, reducing coupling.
type LogReader interface {
	// GetLogSnapshot returns a snapshot of the current log state
	GetLogSnapshot() LogSnapshot
	// GetLogCount returns the current number of log entries
	GetLogCount() int
	// GetLogTotalAdded returns the monotonic total of entries ever added
	GetLogTotalAdded() int64
	// GetLogEntries returns a copy of all log entries
	GetLogEntries() []types.LogEntry
	// GetLogTimestamps returns a copy of all log entry timestamps
	GetLogTimestamps() []time.Time
	// GetOldestLogTime returns the timestamp of the oldest entry
	GetOldestLogTime() time.Time
	// GetNewestLogTime returns the timestamp of the newest entry
	GetNewestLogTime() time.Time
}

// Verify that Server implements LogReader
var _ LogReader = (*Server)(nil)

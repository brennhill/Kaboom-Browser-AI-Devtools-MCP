// Purpose: Implements core server log storage/handler state and file-backed log lifecycle operations.
// Why: Centralizes server-side log persistence and handler behavior behind one synchronized runtime.
// Docs: docs/features/feature/backend-log-streaming/index.md

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Server holds the server state
type Server struct {
	logFile       string
	maxEntries    int
	entries       []types.LogEntry
	logAddedAt    []time.Time // parallel slice: when each entry was added
	mu            sync.RWMutex
	logTotalAdded int64                  // monotonic counter of total entries ever added
	onEntries     func([]types.LogEntry) // optional callback when entries are added (e.g., for clustering)
	TTL           time.Duration          // TTL for read-time filtering (0 means unlimited)
	now           func() time.Time
}

// NewServer creates a new server instance
func NewServer(logFile string, maxEntries int) (*Server, error) {
	s := &Server{
		logFile:    logFile,
		maxEntries: maxEntries,
		entries:    make([]types.LogEntry, 0),
		now:        time.Now,
	}

	// Ensure log directory exists.
	dir := filepath.Dir(logFile)
	// #nosec G301 -- 0o755 is appropriate for log directory
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Load existing entries.
	if err := s.loadEntries(); err != nil {
		// File might not exist yet, that's OK.
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load existing entries: %w", err)
		}
	}

	return s, nil
}

// SetOnEntries sets the callback invoked when new log entries are added.
// Thread-safe: acquires the write lock to avoid racing with addEntries.
func (s *Server) SetOnEntries(cb func([]types.LogEntry)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onEntries = cb
}

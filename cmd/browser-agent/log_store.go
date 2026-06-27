// log_store.go — Focused log entry storage with TTL rotation and async channel pipeline.
// Why: Extracts log state from the Server god object into a single-purpose subsystem.

package main

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// LogEntry represents a single log entry (alias to internal/mcp).
type LogEntry = mcp.LogEntry

// defaultMaxFileSize is the log file size threshold for rotation (50MB).
const defaultMaxFileSize int64 = 50 * 1024 * 1024

// logCompactionFactor sets the file-rewrite hysteresis: the async worker only
// compacts (rewrites) the log file once it holds more than
// logCompactionFactor*maxEntries appended entries. Steady-state ingest is
// append-only; without hysteresis every POST past the in-memory cap would
// rewrite the whole file.
const logCompactionFactor = 2

// LogStore holds log entry state, TTL rotation, and async file I/O pipeline.
type LogStore struct {
	logFile     string
	maxEntries  int
	maxFileSize int64 // max log file size in bytes before rotation (0 = disabled)

	entries       []LogEntry
	logAddedAt    []time.Time // parallel slice: when each entry was added
	mu            sync.RWMutex
	logTotalAdded   int64            // monotonic counter of total entries ever added
	errorTotalAdded int64            // monotonic counter of error-level entries ever added
	telemetryMode   string           // telemetry summary verbosity: off|auto|full
	onEntries       func([]LogEntry) // optional callback when entries are added (e.g., for clustering)
	TTL             time.Duration    // TTL for read-time filtering (0 means unlimited)

	// Async logging
	logChan       chan []LogEntry // buffered channel for async log writes
	logDropCount  int64           // atomic counter for dropped logs (when channel full)
	logDone       chan struct{}   // signal when async logger exits
	logChanClosed atomic.Bool     // guards against double-close panic on logChan

	// Single-writer file persistence state. The async logger worker is the only
	// hot-path file writer; clearEntries (rare, user-triggered) synchronizes with
	// it via fileMu.
	fileMu           sync.Mutex   // serializes all log-file writes (worker appends/compaction, clear truncate)
	fileEntryCount   atomic.Int64 // approximate entry count in the log file (load/worker/clear maintained)
	clearGen         atomic.Int64 // bumped by clearEntries; lets the worker discard stale compaction snapshots
	fileAppendCount  atomic.Int64 // diagnostic: append batches written to the log file
	fileRewriteCount atomic.Int64 // diagnostic: full-file rewrites (tmp+rename) performed

	// addWarning is a callback to report warnings to the server.
	// Set during construction to avoid circular dependency.
	addWarning func(string)
}

// NewLogStore creates a new LogStore.
// The addWarning callback is used to surface diagnostics to the parent Server.
func NewLogStore(logFile string, maxEntries int, addWarning func(string)) *LogStore {
	return &LogStore{
		logFile:       logFile,
		maxEntries:    maxEntries,
		maxFileSize:   defaultMaxFileSize,
		entries:       make([]LogEntry, 0),
		telemetryMode: telemetryModeAuto,
		logChan:       make(chan []LogEntry, 10000), // 10k buffer for burst traffic
		logDone:       make(chan struct{}),
		addWarning:    addWarning,
	}
}

// SetOnEntries sets the callback invoked when new log entries are added.
// Thread-safe: acquires the write lock to avoid racing with addEntries.
func (ls *LogStore) SetOnEntries(cb func([]LogEntry)) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.onEntries = cb
}

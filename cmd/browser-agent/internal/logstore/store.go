// store.go — Focused log entry storage with TTL rotation and async channel pipeline.
// Why: Extracts log state from the Server god object into a single-purpose subsystem.

package logstore

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Entry represents a single log entry (alias to internal/mcp).
type Entry = mcp.LogEntry

// DefaultMaxFileSize is the log file size threshold for rotation (50MB).
const DefaultMaxFileSize int64 = 50 * 1024 * 1024

// defaultChanSize is the async write queue depth; sized for burst traffic.
const defaultChanSize = 10000

// compactionFactor sets the file-rewrite hysteresis: the async worker only
// compacts (rewrites) the log file once it holds more than
// compactionFactor*maxEntries appended entries. Steady-state ingest is
// append-only; without hysteresis every POST past the in-memory cap would
// rewrite the whole file.
const compactionFactor = 2

// Config carries the dependencies and tunables a Store is built with.
// AddWarning surfaces diagnostics to the parent Server; Stderrf writes
// operator-facing alerts through the caller's (swappable) stderr sink so
// nothing here can corrupt the MCP stdout transport.
type Config struct {
	LogFile       string
	MaxEntries    int
	TelemetryMode string
	AddWarning    func(string)
	Stderrf       func(format string, args ...any)
	// ChanSize overrides the async write queue depth (0 = defaultChanSize).
	ChanSize int
}

// Store holds log entry state, TTL rotation, and async file I/O pipeline.
type Store struct {
	logFile     string
	maxEntries  int
	maxFileSize int64 // max log file size in bytes before rotation (0 = disabled)

	entries         []Entry
	logAddedAt      []time.Time // parallel slice: when each entry was added
	mu              sync.RWMutex
	logTotalAdded   int64         // monotonic counter of total entries ever added
	errorTotalAdded int64         // monotonic counter of error-level entries ever added
	telemetryMode   string        // telemetry summary verbosity: off|auto|full
	onEntries       func([]Entry) // optional callback when entries are added (e.g., for clustering)
	TTL             time.Duration // TTL for read-time filtering (0 means unlimited)

	// Async logging
	logChan       chan []Entry  // buffered channel for async log writes
	logDropCount  int64         // atomic counter for dropped logs (when channel full)
	logDone       chan struct{} // signal when async logger exits
	logChanClosed atomic.Bool   // guards against double-close panic on logChan

	// Single-writer file persistence state. The async logger worker is the only
	// hot-path file writer; ClearEntries (rare, user-triggered) synchronizes with
	// it via fileMu.
	fileMu           sync.Mutex   // serializes all log-file writes (worker appends/compaction, clear truncate)
	fileEntryCount   atomic.Int64 // approximate entry count in the log file (load/worker/clear maintained)
	clearGen         atomic.Int64 // bumped by ClearEntries; lets the worker discard stale compaction snapshots
	fileAppendCount  atomic.Int64 // diagnostic: append batches written to the log file
	fileRewriteCount atomic.Int64 // diagnostic: full-file rewrites (tmp+rename) performed

	// addWarning is a callback to report warnings to the server.
	// Set during construction to avoid circular dependency.
	addWarning func(string)
	// stderrf writes operator alerts through the caller's stderr sink.
	stderrf func(format string, args ...any)
}

// New creates a new Store from cfg.
func New(cfg Config) *Store {
	chanSize := cfg.ChanSize
	if chanSize <= 0 {
		chanSize = defaultChanSize
	}
	addWarning := cfg.AddWarning
	if addWarning == nil {
		addWarning = func(string) {}
	}
	stderrf := cfg.Stderrf
	if stderrf == nil {
		stderrf = func(string, ...any) {}
	}
	return &Store{
		logFile:       cfg.LogFile,
		maxEntries:    cfg.MaxEntries,
		maxFileSize:   DefaultMaxFileSize,
		entries:       make([]Entry, 0),
		telemetryMode: cfg.TelemetryMode,
		logChan:       make(chan []Entry, chanSize),
		logDone:       make(chan struct{}),
		addWarning:    addWarning,
		stderrf:       stderrf,
	}
}

// SetOnEntries sets the callback invoked when new log entries are added.
// Thread-safe: acquires the write lock to avoid racing with AddEntries.
func (ls *Store) SetOnEntries(cb func([]Entry)) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.onEntries = cb
}

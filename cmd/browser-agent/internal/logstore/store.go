// store.go — Focused log entry storage with TTL rotation and async channel pipeline.
// Why: Extracts log state from the Server god object into a single-purpose subsystem.

package logstore

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

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

type entryWindow struct {
	entries   []types.LogEntry
	addedAt   []time.Time
	head      int
	size      int
	addedSize int
}

func newEntryWindow(capacity int) entryWindow {
	if capacity < 0 {
		capacity = 0
	}
	return entryWindow{
		entries: make([]types.LogEntry, capacity),
		addedAt: make([]time.Time, capacity),
	}
}

func (w *entryWindow) len() int { return w.size }

func (w *entryWindow) append(entry types.LogEntry, addedAt time.Time) {
	if len(w.entries) == 0 {
		return
	}
	index := (w.head + w.size) % len(w.entries)
	if w.size == len(w.entries) {
		index = w.head
		w.head = (w.head + 1) % len(w.entries)
	} else {
		w.size++
	}
	w.entries[index] = entry
	w.addedAt[index] = addedAt
	w.addedSize = w.size
}

func (w *entryWindow) clear() {
	for i := range w.entries {
		w.entries[i] = nil
		w.addedAt[i] = time.Time{}
	}
	w.head = 0
	w.size = 0
	w.addedSize = 0
}

func (w *entryWindow) snapshot() ([]types.LogEntry, []time.Time) {
	entries := make([]types.LogEntry, w.size)
	addedAt := make([]time.Time, w.addedSize)
	for i := 0; i < w.size; i++ {
		index := (w.head + i) % len(w.entries)
		entries[i] = w.entries[index]
		if i < w.addedSize {
			addedAt[i] = w.addedAt[index]
		}
	}
	return entries, addedAt
}

func (w *entryWindow) last() (types.LogEntry, bool) {
	if w.size == 0 {
		return nil, false
	}
	return w.entries[(w.head+w.size-1)%len(w.entries)], true
}

func (w *entryWindow) seed(entries []types.LogEntry, addedAt []time.Time) {
	existing, existingAt := w.snapshot()
	capacity := len(w.entries)
	if needed := len(existing) + len(entries); capacity < needed {
		capacity = needed
	}
	*w = newEntryWindow(capacity)
	for i, entry := range existing {
		var timestamp time.Time
		if i < len(existingAt) {
			timestamp = existingAt[i]
		}
		w.append(entry, timestamp)
	}
	w.addedSize = len(existingAt)
	for i, entry := range entries {
		index := (w.head + w.size) % len(w.entries)
		w.entries[index] = entry
		w.size++
		if addedAt != nil && i < len(addedAt) {
			w.addedAt[index] = addedAt[i]
			w.addedSize++
		}
	}
}

// Store holds log entry state, TTL rotation, and async file I/O pipeline.
type Store struct {
	logFile     string
	maxEntries  int
	maxFileSize int64 // max log file size in bytes before rotation (0 = disabled)

	window          entryWindow
	mu              sync.RWMutex
	logTotalAdded   int64                  // monotonic counter of total entries ever added
	errorTotalAdded int64                  // monotonic counter of error-level entries ever added
	telemetryMode   string                 // telemetry summary verbosity: off|auto|full
	onEntries       func([]types.LogEntry) // optional callback when entries are added (e.g., for clustering)
	TTL             time.Duration          // TTL for read-time filtering (0 means unlimited)

	// Async logging
	logChan       chan []types.LogEntry // buffered channel for async log writes
	logDropCount  int64                 // atomic counter for dropped logs (when channel full)
	logDone       chan struct{}         // signal when async logger exits
	logChanClosed atomic.Bool           // guards against double-close panic on logChan

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
		window:        newEntryWindow(cfg.MaxEntries),
		telemetryMode: cfg.TelemetryMode,
		logChan:       make(chan []types.LogEntry, chanSize),
		logDone:       make(chan struct{}),
		addWarning:    addWarning,
		stderrf:       stderrf,
	}
}

// SetOnEntries sets the callback invoked when new log entries are added.
// Thread-safe: acquires the write lock to avoid racing with AddEntries.
func (ls *Store) SetOnEntries(cb func([]types.LogEntry)) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.onEntries = cb
}

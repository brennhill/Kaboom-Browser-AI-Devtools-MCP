// accessors.go — Thread-safe accessors and mutators for in-memory logging state.
// Why: Keeps read/write state helpers separate from async queue and file I/O behavior,
// and gives callers outside this package a lock-respecting alternative to poking fields.

package logstore

import (
	"sync/atomic"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// DropCount returns the total number of dropped log entries (thread-safe).
func (ls *Store) DropCount() int64 {
	return atomic.LoadInt64(&ls.logDropCount)
}

// EntryCount returns current entry count.
func (ls *Store) EntryCount() int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.window.len()
}

// ErrorTotalAdded returns the total number of error-level log entries ever added.
func (ls *Store) ErrorTotalAdded() int64 {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.errorTotalAdded
}

// TotalAdded returns the monotonic counter of total entries ever added.
func (ls *Store) TotalAdded() int64 {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.logTotalAdded
}

// TelemetryMode returns the telemetry summary verbosity (off|auto|full).
func (ls *Store) TelemetryMode() string {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.telemetryMode
}

// SetTelemetryMode sets the telemetry summary verbosity.
func (ls *Store) SetTelemetryMode(mode string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.telemetryMode = mode
}

// Entries returns a copy of all entries.
func (ls *Store) Entries() []types.LogEntry {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	entries, _ := ls.window.snapshot()
	return entries
}

// EntriesWithAddedAt returns copies of the entry window and its parallel
// add-time slice, captured under a single read lock so the two stay aligned.
func (ls *Store) EntriesWithAddedAt() ([]types.LogEntry, []time.Time) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.window.snapshot()
}

// LastEntry returns the most recent entry, or ok=false when the window is empty.
func (ls *Store) LastEntry() (types.LogEntry, bool) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.window.last()
}

// LogFile returns the configured log file path ("" = persistence disabled).
func (ls *Store) LogFile() string { return ls.logFile }

// SetLogFile redirects persistence to path. Startup-only: the async worker
// reads this field without synchronization, so it must not be called once
// entries are being ingested.
func (ls *Store) SetLogFile(path string) { ls.logFile = path }

// MaxEntries returns the in-memory window capacity.
func (ls *Store) MaxEntries() int { return ls.maxEntries }

// MaxFileSize returns the log-file rotation threshold in bytes (0 = disabled).
func (ls *Store) MaxFileSize() int64 { return ls.maxFileSize }

// SetMaxFileSize sets the log-file rotation threshold in bytes (0 disables
// rotation). Startup-only, for the same reason as SetLogFile.
func (ls *Store) SetMaxFileSize(n int64) { ls.maxFileSize = n }

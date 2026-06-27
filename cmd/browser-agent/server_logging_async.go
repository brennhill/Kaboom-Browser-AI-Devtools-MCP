// Purpose: Async logging pipeline and in-memory entry bookkeeping.
// Why: Separates log ingestion/rotation mechanics from server construction and API surface.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// addEntries adds new entries to the in-memory window and queues them for
// append-only persistence. The hot path performs no file I/O: all file writes
// (appends, compaction rewrites, size rotation) happen on the async logger
// worker goroutine, which is the single file writer.
func (ls *LogStore) addEntries(newEntries []LogEntry) int {
	appendOnly, cb := ls.addEntriesInMemory(newEntries)

	if err := ls.appendToFile(appendOnly); err != nil {
		ls.addWarning(fmt.Sprintf("log_append_failed: %v", err))
	}

	// Notify listeners outside the lock (e.g., cluster manager)
	if cb != nil {
		cb(newEntries)
	}

	return len(newEntries)
}

// addEntriesInMemory mutates log state under lock and returns a snapshot of the
// new entries for async file I/O outside the lock. The in-memory window is
// trimmed to maxEntries here; the file is compacted separately by the async
// worker (see maybeCompactLogFile) so steady-state ingest stays append-only.
func (ls *LogStore) addEntriesInMemory(newEntries []LogEntry) (appendOnly []LogEntry, cb func([]LogEntry)) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.logTotalAdded += int64(len(newEntries))
	for _, entry := range newEntries {
		level, ok := entry["level"].(string)
		if ok && level == "error" {
			ls.errorTotalAdded++
		}
	}

	now := time.Now()
	for range newEntries {
		ls.logAddedAt = append(ls.logAddedAt, now)
	}
	ls.entries = append(ls.entries, newEntries...)

	// Trim the in-memory window — copy to new slice to allow GC of evicted entries
	if len(ls.entries) > ls.maxEntries {
		kept := make([]LogEntry, ls.maxEntries)
		copy(kept, ls.entries[len(ls.entries)-ls.maxEntries:])
		ls.entries = kept
		keptAt := make([]time.Time, ls.maxEntries)
		copy(keptAt, ls.logAddedAt[len(ls.logAddedAt)-ls.maxEntries:])
		ls.logAddedAt = keptAt
	}

	// Snapshot new entries for file I/O outside the lock
	appendOnly = make([]LogEntry, len(newEntries))
	copy(appendOnly, newEntries)
	cb = ls.onEntries
	return appendOnly, cb
}

// asyncLoggerWorker runs in a background goroutine and is the single writer
// for the log file: it appends queued batches and performs occasional
// compaction rewrites. No other goroutine writes the file on the hot path.
func (ls *LogStore) asyncLoggerWorker() {
	defer close(ls.logDone)

	for entries := range ls.logChan {
		// Synchronous file I/O happens here (off the hot path)
		if err := ls.appendToFileSync(entries); err != nil {
			ls.addWarning(fmt.Sprintf("log_write_failed: %v", err))
		}
		ls.maybeCompactLogFile()
	}
}

// appendToFileSync does synchronous file I/O (called by async worker only).
func (ls *LogStore) appendToFileSync(entries []LogEntry) error {
	if ls.logFile == "" {
		return nil
	}
	ls.fileMu.Lock()
	defer ls.fileMu.Unlock()
	f, err := os.OpenFile(ls.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- path set at startup
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			ls.addWarning(fmt.Sprintf("log_close_failed: %v", closeErr))
		}
	}()
	ls.fileAppendCount.Add(1)

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
		ls.fileEntryCount.Add(1)
	}

	// Check file size and rotate if needed (non-blocking, off the hot path)
	ls.maybeRotateLogFile(f)

	return nil
}

// maybeRotateLogFile checks the log file size and rotates if it exceeds maxFileSize.
// Rotation renames the current file to .jsonl.old and lets the next write create a fresh file.
// Called only from the async logger worker, so no additional locking is needed for file I/O.
func (ls *LogStore) maybeRotateLogFile(f *os.File) {
	if ls.maxFileSize <= 0 {
		return
	}

	fi, err := f.Stat()
	if err != nil {
		return
	}
	if fi.Size() <= ls.maxFileSize {
		return
	}

	oldFile := ls.logFile + ".old"
	// Rename overwrites any existing .old file atomically on POSIX systems
	if err := os.Rename(ls.logFile, oldFile); err != nil { // #nosec G703 -- s.logFile is configured by local operator, not remote input
		ls.addWarning(fmt.Sprintf("log_rotate_failed: %v", err))
		return
	}
	// The next append starts a fresh, empty file.
	ls.fileEntryCount.Store(0)
	ls.addWarning(fmt.Sprintf("log_rotated: %s -> %s (%d bytes)", ls.logFile, oldFile, fi.Size()))
}

// maybeCompactLogFile rewrites the log file with the in-memory window once the
// file has accumulated more than logCompactionFactor*maxEntries entries.
// Hysteresis keeps steady-state ingest append-only (append-only I/O on hot
// paths); the rewrite runs on the async worker goroutine so the file has a
// single writer. Compaction is deferred while batches are still queued:
// rewriting from memory would re-append queued entries afterwards (duplicates).
func (ls *LogStore) maybeCompactLogFile() {
	if ls.logFile == "" || ls.maxEntries <= 0 {
		return
	}
	if ls.fileEntryCount.Load() <= int64(logCompactionFactor*ls.maxEntries) {
		return
	}
	if len(ls.logChan) > 0 {
		return
	}

	gen := ls.clearGen.Load()
	snapshot := ls.getEntries()

	ls.fileMu.Lock()
	defer ls.fileMu.Unlock()
	if ls.clearGen.Load() != gen {
		// clearEntries ran after the snapshot was taken; writing the snapshot
		// would resurrect cleared entries. The file was already truncated.
		return
	}
	if err := ls.saveEntriesCopy(snapshot); err != nil {
		ls.addWarning(fmt.Sprintf("log_compact_failed: %v", err))
		return
	}
	ls.fileEntryCount.Store(int64(len(snapshot)))
}

// appendToFile queues log entries for async writing (never blocks).
func (ls *LogStore) appendToFile(entries []LogEntry) error {
	if ls.logChanClosed.Load() {
		return fmt.Errorf("log channel closed, %d entries dropped", len(entries))
	}
	select {
	case ls.logChan <- entries:
		// Queued successfully
		return nil
	default:
		// Channel full - drop log to maintain availability
		dropped := atomic.AddInt64(&ls.logDropCount, 1)

		// Alert to stderr (but don't spam)
		if dropped%1000 == 1 { // Alert on 1st, 1001st, 2001st, etc.
			stderrf("[Kaboom] WARNING: Log buffer full, %d logs dropped\n", dropped)
		}

		return fmt.Errorf("log buffer full (%d total drops)", dropped)
	}
}

// clearEntries removes all entries from memory and truncates the log file.
// The truncate synchronizes with the async worker via fileMu so it cannot
// interleave with an in-progress append or compaction, and the clear
// generation bump makes the worker discard compaction snapshots taken before
// the clear. Batches already queued in logChan before the clear may still be
// appended afterwards; that window is short and the next compaction rewrites
// the file from (cleared) memory.
func (ls *LogStore) clearEntries() {
	ls.clearEntriesInMemory()
	if ls.logFile == "" {
		return
	}
	ls.fileMu.Lock()
	defer ls.fileMu.Unlock()
	// #nosec G306 -- log files are owner-only (0600) for privacy
	if err := os.WriteFile(ls.logFile, []byte{}, 0600); err != nil {
		ls.addWarning(fmt.Sprintf("log_clear_failed: %v", err))
		return
	}
	ls.fileEntryCount.Store(0)
}

func (ls *LogStore) clearEntriesInMemory() {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.entries = nil
	ls.logAddedAt = nil
	ls.clearGen.Add(1)
}

// shutdownAsyncLogger gracefully shuts down the async logger, draining remaining logs.
// Safe to call multiple times (e.g., from both Server.Close and awaitShutdownSignal).
func (ls *LogStore) shutdownAsyncLogger(timeout time.Duration) {
	// Guard against double-close panic: only the first caller closes the channel.
	if !ls.logChanClosed.CompareAndSwap(false, true) {
		return
	}
	close(ls.logChan)

	// Wait for worker to finish draining, with timeout
	select {
	case <-ls.logDone:
		// Worker exited cleanly
		dropped := atomic.LoadInt64(&ls.logDropCount)
		if dropped > 0 {
			ls.addWarning(fmt.Sprintf("log_drops: %d logs were dropped during session", dropped))
		}
	case <-time.After(timeout):
		// Timeout - worker still draining, but we need to exit
		ls.addWarning(fmt.Sprintf("log_shutdown_timeout: %d logs may be lost", len(ls.logChan)))
	}
}

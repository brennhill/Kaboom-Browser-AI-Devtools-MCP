// persistence.go — Log-file persistence helpers for loading, saving, and writability checks.
// Why: Isolates disk persistence logic from in-memory server state orchestration.

package logstore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// LoadEntries reads existing log entries from file.
func (ls *Store) LoadEntries() error {
	file, err := os.Open(ls.logFile)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // deferred close

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // Allow up to 10MB per line (screenshots can be large)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry types.LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // Skip malformed lines
		}
		lineCount++
		ls.window.append(entry, time.Time{})
	}

	// Seed the file entry count for compaction hysteresis: if the file already
	// holds more than compactionFactor*maxEntries entries, the async worker
	// compacts it after the next append.
	ls.fileEntryCount.Store(int64(lineCount))

	return scanner.Err()
}

// saveEntriesCopy atomically rewrites the log file with the given snapshot.
// Uses atomic write pattern: write to temp file then rename for crash safety.
// Called only from the async logger worker (maybeCompactLogFile); the caller
// must hold fileMu so the rewrite cannot interleave with appends or clears.
func (ls *Store) saveEntriesCopy(entries []types.LogEntry) error {
	if ls.logFile == "" {
		return nil
	}
	// Write to temporary file first, then atomically rename
	// This ensures log file remains intact if write fails partway through
	tmpFile := ls.logFile + ".tmp"
	file, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			ls.addWarning(fmt.Sprintf("log_temp_close_failed: %v", closeErr))
		}
	}()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		if _, err := file.Write(data); err != nil {
			// Clean up temp file on write failure
			_ = os.Remove(tmpFile)
			return err
		}
		if _, err := file.WriteString("\n"); err != nil {
			// Clean up temp file on write failure
			_ = os.Remove(tmpFile)
			return err
		}
	}

	// Atomic rename: ensures log file is only updated if write succeeded
	if err := os.Rename(tmpFile, ls.logFile); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}

	ls.fileRewriteCount.Add(1)
	return nil
}

// FallbackFilePath is the log destination used when the configured directory
// is not writable.
func FallbackFilePath() string {
	return filepath.Join(os.TempDir(), "kaboom", "logs", "kaboom.jsonl")
}

// EnsureFileWritable verifies the process can append to path.
func EnsureFileWritable(path string) error {
	if path == "" {
		return fmt.Errorf("log_init: log file path is empty. Set a valid path via --log-file or KABOOM_LOG_FILE")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- local path configured at startup
	if err != nil {
		return err
	}
	return f.Close()
}

// PreparePersistence creates and validates the configured log destination,
// redirects an unusable destination to the local fallback, and loads existing
// entries. It never terminates the process; all degradation is surfaced through
// addWarning.
func PreparePersistence(store *Store, addWarning func(string)) {
	if store == nil || store.LogFile() == "" {
		return
	}
	if addWarning == nil {
		addWarning = func(string) {}
	}
	ensureDestination := func(path string) error {
		// #nosec G301 -- local diagnostic directory needs owner rwx and group rx.
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		return EnsureFileWritable(path)
	}

	if err := ensureDestination(store.LogFile()); err != nil {
		fallback := FallbackFilePath()
		addWarning(fmt.Sprintf("state_dir_not_writable: %v; falling back to %s", err, fallback))
		store.SetLogFile(fallback)
		if fallbackErr := ensureDestination(fallback); fallbackErr != nil {
			addWarning(fmt.Sprintf("log_persistence_disabled: %v", fallbackErr))
			store.SetLogFile("")
			return
		}
	}

	if err := store.LoadEntries(); err != nil && !os.IsNotExist(err) {
		addWarning(fmt.Sprintf("log_load_failed: %v", err))
	}
}

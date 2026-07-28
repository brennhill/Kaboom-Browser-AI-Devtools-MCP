// Purpose: Regression and invariant tests for the async log persistence pipeline.
// Docs: docs/features/feature/mcp-persistent-server/index.md
//
// These tests pin the single-writer, append-only hot-path contract:
//   - Every POST-equivalent AddEntries call results in a file APPEND queued to
//     the async worker; the request goroutine never rewrites the whole file.
//   - Full-file rewrites (compaction) happen only on the worker goroutine, and
//     only when the file holds more than compactionFactor*maxEntries entries
//     (hysteresis), so steady-state ingest past the in-memory cap stays cheap.

package logstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// newAsyncLogStoreForTest builds a Store with a running worker, mirroring
// the wiring in NewServer (worker started before any entries are added).
func newAsyncLogStoreForTest(t *testing.T, maxEntries int) (*Store, string) {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "async.jsonl")
	ls := New(Config{LogFile: logFile, MaxEntries: maxEntries, AddWarning: func(string) {}})
	go ls.RunWorker()
	return ls, logFile
}

func readLogLines(t *testing.T, logFile string) []string {
	t.Helper()
	data, err := os.ReadFile(logFile) // nosemgrep: go_filesystem_rule-fileread -- test reads its own output file
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logFile, err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// TestLogStoreSteadyStatePostsAppendInsteadOfRewriting is the core regression
// test for the hot-path rewrite bug: once total volume passed maxEntries, every
// subsequent AddEntries call (one per /logs POST) synchronously rewrote the
// entire log file on the request goroutine instead of appending.
//
// Contract: every add must produce exactly one queued append batch, and
// full-file rewrites must be rare (hysteresis), not once per POST.
func TestLogStoreSteadyStatePostsAppendInsteadOfRewriting(t *testing.T) {
	const maxEntries = 5
	const posts = 26
	ls, _ := newAsyncLogStoreForTest(t, maxEntries)

	for i := 0; i < posts; i++ {
		ls.AddEntries([]types.LogEntry{{"level": "info", "message": fmt.Sprintf("entry-%d", i)}})
	}
	ls.Shutdown(2 * time.Second)

	appends := ls.fileAppendCount.Load()
	rewrites := ls.fileRewriteCount.Load()
	if appends != posts {
		t.Fatalf("append batches = %d, want %d (every POST must append; hot path must not rewrite)", appends, posts)
	}
	if rewrites >= posts/2 {
		t.Fatalf("full-file rewrites = %d for %d posts; steady state must be append-only with occasional compaction", rewrites, posts)
	}

	// In-memory window semantics unchanged: last maxEntries entries.
	entries := ls.Entries()
	if len(entries) != maxEntries {
		t.Fatalf("len(entries) = %d, want %d", len(entries), maxEntries)
	}
	if entries[maxEntries-1]["message"] != fmt.Sprintf("entry-%d", posts-1) {
		t.Fatalf("newest in-memory entry = %v, want entry-%d", entries[maxEntries-1]["message"], posts-1)
	}
}

// TestLogStoreCompactionRewritesAfterThreshold verifies the hysteresis: the
// worker compacts the file (tmp+rename rewrite of the in-memory window) only
// once the file exceeds compactionFactor*maxEntries entries.
func TestLogStoreCompactionRewritesAfterThreshold(t *testing.T) {
	const maxEntries = 5
	// One past the threshold: counts 1..10 stay append-only, 11 triggers compaction.
	const posts = compactionFactor*maxEntries + 1
	ls, logFile := newAsyncLogStoreForTest(t, maxEntries)

	for i := 0; i < posts; i++ {
		ls.AddEntries([]types.LogEntry{{"level": "info", "message": fmt.Sprintf("entry-%d", i)}})
	}
	ls.Shutdown(2 * time.Second)

	if appends := ls.fileAppendCount.Load(); appends != posts {
		t.Fatalf("append batches = %d, want %d", appends, posts)
	}
	if rewrites := ls.fileRewriteCount.Load(); rewrites != 1 {
		t.Fatalf("full-file rewrites = %d, want exactly 1 (compaction after crossing %d entries)", rewrites, compactionFactor*maxEntries)
	}

	// After compaction the file holds exactly the in-memory window.
	lines := readLogLines(t, logFile)
	if len(lines) != maxEntries {
		t.Fatalf("file line count = %d, want %d after compaction", len(lines), maxEntries)
	}
	var first, last types.LogEntry
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first line not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("last line not valid JSON: %v", err)
	}
	if first["message"] != fmt.Sprintf("entry-%d", posts-maxEntries) || last["message"] != fmt.Sprintf("entry-%d", posts-1) {
		t.Fatalf("compacted window = [%v .. %v], want [entry-%d .. entry-%d]",
			first["message"], last["message"], posts-maxEntries, posts-1)
	}
	if ls.fileEntryCount.Load() != int64(maxEntries) {
		t.Fatalf("fileEntryCount = %d, want %d after compaction", ls.fileEntryCount.Load(), maxEntries)
	}

	// Crash-safety artifact must not linger.
	if _, err := os.Stat(logFile + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("compaction left %q behind", logFile+".tmp")
	}
}

// TestLogStoreClearEntriesTruncatesFileAndResetsCount verifies DELETE /logs
// semantics survive the single-writer refactor: the file is emptied and the
// worker's file-entry accounting resets so compaction does not fire spuriously.
func TestLogStoreClearEntriesTruncatesFileAndResetsCount(t *testing.T) {
	ls, logFile := newAsyncLogStoreForTest(t, 5)

	ls.AddEntries([]types.LogEntry{
		{"level": "info", "message": "a"},
		{"level": "info", "message": "b"},
		{"level": "info", "message": "c"},
	})
	ls.Shutdown(2 * time.Second)

	ls.ClearEntries()

	if got := ls.EntryCount(); got != 0 {
		t.Fatalf("EntryCount() = %d, want 0 after clear", got)
	}
	if lines := readLogLines(t, logFile); len(lines) != 0 {
		t.Fatalf("file has %d lines after clear, want 0", len(lines))
	}
	if got := ls.fileEntryCount.Load(); got != 0 {
		t.Fatalf("fileEntryCount = %d, want 0 after clear", got)
	}
}

// TestLogStoreConcurrentAddsProduceValidFile exercises concurrent POST
// goroutines against the single async writer. Run with -race in CI. Before the
// single-writer fix, concurrent request goroutines rewrote the same .tmp path
// while the worker appended to a possibly renamed-away fd, so lines could be
// lost or interleaved mid-record.
func TestLogStoreConcurrentAddsProduceValidFile(t *testing.T) {
	const maxEntries = 10
	const goroutines = 4
	const addsPerGoroutine = 30
	ls, logFile := newAsyncLogStoreForTest(t, maxEntries)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < addsPerGoroutine; i++ {
				ls.AddEntries([]types.LogEntry{{"level": "info", "message": fmt.Sprintf("g%d-%d", g, i)}})
			}
		}(g)
	}
	wg.Wait()
	ls.Shutdown(2 * time.Second)

	if got := ls.EntryCount(); got != maxEntries {
		t.Fatalf("EntryCount() = %d, want %d", got, maxEntries)
	}
	for i, line := range readLogLines(t, logFile) {
		var entry types.LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("file line %d is not valid JSON (%v): %q", i, err, line)
		}
	}
	if _, err := os.Stat(logFile + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("writer left %q behind", logFile+".tmp")
	}
}

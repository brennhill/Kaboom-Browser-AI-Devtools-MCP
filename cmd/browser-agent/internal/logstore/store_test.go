// Purpose: Unit tests for the log store's window, rotation, drop and file-sync logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package logstore

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// newStoreForTest mirrors NewServer's wiring: build the store, start the single
// writer, then load any pre-existing file contents.
func newStoreForTest(t *testing.T, logFile string, maxEntries int) *Store {
	t.Helper()
	ls := New(Config{LogFile: logFile, MaxEntries: maxEntries, AddWarning: func(string) {}})
	go ls.RunWorker()
	if logFile != "" {
		if err := ls.LoadEntries(); err != nil && !os.IsNotExist(err) {
			t.Fatalf("LoadEntries() error = %v", err)
		}
	}
	return ls
}

func TestStoreAddEntriesRotationPath(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotation.jsonl")
	ls := newStoreForTest(t, logFile, 2)

	added := ls.AddEntries([]types.LogEntry{
		{"level": "info", "message": "a"},
		{"level": "info", "message": "b"},
		{"level": "info", "message": "c"},
	})
	if added != 3 {
		t.Fatalf("AddEntries() = %d, want 3", added)
	}

	entries := ls.Entries()
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2 after rotation", len(entries))
	}
	if entries[0]["message"] != "b" || entries[1]["message"] != "c" {
		t.Fatalf("rotated entries = %+v, want last two entries", entries)
	}

	ls.Shutdown(2 * time.Second)
	data, err := os.ReadFile(logFile) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logFile, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Persistence is append-only on the hot path: all 3 entries are appended.
	// The file is only compacted down to the in-memory window once it exceeds
	// compactionFactor*maxEntries entries (see
	// TestLogStoreCompactionRewritesAfterThreshold); LoadEntries bounds reads.
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3 (append-only persistence)", len(lines))
	}
}

func TestStoreSetOnEntriesAndAppendPath(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "append.jsonl")
	ls := newStoreForTest(t, logFile, 10)

	var callbackCount atomic.Int32
	ls.SetOnEntries(func(entries []types.LogEntry) {
		callbackCount.Add(int32(len(entries)))
	})

	added := ls.AddEntries([]types.LogEntry{{"level": "info", "message": "hello"}})
	if added != 1 {
		t.Fatalf("AddEntries() = %d, want 1", added)
	}
	ls.Shutdown(2 * time.Second)

	if got := callbackCount.Load(); got != 1 {
		t.Fatalf("callback count = %d, want 1", got)
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logFile, err)
	}
	if !strings.Contains(string(data), `"message":"hello"`) {
		t.Fatalf("log file missing appended entry: %q", string(data))
	}
}

func TestStoreLoadEntriesBoundsAndMalformedLines(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "load.jsonl")
	content := strings.Join([]string{
		`{"level":"info","message":"first"}`,
		`malformed-json`,
		`{"level":"warn","message":"second"}`,
		`{"level":"error","message":"third"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(logFile, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", logFile, err)
	}

	ls := newStoreForTest(t, logFile, 2)
	defer ls.Shutdown(2 * time.Second)

	entries := ls.Entries()
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0]["message"] != "second" || entries[1]["message"] != "third" {
		t.Fatalf("loaded entries = %+v, want last two valid entries", entries)
	}
}

func TestStoreAppendToFileDropAndShutdownTimeout(t *testing.T) {
	ls := New(Config{ChanSize: 1, AddWarning: func(string) {}})
	ls.logChan <- queuedBatch{entries: []types.LogEntry{{"level": "info", "message": "queued"}}}
	if err := ls.AppendToFile([]types.LogEntry{{"level": "info", "message": "drop"}}); err == nil {
		t.Fatal("AppendToFile() expected drop error when channel is full")
	}
	if dropped := ls.DropCount(); dropped != 1 {
		t.Fatalf("DropCount() = %d, want 1", dropped)
	}

	ls.Shutdown(10 * time.Millisecond)
}

func TestStoreFileRotationOnSizeExceeded(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-size.jsonl")
	ls := newStoreForTest(t, logFile, 10000)
	// Set a tiny max file size (1KB) to trigger rotation quickly
	ls.SetMaxFileSize(1024)

	// Write enough entries to exceed 1KB (triggers rotation)
	var entries []types.LogEntry
	for i := 0; i < 50; i++ {
		entries = append(entries, types.LogEntry{"level": "info", "message": strings.Repeat("x", 100)})
	}
	ls.AddEntries(entries)

	// Let async worker process and rotate
	time.Sleep(50 * time.Millisecond)

	// Write a second small batch so a new main file is created after rotation
	ls.AddEntries([]types.LogEntry{{"level": "info", "message": "after-rotation"}})
	ls.Shutdown(2 * time.Second)

	// The .old file should exist after rotation
	oldFile := logFile + ".old"
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		t.Fatalf("expected %q to exist after file rotation", oldFile)
	}

	// The main log file should exist and be smaller than the old file
	mainInfo, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", logFile, err)
	}
	oldInfo, err := os.Stat(oldFile)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", oldFile, err)
	}
	if mainInfo.Size() >= oldInfo.Size() {
		t.Fatalf("main file (%d bytes) should be smaller than old file (%d bytes)",
			mainInfo.Size(), oldInfo.Size())
	}
}

func TestStoreFileRotationCreatesOldFile(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-old.jsonl")
	ls := newStoreForTest(t, logFile, 10000)
	// 512 bytes to trigger on just a handful of entries
	ls.SetMaxFileSize(512)

	// Write entries in two batches to trigger rotation
	batch1 := []types.LogEntry{
		{"level": "info", "message": strings.Repeat("a", 200)},
		{"level": "info", "message": strings.Repeat("b", 200)},
		{"level": "info", "message": strings.Repeat("c", 200)},
	}
	ls.AddEntries(batch1)

	// Let the async logger process
	time.Sleep(50 * time.Millisecond)

	batch2 := []types.LogEntry{
		{"level": "info", "message": strings.Repeat("d", 200)},
	}
	ls.AddEntries(batch2)
	ls.Shutdown(2 * time.Second)

	// Old file should exist
	oldFile := logFile + ".old"
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		t.Fatalf("expected %q to exist after file rotation", oldFile)
	}

	// New main file should be valid JSONL (readable)
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logFile, err)
	}
	if len(data) == 0 {
		t.Fatal("main log file should not be empty after rotation (new writes go there)")
	}
}

func TestStoreFileRotationOverwritesExistingOld(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-overwrite.jsonl")
	oldFile := logFile + ".old"

	// Create a pre-existing .old file
	if err := os.WriteFile(oldFile, []byte("stale-old-data\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", oldFile, err)
	}

	ls := newStoreForTest(t, logFile, 10000)
	ls.SetMaxFileSize(256)

	entries := []types.LogEntry{
		{"level": "info", "message": strings.Repeat("z", 200)},
		{"level": "info", "message": strings.Repeat("y", 200)},
	}
	ls.AddEntries(entries)
	ls.Shutdown(2 * time.Second)

	// Old file should be overwritten (no longer contain stale data)
	data, err := os.ReadFile(oldFile) // nosemgrep: go_filesystem_rule-fileread
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", oldFile, err)
	}
	if strings.Contains(string(data), "stale-old-data") {
		t.Fatal("old file should have been overwritten by rotation, still contains stale data")
	}
}

func TestStoreFileRotationDefaultMaxFileSize(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-default.jsonl")
	ls := newStoreForTest(t, logFile, 100)
	defer ls.Shutdown(2 * time.Second)

	// Default max file size should be 50MB
	if ls.MaxFileSize() != 50*1024*1024 {
		t.Fatalf("default MaxFileSize() = %d, want %d", ls.MaxFileSize(), 50*1024*1024)
	}
}

func TestStoreFileRotationZeroDisablesRotation(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-disabled.jsonl")
	ls := newStoreForTest(t, logFile, 10000)
	// Explicitly disable file rotation
	ls.SetMaxFileSize(0)

	entries := []types.LogEntry{
		{"level": "info", "message": strings.Repeat("x", 200)},
		{"level": "info", "message": strings.Repeat("y", 200)},
	}
	ls.AddEntries(entries)
	ls.Shutdown(2 * time.Second)

	// No .old file should exist
	oldFile := logFile + ".old"
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected no .old file when rotation disabled, but it exists")
	}
}

func TestStoreDropCount(t *testing.T) {
	t.Parallel()

	var alerts atomic.Int32
	ls := New(Config{
		ChanSize:   1,
		AddWarning: func(string) {},
		Stderrf:    func(string, ...any) { alerts.Add(1) },
	})

	// Initially zero
	if got := ls.DropCount(); got != 0 {
		t.Fatalf("DropCount() = %d, want 0", got)
	}

	// Fill channel, then trigger a drop
	ls.logChan <- queuedBatch{entries: []types.LogEntry{{"level": "info", "message": "fill"}}}
	_ = ls.AppendToFile([]types.LogEntry{{"level": "info", "message": "drop"}})

	if got := ls.DropCount(); got != 1 {
		t.Fatalf("DropCount() = %d, want 1", got)
	}

	// Trigger a second drop
	_ = ls.AppendToFile([]types.LogEntry{{"level": "info", "message": "drop2"}})

	if got := ls.DropCount(); got != 2 {
		t.Fatalf("DropCount() = %d, want 2", got)
	}
	// Only the 1st drop (and every 1000th after) alerts the operator sink.
	if got := alerts.Load(); got != 1 {
		t.Fatalf("stderr alerts = %d, want 1 (throttled to 1st drop)", got)
	}

	ls.Shutdown(10 * time.Millisecond)
}

func TestStoreAppendToFileSyncSkipsUnmarshalableEntry(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "sync.jsonl")
	ls := New(Config{LogFile: logFile, AddWarning: func(string) {}})
	err := ls.appendToFileSync([]types.LogEntry{
		{"level": "info", "message": "ok"},
		{"level": "info", "value": math.NaN()},
	}, ls.clearGen.Load())
	if err != nil {
		t.Fatalf("appendToFileSync() error = %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logFile, err)
	}
	text := string(data)
	if !strings.Contains(text, `"message":"ok"`) {
		t.Fatalf("valid entry missing from file: %q", text)
	}
	if strings.Contains(text, "NaN") {
		t.Fatalf("unmarshalable entry should be skipped: %q", text)
	}
}

func TestStoreAccessorsRoundTrip(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "accessors.jsonl")
	ls := newStoreForTest(t, logFile, 5)
	defer ls.Shutdown(2 * time.Second)

	if got := ls.LogFile(); got != logFile {
		t.Fatalf("LogFile() = %q, want %q", got, logFile)
	}
	if got := ls.MaxEntries(); got != 5 {
		t.Fatalf("MaxEntries() = %d, want 5", got)
	}
	if _, ok := ls.LastEntry(); ok {
		t.Fatal("LastEntry() ok = true on empty store, want false")
	}

	ls.SetTelemetryMode("full")
	if got := ls.TelemetryMode(); got != "full" {
		t.Fatalf("TelemetryMode() = %q, want %q", got, "full")
	}

	ls.AddEntries([]types.LogEntry{
		{"level": "info", "message": "one"},
		{"level": "error", "message": "two"},
	})

	if got := ls.TotalAdded(); got != 2 {
		t.Fatalf("TotalAdded() = %d, want 2", got)
	}
	if got := ls.ErrorTotalAdded(); got != 1 {
		t.Fatalf("ErrorTotalAdded() = %d, want 1", got)
	}
	if got := ls.EntryCount(); got != 2 {
		t.Fatalf("EntryCount() = %d, want 2", got)
	}
	last, ok := ls.LastEntry()
	if !ok || last["message"] != "two" {
		t.Fatalf("LastEntry() = %v, %v; want the newest entry", last, ok)
	}
	entries, addedAt := ls.EntriesWithAddedAt()
	if len(entries) != 2 || len(addedAt) != 2 {
		t.Fatalf("EntriesWithAddedAt() = %d entries / %d timestamps, want 2/2", len(entries), len(addedAt))
	}
	if addedAt[0].IsZero() {
		t.Fatal("EntriesWithAddedAt() returned a zero add-time for a live entry")
	}
}

func TestEnsureFileWritable(t *testing.T) {
	if err := EnsureFileWritable(""); err == nil {
		t.Fatal("EnsureFileWritable(\"\") = nil, want error for empty path")
	}
	path := filepath.Join(t.TempDir(), "writable.jsonl")
	if err := EnsureFileWritable(path); err != nil {
		t.Fatalf("EnsureFileWritable(%q) error = %v", path, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("EnsureFileWritable did not create %q: %v", path, err)
	}
	if got := FallbackFilePath(); !strings.HasSuffix(got, filepath.Join("kaboom", "logs", "kaboom.jsonl")) {
		t.Fatalf("FallbackFilePath() = %q, want a kaboom/logs/kaboom.jsonl path", got)
	}
}

// TestStoreSetLogFileRedirectsPersistence pins the startup fallback contract:
// NewServer replaces the configured path when the directory is unwritable, and
// every later append must land in the replacement, not the original.
func TestStoreSetLogFileRedirectsPersistence(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.jsonl")
	fallback := filepath.Join(dir, "fallback.jsonl")

	ls := New(Config{LogFile: original, MaxEntries: 10, AddWarning: func(string) {}})
	ls.SetLogFile(fallback)
	if got := ls.LogFile(); got != fallback {
		t.Fatalf("LogFile() = %q, want %q", got, fallback)
	}
	go ls.RunWorker()

	ls.AddEntries([]types.LogEntry{{"level": "info", "message": "redirected"}})
	ls.Shutdown(2 * time.Second)

	data, err := os.ReadFile(fallback)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", fallback, err)
	}
	if !strings.Contains(string(data), "redirected") {
		t.Fatalf("fallback file missing the entry: %q", string(data))
	}
	if _, err := os.Stat(original); !os.IsNotExist(err) {
		t.Fatalf("original path %q was written after SetLogFile", original)
	}
}

// TestStoreSeedEntriesBypassesIngest pins the test-support contract that
// cmd/browser-agent's data-age and audit tests depend on: SeedEntries appends
// to the window (and add-times) WITHOUT touching the counters, the trim, the
// file queue, or the onEntries callback. If seeding ever started routing
// through AddEntries, those tests would silently stop testing backdated data.
func TestStoreSeedEntriesBypassesIngest(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "seed.jsonl")
	ls := New(Config{LogFile: logFile, MaxEntries: 1, AddWarning: func(string) {}})

	var callbacks atomic.Int32
	ls.SetOnEntries(func([]types.LogEntry) { callbacks.Add(1) })

	backdated := time.Now().Add(-time.Hour)
	ls.SeedEntries([]types.LogEntry{
		{"level": "error", "message": "seeded-1"},
		{"level": "error", "message": "seeded-2"},
	}, []time.Time{backdated, backdated})

	// No trimming to MaxEntries=1, no callback, no counters.
	if got := ls.EntryCount(); got != 2 {
		t.Fatalf("EntryCount() = %d, want 2 (seeding must not trim to MaxEntries)", got)
	}
	if got := callbacks.Load(); got != 0 {
		t.Fatalf("onEntries fired %d times, want 0", got)
	}
	if got := ls.TotalAdded(); got != 0 {
		t.Fatalf("TotalAdded() = %d, want 0 (seeding must not bump counters)", got)
	}
	if got := ls.ErrorTotalAdded(); got != 0 {
		t.Fatalf("ErrorTotalAdded() = %d, want 0", got)
	}
	_, addedAt := ls.EntriesWithAddedAt()
	if len(addedAt) != 2 || !addedAt[0].Equal(backdated) {
		t.Fatalf("add-times = %v, want both %v", addedAt, backdated)
	}

	// nil add-times must leave the parallel slice untouched.
	ls.SeedEntries([]types.LogEntry{{"level": "info", "message": "seeded-3"}}, nil)
	if _, addedAt = ls.EntriesWithAddedAt(); len(addedAt) != 2 {
		t.Fatalf("len(addedAt) = %d after a nil-time seed, want 2", len(addedAt))
	}

	ls.SeedTotalAdded(7)
	if got := ls.TotalAdded(); got != 7 {
		t.Fatalf("TotalAdded() = %d after SeedTotalAdded(7), want 7", got)
	}

	// Nothing was queued for persistence, so the file was never created.
	if _, err := os.Stat(logFile); !os.IsNotExist(err) {
		t.Fatalf("seeding wrote %q; it must not touch the file pipeline", logFile)
	}
}

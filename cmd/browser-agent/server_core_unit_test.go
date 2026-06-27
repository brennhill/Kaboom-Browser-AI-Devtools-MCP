// Purpose: Unit tests for browser-agent server core logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestServerAddEntriesRotationPath(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotation.jsonl")
	srv, err := NewServer(logFile, 2)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	added := srv.logs.addEntries([]LogEntry{
		{"level": "info", "message": "a"},
		{"level": "info", "message": "b"},
		{"level": "info", "message": "c"},
	})
	if added != 3 {
		t.Fatalf("addEntries() = %d, want 3", added)
	}

	entries := srv.logs.getEntries()
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2 after rotation", len(entries))
	}
	if entries[0]["message"] != "b" || entries[1]["message"] != "c" {
		t.Fatalf("rotated entries = %+v, want last two entries", entries)
	}

	srv.logs.shutdownAsyncLogger(2 * time.Second)
	data, err := os.ReadFile(logFile) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logFile, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Persistence is append-only on the hot path: all 3 entries are appended.
	// The file is only compacted down to the in-memory window once it exceeds
	// logCompactionFactor*maxEntries entries (see
	// TestLogStoreCompactionRewritesAfterThreshold); loadEntries bounds reads.
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3 (append-only persistence)", len(lines))
	}
}

func TestServerSetOnEntriesAndAppendPath(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "append.jsonl")
	srv, err := NewServer(logFile, 10)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	var callbackCount atomic.Int32
	srv.SetOnEntries(func(entries []LogEntry) {
		callbackCount.Add(int32(len(entries)))
	})

	added := srv.logs.addEntries([]LogEntry{{"level": "info", "message": "hello"}})
	if added != 1 {
		t.Fatalf("addEntries() = %d, want 1", added)
	}
	srv.logs.shutdownAsyncLogger(2 * time.Second)

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

func TestServerLoadEntriesBoundsAndMalformedLines(t *testing.T) {
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

	srv, err := NewServer(logFile, 2)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.logs.shutdownAsyncLogger(2 * time.Second)

	entries := srv.logs.getEntries()
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0]["message"] != "second" || entries[1]["message"] != "third" {
		t.Fatalf("loaded entries = %+v, want last two valid entries", entries)
	}
}

func TestServerAppendToFileDropAndShutdownTimeout(t *testing.T) {
	srv := &LogStore{
		logChan:    make(chan []LogEntry, 1),
		logDone:    make(chan struct{}),
		addWarning: func(string) {},
	}
	srv.logChan <- []LogEntry{{"level": "info", "message": "queued"}}
	if err := srv.appendToFile([]LogEntry{{"level": "info", "message": "drop"}}); err == nil {
		t.Fatal("appendToFile() expected drop error when channel is full")
	}
	if dropped := atomic.LoadInt64(&srv.logDropCount); dropped != 1 {
		t.Fatalf("logDropCount = %d, want 1", dropped)
	}

	srv.shutdownAsyncLogger(10 * time.Millisecond)
}

func TestServerFileRotationOnSizeExceeded(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-size.jsonl")
	srv, err := NewServer(logFile, 10000)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	// Set a tiny max file size (1KB) to trigger rotation quickly
	srv.logs.maxFileSize = 1024

	// Write enough entries to exceed 1KB (triggers rotation)
	var entries []LogEntry
	for i := 0; i < 50; i++ {
		entries = append(entries, LogEntry{"level": "info", "message": strings.Repeat("x", 100)})
	}
	srv.logs.addEntries(entries)

	// Let async worker process and rotate
	time.Sleep(50 * time.Millisecond)

	// Write a second small batch so a new main file is created after rotation
	srv.logs.addEntries([]LogEntry{{"level": "info", "message": "after-rotation"}})
	srv.logs.shutdownAsyncLogger(2 * time.Second)

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

func TestServerFileRotationCreatesOldFile(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-old.jsonl")
	srv, err := NewServer(logFile, 10000)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	// 512 bytes to trigger on just a handful of entries
	srv.logs.maxFileSize = 512

	// Write entries in two batches to trigger rotation
	batch1 := []LogEntry{
		{"level": "info", "message": strings.Repeat("a", 200)},
		{"level": "info", "message": strings.Repeat("b", 200)},
		{"level": "info", "message": strings.Repeat("c", 200)},
	}
	srv.logs.addEntries(batch1)

	// Let the async logger process
	time.Sleep(50 * time.Millisecond)

	batch2 := []LogEntry{
		{"level": "info", "message": strings.Repeat("d", 200)},
	}
	srv.logs.addEntries(batch2)
	srv.logs.shutdownAsyncLogger(2 * time.Second)

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

func TestServerFileRotationOverwritesExistingOld(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-overwrite.jsonl")
	oldFile := logFile + ".old"

	// Create a pre-existing .old file
	if err := os.WriteFile(oldFile, []byte("stale-old-data\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", oldFile, err)
	}

	srv, err := NewServer(logFile, 10000)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	srv.logs.maxFileSize = 256

	entries := []LogEntry{
		{"level": "info", "message": strings.Repeat("z", 200)},
		{"level": "info", "message": strings.Repeat("y", 200)},
	}
	srv.logs.addEntries(entries)
	srv.logs.shutdownAsyncLogger(2 * time.Second)

	// Old file should be overwritten (no longer contain stale data)
	data, err := os.ReadFile(oldFile) // nosemgrep: go_filesystem_rule-fileread
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", oldFile, err)
	}
	if strings.Contains(string(data), "stale-old-data") {
		t.Fatal("old file should have been overwritten by rotation, still contains stale data")
	}
}

func TestServerFileRotationDefaultMaxFileSize(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-default.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer srv.logs.shutdownAsyncLogger(2 * time.Second)

	// Default maxFileSize should be 50MB
	if srv.logs.maxFileSize != 50*1024*1024 {
		t.Fatalf("default maxFileSize = %d, want %d", srv.logs.maxFileSize, 50*1024*1024)
	}
}

func TestServerFileRotationZeroDisablesRotation(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "rotate-disabled.jsonl")
	srv, err := NewServer(logFile, 10000)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	// Explicitly disable file rotation
	srv.logs.maxFileSize = 0

	entries := []LogEntry{
		{"level": "info", "message": strings.Repeat("x", 200)},
		{"level": "info", "message": strings.Repeat("y", 200)},
	}
	srv.logs.addEntries(entries)
	srv.logs.shutdownAsyncLogger(2 * time.Second)

	// No .old file should exist
	oldFile := logFile + ".old"
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected no .old file when rotation disabled, but it exists")
	}
}

func TestServerGetLogDropCount(t *testing.T) {
	t.Parallel()

	ls := &LogStore{
		logChan:    make(chan []LogEntry, 1),
		logDone:    make(chan struct{}),
		addWarning: func(string) {},
	}

	// Initially zero
	if got := ls.getLogDropCount(); got != 0 {
		t.Fatalf("getLogDropCount() = %d, want 0", got)
	}

	// Fill channel, then trigger a drop
	ls.logChan <- []LogEntry{{"level": "info", "message": "fill"}}
	_ = ls.appendToFile([]LogEntry{{"level": "info", "message": "drop"}})

	if got := ls.getLogDropCount(); got != 1 {
		t.Fatalf("getLogDropCount() = %d, want 1", got)
	}

	// Trigger a second drop
	_ = ls.appendToFile([]LogEntry{{"level": "info", "message": "drop2"}})

	if got := ls.getLogDropCount(); got != 2 {
		t.Fatalf("getLogDropCount() = %d, want 2", got)
	}

	ls.shutdownAsyncLogger(10 * time.Millisecond)
}

func TestServerAppendToFileSyncSkipsUnmarshalableEntry(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "sync.jsonl")
	ls := &LogStore{logFile: logFile, addWarning: func(string) {}}
	err := ls.appendToFileSync([]LogEntry{
		{"level": "info", "message": "ok"},
		{"level": "info", "value": math.NaN()},
	})
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

func TestNewServer_FallbacksWhenLogDirUnwritable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(dir, 0o500); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	logFile := filepath.Join(dir, "kaboom.jsonl")

	srv, err := NewServer(logFile, 10)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if srv.logs.logFile == logFile {
		t.Fatalf("expected fallback log file, got original path %q", srv.logs.logFile)
	}
	warnings := srv.TakeWarnings()
	if len(warnings) == 0 {
		t.Fatal("expected warning about unwritable log directory")
	}
}

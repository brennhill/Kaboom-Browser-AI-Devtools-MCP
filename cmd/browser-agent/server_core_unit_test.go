// Purpose: Unit tests for browser-agent server core logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if srv.logs.LogFile() == logFile {
		t.Fatalf("expected fallback log file, got original path %q", srv.logs.LogFile())
	}
	warnings := srv.TakeWarnings()
	if len(warnings) == 0 {
		t.Fatal("expected warning about unwritable log directory")
	}
}

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

func TestServerWarningsDeduplicateAndDrainOnce(t *testing.T) {
	server := &Server{}
	server.AddWarning("")
	server.AddWarning("disk unavailable")
	server.AddWarning("disk unavailable")
	server.AddWarning("telemetry disabled")

	warnings := server.TakeWarnings()
	if len(warnings) != 2 {
		t.Fatalf("expected two unique warnings, got %#v", warnings)
	}
	if warnings[0] != "disk unavailable" || warnings[1] != "telemetry disabled" {
		t.Fatalf("warnings lost insertion order: %#v", warnings)
	}
	if secondDrain := server.TakeWarnings(); secondDrain != nil {
		t.Fatalf("expected warnings to drain once, got %#v", secondDrain)
	}
}

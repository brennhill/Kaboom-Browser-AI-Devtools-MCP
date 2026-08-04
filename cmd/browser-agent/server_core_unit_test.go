// Purpose: Unit tests for browser-agent server core logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestConfigPathsInitializeLocalStateAndUploadBoundary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(state.StateDirEnv, "")
	security := initUploadSecurity(false, "", multiFlag{"private-*"})
	if security == nil {
		t.Fatal("default upload security was not initialized")
	}
	if _, err := os.Stat(filepath.Join(home, "kaboom-upload-dir")); err != nil {
		t.Fatalf("default upload directory: %v", err)
	}

	relative := filepath.Join(".", "test-state")
	normalizeStateDir(&relative)
	if !filepath.IsAbs(relative) || os.Getenv(state.StateDirEnv) != relative {
		t.Fatalf("normalized state directory = %q env=%q", relative, os.Getenv(state.StateDirEnv))
	}

	explicit := filepath.Join(t.TempDir(), "explicit")
	var warnings []string
	if err := applyParallelModeStateDir(true, &explicit, &warnings); err != nil || explicit == "" {
		t.Fatalf("explicit parallel state directory = %q, err=%v", explicit, err)
	}
	logFile := ""
	resolveDefaultLogFile(&logFile, &warnings)
	if logFile == "" || !strings.HasSuffix(logFile, "kaboom.jsonl") {
		t.Fatalf("default log file = %q", logFile)
	}
}

func TestServerRuntimeConfigurationIsInstanceOwned(t *testing.T) {
	first := &Server{}
	second := &Server{}
	first.applyRuntimeConfig(&serverConfig{uploadAutomation: true})
	second.applyRuntimeConfig(&serverConfig{uploadAutomation: false})

	if !first.uploadAutomation || second.uploadAutomation {
		t.Fatal("upload automation configuration leaked between server instances")
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

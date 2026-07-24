// Purpose: Tests for exit diagnostic output on shutdown.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestWriteDiagnosticToCandidates_WritesFirstAvailable(t *testing.T) {
	t.Parallel()

	dir1 := t.TempDir()
	dir2 := t.TempDir()
	first := filepath.Join(dir1, "logs", "crash.log")
	second := filepath.Join(dir2, "logs", "crash.log")

	path, err := writeDiagnosticToCandidates(
		[]string{first, second},
		map[string]any{"event": "daemon_shutdown", "reason": "test"},
	)
	if err != nil {
		t.Fatalf("writeDiagnosticToCandidates error: %v", err)
	}
	if path != first {
		t.Fatalf("write path = %q, want %q", path, first)
	}

	data, err := os.ReadFile(first) // nosemgrep: go_filesystem_rule-fileread -- unit test reads temp file output
	if err != nil {
		t.Fatalf("read first candidate: %v", err)
	}
	if !strings.Contains(string(data), `"event":"daemon_shutdown"`) {
		t.Fatalf("expected event in diagnostic entry, got: %s", string(data))
	}
}

func TestWriteDiagnosticToCandidates_FallsBackOnInvalidPath(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	bad := filepath.Join(blocker, "logs", "crash.log")
	good := filepath.Join(base, "ok", "crash.log")

	path, err := writeDiagnosticToCandidates(
		[]string{bad, good},
		map[string]any{"event": "daemon_shutdown", "reason": "fallback"},
	)
	if err != nil {
		t.Fatalf("writeDiagnosticToCandidates error: %v", err)
	}
	if path != good {
		t.Fatalf("fallback path = %q, want %q", path, good)
	}
}

func TestAppendExitDiagnostic_UsesStateCrashPath(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(state.StateDirEnv, stateDir)

	path := appendExitDiagnostic("daemon_shutdown", map[string]any{"reason": "unit_test"})
	if path == "" {
		t.Fatal("appendExitDiagnostic returned empty path")
	}
	want := filepath.Join(stateDir, "logs", "exit-diagnostics.log")
	if path != want {
		t.Fatalf("append path = %q, want %q", path, want)
	}

	data, err := os.ReadFile(path) // nosemgrep: go_filesystem_rule-fileread -- unit test reads temp file output
	if err != nil {
		t.Fatalf("read crash diagnostic: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"event":"daemon_shutdown"`) {
		t.Fatalf("missing daemon_shutdown event: %s", text)
	}
	if !strings.Contains(text, `"reason":"unit_test"`) {
		t.Fatalf("missing reason field: %s", text)
	}
}

// TestBridgeShutdown_WritesBridgeExitDiagnostics moved to cmd/browser-agent/internal/bridge/

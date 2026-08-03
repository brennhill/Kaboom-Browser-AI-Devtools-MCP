// Purpose: Tests for exit diagnostic output on shutdown.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package exitdiag

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestRecorderRecoverUsesInjectedExitAndStderr(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	var stderr bytes.Buffer
	exitCode := 0
	recorder := New(Options{
		Version: "test", Stderr: &stderr, Exit: func(code int) { exitCode = code },
	})

	recorder.Recover("boom")

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "FATAL ERROR") || !strings.Contains(stderr.String(), "Crash details written") {
		t.Fatalf("stderr missing panic diagnostics: %q", stderr.String())
	}
}

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

func TestWriteDiagnosticToCandidatesClassifiesUnavailableDestinations(t *testing.T) {
	if _, err := writeDiagnosticToCandidates(nil, map[string]any{"event": "test"}); err == nil || !strings.Contains(err.Error(), "no crash-log candidates") {
		t.Fatalf("empty candidates error = %v", err)
	}
	if _, err := writeDiagnosticToCandidates([]string{""}, map[string]any{"event": "test"}); err == nil || !strings.Contains(err.Error(), "no writable") {
		t.Fatalf("blank candidate error = %v", err)
	}
	if _, err := writeDiagnosticToCandidates([]string{filepath.Join(t.TempDir(), "unused")}, map[string]any{"invalid": make(chan int)}); err == nil {
		t.Fatal("unencodable diagnostic entry was accepted")
	}
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := writeDiagnosticToCandidates([]string{filepath.Join(blocker, "crash.log")}, map[string]any{"event": "test"}); err == nil {
		t.Fatal("unwritable diagnostic destination was accepted")
	}
}

func TestAppendExitDiagnostic_UsesStateCrashPath(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(state.StateDirEnv, stateDir)

	path := New(Options{Version: "test"}).Append("daemon_shutdown", map[string]any{"reason": "unit_test"})
	if path == "" {
		t.Fatal("Append returned empty path")
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

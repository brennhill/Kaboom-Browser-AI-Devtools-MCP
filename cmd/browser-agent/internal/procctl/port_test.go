// Purpose: Tests for platform-specific error message formatting.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package procctl

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPortKillHint(t *testing.T) {
	t.Parallel()

	hint := PortKillHint(7890)
	if hint == "" {
		t.Fatal("PortKillHint returned empty string")
	}

	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(hint, "netstat") {
			t.Errorf("Windows hint should contain netstat, got: %s", hint)
		}
		if !strings.Contains(hint, "taskkill") {
			t.Errorf("Windows hint should contain taskkill, got: %s", hint)
		}
		if !strings.Contains(hint, "7890") {
			t.Errorf("Windows hint should contain port number, got: %s", hint)
		}
	default:
		if !strings.Contains(hint, "lsof") {
			t.Errorf("Unix hint should contain lsof, got: %s", hint)
		}
		if !strings.Contains(hint, "7890") {
			t.Errorf("Unix hint should contain port number, got: %s", hint)
		}
	}
}

func TestPortKillHintForce(t *testing.T) {
	t.Parallel()

	hint := PortKillHintForce(7890)
	if hint == "" {
		t.Fatal("PortKillHintForce returned empty string")
	}

	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(hint, "taskkill") {
			t.Errorf("Windows force hint should contain taskkill, got: %s", hint)
		}
	default:
		if !strings.Contains(hint, "kill -9") {
			t.Errorf("Unix force hint should contain kill -9, got: %s", hint)
		}
		if !strings.Contains(hint, "7890") {
			t.Errorf("Unix force hint should contain port number, got: %s", hint)
		}
	}
}

func TestFindProcessOnPort(t *testing.T) {
	t.Parallel()

	// FindProcessOnPort should not panic on any platform
	pids, err := FindProcessOnPort(0)
	// Port 0 is unlikely to have a process; we just verify no panic
	if err != nil {
		t.Logf("FindProcessOnPort(0) returned error (expected): %v", err)
	}
	_ = pids // may be empty
}

func TestGetProcessCommand(t *testing.T) {
	t.Parallel()

	// GetProcessCommand should not panic for an invalid PID
	cmd := GetProcessCommand(999999)
	// Should return empty or some value, but not panic
	_ = cmd
}

func TestKillProcessByPID(t *testing.T) {
	t.Parallel()

	// KillProcessByPID should not panic for an invalid PID
	// It should gracefully handle the error
	err := KillProcessByPID(999999)
	// We expect an error since process doesn't exist
	if err == nil {
		t.Log("KillProcessByPID(999999) returned nil (process may exist)")
	}
}

func TestFindProcessOnPortUsesListenFilterUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only lsof behavior")
	}

	fakeBin := t.TempDir()
	lsofPath := filepath.Join(fakeBin, "lsof")
	script := `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "-sTCP:LISTEN" ]; then
    echo "43210"
    exit 0
  fi
done
exit 1
`
	if err := os.WriteFile(lsofPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", lsofPath, err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	pids, err := FindProcessOnPort(7890)
	if err != nil {
		t.Fatalf("FindProcessOnPort() error = %v", err)
	}
	if len(pids) != 1 || pids[0] != 43210 {
		t.Fatalf("FindProcessOnPort() = %v, want [43210]", pids)
	}
}

// TestGetProcessCommand_LiveProcess pins the success path that the daemon's
// port-conflict diagnostics depend on: for a PID that really exists, the OS
// query must come back with a non-empty command line. The error path (unknown
// PID) is covered by TestGetProcessCommand.
func TestGetProcessCommand_LiveProcess(t *testing.T) {
	t.Parallel()

	got := GetProcessCommand(os.Getpid())
	if strings.TrimSpace(got) == "" {
		t.Fatalf("GetProcessCommand(self) = %q, want the running process's command line", got)
	}
	// The test binary's own command line must mention it — this is what makes the
	// "port N is held by <command>" diagnostic actionable rather than blank.
	if !strings.Contains(got, ".test") && !strings.Contains(got, "procctl") {
		t.Fatalf("GetProcessCommand(self) = %q, want it to name the running test binary", got)
	}
}

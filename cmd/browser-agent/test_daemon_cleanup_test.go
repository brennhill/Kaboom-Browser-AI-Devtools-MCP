// Purpose: Tests for test daemon cleanup and resource release.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// TestMain enforces process hygiene for the full cmd/browser-agent test suite.
// Some tests spawn client processes that in turn spawn detached daemons.
// We run cleanup before and after tests to prevent stale daemons from accumulating.
func TestMain(m *testing.M) {
	initBridge()
	cleanupGoTestDaemons()
	code := m.Run()
	cleanupGoTestDaemons()
	os.Exit(code)
}

func cleanupGoTestDaemons() {
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/F", "/IM", "kaboom-test-binary.exe").Run()
		return
	}

	killPattern("kaboom-test-binary --daemon --port")
	killPattern("kaboom-test-binary --port")

	// Clean known test PID file ranges used by shell and regression tests.
	cleanupPIDFiles()
	for port := 17890; port <= 17999; port++ {
		procctl.RemovePIDFile(port)
	}
}

func killPattern(pattern string) {
	_ = exec.Command("pkill", "-TERM", "-f", pattern).Run()
	time.Sleep(200 * time.Millisecond)
	_ = exec.Command("pkill", "-KILL", "-f", pattern).Run()
}

func TestCleanupPIDFilesRemovesKaboomAndKaboomPIDVariants(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	paths := []string{
		filepath.Join(stateRoot, "run", "kaboom-7890.pid"),
		filepath.Join(stateRoot, "run", "strum-7890.pid"),
		filepath.Join(home, ".kaboom", "run", "kaboom-7890.pid"),
		filepath.Join(home, ".strum", "run", "strum-7890.pid"),
		filepath.Join(home, ".kaboom-7890.pid"),
		filepath.Join(home, ".strum-7890.pid"),
	}

	for _, pidPath := range paths {
		if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(pidPath), err)
		}
		if err := os.WriteFile(pidPath, []byte("12345"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", pidPath, err)
		}
	}

	cleanupPIDFiles()

	for _, pidPath := range paths {
		if _, err := os.Stat(pidPath); err == nil {
			t.Fatalf("expected cleanupPIDFiles to remove %q", pidPath)
		}
	}
}

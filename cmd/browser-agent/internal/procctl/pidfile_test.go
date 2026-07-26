// pidfile_test.go — PID file lifecycle, legacy-path fallback and process
// liveness checks. Moved here with the code they exercise.

package procctl

import (
	"os"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestIsProcessAlive(t *testing.T) {
	t.Parallel()

	t.Run("zero pid", func(t *testing.T) {
		if IsProcessAlive(0) {
			t.Fatal("pid=0 should not be alive")
		}
	})

	t.Run("negative pid", func(t *testing.T) {
		if IsProcessAlive(-1) {
			t.Fatal("pid=-1 should not be alive")
		}
	})

	t.Run("nonexistent pid", func(t *testing.T) {
		// PID 999999 is almost certainly not running
		if IsProcessAlive(999999) {
			t.Skip("PID 999999 is somehow alive; skipping")
		}
	})
}

func TestPidFilePath(t *testing.T) {
	t.Parallel()

	path := PIDFilePath(3000)
	if path == "" {
		t.Fatal("PIDFilePath(3000) should return a non-empty path")
	}
}

func TestLegacyPIDFilePath(t *testing.T) {
	t.Parallel()

	path := LegacyPIDFilePath(3000)
	if path == "" {
		t.Fatal("LegacyPIDFilePath(3000) should return a non-empty path")
	}
}

func TestPIDFileLifecycleAndLegacyFallback(t *testing.T) {
	// Do not run in parallel; uses Setenv.
	stateRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	const port = 56789
	if err := WritePIDFile(port); err != nil {
		t.Fatalf("WritePIDFile(%d) error = %v", port, err)
	}
	if got := ReadPIDFile(port); got != os.Getpid() {
		t.Fatalf("ReadPIDFile(%d) = %d, want current pid %d", port, got, os.Getpid())
	}
	RemovePIDFile(port)
	if got := ReadPIDFile(port); got != 0 {
		t.Fatalf("ReadPIDFile(%d) after remove = %d, want 0", port, got)
	}

	legacyPath, err := state.LegacyPIDFile(43210)
	if err != nil {
		t.Fatalf("LegacyPIDFile() error = %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("12345"), 0o600); err != nil {
		t.Fatalf("WriteFile(legacy pid) error = %v", err)
	}
	if got := ReadPIDFile(43210); got != 12345 {
		t.Fatalf("ReadPIDFile(legacy) = %d, want 12345", got)
	}
}

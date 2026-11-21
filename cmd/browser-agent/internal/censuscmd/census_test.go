// census_test.go — Tests for the --instances census and --reap operator commands.

package censuscmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// isolate points the registry and state root at fresh temp directories. These
// tests mutate process environment, so they must not run in parallel.
func isolate(t *testing.T) (stateRoot string) {
	t.Helper()
	t.Setenv(instancereg.DirEnv, t.TempDir())
	stateRoot = t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	return stateRoot
}

func staleRunDir(t *testing.T, stateRoot string) string {
	t.Helper()
	dir := filepath.Join(stateRoot, "parallel", "run-123-456")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * staleParallelDirMaxAge)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCensusEmptyRegistrySucceeds(t *testing.T) {
	isolate(t)
	if code := Census(); code != 0 {
		t.Fatalf("Census() = %d, want 0", code)
	}
}

func TestCensusAndReapFailLoudWhenRegistryUnavailable(t *testing.T) {
	// Under go test, instancereg refuses the real user registry without DirEnv.
	t.Setenv(instancereg.DirEnv, "")
	t.Setenv(state.StateDirEnv, t.TempDir())
	if code := Census(); code != 1 {
		t.Fatalf("Census() = %d, want 1 when the registry cannot be read", code)
	}
	if code := Reap(true); code != 1 {
		t.Fatalf("Reap() = %d, want 1 when the registry cannot be read", code)
	}
}

func TestReapDryRunReportsButKeepsAbandonedRunDir(t *testing.T) {
	dir := staleRunDir(t, isolate(t))
	if code := Reap(true); code != 0 {
		t.Fatalf("Reap(dryRun) = %d, want 0", code)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("dry run removed %s: %v", dir, err)
	}
}

func TestReapRemovesAbandonedRunDirButKeepsFreshOne(t *testing.T) {
	stateRoot := isolate(t)
	stale := staleRunDir(t, stateRoot)
	fresh := filepath.Join(stateRoot, "parallel", "run-789-1")
	if err := os.MkdirAll(fresh, 0o700); err != nil {
		t.Fatal(err)
	}
	if code := Reap(false); code != 0 {
		t.Fatalf("Reap() = %d, want 0", code)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("abandoned run dir still present (err=%v)", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh run dir removed: %v", err)
	}
}

func TestSweepReportsUnreadableParallelRoot(t *testing.T) {
	stateRoot := isolate(t)
	// A file where the parallel directory belongs makes ReadDir fail.
	if err := os.WriteFile(filepath.Join(stateRoot, "parallel"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if removed := sweepAbandonedParallelDirs(nil, time.Now(), false); removed != 0 {
		t.Fatalf("sweep removed = %d, want 0", removed)
	}
}

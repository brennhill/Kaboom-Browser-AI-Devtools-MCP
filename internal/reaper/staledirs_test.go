// staledirs_test.go — Proves abandoned --parallel state directories are reclaimed
// and that a directory belonging to a live run never is.

package reaper_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/reaper"
)

func makeRunDir(t *testing.T, root, name string, age time.Duration, now time.Time) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(path, "run"), 0o750); err != nil {
		t.Fatal(err)
	}
	stamp := now.Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return path
}

// 880 of these accumulated on one developer machine over a month, because nothing
// ever removed one.
func TestStaleParallelDirsAreRemoved(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	old := makeRunDir(t, root, "run-1785289891202396000-43950", 48*time.Hour, now)
	recent := makeRunDir(t, root, "run-1785289891206254000-43948", time.Minute, now)

	result, err := reaper.SweepParallelDirs(root, nil, 24*time.Hour, now, false)
	if err != nil {
		t.Fatalf("SweepParallelDirs() error = %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("removed %d dirs, want 1", result.Removed)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("the abandoned run directory survived")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Error("a recent run directory was removed")
	}
}

// A directory whose daemon is still registered must never be removed, however old
// it looks: a long test run is not an abandoned one.
func TestLiveParallelDirIsNeverRemoved(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	live := makeRunDir(t, root, "run-1785289891202396000-43950", 90*24*time.Hour, now)

	result, err := reaper.SweepParallelDirs(root,
		[]instancereg.Record{{PID: 43950, StateDir: live, Parallel: true}},
		24*time.Hour, now, false)
	if err != nil {
		t.Fatalf("SweepParallelDirs() error = %v", err)
	}
	if result.Removed != 0 {
		t.Fatalf("removed %d dirs, want 0 (a live run owns it)", result.Removed)
	}
	if _, err := os.Stat(live); err != nil {
		t.Error("a live run's state directory was removed")
	}
}

func TestDryRunReportsWithoutRemoving(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	old := makeRunDir(t, root, "run-1-1", 48*time.Hour, now)

	result, err := reaper.SweepParallelDirs(root, nil, 24*time.Hour, now, true)
	if err != nil {
		t.Fatalf("SweepParallelDirs() error = %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("dry run reported %d, want 1 planned removal", result.Removed)
	}
	if _, err := os.Stat(old); err != nil {
		t.Error("dry run actually removed a directory")
	}
}

// Only generated run directories are eligible. A user's own --state-dir must never
// be swept, so anything not matching the generated name shape is left alone.
func TestOnlyGeneratedRunDirsAreEligible(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	mine := makeRunDir(t, root, "my-important-state", 90*24*time.Hour, now)

	result, err := reaper.SweepParallelDirs(root, nil, time.Hour, now, false)
	if err != nil {
		t.Fatalf("SweepParallelDirs() error = %v", err)
	}
	if result.Removed != 0 {
		t.Fatalf("removed %d, want 0 (name does not match the generated shape)", result.Removed)
	}
	if _, err := os.Stat(mine); err != nil {
		t.Error("a directory that is not a generated run dir was removed")
	}
}

func TestMissingParallelRootIsNotAnError(t *testing.T) {
	result, err := reaper.SweepParallelDirs(filepath.Join(t.TempDir(), "absent"), nil, time.Hour, time.Now(), false)
	if err != nil {
		t.Fatalf("SweepParallelDirs() error = %v, want nil", err)
	}
	if result.Removed != 0 {
		t.Fatalf("removed %d from a missing root", result.Removed)
	}
}

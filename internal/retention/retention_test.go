// retention_test.go — Proves capture directories stay inside a budget and that
// eviction is oldest-first, single-pass, and never removes what it must keep.

package retention_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/retention"
)

func writeFile(t *testing.T, dir, name string, size int, age time.Duration, now time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := now.Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnforceRemovesOldestUntilFileCountFits(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for i, age := range []time.Duration{5 * time.Hour, 4 * time.Hour, 3 * time.Hour, 2 * time.Hour, time.Hour} {
		writeFile(t, dir, string(rune('a'+i))+".png", 10, age, now)
	}

	result, err := retention.Enforce(dir, retention.Budget{MaxFiles: 2}, now)
	if err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	if result.RemovedFiles != 3 {
		t.Fatalf("Enforce() removed %d files, want 3", result.RemovedFiles)
	}
	remaining, _ := os.ReadDir(dir)
	if len(remaining) != 2 {
		t.Fatalf("%d files remain, want 2", len(remaining))
	}
	// The two newest must survive.
	for _, entry := range remaining {
		if entry.Name() != "d.png" && entry.Name() != "e.png" {
			t.Errorf("kept %s, want only the two newest (d.png, e.png)", entry.Name())
		}
	}
}

func TestEnforceRemovesOldestUntilByteBudgetFits(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeFile(t, dir, "old.bin", 1000, 3*time.Hour, now)
	writeFile(t, dir, "mid.bin", 1000, 2*time.Hour, now)
	writeFile(t, dir, "new.bin", 1000, time.Hour, now)

	result, err := retention.Enforce(dir, retention.Budget{MaxBytes: 2500}, now)
	if err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	if result.RemovedFiles != 1 || result.RemovedBytes != 1000 {
		t.Fatalf("Enforce() removed %d files / %d bytes, want 1 / 1000",
			result.RemovedFiles, result.RemovedBytes)
	}
	if _, err := os.Stat(filepath.Join(dir, "old.bin")); !os.IsNotExist(err) {
		t.Error("the oldest file survived the byte budget")
	}
}

func TestEnforceRemovesFilesPastMaxAge(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeFile(t, dir, "ancient.json", 10, 90*24*time.Hour, now)
	writeFile(t, dir, "recent.json", 10, time.Hour, now)

	result, err := retention.Enforce(dir, retention.Budget{MaxAge: 7 * 24 * time.Hour}, now)
	if err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	if result.RemovedFiles != 1 {
		t.Fatalf("Enforce() removed %d files, want 1", result.RemovedFiles)
	}
	if _, err := os.Stat(filepath.Join(dir, "recent.json")); err != nil {
		t.Error("Enforce() removed a file inside the age budget")
	}
}

// An empty budget must be a no-op. A zero value meaning "delete everything" would
// turn a missing config into data loss.
func TestZeroBudgetRemovesNothing(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeFile(t, dir, "keep.png", 10, 400*24*time.Hour, now)

	result, err := retention.Enforce(dir, retention.Budget{}, now)
	if err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	if result.RemovedFiles != 0 {
		t.Fatalf("Enforce() removed %d files under a zero budget, want 0", result.RemovedFiles)
	}
}

func TestMissingDirectoryIsNotAnError(t *testing.T) {
	result, err := retention.Enforce(filepath.Join(t.TempDir(), "absent"), retention.Budget{MaxFiles: 1}, time.Now())
	if err != nil {
		t.Fatalf("Enforce() on a missing directory error = %v, want nil", err)
	}
	if result.RemovedFiles != 0 {
		t.Fatalf("Enforce() removed %d files from a missing directory", result.RemovedFiles)
	}
}

func TestSubdirectoriesAreNotRemoved(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.png", 10, 5*time.Hour, now)
	writeFile(t, dir, "b.png", 10, time.Hour, now)

	if _, err := retention.Enforce(dir, retention.Budget{MaxFiles: 1}, now); err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nested")); err != nil {
		t.Error("Enforce() removed a subdirectory; it must only evict files")
	}
}

// All three limits apply together, and one pass must satisfy every one of them.
func TestAllBudgetsApplyInOnePass(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeFile(t, dir, "a.bin", 1000, 30*24*time.Hour, now) // over age
	writeFile(t, dir, "b.bin", 1000, 3*time.Hour, now)
	writeFile(t, dir, "c.bin", 1000, 2*time.Hour, now)
	writeFile(t, dir, "d.bin", 1000, time.Hour, now)

	result, err := retention.Enforce(dir, retention.Budget{
		MaxFiles: 3, MaxBytes: 2000, MaxAge: 7 * 24 * time.Hour,
	}, now)
	if err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	if result.RemovedFiles != 2 {
		t.Fatalf("Enforce() removed %d files, want 2 (age evicts a, bytes evict b)", result.RemovedFiles)
	}
	remaining, _ := os.ReadDir(dir)
	if len(remaining) != 2 {
		t.Fatalf("%d files remain, want 2", len(remaining))
	}
}

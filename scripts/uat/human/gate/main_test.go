// main_test.go — Proves the gate picks the run that describes the build in hand,
// and refuses when there is none.

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeRun(t *testing.T, dir, name string, modified time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheNewestRunIsTheOneLastWrittenTo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A log is appended to across sittings, so the file last written to is the
	// one that describes the build in hand — whatever date is in its name.
	writeRun(t, dir, "2026-09-05.jsonl", time.Now().Add(-48*time.Hour))
	want := writeRun(t, dir, "2026-01-01.jsonl", time.Now())

	got, err := newestRun(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("newestRun = %s, want %s — picking by filename would judge the build against a stale run", got, want)
	}
}

func TestNoRunAtAllIsRefused(t *testing.T) {
	t.Parallel()
	// Silence here would let a release proceed with nobody having looked at it.
	if _, err := newestRun(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("a missing run directory was accepted")
	}
	if _, err := newestRun(t.TempDir()); err == nil {
		t.Fatal("an empty run directory was accepted")
	}

	// Control: a directory holding a run does resolve, or the two assertions
	// above would hold for a function that always failed.
	dir := t.TempDir()
	writeRun(t, dir, "2026-09-05.jsonl", time.Now())
	if _, err := newestRun(dir); err != nil {
		t.Fatalf("a real run directory was refused: %v", err)
	}
}

func TestNonRunFilesAreIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeRun(t, dir, "notes.txt", time.Now())
	if _, err := newestRun(dir); err == nil {
		t.Error("a directory holding no run log resolved to one anyway")
	}
	// The evidence directory sits beside the logs and must not be mistaken for one.
	if err := os.Mkdir(filepath.Join(dir, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := newestRun(dir); err == nil {
		t.Error("the evidence directory was taken for a run log")
	}
}

// Purpose: Additional tests for state save/load path coverage.
// Docs: docs/features/feature/state-time-travel/index.md

package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootDirFallsBackToHomeDotKaboom(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}

	want := filepath.Join(home, ".kaboom")
	if got != want {
		t.Fatalf("RootDir() = %q, want %q", got, want)
	}
}
func TestProjectDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")

	got, err := ProjectDir("/Users/brenn/dev/myproject")
	if err != nil {
		t.Fatalf("ProjectDir() error = %v", err)
	}
	want := filepath.Join(root, "projects", "Users", "brenn", "dev", "myproject")
	if got != want {
		t.Fatalf("ProjectDir() = %q, want %q", got, want)
	}
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	if _, err := normalizePath(""); err == nil {
		t.Fatal("normalizePath(\"\") should return error")
	}

	absInput := filepath.Join(string(os.PathSeparator), "tmp", "a", "..", "b")
	absGot, err := normalizePath(absInput)
	if err != nil {
		t.Fatalf("normalizePath(abs) error = %v", err)
	}
	if absGot != filepath.Clean(absInput) {
		t.Fatalf("normalizePath(abs) = %q, want %q", absGot, filepath.Clean(absInput))
	}

	relGot, err := normalizePath(filepath.Join(".", "x", "..", "y"))
	if err != nil {
		t.Fatalf("normalizePath(rel) error = %v", err)
	}
	if !filepath.IsAbs(relGot) {
		t.Fatalf("normalizePath(rel) = %q, want absolute path", relGot)
	}
}

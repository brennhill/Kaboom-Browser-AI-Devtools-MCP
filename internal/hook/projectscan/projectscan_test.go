// projectscan_test.go — Proves the walk prunes dependencies and reads real source.

package projectscan

import (
	"os"
	"path/filepath"
	"testing"
)

// entries reads a directory once so tests can hand real os.DirEntry values in.
func entries(t *testing.T, dir string) map[string]os.DirEntry {
	t.Helper()
	read, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]os.DirEntry{}
	for _, entry := range read {
		byName[entry.Name()] = entry
	}
	return byName
}

func TestDirectoryDecisionPrunesWhatIsNotOurSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"node_modules", "dist", ".git", ".hidden", "internal"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	byName := entries(t, root)

	for _, name := range []string{"node_modules", "dist", ".git", ".hidden"} {
		decision, handled := DirectoryDecision(byName[name])
		if !handled || decision != filepath.SkipDir {
			t.Errorf("%s: walked into a directory that is not this project's source; its patterns would be reported as the project's conventions", name)
		}
	}

	// Control: an ordinary source directory is walked, or the scanner would find
	// nothing at all and every convention answer would be empty.
	if decision, handled := DirectoryDecision(byName["internal"]); !handled || decision != nil {
		t.Errorf("internal/: decision=%v handled=%v; a normal directory must be walked", decision, handled)
	}
	// A file is not the walk's decision to make.
	if _, handled := DirectoryDecision(byName["main.go"]); handled {
		t.Error("main.go was pruned as a directory, so no file would ever be read")
	}
}

func TestTooLargeSkipsBundlesAndKeepsSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "small.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bundle.js"), make([]byte, MaxFileSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	byName := entries(t, root)

	if !TooLarge(byName["bundle.js"]) {
		t.Error("a file past the size cap was accepted; one minified bundle can outweigh the entire rest of the walk")
	}
	if TooLarge(byName["small.go"]) {
		t.Error("ordinary source was rejected as too large, so the scanner would read nothing")
	}
}

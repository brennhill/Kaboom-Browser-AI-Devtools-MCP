// boundary_test.go — Prevents deterministic fault fixtures from entering production binaries.
package statefault

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStateFaultPackageIsImportedOnlyByTests(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve statefault package path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(repositoryRoot, root), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return walkErr
			}
			data, err := os.ReadFile(path) // #nosec G304 -- test scans repository-owned Go source only.
			if err != nil {
				return err
			}
			if strings.Contains(string(data), `internal/statefault`) {
				t.Errorf("production source imports deterministic fault fixtures: %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

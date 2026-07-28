// no_facade_test.go — Guards query dispatch against compatibility-only APIs.

package queries

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryDispatcherHasNoCompatibilityFacades(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var source strings.Builder
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		source.Write(contents)
	}
	for _, forbidden := range []string{
		"CompleteCommand(",
	} {
		if strings.Contains(source.String(), forbidden) {
			t.Errorf("queries retains compatibility surface %q", forbidden)
		}
	}
}

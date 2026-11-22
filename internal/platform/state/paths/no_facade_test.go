// no_facade_test.go — Guards state paths against compatibility-only APIs.

package state

import (
	"os"
	"strings"
	"testing"
)

func TestStatePathsHaveNoLegacyAPIs(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("paths.go")
	if err != nil {
		t.Fatalf("read paths.go: %v", err)
	}
	if strings.Contains(string(source), "func Legacy") {
		t.Fatal("state paths retain Legacy* compatibility APIs")
	}
}

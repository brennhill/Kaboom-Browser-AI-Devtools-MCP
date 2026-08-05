// no_facade_test.go — Guards the performance package against alias-only compatibility surfaces.

package performance

import (
	"os"
	"testing"
)

func TestPerformancePackageHasNoTypeAliasFacade(t *testing.T) {
	if _, err := os.Stat("type_aliases.go"); !os.IsNotExist(err) {
		t.Fatalf("type_aliases.go compatibility facade must not exist (stat error: %v)", err)
	}
}

func TestPerformancePackageRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("internal/performance has %d files; want at most 10 change-coupled owners", files)
	}
}

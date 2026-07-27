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

// no_facade_test.go — Guards the redaction package against alias-only compatibility surfaces.

package redaction

import (
	"os"
	"testing"
)

func TestRedactionPackageHasNoTypeAliasFacade(t *testing.T) {
	if _, err := os.Stat("type_aliases.go"); !os.IsNotExist(err) {
		t.Fatalf("type_aliases.go compatibility facade must not exist (stat error: %v)", err)
	}
}

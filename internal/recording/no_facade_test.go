// no_facade_test.go — Guards the recording package against alias-only compatibility surfaces.

package recording

import (
	"os"
	"testing"
)

func TestRecordingPackageHasNoTypeAliasFacade(t *testing.T) {
	if _, err := os.Stat("type_aliases.go"); !os.IsNotExist(err) {
		t.Fatalf("type_aliases.go compatibility facade must not exist (stat error: %v)", err)
	}
}

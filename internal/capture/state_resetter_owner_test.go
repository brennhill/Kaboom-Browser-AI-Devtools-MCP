// state_resetter_owner_test.go — Structural ownership tests for global capture reset.

package capture

import (
	"reflect"
	"testing"
)

func TestStateResetterOwnsClearAllWithoutCaptureFacade(t *testing.T) {
	if _, exists := reflect.TypeOf((*Capture)(nil)).MethodByName("ClearAll"); exists {
		t.Error("Capture retains ClearAll compatibility facade")
	}
	if _, exists := reflect.TypeOf((*StateResetter)(nil)).MethodByName("ClearAll"); !exists {
		t.Error("StateResetter does not own ClearAll")
	}
}

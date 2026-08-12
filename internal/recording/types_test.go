// Purpose: Tests for the core recording data types and their methods.
// Docs: docs/features/feature/playback-engine/index.md

package recording

import (
	"os"
	"testing"
)

func TestIsFragileSelectorAction(t *testing.T) {
	t.Parallel()

	fragile := map[string]bool{
		"css:#login-button": true,
	}

	tests := []struct {
		name   string
		action RecordingAction
		want   bool
	}{
		{
			name:   "empty selector",
			action: RecordingAction{Type: "click", Selector: ""},
			want:   false,
		},
		{
			name:   "selector marked fragile",
			action: RecordingAction{Type: "click", Selector: "#login-button"},
			want:   true,
		},
		{
			name:   "selector not marked fragile",
			action: RecordingAction{Type: "click", Selector: "#safe-button"},
			want:   false,
		},
	}

	for _, tt := range tests {
		got := tt.action.IsFragileSelectorAction(fragile)
		if got != tt.want {
			t.Fatalf("%s: IsFragileSelectorAction() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// Merged from no_facade_test.go to keep the package within its ten-file budget.
// Guards the recording package against alias-only compatibility surfaces.
func TestRecordingPackageHasNoTypeAliasFacade(t *testing.T) {
	if _, err := os.Stat("type_aliases.go"); !os.IsNotExist(err) {
		t.Fatalf("type_aliases.go compatibility facade must not exist (stat error: %v)", err)
	}
}

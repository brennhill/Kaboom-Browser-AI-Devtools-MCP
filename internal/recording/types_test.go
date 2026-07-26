// Purpose: Tests for the core recording data types and their methods.
// Docs: docs/features/feature/playback-engine/index.md

package recording

import "testing"

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

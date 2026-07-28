// Purpose: Tests for capture settings path resolution.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"path/filepath"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestGetSettingsPathUsesStateDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	path, err := getSettingsPath()
	if err != nil {
		t.Fatalf("getSettingsPath() error = %v", err)
	}

	want := filepath.Join(stateRoot, "settings", "extension-settings.json")
	if path != want {
		t.Fatalf("getSettingsPath() = %q, want %q", path, want)
	}
}

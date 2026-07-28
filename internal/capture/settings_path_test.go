// Purpose: Tests for capture settings path resolution.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestSaveSettingsToDiskWritesToStateDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	c := NewCapture()
	now := time.Now().UTC()

	c.mu.Lock()
	c.extensionState.pilotEnabled = true
	c.extensionState.pilotStatusKnown = true
	c.extensionState.pilotUpdatedAt = now
	c.extensionState.extSessionID = "session-123"
	c.mu.Unlock()

	persisted := saveAndLoadPersistedSettings(t, c)

	if persisted.AIWebPilotEnabled == nil || !*persisted.AIWebPilotEnabled {
		t.Fatalf("AIWebPilotEnabled = %v, want true", persisted.AIWebPilotEnabled)
	}
	if persisted.ExtSessionID != "session-123" {
		t.Fatalf("SessionID = %q, want %q", persisted.ExtSessionID, "session-123")
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func saveAndLoadPersistedSettings(t *testing.T, c *Capture) PersistedSettings {
	t.Helper()
	if err := c.SaveSettingsToDisk(); err != nil {
		t.Fatalf("SaveSettingsToDisk() error = %v", err)
	}

	path, err := getSettingsPath()
	if err != nil {
		t.Fatalf("getSettingsPath() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}

	var persisted PersistedSettings
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return persisted
}

func TestSaveSettingsToDiskOmitsUnknownPilotState(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	c := NewCapture()
	c.mu.Lock()
	c.extensionState.extSessionID = "session-unknown"
	c.mu.Unlock()

	persisted := saveAndLoadPersistedSettings(t, c)
	if persisted.AIWebPilotEnabled != nil {
		t.Fatalf("AIWebPilotEnabled = %v, want nil when pilot state is unknown", persisted.AIWebPilotEnabled)
	}
}

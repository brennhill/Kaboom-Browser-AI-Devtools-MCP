// Purpose: Unit tests for capture pipeline settings handler logic.
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

func TestLoadSettingsFromDiskStaleAndMalformedAreIgnored(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	path, err := getSettingsPath()
	if err != nil {
		t.Fatalf("getSettingsPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Stale settings should be ignored.
	stalePayload := PersistedSettings{
		AIWebPilotEnabled: boolPtrSettings(true),
		Timestamp:         time.Now().Add(-1 * time.Minute),
		ExtSessionID:      "stale",
	}
	staleData, err := json.Marshal(stalePayload)
	if err != nil {
		t.Fatalf("json.Marshal(stale payload) error = %v", err)
	}
	if err := os.WriteFile(path, staleData, 0o600); err != nil {
		t.Fatalf("WriteFile(stale) error = %v", err)
	}

	c := NewCapture()
	c.LoadSettingsFromDisk()
	if c.Extension().IsPilotEnabled() {
		t.Fatal("stale settings unexpectedly enabled pilot")
	}

	// Malformed JSON should be ignored without changing state.
	c.Extension().SetPilotEnabled(true)
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("WriteFile(malformed) error = %v", err)
	}
	c.LoadSettingsFromDisk()
	if !c.Extension().IsPilotEnabled() {
		t.Fatal("malformed settings load should not mutate existing pilot state")
	}
}

func boolPtrSettings(v bool) *bool {
	return &v
}

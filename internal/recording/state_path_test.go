// Purpose: Tests for recording state transition paths.
// Docs: docs/features/feature/playback-engine/index.md

// state_path_test.go — Tests for the canonical recording state location.
package recording

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestPersistRecordingWritesToStateDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	manager := NewRecordingManager()
	recordingID, err := manager.StartRecording("state-storage", "https://example.com", false)
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}

	if _, _, err := manager.StopRecording(recordingID); err != nil {
		t.Fatalf("StopRecording() error = %v", err)
	}

	stateDir, err := state.RecordingsDir()
	if err != nil {
		t.Fatalf("state.RecordingsDir() error = %v", err)
	}
	stateMetadata := filepath.Join(stateDir, recordingID, "metadata.json")
	if _, err := os.Stat(stateMetadata); err != nil {
		t.Fatalf("expected metadata in state directory at %q: %v", stateMetadata, err)
	}

}

// Purpose: Tests for recording state transition paths.
// Docs: docs/features/feature/playback-engine/index.md

// state_path_test.go — Tests for the canonical recording state location.
package recording

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
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

func TestRecordingCleanupAndQuotaFaultsAreRedactedAndRecoverable(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	diagnostics := statediag.NewCollector()
	manager := NewRecordingManager()
	manager.SetDiagnostics(diagnostics)
	id, err := manager.StartRecording("cleanup", "https://example.com", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.StopRecording(id); err != nil {
		t.Fatal(err)
	}
	previousFiles := manager.files
	manager.files = &faultRecordingFilesystem{
		recordingFilesystem: previousFiles,
		walkErr:             statefault.New(statefault.Quota, "private-recording").Error(),
	}
	if err := manager.RecalculateStorageUsed(); err == nil || strings.Contains(err.Error(), "private-recording") {
		t.Fatalf("quota error = %v", err)
	}
	manager.files = &faultRecordingFilesystem{
		recordingFilesystem: previousFiles,
		removeErr:           statefault.New(statefault.Write, "private-recording").Error(),
	}
	if err := manager.DeleteRecording(id); err == nil || strings.Contains(err.Error(), "private-recording") {
		t.Fatalf("cleanup error = %v", err)
	}
	manager.files = previousFiles
	if err := manager.RecalculateStorageUsed(); err != nil {
		t.Fatal(err)
	}
	if err := manager.DeleteRecording(id); err != nil {
		t.Fatal(err)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "event_recording_state" || got[0].Lifecycle != statediag.LifecycleRecovered {
		t.Fatalf("resolved storage diagnostics = %#v", got)
	}
}

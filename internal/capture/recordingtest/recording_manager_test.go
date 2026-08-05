// Purpose: Tests for direct access to Capture's canonical recording manager.
// Docs: docs/features/feature/backend-log-streaming/index.md

// recording_manager_test.go — Tests for Capture recording-manager ownership.
package recordingtest

import (
	. "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/logdiff"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestCaptureRecordingManager(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	c := NewCapture()
	t.Cleanup(c.Close)

	id, err := c.Recordings().StartRecording("delegate-test", "https://example.com", true)
	if err != nil {
		t.Fatalf("StartRecording error = %v", err)
	}
	if id == "" {
		t.Fatal("StartRecording returned empty id")
	}

	err = c.Recordings().AddRecordingAction(recording.RecordingAction{Type: "click", Selector: "#btn"})
	if err != nil {
		t.Fatalf("AddRecordingAction error = %v", err)
	}

	actionCount, duration, err := c.Recordings().StopRecording(id)
	if err != nil {
		t.Fatalf("StopRecording error = %v", err)
	}
	if actionCount != 1 {
		t.Errorf("actionCount = %d, want 1", actionCount)
	}
	if duration < 0 {
		t.Errorf("duration = %d, want >= 0", duration)
	}

	info, err := c.Recordings().GetStorageInfo()
	if err != nil {
		t.Fatalf("GetStorageInfo error = %v", err)
	}
	if info.MaxBytes != recording.RecordingStorageMax {
		t.Errorf("MaxBytes = %d, want %d", info.MaxBytes, recording.RecordingStorageMax)
	}
	if info.WarningBytes != recording.RecordingWarningLevel {
		t.Errorf("WarningBytes = %d, want %d", info.WarningBytes, recording.RecordingWarningLevel)
	}

	err = c.Recordings().RecalculateStorageUsed()
	if err != nil {
		t.Fatalf("RecalculateStorageUsed error = %v", err)
	}

	rec := &recording.Recording{
		Actions: []recording.RecordingAction{{Type: "click"}, {Type: "type"}},
	}
	counts := logdiff.CategorizeActionTypes(rec)
	if counts["click"] != 1 || counts["type"] != 1 {
		t.Errorf("CategorizeActionTypes = %+v, want click=1,type=1", counts)
	}
}

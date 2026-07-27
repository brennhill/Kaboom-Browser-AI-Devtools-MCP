// Purpose: Tests for capture recording delegation to recording manager.
// Docs: docs/features/feature/backend-log-streaming/index.md

// recording_delegation_test.go — Tests for Capture delegation to recording.RecordingManager.
package capture

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestNewCaptureDelegation_RecordingManager(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	c := NewCapture()
	t.Cleanup(c.Close)

	id, err := c.StartRecording("delegate-test", "https://example.com", true)
	if err != nil {
		t.Fatalf("StartRecording error = %v", err)
	}
	if id == "" {
		t.Fatal("StartRecording returned empty id")
	}

	err = c.AddRecordingAction(recording.RecordingAction{Type: "click", Selector: "#btn"})
	if err != nil {
		t.Fatalf("AddRecordingAction error = %v", err)
	}

	actionCount, duration, err := c.StopRecording(id)
	if err != nil {
		t.Fatalf("StopRecording error = %v", err)
	}
	if actionCount != 1 {
		t.Errorf("actionCount = %d, want 1", actionCount)
	}
	if duration < 0 {
		t.Errorf("duration = %d, want >= 0", duration)
	}

	info, err := c.GetStorageInfo()
	if err != nil {
		t.Fatalf("GetStorageInfo error = %v", err)
	}
	if info.MaxBytes != recordingStorageMax {
		t.Errorf("MaxBytes = %d, want %d", info.MaxBytes, recordingStorageMax)
	}
	if info.WarningBytes != recordingWarningLevel {
		t.Errorf("WarningBytes = %d, want %d", info.WarningBytes, recordingWarningLevel)
	}

	err = c.RecalculateStorageUsed()
	if err != nil {
		t.Fatalf("RecalculateStorageUsed error = %v", err)
	}

	rec := &recording.Recording{
		Actions: []recording.RecordingAction{{Type: "click"}, {Type: "type"}},
	}
	counts := c.CategorizeActionTypes(rec)
	if counts["click"] != 1 || counts["type"] != 1 {
		t.Errorf("CategorizeActionTypes = %+v, want click=1,type=1", counts)
	}
}

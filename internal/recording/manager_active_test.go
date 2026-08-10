// manager_active_test.go — Contracts for discovering and stopping the active recording.

package recording

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Active recording discoverability
// ---------------------------------------------------------------------------

// event_recording_stop requires the id that start returned, and observe(recordings)
// lists only completed recordings. An agent that lost the id — context truncation,
// a crashed step, a different session — could therefore never record again, and the
// failure it saw blamed storage quota. The manager must expose the active id so the
// stuck state is both diagnosable and recoverable.
func TestActiveRecordingIDIsDiscoverable(t *testing.T) {
	manager := NewRecordingManager()

	if got := manager.ActiveRecordingID(); got != "" {
		t.Fatalf("ActiveRecordingID with nothing recording = %q, want empty", got)
	}

	started, err := manager.StartRecording("discoverable", "https://example.com", false)
	if err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	if got := manager.ActiveRecordingID(); got != started {
		t.Fatalf("ActiveRecordingID = %q, want the started id %q", got, started)
	}

	if _, _, err := manager.StopRecording(started); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}
	if got := manager.ActiveRecordingID(); got != "" {
		t.Fatalf("ActiveRecordingID after stop = %q, want empty", got)
	}
}

// Stopping without an id must close whatever is active, so losing the id is not
// an unrecoverable state.
func TestStopRecordingWithoutIDStopsTheActiveRecording(t *testing.T) {
	manager := NewRecordingManager()

	started, err := manager.StartRecording("implicit-stop", "https://example.com", false)
	if err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	if _, _, err := manager.StopRecording(""); err != nil {
		t.Fatalf("StopRecording(\"\") must stop the active recording: %v", err)
	}
	if got := manager.ActiveRecordingID(); got != "" {
		t.Fatalf("active recording still %q after an implicit stop", got)
	}

	// A second implicit stop has nothing to close and must say so plainly.
	if _, _, err := manager.StopRecording(""); err == nil {
		t.Fatal("StopRecording(\"\") with nothing active must report an error")
	} else if !strings.Contains(err.Error(), "no_active_recording") {
		t.Fatalf("error = %v, want no_active_recording", err)
	}
	_ = started
}

// The already-active condition must be identifiable by callers rather than
// collapsed into a generic failure that blames storage.
func TestStartRecordingReportsAlreadyRecordingDistinctly(t *testing.T) {
	manager := NewRecordingManager()

	first, err := manager.StartRecording("first", "https://example.com", false)
	if err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	_, err = manager.StartRecording("second", "https://example.com", false)
	if err == nil {
		t.Fatal("a second concurrent recording must fail")
	}
	if !IsAlreadyRecording(err) {
		t.Fatalf("IsAlreadyRecording(%v) = false, want true", err)
	}
	if !strings.Contains(err.Error(), first) {
		t.Fatalf("error %v must name the active recording id %q", err, first)
	}
}

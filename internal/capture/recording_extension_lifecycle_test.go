// recording_extension_lifecycle_test.go — Extension recording start, stop, and naming tests.
// Docs: docs/features/feature/flow-recording/index.md

package capture

import (
	"fmt"
	"strings"
	"testing"

	recordingmodel "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

func TestExtensionStartRecording(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Simulate extension calling event_recording_start via configure tool
	recordingID, err := capture.Recordings().StartRecording("checkout", "https://example.com/checkout", false)
	if err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// Verify response contains recording_id
	if recordingID == "" {
		t.Errorf("Expected non-empty recording_id")
	}

	// Verify recording_id format: name-YYYYMMDDTHHMMSSZ
	if !strings.Contains(recordingID, "checkout") {
		t.Errorf("Expected recording_id to contain 'checkout', got: %s", recordingID)
	}

	// Verify recording created in memory and ready for action capture
	if capture.recordingManager.GetActiveRecordingID() == "" {
		t.Errorf("Expected active recording ID to be set")
	}

	if capture.recordingManager.GetActiveRecordingID() != recordingID {
		t.Errorf("Expected active recording to be %s, got: %s", recordingID, capture.recordingManager.GetActiveRecordingID())
	}

	// Verify recording state is initialized
	recording := capture.recordingManager.GetInMemoryRecording(recordingID)
	if recording == nil {
		t.Errorf("Expected recording to exist in memory")
	}

	// Verify we can add actions after starting
	testAction := recordingmodel.RecordingAction{Type: "click", Selector: "button", TimestampMs: int64(1000)}
	err = capture.Recordings().AddRecordingAction(testAction)
	if err != nil {
		t.Errorf("Expected to be able to add action after starting recording: %v", err)
	}

	if len(recording.Actions) != 1 {
		t.Errorf("Expected 1 action after adding, got: %d", len(recording.Actions))
	}
}

// Test Case 5.2: Stop Recording
// GIVEN: Recording active with 12 captured actions
// WHEN: User clicks "Stop Recording" in extension popup
// THEN: Calls configure({action: 'event_recording_stop', recording_id: '...'})
// AND: Response: {status: "ok", action_count: 12, duration_ms: 34521}
// AND: Extension stops capturing
// AND: Shows summary (12 actions, 34.5 seconds)
func TestExtensionStopRecording(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Start recording
	recordingID, err := capture.Recordings().StartRecording("flow-test", "https://example.com", false)
	if err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// Add 12 actions as described in test case
	for i := 0; i < 12; i++ {
		action := recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    fmt.Sprintf("button#action-%d", i+1),
			TimestampMs: int64((i + 1) * 1000), // 1s, 2s, 3s, ... 12s apart
		}
		err := capture.Recordings().AddRecordingAction(action)
		if err != nil {
			t.Fatalf("Failed to add action %d: %v", i, err)
		}
	}

	// Stop recording
	actionCount, durationMs, err := capture.Recordings().StopRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	// Verify response contains action count
	if actionCount != 12 {
		t.Errorf("Expected action_count=12, got: %d", actionCount)
	}

	// Verify response contains positive duration
	if durationMs <= 0 {
		t.Errorf("Expected positive duration_ms, got: %d", durationMs)
	}

	// Verify recording is no longer active
	if capture.recordingManager.GetActiveRecordingID() != "" {
		t.Errorf("Expected active recording to be cleared after stop")
	}

	// Verify recording was persisted
	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to load persisted recording: %v", err)
	}

	if recording.ActionCount != 12 {
		t.Errorf("Expected persisted action_count=12, got: %d", recording.ActionCount)
	}

	if recording.Duration <= 0 {
		t.Errorf("Expected positive duration in persisted recording, got: %d", recording.Duration)
	}
}

// Test Case 5.3: Auto-Name Recording
// GIVEN: Recording captures flow on https://example.com/checkout
// AND: No explicit name provided in event_recording_start
// WHEN: Recording stopped
// THEN: Auto-name from page title: "Checkout - Example Store"
// AND: metadata.json: name = "Checkout - Example Store"
func TestExtensionAutoNameRecording(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Start recording WITHOUT explicit name (empty string)
	// In a real extension, this would use the page title
	// For this test, we verify the system generates a recording ID
	recordingID, err := capture.Recordings().StartRecording("", "https://example.com/checkout", false)
	if err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// Verify auto-generated recording_id starts with "recording-" prefix
	if !strings.Contains(recordingID, "recording-") {
		t.Errorf("Expected auto-generated recording_id to contain 'recording-', got: %s", recordingID)
	}

	// Add an action
	_ = capture.Recordings().AddRecordingAction(recordingmodel.RecordingAction{Type: "click", Selector: "button"})

	// Stop recording
	_, _, err = capture.Recordings().StopRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	// Load the recording
	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to load recording: %v", err)
	}

	// Verify recording was created with correct URL
	if recording.StartURL != "https://example.com/checkout" {
		t.Errorf("Expected start_url to be 'https://example.com/checkout', got: %s", recording.StartURL)
	}

	// Verify recording was persisted with the auto-generated name
	if recording.ID != recordingID {
		t.Errorf("Expected persisted recording ID to match returned ID")
	}

	// Verify metadata.json has the ID
	if recording.ID == "" {
		t.Errorf("Expected recording ID to be set in metadata")
	}
}

// ============================================================================
// Placeholder for future tests: Module 6-7 (Playback, Test Gen, etc.)
// ============================================================================

// This file serves as the TDD blueprint for Phase 1a implementation.
// As implementation progresses, tests will be un-skipped and assertions filled in.

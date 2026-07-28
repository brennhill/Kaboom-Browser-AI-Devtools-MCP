// recording_store_integration_test.go — Recording metadata, persistence, privacy, and query tests.
// Docs: docs/features/feature/flow-recording/index.md

package capture

import (
	"strings"
	"testing"

	recordingmodel "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

func TestRecordingCreateMetadata(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Start recording
	recordingID, err := capture.Recordings().StartRecording("checkout", "https://example.com/checkout", false)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify recording ID format (name-YYYYMMDDTHHMMSSZ)
	if recordingID == "" {
		t.Errorf("Expected non-empty recording_id")
	}
	if !strings.Contains(recordingID, "checkout") {
		t.Errorf("Expected recording_id to contain 'checkout', got: %s", recordingID)
	}

	// Verify recording exists in memory
	recording := capture.recordingManager.GetInMemoryRecording(recordingID)
	if recording == nil {
		t.Errorf("Expected recording to exist in memory")
	}

	// Verify metadata fields
	if recording.Name != "checkout" {
		t.Errorf("Expected name 'checkout', got: %s", recording.Name)
	}
	if recording.StartURL != "https://example.com/checkout" {
		t.Errorf("Expected url, got: %s", recording.StartURL)
	}
	if recording.SensitiveDataEnabled != false {
		t.Errorf("Expected sensitive_data_enabled=false")
	}
	if recording.CreatedAt == "" {
		t.Errorf("Expected created_at to be set")
	}
	if recording.ActionCount != 0 {
		t.Errorf("Expected action_count=0 initially, got: %d", recording.ActionCount)
	}
}

// Test Case 2.2: Add Actions to Recording
// GIVEN: Active recording (recording_id = "checkout-123")
// WHEN: 5 actions sent via POST /query: click, type, navigate, click, type
// THEN: All actions added to recording in memory
// AND: Each action has: type, timestamp_ms, selector, x, y, screenshot_path
// AND: Timestamps in ascending order
func TestRecordingAddActions(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Start recording
	recordingID, err := capture.Recordings().StartRecording("checkout", "https://example.com", false)
	if err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// Add 5 actions
	actions := []recordingmodel.RecordingAction{
		{Type: "navigate", URL: "https://example.com/checkout", X: 0, Y: 0},
		{Type: "click", Selector: "[data-testid=email]", X: 100, Y: 50},
		{Type: "type", Selector: "[data-testid=email]", Text: "test@example.com"},
		{Type: "click", Selector: "[data-testid=next]", X: 200, Y: 100},
		{Type: "navigate", URL: "https://example.com/payment", X: 0, Y: 0},
	}

	for i, action := range actions {
		action.TimestampMs = int64((i + 1) * 1000) // 1000, 2000, 3000, 4000, 5000
		err := capture.Recordings().AddRecordingAction(action)
		if err != nil {
			t.Fatalf("Failed to add action %d: %v", i, err)
		}
	}

	// Verify all actions added
	recording := capture.recordingManager.GetInMemoryRecording(recordingID)
	if len(recording.Actions) != 5 {
		t.Errorf("Expected 5 actions, got: %d", len(recording.Actions))
	}

	// Verify timestamps in ascending order
	for i := 1; i < len(recording.Actions); i++ {
		if recording.Actions[i].TimestampMs < recording.Actions[i-1].TimestampMs {
			t.Errorf("Actions not in timestamp order at index %d", i)
		}
	}

	// Verify action types
	expectedTypes := []string{"navigate", "click", "type", "click", "navigate"}
	for i, expectedType := range expectedTypes {
		if recording.Actions[i].Type != expectedType {
			t.Errorf("Action %d: expected type %s, got %s", i, expectedType, recording.Actions[i].Type)
		}
	}
}

// Test Case 2.3: Persist Recording to Disk
// GIVEN: Active recording with 10 actions
// WHEN: configure({action: 'event_recording_stop', recording_id: '...'})
// THEN: metadata.json persisted with all 10 actions
// AND: File readable as valid JSON
// AND: action_count = 10
// AND: duration_ms >= 0
func TestRecordingPersistToDisk(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Start recording
	recordingID, err := capture.Recordings().StartRecording("test", "https://example.com", false)
	if err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// Add 10 actions
	for i := 0; i < 10; i++ {
		action := recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    "[data-testid=btn]",
			TimestampMs: int64((i + 1) * 1000),
		}
		err := capture.Recordings().AddRecordingAction(action)
		if err != nil {
			t.Fatalf("Failed to add action: %v", err)
		}
	}

	// Stop recording
	actionCount, duration, err := capture.Recordings().StopRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	// Verify counts
	if actionCount != 10 {
		t.Errorf("Expected 10 actions, got: %d", actionCount)
	}
	if duration < 0 {
		t.Errorf("Expected non-negative duration, got: %d", duration)
	}

	// Try to load the recording back from disk
	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to load recording from disk: %v", err)
	}

	// Verify loaded data
	if recording.ActionCount != 10 {
		t.Errorf("Loaded recording: expected 10 actions, got: %d", recording.ActionCount)
	}
	if len(recording.Actions) != 10 {
		t.Errorf("Loaded recording: expected 10 action objects, got: %d", len(recording.Actions))
	}
}

// Test Case 2.4: Sensitive Data Redaction
// GIVEN: Recording with sensitive_data_enabled = false (default)
// WHEN: Type action on password input: "my_password_123"
// THEN: Stored as: {type: "type", text: "[redacted]", ...}
// AND: Original text never stored
func TestRecordingSensitiveDataRedaction(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Start recording with sensitive_data_enabled = false (default)
	recordingID, err := capture.Recordings().StartRecording("login", "https://example.com/login", false)
	if err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// Add type action with sensitive text
	action := recordingmodel.RecordingAction{
		Type:     "type",
		Selector: "input[type=password]",
		Text:     "my_password_123",
	}
	err = capture.Recordings().AddRecordingAction(action)
	if err != nil {
		t.Fatalf("Failed to add action: %v", err)
	}

	// Verify text was redacted
	recording := capture.recordingManager.GetInMemoryRecording(recordingID)
	if len(recording.Actions) != 1 {
		t.Fatalf("Expected 1 action, got: %d", len(recording.Actions))
	}

	if recording.Actions[0].Text != "[redacted]" {
		t.Errorf("Expected text to be '[redacted]', got: '%s'", recording.Actions[0].Text)
	}
}

// Test Case 2.5: Sensitive Data Full Capture (Opt-In)
// GIVEN: User calls configure({action: 'event_recording_start', sensitive_data_enabled: true})
// AND: Extension shows warning popup (mocked in test)
// WHEN: Type action on password input: "test_password"
// THEN: Stored as: {type: "type", text: "test_password", ...}
// AND: metadata.json: sensitive_data_enabled: true
func TestRecordingSensitiveDataOptIn(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Start recording with sensitive_data_enabled = true
	recordingID, err := capture.Recordings().StartRecording("login", "https://example.com/login", true)
	if err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// Verify flag is set
	recording := capture.recordingManager.GetInMemoryRecording(recordingID)
	if !recording.SensitiveDataEnabled {
		t.Errorf("Expected sensitive_data_enabled=true")
	}

	// Add type action with sensitive text
	action := recordingmodel.RecordingAction{
		Type:     "type",
		Selector: "input[type=password]",
		Text:     "test_password",
	}
	err = capture.Recordings().AddRecordingAction(action)
	if err != nil {
		t.Fatalf("Failed to add action: %v", err)
	}

	// Verify text was NOT redacted (because opt-in is enabled)
	if recording.Actions[0].Text != "test_password" {
		t.Errorf("Expected text='test_password', got: '%s'", recording.Actions[0].Text)
	}

	// Verify it persists to disk with flag set
	_, _, err = capture.Recordings().StopRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	// Load it back and verify flag
	loaded, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to load recording: %v", err)
	}
	if !loaded.SensitiveDataEnabled {
		t.Errorf("Loaded recording: expected sensitive_data_enabled=true")
	}
}

// Test Case 2.6: Storage Quota Enforcement
// GIVEN: Recording storage at 100% (1GB used)
// WHEN: User calls configure({action: 'event_recording_start', name: 'new'})
// THEN: Error returned: "recording_storage_full: Recording storage at capacity (1GB)..."
// AND: No recording created
// AND: Next call still fails (no auto-delete)
func TestRecordingStorageQuotaEnforcement(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Simulate storage being at max capacity
	// Set recordingStorageUsed to 1GB (recording.go constant: recordingStorageMax = 1GB)
	capture.recordingManager.SetRecordingStorageUsed(1024 * 1024 * 1024) // 1GB

	// Try to start a new recording when storage is full
	recordingID, err := capture.Recordings().StartRecording("over-quota", "https://example.com", false)

	// Verify error is returned
	if err == nil {
		t.Errorf("Expected error when storage at capacity, got nil")
	}

	// Verify error message mentions storage is full
	if err != nil && !strings.Contains(err.Error(), "recording_storage_full") {
		t.Errorf("Expected error to mention 'recording_storage_full', got: %v", err)
	}

	// Verify no recording was created
	if recordingID != "" {
		t.Errorf("Expected empty recording_id when over quota, got: %s", recordingID)
	}

	// Verify activeRecordingID is empty (no recording started)
	if capture.recordingManager.GetActiveRecordingID() != "" {
		t.Errorf("Expected activeRecordingID to be empty when over quota")
	}
}

// Test Case 2.7: Storage Warning at 80%
// GIVEN: Recording storage at 80% (800MB used)
// WHEN: Any recording operation
// THEN: Warning logged: "recording_storage_warning: Recording storage at 80%..."
// AND: Operation proceeds (non-blocking)
func TestRecordingStorageWarning(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Simulate storage at 80% capacity (warning threshold)
	// recording.go constant: recordingWarningLevel = 800MB
	capture.recordingManager.SetRecordingStorageUsed(800 * 1024 * 1024) // 800MB (80% of 1GB)

	// Try to start a recording when at warning level
	// The operation should proceed (non-blocking) but a warning should be logged
	recordingID, err := capture.Recordings().StartRecording("at-warning-level", "https://example.com", false)

	// Verify no error - operation should succeed despite warning
	if err != nil {
		t.Errorf("Expected operation to proceed at warning level, got error: %v", err)
	}

	// Verify recording was created
	if recordingID == "" {
		t.Errorf("Expected recording_id to be returned even at warning level")
	}

	// Verify recording is active
	if capture.recordingManager.GetActiveRecordingID() != recordingID {
		t.Errorf("Expected active recording to be set")
	}

	// Verify we can still add actions (non-blocking)
	action := recordingmodel.RecordingAction{Type: "click", Selector: "button", TimestampMs: int64(1000)}
	err = capture.Recordings().AddRecordingAction(action)
	if err != nil {
		t.Errorf("Expected to add actions at warning level, got error: %v", err)
	}

	// Verify we can stop recording (non-blocking)
	actionCount, _, err := capture.Recordings().StopRecording(recordingID)
	if err != nil {
		t.Errorf("Expected to stop recording at warning level, got error: %v", err)
	}

	if actionCount != 1 {
		t.Errorf("Expected 1 action captured, got: %d", actionCount)
	}
}

// Test Case 2.8: List Recordings
// GIVEN: 5 recordings stored on disk
// WHEN: observe({what: 'recordings', limit: 10})
// THEN: Returns array of 5 recordings
// AND: Each includes: id, name, created_at, action_count, url
// AND: Sorted by created_at (newest first)
func TestRecordingListRecordings(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create 1 recording to test listing
	recordingID, err := capture.Recordings().StartRecording("listtest", "https://example.com", false)
	if err != nil {
		t.Fatalf("Failed to create recording: %v", err)
	}

	// Add an action
	err = capture.Recordings().AddRecordingAction(recordingmodel.RecordingAction{Type: "click", Selector: "btn"})
	if err != nil {
		t.Fatalf("Failed to add action: %v", err)
	}

	// Stop recording
	_, _, err = capture.Recordings().StopRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	// List recordings
	recordings, err := capture.Recordings().ListRecordings(100)
	if err != nil {
		t.Fatalf("Failed to list recordings: %v", err)
	}

	// We should have at least 1 recording
	if len(recordings) < 1 {
		t.Errorf("Expected at least 1 recording, got: %d", len(recordings))
	}

	// Verify required fields are present on all recordings
	for i, recording := range recordings {
		if recording.ID == "" {
			t.Errorf("Recording %d: expected non-empty id", i)
		}
		if recording.CreatedAt == "" {
			t.Errorf("Recording %d: expected non-empty created_at", i)
		}
		if recording.StartURL == "" {
			t.Errorf("Recording %d: expected non-empty start_url", i)
		}
	}
}

// Test Case 2.9: Query Recording Actions
// GIVEN: Recording with 10 actions
// WHEN: observe({what: 'recording_actions', recording_id: 'checkout-123'})
// THEN: Returns: {recording_id: "...", actions: [...10 items...]}
// AND: Each action has all fields
// AND: Timestamps in order
func TestRecordingQueryActions(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create recording with 10 actions
	recordingID, err := capture.Recordings().StartRecording("query-test", "https://example.com", false)
	if err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	for i := 0; i < 10; i++ {
		action := recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    "button",
			TimestampMs: int64((i + 1) * 1000),
			X:           100 + i*10,
			Y:           50 + i*10,
		}
		err := capture.Recordings().AddRecordingAction(action)
		if err != nil {
			t.Fatalf("Failed to add action %d: %v", i, err)
		}
	}

	// Stop and load the recording
	_, _, err = capture.Recordings().StopRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to get recording: %v", err)
	}

	// Verify all actions are returned
	if len(recording.Actions) != 10 {
		t.Errorf("Expected 10 actions, got: %d", len(recording.Actions))
	}

	// Verify all have required fields
	for i, action := range recording.Actions {
		if action.Type == "" {
			t.Errorf("Action %d: missing type", i)
		}
		if action.TimestampMs <= 0 {
			t.Errorf("Action %d: missing timestamp_ms", i)
		}
	}

	// Verify timestamps in order
	for i := 1; i < len(recording.Actions); i++ {
		if recording.Actions[i].TimestampMs < recording.Actions[i-1].TimestampMs {
			t.Errorf("Actions not in timestamp order at index %d", i)
		}
	}
}

// ============================================================================
// Module 3: Playback Engine Tests (for Flow Recording feature)
// ============================================================================

// Test Case 3.1: Load Recording
// GIVEN: Recording stored at ~/.kaboom/recordings/checkout-123/metadata.json
// WHEN: playback.LoadRecording("checkout-123")
// THEN: Recording loaded successfully
// AND: All 8 actions in memory
// AND: No errors

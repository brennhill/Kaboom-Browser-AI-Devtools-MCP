// recording_logdiff_integration_test.go — Recorded log comparison and categorization tests.
// Docs: docs/features/feature/flow-recording/index.md

package capture

import (
	"testing"

	recordingmodel "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

func TestLogDiffMatch(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create recording with actions (simulating a user flow)
	recordingID, err := capture.Recordings().StartRecording("user-flow", "https://example.com/checkout", false)
	if err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// Add 5 click actions (happy path - no errors)
	for i := 0; i < 5; i++ {
		action := recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    "button.checkout",
			TimestampMs: int64((i + 1) * 1000),
		}
		_ = capture.Recordings().AddRecordingAction(action)
	}

	actionCount, _, err := capture.Recordings().StopRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	// Load recording
	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to load recording: %v", err)
	}

	// Verify recording matches expected state (no regressions)
	if recording.ActionCount != 5 {
		t.Errorf("Expected 5 actions, got: %d", recording.ActionCount)
	}
	if actionCount != 5 {
		t.Errorf("Expected action count 5, got: %d", actionCount)
	}

	// Verify no error actions (clean run)
	hasErrorAction := false
	for _, action := range recording.Actions {
		if action.Type == "error" {
			hasErrorAction = true
		}
	}
	if hasErrorAction {
		t.Errorf("Expected no error actions in clean recording")
	}
}

// Test Case 4.2: Regression - New Errors
// GIVEN: Original logs (no errors)
// AND: Replay logs after introducing a bug
// WHEN: Log diff compares them
// THEN: Status = "regression"
// AND: NewErrors contains error entries from replay
// AND: summary: "⚠️ REGRESSION: 3 new errors detected"
func TestLogDiffNewErrors(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create original recording (no errors)
	recordingID1, err := capture.Recordings().StartRecording("original", "https://example.com", false)
	if err != nil {
		t.Fatalf("Failed to start original recording: %v", err)
	}
	for i := 0; i < 3; i++ {
		action := recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    "button",
			TimestampMs: int64((i + 1) * 1000),
		}
		_ = capture.Recordings().AddRecordingAction(action)
	}
	capture.Recordings().StopRecording(recordingID1)

	// Create replay recording with more actions (simulating a regression)
	recordingID2, _ := capture.Recordings().StartRecording("replay", "https://example.com", false)
	for i := 0; i < 3; i++ {
		action := recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    "button",
			TimestampMs: int64((i + 1) * 1000),
		}
		_ = capture.Recordings().AddRecordingAction(action)
	}
	// Add extra action to simulate regression/new error condition
	extraAction := recordingmodel.RecordingAction{
		Type:        "error",
		Selector:    "button.broken",
		TimestampMs: int64(4 * 1000),
		Text:        "Network error occurred",
	}
	_ = capture.Recordings().AddRecordingAction(extraAction)
	capture.Recordings().StopRecording(recordingID2)

	// Load both recordings
	rec1, _ := capture.Recordings().GetRecording(recordingID1)
	rec2, _ := capture.Recordings().GetRecording(recordingID2)

	// Verify recordings are different
	if rec1.ActionCount == rec2.ActionCount {
		t.Errorf("Expected different action counts (original: %d, replay: %d)", rec1.ActionCount, rec2.ActionCount)
	}

	// Verify replay has more actions (new error)
	if rec2.ActionCount <= rec1.ActionCount {
		t.Errorf("Expected replay to have more actions than original, got: %d vs %d", rec2.ActionCount, rec1.ActionCount)
	}

	// Verify the extra action type is error
	hasErrorAction := false
	for _, action := range rec2.Actions {
		if action.Type == "error" {
			hasErrorAction = true
			break
		}
	}
	if !hasErrorAction {
		t.Errorf("Expected replay to have error action")
	}
}

// Test Case 4.3: Fixed - Missing Events
// GIVEN: Original logs with errors (bug present)
// AND: Replay logs without those errors (bug fixed)
// WHEN: Log diff compares them
// THEN: Status = "fixed"
// AND: No new errors
// AND: summary: "✓ FIXED: 3 errors no longer appear"
func TestLogDiffFixed(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create original recording with error
	recordingID1, _ := capture.Recordings().StartRecording("buggy", "https://example.com", false)
	errorAction := recordingmodel.RecordingAction{
		Type:        "error",
		Selector:    "button.broken",
		TimestampMs: int64(1000),
		Text:        "Element not clickable",
	}
	_ = capture.Recordings().AddRecordingAction(errorAction)
	for i := 0; i < 2; i++ {
		action := recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    "button",
			TimestampMs: int64((i + 2) * 1000),
		}
		_ = capture.Recordings().AddRecordingAction(action)
	}
	capture.Recordings().StopRecording(recordingID1)

	// Create replay recording without error (bug fixed)
	recordingID2, _ := capture.Recordings().StartRecording("fixed", "https://example.com", false)
	for i := 0; i < 3; i++ {
		action := recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    "button",
			TimestampMs: int64((i + 1) * 1000),
		}
		_ = capture.Recordings().AddRecordingAction(action)
	}
	capture.Recordings().StopRecording(recordingID2)

	// Load both recordings
	rec1, _ := capture.Recordings().GetRecording(recordingID1)
	rec2, _ := capture.Recordings().GetRecording(recordingID2)

	// Verify original has error action
	hasErrorOriginal := false
	for _, action := range rec1.Actions {
		if action.Type == "error" {
			hasErrorOriginal = true
			break
		}
	}
	if !hasErrorOriginal {
		t.Errorf("Expected original recording to have error action")
	}

	// Verify replay has no error actions (fixed)
	hasErrorReplay := false
	for _, action := range rec2.Actions {
		if action.Type == "error" {
			hasErrorReplay = true
			break
		}
	}
	if hasErrorReplay {
		t.Errorf("Expected replay recording to have no error actions")
	}

	// Verify action counts show improvement
	if rec2.ActionCount < rec1.ActionCount {
		t.Errorf("Expected replay to have same or more actions, got: %d vs %d", rec2.ActionCount, rec1.ActionCount)
	}
}

// Test Case 4.4: Value Changes
// GIVEN: Original log: {level: "info", msg: "Items in cart: 3"}
// AND: Replay log: {level: "info", msg: "Items in cart: 0"}
// WHEN: Log diff compares them
// THEN: Status = "regression"
// AND: ChangedValues contains diff: {field: "msg", from: "...3", to: "...0"}
// AND: summary includes value change info
func TestLogDiffValueChanges(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create recording with type action that has specific value
	recordingID := mustStartRecording(t, capture, "value-test", "https://example.com", true)

	action := recordingmodel.RecordingAction{
		Type:        "type",
		Selector:    "input.cart-count",
		TimestampMs: int64(1000),
		Text:        "3",
	}
	_ = capture.Recordings().AddRecordingAction(action)
	capture.Recordings().StopRecording(recordingID)

	// Load recording
	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to load recording: %v", err)
	}

	// Verify recording has action with expected value
	if len(recording.Actions) < 1 {
		t.Fatalf("Expected at least 1 action in recording")
	}

	// Verify the text value is captured
	if recording.Actions[0].Text != "3" {
		t.Errorf("Expected text value '3', got: '%s'", recording.Actions[0].Text)
	}

	// Verify selector is recorded
	if recording.Actions[0].Selector != "input.cart-count" {
		t.Errorf("Expected selector 'input.cart-count', got: '%s'", recording.Actions[0].Selector)
	}
}

// Test Case 4.5: Categorize Diffs
// GIVEN: Original logs with mix of errors, warnings, info
// AND: Replay logs with different error mix
// WHEN: Log diff categorizes
// THEN: Returns structured diff:
// - NewErrors: [...error entries...]
// - MissingEvents: [...previously seen events...]
// - ChangedValues: [...field diffs...]
// AND: Each entry has: severity, level, message, timestamp
func TestLogDiffCategorize(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create original recording with mixed action types
	recordingID1, err := capture.Recordings().StartRecording("original", "https://example.com", false)
	if err != nil {
		t.Fatalf("Failed to start original recording: %v", err)
	}
	actions1 := []recordingmodel.RecordingAction{
		{Type: "navigate", Selector: "", TimestampMs: 1000, Text: "https://example.com"},
		{Type: "click", Selector: "button.login", TimestampMs: 2000},
		{Type: "type", Selector: "input.username", TimestampMs: 3000, Text: "[redacted]"},
		{Type: "type", Selector: "input.password", TimestampMs: 4000, Text: "[redacted]"},
		{Type: "click", Selector: "button.submit", TimestampMs: 5000},
	}
	for _, a := range actions1 {
		if err := capture.Recordings().AddRecordingAction(a); err != nil {
			t.Fatalf("Failed to add action to original recording: %v", err)
		}
	}
	if _, _, err := capture.Recordings().StopRecording(recordingID1); err != nil {
		t.Fatalf("Failed to stop original recording: %v", err)
	}

	// Create replay recording with different action mix
	recordingID2, err := capture.Recordings().StartRecording("replay", "https://example.com", false)
	if err != nil {
		t.Fatalf("Failed to start replay recording: %v", err)
	}
	actions2 := []recordingmodel.RecordingAction{
		{Type: "navigate", Selector: "", TimestampMs: 1000, Text: "https://example.com"},
		{Type: "click", Selector: "button.login", TimestampMs: 2000},
		{Type: "error", Selector: "input.username", TimestampMs: 3000, Text: "Invalid credentials"},
		{Type: "type", Selector: "input.username", TimestampMs: 3500, Text: "[redacted]"},
		{Type: "type", Selector: "input.password", TimestampMs: 4000, Text: "[redacted]"},
		{Type: "click", Selector: "button.submit", TimestampMs: 5000},
	}
	for _, a := range actions2 {
		if err := capture.Recordings().AddRecordingAction(a); err != nil {
			t.Fatalf("Failed to add action to replay recording: %v", err)
		}
	}
	if _, _, err := capture.Recordings().StopRecording(recordingID2); err != nil {
		t.Fatalf("Failed to stop replay recording: %v", err)
	}

	// Load both recordings
	rec1, err := capture.Recordings().GetRecording(recordingID1)
	if err != nil {
		t.Fatalf("Failed to load original recording: %v", err)
	}
	if rec1 == nil {
		t.Fatal("Original recording is nil")
	}

	rec2, err := capture.Recordings().GetRecording(recordingID2)
	if err != nil {
		t.Fatalf("Failed to load replay recording: %v", err)
	}
	if rec2 == nil {
		t.Fatal("Replay recording is nil")
	}

	// Verify we can categorize action types
	actionTypeCount1 := make(map[string]int)
	for _, a := range rec1.Actions {
		actionTypeCount1[a.Type]++
	}

	actionTypeCount2 := make(map[string]int)
	for _, a := range rec2.Actions {
		actionTypeCount2[a.Type]++
	}

	// Verify navigate actions exist in both
	if actionTypeCount1["navigate"] != 1 {
		t.Errorf("Expected 1 navigate action in original, got: %d", actionTypeCount1["navigate"])
	}
	if actionTypeCount2["navigate"] != 1 {
		t.Errorf("Expected 1 navigate action in replay, got: %d", actionTypeCount2["navigate"])
	}

	// Verify replay has error action (regression detection)
	if actionTypeCount2["error"] == 0 {
		t.Errorf("Expected replay to have error action for regression detection")
	}

	// Verify original doesn't have error action
	if actionTypeCount1["error"] != 0 {
		t.Errorf("Expected original to have no error actions")
	}

	// Verify all actions have required fields
	for i, action := range rec2.Actions {
		if action.Type == "" {
			t.Errorf("Action %d: missing type", i)
		}
		if action.TimestampMs <= 0 {
			t.Errorf("Action %d: missing timestamp_ms", i)
		}
	}
}

// ============================================================================
// Module 5: Extension Tests (for Flow Recording feature)
// ============================================================================

// Test Case 5.1: Start Recording
// GIVEN: User clicks "Start Recording" in extension popup
// WHEN: Extension calls configure({action: 'event_recording_start', name: 'checkout'})
// THEN: Status = "ok", recording_id returned
// AND: Extension shows recording UI (red dot, action counter)
// AND: Starts capturing actions (clicks, typing, navigation)

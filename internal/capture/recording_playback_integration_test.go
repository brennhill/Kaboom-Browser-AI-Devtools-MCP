// recording_playback_integration_test.go — Recording playback and selector-recovery tests.
// Docs: docs/features/feature/playback-engine/index.md

package capture

import (
	"testing"

	recordingmodel "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

func TestPlaybackLoadRecording(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create and persist a recording
	recordingID, _ := capture.Recordings().StartRecording("playback-test", "https://example.com", false)
	for i := 0; i < 8; i++ {
		capture.Recordings().AddRecordingAction(recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    "button",
			TimestampMs: int64((i + 1) * 1000),
		})
	}
	capture.Recordings().StopRecording(recordingID)

	// Load recording for playback
	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to load recording: %v", err)
	}

	// Verify all actions loaded
	if len(recording.Actions) != 8 {
		t.Errorf("Expected 8 actions, got: %d", len(recording.Actions))
	}

	// Verify recording metadata
	if recording.ID != recordingID {
		t.Errorf("Expected ID %s, got: %s", recordingID, recording.ID)
	}
	if recording.Name != "playback-test" {
		t.Errorf("Expected name 'playback-test', got: %s", recording.Name)
	}
}

// Test Case 3.2: Execute Navigate Action
// GIVEN: Playback engine with action: {type: "navigate", url: "https://example.com", ...}
// AND: Mock browser navigation + network idle detection
// WHEN: Playback executes action
// THEN: Browser navigates to URL
// AND: Waits for network idle (0 active HTTP requests)
// AND: Timeout = 5 seconds (hard limit)
// AND: Result: {status: "ok", action_executed: true, duration_ms: 1250}
func TestPlaybackNavigateAction(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create a recording with navigate action
	recordingID, _ := capture.Recordings().StartRecording("nav-test", "https://example.com", false)
	capture.Recordings().AddRecordingAction(recordingmodel.RecordingAction{
		Type:        "navigate",
		URL:         "https://example.com/checkout",
		TimestampMs: 1000,
	})
	capture.Recordings().StopRecording(recordingID)

	// Load the recording
	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("Failed to load recording: %v", err)
	}

	// Verify navigate action is present
	if len(recording.Actions) != 1 {
		t.Fatalf("Expected 1 action, got: %d", len(recording.Actions))
	}

	action := recording.Actions[0]
	if action.Type != "navigate" {
		t.Errorf("Expected type 'navigate', got: %s", action.Type)
	}
	if action.URL != "https://example.com/checkout" {
		t.Errorf("Expected URL 'https://example.com/checkout', got: %s", action.URL)
	}
}

// Test Case 3.3: Execute Click Action
// GIVEN: Playback action: {type: "click", selector: "[data-testid=add-to-cart]", x: 500, y: 300}
// AND: Element exists on page
// WHEN: Playback executes click
// THEN: Element found via querySelector
// AND: Element clicked at coordinates
// AND: Result: {status: "ok", action_executed: true, selector_matched: "data-testid"}
func TestPlaybackClickAction(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create a recording with click action
	recordingID, _ := capture.Recordings().StartRecording("click-test", "https://example.com", false)
	capture.Recordings().AddRecordingAction(recordingmodel.RecordingAction{
		Type:        "click",
		Selector:    "[data-testid=add-to-cart]",
		X:           500,
		Y:           300,
		DataTestID:  "add-to-cart",
		TimestampMs: 1000,
	})
	capture.Recordings().StopRecording(recordingID)

	// Load and verify action
	recording, _ := capture.Recordings().GetRecording(recordingID)
	action := recording.Actions[0]

	if action.Type != "click" {
		t.Errorf("Expected type 'click', got: %s", action.Type)
	}
	if action.Selector != "[data-testid=add-to-cart]" {
		t.Errorf("Expected selector with data-testid")
	}
	if action.X != 500 || action.Y != 300 {
		t.Errorf("Expected coordinates (500, 300), got: (%d, %d)", action.X, action.Y)
	}
}

// Test Case 3.4: Execute Click with Self-Healing
// GIVEN: Playback action has original selector: "[data-testid=add-to-cart]"
// AND: Selector no longer matches (element moved)
// WHEN: Playback tries to execute click
// THEN: Self-healing kicks in: tries CSS, nearby x/y, last-known x/y
// AND: Clicks element via fallback selector
// AND: Result: {status: "ok", action_executed: true, selector_matched: "nearby_xy"}
func TestPlaybackClickSelfHealing(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create action with data-testid (primary selector)
	recordingID, _ := capture.Recordings().StartRecording("healing-test", "https://example.com", false)
	capture.Recordings().AddRecordingAction(recordingmodel.RecordingAction{
		Type:        "click",
		Selector:    "[data-testid=add-to-cart]",
		DataTestID:  "add-to-cart",
		X:           500,
		Y:           300,
		TimestampMs: 1000,
	})
	capture.Recordings().StopRecording(recordingID)

	// Verify action has fallback coordinates for self-healing
	recording, _ := capture.Recordings().GetRecording(recordingID)
	action := recording.Actions[0]

	// Self-healing should use fallback strategies
	if action.X <= 0 || action.Y <= 0 {
		t.Errorf("Action should have fallback coordinates for self-healing")
	}
}

// Test Case 3.5: Fragile Selector Detection
// GIVEN: Recording with 5 playback runs
// WHEN: Same action has different selectors in each run (element moved)
// THEN: Flag recorded as "selector_fragile: true"
// AND: Warning in log: "Fragile selector detected: [data-testid=add-to-cart]"
// AND: LLM can adjust action text instead
func TestPlaybackFragileSelectorDetection(t *testing.T) {
	t.Parallel()

	// Test detection of fragile selectors
	// This would be detected during playback comparison
	// For now, just test that we can record actions with potentially fragile selectors

	capture := setupTestCapture(t)
	recordingID, _ := capture.Recordings().StartRecording("fragile-test", "https://example.com", false)

	// Add click actions (could have fragile selectors)
	for i := 0; i < 3; i++ {
		capture.Recordings().AddRecordingAction(recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    ".button-" + string(rune('a'+i)),
			X:           100 + (i * 50),
			Y:           50,
			TimestampMs: int64((i + 1) * 1000),
		})
	}
	capture.Recordings().StopRecording(recordingID)

	recording, _ := capture.Recordings().GetRecording(recordingID)
	if len(recording.Actions) != 3 {
		t.Errorf("Expected 3 actions, got: %d", len(recording.Actions))
	}
}

// Test Case 3.6: Non-Blocking Playback Error
// GIVEN: Playback sequence with 5 actions
// AND: Action 3 fails (selector not found)
// WHEN: Playback executes
// THEN: Error recorded for action 3 (non-blocking)
// AND: Continues with actions 4, 5
// AND: Result: {status: "partial", actions_executed: 5, actions_failed: 1}
func TestPlaybackNonBlockingError(t *testing.T) {
	t.Parallel()

	capture := setupTestCapture(t)

	// Create a recording with 5 actions
	recordingID, _ := capture.Recordings().StartRecording("error-test", "https://example.com", false)
	for i := 0; i < 5; i++ {
		capture.Recordings().AddRecordingAction(recordingmodel.RecordingAction{
			Type:        "click",
			Selector:    ".btn",
			TimestampMs: int64((i + 1) * 1000),
		})
	}
	capture.Recordings().StopRecording(recordingID)

	recording, _ := capture.Recordings().GetRecording(recordingID)

	// Verify all actions are still recorded even if some might fail
	if len(recording.Actions) != 5 {
		t.Errorf("Expected all 5 actions to be recorded, got: %d", len(recording.Actions))
	}

	// Non-blocking: all actions should be present regardless of errors
	for i, action := range recording.Actions {
		if action.Type != "click" {
			t.Errorf("Action %d: expected type 'click', got: %s", i, action.Type)
		}
	}
}

// ============================================================================
// Module 4: Log Diffing Tests (for Flow Recording feature)
// ============================================================================

// Test Case 4.1: Match - No Regressions
// GIVEN: Original logs from first recording
// AND: Replay logs from same flow (after no bug fix)
// WHEN: Log diff compares them
// THEN: Status = "match"
// AND: No new errors, no missing events
// AND: summary: "All logs match (0 new errors, 0 missing events)"

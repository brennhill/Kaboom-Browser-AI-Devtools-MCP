// Purpose: Unit tests for the replay engine (session lifecycle, action execution, fragility analysis).
// Docs: docs/features/feature/playback-engine/index.md

// playback_test.go — Unit tests for playback engine internals.
package playback

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

// fakeSource is an in-memory RecordingSource so replay can be exercised
// without touching disk.
type fakeSource struct {
	recordings map[string]*recording.Item
}

func (f *fakeSource) LookupRecording(id string) (*recording.Item, error) {
	if rec, ok := f.recordings[id]; ok {
		return rec, nil
	}
	return nil, errors.New("read_failed: recording not found: " + id)
}

func TestPlaybackStartAndExecute(t *testing.T) {
	t.Parallel()

	src := &fakeSource{recordings: map[string]*recording.Item{}}

	if _, err := Start(src, "missing-recording"); err == nil || !strings.Contains(err.Error(), "playback_recording_not_found") {
		t.Fatalf("Start(missing) error = %v, want playback_recording_not_found", err)
	}

	src.recordings["empty"] = &recording.Item{ID: "empty", Actions: nil}
	if _, err := Start(src, "empty"); err == nil || !strings.Contains(err.Error(), "playback_no_actions") {
		t.Fatalf("Start(empty) error = %v, want playback_no_actions", err)
	}

	src.recordings["flow"] = &recording.Item{
		ID: "flow",
		Actions: []recording.Action{
			{Type: "navigate"},
			{Type: "click", Selector: "#submit"},
			{Type: "type", Text: "hello"},
			{Type: "unknown"},
		},
	}

	session, err := Execute(src, "flow")
	if err != nil {
		t.Fatalf("Execute(flow) error = %v", err)
	}
	if got := len(session.Results); got != 4 {
		t.Fatalf("len(session.Results) = %d, want 4", got)
	}
	if session.ActionsExecuted != 3 {
		t.Fatalf("ActionsExecuted = %d, want 3", session.ActionsExecuted)
	}
	if session.ActionsFailed != 1 {
		t.Fatalf("ActionsFailed = %d, want 1", session.ActionsFailed)
	}
	if session.Results[3].Status != "error" || !strings.Contains(session.Results[3].Error, "unknown_action_type") {
		t.Fatalf("unexpected unknown action result: %+v", session.Results[3])
	}
}

// vanishingSource returns the recording once and then reports it missing,
// reproducing a delete landing between Start and the replay read.
type vanishingSource struct {
	rec  *recording.Item
	seen int
}

func (v *vanishingSource) LookupRecording(string) (*recording.Item, error) {
	v.seen++
	if v.seen == 1 {
		return v.rec, nil
	}
	return nil, errors.New("read_failed: recording deleted mid-playback")
}

func TestExecuteRecordingDeletedAfterStart(t *testing.T) {
	t.Parallel()

	src := &vanishingSource{rec: &recording.Item{ID: "flow", Actions: []recording.Action{{Type: "navigate"}}}}
	_, err := Execute(src, "flow")
	if err == nil || !strings.Contains(err.Error(), "playback_load_failed") {
		t.Fatalf("Execute(deleted mid-run) error = %v, want playback_load_failed", err)
	}
}

func TestExecuteClickWithHealingStrategies(t *testing.T) {
	t.Parallel()

	dataTestID := executeClickWithHealing(recording.Action{Type: "click", DataTestID: "login"})
	if dataTestID.Status != "ok" || dataTestID.SelectorUsed != "data-testid" {
		t.Fatalf("data-testid strategy failed: %+v", dataTestID)
	}

	css := executeClickWithHealing(recording.Action{Type: "click", Selector: ".primary-button"})
	if css.Status != "ok" || css.SelectorUsed != "css" {
		t.Fatalf("css strategy failed: %+v", css)
	}

	nearby := executeClickWithHealing(recording.Action{Type: "click", X: 10, Y: 20})
	if nearby.Status != "ok" || nearby.SelectorUsed != "nearby_xy" {
		t.Fatalf("nearby strategy failed: %+v", nearby)
	}

	lastKnown := executeClickWithHealing(recording.Action{Type: "click", ScreenshotPath: "shot.png"})
	if lastKnown.Status != "ok" || lastKnown.SelectorUsed != "last_known" {
		t.Fatalf("last-known strategy failed: %+v", lastKnown)
	}

	failed := executeClickWithHealing(recording.Action{Type: "click"})
	if failed.Status != "error" || !strings.Contains(failed.Error, "selector_not_found") {
		t.Fatalf("failed strategy result = %+v, want selector_not_found error", failed)
	}
}

func TestTryClickSelectorValidation(t *testing.T) {
	t.Parallel()

	if tryClickSelector("") {
		t.Fatal("empty selector should fail")
	}
	if !tryClickSelector("[data-testid=submit]") {
		t.Fatal("data-testid selector should pass")
	}
	if !tryClickSelector(".btn") {
		t.Fatal("class selector should pass")
	}
	if !tryClickSelector("#submit") {
		t.Fatal("id selector should pass")
	}
	if !tryClickSelector("[role=button]") {
		t.Fatal("attribute selector should pass")
	}
	if tryClickSelector("div > button") {
		t.Fatal("unsupported selector prefix should fail")
	}
}

func TestDetectFragileSelectorsAndStatus(t *testing.T) {
	t.Parallel()

	// Needs at least 2 sessions.
	if got := DetectFragileSelectors([]*Session{{}}); len(got) != 0 {
		t.Fatalf("DetectFragileSelectors(single run) = %+v, want empty", got)
	}

	s1 := &Session{
		Results: []Result{
			{ActionType: "click", SelectorUsed: "css", Status: "error"},
			{ActionType: "click", SelectorUsed: "css", Status: "error"},
			{ActionType: "click", SelectorUsed: "data-testid", Status: "ok"},
		},
	}
	s2 := &Session{
		Results: []Result{
			{ActionType: "click", SelectorUsed: "css", Status: "ok"},
			{ActionType: "click", SelectorUsed: "data-testid", Status: "ok"},
		},
	}
	fragile := DetectFragileSelectors([]*Session{s1, s2})
	if !fragile["css:css"] {
		t.Fatalf("expected css selector to be marked fragile, got %+v", fragile)
	}
	if fragile["data-testid:data-testid"] {
		t.Fatalf("data-testid selector should not be fragile, got %+v", fragile)
	}

	now := time.Now()
	failedStatus := Status(&Session{StartedAt: now, ActionsExecuted: 0, ActionsFailed: 0})
	if failedStatus["status"] != "failed" {
		t.Fatalf("status for zero executed actions = %v, want failed", failedStatus["status"])
	}

	partialStatus := Status(&Session{
		StartedAt:       now.Add(-5 * time.Millisecond),
		ActionsExecuted: 3,
		ActionsFailed:   1,
		Results:         []Result{{}, {}, {}, {}},
	})
	if partialStatus["status"] != "partial" {
		t.Fatalf("status for mixed results = %v, want partial", partialStatus["status"])
	}
}

// ============================================
// Status projection
// ============================================

func TestNewGetPlaybackStatus_AllOK(t *testing.T) {
	t.Parallel()

	session := &Session{
		StartedAt:        time.Now().Add(-1 * time.Second),
		ActionsExecuted:  5,
		ActionsFailed:    0,
		Results:          make([]Result, 5),
		SelectorFailures: map[string]int{},
	}

	status := Status(session)

	if status["status"] != "ok" {
		t.Errorf("status = %v, want ok", status["status"])
	}
	if status["actions_executed"] != 5 {
		t.Errorf("actions_executed = %v, want 5", status["actions_executed"])
	}
	if status["actions_failed"] != 0 {
		t.Errorf("actions_failed = %v, want 0", status["actions_failed"])
	}
	if status["actions_total"] != 5 {
		t.Errorf("actions_total = %v, want 5", status["actions_total"])
	}
	if status["results_count"] != 5 {
		t.Errorf("results_count = %v, want 5", status["results_count"])
	}
}

func TestNewGetPlaybackStatus_Partial(t *testing.T) {
	t.Parallel()

	session := &Session{
		StartedAt:        time.Now().Add(-2 * time.Second),
		ActionsExecuted:  3,
		ActionsFailed:    2,
		Results:          make([]Result, 5),
		SelectorFailures: map[string]int{"css": 2},
	}

	status := Status(session)

	if status["status"] != "partial" {
		t.Errorf("status = %v, want partial", status["status"])
	}
	if status["actions_total"] != 5 {
		t.Errorf("actions_total = %v, want 5", status["actions_total"])
	}
}

func TestNewGetPlaybackStatus_Failed(t *testing.T) {
	t.Parallel()

	session := &Session{
		StartedAt:        time.Now().Add(-1 * time.Second),
		ActionsExecuted:  0,
		ActionsFailed:    3,
		Results:          make([]Result, 3),
		SelectorFailures: map[string]int{},
	}

	status := Status(session)

	if status["status"] != "failed" {
		t.Errorf("status = %v, want failed", status["status"])
	}
}

func TestNewGetPlaybackStatus_DurationPositive(t *testing.T) {
	t.Parallel()

	session := &Session{
		StartedAt:        time.Now().Add(-100 * time.Millisecond),
		ActionsExecuted:  1,
		SelectorFailures: map[string]int{},
	}

	status := Status(session)

	durationMs, ok := status["duration_ms"].(int64)
	if !ok {
		t.Fatal("duration_ms not int64")
	}
	if durationMs < 0 {
		t.Errorf("duration_ms = %d, want >= 0", durationMs)
	}
}

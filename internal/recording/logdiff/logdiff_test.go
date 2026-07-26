// Purpose: Unit tests for recording log-diff comparison, status branches, and report rendering.
// Docs: docs/features/feature/playback-engine/index.md

// logdiff_test.go — Unit tests for log diff internals.
package logdiff

import (
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

// fakeSource is an in-memory RecordingSource so diffing can be exercised
// without touching disk.
type fakeSource struct {
	recordings map[string]*recording.Item
}

func (f *fakeSource) GetRecording(id string) (*recording.Item, error) {
	if rec, ok := f.recordings[id]; ok {
		return rec, nil
	}
	return nil, errors.New("read_failed: recording not found: " + id)
}

func sourceWith(t *testing.T, entries map[string][]recording.Action) *fakeSource {
	t.Helper()
	src := &fakeSource{recordings: make(map[string]*recording.Item, len(entries))}
	for id, actions := range entries {
		src.recordings[id] = &recording.Item{
			ID:          id,
			Name:        id,
			StartURL:    "https://app.example.com",
			ActionCount: len(actions),
			Actions:     actions,
		}
	}
	return src
}

func TestDiffRecordingsRegressionAndValueChanges(t *testing.T) {
	t.Parallel()

	src := sourceWith(t, map[string][]recording.Action{
		"orig": {
			{Type: "click", Selector: "#submit", TimestampMs: 10},
			{Type: "type", Selector: "#email", Text: "before@example.com", TimestampMs: 20},
			{Type: "error", Text: "E1 existing", TimestampMs: 30},
		},
		"replay": {
			{Type: "click", Selector: "#submit", TimestampMs: 15},
			{Type: "type", Selector: "#email", Text: "after@example.com", TimestampMs: 25},
			{Type: "error", Text: "E1 existing", TimestampMs: 35},
			{Type: "error", Text: "E2 new regression", TimestampMs: 45},
		},
	})

	result, err := Compare(src, "orig", "replay")
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}

	if result.Status != "regression" {
		t.Fatalf("result.Status = %q, want regression", result.Status)
	}
	if len(result.NewErrors) != 1 || result.NewErrors[0].Message != "E2 new regression" {
		t.Fatalf("unexpected NewErrors: %+v", result.NewErrors)
	}
	if len(result.ChangedValues) != 1 || result.ChangedValues[0].Field != "#email" {
		t.Fatalf("unexpected ChangedValues: %+v", result.ChangedValues)
	}
	if result.ActionStats.OriginalCount != 3 || result.ActionStats.ReplayCount != 4 {
		t.Fatalf("unexpected ActionStats counts: %+v", result.ActionStats)
	}
	if result.ActionStats.ErrorsOriginal != 1 || result.ActionStats.ErrorsReplay != 2 {
		t.Fatalf("unexpected ActionStats error counts: %+v", result.ActionStats)
	}

	report := result.GetRegressionReport()
	if !strings.Contains(report, "Status: regression") {
		t.Fatalf("report missing status: %s", report)
	}
	if !strings.Contains(report, "New Errors (1)") {
		t.Fatalf("report missing new errors section: %s", report)
	}
	if !strings.Contains(report, "Changed Values (1)") {
		t.Fatalf("report missing changed values section: %s", report)
	}
}

func TestDiffRecordingsLoadError(t *testing.T) {
	t.Parallel()

	src := sourceWith(t, map[string][]recording.Action{
		"present": {{Type: "click", Selector: "#a"}},
	})

	if _, err := Compare(src, "missing-original", "missing-replay"); err == nil ||
		!strings.Contains(err.Error(), "logdiff_load_original_failed") {
		t.Fatalf("Compare(missing, missing) error = %v, want logdiff_load_original_failed", err)
	}

	if _, err := Compare(src, "present", "missing-replay"); err == nil ||
		!strings.Contains(err.Error(), "logdiff_load_replay_failed") {
		t.Fatalf("Compare(present, missing) error = %v, want logdiff_load_replay_failed", err)
	}
}

func TestLogDiffStatusBranchesAndHelpers(t *testing.T) {
	t.Parallel()

	r := &Result{NewErrors: []LogEntry{{Message: "new"}}}
	determineStatus(r)
	if r.Status != "regression" {
		t.Fatalf("determineStatus(regression) = %q, want regression", r.Status)
	}

	r = &Result{MissingEvents: []LogEntry{{Message: "fixed"}}}
	determineStatus(r)
	if r.Status != "fixed" {
		t.Fatalf("determineStatus(fixed) = %q, want fixed", r.Status)
	}

	r = &Result{ChangedValues: []ValueChange{{Field: "#email"}}}
	determineStatus(r)
	if r.Status != "changed" {
		t.Fatalf("determineStatus(changed) = %q, want changed", r.Status)
	}

	r = &Result{}
	determineStatus(r)
	if r.Status != "match" {
		t.Fatalf("determineStatus(match) = %q, want match", r.Status)
	}

	stats := compareActions(
		&recording.Item{
			ActionCount: 3,
			Actions: []recording.Action{
				{Type: "error"},
				{Type: "click"},
				{Type: "navigate"},
			},
		},
		&recording.Item{
			ActionCount: 2,
			Actions: []recording.Action{
				{Type: "type"},
				{Type: "click"},
			},
		},
	)
	if stats.OriginalCount != 3 || stats.ReplayCount != 2 {
		t.Fatalf("compareActions counts = %+v", stats)
	}
	if stats.ErrorsOriginal != 1 || stats.ClicksOriginal != 1 || stats.NavigatesOriginal != 1 {
		t.Fatalf("compareActions original breakdown = %+v", stats)
	}
	if stats.TypesReplay != 1 || stats.ClicksReplay != 1 {
		t.Fatalf("compareActions replay breakdown = %+v", stats)
	}
}

// ============================================
// CategorizeActionTypes Tests
// ============================================

func TestNewCategorizeActionTypes_MixedActions(t *testing.T) {
	t.Parallel()

	rec := &recording.Item{
		Actions: []recording.Action{
			{Type: "click"}, {Type: "click"}, {Type: "type"},
			{Type: "navigate"}, {Type: "scroll"}, {Type: "click"}, {Type: "error"},
		},
	}

	counts := CategorizeActionTypes(rec)

	if counts["click"] != 3 {
		t.Errorf("click count = %d, want 3", counts["click"])
	}
	if counts["type"] != 1 {
		t.Errorf("type count = %d, want 1", counts["type"])
	}
	if counts["navigate"] != 1 {
		t.Errorf("navigate count = %d, want 1", counts["navigate"])
	}
	if counts["scroll"] != 1 {
		t.Errorf("scroll count = %d, want 1", counts["scroll"])
	}
	if counts["error"] != 1 {
		t.Errorf("error count = %d, want 1", counts["error"])
	}
}

func TestNewCategorizeActionTypes_EmptyRecording(t *testing.T) {
	t.Parallel()

	counts := CategorizeActionTypes(&recording.Item{Actions: []recording.Action{}})

	if len(counts) != 0 {
		t.Errorf("counts len = %d, want 0 for empty recording", len(counts))
	}
}

func TestNewCategorizeActionTypes_SingleType(t *testing.T) {
	t.Parallel()

	rec := &recording.Item{
		Actions: []recording.Action{{Type: "click"}, {Type: "click"}, {Type: "click"}},
	}

	counts := CategorizeActionTypes(rec)
	if counts["click"] != 3 {
		t.Errorf("click count = %d, want 3", counts["click"])
	}
	if len(counts) != 1 {
		t.Errorf("counts has %d types, want 1", len(counts))
	}
}

// ============================================
// Report rendering
// ============================================

func TestNewLogDiffResult_GetRegressionReport(t *testing.T) {
	t.Parallel()

	result := &Result{
		Status:            "regression",
		OriginalRecording: "orig-123",
		ReplayRecording:   "replay-456",
		Summary:           "REGRESSION: 1 new errors detected",
		NewErrors: []LogEntry{
			{
				Type:       "error",
				Severity:   "high",
				Level:      "error",
				Message:    "TypeError: undefined is not a function",
				Timestamp:  1500,
				Selector:   "#btn",
				ActionType: "error",
			},
		},
		MissingEvents: []LogEntry{},
		ChangedValues: []ValueChange{
			{
				Field:     "#email",
				FromValue: "old@test.com",
				ToValue:   "new@test.com",
				Timestamp: 2000,
			},
		},
		ActionStats: ActionComparison{
			OriginalCount:     10,
			ReplayCount:       12,
			ErrorsOriginal:    0,
			ErrorsReplay:      1,
			ClicksOriginal:    5,
			ClicksReplay:      5,
			TypesOriginal:     3,
			TypesReplay:       3,
			NavigatesOriginal: 2,
			NavigatesReplay:   3,
		},
	}

	report := result.GetRegressionReport()

	if !strings.Contains(report, "regression") {
		t.Error("report should contain 'regression'")
	}
	if !strings.Contains(report, "TypeError: undefined is not a function") {
		t.Error("report should contain the new error message")
	}
	if !strings.Contains(report, "#email") {
		t.Error("report should contain the changed field")
	}
	if !strings.Contains(report, "old@test.com") {
		t.Error("report should contain the original value")
	}
	if !strings.Contains(report, "new@test.com") {
		t.Error("report should contain the new value")
	}
	if !strings.Contains(report, "Original: 10 actions") {
		t.Error("report should contain original action count")
	}
	if !strings.Contains(report, "Replay: 12 actions") {
		t.Error("report should contain replay action count")
	}
}

func TestNewLogDiffResult_MatchReport(t *testing.T) {
	t.Parallel()

	result := &Result{
		Status:        "match",
		Summary:       "All logs match (0 new errors, 0 missing events)",
		NewErrors:     []LogEntry{},
		MissingEvents: []LogEntry{},
		ChangedValues: []ValueChange{},
		ActionStats:   ActionComparison{OriginalCount: 5, ReplayCount: 5},
	}

	report := result.GetRegressionReport()

	if !strings.Contains(report, "match") {
		t.Error("report should contain 'match'")
	}
	if strings.Contains(report, "New Errors") {
		t.Error("match report should not contain 'New Errors' section")
	}
}

func TestNewLogDiffResult_FixedReport(t *testing.T) {
	t.Parallel()

	result := &Result{
		Status:  "fixed",
		Summary: "FIXED: 2 errors no longer appear",
		MissingEvents: []LogEntry{
			{Type: "error", Message: "Fixed error 1", Timestamp: 1000},
			{Type: "error", Message: "Fixed error 2", Timestamp: 2000},
		},
		NewErrors:     []LogEntry{},
		ChangedValues: []ValueChange{},
		ActionStats:   ActionComparison{},
	}

	report := result.GetRegressionReport()

	if !strings.Contains(report, "fixed") {
		t.Error("report should contain 'fixed'")
	}
	if !strings.Contains(report, "Fixed error 1") {
		t.Error("report should contain fixed error message")
	}
	if !strings.Contains(report, "Fixed/Missing Events (2)") {
		t.Error("report should show missing events count")
	}
}

// ============================================
// CountActionTypes
// ============================================

func TestNewCountActionTypes(t *testing.T) {
	t.Parallel()

	actions := []recording.Action{
		{Type: "error"}, {Type: "click"}, {Type: "click"},
		{Type: "type"}, {Type: "navigate"}, {Type: "navigate"},
		{Type: "navigate"}, {Type: "scroll"},
	}

	errs, clicks, types, navigates := CountActionTypes(actions)

	if errs != 1 {
		t.Errorf("errors = %d, want 1", errs)
	}
	if clicks != 2 {
		t.Errorf("clicks = %d, want 2", clicks)
	}
	if types != 1 {
		t.Errorf("types = %d, want 1", types)
	}
	if navigates != 3 {
		t.Errorf("navigates = %d, want 3", navigates)
	}
}

func TestNewCountActionTypes_Empty(t *testing.T) {
	t.Parallel()

	errs, clicks, types, navigates := CountActionTypes([]recording.Action{})

	if errs != 0 || clicks != 0 || types != 0 || navigates != 0 {
		t.Errorf("all counts should be 0, got errors=%d, clicks=%d, types=%d, navigates=%d",
			errs, clicks, types, navigates)
	}
}

func TestNewCountActionTypes_UnknownTypes(t *testing.T) {
	t.Parallel()

	actions := []recording.Action{
		{Type: "scroll"}, {Type: "unknown"}, {Type: "custom"},
	}

	errs, clicks, types, navigates := CountActionTypes(actions)

	if errs != 0 || clicks != 0 || types != 0 || navigates != 0 {
		t.Errorf("unknown types should not be counted, got errors=%d, clicks=%d, types=%d, navigates=%d",
			errs, clicks, types, navigates)
	}
}

// ============================================
// BuildTypeValueMap
// ============================================

func TestNewBuildTypeValueMap(t *testing.T) {
	t.Parallel()

	actions := []recording.Action{
		{Type: "type", Selector: "#email", Text: "user@test.com"},
		{Type: "type", Selector: "#password", Text: "secret123"},
		{Type: "click", Selector: "#submit"},
		{Type: "type", Selector: "", Text: "no-sel"},
	}

	values := BuildTypeValueMap(actions)

	if values["#email"] != "user@test.com" {
		t.Errorf("values[#email] = %q, want user@test.com", values["#email"])
	}
	if values["#password"] != "secret123" {
		t.Errorf("values[#password] = %q, want secret123", values["#password"])
	}
	if _, ok := values["#submit"]; ok {
		t.Error("click action should not be in type value map")
	}
	if len(values) != 2 {
		t.Errorf("values len = %d, want 2", len(values))
	}
}

func TestNewBuildTypeValueMap_Empty(t *testing.T) {
	t.Parallel()

	values := BuildTypeValueMap([]recording.Action{})
	if len(values) != 0 {
		t.Errorf("values len = %d, want 0", len(values))
	}
}

// Purpose: Tests for snapshot console-error diffing and verdict summarization.
// Docs: docs/features/feature/request-session-correlation/index.md

// errors_test.go — Tests for errors.go.
// Covers: Errors, countPerfRegressions, hasStatusRegression, Summarize.
package snapdiff

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// diffErrors
// ============================================

func TestDiffErrors_NewErrors(t *testing.T) {
	t.Parallel()
	snapA := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{}}
	snapB := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{
		{Type: "error", Message: "TypeError: x is null", Count: 2},
		{Type: "error", Message: "RangeError: invalid index", Count: 1},
	}}

	diff := Errors(snapA, snapB)

	if len(diff.New) != 2 {
		t.Fatalf("Expected 2 new errors, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 0 {
		t.Errorf("Expected 0 resolved errors, got %d", len(diff.Resolved))
	}
	if len(diff.Unchanged) != 0 {
		t.Errorf("Expected 0 unchanged errors, got %d", len(diff.Unchanged))
	}
}

func TestDiffErrors_ResolvedErrors(t *testing.T) {
	t.Parallel()
	snapA := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{
		{Type: "error", Message: "old error 1", Count: 1},
		{Type: "error", Message: "old error 2", Count: 3},
	}}
	snapB := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{}}

	diff := Errors(snapA, snapB)

	if len(diff.Resolved) != 2 {
		t.Fatalf("Expected 2 resolved errors, got %d", len(diff.Resolved))
	}
	if len(diff.New) != 0 {
		t.Errorf("Expected 0 new errors, got %d", len(diff.New))
	}
	if len(diff.Unchanged) != 0 {
		t.Errorf("Expected 0 unchanged errors, got %d", len(diff.Unchanged))
	}
}

func TestDiffErrors_UnchangedErrors(t *testing.T) {
	t.Parallel()
	errors := []types.SnapshotError{
		{Type: "error", Message: "persistent error", Count: 5},
	}
	snapA := &types.NamedSnapshot{ConsoleErrors: errors}
	snapB := &types.NamedSnapshot{ConsoleErrors: errors}

	diff := Errors(snapA, snapB)

	if len(diff.Unchanged) != 1 {
		t.Fatalf("Expected 1 unchanged error, got %d", len(diff.Unchanged))
	}
	if diff.Unchanged[0].Message != "persistent error" {
		t.Errorf("Expected 'persistent error', got %q", diff.Unchanged[0].Message)
	}
	if diff.Unchanged[0].Count != 5 {
		t.Errorf("Expected count=5, got %d", diff.Unchanged[0].Count)
	}
	if len(diff.New) != 0 {
		t.Errorf("Expected 0 new errors, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 0 {
		t.Errorf("Expected 0 resolved errors, got %d", len(diff.Resolved))
	}
}

func TestDiffErrors_MixedChanges(t *testing.T) {
	t.Parallel()
	snapA := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{
		{Type: "error", Message: "stays", Count: 1},
		{Type: "error", Message: "goes away", Count: 1},
	}}
	snapB := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{
		{Type: "error", Message: "stays", Count: 1},
		{Type: "error", Message: "brand new", Count: 1},
	}}

	diff := Errors(snapA, snapB)

	if len(diff.New) != 1 {
		t.Errorf("Expected 1 new error, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 1 {
		t.Errorf("Expected 1 resolved error, got %d", len(diff.Resolved))
	}
	if len(diff.Unchanged) != 1 {
		t.Errorf("Expected 1 unchanged error, got %d", len(diff.Unchanged))
	}

	// Verify exact messages
	if diff.New[0].Message != "brand new" {
		t.Errorf("Expected new error 'brand new', got %q", diff.New[0].Message)
	}
	if diff.Resolved[0].Message != "goes away" {
		t.Errorf("Expected resolved error 'goes away', got %q", diff.Resolved[0].Message)
	}
	if diff.Unchanged[0].Message != "stays" {
		t.Errorf("Expected unchanged error 'stays', got %q", diff.Unchanged[0].Message)
	}
}

func TestDiffErrors_BothEmpty(t *testing.T) {
	t.Parallel()
	snapA := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{}}
	snapB := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{}}

	diff := Errors(snapA, snapB)

	if len(diff.New) != 0 {
		t.Errorf("Expected 0 new, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 0 {
		t.Errorf("Expected 0 resolved, got %d", len(diff.Resolved))
	}
	if len(diff.Unchanged) != 0 {
		t.Errorf("Expected 0 unchanged, got %d", len(diff.Unchanged))
	}
}

func TestDiffErrors_NilConsoleErrors(t *testing.T) {
	t.Parallel()
	snapA := &types.NamedSnapshot{ConsoleErrors: nil}
	snapB := &types.NamedSnapshot{ConsoleErrors: nil}

	diff := Errors(snapA, snapB)

	if len(diff.New) != 0 {
		t.Errorf("Expected 0 new for nil, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 0 {
		t.Errorf("Expected 0 resolved for nil, got %d", len(diff.Resolved))
	}
}

func TestDiffErrors_DuplicateMessagesDeduped(t *testing.T) {
	t.Parallel()
	// Two entries with same message in A - map deduplication means last wins
	snapA := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{
		{Type: "error", Message: "duplicate", Count: 1},
		{Type: "error", Message: "duplicate", Count: 5},
	}}
	snapB := &types.NamedSnapshot{ConsoleErrors: []types.SnapshotError{}}

	diff := Errors(snapA, snapB)

	// Should be 1 resolved (deduped by message)
	if len(diff.Resolved) != 1 {
		t.Errorf("Expected 1 resolved (deduped), got %d", len(diff.Resolved))
	}
}

// ============================================
// countPerfRegressions
// ============================================

func TestCountPerfRegressions_NoRegressions(t *testing.T) {
	t.Parallel()
	diff := PerformanceDiff{
		LoadTime:     &MetricChange{Regression: false},
		RequestCount: &MetricChange{Regression: false},
		TransferSize: &MetricChange{Regression: false},
	}
	if count := countPerfRegressions(diff); count != 0 {
		t.Errorf("Expected 0 regressions, got %d", count)
	}
}

func TestCountPerfRegressions_AllRegressions(t *testing.T) {
	t.Parallel()
	diff := PerformanceDiff{
		LoadTime:     &MetricChange{Regression: true},
		RequestCount: &MetricChange{Regression: true},
		TransferSize: &MetricChange{Regression: true},
	}
	if count := countPerfRegressions(diff); count != 3 {
		t.Errorf("Expected 3 regressions, got %d", count)
	}
}

func TestCountPerfRegressions_NilMetrics(t *testing.T) {
	t.Parallel()
	diff := PerformanceDiff{
		LoadTime:     nil,
		RequestCount: nil,
		TransferSize: nil,
	}
	if count := countPerfRegressions(diff); count != 0 {
		t.Errorf("Expected 0 regressions for nil metrics, got %d", count)
	}
}

func TestCountPerfRegressions_PartialNilMixed(t *testing.T) {
	t.Parallel()
	diff := PerformanceDiff{
		LoadTime:     &MetricChange{Regression: true},
		RequestCount: nil,
		TransferSize: &MetricChange{Regression: false},
	}
	if count := countPerfRegressions(diff); count != 1 {
		t.Errorf("Expected 1 regression, got %d", count)
	}
}

// ============================================
// hasStatusRegression
// ============================================

func TestHasStatusRegression_OKToError(t *testing.T) {
	t.Parallel()
	changes := []NetworkChange{
		{BeforeStatus: 200, AfterStatus: 500},
	}
	if !hasStatusRegression(changes) {
		t.Error("Expected true for 200->500")
	}
}

func TestHasStatusRegression_ErrorToOK(t *testing.T) {
	t.Parallel()
	changes := []NetworkChange{
		{BeforeStatus: 500, AfterStatus: 200},
	}
	if hasStatusRegression(changes) {
		t.Error("Expected false for 500->200 (improvement)")
	}
}

func TestHasStatusRegression_ErrorToError(t *testing.T) {
	t.Parallel()
	changes := []NetworkChange{
		{BeforeStatus: 400, AfterStatus: 500},
	}
	if hasStatusRegression(changes) {
		t.Error("Expected false for 400->500 (was already erroring)")
	}
}

func TestHasStatusRegression_EmptyChanges(t *testing.T) {
	t.Parallel()
	if hasStatusRegression(nil) {
		t.Error("Expected false for nil changes")
	}
	if hasStatusRegression([]NetworkChange{}) {
		t.Error("Expected false for empty changes")
	}
}

func TestHasStatusRegression_MultipleChanges(t *testing.T) {
	t.Parallel()
	changes := []NetworkChange{
		{BeforeStatus: 500, AfterStatus: 200}, // improvement
		{BeforeStatus: 200, AfterStatus: 404}, // regression
	}
	if !hasStatusRegression(changes) {
		t.Error("Expected true when at least one change is regression")
	}
}

func TestHasStatusRegression_BoundaryValues(t *testing.T) {
	t.Parallel()
	// 399 -> 400: OK to error
	if !hasStatusRegression([]NetworkChange{{BeforeStatus: 399, AfterStatus: 400}}) {
		t.Error("Expected true for 399->400")
	}
	// 400 -> 399: error to OK, not a regression in the "OK to error" sense
	if hasStatusRegression([]NetworkChange{{BeforeStatus: 400, AfterStatus: 399}}) {
		t.Error("Expected false for 400->399 (was already erroring, now OK)")
	}
}

// ============================================
// computeSummary
// ============================================

func TestComputeSummary_Unchanged(t *testing.T) {
	t.Parallel()
	result := &Result{
		Errors:      ErrorDiff{New: []types.SnapshotError{}, Resolved: []types.SnapshotError{}, Unchanged: []types.SnapshotError{{Message: "still here"}}},
		Network:     NetworkDiff{NewErrors: []types.SnapshotNetworkRequest{}, StatusChanges: []NetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := Summarize(result)

	if summary.Verdict != "unchanged" {
		t.Errorf("Expected 'unchanged', got %q", summary.Verdict)
	}
	if summary.NewErrors != 0 {
		t.Errorf("Expected new_errors=0, got %d", summary.NewErrors)
	}
	if summary.ResolvedErrors != 0 {
		t.Errorf("Expected resolved_errors=0, got %d", summary.ResolvedErrors)
	}
}

func TestComputeSummary_Improved(t *testing.T) {
	t.Parallel()
	result := &Result{
		Errors:      ErrorDiff{New: []types.SnapshotError{}, Resolved: []types.SnapshotError{{Message: "fixed"}}, Unchanged: []types.SnapshotError{}},
		Network:     NetworkDiff{NewErrors: []types.SnapshotNetworkRequest{}, StatusChanges: []NetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := Summarize(result)
	if summary.Verdict != "improved" {
		t.Errorf("Expected 'improved', got %q", summary.Verdict)
	}
	if summary.ResolvedErrors != 1 {
		t.Errorf("Expected resolved_errors=1, got %d", summary.ResolvedErrors)
	}
}

func TestComputeSummary_Regressed_NewErrors(t *testing.T) {
	t.Parallel()
	result := &Result{
		Errors:      ErrorDiff{New: []types.SnapshotError{{Message: "new err"}}, Resolved: []types.SnapshotError{}, Unchanged: []types.SnapshotError{}},
		Network:     NetworkDiff{NewErrors: []types.SnapshotNetworkRequest{}, StatusChanges: []NetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := Summarize(result)
	if summary.Verdict != "regressed" {
		t.Errorf("Expected 'regressed', got %q", summary.Verdict)
	}
}

func TestComputeSummary_Regressed_NetworkErrors(t *testing.T) {
	t.Parallel()
	result := &Result{
		Errors:      ErrorDiff{New: []types.SnapshotError{}, Resolved: []types.SnapshotError{}, Unchanged: []types.SnapshotError{}},
		Network:     NetworkDiff{NewErrors: []types.SnapshotNetworkRequest{{Status: 500}}, StatusChanges: []NetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := Summarize(result)
	if summary.Verdict != "regressed" {
		t.Errorf("Expected 'regressed', got %q", summary.Verdict)
	}
	if summary.NewNetworkErrors != 1 {
		t.Errorf("Expected new_network_errors=1, got %d", summary.NewNetworkErrors)
	}
}

func TestComputeSummary_Regressed_PerfRegressions(t *testing.T) {
	t.Parallel()
	result := &Result{
		Errors:      ErrorDiff{New: []types.SnapshotError{}, Resolved: []types.SnapshotError{}, Unchanged: []types.SnapshotError{}},
		Network:     NetworkDiff{NewErrors: []types.SnapshotNetworkRequest{}, StatusChanges: []NetworkChange{}},
		Performance: PerformanceDiff{LoadTime: &MetricChange{Regression: true}},
	}

	summary := Summarize(result)
	if summary.Verdict != "regressed" {
		t.Errorf("Expected 'regressed', got %q", summary.Verdict)
	}
	if summary.PerformanceRegressions != 1 {
		t.Errorf("Expected performance_regressions=1, got %d", summary.PerformanceRegressions)
	}
}

func TestComputeSummary_Regressed_StatusRegression(t *testing.T) {
	t.Parallel()
	result := &Result{
		Errors: ErrorDiff{New: []types.SnapshotError{}, Resolved: []types.SnapshotError{}, Unchanged: []types.SnapshotError{}},
		Network: NetworkDiff{
			NewErrors:     []types.SnapshotNetworkRequest{},
			StatusChanges: []NetworkChange{{BeforeStatus: 200, AfterStatus: 500}},
		},
		Performance: PerformanceDiff{},
	}

	summary := Summarize(result)
	if summary.Verdict != "regressed" {
		t.Errorf("Expected 'regressed', got %q", summary.Verdict)
	}
}

func TestComputeSummary_Mixed(t *testing.T) {
	t.Parallel()
	result := &Result{
		Errors: ErrorDiff{
			New:      []types.SnapshotError{{Message: "new"}},
			Resolved: []types.SnapshotError{{Message: "fixed"}},
		},
		Network:     NetworkDiff{NewErrors: []types.SnapshotNetworkRequest{}, StatusChanges: []NetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := Summarize(result)
	if summary.Verdict != "mixed" {
		t.Errorf("Expected 'mixed', got %q", summary.Verdict)
	}
}

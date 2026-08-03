// runner_test.go — Workflow verification ordering, diagnosis, and cleanup tests.

package workflowverify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/verification"
)

func TestRunnerReportsEarliestInvariantAndBoundsCorrelatedDiagnostics(t *testing.T) {
	fake := &fakeExecutor{checks: map[string]CheckResult{
		"ready": {Passed: true}, "visible": {Passed: false, Reason: "checkout total missing"},
	}}
	fake.diagnostics = DiagnosticBundle{
		DOM: evidenceRefs("dom", 3), Network: evidenceRefs("network", 3), Console: evidenceRefs("console", 3),
		Doctor: evidenceRefs("doctor", 3), Screenshots: evidenceRefs("screenshot", 3),
	}
	fake.diagnostics.DOM = append([]verification.EvidenceRef{{ID: "sha256:foreign", Kind: "dom", CorrelationID: "other-workflow", CapturedAt: time.Now()}}, fake.diagnostics.DOM...)
	runner := Runner{Executor: fake, CleanupTimeout: time.Second, DiagnosticTimeout: time.Second, MaxDiagnosticsPerKind: 2}
	report, err := runner.Run(context.Background(), validWorkflow())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Verdict != verification.VerdictFail || report.FirstFailure.InvariantID != "visible" {
		t.Fatalf("unexpected first failure: %#v", report)
	}
	if len(fake.executed) != 1 || fake.executed[0] != "checkout" {
		t.Fatalf("executed steps = %v", fake.executed)
	}
	if report.Diagnostics.CorrelationID != "workflow-123" || len(report.Diagnostics.DOM) != 2 || len(report.Diagnostics.Screenshots) != 2 {
		t.Fatalf("unbounded or uncorrelated diagnostics: %#v", report.Diagnostics)
	}
	for _, ref := range report.Diagnostics.DOM {
		if ref.CorrelationID != "workflow-123" {
			t.Fatalf("foreign evidence included in diagnostics: %#v", ref)
		}
	}
	if len(fake.cleaned) != 2 || fake.cleaned[0] != "reset_cart" || fake.cleaned[1] != "restore_session" {
		t.Fatalf("cleanup order = %v", fake.cleaned)
	}
}

func TestRunnerHandlesPartialNavigationReconnectAndCancellation(t *testing.T) {
	tests := []struct {
		name       string
		executeErr error
		check      CheckResult
		cancel     bool
	}{
		{name: "partial navigation", executeErr: errors.New("partial navigation")},
		{name: "extension reconnect", check: CheckResult{Passed: false, Reason: "extension reconnecting"}},
		{name: "cancellation", cancel: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := validWorkflow()
			fake := &fakeExecutor{executeErr: tt.executeErr, checks: map[string]CheckResult{"ready": {Passed: true}, "visible": tt.check}}
			ctx := context.Background()
			if tt.cancel {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			report, err := (Runner{Executor: fake, CleanupTimeout: time.Second, DiagnosticTimeout: time.Second}).Run(ctx, workflow)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if report.Verdict != verification.VerdictFail || report.FirstFailure == nil {
				t.Fatalf("runtime fault was not reported: %#v", report)
			}
			if len(fake.cleaned) != 2 || fake.cleanupSawCancelledContext {
				t.Fatalf("interruption-safe cleanup failed: cleaned=%v cancelled_context=%v", fake.cleaned, fake.cleanupSawCancelledContext)
			}
		})
	}
}

func TestRunnerReportsCleanupFailureWithoutDiscardingCleanupAttempts(t *testing.T) {
	fake := &fakeExecutor{checks: map[string]CheckResult{"ready": {Passed: true}, "visible": {Passed: true}}, cleanupErr: map[string]error{"reset_cart": errors.New("cart API unavailable")}}
	report, err := (Runner{Executor: fake, CleanupTimeout: time.Second}).Run(context.Background(), validWorkflow())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Verdict != verification.VerdictFail || len(report.CleanupFailures) != 1 || report.FirstFailure.Stage != "cleanup" {
		t.Fatalf("cleanup failure report = %#v", report)
	}
	if len(fake.cleaned) != 2 {
		t.Fatalf("cleanup stopped after first failure: %v", fake.cleaned)
	}
}

func TestRunnerRejectsIncompleteWorkflow(t *testing.T) {
	workflow := validWorkflow()
	workflow.Cleanup = nil
	if _, err := (Runner{Executor: &fakeExecutor{}}).Run(context.Background(), workflow); err == nil {
		t.Fatal("workflow without cleanup should fail validation")
	}
}

func validWorkflow() Workflow {
	return Workflow{
		ID: "checkout-flow", CorrelationID: "workflow-123",
		Preconditions: []Invariant{{ID: "ready", Description: "checkout is ready"}},
		Steps:         []Step{{ID: "checkout", Description: "submit checkout", Invariants: []Invariant{{ID: "visible", Description: "total is visible"}}}},
		Cleanup:       []CleanupStep{{ID: "restore_session"}, {ID: "reset_cart"}},
	}
}

func evidenceRefs(kind string, count int) []verification.EvidenceRef {
	refs := make([]verification.EvidenceRef, count)
	for index := range refs {
		refs[index] = verification.EvidenceRef{ID: "sha256:" + kind, Kind: kind, CorrelationID: "workflow-123", CapturedAt: time.Now()}
	}
	return refs
}

type fakeExecutor struct {
	checks                     map[string]CheckResult
	executeErr                 error
	cleanupErr                 map[string]error
	diagnostics                DiagnosticBundle
	executed                   []string
	cleaned                    []string
	cleanupSawCancelledContext bool
}

func (f *fakeExecutor) Check(_ context.Context, invariant Invariant) (CheckResult, error) {
	return f.checks[invariant.ID], nil
}

func (f *fakeExecutor) Execute(ctx context.Context, step Step) error {
	f.executed = append(f.executed, step.ID)
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.executeErr
}

func (f *fakeExecutor) Diagnose(_ context.Context, _ Failure, _ int) DiagnosticBundle {
	return f.diagnostics
}

func (f *fakeExecutor) Cleanup(ctx context.Context, step CleanupStep) error {
	f.cleaned = append(f.cleaned, step.ID)
	f.cleanupSawCancelledContext = f.cleanupSawCancelledContext || ctx.Err() != nil
	return f.cleanupErr[step.ID]
}

// runner_test.go — Deterministic flaky-workflow reproduction tests.

package flakerepro

import (
	"context"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/verification"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/workflowverify"
)

func TestRunnerSeparatesReproducedNonReproducedAndEnvironmentCorrelated(t *testing.T) {
	original := failedReport("workflow-123", "visible")
	executor := &fakeExecutor{reports: []workflowverify.Report{
		failedReport("ignored", "visible"),
		{Verdict: verification.VerdictPass},
		failedReport("ignored", "visible"),
	}}
	plan := Plan{
		CorrelationID: "repro-123", Segment: "checkout/submit", Original: original, MaxAttempts: 3,
		Attempts: []Perturbation{
			{Name: "baseline", CacheState: "preserve", TabLifecycle: "none"},
			{Name: "cold-cache", CacheState: "cold", TabLifecycle: "none"},
			{Name: "reconnect", CacheState: "preserve", TabLifecycle: "reconnect"},
		},
	}
	report, err := (Runner{Executor: executor}).Run(context.Background(), plan)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Verdict != verification.VerdictFlaky {
		t.Fatalf("verdict = %q, want FLAKY", report.Verdict)
	}
	if len(report.Reproduced) != 1 || len(report.NonReproduced) != 1 || len(report.EnvironmentCorrelated) != 1 {
		t.Fatalf("outcome buckets = reproduced:%d non:%d correlated:%d", len(report.Reproduced), len(report.NonReproduced), len(report.EnvironmentCorrelated))
	}
	for index, attempt := range executor.attempts {
		want := []string{"repro-123/retry-001", "repro-123/retry-002", "repro-123/retry-003"}[index]
		if attempt.CorrelationID != want || attempt.Perturbation.Name != plan.Attempts[index].Name {
			t.Fatalf("attempt[%d] = %#v, want correlation %q", index, attempt, want)
		}
	}
	if report.Original.FirstFailure.InvariantID != "visible" {
		t.Fatalf("original failure changed: %#v", report.Original)
	}
}

func TestRunnerCancellationIsBoundedAndCannotTurnOriginalFailureGreen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	executor := &fakeExecutor{reports: []workflowverify.Report{{Verdict: verification.VerdictPass}}, afterRun: cancel}
	plan := Plan{
		CorrelationID: "repro-123", Segment: "checkout/submit", Original: failedReport("workflow-123", "visible"), MaxAttempts: 3,
		Attempts: []Perturbation{{Name: "baseline", CacheState: "preserve", TabLifecycle: "none"}, {Name: "again", CacheState: "warm", TabLifecycle: "reload"}, {Name: "third", CacheState: "cold", TabLifecycle: "reconnect"}},
	}
	report, err := (Runner{Executor: executor}).Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.Cancelled || report.AttemptsRun != 1 || report.Verdict == verification.VerdictPass {
		t.Fatalf("cancellation report = %#v", report)
	}
}

func TestRunnerRejectsUnboundedOrImplicitPerturbations(t *testing.T) {
	valid := Plan{
		CorrelationID: "repro-123", Segment: "checkout/submit", Original: failedReport("workflow-123", "visible"), MaxAttempts: 1,
		Attempts: []Perturbation{{Name: "baseline", CacheState: "preserve", TabLifecycle: "none"}},
	}
	tests := []Plan{
		{CorrelationID: valid.CorrelationID, Segment: valid.Segment, Original: valid.Original, MaxAttempts: MaxAttempts + 1, Attempts: valid.Attempts},
		{CorrelationID: valid.CorrelationID, Segment: valid.Segment, Original: valid.Original, MaxAttempts: 1, Attempts: []Perturbation{{Name: "", CacheState: "preserve", TabLifecycle: "none"}}},
		{CorrelationID: valid.CorrelationID, Segment: valid.Segment, Original: valid.Original, MaxAttempts: 1, Attempts: []Perturbation{{Name: "bad-cache", CacheState: "random", TabLifecycle: "none"}}},
	}
	for _, plan := range tests {
		if _, err := (Runner{Executor: &fakeExecutor{}}).Run(context.Background(), plan); err == nil {
			t.Fatalf("expected invalid plan: %#v", plan)
		}
	}
}

func failedReport(correlationID, invariantID string) workflowverify.Report {
	return workflowverify.Report{
		WorkflowID: "checkout", CorrelationID: correlationID, Verdict: verification.VerdictFail,
		FirstFailure: &workflowverify.Failure{Stage: "invariant", StepID: "checkout", InvariantID: invariantID, Reason: "not visible"},
	}
}

type fakeExecutor struct {
	reports  []workflowverify.Report
	attempts []Attempt
	afterRun func()
}

func (f *fakeExecutor) Run(_ context.Context, attempt Attempt) workflowverify.Report {
	f.attempts = append(f.attempts, attempt)
	index := len(f.attempts) - 1
	report := f.reports[index]
	if f.afterRun != nil {
		f.afterRun()
	}
	return report
}

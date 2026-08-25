// runner.go — Ordered workflow assertions, first-failure diagnosis, and interruption-safe cleanup.
// Docs: docs/features/feature/workflow-verification/index.md

package workflowverify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/verification"
)

type Workflow struct {
	ID            string        `json:"workflow_id"`
	CorrelationID string        `json:"correlation_id"`
	Preconditions []Invariant   `json:"preconditions"`
	Steps         []Step        `json:"steps"`
	Cleanup       []CleanupStep `json:"cleanup"`
}

type Step struct {
	ID          string      `json:"step_id"`
	Description string      `json:"description"`
	Invariants  []Invariant `json:"invariants"`
}

type Invariant struct {
	ID          string `json:"invariant_id"`
	Description string `json:"description"`
}

type CleanupStep struct {
	ID string `json:"cleanup_id"`
}

type CheckResult struct {
	Passed   bool                       `json:"passed"`
	Reason   string                     `json:"reason,omitempty"`
	Evidence []verification.EvidenceRef `json:"evidence,omitempty"`
}

type CheckRecord struct {
	Stage       string      `json:"stage"`
	StepID      string      `json:"step_id,omitempty"`
	InvariantID string      `json:"invariant_id"`
	Result      CheckResult `json:"result"`
}

type Failure struct {
	Stage       string `json:"stage"`
	StepID      string `json:"step_id,omitempty"`
	InvariantID string `json:"invariant_id,omitempty"`
	Reason      string `json:"reason"`
}

type CleanupFailure struct {
	CleanupID string `json:"cleanup_id"`
	Reason    string `json:"reason"`
}

type DiagnosticBundle struct {
	CorrelationID string                     `json:"correlation_id"`
	DOM           []verification.EvidenceRef `json:"dom,omitempty"`
	Network       []verification.EvidenceRef `json:"network,omitempty"`
	Console       []verification.EvidenceRef `json:"console,omitempty"`
	Doctor        []verification.EvidenceRef `json:"doctor,omitempty"`
	Screenshots   []verification.EvidenceRef `json:"screenshots,omitempty"`
}

type Report struct {
	WorkflowID      string               `json:"workflow_id"`
	CorrelationID   string               `json:"correlation_id"`
	Verdict         verification.Verdict `json:"verdict"`
	Checks          []CheckRecord        `json:"checks"`
	FirstFailure    *Failure             `json:"first_failure,omitempty"`
	Diagnostics     DiagnosticBundle     `json:"diagnostics"`
	CleanupFailures []CleanupFailure     `json:"cleanup_failures,omitempty"`
}

type Executor interface {
	Check(context.Context, Invariant) (CheckResult, error)
	Execute(context.Context, Step) error
	Diagnose(context.Context, Failure, int) DiagnosticBundle
	Cleanup(context.Context, CleanupStep) error
}

type Runner struct {
	Executor              Executor
	CleanupTimeout        time.Duration
	DiagnosticTimeout     time.Duration
	MaxDiagnosticsPerKind int
}

func (r Runner) Run(ctx context.Context, workflow Workflow) (report Report, err error) {
	if err := validateWorkflow(workflow, r.Executor); err != nil {
		return Report{}, err
	}
	report = Report{WorkflowID: workflow.ID, CorrelationID: workflow.CorrelationID, Verdict: verification.VerdictPass}
	defer func() {
		r.runCleanup(workflow, &report)
	}()

	for _, invariant := range workflow.Preconditions {
		if failure := r.checkInvariant(ctx, "precondition", "", invariant, &report); failure != nil {
			r.fail(workflow, &report, *failure)
			return report, nil
		}
	}
	for _, step := range workflow.Steps {
		if err := r.Executor.Execute(ctx, step); err != nil {
			r.fail(workflow, &report, Failure{Stage: "step", StepID: step.ID, InvariantID: step.ID + ".execution", Reason: err.Error()})
			return report, nil
		}
		for _, invariant := range step.Invariants {
			if failure := r.checkInvariant(ctx, "invariant", step.ID, invariant, &report); failure != nil {
				r.fail(workflow, &report, *failure)
				return report, nil
			}
		}
	}
	return report, nil
}

func (r Runner) checkInvariant(ctx context.Context, stage, stepID string, invariant Invariant, report *Report) *Failure {
	result, err := r.Executor.Check(ctx, invariant)
	if err != nil {
		result = CheckResult{Passed: false, Reason: err.Error()}
	}
	report.Checks = append(report.Checks, CheckRecord{Stage: stage, StepID: stepID, InvariantID: invariant.ID, Result: result})
	if result.Passed {
		return nil
	}
	reason := strings.TrimSpace(result.Reason)
	if reason == "" {
		reason = "invariant was not satisfied"
	}
	return &Failure{Stage: stage, StepID: stepID, InvariantID: invariant.ID, Reason: reason}
}

func (r Runner) fail(workflow Workflow, report *Report, failure Failure) {
	report.Verdict = verification.VerdictFail
	if report.FirstFailure != nil {
		return
	}
	report.FirstFailure = &failure
	report.Diagnostics = r.diagnose(workflow.CorrelationID, failure)
}

func (r Runner) diagnose(correlationID string, failure Failure) DiagnosticBundle {
	timeout := r.DiagnosticTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	limit := r.MaxDiagnosticsPerKind
	if limit <= 0 {
		limit = 5
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	bundle := r.Executor.Diagnose(ctx, failure, limit)
	bundle.CorrelationID = correlationID
	bundle.DOM = correlated(bundle.DOM, correlationID, limit)
	bundle.Network = correlated(bundle.Network, correlationID, limit)
	bundle.Console = correlated(bundle.Console, correlationID, limit)
	bundle.Doctor = correlated(bundle.Doctor, correlationID, limit)
	bundle.Screenshots = correlated(bundle.Screenshots, correlationID, limit)
	return bundle
}

func (r Runner) runCleanup(workflow Workflow, report *Report) {
	timeout := r.CleanupTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for index := len(workflow.Cleanup) - 1; index >= 0; index-- {
		step := workflow.Cleanup[index]
		if err := r.Executor.Cleanup(ctx, step); err != nil {
			report.CleanupFailures = append(report.CleanupFailures, CleanupFailure{CleanupID: step.ID, Reason: err.Error()})
			if report.FirstFailure == nil {
				r.fail(workflow, report, Failure{Stage: "cleanup", StepID: step.ID, Reason: err.Error()})
			} else {
				report.Verdict = verification.VerdictFail
			}
		}
	}
}

func validateWorkflow(workflow Workflow, executor Executor) error {
	if executor == nil {
		return fmt.Errorf("workflow executor is required")
	}
	if strings.TrimSpace(workflow.ID) == "" || strings.TrimSpace(workflow.CorrelationID) == "" {
		return fmt.Errorf("workflow_id and correlation_id are required")
	}
	if len(workflow.Preconditions) == 0 || len(workflow.Steps) == 0 || len(workflow.Cleanup) == 0 {
		return fmt.Errorf("workflow requires preconditions, steps, and cleanup")
	}
	seen := make(map[string]bool)
	if err := validateInvariants(workflow.Preconditions, seen); err != nil {
		return err
	}
	if err := validateSteps(workflow.Steps, seen); err != nil {
		return err
	}
	return validateCleanupSteps(workflow.Cleanup)
}

func validateInvariants(invariants []Invariant, seen map[string]bool) error {
	for _, invariant := range invariants {
		if err := validateInvariant(invariant, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateSteps(steps []Step, seen map[string]bool) error {
	for _, step := range steps {
		if strings.TrimSpace(step.ID) == "" || strings.TrimSpace(step.Description) == "" || len(step.Invariants) == 0 {
			return fmt.Errorf("every step requires step_id, description, and invariants")
		}
		if err := validateInvariants(step.Invariants, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateCleanupSteps(steps []CleanupStep) error {
	for _, step := range steps {
		if strings.TrimSpace(step.ID) == "" {
			return fmt.Errorf("every cleanup step requires cleanup_id")
		}
	}
	return nil
}

func validateInvariant(invariant Invariant, seen map[string]bool) error {
	if strings.TrimSpace(invariant.ID) == "" || strings.TrimSpace(invariant.Description) == "" {
		return fmt.Errorf("every invariant requires invariant_id and description")
	}
	if seen[invariant.ID] {
		return fmt.Errorf("duplicate invariant_id %q", invariant.ID)
	}
	seen[invariant.ID] = true
	return nil
}

func correlated(refs []verification.EvidenceRef, correlationID string, limit int) []verification.EvidenceRef {
	filtered := make([]verification.EvidenceRef, 0, min(len(refs), limit))
	for _, ref := range refs {
		if ref.CorrelationID == correlationID {
			filtered = append(filtered, ref)
			if len(filtered) == limit {
				break
			}
		}
	}
	return filtered
}

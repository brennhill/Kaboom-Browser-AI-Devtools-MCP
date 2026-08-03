// runner.go — Bounded deterministic reproduction of flaky workflow failures.
// Docs: docs/features/feature/flaky-reproduction/index.md

package flakerepro

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/verification"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/workflowverify"
)

const MaxAttempts = 20

type Perturbation struct {
	Name               string `json:"name"`
	LatencyMS          int    `json:"latency_ms"`
	CPUPressurePercent int    `json:"cpu_pressure_percent"`
	CacheState         string `json:"cache_state"`
	TabLifecycle       string `json:"tab_lifecycle"`
}

type Plan struct {
	CorrelationID string                `json:"correlation_id"`
	Segment       string                `json:"segment"`
	Original      workflowverify.Report `json:"original_failure"`
	MaxAttempts   int                   `json:"max_attempts"`
	Attempts      []Perturbation        `json:"attempts"`
}

type Attempt struct {
	Index         int          `json:"index"`
	CorrelationID string       `json:"correlation_id"`
	Segment       string       `json:"segment"`
	Perturbation  Perturbation `json:"perturbation"`
}

type Outcome string

const (
	OutcomeReproduced            Outcome = "reproduced"
	OutcomeNonReproduced         Outcome = "non_reproduced"
	OutcomeEnvironmentCorrelated Outcome = "environment_correlated"
)

type AttemptResult struct {
	Attempt Attempt               `json:"attempt"`
	Outcome Outcome               `json:"outcome"`
	Report  workflowverify.Report `json:"report"`
}

type Distribution struct {
	Count int     `json:"count"`
	Rate  float64 `json:"rate"`
}

type Report struct {
	CorrelationID         string                   `json:"correlation_id"`
	Original              workflowverify.Report    `json:"original_failure"`
	Verdict               verification.Verdict     `json:"verdict"`
	AttemptsRun           int                      `json:"attempts_run"`
	Cancelled             bool                     `json:"cancelled"`
	Reproduced            []AttemptResult          `json:"reproduced"`
	NonReproduced         []AttemptResult          `json:"non_reproduced"`
	EnvironmentCorrelated []AttemptResult          `json:"environment_correlated"`
	Distributions         map[Outcome]Distribution `json:"distributions"`
}

type Executor interface {
	Run(context.Context, Attempt) workflowverify.Report
}

type Runner struct {
	Executor Executor
}

func (r Runner) Run(ctx context.Context, plan Plan) (Report, error) {
	if err := validatePlan(plan, r.Executor); err != nil {
		return Report{}, err
	}
	original, err := cloneReport(plan.Original)
	if err != nil {
		return Report{}, err
	}
	report := Report{CorrelationID: plan.CorrelationID, Original: original}
	for index, perturbation := range plan.Attempts {
		if index >= plan.MaxAttempts {
			break
		}
		if ctx.Err() != nil {
			report.Cancelled = true
			break
		}
		attempt := Attempt{
			Index: index + 1, CorrelationID: fmt.Sprintf("%s/retry-%03d", plan.CorrelationID, index+1),
			Segment: plan.Segment, Perturbation: perturbation,
		}
		attemptReport := r.Executor.Run(ctx, attempt)
		result := AttemptResult{Attempt: attempt, Report: attemptReport}
		sameFailure := failureMatches(original.FirstFailure, attemptReport.FirstFailure)
		switch {
		case sameFailure && isBaseline(perturbation):
			result.Outcome = OutcomeReproduced
			report.Reproduced = append(report.Reproduced, result)
		case sameFailure:
			result.Outcome = OutcomeEnvironmentCorrelated
			report.EnvironmentCorrelated = append(report.EnvironmentCorrelated, result)
		default:
			result.Outcome = OutcomeNonReproduced
			report.NonReproduced = append(report.NonReproduced, result)
		}
		report.AttemptsRun++
	}
	report.Verdict = reproductionVerdict(report)
	report.Distributions = distributions(report)
	return report, nil
}

func validatePlan(plan Plan, executor Executor) error {
	if executor == nil {
		return fmt.Errorf("reproduction executor is required")
	}
	if strings.TrimSpace(plan.CorrelationID) == "" || strings.TrimSpace(plan.Segment) == "" {
		return fmt.Errorf("correlation_id and segment are required")
	}
	if plan.Original.Verdict != verification.VerdictFail || plan.Original.FirstFailure == nil {
		return fmt.Errorf("original_failure must retain a failed workflow report")
	}
	if plan.MaxAttempts <= 0 || plan.MaxAttempts > MaxAttempts || len(plan.Attempts) == 0 || len(plan.Attempts) > plan.MaxAttempts {
		return fmt.Errorf("attempts must contain 1..max_attempts entries and max_attempts cannot exceed %d", MaxAttempts)
	}
	for _, perturbation := range plan.Attempts {
		if err := validatePerturbation(perturbation); err != nil {
			return err
		}
	}
	return nil
}

func validatePerturbation(perturbation Perturbation) error {
	if strings.TrimSpace(perturbation.Name) == "" {
		return fmt.Errorf("every perturbation requires an explicit name")
	}
	if perturbation.LatencyMS < 0 || perturbation.LatencyMS > 60000 || perturbation.CPUPressurePercent < 0 || perturbation.CPUPressurePercent > 100 {
		return fmt.Errorf("perturbation pressure is outside supported bounds")
	}
	if perturbation.CacheState != "preserve" && perturbation.CacheState != "cold" && perturbation.CacheState != "warm" {
		return fmt.Errorf("cache_state must be preserve, cold, or warm")
	}
	if perturbation.TabLifecycle != "none" && perturbation.TabLifecycle != "reload" && perturbation.TabLifecycle != "reconnect" && perturbation.TabLifecycle != "navigate" {
		return fmt.Errorf("tab_lifecycle must be none, reload, reconnect, or navigate")
	}
	return nil
}

func cloneReport(input workflowverify.Report) (workflowverify.Report, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return workflowverify.Report{}, fmt.Errorf("copy original failure: %w", err)
	}
	var cloned workflowverify.Report
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return workflowverify.Report{}, fmt.Errorf("copy original failure: %w", err)
	}
	return cloned, nil
}

func failureMatches(original, attempted *workflowverify.Failure) bool {
	return original != nil && attempted != nil && original.Stage == attempted.Stage &&
		original.StepID == attempted.StepID && original.InvariantID == attempted.InvariantID
}

func isBaseline(perturbation Perturbation) bool {
	return perturbation.LatencyMS == 0 && perturbation.CPUPressurePercent == 0 &&
		perturbation.CacheState == "preserve" && perturbation.TabLifecycle == "none"
}

func reproductionVerdict(report Report) verification.Verdict {
	if report.Cancelled {
		return verification.VerdictBlocked
	}
	reproduced := len(report.Reproduced) + len(report.EnvironmentCorrelated)
	if reproduced == report.AttemptsRun {
		return verification.VerdictFail
	}
	if reproduced > 0 {
		return verification.VerdictFlaky
	}
	return verification.VerdictUnverified
}

func distributions(report Report) map[Outcome]Distribution {
	result := make(map[Outcome]Distribution, 3)
	counts := map[Outcome]int{
		OutcomeReproduced: len(report.Reproduced), OutcomeNonReproduced: len(report.NonReproduced),
		OutcomeEnvironmentCorrelated: len(report.EnvironmentCorrelated),
	}
	for outcome, count := range counts {
		rate := 0.0
		if report.AttemptsRun > 0 {
			rate = float64(count) / float64(report.AttemptsRun)
		}
		result[outcome] = Distribution{Count: count, Rate: rate}
	}
	return result
}

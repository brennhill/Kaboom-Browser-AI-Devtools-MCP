// classify_test.go — Contracts for turning an effect window into an outcome the
// caller can act on, including the retry advice that outcome implies.
// Docs: docs/features/feature/effect-verification/index.md

package actioneffects

import (
	"strings"
	"testing"
)

func TestClassifyNamesTheThreeOutcomes(t *testing.T) {
	observed := Effects{Evaluated: true, NetworkRequestCount: 1}
	silent := Effects{Evaluated: true}

	tests := []struct {
		name    string
		failed  bool
		effects Effects
		dom     DOMChange
		want    string
	}{
		{"a failed dispatch", true, silent, DOMUnknown, OutcomeError},
		{"telemetry arrived", false, observed, DOMUnknown, OutcomeObserved},
		{"the DOM changed and nothing else did", false, silent, DOMChanged, OutcomeObserved},
		{"nothing at all", false, silent, DOMUnchanged, OutcomeNoEffect},
		{"nothing, and no DOM report either", false, silent, DOMUnknown, OutcomeNoEffect},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			effects := tc.effects
			effects.DOM = tc.dom
			if got := Classify(tc.failed, effects); got != tc.want {
				t.Fatalf("outcome = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyReportsAnUnevaluatedWindowRatherThanClaimingNoEffect(t *testing.T) {
	// kaboom-knms was a no-effect action reported as a success. Reporting an
	// unrun window as a no-effect action is the same lie with the sign flipped.
	if got := Classify(false, Effects{Evaluated: false}); got != OutcomeNotEvaluated {
		t.Fatalf("outcome = %q, want %q", got, OutcomeNotEvaluated)
	}
}

func TestPayloadCarriesTheOutcomeAndItsEvidence(t *testing.T) {
	effects := Effects{
		Evaluated:           true,
		Attribution:         AttributionTemporalWindow,
		WindowMs:            50,
		ClosedEarly:         true,
		NetworkRequests:     []NetworkEffect{{URL: "https://api.example/x", Status: 500}},
		NetworkRequestCount: 1,
		ConsoleErrors:       []string{"TypeError: undefined"},
		ConsoleErrorCount:   1,
		Navigation:          &NavigationEffect{FromURL: "https://a/", ToURL: "https://b/"},
		DOM:                 DOMChanged,
	}
	effects.Outcome = Classify(false, effects)

	payload := effects.Payload()

	for key, want := range map[string]any{
		"outcome":               OutcomeObserved,
		"attribution":           AttributionTemporalWindow,
		"window_ms":             50,
		"closed_early":          true,
		"network_request_count": 1,
		"console_error_count":   1,
		"dom_changed":           true,
	} {
		if payload[key] != want {
			t.Errorf("%s = %#v, want %#v", key, payload[key], want)
		}
	}
	if _, ok := payload["network_requests"]; !ok {
		t.Error("network_requests missing from the evidence")
	}
	if _, ok := payload["navigation"]; !ok {
		t.Error("navigation missing from the evidence")
	}
}

func TestPayloadOmitsEmptyEvidenceRatherThanSpendingTokensOnZeroes(t *testing.T) {
	effects := Effects{Evaluated: true, Attribution: AttributionTemporalWindow, WindowMs: 300, DOM: DOMUnchanged}
	effects.Outcome = Classify(false, effects)

	payload := effects.Payload()

	for _, key := range []string{"network_requests", "console_errors", "navigation", "transients"} {
		if _, present := payload[key]; present {
			t.Errorf("%s present with nothing to report", key)
		}
	}
	if payload["network_request_count"] != 0 || payload["console_error_count"] != 0 {
		t.Error("counts must stay present so a zero is stated, not inferred from an absence")
	}
}

// ---------------------------------------------------------------------------
// Retry advice
// ---------------------------------------------------------------------------

func TestApplyRetryAdviceStopsARetryOfAnActionThatChangedNothing(t *testing.T) {
	// Re-issuing the identical action against an unchanged page reproduces the
	// same nothing. The advice has to say so, or the agent burns its budget.
	data := map[string]any{"success": true}

	ApplyRetryAdvice(data, OutcomeNoEffect)

	if data["retryable"] != false {
		t.Fatalf("retryable = %#v, want false", data["retryable"])
	}
	advice, _ := data["retry"].(string)
	if !strings.Contains(advice, "same") || advice == "" {
		t.Fatalf("retry advice = %q; it must say a repeat produces the same result", advice)
	}
}

func TestApplyRetryAdviceLeavesOtherOutcomesToTheExistingRetryPolicy(t *testing.T) {
	for _, outcome := range []string{OutcomeObserved, OutcomeError, OutcomeNotEvaluated} {
		data := map[string]any{}
		ApplyRetryAdvice(data, outcome)
		if len(data) != 0 {
			t.Fatalf("%s wrote retry fields: %#v", outcome, data)
		}
	}
}

func TestApplyRetryAdviceNeverOverridesAnExplicitDecision(t *testing.T) {
	data := map[string]any{"retryable": true, "retry": "caller knows better"}

	ApplyRetryAdvice(data, OutcomeNoEffect)

	if data["retryable"] != true || data["retry"] != "caller knows better" {
		t.Fatalf("existing retry decision overwritten: %#v", data)
	}
}

// ---------------------------------------------------------------------------
// Reading the DOM report the extension already sends
// ---------------------------------------------------------------------------

func TestDOMChangeReadsTheSummaryEveryDOMPrimitiveAlreadyReturns(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want DOMChange
	}{
		{"no summary at all", map[string]any{"success": true}, DOMUnknown},
		{"the page did not move", map[string]any{"dom_summary": "no DOM changes"}, DOMUnchanged},
		{"nodes were added", map[string]any{"dom_summary": "3 added, 1 removed"}, DOMChanged},
		{"counts instead of prose", map[string]any{"dom_changes": map[string]any{"added": float64(2)}}, DOMChanged},
		{"counts, all zero", map[string]any{"dom_changes": map[string]any{"added": float64(0), "removed": float64(0), "modified": float64(0)}}, DOMUnchanged},
		{"nested under an async result", map[string]any{"result": map[string]any{"dom_summary": "1 added"}}, DOMChanged},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DOMChangeFrom(tc.data); got != tc.want {
				t.Fatalf("dom change = %v, want %v", got, tc.want)
			}
		})
	}
}

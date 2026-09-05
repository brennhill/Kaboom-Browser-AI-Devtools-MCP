// classify.go — Turns an effect window into the outcome a caller can act on, and
// the retry advice that outcome implies.
// Docs: docs/features/feature/effect-verification/index.md

package actioneffects

import "strings"

// The outcomes that replace a bare dispatch success. An action that ran is not
// an action that did something, and the difference is what these name.
const (
	// OutcomeObserved: the action dispatched and telemetry followed it.
	OutcomeObserved = "dispatched_and_observed_effect"
	// OutcomeNoEffect: the action dispatched, the window ran, and nothing moved.
	OutcomeNoEffect = "dispatched_and_no_observable_effect"
	// OutcomeError: the action itself failed.
	OutcomeError = "dispatched_then_error"
	// OutcomeNotEvaluated: no window ran, so nothing is claimed either way.
	OutcomeNotEvaluated = "not_evaluated"
)

// DOMChange is what the extension's own mutation report says about the action.
// Every DOM primitive already installs a MutationObserver before it runs and
// returns a summary; this reads that report rather than asking for a new one.
type DOMChange int

const (
	// DOMUnknown: the response carried no mutation report.
	DOMUnknown DOMChange = iota
	// DOMUnchanged: the report ran and saw nothing move.
	DOMUnchanged
	// DOMChanged: the report saw the document change.
	DOMChanged
)

// noChangeSummary is the literal the DOM primitives emit for a still document.
const noChangeSummary = "no DOM changes"

// retryAdviceNoEffect is handed to a caller whose action provably did nothing.
const retryAdviceNoEffect = "The action dispatched but nothing observable followed it. Repeating it against an unchanged page produces the same result — re-target the element (element_id/selector/index/frame), or fix the precondition (scroll it into view, dismiss a blocking overlay, wait for the control to enable) before acting again."

// Classify names what the window saw. Order matters: a failed dispatch is not a
// no-effect action, and an unrun window is neither. The DOM report travels on
// Effects so the outcome and the evidence behind it cannot disagree.
func Classify(dispatchFailed bool, effects Effects) string {
	if dispatchFailed {
		return OutcomeError
	}
	if !effects.Evaluated {
		return OutcomeNotEvaluated
	}
	if effects.observed() || effects.DOM == DOMChanged {
		return OutcomeObserved
	}
	return OutcomeNoEffect
}

// Payload renders the effects block that rides on the action's own response.
// Counts are always present so a zero is stated rather than inferred from an
// absent key; listings are omitted when there is nothing in them.
func (e Effects) Payload() map[string]any {
	payload := map[string]any{
		"outcome":               e.Outcome,
		"attribution":           e.Attribution,
		"attribution_note":      attributionNote,
		"window_ms":             e.WindowMs,
		"closed_early":          e.ClosedEarly,
		"network_request_count": e.NetworkRequestCount,
		"console_error_count":   e.ConsoleErrorCount,
		"console_warning_count": e.ConsoleWarningCount,
	}
	if e.DOM != DOMUnknown {
		payload["dom_changed"] = e.DOM == DOMChanged
	}
	if len(e.NetworkRequests) > 0 {
		payload["network_requests"] = e.NetworkRequests
	}
	if len(e.ConsoleErrors) > 0 {
		payload["console_errors"] = e.ConsoleErrors
	}
	if e.Navigation != nil {
		payload["navigation"] = e.Navigation
	}
	if e.RecordedActionCount > 0 {
		payload["recorded_action_count"] = e.RecordedActionCount
	}
	if len(e.Transients) > 0 {
		payload["transients"] = e.Transients
	}
	return payload
}

// ApplyRetryAdvice stops a retry that cannot succeed. Every other outcome is
// left to the existing retry policy, and an explicit decision already on the
// response is never overwritten.
func ApplyRetryAdvice(data map[string]any, outcome string) {
	if data == nil || outcome != OutcomeNoEffect {
		return
	}
	if _, decided := data["retryable"]; decided {
		return
	}
	if _, decided := data["retry"]; decided {
		return
	}
	data["retryable"] = false
	data["retry"] = retryAdviceNoEffect
}

// DOMChangeFrom reads the mutation report the extension already returns, at the
// top level or nested under the async lifecycle envelope's result.
func DOMChangeFrom(data map[string]any) DOMChange {
	if change := domChangeAtLevel(data); change != DOMUnknown {
		return change
	}
	if inner, ok := data["result"].(map[string]any); ok {
		return domChangeAtLevel(inner)
	}
	return DOMUnknown
}

func domChangeAtLevel(data map[string]any) DOMChange {
	if counts, ok := data["dom_changes"].(map[string]any); ok {
		if change := domChangeFromCounts(counts); change != DOMUnknown {
			return change
		}
	}
	summary, ok := data["dom_summary"].(string)
	if !ok {
		return DOMUnknown
	}
	if strings.TrimSpace(summary) == "" || strings.Contains(summary, noChangeSummary) {
		return DOMUnchanged
	}
	return DOMChanged
}

func domChangeFromCounts(counts map[string]any) DOMChange {
	seen := false
	for _, key := range []string{"added", "removed", "modified"} {
		value, ok := counts[key].(float64)
		if !ok {
			continue
		}
		seen = true
		if value > 0 {
			return DOMChanged
		}
	}
	if seen {
		return DOMUnchanged
	}
	return DOMUnknown
}

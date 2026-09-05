// effects.go — The effect window: telemetry recorded between an action's dispatch
// and the moment it settles, attributed to that action.
// Docs: docs/features/feature/effect-verification/index.md

// Package actioneffects answers a question dispatch success cannot: did the
// action do anything? It takes a high-water mark across the capture buffers
// before an action runs, reads what arrives after it, and reports the result as
// evidence rather than as proof — entries land in the window because they were
// recorded inside it, not because they were shown to be caused by the action.
package actioneffects

import (
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// AttributionTemporalWindow names the only join this package performs.
const AttributionTemporalWindow = "temporal_window"

// attributionNote travels with every effects block. Point 2 of the design: a
// happens-after must never be handed to a caller spelled as a happens-because.
const attributionNote = "Entries are attributed to this action because they were recorded inside its window, not because they were proven to be caused by it."

// Evidence listings are capped so a chatty page cannot crowd out the response.
// The counts stay exact; only the listing is trimmed.
const (
	maxAttributedRequests = 10
	maxAttributedErrors   = 3
	maxErrorMessageChars  = 200
)

// Budget bounds one effect window. Poll is how often the buffers are re-read;
// Total is the longest the window may stay open with nothing to show for it.
type Budget struct {
	Total time.Duration
	Poll  time.Duration
}

// Deps names the capture readers the window needs. Every reader is optional; a
// window with none wired reports itself unevaluated rather than effect-free.
type Deps struct {
	Now        func() time.Time
	LogEntries func() ([]types.LogEntry, []time.Time)
	// NetworkRequests reads the captured request bodies. The waterfall is
	// deliberately not used here: nothing pushes it, so it is only ever
	// populated when observe pulls it, and a window reading it would report
	// "no requests" for a page that made several.
	NetworkRequests func() ([]types.NetworkBody, []time.Time)
	Actions         func() ([]types.EnhancedAction, []time.Time)
	TrackedURL      func() string
	Wait            func(time.Duration)
}

func (d Deps) hasReader() bool {
	return d.LogEntries != nil || d.NetworkRequests != nil || d.Actions != nil || d.TrackedURL != nil
}

// Mark is the high-water mark taken immediately before an action dispatches.
type Mark struct {
	At  time.Time
	URL string
}

// NetworkEffect is one request attributed to the window.
type NetworkEffect struct {
	URL        string `json:"url"`
	Method     string `json:"method,omitempty"`
	Status     int    `json:"status,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
}

// NavigationEffect records a tracked-tab URL that moved during the window.
type NavigationEffect struct {
	FromURL string `json:"from_url"`
	ToURL   string `json:"to_url"`
}

// Effects is what the window saw.
type Effects struct {
	Outcome             string
	Evaluated           bool
	Attribution         string
	WindowMs            int
	ClosedEarly         bool
	NetworkRequests     []NetworkEffect
	NetworkRequestCount int
	ConsoleErrors       []string
	ConsoleErrorCount   int
	ConsoleWarningCount int
	Navigation          *NavigationEffect
	RecordedActionCount int
	Transients          []string
	DOM                 DOMChange
}

// observed reports whether anything at all was attributed to the action. Kept
// unexported: a caller should read the classified outcome, not re-derive it.
func (e Effects) observed() bool {
	return e.NetworkRequestCount > 0 ||
		e.ConsoleErrorCount > 0 ||
		e.ConsoleWarningCount > 0 ||
		e.Navigation != nil ||
		e.RecordedActionCount > 0 ||
		len(e.Transients) > 0
}

// Open takes the mark the window will measure against.
func Open(deps Deps) Mark {
	mark := Mark{}
	if deps.Now != nil {
		mark.At = deps.Now()
	} else {
		mark.At = time.Now()
	}
	if deps.TrackedURL != nil {
		mark.URL = deps.TrackedURL()
	}
	return mark
}

// Collect polls the capture buffers until an effect appears or the budget runs
// out. It closes early on the first observed effect: a page that answered has
// nothing left to prove, and only a genuinely dead action pays the full budget.
//
// A dom verdict of DOMChanged answers the question before any polling starts.
// The extension's mutation report rides on the action's own response, so it
// costs nothing and is already in hand — waiting after it would be latency
// spent confirming something already known. Latency is therefore paid only by
// actions that appear to have done nothing, which is the case worth paying for.
func Collect(deps Deps, mark Mark, budget Budget, dom DOMChange) Effects {
	effects := Effects{Attribution: AttributionTemporalWindow, DOM: dom}
	if !deps.hasReader() || deps.Wait == nil || budget.Poll <= 0 {
		return effects
	}
	effects.Evaluated = true
	if dom == DOMChanged {
		effects.ClosedEarly = true
		return effects
	}

	var elapsed time.Duration
	for elapsed < budget.Total {
		deps.Wait(budget.Poll)
		elapsed += budget.Poll
		effects = snapshot(deps, mark, effects)
		if effects.observed() {
			effects.ClosedEarly = elapsed < budget.Total
			break
		}
	}
	effects.WindowMs = int(elapsed / time.Millisecond)
	return effects
}

// snapshot re-reads every buffer and rebuilds the attributed set from scratch,
// so a late-arriving batch of telemetry cannot be double counted.
func snapshot(deps Deps, mark Mark, prior Effects) Effects {
	effects := Effects{Attribution: prior.Attribution, Evaluated: prior.Evaluated, Outcome: prior.Outcome, DOM: prior.DOM}
	collectNetwork(deps, mark, &effects)
	collectConsole(deps, mark, &effects)
	collectActions(deps, mark, &effects)
	collectNavigation(deps, mark, &effects)
	return effects
}

func collectNetwork(deps Deps, mark Mark, effects *Effects) {
	if deps.NetworkRequests == nil {
		return
	}
	bodies, times := deps.NetworkRequests()
	for i, body := range bodies {
		if i >= len(times) || !times[i].After(mark.At) {
			continue
		}
		effects.NetworkRequestCount++
		if len(effects.NetworkRequests) >= maxAttributedRequests {
			continue
		}
		effects.NetworkRequests = append(effects.NetworkRequests, NetworkEffect{
			URL:        body.URL,
			Method:     body.Method,
			Status:     body.Status,
			DurationMs: body.Duration,
		})
	}
}

func collectConsole(deps Deps, mark Mark, effects *Effects) {
	if deps.LogEntries == nil {
		return
	}
	entries, times := deps.LogEntries()
	for i, entry := range entries {
		if i >= len(times) || !times[i].After(mark.At) {
			continue
		}
		switch level, _ := entry["level"].(string); level {
		case "error":
			effects.ConsoleErrorCount++
			if len(effects.ConsoleErrors) < maxAttributedErrors {
				effects.ConsoleErrors = append(effects.ConsoleErrors, truncate(messageOf(entry)))
			}
		case "warn", "warning":
			effects.ConsoleWarningCount++
		}
	}
}

func collectActions(deps Deps, mark Mark, effects *Effects) {
	if deps.Actions == nil {
		return
	}
	actions, times := deps.Actions()
	for i, action := range actions {
		if i >= len(times) || !times[i].After(mark.At) {
			continue
		}
		effects.RecordedActionCount++
		if action.Classification != "" {
			effects.Transients = append(effects.Transients, action.Classification)
		}
	}
}

func collectNavigation(deps Deps, mark Mark, effects *Effects) {
	if deps.TrackedURL == nil {
		return
	}
	current := deps.TrackedURL()
	if current == "" || current == mark.URL {
		return
	}
	effects.Navigation = &NavigationEffect{FromURL: mark.URL, ToURL: current}
}

func messageOf(entry types.LogEntry) string {
	if message, ok := entry["message"].(string); ok {
		return message
	}
	return ""
}

func truncate(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxErrorMessageChars {
		return text
	}
	return text[:maxErrorMessageChars] + "…"
}

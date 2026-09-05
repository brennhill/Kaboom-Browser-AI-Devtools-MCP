// effects_test.go — Contracts for the effect window: what an action is observed
// to have done, and what it is only observed NOT to have done.
// Docs: docs/features/feature/effect-verification/index.md

package actioneffects

import (
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// world is a controllable stand-in for the capture buffers. Every test advances
// its clock explicitly; nothing here sleeps.
type world struct {
	now       time.Time
	logs      []types.LogEntry
	logTimes  []time.Time
	network   []types.NetworkBody
	networkAt []time.Time
	actions   []types.EnhancedAction
	actionAt  []time.Time
	url       string
	waits     []time.Duration
	onWait    func(*world)
	waitCalls int
}

func newWorld() *world {
	return &world{now: time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC), url: "https://app.example/start"}
}

func (w *world) deps() Deps {
	return Deps{
		Now:             func() time.Time { return w.now },
		LogEntries:      func() ([]types.LogEntry, []time.Time) { return w.logs, w.logTimes },
		NetworkRequests: func() ([]types.NetworkBody, []time.Time) { return w.network, w.networkAt },
		Actions:         func() ([]types.EnhancedAction, []time.Time) { return w.actions, w.actionAt },
		TrackedURL:      func() string { return w.url },
		Wait: func(d time.Duration) {
			w.waits = append(w.waits, d)
			w.waitCalls++
			w.now = w.now.Add(d)
			if w.onWait != nil {
				w.onWait(w)
			}
		},
	}
}

func (w *world) addLog(level, message string) {
	w.logs = append(w.logs, types.LogEntry{"level": level, "message": message})
	w.logTimes = append(w.logTimes, w.now)
}

func (w *world) addRequest(url string, status int) {
	w.network = append(w.network, types.NetworkBody{URL: url, Status: status, Method: "POST", Duration: 12})
	w.networkAt = append(w.networkAt, w.now)
}

func (w *world) addAction(kind string) {
	w.actions = append(w.actions, types.EnhancedAction{Type: kind})
	w.actionAt = append(w.actionAt, w.now)
}

func budget() Budget { return Budget{Total: 300 * time.Millisecond, Poll: 50 * time.Millisecond} }

// ---------------------------------------------------------------------------
// The window boundary
// ---------------------------------------------------------------------------

func TestCollectIgnoresEverythingRecordedBeforeTheAction(t *testing.T) {
	w := newWorld()
	w.addLog("error", "a failure from the page before we touched it")
	w.addRequest("https://app.example/earlier.json", 200)
	w.addAction("click")

	w.now = w.now.Add(time.Second)
	mark := Open(w.deps())
	effects := Collect(w.deps(), mark, budget(), DOMUnknown)

	if effects.ConsoleErrorCount != 0 || effects.NetworkRequestCount != 0 || effects.RecordedActionCount != 0 {
		t.Fatalf("pre-action telemetry attributed to the action: %+v", effects)
	}
	if effects.observed() {
		t.Fatal("an action that caused nothing was reported as having an effect")
	}
}

func TestCollectAttributesTelemetryRecordedInsideTheWindow(t *testing.T) {
	w := newWorld()
	mark := Open(w.deps())
	w.onWait = func(w *world) {
		if w.waitCalls == 1 {
			w.addRequest("https://api.example/submit", 201)
			w.addLog("error", "boom")
			w.addLog("warn", "deprecated")
		}
	}

	effects := Collect(w.deps(), mark, budget(), DOMUnknown)

	if effects.NetworkRequestCount != 1 || len(effects.NetworkRequests) != 1 {
		t.Fatalf("network = %+v", effects.NetworkRequests)
	}
	if got := effects.NetworkRequests[0]; got.URL != "https://api.example/submit" || got.Status != 201 {
		t.Fatalf("request = %+v", got)
	}
	if effects.ConsoleErrorCount != 1 || effects.ConsoleWarningCount != 1 {
		t.Fatalf("console errors=%d warnings=%d", effects.ConsoleErrorCount, effects.ConsoleWarningCount)
	}
	if !effects.observed() {
		t.Fatal("attributed telemetry did not count as an observed effect")
	}
}

func TestCollectClosesEarlyOnTheFirstObservedEffect(t *testing.T) {
	w := newWorld()
	mark := Open(w.deps())
	w.onWait = func(w *world) {
		if w.waitCalls == 1 {
			w.addRequest("https://api.example/submit", 200)
		}
	}

	effects := Collect(w.deps(), mark, budget(), DOMUnknown)

	if !effects.ClosedEarly {
		t.Fatal("window stayed open after the effect it was waiting for")
	}
	if w.waitCalls != 1 {
		t.Fatalf("polled %d times after the effect appeared; want 1", w.waitCalls)
	}
	if effects.WindowMs != 50 {
		t.Fatalf("window_ms = %d, want the elapsed 50", effects.WindowMs)
	}
}

func TestCollectSpendsTheWholeBudgetWhenNothingHappens(t *testing.T) {
	// The dead-click case is the one worth waiting for: it is the only way to tell
	// "did nothing" apart from "has not happened yet".
	w := newWorld()
	mark := Open(w.deps())

	effects := Collect(w.deps(), mark, budget(), DOMUnknown)

	if effects.ClosedEarly {
		t.Fatal("reported an early close with no effect to close on")
	}
	if w.waitCalls != 6 {
		t.Fatalf("polled %d times; want 6 (300ms budget / 50ms poll)", w.waitCalls)
	}
	if effects.WindowMs != 300 {
		t.Fatalf("window_ms = %d, want 300", effects.WindowMs)
	}
	if effects.observed() {
		t.Fatal("an empty window reported an effect")
	}
}

// ---------------------------------------------------------------------------
// Individual effect kinds
// ---------------------------------------------------------------------------

func TestCollectReportsNavigationAsAUrlChange(t *testing.T) {
	w := newWorld()
	mark := Open(w.deps())
	w.onWait = func(w *world) { w.url = "https://app.example/done" }

	effects := Collect(w.deps(), mark, budget(), DOMUnknown)

	if effects.Navigation == nil {
		t.Fatal("navigation not reported")
	}
	if effects.Navigation.FromURL != "https://app.example/start" || effects.Navigation.ToURL != "https://app.example/done" {
		t.Fatalf("navigation = %+v", effects.Navigation)
	}
	if !effects.observed() {
		t.Fatal("a navigation is an effect")
	}
}

func TestCollectOmitsNavigationWhenTheURLHeldStill(t *testing.T) {
	w := newWorld()
	mark := Open(w.deps())

	if effects := Collect(w.deps(), mark, budget(), DOMUnknown); effects.Navigation != nil {
		t.Fatalf("navigation reported for an unchanged URL: %+v", effects.Navigation)
	}
}

func TestCollectNamesTransientsThePageRaised(t *testing.T) {
	w := newWorld()
	mark := Open(w.deps())
	w.onWait = func(w *world) {
		if w.waitCalls == 1 {
			w.actions = append(w.actions, types.EnhancedAction{Type: "transient", Classification: "toast"})
			w.actionAt = append(w.actionAt, w.now)
		}
	}

	effects := Collect(w.deps(), mark, budget(), DOMUnknown)

	if len(effects.Transients) != 1 || effects.Transients[0] != "toast" {
		t.Fatalf("transients = %v", effects.Transients)
	}
}

func TestCollectCapsTheEvidenceItReturns(t *testing.T) {
	w := newWorld()
	mark := Open(w.deps())
	w.onWait = func(w *world) {
		if w.waitCalls != 1 {
			return
		}
		for i := 0; i < maxAttributedRequests+5; i++ {
			w.addRequest("https://api.example/req", 200)
		}
		for i := 0; i < maxAttributedErrors+5; i++ {
			w.addLog("error", "repeated failure")
		}
	}

	effects := Collect(w.deps(), mark, budget(), DOMUnknown)

	if effects.NetworkRequestCount != maxAttributedRequests+5 {
		t.Fatalf("count = %d; the count must not be capped, only the listing", effects.NetworkRequestCount)
	}
	if len(effects.NetworkRequests) != maxAttributedRequests {
		t.Fatalf("listed %d requests; want %d", len(effects.NetworkRequests), maxAttributedRequests)
	}
	if effects.ConsoleErrorCount != maxAttributedErrors+5 || len(effects.ConsoleErrors) != maxAttributedErrors {
		t.Fatalf("errors count=%d listed=%d", effects.ConsoleErrorCount, len(effects.ConsoleErrors))
	}
}

// ---------------------------------------------------------------------------
// Attribution is correlational and must say so
// ---------------------------------------------------------------------------

func TestEffectsDeclareAttributionIsTemporalNotCausal(t *testing.T) {
	w := newWorld()
	mark := Open(w.deps())
	effects := Collect(w.deps(), mark, budget(), DOMUnknown)

	if effects.Attribution != AttributionTemporalWindow {
		t.Fatalf("attribution = %q", effects.Attribution)
	}
	payload := effects.Payload()
	note, _ := payload["attribution_note"].(string)
	if note == "" {
		t.Fatal("no attribution note: a happens-after presented as a happens-because is the failure this guards")
	}
	for _, phrase := range []string{"caused", "window"} {
		if !strings.Contains(note, phrase) {
			t.Fatalf("attribution note %q does not mention %q", note, phrase)
		}
	}
}

// ---------------------------------------------------------------------------
// Degraded inputs
// ---------------------------------------------------------------------------

func TestCollectWithNoReadersReportsNotEvaluatedRatherThanNoEffect(t *testing.T) {
	// Claiming "no observable effect" when nothing was observable at all would tell
	// the agent the action failed when the truth is the window never ran.
	effects := Collect(Deps{}, Mark{}, budget(), DOMUnknown)

	if effects.Evaluated {
		t.Fatal("an unwired window reported itself as evaluated")
	}
	if effects.observed() {
		t.Fatal("an unwired window reported an effect")
	}
}

// ---------------------------------------------------------------------------
// The DOM verdict short-circuits the window
// ---------------------------------------------------------------------------

func TestCollectAnswersWithoutPollingWhenTheDOMAlreadyChanged(t *testing.T) {
	// The extension's mutation report rides on the action's own response, so it
	// is already in hand. Polling after it would be latency spent confirming
	// something known — and it is the common success case, so it would be paid
	// on nearly every action.
	w := newWorld()
	mark := Open(w.deps())

	effects := Collect(w.deps(), mark, budget(), DOMChanged)

	if w.waitCalls != 0 {
		t.Fatalf("polled %d times after the DOM already answered", w.waitCalls)
	}
	if effects.WindowMs != 0 || !effects.ClosedEarly {
		t.Fatalf("window_ms=%d closed_early=%v", effects.WindowMs, effects.ClosedEarly)
	}
	if effects.DOM != DOMChanged {
		t.Fatal("the DOM verdict did not survive onto the effects")
	}
}

func TestCollectStillRunsTheWindowWhenTheDOMSaysNothingMoved(t *testing.T) {
	// This is kaboom-knms. A report of "no DOM changes" is the reason to look
	// harder, not the reason to stop looking.
	w := newWorld()
	mark := Open(w.deps())

	effects := Collect(w.deps(), mark, budget(), DOMUnchanged)

	if w.waitCalls != 6 {
		t.Fatalf("polled %d times; want the full 6", w.waitCalls)
	}
	if effects.DOM != DOMUnchanged {
		t.Fatalf("dom = %v", effects.DOM)
	}
}

func TestCollectAttributesNetworkRequestsByServerIngestTime(t *testing.T) {
	w := newWorld()
	w.addRequest("https://api.example/before", 200)
	w.now = w.now.Add(time.Second)
	mark := Open(w.deps())
	w.onWait = func(w *world) {
		if w.waitCalls == 1 {
			w.addRequest("https://api.example/after", 204)
		}
	}

	effects := Collect(w.deps(), mark, budget(), DOMUnchanged)

	if effects.NetworkRequestCount != 1 || len(effects.NetworkRequests) != 1 {
		t.Fatalf("network = %+v", effects.NetworkRequests)
	}
	got := effects.NetworkRequests[0]
	if got.URL != "https://api.example/after" || got.Method != "POST" || got.Status != 204 {
		t.Fatalf("request = %+v", got)
	}
}

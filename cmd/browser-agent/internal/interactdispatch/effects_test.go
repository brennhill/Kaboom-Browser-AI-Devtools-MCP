// effects_test.go — Contracts for the effect window the dispatcher opens around
// every mutating action, so no action owner can report a success it did not have.
// Docs: docs/features/feature/effect-verification/index.md

package interactdispatch

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/actioneffects"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// effectWorld drives the dispatcher's window with no wall clock: telemetry is
// added by the action itself, so "during the window" is exact.
type effectWorld struct {
	now       time.Time
	network   []types.NetworkBody
	networkAt []time.Time
	url       string
	opened    int
	waits     int
}

func newEffectWorld() *effectWorld {
	return &effectWorld{now: time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC), url: "https://app.example/"}
}

func (w *effectWorld) effectDeps() actioneffects.Deps {
	return actioneffects.Deps{
		Now:             func() time.Time { return w.now },
		NetworkRequests: func() ([]types.NetworkBody, []time.Time) { return w.network, w.networkAt },
		TrackedURL:      func() string { return w.url },
		Wait:            func(d time.Duration) { w.waits++; w.now = w.now.Add(d) },
	}
}

func (w *effectWorld) handler(t *testing.T, actionResult map[string]any, onAction func(*effectWorld)) *Handler {
	t.Helper()
	run := func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		if onAction != nil {
			onAction(w)
		}
		return mcp.Succeed(req, "ok", actionResult)
	}
	return New(Deps{
		Actions: map[string]Action{
			"click":            run,
			"navigate":         run,
			"get_readable":     run,
			"list_interactive": run,
		},
		Effects: func() actioneffects.Deps {
			w.opened++
			return w.effectDeps()
		},
		EffectBudget: actioneffects.Budget{Total: 200 * time.Millisecond, Poll: 50 * time.Millisecond},
		Delay:        func(time.Duration) {},
	})
}

func effectsBlock(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	data, ok := mcp.ReadResultPayload(resp)
	if !ok {
		t.Fatalf("response payload unreadable: %s", resp.Result)
	}
	block, ok := data["effects"].(map[string]any)
	if !ok {
		t.Fatalf("no effects block in %#v", data)
	}
	return block
}

func TestDispatchReportsAnActionThatDidNothing(t *testing.T) {
	// kaboom-knms: a click that hit nothing came back as a plain success. The
	// dispatcher now says so in the same response.
	w := newEffectWorld()
	h := w.handler(t, map[string]any{"success": true, "dom_summary": "no DOM changes"}, nil)

	resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"click"}`))

	block := effectsBlock(t, resp)
	if block["outcome"] != actioneffects.OutcomeNoEffect {
		t.Fatalf("outcome = %v, want %v", block["outcome"], actioneffects.OutcomeNoEffect)
	}
	if block["dom_changed"] != false {
		t.Fatalf("dom_changed = %v", block["dom_changed"])
	}
	data, _ := mcp.ReadResultPayload(resp)
	if data["retryable"] != false {
		t.Fatalf("retryable = %v; a repeat of a no-op is another no-op", data["retryable"])
	}
	if advice, _ := data["retry"].(string); !strings.Contains(advice, "same result") {
		t.Fatalf("retry advice = %q", advice)
	}
}

func TestDispatchReportsAnActionTheTelemetryConfirms(t *testing.T) {
	w := newEffectWorld()
	h := w.handler(t, map[string]any{"success": true, "dom_summary": "no DOM changes"}, func(w *effectWorld) {
		w.network = append(w.network, types.NetworkBody{URL: "https://api.example/submit", Method: "POST", Status: 200})
		w.networkAt = append(w.networkAt, w.now.Add(10*time.Millisecond))
	})

	resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"click"}`))

	block := effectsBlock(t, resp)
	if block["outcome"] != actioneffects.OutcomeObserved {
		t.Fatalf("outcome = %v", block["outcome"])
	}
	if block["network_request_count"] != float64(1) {
		t.Fatalf("network_request_count = %#v", block["network_request_count"])
	}
	data, _ := mcp.ReadResultPayload(resp)
	if _, decided := data["retryable"]; decided {
		t.Fatal("an action that worked was given retry advice")
	}
}

func TestDispatchTrustsTheDOMReportTheExtensionAlreadySends(t *testing.T) {
	// A click that only re-rendered the page makes no request and logs nothing.
	// The mutation summary is the only evidence it did anything.
	w := newEffectWorld()
	h := w.handler(t, map[string]any{"success": true, "dom_summary": "3 added, 1 removed"}, nil)

	block := effectsBlock(t, h.Handle(dispatchReq(), json.RawMessage(`{"what":"click"}`)))

	if block["outcome"] != actioneffects.OutcomeObserved || block["dom_changed"] != true {
		t.Fatalf("effects = %#v", block)
	}
}

func TestDispatchOpensNoWindowForReadOnlyActions(t *testing.T) {
	// get_readable cannot have an effect to verify; charging it the window would
	// be latency spent on a question with no answer.
	w := newEffectWorld()
	h := w.handler(t, map[string]any{"text": "hello"}, nil)

	for _, action := range []string{"get_readable", "list_interactive"} {
		resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"`+action+`"}`))
		data, ok := mcp.ReadResultPayload(resp)
		if !ok {
			t.Fatalf("%s: payload unreadable", action)
		}
		if _, present := data["effects"]; present {
			t.Fatalf("%s opened an effect window", action)
		}
	}
	if w.opened != 0 {
		t.Fatalf("effect deps requested %d times for read-only actions", w.opened)
	}
}

func TestDispatchOpensNoWindowWhenTheCallerOptsOut(t *testing.T) {
	w := newEffectWorld()
	h := w.handler(t, map[string]any{"success": true}, nil)

	resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"click","effects":false}`))

	data, _ := mcp.ReadResultPayload(resp)
	if _, present := data["effects"]; present {
		t.Fatal("effects:false still opened a window")
	}
	if w.opened != 0 {
		t.Fatalf("effect deps requested %d times after opt-out", w.opened)
	}
}

func TestDispatchHonoursAWiderWindowWhenAsked(t *testing.T) {
	w := newEffectWorld()
	h := w.handler(t, map[string]any{"success": true, "dom_summary": "no DOM changes"}, nil)

	block := effectsBlock(t, h.Handle(dispatchReq(), json.RawMessage(`{"what":"click","effect_window_ms":500}`)))

	if block["window_ms"] != float64(500) {
		t.Fatalf("window_ms = %#v, want 500", block["window_ms"])
	}
}

func TestDispatchClampsAnAbsurdWindowRatherThanHangingTheCaller(t *testing.T) {
	w := newEffectWorld()
	h := w.handler(t, map[string]any{"success": true, "dom_summary": "no DOM changes"}, nil)

	block := effectsBlock(t, h.Handle(dispatchReq(), json.RawMessage(`{"what":"click","effect_window_ms":600000}`)))

	if block["window_ms"] != float64(maxEffectWindow/time.Millisecond) {
		t.Fatalf("window_ms = %#v, want the clamp %d", block["window_ms"], maxEffectWindow/time.Millisecond)
	}
}

func TestDispatchOpensNoWindowInBackgroundMode(t *testing.T) {
	// A background call returns a queue receipt, not an outcome. Measuring a
	// window around the receipt would time the queueing, not the action.
	w := newEffectWorld()
	h := w.handler(t, map[string]any{"status": "queued", "correlation_id": "c1"}, nil)

	resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"click","background":true}`))

	data, _ := mcp.ReadResultPayload(resp)
	if _, present := data["effects"]; present {
		t.Fatal("background dispatch opened an effect window")
	}
}

func TestDispatchAttachesNoEffectsToAFailedAction(t *testing.T) {
	// A failure already says what went wrong, and the outcome is not in doubt.
	// The mark is taken before dispatch (nothing knows yet that it will fail),
	// but the window must never be collected: that is where the latency is.
	w := newEffectWorld()
	h := New(Deps{
		Actions: map[string]Action{"click": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			return mcp.Fail(req, mcp.ErrInvalidParam, "no such element", "try another selector")
		}},
		Effects:      func() actioneffects.Deps { w.opened++; return w.effectDeps() },
		EffectBudget: actioneffects.Budget{Total: 200 * time.Millisecond, Poll: 50 * time.Millisecond},
		Delay:        func(time.Duration) {},
	})

	resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"click"}`))

	if w.waits != 0 {
		t.Fatalf("a failed dispatch waited out %d effect polls", w.waits)
	}
	if data, ok := mcp.ReadResultPayload(resp); ok {
		if _, present := data["effects"]; present {
			t.Fatal("a failed action carried an effects block")
		}
	}
}

func TestDispatchWithoutEffectWiringLeavesResponsesAlone(t *testing.T) {
	h := New(Deps{
		Actions: map[string]Action{"click": func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
			return mcp.Succeed(req, "ok", map[string]any{"success": true})
		}},
		Delay: func(time.Duration) {},
	})

	resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"click"}`))

	data, ok := mcp.ReadResultPayload(resp)
	if !ok {
		t.Fatal("payload unreadable")
	}
	if _, present := data["effects"]; present {
		t.Fatal("an unwired dispatcher invented an effects block")
	}
}

func TestDispatchSpendsNoWindowWhenTheDOMAlreadyAnswered(t *testing.T) {
	// The common success case must cost nothing, or effect verification is a tax
	// on every working action rather than an answer about a broken one.
	w := newEffectWorld()
	h := w.handler(t, map[string]any{"success": true, "dom_summary": "2 added"}, nil)

	block := effectsBlock(t, h.Handle(dispatchReq(), json.RawMessage(`{"what":"click"}`)))

	if w.waits != 0 {
		t.Fatalf("a click that visibly changed the page still waited out %d polls", w.waits)
	}
	if block["window_ms"] != float64(0) || block["closed_early"] != true {
		t.Fatalf("effects = %#v", block)
	}
}

func TestDispatchOpensNoWindowForActionsItsSignalsCannotSee(t *testing.T) {
	// Storage and cookie writes mutate state no signal in the window observes.
	// A window over them would report no_observable_effect for an action that
	// worked, and then tell the caller not to retry it.
	w := newEffectWorld()
	run := func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return mcp.Succeed(req, "ok", map[string]any{"success": true})
	}
	actions := map[string]Action{}
	for _, name := range effectBlindActions() {
		actions[name] = run
	}
	h := New(Deps{
		Actions:      actions,
		Effects:      func() actioneffects.Deps { w.opened++; return w.effectDeps() },
		EffectBudget: actioneffects.Budget{Total: 200 * time.Millisecond, Poll: 50 * time.Millisecond},
		Delay:        func(time.Duration) {},
	})

	for _, name := range effectBlindActions() {
		resp := h.Handle(dispatchReq(), json.RawMessage(`{"what":"`+name+`"}`))
		data, ok := mcp.ReadResultPayload(resp)
		if !ok {
			t.Fatalf("%s: payload unreadable", name)
		}
		if _, present := data["effects"]; present {
			t.Fatalf("%s carried an effects block its signals cannot support", name)
		}
		if _, decided := data["retryable"]; decided {
			t.Fatalf("%s was given retry advice from a window that never ran", name)
		}
	}
	if w.opened != 0 {
		t.Fatalf("effect deps requested %d times for effect-blind actions", w.opened)
	}
}

func TestEveryEffectBlindActionIsStillAMutationAction(t *testing.T) {
	// The exclusion list only means anything as a subtraction from
	// IsMutationAction. An entry that is not a mutation action is dead weight
	// that will quietly stop matching the thing it was written to exclude.
	for _, name := range effectBlindActions() {
		if !toolinteract.IsMutationAction(name) {
			t.Errorf("%q is in effectBlindActions but is not a mutation action, so it excludes nothing", name)
		}
	}
}

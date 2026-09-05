// effects.go — The effect window the dispatcher opens around every mutating
// action, so a response says what the action did rather than that it ran.
// Docs: docs/features/feature/effect-verification/index.md

package interactdispatch

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/actioneffects"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Window bounds. The default is short because it closes on the first observed
// effect: only an action that genuinely did nothing spends the whole budget,
// and that is the one case where the answer is worth waiting for.
// The default has to clear the extension's own send cadence or the window
// reports "nothing happened" when the evidence simply had not arrived: the
// console batcher debounces 100 ms, the action batcher 200 ms, and each then
// makes an HTTP round trip. 600 ms clears both with room for the trip.
//
// This budget is only ever spent by an action whose DOM report said nothing
// moved — a DOM change answers before any polling starts.
const (
	defaultEffectWindow = 600 * time.Millisecond
	defaultEffectPoll   = 50 * time.Millisecond
	maxEffectWindow     = 5 * time.Second
)

// effectBlindActions names the mutations none of the window's signals can see, so
// running a window over them would report "no observable effect" for an action
// that worked — and then stop the caller retrying it. They are a subtraction from
// IsMutationAction rather than a second inclusion list, so the two cannot drift.
//
// A function rather than a package var: the list is fixed, and a mutable global
// here is a shared surface any caller could edit.
func effectBlindActions() []string {
	return []string{
		"set_storage", "delete_storage", "clear_storage",
		"set_cookie", "delete_cookie",
		// highlight injects Kaboom's own overlay. Verifying our own injection
		// measures the tool, not the page.
		"highlight",
	}
}

func isEffectBlindAction(what string) bool {
	what = strings.ToLower(strings.TrimSpace(what))
	for _, name := range effectBlindActions() {
		if name == what {
			return true
		}
	}
	return false
}

// effectArgs are the per-call controls over the window.
type effectArgs struct {
	Effects      *bool `json:"effects"`
	EffectWindow int   `json:"effect_window_ms,omitempty"`
	Background   bool  `json:"background"`
	Async        bool  `json:"async"`
}

func parseEffectArgs(args json.RawMessage) effectArgs {
	var params effectArgs
	if len(args) == 0 {
		return params
	}
	if err := json.Unmarshal(args, &params); err != nil {
		// EXPECTED_ABSENCE: canonical toolrouting.Dispatch already reports malformed
		// JSON to the caller. A second log here would be duplicate logging of one fault.
		return effectArgs{}
	}
	return params
}

// wantsEffectWindow decides whether this call earns a window. A read-only action
// has no effect to verify, a background call returns a queue receipt rather than
// an outcome, and a caller may always opt out.
func (h *Handler) wantsEffectWindow(params effectArgs, what string, failed bool) bool {
	if h.deps.Effects == nil || failed {
		return false
	}
	if params.Effects != nil && !*params.Effects {
		return false
	}
	if params.Background || params.Async {
		return false
	}
	if isEffectBlindAction(what) {
		return false
	}
	return toolinteract.IsMutationAction(what)
}

// effectBudget resolves the window bounds for one call, clamped so a caller
// cannot park the dispatcher behind an arbitrary wait.
func (h *Handler) effectBudget(params effectArgs) actioneffects.Budget {
	budget := h.deps.EffectBudget
	if budget.Total <= 0 {
		budget.Total = defaultEffectWindow
	}
	if budget.Poll <= 0 {
		budget.Poll = defaultEffectPoll
	}
	if params.EffectWindow > 0 {
		budget.Total = time.Duration(params.EffectWindow) * time.Millisecond
	}
	if budget.Total > maxEffectWindow {
		budget.Total = maxEffectWindow
	}
	if budget.Poll > budget.Total {
		budget.Poll = budget.Total
	}
	return budget
}

// attachEffects runs the window that was opened before dispatch and writes its
// verdict onto the action's own response. Provenance stays with the evidence:
// the block names its attribution as temporal, never as causal.
func (h *Handler) attachEffects(
	response mcp.JSONRPCResponse, deps actioneffects.Deps, mark actioneffects.Mark, budget actioneffects.Budget,
) mcp.JSONRPCResponse {
	// The DOM verdict is read before the window runs, not after: it decides
	// whether the window needs to run at all.
	dom := actioneffects.DOMUnknown
	if data, readable := mcp.ReadResultPayload(response); readable {
		dom = actioneffects.DOMChangeFrom(data)
	}
	effects := actioneffects.Collect(deps, mark, budget, dom)
	effects.Outcome = actioneffects.Classify(false, effects)
	return mcp.MutateResultPayload(response, func(data map[string]any) bool {
		data["effects"] = effects.Payload()
		actioneffects.ApplyRetryAdvice(data, effects.Outcome)
		return true
	})
}

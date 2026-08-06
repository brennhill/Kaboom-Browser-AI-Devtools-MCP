---
status: in-progress
scope: v5.9-v6
ai-priority: critical
tags: [roadmap, v5.9, v6, practical]
last-verified: 2026-02-06
---

# Kaboom Roadmap: What Actually Helps AI Debug UI

v5.8 shipped DOM primitives, smart selectors, visual toasts, and 13 interact actions.
That covers ~80% of what an AI needs to debug UI issues.

The remaining 20% is better context, not more data sources.

---

## The 5 Features That Matter

### 1. Error Bundling (auto-context on errors) — SHIPPED

**Status:** Implemented in v5.8.x. `observe({what: 'error_bundles', limit: 5, window_seconds: 3})`

**What it does:** For each error, automatically packages:

- The error itself (message, stack, source, URL)
- Network requests/responses within the time window
- User actions within the time window
- Console logs within the time window

One call replaces 3-4 separate observe() calls. Go-side only, no extension changes.

**Tech spec:** [docs/features/feature/error-bundling/tech-spec.md](../features/feature/error-bundling/tech-spec.md)
**Implementation:** `cmd/browser-agent/tools_observe_bundling.go` (~130 lines)
**Tests:** 11 behavioral tests in `tools_observe_bundling_test.go`

---

### 2. Rich Action Results (perf diff + DOM diff + interaction analysis)

**Problem:** The AI's iteration loop is slow. It edits code, refreshes, then makes 3+ observe calls to understand what changed. For DOM actions, it clicks a button and has no idea what happened without follow-up queries.

**Solution:** Action results include everything that happened. The extension is the measurement instrument.

**Navigation actions** (refresh, navigate) — auto-diff perf vs previous load:

```json
interact({ what: "refresh" })
// → result includes:
{
  "perf_diff": {
    "metrics": {
      "lcp": { "before": 2800, "after": 1200, "pct": "-57%", "improved": true },
      "transfer_kb": { "before": 768, "after": 512, "pct": "-33%", "improved": true }
    },
    "resources": {
      "removed": [{ "url": "/old-bundle.js", "type": "script", "kb": 256 }]
    },
    "summary": "LCP improved 57% (2.8s → 1.2s). 256KB saved by removing old-bundle.js."
  }
}
```

**DOM actions** — always-on compact feedback (~30 tokens):

```json
interact({ what: "click", selector: "text=Submit" })
// → { "timing_ms": 85, "dom_summary": "2 added, 1 attr changed" }
```

**DOM actions with `analyze: true`** — full interaction profiling:

```json
interact({ what: "click", selector: "text=Load More", analyze: true })
// → timing breakdown, network requests, long tasks, layout shifts, detailed DOM changes
// → analysis: "340ms total: 180ms network (/api/items), 120ms JS long task, 40ms render."
```

**User Timing passthrough** — extension captures standard `performance.mark()` / `performance.measure()` entries and surfaces them through `observe({what: 'performance'})`. No Kaboom-specific API.

**The optimization loop:** edit → `interact(refresh)` → read perf_diff → repeat. One call per iteration.

**Tech spec:** [docs/features/feature/perf-experimentation/tech-spec.md](../features/feature/perf-experimentation/tech-spec.md)
**Effort:** ~1.5 weeks. Extension-side: perf tracking, MutationObserver in domPrimitive, User Timing capture. Go-side: pass through `analyze` flag, surface user timing.

---

### 3. Noise Reduction (show only what changed since last action)

**Problem:** The AI calls observe({what: 'logs'}) and gets 200 entries, most of which are irrelevant noise from before its last action. It has to mentally filter to find the 3 entries that matter. This wastes context window and makes the AI less effective.

**Solution:** Track a "high-water mark" per client session. Provide a simple filter: "only show me things that happened since my last interact/DOM action."

**API:** `observe({what: 'logs', since: 'last_action'})` — returns only entries timestamped after the most recent interact action for this client. Works for all observe modes: logs, errors, network_waterfall, network_bodies, actions.

**Effort:** ~0.5 weeks. The daemon already tracks actions via recordAIAction. Store the timestamp. Add a `since: 'last_action'` filter that converts to a timestamp cursor internally. Minimal code — mostly wiring.

---

### 4. Screenshot After Actions

**Problem:** The AI performs DOM actions but can't see what the page looks like after. It's operating blind — clicking buttons and hoping. `captureScreenshot` exists but isn't integrated into the DOM primitive flow.

**Solution:** After mutating DOM actions (click, type, select, check), optionally capture a screenshot and include a reference in the result.

**API:** `interact({what: 'click', selector: 'text=Submit', screenshot: true})` — result includes `screenshot_url` or base64 data that the AI can view.

**Effort:** ~0.5 weeks. Wire `chrome.tabs.captureVisibleTab()` into the DOM action result path. Add `screenshot` boolean param to schema. Return as base64 data URL in the async result.

---

### 5. Action Export (recorded actions → repeatable UAT scripts)

**Problem:** The AI (or human) performs a flow — login, add to cart, checkout — and it works. But there's no way to replay that flow later to verify it still works. Actions are captured in a ring buffer (50 entries) and then lost. `generate({what: 'test'})` is a stub. The existing `test_from_context` is hidden and only works for error reproduction.

**Solution:** Export captured actions as either:
1. **Playwright test scripts** — portable, runs in CI, standard tooling
2. **Kaboom narratives** — JSON action sequences the AI replays via DOM primitives

The key insight: DOM primitives (`interact({what: 'click', selector: 'text=Submit'})`) already solve browser automation. A Kaboom narrative is just a JSON array of interact calls the AI executes step-by-step, checking state between each step.

**Playwright export** — wire the existing `generatePlaywrightScript()` in testgen.go to `format: 'test'`:
```
generate({what: 'test', test_name: 'checkout-flow'})
→ returns full Playwright test using captured actions + multi-strategy selectors
```

**Kaboom narrative export** — new format that produces a replayable JSON sequence:
```
generate({what: 'narrative', name: 'checkout-flow'})
```
Returns:
```json
{
  "name": "checkout-flow",
  "base_url": "https://example.com",
  "steps": [
    {
      "action": "navigate",
      "url": "https://example.com/login"
    },
    {
      "action": "type",
      "selector": "placeholder=Email",
      "text": "user@example.com",
      "description": "Enter email"
    },
    {
      "action": "type",
      "selector": "placeholder=Password",
      "text": "••••••••",
      "description": "Enter password"
    },
    {
      "action": "click",
      "selector": "text=Sign In",
      "description": "Submit login form"
    },
    {
      "action": "wait_for",
      "selector": "text=Dashboard",
      "description": "Verify login succeeded"
    }
  ]
}
```

The AI replays a narrative by iterating steps and calling `interact()` for each one. Between steps it can `observe()` to verify state, handle errors, or adapt if selectors changed. This is more resilient than Playwright because the AI can self-heal on the fly.

**Why this matters:** Turns Kaboom from a debugging tool into a UAT tool. Teams can record flows once, export them, and replay them whenever they want to verify nothing broke. The AI becomes both the recorder and the test runner.

**What already exists:**
- `EnhancedAction` captures rich multi-strategy selectors (testId, role, ariaLabel, text, id, cssPath)
- `generatePlaywrightScript()` in testgen.go converts actions → Playwright code
- DOM primitives handle smart selectors (text=, role=, placeholder=, label=, aria-label=)
- Recording/playback architecture exists (start/stop, persistence to ~/.kaboom/recordings/)

**What's missing:**
- Wire `format: 'test'` to actual Playwright generation (replace the stub)
- New `format: 'narrative'` that exports actions as interact-compatible JSON
- Convert multi-strategy selectors to the best smart selector format (prefer text=, role= over CSS)
- Narrative persistence (save/load from ~/.kaboom/narratives/)

**Effort:** ~1 week. Most of the hard work is done — selector capture, Playwright generation, and DOM primitives all exist. This is mostly wiring + a new export format.

---

## Priority Order

| # | Feature | Status | Impact | Effort |
| --- | --- | --- | --- | --- |
| 1 | Error Bundling | SHIPPED | Saves 3-4 calls per error | Done |
| 2 | Rich Action Results | Spec ready | One-call perf loop + cause-and-effect | 1.5 weeks |
| 3 | Noise Reduction | Proposed | Reduces context waste | 0.5 weeks |
| 4 | Screenshot After Actions | Proposed | Visual feedback | 0.5 weeks |
| 5 | Action Export | Proposed | Repeatable UAT scripts | 1 week |

**Remaining: ~3.5 weeks for features 2-5.**

---

## Future: AI-Native Dev Cycle Acceleration

These features go beyond debugging into making the AI's entire dev cycle faster and more reliable.

### 6. On-Demand Screenshot — SHIPPED

**Status:** Implemented in v5.8.x. `observe({what: 'screenshot'})`

**What it does:** Captures the current viewport and saves as JPEG. Returns filename and path. The AI can request a screenshot at any point — not just on errors.

Built on existing `captureVisibleTab()` infrastructure. Same rate limiting (1/sec, 10/min per tab).

---

### 7. DOM Diff (structural change detection after actions)

**Problem:** After a DOM action, the AI knows "click succeeded" but not *what changed*. It has to execute JS to inspect elements, costing 2-3 round-trips per verification. `perf_diff` covers timing, but not structure.

**Solution:** `observe({what: 'dom_diff'})` or integrate into the `analyze: true` flag. Snapshot DOM before action, diff after. Return compact structured delta:

```json
{
  "added": [{"tag": "div", "class": "modal", "text": "Confirm deletion?"}],
  "removed": [{"tag": "tr", "id": "row-42"}],
  "changed": [{"selector": "#status", "attribute": "textContent", "from": "Pending", "to": "Complete"}],
  "summary": "Modal appeared, table row removed, status text updated"
}
```

**Key design:** Cap at top-N changes, ignore Kaboom's own injected elements, use MutationObserver in the extension. The summary field is the AI's primary signal — it can decide whether to dig deeper.

**Effort:** ~1.5 weeks. MutationObserver setup, serialization, Go passthrough.

---

### 8. Network Request Mocking (deterministic testing)

**Problem:** AI testing is unreliable because it depends on live APIs. Rate limits, slow responses, intermittent failures all cause false negatives. Testing error states (500s, timeouts, malformed JSON) requires actual server failures.

**Solution:** `configure({what: 'mock', url: '/api/users', method: 'GET', response: {status: 200, body: [...]}})`. The extension intercepts matching fetch/XHR requests and returns the mock instead.

```json
configure({what: "mock", url: "/api/checkout", method: "POST", response: {
  status: 500,
  body: {"error": "payment_failed"},
  delay_ms: 2000
}})
// Now the AI can test error handling without a real backend failure
```

**Key design:** Mock rules stored in extension memory (not persisted). `configure({what: 'mock_clear'})` removes all mocks. Mocks are per-session. Uses service worker fetch event interception or declarativeNetRequest.

**Effort:** ~2 weeks. Extension-side fetch interception, Go-side mock rule management, cleanup lifecycle.

---

### 9. Assertion Batching (multi-check in one call)

**Problem:** Verifying page state requires multiple round-trips: check element visible, check text content, check network call happened, check no console errors. Each round-trip costs 1-2 seconds. A 10-assertion verification takes 15+ seconds.

**Solution:** `observe({what: 'assertions', checks: [...]})` — verify multiple conditions in one call:

```json
observe({what: "assertions", checks: [
  {type: "element", selector: ".success-banner", visible: true},
  {type: "text", selector: "#total", contains: "$42.00"},
  {type: "network", url: "/api/order", status: 200},
  {type: "console", level: "error", count: 0}
]})
// → { passed: 3, failed: 1, results: [{...}, {...}, {...}, {...}] }
```

**Key design:** Extension evaluates element/text checks via DOM query. Network/console checks run Go-side from captured data. Single response with pass/fail per check + overall verdict.

**Effort:** ~1 week. Extension-side batch DOM evaluation, Go-side check dispatching and aggregation.

---

### 10. Session Recording & Replay (regression testing)

**Problem:** The AI performs a multi-step flow (login → navigate → fill form → submit) and it works. But there's no way to deterministically replay that flow later. Actions are captured but ephemeral. Network responses aren't persisted alongside actions.

**Solution:** Two new operations:
- `configure({what: 'record_start', name: 'checkout-flow'})` — starts recording actions + network responses
- `configure({what: 'record_stop'})` — saves recording to `~/.kaboom/recordings/`
- `configure({what: 'replay', name: 'checkout-flow'})` — replays actions with mocked network responses

**Key design:** Recording bundles actions + network response snapshots. Replay uses network mocking (feature 8) to inject saved responses, then executes actions via DOM primitives. The AI monitors each step and can self-heal if selectors changed.

**Effort:** ~2 weeks. Depends on network mocking (feature 8). Recording serialization, replay orchestration, recording persistence.

---

## Updated Priority Order

| # | Feature | Status | Impact | Effort |
| --- | --- | --- | --- | --- |
| 1 | Error Bundling | SHIPPED | Saves 3-4 calls per error | Done |
| 2 | Rich Action Results | Spec ready | One-call perf loop + cause-and-effect | 1.5 weeks |
| 3 | Noise Reduction | Proposed | Reduces context waste | 0.5 weeks |
| 4 | On-Demand Screenshot | SHIPPED | Visual verification on demand | Done |
| 5 | Action Export | Proposed | Repeatable UAT scripts | 1 week |
| 6 | DOM Diff | Proposed | Eliminates 2-3 round-trips per action | 1.5 weeks |
| 7 | Assertion Batching | Proposed | 5-10x faster page verification | 1 week |
| 8 | Network Mocking | Proposed | Deterministic test environments | 2 weeks |
| 9 | Session Recording & Replay | Proposed | Full regression testing | 2 weeks |
| 10 | Screen Annotation & Drawing | Proposed | Visual bug reporting for AI | 2 weeks |

**Remaining: ~10.5 weeks for features 2-10 (features 1, 4 shipped).**

---

### 11. Screen Annotation & Drawing (visual bug reporting for AI)

**Problem:** The AI can observe page state, but the human can't easily *point at things* and say "this is broken" or "move this here." Today the user has to describe layout issues, visual glitches, and design changes in words — which is slow and imprecise. Screenshots help but lack annotation. The human sees the problem instantly; translating that into text for the AI is the bottleneck.

**Solution:** An interactive drawing/annotation overlay that lets the user mark up the live page and attach natural language descriptions. The AI receives both the visual annotations and the text as structured context.

**Activation:** `interact({what: 'annotate'})` — enables drawing mode overlay on the active tab. User draws, types notes, then submits. The AI receives:

```json
{
  "annotations": [
    {
      "type": "circle",
      "bounds": {"x": 340, "y": 120, "width": 200, "height": 80},
      "note": "This button is misaligned — should be flush right with the card",
      "target_selector": "#checkout-btn"
    },
    {
      "type": "arrow",
      "from": {"x": 100, "y": 300},
      "to": {"x": 400, "y": 300},
      "note": "These two sections should have equal spacing"
    },
    {
      "type": "freehand",
      "path": [[10,10], [50,20], [80,60]],
      "note": "This whole area flickers on scroll"
    }
  ],
  "screenshot_base64": "...",
  "url": "https://example.com/checkout",
  "viewport": {"width": 1440, "height": 900}
}
```

**Drawing tools:** Circle/rectangle highlight, arrow, freehand draw, text pin. Each annotation gets an optional text note. Annotations are rendered as a canvas overlay — never modify the page DOM.

**Why this matters:** Bridges the gap between what the human *sees* and what the AI *knows*. A circle around a misaligned button + "fix this" is worth 50 words of description. Turns Kaboom into a visual collaboration layer between human and AI — the human points, the AI fixes.

**Key design decisions:**
- Canvas overlay (not DOM injection) — zero interference with page layout/styles
- Annotations auto-map to nearest DOM element via `elementFromPoint()` for selector context
- Screenshot captured with annotations baked in, so the AI sees exactly what the human drew
- Annotations are ephemeral (not persisted) unless explicitly saved via `configure({what: 'store'})`

**Effort:** ~2 weeks. Extension-side: canvas overlay, drawing tools, annotation serialization, screenshot compositing. Go-side: new annotate action in interact, annotation data passthrough, screenshot integration.

---

## What's NOT on This Roadmap (and why)

| Dropped Feature | Why |
|----------------|-----|
| Backend log streaming | Network bodies already show error responses. AI has Bash to grep backend code. |
| Test execution capture | AI runs `npm test` via Bash and sees output directly. |
| Application Events API | Requires devs to instrument code for Kaboom. High friction. |
| Request tracing | Network waterfall + bodies already cover this. |
| Code navigation | IDE (Claude Code, Cursor) already has full file access. |
| Dev environment control | AI already has Bash for restart, env vars, etc. |
| Doom loop detection | Should be in the AI agent layer, not the browser extension. |
| Edge case registry | Manual curation nobody will maintain. |
| Enterprise features | Premature. Prove value first. |
| v7 full-stack | Premature. v5.8 + these remaining features covers the real gaps. |

---
feature: agent-supervision
status: shipped
doc_type: qa-plan
feature_id: feature-agent-supervision
last_reviewed: 2026-09-05
---

# Agent supervision surface — QA Plan

## Behaviours under test

| Behaviour | Test file |
| --- | --- |
| Driving state starts false; a fresh page shows nothing. | `tests/extension/content/agent-indicator.test.js` |
| Starting drives once; relabelling (a second `startDriving` call) updates the action without restarting the session. | `tests/extension/content/agent-indicator.test.js` |
| `stopDriving` clears the cursor so no ghost pointer is left behind. | `tests/extension/content/agent-indicator.test.js` |
| `snapshot()` returns a copy the caller cannot mutate back into the core. | `tests/extension/content/agent-indicator.test.js` |
| Phantom cursor tracks the target coordinate; refuses to move while not driving. | `tests/extension/content/agent-indicator.test.js` |
| Heartbeat holds the overlay open while beats keep arriving; expiry fires once the TTL is exceeded and is reported only once, not on every later tick; ticks are inert while not driving; exactly at the TTL boundary the overlay is still held. | `tests/extension/content/agent-indicator.test.js` |
| Stop is gated on `event.isTrusted`: a synthetic click is refused, a real gesture is honoured, a missing/malformed event is refused rather than defaulted. | `tests/extension/content/agent-indicator.test.js` |
| `drivingLabel` names the action, reads as prose for multi-word names, falls back to a bare statement for an empty action, truncates a hostile (over-length) action name instead of overflowing the pill. | `tests/extension/content/agent-indicator.test.js` |
| Overlay exposes stable element ids and a z-index above page content. | `tests/extension/content/agent-indicator.test.js` |
| Heartbeat cadence (5,000 ms) is strictly shorter than the overlay TTL (15,000 ms), and specifically survives one dropped beat. | `extension/background/__tests__/driving-session.test.js` |
| Heartbeats are sent for as long as driving continues; stop heartbeating and leak no timer once driving ends; a second `start` on the same tab does not stack a second heartbeat timer. | `extension/background/__tests__/driving-session.test.js` |
| A user stop aborts the tab's CDP session; a stop is reported to the caller exactly once (`consumeStopRequest` true then false); a stop on one tab does not abort another; stopping clears the overlay and the heartbeat; a stop for an idle tab is inert. | `extension/background/__tests__/driving-session.test.js` |
| Driving notices name the action so the pill is never blank; cursor updates are forwarded only while driving; `stop` is idempotent and does not emit a second `idle`. | `extension/background/__tests__/driving-session.test.js` |
| `STOPPED_BY_USER` is a named terminal state (`stopped_by_user...`); an unrequested stop is never fabricated by `consumeStopRequest`. | `extension/background/__tests__/driving-session.test.js` |
| `CDPSessionManager.abort` invalidates every outstanding lease so the in-flight action's next `send` fails loud, and detaches immediately rather than after the idle grace; a later action reattaches cleanly after an abort; aborting a tab with no session is inert; abort on one tab does not touch another tab's session. | `extension/background/__tests__/cdp-session.test.js` (describe block `CDPSessionManager — user abort`) |
| Focus emulation is cleared when the user presses Stop (interacts with the same abort path). | `extension/background/__tests__/cdp-session.test.js` (describe block `CDPSessionManager — focus emulation`) |
| The overlay stripper selects by the `data-kaboom-overlay` marker attribute, not a hardcoded id list, and `kaboom-draw-toolbar` (the id that was never created) is absent from the source. | `tests/extension/content/overlay-capture-stripping.test.js` |
| Every known overlay root (`tracked-hover-launcher.ts`, `lifecycle-overlay.js`, `agent-indicator.ts`) sets the marker on its root. | `tests/extension/content/overlay-capture-stripping.test.js` |
| The draw overlay's real id (`kaboom-draw-overlay`) still exists, guarding against a silent rename. | `tests/extension/content/overlay-capture-stripping.test.js` |

`tests/extension/draw-mode/draw-mode-fixture.js` is a shared DOM/Chrome/timer mock module consumed by draw-mode tests; it contains no assertions of its own and is not a supervision-specific test.

## Manual verification

- Kill-switch UAT (`kaboom-fs9k.4`) — a human confirms the Stop button interrupts a live agent session in a real browser tab, not just in the unit-test fixtures.

## Not covered today

- No test drives an actual MV3 service-worker termination; heartbeat-expiry coverage is entirely through the injected clock in `AgentIndicatorCore`/`AgentIndicatorCore.tick()`, which proves the timing logic but not real Chrome service-worker lifecycle behaviour.
- No test renders the shadow-DOM overlay itself (glow opacity, pill layout, cursor SVG positioning) — coverage stops at the `AgentIndicatorCore` state machine and the `AgentIndicator` class's public methods; this repo has no jsdom, so anything that only exists inside a DOM callback is untested here.
- No listed test exercises `executeCDPAction`'s stop-consumption path (the `driving.consumeStopRequest(tabId) ? STOPPED_BY_USER : mapCDPError(err)` branch used by the `hardware_click` / direct-CDP-action path) directly — `driving-session.test.js`'s "a user stop must not be re-run through the DOM fallback" test asserts `STOPPED_BY_USER`'s shape and that an unrequested stop is never fabricated, but does not simulate a live `executeCDPAction` failure to prove the branch is taken.
- No test asserts the wiring in `src/background/init.ts` (`createSupervisionMessageHandler()` actually being registered via `installBackgroundMessageHandlers`) or in `src/content/runtime-message-listener.ts` (the `kaboom_agent_indicator` dispatch table entry) beyond what the source-contract style tests in this repo check structurally — no runtime integration test sends a real `chrome.runtime` message end-to-end through both files.

## See also

- [./index.md](./index.md)
- [./product-spec.md](./product-spec.md)
- [./tech-spec.md](./tech-spec.md)

---
feature: agent-supervision
status: shipped
doc_type: tech-spec
feature_id: feature-agent-supervision
last_reviewed: 2026-09-05
---

# Agent supervision surface — Tech Spec

## Components

| Component | File | Responsibility |
| --- | --- | --- |
| `AgentIndicatorCore` | `src/content/ui/supervision/agent-indicator.ts` | Pure state machine: driving flag, action label, cursor position, heartbeat clock, TTL expiry. No DOM. |
| `AgentIndicator` | `src/content/ui/supervision/agent-indicator.ts` | Mounts/unmounts the shadow-DOM overlay (glow, pill, stop button, phantom cursor) and paints it from `AgentIndicatorCore` state. |
| `isHonouredStop` | `src/content/ui/supervision/agent-indicator.ts` | Gate: a stop is only honoured when `event.isTrusted === true`. |
| `drivingLabel` | `src/content/ui/supervision/agent-indicator.ts` | Formats the pill text; truncates action names over 40 chars to 39 chars + `…`. |
| Message contracts | `src/types/runtime/agent-indicator.ts` | `AgentIndicatorMessage` (background→content, phases `driving`/`idle`/`cursor`/`heartbeat`) and `AgentStopRequestMessage` (content→background). |
| Content listener | `src/content/runtime-message-listener.ts` | Owns the single `AgentIndicator` instance per page (`ensureAgentIndicator`), routes incoming phase messages (`handleAgentIndicatorMessage`), and polls `indicator.tick()` every `AGENT_INDICATOR_TICK_MS` (2,000 ms). |
| `DrivingSessions` | `src/background/supervision/driving-session.ts` | Per-tab driving state in the background: starts/relabels a session, runs the 5 s heartbeat, and owns `requestStop`/`consumeStopRequest`. |
| `createSupervisionMessageHandler` | `src/background/supervision/driving-session.ts` | Background message handler for `kaboom_agent_stop_requested`, registered in `src/background/init.ts` via `installBackgroundMessageHandlers`. |
| `sendAgentIndicator` / `setKaboomOverlayVisibility` | `src/background/ui/content-script-bridge.ts` | Fire-and-forget phase delivery to the tab, and marker-based overlay hide/show before a screenshot. |
| CDP integration | `src/background/dom/cdp/cdp-dispatch.ts` | `tryCDPEscalation` (selector/gesture actions) and `executeCDPAction` (a coordinate-addressed `click` and the other direct CDP actions) both call into `drivingSessions()`. |
| Draw-mode overlay | `src/content/draw-mode/lifecycle-overlay.js` | A second overlay root (`kaboom-draw-overlay`) that also carries `data-kaboom-overlay`, proving the marker is not agent-indicator-specific. |
| `tracked-hover-launcher` | `src/content/ui/tracked-hover-launcher.ts` | A third overlay root (`kaboom-tracked-hover-launcher`) carrying the same marker. |

## Data and control flow

### Starting a driving session

1. `tryCDPEscalation` or `executeCDPAction` resolves the CDP-escalatable action's target and calls `drivingSessions().start(tabId, action)`.
2. `DrivingSessions.start`: if the tab has no session, creates one, starts a `setInterval` heartbeat at `HEARTBEAT_INTERVAL_MS` (5,000 ms), and calls `notify(tabId, 'driving', { action })` → `sendAgentIndicator`. If the tab already has a session, it only relabels `session.action` and re-notifies — **no second timer is started**, so a burst of actions on one tab does not stack heartbeat intervals or leak one on stop.
3. The content listener's `handleAgentIndicatorMessage` receives the `driving` phase, calls `indicator.startDriving(action)` (which mounts the shadow-DOM overlay if absent) and starts the 2 s self-tick poll if one is not already running.
4. Before dispatching input, the background also calls `driving.cursor(tabId, x, y)`, which is forwarded only while the tab has an active session (`cursor()` is a no-op otherwise) and moves the phantom cursor via the `cursor` phase.

### Heartbeat and expiry

- Background: every 5,000 ms while a session is open, `sendAgentIndicator(tabId, 'heartbeat')` fires.
- Content: `AgentIndicatorCore.heartbeat()` resets `lastHeartbeatAt = now()`. Every 2,000 ms the listener calls `indicator.tick()`, which calls `AgentIndicatorCore.tick()`: if `now() - lastHeartbeatAt > HEARTBEAT_TTL_MS` (15,000 ms) it stops driving locally and returns `'heartbeat_expired'`; otherwise it returns `null`. On a non-null result the content script clears its own poll timer and the overlay is already unmounted (`AgentIndicator.tick()` calls `unmount()` when `core.tick()` returns a reason).
- `HEARTBEAT_INTERVAL_MS < HEARTBEAT_TTL_MS`, and specifically `HEARTBEAT_INTERVAL_MS * 2 <= HEARTBEAT_TTL_MS` (5,000 × 2 = 10,000 ≤ 15,000): a single dropped heartbeat cannot expire the overlay.

### Stopping

1. User clicks the Stop button in the shadow DOM. The click handler checks `isHonouredStop(event)` — only proceeds if `event.isTrusted === true`.
2. On an honoured click, `AgentIndicator.stopDriving()` runs locally (removes cursor/glow/pill immediately) and `deps.onStop()` sends `{ type: 'kaboom_agent_stop_requested', at: Date.now() }` to the background via `chrome.runtime.sendMessage`.
3. `createSupervisionMessageHandler` reads the tab id from **`sender.tab?.id`**, never from the message body — a content script can only speak for its own tab. A missing tab id responds `{ success: false, error: 'stop_without_tab' }` and does nothing else.
4. `DrivingSessions.requestStop(tabId)`: if no session exists for the tab, it is a no-op (a race where the action already finished, not an error). Otherwise it marks `stopRequested`, calls `abortSession(tabId, 'stopped_by_user')` → `cdpSessions()?.abort(tabId, 'stopped_by_user')`, clears the heartbeat interval, deletes the session, adds the tab id to the internal `stopRequests` set, and sends the `idle` phase.
5. `CDPSessionManager.abort` invalidates every outstanding lease for that tab immediately (not after the idle grace that a normal `release()` waits out), so the in-flight action's next `lease.send(...)` rejects.
6. Inside `tryCDPEscalation`'s `catch` block (and the equivalent in `executeCDPAction`), the code calls `drivingSessions().consumeStopRequest(tabId)`. If it returns `true` (the stop was pending for this tab and has now been consumed exactly once), the function returns a result with `error: STOPPED_BY_USER` instead of `null`. Returning `null` here would have meant "CDP did not handle this," which sends the caller down the DOM-fallback path and **re-executes the very action being stopped**.

### Screenshot / capture overlay stripping

- `setKaboomOverlayVisibility(tabId, visible)` (`src/background/ui/content-script-bridge.ts`) runs `chrome.scripting.executeScript` in the tab, selecting every element via `document.querySelectorAll('[data-kaboom-overlay]')`.
- To hide: if the element does not already carry the stash attribute `data-kaboom-display-before-capture`, its current inline `display` is stashed there and `display` is set to `'none'`.
- To restore: the stashed value is written back to `style.display` and the stash attribute is removed. The original code forced `display: flex` on restore, which silently rewrote the layout of any overlay that was not a flex container; the stash/restore pair fixes that.
- Every overlay root — `kaboom-agent-indicator` (this feature), `kaboom-tracked-hover-launcher`, and `kaboom-draw-overlay`/`kaboom-draw-badge`/`kaboom-draw-instruction` (draw mode, via the shared `kaboom-draw-overlay` root) — sets `data-kaboom-overlay` on its root. A new overlay that forgets the marker is caught by `tests/extension/content/overlay-capture-stripping.test.js`, not by code review alone.
- Callers: `src/background/ui/tracked-tab-state.ts` and `src/background/message-routing/capture-handler.ts` toggle visibility around each capture.

## Invariants

| Invariant | Enforced by |
| --- | --- |
| A stop is only honoured on a trusted DOM event. | `isHonouredStop` checked in the button's click listener before anything else runs. |
| The tab a stop applies to comes from the message sender, never the message body. | `createSupervisionMessageHandler` reads `sender.tab?.id`. |
| A consumed stop is reported to the interrupted action exactly once. | `DrivingSessions.consumeStopRequest` deletes from a `Set<number>` on read. |
| Relabelling an in-progress session never stacks a second heartbeat timer. | `DrivingSessions.start` checks for an existing session before creating an interval. |
| `tryCDPEscalation`/`executeCDPAction` never return `null` for a consumed stop. | Explicit `consumeStopRequest` check inside each `catch` block, returning a `STOPPED_BY_USER` result. |
| Every overlay root is strippable before a capture, by construction. | `data-kaboom-overlay` marker attribute, not an id list. |
| Cursor updates are ignored for a tab that is not being driven. | `DrivingSessions.cursor` and `AgentIndicatorCore.moveCursor` both short-circuit when not driving. |

## Failure modes

| Failure | Behaviour |
| --- | --- |
| Background service worker dies mid-action (MV3 can terminate it without warning). | No further heartbeats arrive; the content-side 2 s poll finds `now() - lastHeartbeatAt > 15,000 ms` and unmounts the overlay locally with `TeardownReason: 'heartbeat_expired'`. |
| The tab has no content script (restricted page, mid-navigation, closed tab) when the background calls `sendAgentIndicator` or `setKaboomOverlayVisibility`. | Both are fire-and-forget; the rejected `chrome.tabs.sendMessage` promise is caught and swallowed (documented `EXPECTED_ABSENCE`). The action itself is unaffected — the overlay is decoration around the action, not a precondition of it. |
| User presses Stop for a tab whose action already finished (race). | `DrivingSessions.requestStop` finds no session and returns without calling `abortSession`; `consumeStopRequest` later returns `false`. |
| A page dispatches a synthetic `click` on the Stop button. | `isHonouredStop` returns `false`; nothing happens; the agent keeps driving. |
| A page crafts `{ type: 'kaboom_agent_stop_requested' }` from a different origin/frame. | The handler still resolves the tab from `sender.tab?.id`, which is the tab the message actually came from — a page cannot name a different tab to stop. |
| `chrome.runtime.sendMessage` for the stop request itself fails (e.g. background already dead). | Caught and swallowed with an `EXPECTED_ABSENCE` comment — the overlay has already torn itself down locally, so the user-visible outcome (agent no longer appears to be driving) is correct even though the background never received the message. |

## See also

- [./index.md](./index.md)
- [./product-spec.md](./product-spec.md)
- [./qa-plan.md](./qa-plan.md)

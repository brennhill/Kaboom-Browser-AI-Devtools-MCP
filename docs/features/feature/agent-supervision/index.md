---
doc_type: feature_index
feature_id: feature-agent-supervision
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-03
code_paths:
  - src/content/ui/agent-indicator.ts
  - src/content/runtime-message-listener.ts
  - src/types/runtime/agent-indicator.ts
  - src/background/ui/content-script-bridge.ts
  - src/background/dom/cdp/cdp-dispatch.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/content/draw-mode/lifecycle-overlay.js
test_paths:
  - tests/extension/content/agent-indicator.test.js
  - tests/extension/content/overlay-capture-stripping.test.js
  - tests/extension/draw-mode/draw-mode-fixture.js
---

# Agent supervision surface

Shows the person whose tab it is what the agent is about to do, and gives them a way to stop it.

## Why this exists

Since `kaboom-05ue.1`, kaboom drives with trusted CDP input over a session that outlives a
single action. Before this feature the product had per-action toasts, which narrate what
*already happened*, and **no stop control anywhere** — a `grep` for a kill switch returned
nothing outside a sidepanel comment. Open bead `kaboom-fs9k.4` asked for kill-switch UAT
against a kill switch that did not exist.

## The four parts

| Part | Behaviour |
| --- | --- |
| Phantom cursor | Animates to the resolved target **before** input dispatches, so the user sees intent rather than history. Coordinates come from `resolveElement()`. |
| Driving indicator | Viewport-edge glow plus a pill naming the action, held for the **lifetime of the CDP lease**, not per action. |
| Stop control | Aborts the action and releases the lease. Gated on `event.isTrusted`. |
| Heartbeat | The overlay removes *itself* if heartbeats stop. |

### Why the stop button is gated on `isTrusted`

A page can dispatch a synthetic click on any element in its own document. Without the gate a
hostile page could abort the agent at will — or fire the stop constantly and strip the
control of meaning. Only a real user gesture carries `isTrusted: true`.

### Why the heartbeat exists

MV3 terminates the service worker without warning. If it dies mid-action, no further
heartbeats arrive and the overlay tears itself down. Without it the user keeps a permanent
"an agent is driving this tab" badge on a tab nothing is driving — the same staleness failure
that left `TERMINAL_UI_STATE='open'` forever (CLAUDE.md rule 18).

## Design note: no approval gates

This surface is deliberately **reactive**. It reports and offers an escape hatch; it never
blocks an action waiting for permission. An earlier per-origin approval gate was built and
reverted (`d27ebacfd`) because prompts are friction rather than protection. Do not add a
blocking prompt to the driving path.

## Overlay capture stripping (bug fixed here)

`setKaboomOverlayVisibility` hid a hardcoded list of two element ids,
`['kaboom-tracked-hover-launcher', 'kaboom-draw-toolbar']`. **Nothing in the codebase ever
created `kaboom-draw-toolbar`** — the draw overlay's real roots are `kaboom-draw-overlay`,
`kaboom-draw-badge` and `kaboom-draw-instruction`. So every screenshot taken while draw mode
was active captured Kaboom's own overlay, and the agent then reasoned about its own UI as
page content.

Overlays are now selected by a `data-kaboom-overlay` marker attribute, which a new overlay
cannot forget the way a list can. The stripper also stashes and restores the original inline
`display`; the previous code forced `flex` on restore, silently rewriting the layout of any
overlay that was not a flex container.
`tests/extension/content/overlay-capture-stripping.test.js` keeps the invariant true.

## Related

- `kaboom-05ue.1` — the persistent CDP session whose lease defines "driving"
- `kaboom-fs9k.4` — kill-switch UAT, whose missing half this provides

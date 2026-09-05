---
feature: agent-supervision
status: shipped
doc_type: product-spec
feature_id: feature-agent-supervision
last_reviewed: 2026-09-05
---

# Agent supervision surface — Product Spec

## What the user gets

- A visible signal, on the tab itself, that Kaboom is driving it right now — not a log line, not a toast after the fact.
- A phantom cursor that moves to the coordinate Kaboom is about to act on, before the click or keystroke lands.
- A Stop button on the page that, when clicked, actually interrupts the agent's current action.
- Automatic cleanup: if the extension's background worker dies mid-action, the indicator removes itself within 15 seconds instead of staying stuck on-screen.

## Why this exists

Since `kaboom-05ue.1`, Kaboom drives pages with trusted CDP (Chrome DevTools Protocol) input over a session that outlives a single action — one CDP lease can span several clicks and keystrokes. Before this feature:

- The only feedback was per-action toasts, which report what *already happened*.
- There was no stop control anywhere in the product. A `grep` for a kill switch across the codebase returned nothing outside a comment in the sidepanel.
- Open bead `kaboom-fs9k.4` asked for kill-switch UAT against a kill switch that did not exist.

An earlier version of this surface (reviewed at commit `43f2dfb16`) rendered correctly but did not work: the Stop button's message had no listener, so pressing it removed the overlay while the agent kept driving, and nothing ever sent a heartbeat, so the overlay's self-teardown timer — meant to catch a dead background worker — was the only thing that could ever remove a *live* overlay. Commit `21e713277` fixed both defects and added heartbeats to the `hardware_click` path, which previously drove the page with no indicator at all.

## Requirements this satisfies

- The person whose tab is being driven can see that it is happening, for the duration of the driving session, not as a one-off notification.
- The person can stop the agent from a real click, and that click has to actually stop something — not just hide the indicator.
- A dead extension background worker must not leave a permanent "an agent is driving this tab" badge on a tab nothing is driving.
- A malicious page must not be able to fake a stop click, or stop a tab it does not own.

## What it deliberately does NOT do

- **No approval gates.** The surface is reactive: it shows what is happening and offers an escape hatch. It never blocks an action waiting for the user to approve it first. An earlier per-origin approval gate was built and reverted (`d27ebacfd`) because prompts are friction rather than protection. This surface must not gain a blocking prompt on the driving path.
- **No retry after a user stop.** A stop is reported to the agent as the terminal state `stopped_by_user`, which is deliberately not retryable — the agent needs to know a person intervened, not that the browser glitched.
- **Not a permission system.** It supervises input dispatched over CDP (`tryCDPEscalation` and `executeCDPAction` / `hardware_click`); it does not gate or audit every browser action Kaboom takes.

## What it replaced

- Per-action toasts as the only signal of agent activity on a tab — those still exist for other purposes but are no longer the only thing standing between the user and an unsupervised agent.
- A hardcoded, two-entry element-id list (`kaboom-tracked-hover-launcher`, `kaboom-draw-toolbar`) used to hide Kaboom's own UI before a screenshot. `kaboom-draw-toolbar` was never created anywhere in the codebase, so every screenshot captured during draw mode contained Kaboom's own draw overlay, and the agent then reasoned about its own UI as if it were page content. This is now a `data-kaboom-overlay` marker attribute that every overlay root sets, so a new overlay cannot be forgotten by an id list the way the draw overlay was.

## See also

- [./index.md](./index.md)
- [./tech-spec.md](./tech-spec.md)
- [./qa-plan.md](./qa-plan.md)

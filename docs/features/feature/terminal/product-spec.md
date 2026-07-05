---
doc_type: product_spec
feature_id: feature-terminal
status: shipped
last_reviewed: 2026-07-05
owners:
  - Brenn
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Product Spec

## Objective

Provide a reliable terminal side panel for Kaboom users that stays usable during debugging, annotation, and automation workflows without disrupting the primary MCP daemon, while binding the panel to one tab-group-backed work context instead of arbitrary tabs.

## Scope

- Terminal side panel UI with open/minimized/closed states.
- One Kaboom work context maps to one Chrome tab group.
- Hover launcher remains the page overlay for quick actions on tracked workspace pages, but the terminal button opens the side panel on the active workspace tab and hides the launcher only while the panel is open.
- The current side panel rollout is terminal-only so the terminal can use the full panel height.
- Dedicated terminal HTTP server on `main_port + 1`.
- PTY-backed singleton terminal session across tabs.
- Typing-aware queue guard for auto-writes from annotation workflows.
- Reconnect-safe submit behavior and lightweight redraw recovery.

## User Outcomes

- Users get one consistent Kaboom workspace instead of a terminal that appears attached to unrelated tabs.
- Users can keep terminal context visible in the side panel while interacting with tabs inside the active workspace group.
- Auto-generated terminal commands do not interrupt active typing.
- Terminal sessions survive page refreshes and reconnect cleanly.
- Minimizing the panel hides it without destroying the active terminal session.
- Terminal transport failures do not bring down the main MCP daemon.

## Non-Goals

- Multi-tenant terminal sessions per tab (current model is singleton).
- Cloud terminal hosting (all terminal behavior is local-first).
- Replacing full IDE terminal features.
- Full tab-group migration of every tracked-tab feature in the extension. The initial rollout only moves terminal workspace ownership and panel targeting to the tab-group model.
- Shipping the action-builder palette in this rollout. The upper panel area is intentionally deferred.

## Linked Specs

- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Feature Index: [index.md](./index.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)
- Canonical Flow Map: [terminal-server-isolation.md](../../../architecture/flow-maps/terminal-server-isolation.md)

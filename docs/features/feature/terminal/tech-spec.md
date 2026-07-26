---
doc_type: tech_spec
feature_id: feature-terminal
status: shipped
last_reviewed: 2026-07-05
owners:
  - Brenn
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Tech Spec

## Primary Components

- `cmd/browser-agent/terminal_server.go`
- `cmd/browser-agent/terminal_handlers.go`
- `cmd/browser-agent/terminal_assets/terminal.html`
- `extension/sidepanel.html`
- `extension/sidepanel.js`
- `src/background/tab-state.ts`
- `src/content/ui/terminal-panel-bridge.ts`
- `internal/pty/manager.go`
- `internal/pty/session.go`
- `src/content/ui/terminal-widget-session.ts`
- `src/content/ui/terminal-widget-types.ts`

## Core Contracts

- Terminal server runs on `main_port + 1` to isolate WebSocket behavior from MCP request timeouts.
- Terminal session is singleton and restored via `chrome.storage.session`.
- Terminal panel ownership is resolved through a Kaboom work context backed by one Chrome tab group.
- The initial rollout keeps broader tracking flows on `TRACKED_TAB_*`, but terminal open/close targeting is driven by workspace-group resolution.
- The hover launcher keeps the page overlay for screenshots/recording, but terminal visibility is controlled by the side panel and `TERMINAL_UI_STATE`.
- Auto-write queue must defer sends while user typing/focus is active.
- Queued submit must wait for reconnect if WebSocket is disconnected.
- Redraw recovery must not terminate PTY session.
- The current side panel host is terminal-only so the iframe can consume the full available panel height.
- `open_terminal_panel` must preserve the original user-gesture path while resolving the correct workspace host tab.
- Power must close the panel and terminate the PTY session; minimize must close the panel but preserve the current PTY session.

## Failure Modes

- Port bind conflict on terminal server startup.
- WebSocket disconnect during queued auto-submit.
- Corrupted panel geometry or terminal canvas requiring redraw.
- Stale persisted session token after daemon restart.
- Workspace group missing or stale after tabs are regrouped or closed.

## Linked Specs

- Product Spec: [product-spec.md](./product-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Feature Index: [index.md](./index.md)

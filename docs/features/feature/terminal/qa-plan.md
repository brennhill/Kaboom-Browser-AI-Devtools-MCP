---
doc_type: qa_plan
feature_id: feature-terminal
status: shipped
last_reviewed: 2026-07-05
owners:
  - Brenn
last_verified_version: 0.8.1
last_verified_date: 2026-03-21
---

# QA Plan

## Automated Gates

1. `go test ./cmd/browser-agent -run Terminal -count=1`
2. `go test ./internal/pty/...`
3. `node --test tests/extension/terminal-sidepanel/sidepanel-terminal.test.js`
4. `npm run docs:check:strict`

## Manual Checks

1. Open the terminal side panel, minimize the terminal region, restore it, and close the browser side panel.
2. Verify redraw (`↻`) reloads the iframe without killing the session.
3. Type in terminal while annotation auto-write is triggered and confirm queued behavior.
4. Simulate WebSocket disconnect during queued submit and confirm submit resumes after reconnect.
5. Confirm terminal health reports `terminal_port` when running and `0` when unavailable.

## Lifecycle UAT (run before every release)

This sequence exercises the cross-context terminal↔launcher lifecycle that unit
tests can only cover in pieces (side panel, background port, content flame each
mock a different world). Two real bugs lived exactly here. Track a tab first.

| # | Step | Pass criteria |
|---|------|---------------|
| L1 | On a tracked page, confirm the **flame** launcher is visible (bottom-right). | Flame shows on a tracked tab with the terminal closed. |
| L2 | Open the terminal (flame → Terminal, **popup → Open Terminal Panel**, keyboard, or context menu). | Panel opens; the flame hides while it is open. |
| L3 | Click **Browse**, pick a folder, click **Reload / Use folder**. | Terminal restarts **in the chosen folder** — a live prompt in that cwd, not a dead/old session. (Regression: `bootTerminalPanel(forceFresh)` must unmount first — `sidepanel-terminal.test.js` "applyRootFolder … end-to-end".) |
| L4 | Try a **non-existent / unwritable folder**. | A visible error explaining the failure — never a silent no-op or a silent reconnect to the old cwd. (See the 409→distinct-error contract.) |
| L5 | Close the panel with **Chrome's own side-panel X** (not the in-panel close). | Back on the page, the **flame reappears** within ~1s. (Regression: the background resets `TERMINAL_UI_STATE` on panel-port disconnect — `terminal-panel-presence.test.js` "port connect/disconnect mirrors …".) |
| L6 | Reload the extension at `chrome://extensions` with a tracked tab open, then revisit the tab. | Flame still mounts (setup errors must not block the mount). |
| L7 | Annotate the page (draw mode → Escape) with the terminal **closed**. | Terminal is **not** force-opened; annotations still reach the AI via `analyze`. With the terminal **open**, the prompt auto-pastes. |
| L8 | Run `--doctor` after cycling installs. | Reports the daemon restart count / last reason (churn is visible, not hidden in a mislabeled `crash.log`). |

## Regression Focus

- Focus theft while user is typing in xterm.
- Frame-write concurrency corruption under ping/output/control traffic.
- PTY session loss across page refresh.
- Main daemon stability when terminal bind/start fails.

## Linked Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- Feature Index: [index.md](./index.md)
- Canonical Flow Maps:
  - [terminal-side-panel-host.md](../../../architecture/runtime/terminal-side-panel-host.md)

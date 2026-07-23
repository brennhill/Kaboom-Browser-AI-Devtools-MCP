---
doc_type: flow_map
status: active
last_reviewed: 2026-03-28
owners:
  - Brenn
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Terminal Side Panel Host and Launcher Coordination

## Scope

This flow covers the terminal side panel host, the page hover launcher terminal button, the workspace-group resolver that decides which tab should host the panel, and the bridge that keeps launcher visibility in sync with side panel open/closed state.

The terminal server isolation flow remains a separate concern and is still documented in [Terminal Server Isolation](./terminal-server-isolation.md).

## Entrypoints

- `src/content/ui/tracked-hover-launcher.ts`
- `src/content/ui/terminal-panel-bridge.ts`
- `src/background/terminal-panel.ts`
- `src/background/keyboard-shortcuts.ts`
- `src/background/context-menus.ts`
- `src/background/message-handlers.ts`
- `src/background/tab-state.ts`
- `src/types/runtime-messages.ts`
- `src/sidepanel.ts`
- `extension/manifest.json`
- `extension/sidepanel.html`

## The Gesture Constraint

`chrome.sidePanel.open()` only runs while a user gesture is active, and **not every gesture counts**. Chrome hands `runtime.onMessage` listeners a *restricted* gesture, which `sidePanel.open()` rejects on some Chrome/Brave builds ([crbug 355266358](https://issues.chromium.org/issues/355266358)). The in-page launcher button reaches the background through exactly that path, so it can never be the only way in.

| Entry point | Gesture | Tab available synchronously | Dependable |
|-------------|---------|------------------------------|------------|
| `commands.onCommand` (`open_terminal_panel`, unbound by default) | full | yes — listener arg | yes |
| `contextMenus.onClicked` ("Open Kaboom Terminal") | full | yes — listener arg | yes |
| `runtime.onMessage` (launcher button, popup) | restricted | only if the sender supplies `tab_id` | no — best effort |

All three call the single shared opener `openTerminalSidePanel()` in `src/background/terminal-panel.ts` (repo rule 19). It has one hard rule: **nothing may be awaited before `chrome.sidePanel.open()`**, because any await expires the gesture. Workspace grouping and `setOptions()` run afterward as best-effort refinement; the panel loads fine without them via the manifest `default_path` plus the active-tab fallback in `sidepanel.ts`.

## Primary Flow

1. The user clicks the terminal button in the tracked hover launcher.
2. The content script sends `open_terminal_panel` to the background worker.
3. The background worker resolves the Kaboom work context:
   - if a workspace tab group already exists, it uses that group
   - if the tracked tab is ungrouped, it creates a named Kaboom tab group around it
   - if the request came from outside the workspace group, it activates the main workspace tab and opens there
4. The background worker calls `chrome.sidePanel.open()` immediately in that same user-gesture path for the resolved workspace host tab; any `setOptions()` work is best-effort and must not block the open call.
5. The side panel page boots, validates or restores the singleton terminal session, and renders the terminal shell at full panel height.
6. The side panel writes `TERMINAL_UI_STATE` to session storage as `open`, `minimized`, or `closed`.
7. The launcher bridge observes that state and hides the hover overlay only while the panel is open.
8. `minimizePanel()` closes the browser side panel but preserves `TERMINAL_SESSION`.
9. `exitTerminalSession()` stops the PTY session, clears persisted session state, closes the browser side panel, and remounts the launcher.
10. When the launcher emits annotation-driven terminal text, it forwards `terminal_panel_write` to the side panel host.

## Error and Recovery Paths

- If `chrome.sidePanel.open()` fails, `openTerminalPanel()` reports the Chrome error verbatim via `console.error` **and** an error toast naming the two gesture-native fallbacks. It previously did `catch { return false }`, so a rejected open produced no console output, no toast, and no captured error — the Terminal button was indistinguishable from a dead element and there was nothing to diagnose from.
- If the stored workspace group is stale, the background worker should rebuild it around the tracked tab before opening the panel.
- If the terminal daemon is unavailable, the side panel should show an inline unavailable state rather than mounting a page overlay, and startup guidance should point at `npx kaboom-agentic-browser`.
- If the persisted session token is stale, the side panel clears persisted state and starts a fresh PTY session.
- If the panel closes mid-write, queued writes are reset instead of replayed into a closed host.

## State and Contracts

- `TERMINAL_SESSION` stores `{ sessionId, token }` in `chrome.storage.session`.
- `TERMINAL_UI_STATE` is the source of truth for panel visibility.
- Workspace ownership is stored separately from raw tracked-tab state so the panel can stay group-scoped while the rest of the extension is still tracked-tab scoped.
- `terminal_panel_write` is the runtime message that carries terminal text from the page launcher path to the panel host.
- `open_terminal_panel` is both the runtime message (launcher/popup) and the manifest command id (keyboard). The message accepts an optional `tab_id`, which extension pages must supply because `sender.tab` is undefined there.
- `TERMINAL_PANEL_FALLBACK_HINT` in `src/lib/constants.ts` is what the content-script toast shows when the panel refuses to open.
- **Chrome allows at most four commands with a `suggested_key`** and rejects the *entire* manifest past that ("Too many shortcuts specified for 'commands': The maximum is 4") — the extension then fails to load completely. Four are already taken, so `open_terminal_panel` ships unbound. `tests/extension/manifest-command-limits.test.js` enforces the cap.
- The launcher must not mount the terminal iframe in page context.

## Code Paths

- `src/lib/brand.ts`
- `src/content/ui/tracked-hover-launcher.ts`
- `src/content/ui/terminal-panel-bridge.ts`
- `src/background/terminal-panel.ts`
- `src/background/keyboard-shortcuts.ts`
- `src/background/context-menus.ts`
- `src/background/message-handlers.ts`
- `src/background/tab-state.ts`
- `src/lib/constants.ts`
- `src/sidepanel.ts`
- `src/content/ui/terminal-widget-session.ts`
- `src/content/ui/terminal-widget-types.ts`
- `src/content/ui/terminal-widget-ui.ts`
- `extension/manifest.json`
- `extension/sidepanel.html`

## Test Paths

- `tests/extension/brand-metadata.test.js`
- `tests/extension/tracked-hover-launcher.test.js`
- `tests/extension/sidepanel-terminal.test.js`
- `tests/extension/terminal-widget-session-branding.test.js`
- `tests/extension/terminal-widget-ui-branding.test.js`
- `tests/extension/message-handlers.test.js`
- `tests/extension/terminal-panel-gesture-entrypoints.test.js`
- `tests/extension/terminal-panel-open-failure.test.js`

## Edit Guardrails

- Do not reintroduce page-mounted xterm rendering for the terminal.
- Keep launcher visibility controlled by `TERMINAL_UI_STATE`.
- Keep panel open routing workspace-aware; do not reopen the panel on unrelated tabs outside the active Kaboom workspace.
- Keep the terminal session singleton and local-first.
- Keep all terminal shells, including legacy/fallback widget chrome, branded as Kaboom.
- If an action-builder surface is added later, keep it separate from the terminal core instead of reintroducing mixed responsibilities into the terminal host.
- Preserve the direct user-gesture side-panel open path from launcher click through background handler.
- Never await before `chrome.sidePanel.open()` in any entry point.
- Keep at least one gesture-native entry point (keyboard command or context menu). Removing them leaves only the restricted-gesture message path, which Chrome may refuse.
- Never swallow a side-panel open failure; a silent failure is indistinguishable from a dead button.

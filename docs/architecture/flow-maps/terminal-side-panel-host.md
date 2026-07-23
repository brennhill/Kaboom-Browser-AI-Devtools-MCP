---
doc_type: flow_map
status: active
last_reviewed: 2026-07-23
owners:
  - Brenn
last_verified_version: 0.8.5
last_verified_date: 2026-07-23
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

All three call the single shared opener `openTerminalSidePanel()` in `src/background/terminal-panel.ts` (repo rule 19). It has one hard rule: **nothing may be awaited before `chrome.sidePanel.open()`**, because any await expires the gesture. Note the distinction — *dispatching* an async call is free, only awaiting one costs the gesture. The opener uses that to fire `enableTerminalPanelForTab()` immediately before `open()`: Chrome processes both in order, so the tab is available by the time the open lands.

Workspace grouping runs afterward as best-effort refinement; the panel loads fine without it via the manifest `default_path` plus the active-tab fallback in `sidepanel.ts`.

## Availability vs. an Explicit Open

`side_panel.default_path` offers the panel on *every* tab, where it renders empty, so `syncTerminalPanelAvailability()` disables the global default and enables only the tracked tab.

That scoping governs the default only. `chrome.sidePanel.open({tabId})` on a tab where the panel is disabled fails with **"No active side panel for tabId: N"**, which is exactly what the user saw on every untracked page — the Terminal button surfaced a Chrome error instead of a terminal. An explicit request outranks scoping: `openTerminalSidePanel()` enables the target tab before opening it.

**The panel path never varies.** Changing `path` in `setOptions` makes Chrome reload the side panel document. The panel used to be opened with a per-tab `sidepanel.html?tabId=…&tabGroupId=…&mainTabId=…`, set *after* `open()` — so every open booted an xterm and then immediately reloaded the document out from under it. Only `tabId` was ever read, and `getHostTabId()` already falls back to the active tab, so the parameters bought nothing and cost a session.

## Knowing Whether a Panel Exists

The toggle has to decide open-vs-close synchronously (an await would expire the gesture), so the background keeps the answer in a variable. Where that answer comes from matters:

| Source | Sees Chrome's own X dismiss | Verdict |
|--------|------------------------------|---------|
| `TERMINAL_UI_STATE` mirrored from `chrome.storage.session` | no — the document is destroyed with no chance to write | stale "open" forever; the toggle then tried to *close* a panel that was gone, and nothing could reopen it |
| `chrome.runtime.Port` opened by the panel document | yes — Chrome disconnects the port however the document died | current design |

The panel connects `TERMINAL_PANEL_PORT` on boot and reconnects if the service worker restarts. The background holds the port in `livePanelPort`; `isTerminalPanelOpenSync()` is just `livePanelPort !== null`.

`TERMINAL_UI_STATE` still exists and is still the launcher bridge's signal for hover-overlay visibility — it is a UI state, not a liveness check.

## Opening onto a Panel That Already Exists

`chrome.sidePanel.open()` on a live panel merely focuses it; no code runs inside the panel document. A panel sitting minimized, or blank because its `window.close()` was refused, would stay that way and "open" would look broken. So the opener also posts `restore_terminal_panel` over the presence port, and `restoreTerminalPanel()` in `src/sidepanel.ts`:

- re-shows the terminal if one is already mounted;
- rebuilds the panel if it was unmounted, revalidating the token so the xterm reconnects to a shell that is actually alive;
- retries session start if the panel is mounted with no terminal (the daemon was down when it booted).

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

## Choosing the Working Directory

A PTY's cwd is fixed at spawn, so the root folder is not a setting that takes effect later — changing it is a restart. The bar above the terminal makes that visible and reversible:

| Control | Effect |
|---------|--------|
| path field | shows the current root; Enter applies |
| *Browse* | inline directory listing from the daemon, navigable up and down |
| *Reload* | persists the root, stops the running PTY, starts a new one there |

**Why the daemon lists directories.** The browser cannot resolve an absolute path: `<input type="file" webkitdirectory>` exposes only `webkitRelativePath`, and `showDirectoryPicker()` exposes only a handle's `name`. Neither can produce a cwd to spawn a shell in. `GET /terminal/dirs?path=…` (`cmd/browser-agent/internal/terminal/dirs.go`) returns `{path, parent, entries[], truncated}` — directories only, dot-directories hidden, `~` expanded, capped at `MaxDirEntries` with `truncated` set rather than silently trimmed. An unreachable daemon degrades to typing a path.

## Error and Recovery Paths

- If `chrome.sidePanel.open()` fails, `openTerminalPanel()` reports the Chrome error verbatim via `console.error` **and** an error toast naming the two gesture-native fallbacks. It previously did `catch { return false }`, so a rejected open produced no console output, no toast, and no captured error — the Terminal button was indistinguishable from a dead element and there was nothing to diagnose from.
- If the stored workspace group is stale, the background worker should rebuild it around the tracked tab before opening the panel.
- If the terminal daemon is unavailable, the side panel should show an inline unavailable state rather than mounting a page overlay, and startup guidance should point at `npx kaboom-agentic-browser`.
- If the persisted session token is stale, the side panel clears persisted state and starts a fresh PTY session.
- If the panel closes mid-write, queued writes are reset instead of replayed into a closed host.

## State and Contracts

- `TERMINAL_SESSION` stores `{ sessionId, token }` in `chrome.storage.session`.
- `TERMINAL_UI_STATE` drives launcher hover-overlay visibility. It is **not** how the background decides whether a panel exists — see the presence port above.
- `TERMINAL_PANEL_PORT` (`kaboom_terminal_panel`) is the port the panel document holds open for its whole life; its connect/disconnect is the liveness signal.
- `restore_terminal_panel` is posted over that port to bring an existing panel's terminal back.
- Workspace ownership is stored separately from raw tracked-tab state so the panel can stay group-scoped while the rest of the extension is still tracked-tab scoped.
- `terminal_panel_write` is the runtime message that carries terminal text from the page launcher path to the panel host.
- `open_terminal_panel` is both the runtime message (launcher/popup) and the manifest command id (keyboard). The message accepts an optional `tab_id`, which extension pages must supply because `sender.tab` is undefined there.
- `TERMINAL_PANEL_FALLBACK_HINT` in `src/lib/constants.ts` is what the content-script toast shows when the panel refuses to open.
- **Chrome allows at most four commands with a `suggested_key`** and rejects the *entire* manifest past that ("Too many shortcuts specified for 'commands': The maximum is 4") — the extension then fails to load completely. Four are already taken, so `open_terminal_panel` ships unbound. `tests/extension/chrome-platform-limits.test.js` enforces the cap, alongside the other Chrome hard limits.
- The launcher must not mount the terminal iframe in page context.

## Code Paths

- `src/lib/brand.ts`
- `src/content/ui/tracked-hover-launcher.ts`
- `src/content/ui/terminal-panel-bridge.ts`
- `src/background/terminal-panel.ts`
- `src/background/side-panel-availability.ts`
- `src/background/keyboard-shortcuts.ts`
- `src/background/context-menus.ts`
- `src/background/message-handlers.ts`
- `src/background/tab-state.ts`
- `src/lib/constants.ts`
- `src/sidepanel.ts`
- `src/content/ui/terminal-widget-session.ts`
- `src/content/ui/terminal-widget-types.ts`
- `src/content/ui/terminal-widget-ui.ts`
- `src/content/ui/terminal-root-folder.ts`
- `src/content/ui/terminal-panel-states.ts`
- `src/content/ui/terminal-write-guard.ts`
- `cmd/browser-agent/internal/terminal/dirs.go`
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
- `tests/extension/terminal-panel-presence.test.js`
- `tests/extension/terminal-root-folder.test.js`
- `cmd/browser-agent/internal/terminal/dirs_test.go`
- `tests/extension/terminal-panel-close-and-scope.test.js`

## Edit Guardrails

- Do not reintroduce page-mounted xterm rendering for the terminal.
- Keep launcher visibility controlled by `TERMINAL_UI_STATE`.
- Keep panel open routing workspace-aware; do not reopen the panel on unrelated tabs outside the active Kaboom workspace.
- Keep the terminal session singleton and local-first.
- Keep all terminal shells, including legacy/fallback widget chrome, branded as Kaboom.
- If an action-builder surface is added later, keep it separate from the terminal core instead of reintroducing mixed responsibilities into the terminal host.
- Preserve the direct user-gesture side-panel open path from launcher click through background handler.
- Never await before `chrome.sidePanel.open()` in any entry point. This includes *deciding* whether to open: the context-menu toggle reads `isTerminalPanelOpenSync()`, because awaiting a storage read to choose open-vs-close burned the gesture and made "Open Kaboom Terminal" do nothing.
- Never derive panel liveness from storage. A dismissed panel writes nothing, and the resulting stale "open" locks the user out of reopening entirely.
- Keep one constant side panel path. A per-tab path reloads the document and kills the xterm that just booted.
- An explicit open must enable the panel for its target tab first, or it fails with "No active side panel for tabId" wherever availability scoping applies.
- Keep at least one gesture-native entry point (keyboard command or context menu). Removing them leaves only the restricted-gesture message path, which Chrome may refuse.
- Never swallow a side-panel open failure; a silent failure is indistinguishable from a dead button.
- Keep the root folder visible while a session is running. Hiding it behind a failure state means the only way to see where the shell is, is to break it.
- Name the apply control for what it does. A PTY cannot be moved, so "Save" would conceal that the running session is about to be destroyed.

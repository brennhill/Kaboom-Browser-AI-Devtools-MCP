---
feature: tab-tracking-ux
status: proposed
version: null
tool: null
mode: null
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-tab-tracking-ux
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tab Tracking UX Improvements

> Three extension-only UX improvements that make tab tracking state visible, prevent accidental tab switches, and guide recovery when a tracked tab closes.

## Problem

Kaboom's single-tab tracking model (shipped in v5.1.0) silently captures telemetry from one explicitly tracked tab. However, the current UX has three gaps that cause confusion and data loss:

1. **Invisible tracking state.** The extension badge currently shows only server connection status (green with error count, or red "!" for disconnected). There is no visual indicator of *which* tab is being tracked or whether tracking is active at all. A developer glancing at the toolbar cannot tell if Kaboom is capturing their work tab without opening the popup.

2. **Accidental tab switches.** Clicking "Track This Tab" in the popup on a different tab immediately and silently switches tracking to that tab. There is no confirmation step. If a developer opens the popup on the wrong tab and clicks, they lose telemetry from their intended target without any warning. The previous tracking context is gone.

3. **Silent tracking loss on tab close.** When the tracked tab is closed, `background.js` clears `trackedTabId` and `trackedTabUrl` from storage and logs a console message, but the user sees nothing. Tracking simply stops. The developer may continue working, assuming Kaboom is still capturing, only to discover later that all telemetry was lost after the tab close. There is no suggestion to re-attach tracking to another tab.

These three gaps are especially problematic because Kaboom is used alongside AI coding assistants that rely on continuous telemetry. A silent tracking gap means the AI loses context and the developer wastes time reproducing issues.

## Solution

Three coordinated UX improvements, all scoped entirely to the Chrome extension (no server or MCP changes):

**Sub-feature A: Badge Tracking Indicator** — Update the extension badge icon to visually communicate the current tracking state at a glance, without requiring the user to open the popup.

**Sub-feature B: Track Switch Confirmation Dialog** — Show a confirmation dialog when the user attempts to switch tracking from one tab to a different tab, preventing accidental context loss.

**Sub-feature C: Tab Close Recovery Suggestion** — When the tracked tab is closed, notify the user and suggest re-attaching tracking to another open tab, preventing silent telemetry gaps.

## User Stories

- As a developer using Kaboom, I want to see at a glance whether my current tab is being tracked so that I can be confident telemetry is flowing without opening the popup.
- As a developer using Kaboom, I want a confirmation step before switching my tracked tab so that I do not accidentally lose telemetry context.
- As a developer using Kaboom, I want to be notified when my tracked tab closes so that I can immediately re-attach tracking and avoid a gap in capture.
- As an AI coding agent, I want the developer's tracking state to be stable so that my telemetry stream is not unexpectedly interrupted.

## MCP Interface

**This feature has no MCP interface.** All three sub-features are purely extension-side UX improvements. No new MCP tools, modes, or actions are introduced. The existing `observe({what: "tabs"})` response already includes tracking state and is unaffected.

## Requirements

### Sub-feature A: Badge Tracking Indicator

| # | Requirement | Priority |
|---|-------------|----------|
| A1 | The badge must display distinct visual states for: (a) no tab tracked, (b) actively tracking a tab, and (c) server disconnected. | must |
| A2 | When no tab is tracked, the badge must show a grey dash character ("-") with a grey background (#6e7681) to indicate an idle/unconfigured state. | must |
| A3 | When actively tracking a tab and connected to the server, the badge must show a green dot character or empty text with a green background (#3fb950), consistent with the existing connected state. Error counts continue to overlay as they do today. | must |
| A4 | When the server is disconnected, the badge must show "!" with a red background (#f85149), exactly as it does today. Server disconnection always takes visual priority over tracking state. | must |
| A5 | Badge state must update within 200ms of any tracking state change (track, untrack, tab close, server connect/disconnect). | must |
| A6 | The badge tooltip (title text on the extension icon) should describe the current state in plain English, e.g., "Kaboom: tracking tab 42", "Kaboom: not tracking", "Kaboom: server disconnected". | should |
| A7 | Badge updates must not cause visible flicker when multiple state changes occur in rapid succession (e.g., tab close + storage clear). The final state must always be correct. | should |

### Sub-feature B: Track Switch Confirmation Dialog

| # | Requirement | Priority |
|---|-------------|----------|
| B1 | When the user clicks "Track This Tab" while another tab is already being tracked, a confirmation dialog must appear before switching. | must |
| B2 | The confirmation dialog must clearly state which tab is currently being tracked (by URL or title) and which tab will become the new target. | must |
| B3 | The dialog must offer two choices: "Switch Tracking" (confirms the switch) and "Cancel" (keeps the current tracked tab unchanged). | must |
| B4 | If the user cancels, no state change occurs. The popup UI remains unchanged. The previously tracked tab continues to be tracked. | must |
| B5 | The dialog must not appear when starting tracking from a state where no tab is tracked. In that case, tracking begins immediately as it does today. | must |
| B6 | The dialog must not appear when the user clicks "Stop Tracking". Untracking always proceeds immediately. | must |
| B7 | The confirmation dialog should be implemented as an inline HTML element within the popup (not `window.confirm()` or a system dialog) so that it can be styled consistently with the Kaboom dark theme. | should |
| B8 | The dialog should be keyboard-accessible: Enter to confirm, Escape to cancel, and proper focus management (focus moves to dialog on open, returns to trigger button on close). | should |
| B9 | The dialog must include an accessible label (aria-label or aria-labelledby) and role="dialog" for screen readers. | should |

### Sub-feature C: Tab Close Recovery Suggestion

| # | Requirement | Priority |
|---|-------------|----------|
| C1 | When the tracked tab is closed, the extension must notify the user that tracking has stopped, rather than silently clearing state. | must |
| C2 | The notification must be a Chrome notification (via `chrome.notifications` API) that appears even when the popup is not open. Console-only logging is insufficient. | must |
| C3 | The notification message must state that the tracked tab was closed, include the URL or title of the closed tab for context, and suggest opening the Kaboom popup to track a new tab. | must |
| C4 | The notification should auto-dismiss after 8 seconds. Clicking the notification should open the Kaboom popup (if possible via `chrome.action.openPopup()`) or focus the extension icon. | should |
| C5 | If `chrome.action.openPopup()` is not available (it requires Chrome 99+ and the `action` permission), clicking the notification should bring the browser to the foreground so the user can manually click the extension icon. | should |
| C6 | The notification must not appear if the user explicitly clicked "Stop Tracking" before closing the tab. It should only trigger on unexpected tab closure (tracked tab removed while tracking was active). | must |
| C7 | If the user closes multiple tabs rapidly (including the tracked tab), only one notification should be shown. Duplicate notifications must be suppressed. | should |
| C8 | The notification icon should use the Kaboom extension icon for brand consistency. | could |

## Non-Goals

- This feature does NOT change any MCP tools, modes, or server-side behavior. It is purely a Chrome extension UX change.
- This feature does NOT automatically re-attach tracking to another tab when the tracked tab closes. It only *suggests* that the user do so. Automatic re-attachment would violate the explicit opt-in principle of single-tab tracking.
- This feature does NOT add per-tab badge indicators. Chrome's MV3 extension API only supports a single global badge. The badge reflects the overall Kaboom state, not per-tab state.
- This feature does NOT introduce any new popup pages, options pages, or separate UI surfaces. All UI changes are within the existing popup and the extension badge.
- This feature does NOT modify the "Track This Tab" button behavior for first-time tracking (no tab currently tracked). That flow remains a single click with no dialog.
- Out of scope: changing the tracked tab via keyboard shortcut or context menu. Those are separate feature proposals.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Badge update latency | < 200ms from state change to badge render |
| Confirmation dialog render | < 50ms from click to dialog visible |
| Tab close notification dispatch | < 500ms from tab close event to notification shown |
| Memory overhead | < 50KB additional (dialog HTML/CSS, notification logic) |
| Main thread blocking | Zero. All operations must be async or non-blocking. |

## Security Considerations

- **No new data captured.** These UX improvements do not capture, store, or transmit any additional data. They only surface existing state (tracked tab ID, URL) that is already in `chrome.storage.local`.
- **No new permissions required.** Badge text and background color use `chrome.action` (already declared). The `notifications` permission must be added to `manifest.json` for sub-feature C. This permission is low-risk: it only allows showing OS-level notifications, not accessing any user data.
- **Tab URL display.** The confirmation dialog (B2) and tab close notification (C3) display the tracked tab URL. This URL is already visible in `chrome.storage.local` and in the existing popup UI. No new exposure.
- **No cross-origin concerns.** All operations are within the extension context. No web-accessible resources are modified.
- **No attack surface change.** No new message types, no new external communication, no new content script injection.

## Edge Cases

- **What happens when the server disconnects while tracking is active?** The badge shows the server disconnect state ("!" red), which takes priority over tracking state. When the server reconnects, the badge returns to the tracking-active state. Tracking itself is not affected by server connectivity.
- **What happens when the user opens the popup on a tab that is already tracked?** The popup shows "Stop Tracking" as it does today. No confirmation dialog is needed because there is no switch.
- **What happens when the tracked tab navigates to a new URL?** No change in behavior. Tracking follows the tab ID, not the URL. The badge continues to show tracking-active state. The stored `trackedTabUrl` updates via existing logic.
- **What happens when the tracked tab crashes?** Chrome fires `onRemoved` for crashed tabs. The tab close notification (C) fires as normal.
- **What happens when the browser restarts?** The existing `onStartup` listener clears `trackedTabId` and `trackedTabUrl`. The badge resets to the "no tracking" state. No tab close notification fires because the tab removal did not happen during the session; the startup listener handles a stale state rather than an active close.
- **What happens when the extension is updated/reloaded while tracking?** The service worker restarts. `chrome.storage.local` persists across reloads. The badge should re-read storage on startup and reflect the correct state. If the tracked tab still exists, tracking continues. If the tracked tab no longer exists (stale ID), the next storage access or health check should clear it.
- **What happens when the user clicks "Track This Tab" on the same tab that is already tracked?** The popup shows "Stop Tracking", not "Track This Tab", so this click path is not reachable.
- **What happens when `chrome.notifications` is not available?** Some Chromium-based browsers may not support `chrome.notifications`. The extension should gracefully degrade: log the notification to the console and rely on the badge state change (grey dash) to signal tracking loss.
- **What happens when multiple tabs close simultaneously including the tracked tab?** The `onRemoved` listener fires once per tab. Only the event for the tracked tab's ID triggers the notification. Requirement C7 ensures at most one notification even if there is a race.
- **What happens when the popup is already open when the tracked tab closes?** The popup should update its UI to reflect that tracking has stopped (show "Track This Tab" state). The notification still fires because the popup may be on a different screen or partially hidden.

## Dependencies

- **Depends on:** Single-tab tracking isolation (shipped v5.1.0), which provides `trackedTabId`/`trackedTabUrl` in `chrome.storage.local` and the `onRemoved` cleanup logic.
- **Depends on:** `chrome.action` API (MV3, already in use for badge).
- **Depends on:** `chrome.notifications` API (new dependency for sub-feature C, requires adding permission to `manifest.json`).
- **Depended on by:** No other features depend on these UX improvements.

## Assumptions

- A1: The extension is installed and active in Chrome. These features do not apply to Firefox or other browsers.
- A2: Chrome MV3 `chrome.action.setBadgeText` and `chrome.action.setBadgeBackgroundColor` behave synchronously in terms of rendering (no callback needed to confirm visual update).
- A3: `chrome.storage.onChanged` fires reliably within the same event loop tick as `chrome.storage.local.set`/`remove` calls.
- A4: The popup is implemented as a single `popup.html` page; the confirmation dialog can be an inline DOM element toggled via display property.
- A5: `chrome.notifications.create` is available in the target Chrome version (Chrome 28+, MV3).
- A6: `chrome.action.openPopup()` availability is Chrome 99+ with appropriate permissions. Graceful degradation is acceptable for older versions.
- A7: The `onRemoved` event fires before `chrome.storage.local.remove` completes in the existing tracked-tab-close handler, so the notification logic can intercept before state is cleared.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should the badge show different text/color for "tracking active on THIS tab" vs "tracking active on ANOTHER tab"? | open | Chrome badge API is global (not per-tab). Per-tab badge text is supported via `chrome.action.setBadgeText({tabId})` but adds complexity. Could show a checkmark on the tracked tab and the dash on others. |
| OI-2 | Should the confirmation dialog remember a "don't ask again" preference? | open | Adding a checkbox could reduce friction for power users but risks re-enabling accidental switches. If added, it should be resettable from the popup settings. |
| OI-3 | Should the tab close notification include a list of candidate tabs to track next? | open | Listing open tabs in the notification body could help the user pick quickly, but notification body text is limited and listing URLs may be a privacy concern in shared-screen scenarios. |
| OI-4 | What is the exact `notifications` permission string needed in manifest.json? | open | Likely just `"notifications"` in the `permissions` array. Needs verification against MV3 docs. |
| OI-5 | Should the badge tooltip update dynamically with the tracked tab title as the page navigates? | open | Useful for long sessions but requires listening to `chrome.tabs.onUpdated` for title changes, adding a listener that fires frequently. |

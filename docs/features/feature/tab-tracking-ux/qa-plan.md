---
status: proposed
scope: feature/tab-tracking-ux/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-tab-tracking-ux
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Tab Tracking UX Improvements

> QA plan for the Tab Tracking UX Improvements feature (Badge Indicator, Switch Confirmation, Tab Close Recovery). Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Tracked tab URL displayed in confirmation dialog | The switch confirmation dialog shows the URL of the currently tracked tab and the new tab. URLs may contain session tokens, PII in paths, or sensitive query parameters. Verify only necessary URL info is shown. | high |
| DL-2 | Tracked tab URL in notification | The tab close notification shows the URL/title of the closed tab. This may reveal browsing history to anyone who can see OS notifications (shared screen, screen recording). | high |
| DL-3 | Browsing history exposure via badge tooltip | Badge tooltip shows "Kaboom: tracking tab 42" or tab URL. If the tooltip shows a URL, it could expose browsing info to shoulder surfers. | medium |
| DL-4 | Tab data in chrome.storage.local | `trackedTabId` and `trackedTabUrl` are already stored in `chrome.storage.local`. No NEW data is stored by this feature. Verify no additional sensitive data is persisted. | medium |
| DL-5 | Notification content logged by OS | Chrome notifications may be logged by the OS notification center (macOS Notification Center, Windows Action Center). The tab URL/title in the notification persists in the OS log. | medium |
| DL-6 | Data transmission path | This feature is entirely extension-side. No server communication is involved. Verify no data is sent to the Go server for any of the three sub-features. | critical |
| DL-7 | Tab list exposed in recovery notification | If OI-3 (listing candidate tabs) is implemented, the notification would show URLs of other open tabs, potentially exposing browsing history. Verify this is NOT implemented in v1. | high |

### Negative Tests (must NOT leak)
- [ ] No NEW data is written to `chrome.storage.local` beyond what already exists (`trackedTabId`, `trackedTabUrl`)
- [ ] No data is sent to the Go server (127.0.0.1:7890) by any of the three sub-features
- [ ] No data is sent to any external server
- [ ] The tab close notification does NOT list other open tabs' URLs
- [ ] The confirmation dialog only shows URLs already visible to the user (current and tracked tab)
- [ ] Badge tooltip does NOT contain full URLs with query parameters (tab ID or title is sufficient)

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | No MCP interface | This feature has NO MCP interface. LLM should not attempt to call any new tools for badge/dialog/notification control. Verify existing `observe({what: "tabs"})` still works and returns tracking state. | [ ] |
| CL-2 | Tracking state in observe response | `observe({what: "tabs"})` already returns which tab is tracked. LLM should use this to verify tracking state, not rely on badge or notification. | [ ] |
| CL-3 | Tab close detection by AI | If the tracked tab closes, the AI will notice via timeout on subsequent tool calls or via `observe({what: "tabs"})` showing no tracked tab. The notification is for the human, not the AI. | [ ] |
| CL-4 | No behavioral change to existing MCP tools | All existing MCP tools work identically. The only change is in the extension UI (badge, popup dialog, notifications). | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM tries to call a new MCP tool to check badge state or trigger notifications -- verify no new tools exist
- [ ] LLM assumes tab close notification means it needs to take action -- the notification is for the human; the AI detects tracking loss via existing mechanisms
- [ ] LLM confuses badge state with server connection state -- both use the badge, but server disconnect takes priority

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (all UX changes are passive/reactive)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Check tracking state (human) | 0 steps: glance at badge | Already minimal -- this feature adds the capability |
| Switch tracked tab (human) | 2 steps: (1) click Track This Tab, (2) confirm dialog | Was 1 step (no confirmation). Extra step prevents accidents. |
| Recover from tab close (human) | 2 steps: (1) see notification, (2) open popup and track new tab | Was manual discovery (unknown steps). Notification guides recovery. |
| AI checks tracking state | 1 step: `observe({what: "tabs"})` | No change from before this feature |

### Default Behavior Verification
- [ ] Badge shows correct state immediately after extension install (no tab tracked = grey dash)
- [ ] Badge updates within 200ms of any state change
- [ ] Confirmation dialog only appears when switching (not when first tracking or stopping)
- [ ] Notification only appears on unexpected tab close (not on explicit "Stop Tracking")
- [ ] No new configuration needed -- all three sub-features are enabled by default

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| **Sub-feature A: Badge** | | | | |
| UT-1 | Badge shows grey dash when no tab tracked | `trackedTabId` cleared from storage | Badge text: "-", background: #6e7681 | must |
| UT-2 | Badge shows green when tracking and connected | `trackedTabId` set, server connected | Badge text: "" or green dot, background: #3fb950 | must |
| UT-3 | Badge shows red "!" when server disconnected | Server connection lost | Badge text: "!", background: #f85149 | must |
| UT-4 | Server disconnect overrides tracking state | Tracking active + server disconnected | Badge shows red "!", not green | must |
| UT-5 | Badge updates within 200ms | Change tracking state, measure badge update | < 200ms from state change to visual update | must |
| UT-6 | Badge tooltip for tracking state | Tracking tab 42 | Tooltip: "Kaboom: tracking tab 42" | should |
| UT-7 | Badge tooltip for no tracking | No tab tracked | Tooltip: "Kaboom: not tracking" | should |
| UT-8 | Badge tooltip for disconnected | Server disconnected | Tooltip: "Kaboom: server disconnected" | should |
| UT-9 | No flicker on rapid state changes | Multiple state changes within 50ms | Final state is correct, no visible flicker | should |
| UT-10 | Badge reads storage on extension startup | Extension reloaded with existing tracking state | Badge reflects stored state immediately | should |
| **Sub-feature B: Confirmation Dialog** | | | | |
| UT-11 | Dialog appears when switching tabs | Click "Track This Tab" while tab A is tracked, on tab B | Confirmation dialog shown | must |
| UT-12 | Dialog shows current and new tab info | Switching from tab A (url A) to tab B (url B) | Dialog text mentions both URLs/titles | must |
| UT-13 | Confirm switch changes tracked tab | Click "Switch Tracking" in dialog | `trackedTabId` updated to new tab | must |
| UT-14 | Cancel preserves current tracking | Click "Cancel" in dialog | `trackedTabId` unchanged, popup unchanged | must |
| UT-15 | No dialog when starting from no-track state | Click "Track This Tab" with no tab tracked | Tracking starts immediately, no dialog | must |
| UT-16 | No dialog when stopping tracking | Click "Stop Tracking" | Tracking stops immediately, no dialog | must |
| UT-17 | Dialog is inline HTML (not window.confirm) | Inspect dialog implementation | Inline `<div>` with `role="dialog"` in popup.html | should |
| UT-18 | Dialog keyboard: Enter confirms | Press Enter while dialog is open | Switch occurs | should |
| UT-19 | Dialog keyboard: Escape cancels | Press Escape while dialog is open | Dialog closes, no switch | should |
| UT-20 | Dialog focus management | Dialog opens | Focus moves to dialog. On close, focus returns to trigger button. | should |
| UT-21 | Dialog has accessible label | Screen reader reads dialog | `aria-label` or `aria-labelledby` present, `role="dialog"` set | should |
| **Sub-feature C: Tab Close Recovery** | | | | |
| UT-22 | Notification on tracked tab close | Close the tracked tab | Chrome notification appears with tab URL/title | must |
| UT-23 | Notification message content | Tracked tab "localhost:3000" closed | Message mentions the closed tab and suggests opening popup | must |
| UT-24 | No notification on explicit stop tracking | Click "Stop Tracking", then close tab | No notification | must |
| UT-25 | No notification on unrelated tab close | Close a non-tracked tab | No notification | must |
| UT-26 | Notification auto-dismisses | Wait after notification | Notification dismissed after ~8 seconds | should |
| UT-27 | Notification click opens popup | Click notification | Popup opens (if `chrome.action.openPopup()` available) or browser focused | should |
| UT-28 | Only one notification for multiple rapid closes | Close tracked tab + 2 other tabs rapidly | Only one notification shown | should |
| UT-29 | Graceful degradation without `chrome.notifications` | `chrome.notifications` unavailable | Console log fallback, no crash | should |
| UT-30 | Notification uses Kaboom icon | Check notification properties | `iconUrl` set to Kaboom extension icon | could |
| UT-31 | Badge updates to grey dash on tab close | Tracked tab closes | Badge shows "-" with grey background | must |
| UT-32 | Popup updates on tab close (if open) | Popup is open when tracked tab closes | Popup shows "Track This Tab" state | should |
| UT-33 | Stale tab ID on browser restart | Browser restarts, old `trackedTabId` in storage | `onStartup` clears it, badge shows no-tracking state, no notification | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Badge + tracking lifecycle | background.js + chrome.action API | Badge transitions: grey -> green (track) -> grey (untrack) | must |
| IT-2 | Badge + server disconnect | background.js + server connection status | Badge transitions: green -> red (disconnect) -> green (reconnect) | must |
| IT-3 | Confirmation dialog + popup UI | popup.js + popup.html | Dialog appears inline, styled with dark theme, interaction works | must |
| IT-4 | Tab close notification + storage cleanup | background.js + chrome.notifications + chrome.storage | Tab close triggers notification AND clears storage AND updates badge | must |
| IT-5 | Full tracking switch flow | popup.js + background.js + chrome.storage + chrome.action | Open popup on tab B while tracking tab A -> dialog -> confirm -> tab B now tracked, badge green | must |
| IT-6 | Tab close + popup recovery | background.js + popup.js | Close tracked tab -> notification shows -> user opens popup -> sees "Track This Tab" | should |
| IT-7 | No interference with MCP tools | Extension UX + server MCP calls | All MCP tools continue working during badge updates, dialog interactions, and notification display | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Badge update latency | Time from state change to badge render | < 200ms | must |
| PT-2 | Confirmation dialog render | Time from click to dialog visible | < 50ms | must |
| PT-3 | Tab close notification dispatch | Time from onRemoved event to notification | < 500ms | must |
| PT-4 | Memory overhead | Additional memory from dialog HTML/CSS + notification logic | < 50KB | should |
| PT-5 | Main thread blocking | All operations async or non-blocking | Zero blocking | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Server disconnects while tracking | Tracking active, server goes down | Badge changes to red "!". When server reconnects, badge returns to green. | must |
| EC-2 | Tracked tab navigates to new URL | Tab navigates from page A to page B | Badge stays green. Stored URL updates. No notification. | must |
| EC-3 | Tracked tab crashes | Chrome reports tab as crashed | `onRemoved` fires. Notification shown. Badge goes grey. | should |
| EC-4 | Browser restart with stale tracking | Restart Chrome, old `trackedTabId` in storage | `onStartup` clears it. Badge shows grey dash. No notification. | should |
| EC-5 | Extension updated/reloaded while tracking | Reload extension | Service worker restarts. If tracked tab still exists, tracking continues. Badge re-reads storage. | should |
| EC-6 | Popup open on tracked tab | Open popup on the currently tracked tab | Shows "Stop Tracking", NOT "Track This Tab". No switch dialog. | must |
| EC-7 | Multiple tabs close including tracked | Close 5 tabs at once, one is tracked | One notification for tracked tab closure. No duplicate notifications. | should |
| EC-8 | `chrome.action.openPopup()` not available | Click notification on Chrome < 99 | Browser focused instead of popup opening. No crash. | should |
| EC-9 | `chrome.notifications` not available | Chromium fork without notifications API | Console log fallback. Badge still updates. No crash. | should |
| EC-10 | Rapid badge state changes | Toggle tracking on/off 10 times in 1 second | Final badge state is correct. No visual artifacts. | should |
| EC-11 | Confirmation dialog open, tracked tab closes | Tab A tracked, dialog open to switch to B, tab A closes | Dialog should close or update. Badge goes grey. Notification shows. | should |
| EC-12 | Tracking stopped, then tab closes | Stop tracking, then close the formerly tracked tab | No notification (tracking was explicitly stopped). | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] At least 3 browser tabs open with different pages
- [ ] No tab currently tracked (fresh state)
- [ ] `notifications` permission added to manifest.json

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| **Sub-feature A: Badge Indicator** | | | | |
| UAT-1 | N/A (human observes badge) | Look at extension badge with no tab tracked | Grey "-" badge with grey background (#6e7681) | [ ] |
| UAT-2 | N/A (human clicks "Track This Tab" on tab A) | Look at badge after tracking | Green badge (green background #3fb950) | [ ] |
| UAT-3 | N/A (human hovers over extension icon) | Read tooltip | "Kaboom: tracking tab [ID]" or similar descriptive text | [ ] |
| UAT-4 | N/A (stop Kaboom server) | Look at badge after server disconnect | Red "!" badge with red background (#f85149) | [ ] |
| UAT-5 | N/A (restart Kaboom server) | Look at badge after server reconnect | Green badge restored (tracking still active) | [ ] |
| UAT-6 | N/A (click "Stop Tracking") | Look at badge | Grey "-" badge (no tracking) | [ ] |
| **Sub-feature B: Switch Confirmation** | | | | |
| UAT-7 | N/A (human tracks tab A, navigates to tab B, opens popup, clicks "Track This Tab") | Popup UI | Confirmation dialog appears with info about tab A and tab B | [ ] |
| UAT-8 | N/A (human clicks "Cancel" in dialog) | Popup and badge | Dialog closes. Tab A still tracked. Badge stays green. | [ ] |
| UAT-9 | N/A (human clicks "Track This Tab" on tab B again, clicks "Switch Tracking") | Popup and badge | Tab B is now tracked. Badge stays green. | [ ] |
| UAT-10 | N/A (human stops tracking, goes to tab C, clicks "Track This Tab") | Popup | NO confirmation dialog. Tracking starts immediately on tab C. | [ ] |
| UAT-11 | N/A (human presses Escape while dialog is open) | Popup | Dialog closes, no switch | [ ] |
| UAT-12 | N/A (human presses Enter while dialog is open) | Popup | Switch confirmed | [ ] |
| **Sub-feature C: Tab Close Recovery** | | | | |
| UAT-13 | N/A (human tracks tab C, then closes tab C) | OS notification area | Chrome notification: "Tracked tab closed" with URL/title and suggestion to re-track | [ ] |
| UAT-14 | N/A (human observes badge after tab close) | Extension badge | Grey "-" badge (no tracking) | [ ] |
| UAT-15 | N/A (human clicks notification) | Browser behavior | Popup opens or browser comes to foreground | [ ] |
| UAT-16 | N/A (human tracks tab A, clicks "Stop Tracking", then closes tab A) | OS notification area | NO notification appears (tracking was explicitly stopped) | [ ] |
| UAT-17 | N/A (human tracks tab A, closes tabs B and C rapidly) | OS notification area | NO notification (tracked tab A is still open) | [ ] |
| **Cross-feature verification** | | | | |
| UAT-18 | `{"tool": "observe", "arguments": {"what": "tabs"}}` | After tracking tab | Response shows correct tracked tab info | [ ] |
| UAT-19 | `{"tool": "observe", "arguments": {"what": "tabs"}}` | After tracked tab closed | Response shows no tracked tab | [ ] |
| UAT-20 | `{"tool": "observe", "arguments": {"what": "logs"}}` | During all tests | Existing MCP tools continue working normally | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No new server communication | Monitor network during badge/dialog/notification interactions | No requests to 127.0.0.1:7890 from these features (server may receive existing polling) | [ ] |
| DL-UAT-2 | No external server communication | Monitor all network traffic | No requests to external servers | [ ] |
| DL-UAT-3 | No new data in chrome.storage.local | Inspect storage before and after using all 3 sub-features | Only existing `trackedTabId` and `trackedTabUrl` keys. No new keys added. | [ ] |
| DL-UAT-4 | Notification content appropriate | View notification text | URL/title shown, but no full URL with query parameters containing tokens | [ ] |
| DL-UAT-5 | Confirmation dialog content appropriate | View dialog text | URLs/titles shown, but no sensitive query parameters | [ ] |

### Regression Checks
- [ ] All existing MCP tools (`observe`, `configure`, `interact`, `generate`) still work after the UX changes
- [ ] Extension popup functionality unchanged (Track/Stop buttons work as before, with new confirmation dialog for switch only)
- [ ] Server connection status still displayed correctly alongside new badge states
- [ ] Error count overlay on badge still works when errors occur during tracking
- [ ] Extension performance not degraded by badge update listeners or notification logic

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |

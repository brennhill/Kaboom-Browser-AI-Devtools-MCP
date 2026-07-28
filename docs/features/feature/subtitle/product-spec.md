---
status: proposed
scope: feature/subtitle
ai-priority: high
tags: [interact, overlay, narration, demo, walkthrough]
doc_type: product-spec
feature_id: feature-subtitle
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Subtitle

## Problem

AI agents can control the browser and observe its state, but they can't communicate visually with the person watching. During walkthroughs, demos, or recorded sessions, the AI's reasoning is invisible — it happens in the chat window, not on screen.

The existing action toast shows brief feedback ("clicking .submit-btn... done") but it's transient, positioned at the top, and not designed for narration.

## What It Does

Displays persistent, styled text at the bottom of the viewport — like closed captions or subtitles. The AI uses it to narrate what it's doing, explain what the user is seeing, or annotate a demo recording.

## API

### Optional parameter on any `interact` action (primary usage)

`subtitle` is an optional string parameter on **every** `interact` action. One tool call performs the action AND shows the caption:

```json
interact({ what: "navigate", url: "/dashboard", subtitle: "This is the main dashboard." })
interact({ what: "highlight", selector: ".error-banner", subtitle: "This API call is returning a 500." })
interact({ what: "refresh", subtitle: "Let's check if the fix worked..." })
```

No extra tool calls. The subtitle displays alongside the action.

### Standalone action (for text-only updates)

For cases where you just want to update or clear the caption without performing another action:

```json
interact({ what: "subtitle", text: "Now watch what happens when we submit the form..." })
interact({ what: "subtitle", text: "" })  // Clear subtitle
```

### Parameters

| Parameter | Context | Type | Default | Description |
|-----------|---------|------|---------|-------------|
| `subtitle` | Any interact action | string | omitted | Optional caption text. Omit to leave current subtitle unchanged. |
| `text` | `action: "subtitle"` | string | required | Text to display. Empty string clears. |
| `duration_ms` | Both | number | 0 (persistent) | Auto-dismiss after N ms. 0 = stays until replaced or cleared. |

## Visual Design

- Full-width dark bar at bottom of viewport
- White text, ~16px, system font
- Semi-transparent black background (rgba(0,0,0,0.85))
- Padding: 12px 24px
- z-index: 2147483646 (below toast, above everything else)
- Fade-in/fade-out transition (200ms)
- Max 3 lines, overflow ellipsis

## Use Cases

### Site Walkthrough (single tool call per step)
```json
interact({ what: "navigate", url: "/dashboard", subtitle: "Main dashboard — notice the metrics cards at the top." })
interact({ what: "navigate", url: "/settings", subtitle: "Settings page — let's check the API configuration." })
interact({ what: "navigate", url: "/users", subtitle: "User management — 3 pending invitations." })
```

3 navigations + 3 subtitles = 3 tool calls (not 6).

### Demo Video Recording
User records their screen while AI drives the browser with narration:
```json
interact({ what: "subtitle", text: "Kaboom captures every browser event in real time" })
interact({ what: "navigate", url: "https://example.com", subtitle: "Watch — we'll trigger an error and see it immediately" })
interact({ what: "execute_js", script: "fetch('/api/broken')", subtitle: "Calling a broken API endpoint..." })
```

### Guided Debugging
AI walks the user through a problem it found:
```json
interact({ what: "highlight", selector: ".error-banner", subtitle: "See this? The API response was malformed." })
interact({ what: "highlight", selector: "#network-tab", subtitle: "The 500 started after the last deploy." })
```

### UAT Narration
Combined with human-narrated UAT — the AI adds its own captions as the user walks through a flow, confirming what it's capturing.

## Implementation

### Extension side (content script)
- Reuses the existing toast infrastructure pattern from `runtime-message-listener.ts`
- New message type: `KABOOM_SUBTITLE`
- Creates/updates a persistent overlay div (`#kaboom-subtitle`)
- Positioned at bottom of viewport with `position: fixed`
- Same injection pattern as `showActionToast()` but different styling and persistence

### Server side (Go)
- Extract `subtitle` param in the common interact handler (before the action switch)
- If present, send `KABOOM_SUBTITLE` message to extension alongside the action
- New case `"subtitle"` in switch for standalone text-only updates
- Empty `text` / empty `subtitle` = clear

### No new tools
- Extends `interact` tool — adds optional `subtitle` string to schema
- Adds `"subtitle"` to valid actions list for standalone usage
- Same async command pattern as `highlight`

## Scope

- Extension + Go changes
- ~100 lines content script (overlay rendering)
- ~40 lines Go (handler + common param extraction)
- Schema update: add optional `subtitle` string to interact params

## Difference from Toast

| | Toast | Subtitle |
|---|-------|---------|
| Position | Top-right | Bottom (full-width) |
| Duration | 3s default, auto-dismiss | Persistent until replaced/cleared |
| Purpose | Action feedback | Narration/communication |
| Trigger | Automatic (on AI actions) | Explicit (AI chooses when) |
| Style | Compact pill | Caption bar |
| Content | Action + status | Free-form text |
| Composable | No | Yes — optional param on any interact action |

## Extension Toggles

Two independent toggles in the extension popup, under the existing Advanced Capture section:

| Toggle        | Storage Key            | Default | Scope                                      |
| ------------- | ---------------------- | ------- | ------------------------------------------ |
| Action Toasts | `actionToastsEnabled`  | `true`  | Controls `KABOOM_ACTION_TOAST` rendering |
| Subtitles     | `subtitlesEnabled`     | `true`  | Controls `KABOOM_SUBTITLE` rendering     |

### Behavior when off

- **Server side**: No change. The daemon always sends toast/subtitle messages to the extension. No server-side awareness of toggle state.
- **Extension side**: The content script checks the stored setting before rendering. If disabled, the message is silently ignored — no DOM element created, no visual output.
- **No error returned**: The MCP tool call still succeeds. The AI doesn't know (or need to know) whether the overlay was displayed.

### Implementation (follows existing toggle pattern)

1. **popup.html**: Add two checkbox toggles with `toggle-switch` class, IDs `toggle-action-toasts` and `toggle-subtitles`
2. **feature-toggles.ts**: Add entries to `FEATURE_TOGGLES` array:

   ```typescript
   { id: 'toggle-action-toasts', storageKey: 'actionToastsEnabled', messageType: 'setActionToastsEnabled', default: true }
   { id: 'toggle-subtitles', storageKey: 'subtitlesEnabled', messageType: 'setSubtitlesEnabled', default: true }
   ```

3. **background message-handlers.ts**: Handle `setActionToastsEnabled` and `setSubtitlesEnabled` via existing `handleForwardedSetting` pattern — writes to storage, forwards to content scripts
4. **content script runtime-message-listener.ts**: Guard the existing `showActionToast()` and new `showSubtitle()` with a storage check:

   ```typescript
   case 'KABOOM_ACTION_TOAST':
     if (!actionToastsEnabled) return  // silently ignore
     showActionToast(message)
     break
   case 'KABOOM_SUBTITLE':
     if (!subtitlesEnabled) return  // silently ignore
     showSubtitle(message)
     break
   ```

5. **Content script init**: Read both settings from `chrome.storage.local` on load, listen for forwarded messages to update cached state

### Why no server-side gating

- Toggles are a user preference, not a protocol concern
- The daemon has no reason to know — it sends the message, extension decides
- Keeps the MCP response clean (no "subtitle was suppressed" noise)
- If the user re-enables mid-session, the next message just works

## Success Criteria

1. AI can display subtitle text alongside any interact action in a single tool call
2. Text persists until explicitly replaced or cleared
3. Visually distinct from action toast
4. Works on any page (no CSP issues — uses same injection as toast)
5. Readable over any background
6. Smooth transitions between subtitle changes
7. Standalone `action: "subtitle"` works for text-only updates
8. Toast and subtitle can be independently disabled via extension popup toggles
9. When disabled, messages are silently ignored — no errors, no DOM elements

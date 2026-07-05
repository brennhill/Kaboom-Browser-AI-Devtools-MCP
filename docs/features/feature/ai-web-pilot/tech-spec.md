---
status: shipped
scope: feature/ai-web-pilot/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-ai-web-pilot
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-ai-web-pilot.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Ai Web Pilot Review](ai-web-pilot-review.md).

# Technical Spec: AI Web Pilot

## Purpose

Kaboom is capture-only by design — it observes browser state but doesn't act on it. AI Web Pilot breaks that rule intentionally to enable two workflows:

1. **Human verification** — AI points at elements on screen ("this button") so developer can confirm understanding
2. **Faster reproduction** — AI saves/restores browser state and tests expressions without clicking through flows

These features let the AI debug frontend code autonomously while keeping the human in control of when to enable this power.

---

## Safety Model

**Human opt-in required.** All AI Web Pilot features are disabled by default and require explicit toggle in the extension popup. The AI cannot enable these programmatically.

When disabled, MCP tools return structured errors:
```json
{
  "error": "ai_web_pilot_disabled",
  "message": "Enable 'AI Web Pilot' in extension popup to use this tool"
}
```

This prevents runaway agents from self-authorizing code execution or state manipulation.

---

## Features

### 1. Highlight Element (`highlight_element`)

**Purpose:** AI points at DOM elements so developer can visually confirm "this is the button I'm talking about."

#### How It Works:
1. MCP tool receives CSS selector + optional duration (default 5000ms)
2. Server forwards to extension via existing WebSocket channel
3. Extension injects `#kaboom-highlighter` div with:
   - `position: fixed`
   - `border: 4px solid red`
   - `z-index: 2147483647` (max)
   - `pointer-events: none`
4. Div positioned via `element.getBoundingClientRect()`
5. Auto-removed after duration or on next highlight call

#### Tool Schema:
```
highlight_element
  selector: string (required) — CSS selector for target element
  duration_ms: integer (default 5000) — How long to show highlight
```

**Returns:** `{ success: true, selector: "...", bounds: { x, y, width, height } }` or error if element not found.

---

### 2. Browser State Snapshots (`manage_state`)

**Purpose:** Save and restore `localStorage`, `sessionStorage`, and cookies to skip repetitive click-through flows.

#### How It Works:
1. **Save:** Extension serializes all three storage types + current URL into a snapshot object
2. **Load:** Extension clears existing state, restores from snapshot, optionally navigates to saved URL
3. **List:** Returns available snapshot names with metadata (URL, timestamp, size)

Snapshots stored in extension's `chrome.storage.local` under `kaboom_state_snapshots` namespace.

#### Tool Schema:
```
manage_state
  action: "save" | "load" | "list" | "delete" (required)
  snapshot_name: string (required for save/load/delete)
  include_url: boolean (default true) — Navigate to saved URL on load
```

#### Returns:
- Save: `{ success: true, snapshot_name: "...", size_bytes: 1234 }`
- Load: `{ success: true, snapshot_name: "...", restored: { localStorage: 5, sessionStorage: 2, cookies: 3 } }`
- List: `{ snapshots: [{ name, url, timestamp, size_bytes }] }`

---

### 3. Execute JavaScript (`execute_javascript`)

**Purpose:** Run arbitrary JS in browser context to inspect runtime state (Redux stores, globals, framework internals).

#### How It Works:
1. MCP tool receives JS code string
2. Server forwards to extension
3. Extension executes via `new Function()` in page context (not extension context)
4. Result JSON-serialized and returned
5. Execution timeout: 5000ms (prevents infinite loops)

#### Security:
- Localhost-only (Kaboom already binds to 127.0.0.1)
- Human opt-in required (part of AI Web Pilot toggle)
- No persistent side effects guaranteed (user's responsibility)

#### Tool Schema:
```
execute_javascript
  script: string (required) — JS code to execute, must return JSON-serializable value
  timeout_ms: integer (default 5000) — Execution timeout
```

**Returns:** `{ success: true, result: <any> }` or `{ success: false, error: "...", stack: "..." }`

#### Example Uses:
- `window.__REDUX_DEVTOOLS_EXTENSION__ && store.getState()`
- `window.__NEXT_DATA__`
- `document.querySelector('#app').__vue__.$data`
- `localStorage.getItem('auth_token') !== null`

---

## Extension Implementation

### New Message Types

```javascript
// From server to extension (via background.js WebSocket)
{ type: 'KABOOM_HIGHLIGHT', payload: { selector, duration_ms } }
{ type: 'KABOOM_MANAGE_STATE', payload: { action, snapshot_name, include_url } }
{ type: 'KABOOM_EXECUTE_JS', payload: { script, timeout_ms, request_id } }

// From extension to server (responses)
{ type: 'highlight_result', payload: { success, selector, bounds } }
{ type: 'state_result', payload: { success, ... } }
{ type: 'execute_result', payload: { request_id, success, result | error } }
```

### Popup UI Addition

New toggle in popup.html:
```
[ ] AI Web Pilot (highlight, state, execute)
    ⚠️ Allows AI to interact with page
```

Toggle state stored in `chrome.storage.sync` as `aiWebPilotEnabled`.

### Track This Page (Debugging Feature)

**Purpose:** Focus extension debugging on a single tab to eliminate noise from multiple tabs polling simultaneously.

**UI:** Button in popup.html labeled "Track This Page" (blue) that toggles to "Stop Tracking" (red) when active.

#### How It Works:
1. User clicks "Track This Page" in extension popup
2. Extension stores current tab ID and URL in `chrome.storage.local`:
   ```javascript
   { trackedTabId: 123, trackedTabUrl: "http://localhost:3000" }
   ```
3. All pending query executions route to tracked tab instead of active tab
4. User can navigate to other tabs - queries still execute on tracked tab
5. Click "Stop Tracking" to clear and return to active-tab behavior
6. Auto-clears tracking if tracked tab is closed

#### Implementation:
```javascript
// In handlePendingQuery()
const storage = await chrome.storage.local.get(['trackedTabId'])
if (storage.trackedTabId) {
  // Use tracked tab for query execution
  const trackedTab = await chrome.tabs.get(storage.trackedTabId)
  tabId = storage.trackedTabId
} else {
  // Fallback to active tab
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
  tabId = tabs[0].id
}
```

**Polling Behavior:** Extension continues polling `/pending-queries` every 1 second regardless of which tab is active or tracked. Only query execution is routed to the tracked tab.

#### Use Cases:
- Debugging extension behavior on specific localhost development server
- Isolating AI Web Pilot commands to a single test page
- Troubleshooting state management for a particular application
- Reducing server log noise from multiple browser tabs

**Note:** This is a debugging-focused feature, not part of the core AI Web Pilot user experience.

---

## MCP Tool Registration

Tools registered conditionally based on extension state. When disabled, tools still appear in `tools/list` but return the opt-in error on invocation. This gives AI visibility that the capability exists.

Tools added to the `configure` composite tool:
```
configure action:"highlight" selector:"#submit-btn" duration_ms:3000
configure action:"save_state" snapshot_name:"cart_full"
configure action:"load_state" snapshot_name:"cart_full"
configure action:"execute" script:"window.__NEXT_DATA__"
```

Or as standalone tools if the composite pattern doesn't fit the use case.

---

## Test Scenarios

1. **Opt-in enforcement** — Call any AI Web Pilot tool with toggle disabled → error response
2. **Highlight lifecycle** — Highlight element → verify div injected → wait duration → verify removed
3. **State round-trip** — Save state → clear storage → load state → verify restored
4. **Execute success** — Run `1 + 1` → returns `{ result: 2 }`
5. **Execute timeout** — Run `while(true){}` → returns timeout error
6. **Execute error** — Run `throw new Error('test')` → returns error with stack

---

## Files Modified

| File | Change |
|------|--------|
| `extension/popup.html` | Add AI Web Pilot toggle |
| `extension/popup.js` | Handle toggle state |
| `extension/background.js` | Route new message types |
| `extension/inject.js` | Implement highlight, state, execute handlers |
| `cmd/browser-agent/tools_core.go` | Add MCP tool handlers |
| `cmd/browser-agent/pilot.go` (new) | AI Web Pilot domain logic |

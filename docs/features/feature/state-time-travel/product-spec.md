---
feature: State Time-Travel
status: proposed
tool: observe
mode: history
version: 0.7.12
doc_type: product-spec
feature_id: feature-state-time-travel
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: State Time-Travel

## Problem Statement

Debugging requires seeing causality: what changed, when, and why. Standard Chrome DevTools shows a snapshot of the present. If something changes rapidly or the page reloads, critical information vanishes.

### Three critical failure modes:

1. **Page Reloads Lose History** — Form submit crashes and reloads the page. Network logs and console errors are cleared (unless "Preserve Log" is manually enabled). AI sees the blank page but not what caused the crash.

2. **Transient UI Misses** — A loading spinner appears and disappears in 200ms. By the time the AI "thinks" (5-10 seconds), the spinner is gone and DevTools shows the idle state. AI never saw the spinner and can't diagnose the hang.

3. **No Causal Links** — AI sees an error message appear and a network 500 response. But it doesn't know which user action triggered them. Was it the button click? The form submit? A background request? Guessing wastes time.

Without a record of what happened over time, the AI is a detective arriving at a crime scene after cleanup—it sees the result, not the cause.

## Solution

**State Time-Travel** captures and replays a persistent, causal timeline of page events. AI agents see not just the current state, but the entire sequence of what led to it.

Features:

- **Persistent Buffer** — Last 60 seconds of events (logs, network, DOM changes, user actions) survive page reloads
- **Event Timeline** — Linked events show causality: "User clicked 'Save'" → "Network POST /api/save" → "Response 500" → "Error modal appeared"
- **Before/After Snapshots** — Automatic DOM and console snapshots before and after each action
- **Diff Reports** — AI-readable summaries: "Action: Click Save. Result: 3 new errors, 1 network timeout, DOM changed 12 nodes"
- **Transient Event Capture** — UI elements that appear and disappear are recorded with timestamps ("Spinner appeared 0:05, removed 0:07")

Result: AI agents can rewind and understand what happened, not guess at what might have happened.

## Requirements

### Core Requirements

- **Persistent Event Buffer** — Store events in a ring buffer that survives page reloads and navigation:
  - Capacity: 60 seconds of events (adjust based on typical event rate)
  - Survives: page reload, navigation, tab switch (store in sessionStorage with recovery on page load)
  - Includes: console logs, network requests, DOM mutations, user actions (clicks, input), custom events

- **Causal Event Linking** — Connect cause → effect:
  - User action (click) → preceding network request initiators
  - User action (click) → subsequent network requests
  - Network request → response
  - DOM mutation → triggering event
  - Error event → related logs and network failures

- **Automatic Snapshots** — Capture before/after state for each user action:
  - DOM state before action
  - Console output during action
  - Network requests triggered by action
  - DOM state after action (with diff)
  - Execution time for action

- **Diff Reports** — Generate AI-readable summaries for each action:
  - Format: "Action: [action]. Result: [N errors, M network calls, K DOM nodes changed]"
  - Include timing: how long between action and result
  - Example: "Action: Click 'Save'. Result: 1 error (501), DOM changed 3 nodes (Error div appeared)"

- **Transient Event Recording** — Capture ephemeral UI:
  - Format: "Spinner appeared at 0:05.123, removed at 0:07.456"
  - Include element name/role if available
  - Duration helps AI diagnose hangs ("UI frozen for 2 seconds")

### Non-Functional Requirements

- **Memory efficiency** — Ring buffer bounded to 60 seconds; old events auto-evicted
- **Performance overhead** — Event capture <1ms per event (async where possible)
- **Payload size** — Compressed event buffer <1MB (gzip compression for storage)
- **Browser compatibility** — Works in all modern browsers (Edge 79+, Chrome 79+, Firefox 78+)

## Out of Scope

- Exporting time-travel buffer as video/animated replay (that's visual debugging; out of scope)
- Debugging production errors at scale (single-session replay only)
- Machine learning to infer causality (simple event linking, human-readable rules only)
- Modifying events in the buffer (read-only replay)
- Rewinding page state (AI can't revert to past state; only observe what happened)

## Success Criteria

- AI agents can understand page crashes without loss of information across reloads
- Transient UI (spinners, toasts) is never missed; AI sees complete timeline
- Root cause of errors is immediately apparent to AI ("Click → POST failed → Error appeared")
- Context window is not bloated by raw logs; summaries are concise and semantic
- Engineers report: "AI debugged a crash I couldn't reproduce manually"

### Metrics

- **Causal chain completeness** — % of errors with a clear user action → network → response chain
- **Timeline accuracy** — Events appear in correct chronological order (no time skips)
- **Buffer recovery** — % of events recovered after page reload
- **Context efficiency** — Token cost of time-travel data (should be 2-3x less than raw logs)
- **AI success rate** — % of crashes/hangs diagnosed correctly on first try

## User Workflow

### For AI Agents

1. Agent observes page: `observe({what: 'history'})`
2. Kaboom returns event timeline (last 60 seconds):
   ```
   {
     "events": [
       {
         "timestamp": "0:00.000",
         "type": "page_load",
         "url": "https://example.com/form"
       },
       {
         "timestamp": "0:03.214",
         "type": "user_action",
         "action": "click",
         "element": "button.btn-save",
         "kaboom_id": "btn-save",
         "result_summary": "1 error (401), 1 network request (POST /api/save)"
       },
       {
         "timestamp": "0:03.450",
         "type": "network_response",
         "method": "POST",
         "url": "/api/save",
         "status": 401,
         "cause_action": "click on button.btn-save"
       },
       {
         "timestamp": "0:03.500",
         "type": "console_error",
         "message": "Unauthorized: missing auth token",
         "cause_action": "POST /api/save (401)"
       },
       {
         "timestamp": "0:03.600",
         "type": "dom_mutation",
         "element": "div.error-modal",
         "change": "appeared",
         "cause_action": "POST /api/save failed"
       }
     ]
   }
   ```
3. Agent reads causal chain: click → POST failure → error modal
4. Agent suggests: "Add authentication token to POST request before calling /api/save"

### For Engineers

1. Developer runs test that fails mysteriously
2. Opens Kaboom DevTools → "History" tab
3. See a complete timeline of what happened:
   ```
   [0:00] Form load
   [0:02] User enters email
   [0:04] Click Submit
   [0:05] Network POST /register timeout
   [0:08] Page reloaded
   [0:09] Form is back but empty
   ```
4. Immediately understands: network timeout caused reload
5. Can now add retry logic or better error handling

## Examples

### Example 1: Page Reload Crash (Solved)

**Scenario:** Form submit is broken; accidentally calls `form.submit()` instead of `event.preventDefault()` + AJAX.

#### Without Time-Travel:
```
AI clicks Save button.
Page reloads.
AI sees: blank form on new page.
AI says: "I clicked save, but nothing happened. Form is just empty now."
AI can't debug because error is gone.
```

#### With Time-Travel:
```
AI calls observe({what: 'history'})
Response includes:
  [0:05] User action: Click 'Save'
  [0:05] Network: POST /api/save initiated
  [0:05] Network: Request CANCELLED (page unload)
  [0:06] Page unload event
  [0:07] Page reload
  [0:08] New page load (form reset)

AI reads the chain:
  "User clicked Save → Network request started → Page unloaded (native form submit) → Page reloaded"

AI immediately suggests: "You have e.preventDefault() missing. The form is submitting the old way."
```

### Example 2: Transient Loading Spinner (Solved)

**Scenario:** User complains: "The page freezes when I load it." You can't reproduce it.

#### Without Time-Travel:
```
AI loads page.
By the time observe() is called, page is already loaded and spinner is gone.
AI says: "Page loads fine for me. No spinner visible."
AI can't help.
```

#### With Time-Travel:
```
AI calls observe({what: 'history'})
Response includes:
  [0:00] Page load start
  [0:00.500] Spinner (role="status") appeared
  [0:05.200] API request to /api/user_profile completes
  [0:05.300] Spinner removed
  [0:05.400] Page content rendered

AI calculates: "Spinner visible for 4.8 seconds. This is the 'freeze'."
AI suggests: "The /api/user_profile request is slow (5s). Add a loading state message or background fetch."
```

### Example 3: Causal Error Diagnosis (Solved)

**Scenario:** You have a race condition. Multiple errors appear but it's unclear which action caused them.

#### Without Time-Travel:
```
AI sees:
  - Error: "Cannot read property 'email' of undefined"
  - Network error: 500 on /api/user
  - Warning: "State update on unmounted component"

AI guesses: "Maybe the network request failed? Or maybe the component unmounted too early?"
AI wastes time trying both hypotheses.
```

#### With Time-Travel:
```
AI calls observe({what: 'history'})
Response includes causal chain:
  [0:02] User action: Click 'Load Profile'
  [0:02] Network: GET /api/user started
  [0:03] Network: GET /api/user response 500
  [0:03] Console error: "Cannot read property 'email' of undefined"
    (cause: parsing response from failed request)
  [0:04] Component unmount event
  [0:04] Console warning: "State update on unmounted component"
    (cause: error handler tried to update state)

AI immediately diagnoses: "Network request failed (500), error handler tried to parse response, component unmounted during error recovery."
AI suggests: "Add null check when accessing response.user.email, and cancel pending requests on component unmount."
```

## Notes

### Related specs:
- Visual-Semantic Bridge (v6.1) — Provides element identifiers for causal linking
- Prompt-Based Network Mocking (v6.2) — Works with Time-Travel to verify error handling

### Design decisions:
- **60-second buffer** — Typical debugging session; longer = more memory
- **Ring buffer in sessionStorage** — Survives page reload/navigation; cleared on tab close
- **Async event capture** — Non-blocking; events recorded asynchronously to avoid perf hit
- **Causal linking via timestamps + element IDs** — Deterministic, no ML needed

### Dependencies:
- SessionStorage API (all modern browsers)
- Page Visibility API (detect when tab switches)
- MutationObserver (track DOM changes)
- Performance.now() (high-resolution timestamps)

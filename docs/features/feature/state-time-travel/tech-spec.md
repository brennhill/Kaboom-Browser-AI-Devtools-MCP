---
feature: State Time-Travel
status: proposed
doc_type: tech-spec
feature_id: feature-state-time-travel
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: State Time-Travel

> Plain language only. Describes HOW the implementation works at a high level, without code.

## Architecture Overview

State Time-Travel captures and buffers page events (user actions, network, DOM, console) in a persistent ring buffer. When the AI calls `observe({what: 'history'})`, Kaboom returns a curated timeline with causal links.

### Three layers:

1. **Event Capture** — Asynchronously record user actions, network, DOM mutations, console events
2. **Causal Linking** — Group related events and link cause → effect
3. **Timeline Serialization** — Convert buffer to AI-readable format with summaries and diffs

## Key Components

### 1. Event Capture System
**Purpose:** Record all page events without blocking the main thread.

#### Sources:
- **User actions** — `click`, `input`, `change`, `submit` events on interactive elements
- **Network requests** — Intercept `fetch()` and XHR calls
- **Network responses** — Store response status, body (opt-in), timing
- **Console events** — `console.log()`, `console.error()`, `console.warn()` calls
- **DOM mutations** — Use MutationObserver to record element added/removed/changed
- **Page events** — `load`, `unload`, `beforeunload`, `visibilitychange`
- **Custom events** — `CustomEvent` dispatches (app-specific signals)
- **Errors** — `error` and `unhandledrejection` events

#### Implementation approach:
- Async queue per event source (separate queues for network, console, DOM to avoid bottlenecks)
- Event listeners registered at page load via content script
- Events added to queue with timestamp (performance.now())
- Batch worker processes queue every 50ms (async, non-blocking)

#### Data structure per event:
```
{
  timestamp: number (milliseconds since page load),
  type: string (user_action, network_request, network_response, console, dom_mutation, page_event),
  details: { ... event-specific fields ... },
  element?: { id, role, text, kaboom_id },
  cause?: string (pointer to causal parent event)
}
```

### 2. Ring Buffer Storage
**Purpose:** Bounded storage that survives page reload and navigation.

#### Storage mechanism:
- Primary: sessionStorage (survives reload/nav, lost on tab close)
- Fallback: in-memory ring buffer (if sessionStorage unavailable or quota exceeded)
- Format: JSON lines (one event per line, easy to parse and prune)

#### Ring buffer specifics:
- Fixed capacity: 60 seconds of events (adjust dynamically based on event rate)
- Eviction policy: FIFO (oldest events dropped first)
- Compression: Optional gzip compression for storage if events > 500KB

#### Example structure in sessionStorage:
```
sessionStorage['kaboom-event-buffer'] = '
{"timestamp":0,"type":"page_load","url":"..."}
{"timestamp":150,"type":"user_action","action":"input","element_id":"email-input"}
{"timestamp":280,"type":"network_request","method":"POST","url":"/api/login","id":"req-1"}
...
'
```

#### Recovery on page load:
- Content script checks sessionStorage for existing buffer
- Reads last event timestamp
- Continues buffer from there (don't reset on reload)
- Clear buffer only on tab close (unload event)

### 3. Causal Linking Engine
**Purpose:** Connect events to show cause → effect chains.

#### Linking rules:

| Event Pair | Link Rule |
|---|---|
| user_action + network_request | Link if action occurs <100ms before request |
| user_action + dom_mutation | Link if action occurs <50ms before DOM change |
| network_request + network_response | Link by request ID |
| network_response + console_error | Link if error occurs <200ms after response |
| user_action + subsequent dom_mutation | Link if same element involved |

#### Algorithm:
1. For each event E, search back in time for causal parents (last 5 seconds)
2. Apply linking rules: does E match a parent event pattern?
3. If match found, set `E.cause = parent_event_id`
4. Store causal graph in memory (WeakMap to avoid memory leak)

#### Example causal chain:
```
Event A: user_action (click Save) [timestamp: 1000]
Event B: network_request (POST /api/save) [timestamp: 1010, cause: A]
Event C: network_response (status: 401) [timestamp: 1050, cause: B]
Event D: console_error (Unauthorized) [timestamp: 1051, cause: C]
Event E: dom_mutation (Error modal appeared) [timestamp: 1080, cause: D]
```

### 4. Snapshot & Diff Generator
**Purpose:** Capture before/after state for user actions.

#### Before snapshot (on user action):
- Freeze current DOM state (serialize to JSON)
- Capture current console buffer
- Store network requests initiated by action (empty initially)

#### During action:
- Record all events triggered by this action
- Track timing

#### After snapshot (on action completion):
- Freeze new DOM state
- Generate diff: which nodes added/removed/changed?
- Summarize: "3 nodes added, 1 node removed, 2 attributes changed"
- Calculate execution time: timestamp_after - timestamp_before

#### Result summary generation:
```
Action: [action description]
Duration: [N ms]
DOM changes: [M nodes added/removed/changed]
Network: [K requests, responses]
Errors: [L console errors/warnings]
Result: [success/failure/timeout]
```

### 5. Timeline Serialization
**Purpose:** Convert causal graph to AI-readable format.

#### Output format:
- Chronological event list (sorted by timestamp)
- Each event includes: timestamp, type, summary, causal link
- Grouped events (e.g., all network activity from one user action)
- Context: "Action: click" → all related network/DOM/error events

#### Example output:
```json
{
  "timeline": [
    {
      "timestamp": "0:02.345",
      "type": "user_action",
      "action": "click",
      "element": "button#save",
      "kaboom_id": "btn-save",
      "result_summary": "1 network request, 1 error"
    },
    {
      "timestamp": "0:02.350",
      "type": "network_request",
      "method": "POST",
      "url": "/api/save",
      "duration_ms": 100,
      "result": "error"
    },
    {
      "timestamp": "0:02.450",
      "type": "network_response",
      "status": 401,
      "cause_action": "POST /api/save"
    },
    {
      "timestamp": "0:02.460",
      "type": "console_error",
      "message": "Unauthorized",
      "cause": "network_response"
    }
  ]
}
```

## Data Flows

### Event Capture Flow

```
User action (click button)
  ↓
Event listener fires
  ↓
Add to event queue (async)
  ↓
Batch worker (every 50ms)
  ↓
Write to sessionStorage
  ↓
Prune old events (maintain 60-second window)
```

### Timeline Query Flow

```
AI: observe({what: 'history'})
  ↓
Background: Read sessionStorage buffer
  ↓
Parse JSON events
  ↓
Build causal graph (link events)
  ↓
Generate summaries and diffs
  ↓
Serialize to AI-readable format
  ↓
Return timeline
```

### Page Reload Recovery

```
Page unload (navigation, reload, or crash)
  ↓
sessionStorage persists events
  ↓
New page load
  ↓
Content script checks sessionStorage
  ↓
Restore existing buffer
  ↓
Append new events to same buffer
  ↓
Buffer now includes events across reload
```

## Implementation Strategy

### Phase 1: Event Capture (Week 1)
- Implement event listeners for user actions (click, input, submit)
- Implement network interception (fetch + XHR hooks)
- Implement console hooks (log, error, warn)
- Test: verify events are captured in correct order

### Phase 2: Ring Buffer & Storage (Week 1-2)
- Implement ring buffer with sessionStorage persistence
- Implement recovery on page reload
- Test: reload page, verify buffer survives; verify pruning works

### Phase 3: Causal Linking (Week 2)
- Build causal link algorithm
- Test: verify user actions link to network requests
- Test: verify network responses link to console errors

### Phase 4: Snapshot & Diff (Week 2-3)
- Capture before/after DOM snapshots
- Implement diff algorithm
- Generate human-readable summaries
- Test: verify diff accuracy against real page changes

### Phase 5: Serialization & Integration (Week 3)
- Implement timeline serialization (JSON format)
- Add `observe({what: 'history'})` handler
- Integration test: full flow from user action to timeline

### Phase 6: Testing & Polish (Week 3-4)
- UAT on real apps (e.g., React Todo, Vue counter, vanilla form)
- Performance testing: ensure <1ms per event
- Memory testing: ensure 60-sec buffer stays <5MB

## Edge Cases & Assumptions

| Case | Description | Handling |
|---|---|---|
| **Page reload during capture** | Events being written while page unloads | Async queue is flushed before unload; events saved to sessionStorage |
| **sessionStorage unavailable** | Safari private mode, quota exceeded | Fall back to in-memory ring buffer (lost on reload) |
| **Large number of events** | 1000+ events in 60 seconds | Use compression (gzip) for storage; prune oldest events |
| **Network interception failures** | fetch() hook doesn't work (e.g., old browser) | Network events absent from timeline; other events still captured |
| **Cross-origin requests** | CORS requests don't expose response body | Capture request metadata (method, URL, status); skip body |
| **Service Worker** | Request intercepts at SW layer, not visible to content script | May miss network events (graceful degradation) |
| **Multiple tabs** | Same domain in multiple tabs | Each tab has independent sessionStorage; events don't cross-pollinate |
| **Zero-time actions** | User action completes in <1ms | Use high-resolution timestamps (performance.now()); round to nearest 1ms |

## Risks & Mitigations

| Risk | Description | Mitigation |
|---|---|---|
| **Privacy leak** | Capturing network bodies exposes sensitive data | Body capture is opt-in; default to headers-only. Implement redaction patterns (email, credit card, SSN). |
| **Memory leak** | Events stored indefinitely | Use ring buffer with automatic eviction. Weak references for causal graph. Clear on tab close. |
| **Perf impact** | Capturing all events slows main thread | Async queues; batch processing every 50ms; cap queue size to prevent backlog. |
| **Causality errors** | Incorrect linking misguides AI | Linking rules are conservative; only link if high confidence. Manual inspection possible via raw timeline. |
| **Race conditions** | Events out of order due to async capture | Use timestamps for ordering. If timestamp collision, use insertion order as tiebreaker. |
| **Large pages** | Expensive DOM serialization | Use shallow snapshots (only first N levels); skip large subtrees. Lazy diff. |

## Dependencies

### Existing Features Required
- Content script injection (already exists)
- Message passing to background (already exists)
- sessionStorage API access (standard browser API)

### Browser APIs
- `performance.now()` — High-res timestamps
- `MutationObserver` — DOM mutation tracking
- `sessionStorage` — Event buffer persistence
- `fetch()` and `XMLHttpRequest` — Network interception (monkey-patch)
- `console` — Hook into log/error/warn
- Event listeners — Standard DOM event API

### No External Dependencies
- No npm packages (matches Kaboom's zero-deps philosophy)

## Performance Considerations

### Target Benchmarks
- **Event capture:** <0.5ms per event
- **Buffer write:** <1ms per batch (50 events)
- **History query:** <100ms to serialize 60-second timeline
- **Memory footprint:** <5MB for 60-second buffer

### Optimization Strategies
- **Async queues** — Prevent blocking main thread
- **Batch processing** — Write 50 events at once (reduce sessionStorage writes)
- **Lazy serialization** — Only serialize requested time range
- **Compression** — Use gzip for storage if buffer > 500KB

### Memory Profile
- **Per event:** ~200 bytes (timestamp, type, details)
- **60-second buffer:** ~1000 events → 200KB raw (100KB compressed)
- **Causal graph:** <10KB (stored as WeakMap, auto-GC'd)
- **DOM snapshots:** ~50KB per snapshot (not persisted, in-memory only)

## Security Considerations

### Data Exposure
- **Network requests** — Headers captured, bodies optional (redact by default)
- **Console logs** — All captured (may contain sensitive info)
- **DOM content** — Captured (public HTML, but may reveal app structure)
- **User actions** — Element IDs and timestamps captured

### Injection Risks
- **Event listener hijacking** — Attacker could register fake listeners; mitigated by running first (before user scripts)
- **sessionStorage tampering** — XSS could modify buffer; mitigated by content security policy

### Privacy Mitigations
- [ ] Network body redaction: mask email addresses, credit cards, SSNs in bodies
- [ ] Console filtering: option to exclude 3rd-party logs (GTM, analytics)
- [ ] Scope: buffer stored per-session (cleared on tab close)
- [ ] Transparency: debug log shows what's being captured

### Compliance
- [ ] No data sent to server (all client-side)
- [ ] sessionStorage scoped to origin (no cross-origin leaks)
- [ ] Audit log available (show what events were captured)

## Serialization Format

### JSON schema for timeline:

```json
{
  "timeline": [
    {
      "id": "evt-1",
      "timestamp_ms": 0,
      "timestamp_human": "0:00.000",
      "type": "page_load",
      "url": "https://example.com/page"
    },
    {
      "id": "evt-2",
      "timestamp_ms": 3214,
      "timestamp_human": "0:03.214",
      "type": "user_action",
      "action": "click",
      "element": { "id": "save-btn", "text": "Save", "kaboom_id": "btn-save" },
      "result_summary": "1 network request, 0 errors"
    },
    {
      "id": "evt-3",
      "timestamp_ms": 3350,
      "timestamp_human": "0:03.350",
      "type": "network_request",
      "method": "POST",
      "url": "/api/save",
      "request_id": "req-1",
      "cause": "evt-2"
    }
  ]
}
```

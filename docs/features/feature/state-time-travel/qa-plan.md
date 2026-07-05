---
feature: State Time-Travel
doc_type: qa-plan
feature_id: feature-state-time-travel
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: State Time-Travel

> How to test event capture, causal linking, and timeline serialization.

## Testing Strategy

### Code Testing (Automated)

#### Unit Tests: Event Capture

- [ ] **User action capture** — Click event on button is recorded with timestamp and element info
- [ ] **Input capture** — Input event on form field records value changes
- [ ] **Submit capture** — Form submit event is recorded with form data
- [ ] **Network fetch** — fetch() calls are intercepted and recorded
- [ ] **Network XHR** — XMLHttpRequest calls are intercepted and recorded
- [ ] **Network response** — Response status and timing recorded with matching request
- [ ] **Console log** — console.log() output captured with level and timestamp
- [ ] **Console error** — console.error() and console.warn() captured
- [ ] **DOM mutation** — MutationObserver detects element add/remove/attribute change
- [ ] **Page load** — Page load event recorded at timestamp 0
- [ ] **Page unload** — Page unload event recorded before buffer persist
- [ ] **Error event** — window.error and unhandledrejection events captured
- [ ] **Custom events** — CustomEvent dispatches recorded (if app uses them)

#### Unit Tests: Ring Buffer & Storage

- [ ] **Storage in sessionStorage** — Events written to `sessionStorage['kaboom-event-buffer']`
- [ ] **Recovery on reload** — After page reload, buffer restored from sessionStorage
- [ ] **FIFO eviction** — When buffer exceeds 60 seconds, oldest events dropped
- [ ] **Capacity management** — Buffer size stays under 5MB limit
- [ ] **JSON format** — Events stored as JSON lines (one per line, parseable)
- [ ] **Timestamp ordering** — Events retrieved in chronological order
- [ ] **Compression** — Large buffers (>500KB) compressed with gzip
- [ ] **Compression recovery** — Compressed buffers decompressed correctly
- [ ] **No sessionStorage** — Falls back to in-memory ring buffer if unavailable

#### Unit Tests: Causal Linking

- [ ] **User action → network request** — Click on button linked to fetch() call within 100ms
- [ ] **User action → DOM mutation** — Click linked to subsequent DOM change within 50ms
- [ ] **Network request → response** — POST request linked to response by request ID
- [ ] **Network response → console error** — 401 response linked to console.error() within 200ms
- [ ] **No false links** — Unrelated events not linked incorrectly
- [ ] **Multiple events** — Page with 100+ events: all linked correctly
- [ ] **Temporal causality** — Earlier events always causal parents of later events

#### Unit Tests: Snapshot & Diff Generator

- [ ] **Before snapshot** — DOM state captured before user action
- [ ] **After snapshot** — DOM state captured after action completes
- [ ] **Diff calculation** — Nodes added/removed/changed counted correctly
- [ ] **Summary generation** — "2 nodes added, 1 error, 1 network request" generated correctly
- [ ] **Execution time** — Duration calculated as timestamp_after - timestamp_before
- [ ] **Console capture** — Logs during action grouped with action snapshot
- [ ] **Network capture** — Network requests during action grouped with action snapshot

#### Unit Tests: Timeline Serialization

- [ ] **JSON structure** — Output is valid JSON
- [ ] **Timestamp format** — Both milliseconds and human-readable "0:01.234" format
- [ ] **Causal links** — Each event with cause links to parent event ID
- [ ] **Summaries** — Action summaries are concise and AI-readable
- [ ] **Empty timeline** — Empty buffer returns empty timeline (no crash)
- [ ] **Large timeline** — 1000+ events serialized in <100ms

#### Integration Tests: End-to-End

- [ ] **Full flow** — User action → network → response → console → DOM timeline
- [ ] **Cross-reload** — Events persist across page reload
- [ ] **Multiple actions** — 10+ sequential actions all captured and linked
- [ ] **Concurrent events** — Events from multiple sources (network + console) linked correctly
- [ ] **Real React app** — Real React app: state changes captured via DOM mutations
- [ ] **Real Vue app** — Real Vue app: state changes captured via DOM mutations
- [ ] **Real vanilla JS** — Vanilla JS app: clicks, inputs, events all captured

#### Edge Case Tests

- [ ] **Zero-time action** — Action completing in <1ms still has valid duration
- [ ] **Simultaneous events** — Multiple events at same timestamp ordered by insertion
- [ ] **Removed element** — Event on element that's later removed still linked correctly
- [ ] **Large page** — 5000+ DOM nodes: capture and diff still complete in <200ms
- [ ] **Many network requests** — 100+ concurrent requests: all tracked with request IDs
- [ ] **Rapid mutations** — 1000+ DOM mutations per second: queue handles backlog

### Security/Compliance Testing

- [ ] **Network body redaction** — Network bodies contain no unredacted emails/credit cards
- [ ] **Console data scrubbing** — PII in console logs redacted (opt-in capture)
- [ ] **XSS prevention** — Malicious content in events doesn't execute
- [ ] **sessionStorage scope** — Events scoped to origin (no cross-origin read)
- [ ] **Tab isolation** — Events in one tab don't leak to another tab

---

## Human UAT Walkthrough

### Scenario 1: Page Reload Crash Recovery

#### Setup:
1. Create a simple form with email and password fields
2. Form submit handler has bug: calls `form.submit()` instead of preventing default
3. Submit goes to server (or dummy endpoint)

#### Steps:
1. [ ] Load page with Kaboom attached
2. [ ] Enter email in form field
3. [ ] Click Submit button
4. [ ] Page reloads
5. [ ] After reload, open Kaboom DevTools → History tab
6. [ ] Call `observe({what: 'history'})`

#### Expected Result:
- Timeline shows events from before the reload:
  ```
  [0:00] Page load
  [0:02] Input "email" field with value "user@example.com"
  [0:05] Click Submit button
  [0:05] Network POST /form (if applicable)
  [0:06] Page unload
  [0:07] Page reload
  ```
- Events are not cleared by reload
- AI can see: "User submitted form → page reloaded"

#### Verification:
- Buffer recovered correctly
- Events ordered chronologically
- No events lost due to reload

### Scenario 2: Transient Loading Spinner

#### Setup:
1. Create a page with a button that triggers a 3-second API call
2. Show a spinner during the load
3. Hide spinner when response arrives

#### Steps:
1. [ ] Load page
2. [ ] Click the button
3. [ ] Observe the spinner appear and disappear (3 seconds)
4. [ ] Immediately after (before spinner is gone), call `observe({what: 'history'})`

#### Expected Result:
- Timeline includes:
  ```
  [0:01] Click button
  [0:01] Network POST /api/slow (duration: 3000ms)
  [0:01] DOM mutation: Spinner appeared
  [0:04] DOM mutation: Spinner removed
  [0:04] Network response: 200 OK
  ```
- Spinner events recorded even though they were transient
- Timeline shows exact duration spinner was visible (3 seconds)

#### Verification:
- Transient UI elements are not missed
- Timing shows precisely when spinner appeared and disappeared
- AI can diagnose: "Load takes 3 seconds; consider optimizing"

### Scenario 3: Causal Error Diagnosis

#### Setup:
1. Create a form that makes an API call when submitted
2. API endpoint returns 401 if user not authenticated
3. Error handler logs error and shows error modal

#### Steps:
1. [ ] Load page without authentication token
2. [ ] Fill form and click Submit
3. [ ] Wait for error modal to appear
4. [ ] Call `observe({what: 'history'})`

#### Expected Result:
- Timeline shows causal chain:
  ```
  [0:02] Click Submit button
  [0:02] Network POST /api/save initiated
  [0:02] Network response: 401 Unauthorized
  [0:03] Console error: "User not authenticated"
  [0:03] DOM mutation: Error modal appeared
  ```
- Events are linked: response → error → DOM change
- Result summary shows: "1 error, 1 network failure"

#### Verification:
- Causal chain is clear and correct
- Root cause immediately obvious: 401 response

### Scenario 4: Multiple Concurrent Events

#### Setup:
1. Create a page with multiple buttons
2. Each button triggers different async operations (network, DOM, setTimeout)

#### Steps:
1. [ ] Load page
2. [ ] Rapidly click 5 different buttons (within 1 second)
3. [ ] Wait for all operations to complete (5+ seconds total)
4. [ ] Call `observe({what: 'history'})`

#### Expected Result:
- All 5 actions captured with separate timelines
- Each action shows its own result summary
- No events lost despite rapid clicking
- Events ordered chronologically

#### Verification:
- Concurrent event handling works correctly
- All events present in timeline
- Order is preserved

### Scenario 5: Buffer Persistence Across Navigation

#### Setup:
1. Load a single-page app (React, Vue) with client-side routing

#### Steps:
1. [ ] Load page A
2. [ ] Click on events (click, input, network)
3. [ ] Navigate to page B (client-side router, no full reload)
4. [ ] Perform more events on page B
5. [ ] Call `observe({what: 'history'})`

#### Expected Result:
- Timeline includes events from both page A and page B
- Events ordered chronologically across navigation
- No "page reload" event (because it's client-side routing)
- Buffer shows continuous timeline

#### Verification:
- Buffer survives client-side navigation
- Events from multiple "pages" preserved

### Scenario 6: Real-World Debugging

#### Setup:
1. Use a real web app (e.g., Todoist, GitHub, or your own)
2. Reproduce a bug (e.g., task doesn't save, button doesn't respond)

#### Steps:
1. [ ] Load page with Kaboom
2. [ ] Perform actions that trigger the bug (click, input, etc.)
3. [ ] Open Kaboom DevTools → History tab
4. [ ] Review the timeline

#### Expected Result:
- Timeline shows exact sequence of what happened
- Network requests and responses visible
- Console errors (if any) linked to causal actions
- Root cause of bug is apparent from timeline

#### Verification:
- Timeline matches actual user actions
- No events missed
- Bug root cause can be identified from timeline alone

---

## Regression Testing

### Existing features to verify don't break:

- [ ] `observe({what: 'page'})` still works (DOM observation unaffected)
- [ ] `observe({what: 'network'})` still works (network capture unaffected)
- [ ] `observe({what: 'console'})` still works (console capture unaffected)
- [ ] `interact({action: 'execute_js'})` still works
- [ ] `interact({action: 'click'})` still works (events still captured)
- [ ] Page performance is not degraded by event capture
- [ ] Memory usage is bounded (not growing unbounded)

### Performance regression:

- [ ] Page load time increase <50ms
- [ ] 100 sequential actions complete without timeout
- [ ] Memory usage stays <10MB (buffer + caches)
- [ ] observe() calls complete in <150ms

---

## Performance/Load Testing

- [ ] **Event rate** — 1000 events in 60 seconds: buffer still functions
- [ ] **Buffer size** — 60-second buffer with typical event rate stays <5MB
- [ ] **Serialization speed** — 1000-event timeline serialized in <100ms
- [ ] **Causal linking** — 1000 events linked in <50ms
- [ ] **sessionStorage limit** — Buffer handles sites near sessionStorage 10MB limit gracefully
- [ ] **Compression** — Compressed buffer 3-5x smaller than raw
- [ ] **Decompression** — Decompression <50ms for typical buffer size

---

## Privacy & Security Testing

- [ ] **Network body redaction** — Test with API response containing email: redaction works
- [ ] **Network body redaction** — Test with credit card number: redaction works
- [ ] **Console scrubbing** — PII in console logs redacted (if enabled)
- [ ] **sessionStorage isolation** — Events in one tab don't appear in another tab
- [ ] **Tab closure** — Buffer cleared when tab is closed
- [ ] **Origin isolation** — Events from external origin not mixed in buffer

---

## Sign-Off Criteria

- ✅ All unit tests passing
- ✅ All integration tests passing
- ✅ All UAT scenarios passing
- ✅ No regressions in existing features
- ✅ Performance benchmarks met (<150ms observe, <5MB buffer)
- ✅ Security tests passing (redaction, isolation, PII)
- ✅ Manual testing on real apps successful (React, Vue, vanilla)
- ✅ Buffer correctly survives page reload
- ✅ Transient UI events captured (not missed)
- ✅ Causal chains are accurate and helpful for debugging

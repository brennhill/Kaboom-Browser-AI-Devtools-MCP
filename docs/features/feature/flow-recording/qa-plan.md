---
feature: Flow Recording & Playback
status: ready
version: 0.7.12
doc_type: qa-plan
feature_id: feature-flow-recording
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Flow Recording & Playback (Regression Testing)

**Purpose:** Define WHAT to test and HOW to verify Phase 1 MVP works correctly.

**Ownership:** QA engineer / Tech lead

**Gate 4 Requirement:** This plan is executable without implementation details. Tests are written BEFORE code (TDD).

---

## Testing Philosophy

### Test-Driven Development (TDD):
1. Write failing tests FIRST (from this qa-plan.md)
2. Run `make test` → see tests fail ✗
3. Implement minimal code to make tests pass
4. Run `make test` → see tests pass ✓
5. Commit with all tests green

**Coverage Target:** ≥ 90% for all new code

### Test Execution:
```bash
# All tests (Go + extension)
make test

# Go tests only
go test ./cmd/browser-agent/...

# Extension tests only
node --test tests/extension/recording.test.js

# Full CI (includes lint, typecheck)
make ci-local
```

---

## Unit Tests (Code Level)

### Module 1: WebSocket Migration

**File:** `cmd/browser-agent/websocket_test.go` (new)

#### Test Case 1.1: WebSocket Connection Established
```
GIVEN: Server running on localhost:3001
WHEN: Extension calls POST /api/ws-connect with valid API key
THEN: WebSocket upgrade successful
AND: Connection state = "connected"
AND: No polling (WS is primary)
```

##### Implementation Notes:
- Mock WebSocket server using gorilla/websocket
- Verify HTTP 101 upgrade response
- Verify connection remains open for 5+ seconds

#### Test Case 1.2: Real-Time Event Streaming
```
GIVEN: Active WebSocket connection
WHEN: Server emits 5 log events
THEN: Extension receives all 5 events in < 100ms
AND: Event order preserved
AND: No duplicates
```

##### Implementation Notes:
- Send test log events from server → WS client
- Measure latency (should be < 10ms, < 100ms is safe margin)
- Verify event sequence numbers increment

#### Test Case 1.3: Buffer Overflow Handling
```
GIVEN: WebSocket broadcast buffer at max capacity (10,000 events)
WHEN: Server receives 11,000th event
THEN: Oldest event dropped (ring buffer behavior)
AND: Warning logged: "WebSocket buffer overflow"
AND: Newest 10,000 events retained
```

##### Implementation Notes:
- Fill broadcast buffer with dummy events
- Verify FIFO (first-in-first-out) on overflow
- Verify warning appears in logs

#### Test Case 1.4: Connection Drop + Polling Fallback
```
GIVEN: Active WebSocket connection
WHEN: Connection drops (network error)
THEN: Extension detects drop within 5 seconds
AND: Falls back to polling (GET /pending-queries)
AND: Continues capturing events (no data loss)
```

##### Implementation Notes:
- Simulate network failure (close socket)
- Verify extension timeout detection (5s)
- Verify polling mode activated
- Verify event capture continues

#### Test Case 1.5: Reconnection with Exponential Backoff
```
GIVEN: WebSocket connection dropped
WHEN: Network becomes available again
THEN: Extension reconnects with backoff: 100ms, 200ms, 400ms, 800ms, 1600ms
AND: Eventually reconnects (< 10 seconds)
AND: Resumes streaming (no polling after reconnect)
```

##### Implementation Notes:
- Simulate temporary network outage (1-5 seconds)
- Verify backoff progression
- Verify WS mode restored after reconnect

---

### Module 2: Recording Storage (Server)

**File:** `cmd/browser-agent/recording_test.go` (new)

#### Test Case 2.1: Create Recording Metadata
```
GIVEN: User calls configure({what: 'recording_start', name: 'checkout', url: 'https://...'})
WHEN: Recording created
THEN: metadata.json saved to ~/.kaboom/recordings/{id}/metadata.json
AND: File contains: id, name, created_at, duration, action_count, start_url, viewport, sensitive_data_enabled
AND: Response: {status: "ok", recording_id: "checkout-20260130T143022Z"}
```

##### Implementation Notes:
- Create temp directory for test
- Verify JSON structure
- Verify timestamp is valid ISO8601
- Cleanup temp directory after test

#### Test Case 2.2: Add Actions to Recording
```
GIVEN: Active recording (recording_id = "checkout-123")
WHEN: 5 actions sent via POST /query: click, type, navigate, click, type
THEN: All actions added to recording in memory
AND: Each action has: type, timestamp_ms, selector, x, y, screenshot_path
AND: Timestamps in ascending order
```

##### Implementation Notes:
- Send 5 actions with varying timestamps
- Verify in-memory buffer updated
- Verify action order preserved

#### Test Case 2.3: Persist Recording to Disk
```
GIVEN: Active recording with 10 actions
WHEN: configure({what: 'recording_stop', recording_id: '...'})
THEN: metadata.json persisted with all 10 actions
AND: File readable as valid JSON
AND: action_count = 10
AND: duration_ms > 0
```

##### Implementation Notes:
- Read back persisted metadata.json
- Verify all 10 actions present
- Verify no data loss

#### Test Case 2.4: Sensitive Data Redaction
```
GIVEN: Recording with sensitive_data_enabled = false (default)
WHEN: Type action on password input: "my_password_123"
THEN: Stored as: {type: "type", text: "[redacted]", ...}
AND: Original text never stored
```

##### Implementation Notes:
- Mock type action on password field
- Verify redaction in metadata.json
- Verify redaction applied by extension, not server

#### Test Case 2.5: Sensitive Data Full Capture (Opt-In)
```
GIVEN: User calls configure({what: 'recording_start', sensitive_data_enabled: true})
AND: Extension shows warning popup (mocked in test)
WHEN: Type action on password input: "test_password"
THEN: Stored as: {type: "type", text: "test_password", ...}
AND: metadata.json: sensitive_data_enabled: true
```

##### Implementation Notes:
- Mock warning popup acceptance
- Verify full text captured
- Verify flag set in metadata

#### Test Case 2.6: Storage Quota Enforcement
```
GIVEN: Recording storage at 100% (1GB used)
WHEN: User calls configure({what: 'recording_start', name: 'new'})
THEN: Error returned: "recording_storage_full: Recording storage at capacity (1GB)..."
AND: No recording created
AND: Next call still fails (no auto-delete)
```

##### Implementation Notes:
- Create 1GB of dummy recordings
- Attempt to start new recording
- Verify error code and message
- Verify user must manually delete

#### Test Case 2.7: Storage Warning at 80%
```
GIVEN: Recording storage at 80% (800MB used)
WHEN: Any recording operation
THEN: Warning logged: "recording_storage_warning: Recording storage at 80%..."
AND: Operation proceeds (non-blocking)
```

##### Implementation Notes:
- Create 800MB of dummy recordings
- Start new recording
- Verify warning in logs
- Verify recording created successfully

#### Test Case 2.8: List Recordings
```
GIVEN: 5 recordings stored on disk
WHEN: observe({what: 'recordings', limit: 10})
THEN: Returns array of 5 recordings
AND: Each includes: id, name, created_at, action_count, url
AND: Sorted by created_at (newest first)
```

##### Implementation Notes:
- Create 5 test recordings
- Query via observe
- Verify response format
- Verify sorting order

#### Test Case 2.9: Query Recording Actions
```
GIVEN: Recording with 10 actions
WHEN: observe({what: 'recording_actions', recording_id: 'checkout-123'})
THEN: Returns: {recording_id: "...", actions: [...10 items...]}
AND: Each action has all fields
AND: Timestamps in order
```

##### Implementation Notes:
- Create recording with 10 actions
- Query actions
- Verify all fields present
- Verify no data loss

---

### Module 3: Playback Engine

**File:** `cmd/browser-agent/playback_test.go` (new)

#### Test Case 3.1: Load Recording
```
GIVEN: Recording stored at ~/.kaboom/recordings/checkout-123/metadata.json
WHEN: playback.LoadRecording("checkout-123")
THEN: Recording loaded successfully
AND: All 8 actions in memory
AND: No errors
```

##### Implementation Notes:
- Create test recording on disk
- Call LoadRecording
- Verify all actions loaded

#### Test Case 3.2: Execute Navigate Action
```
GIVEN: Playback engine with action: {type: "navigate", url: "https://example.com", ...}
AND: Mock browser navigation + network idle detection
WHEN: Playback executes action
THEN: Browser navigates to URL
AND: Waits for network idle (0 active HTTP requests)
AND: Timeout = 5 seconds (hard limit)
AND: Result: {status: "ok", action_executed: true, duration_ms: 1250}
```

##### Implementation Notes:
- Mock HTTP request tracking
- Mock setTimeout/network wait
- Verify wait logic (100ms polling interval)
- Verify timeout behavior

#### Test Case 3.3: Execute Click Action
```
GIVEN: Playback action: {type: "click", selector: "[data-testid=add-to-cart]", x: 500, y: 300}
AND: Element exists on page
WHEN: Playback executes click
THEN: Element found via querySelector
AND: Element clicked
AND: Screenshot taken (if configured)
AND: Result: {action_executed: true, errors: []}
```

##### Implementation Notes:
- Mock DOM querySelector
- Verify click event fired
- Verify no errors

#### Test Case 3.4: Execute Type Action
```
GIVEN: Playback action: {type: "type", selector: "input#email", text: "test@example.com"}
AND: Element is <input type="text">
WHEN: Playback executes type
THEN: Input found
AND: Text typed into input
AND: Element.value = "test@example.com"
AND: Result: {action_executed: true}
```

##### Implementation Notes:
- Mock input element
- Verify text entry
- Verify no errors on valid input

#### Test Case 3.5: Element Not Found → Self-Healing Fallback
```
GIVEN: Playback action with selector: "[data-testid=product-1]"
AND: Element not found on current page
WHEN: Playback executes click
THEN: Tries data-testid match → fails
AND: Tries CSS selector fallback → fails
AND: Tries nearby x/y search → fails
AND: Uses last-known x/y coordinates
AND: Logs error: "Element not found after self-healing"
AND: Screenshot taken with issue type: "selector_not_found"
AND: Result: {action_executed: false, error: "Element not found..."}
```

##### Implementation Notes:
- Mock element not found scenario
- Verify self-healing strategy progression
- Verify fallback to x/y
- Verify error logging

#### Test Case 3.6: Selector Fragility Detection
```
GIVEN: 3 playback runs for same recording
AND: Selector "[data-testid=button]" changes location each run:
  Run 1: {x: 500, y: 200}
  Run 2: {x: 505, y: 210}
  Run 3: {x: 512, y: 215}
WHEN: Playback detects 3+ location changes
THEN: Warning logged: "Fragile selector: element moved 3 times..."
AND: Screenshot with issue type: "moved-selector"
AND: Recommendation: "Add data-testid=... or update selector"
```

##### Implementation Notes:
- Track selector location across runs
- Verify fragility detection threshold (3+)
- Verify warning message
- Verify screenshot captured

#### Test Case 3.7: Screenshot on Error
```
GIVEN: Playback execution fails on action 3 (selector not found)
WHEN: Error captured
THEN: Screenshot taken automatically
AND: Stored at: ~/.kaboom/recordings/{id}/screenshots/{timestamp}-{index}-error.jpg
AND: Screenshot included in error response
```

##### Implementation Notes:
- Mock screenshot function
- Trigger error scenario
- Verify screenshot path
- Verify included in response

#### Test Case 3.8: Playback with LLM-Generated Actions
```
GIVEN: Custom action array (not from recording):
[
  {type: "navigate", url: "https://shop.com", ...},
  {type: "click", selector: "[data-testid=coupon-btn]", ...},
  {type: "type", selector: "input#coupon", text: "SUMMER2026", ...}
]
WHEN: interact({what: 'playback', actions: [...], test_id: 'variation-1'})
THEN: Playback executes custom actions
AND: All events tagged with test_id: 'variation-1'
AND: Result: {status: "ok", actions_executed: 3, errors: []}
```

##### Implementation Notes:
- Accept custom action array
- Verify execution same as recorded
- Verify test boundary tagging

#### Test Case 3.9: Non-Blocking Error Handling
```
GIVEN: Playback sequence with 5 actions
AND: Action 2 fails (selector not found)
WHEN: Playback executes all 5
THEN: Actions 1, 3, 4, 5 succeed
AND: Action 2 fails (logged, not blocking)
AND: Result: {actions_executed: 5, errors: [{action_index: 2, ...}]}
AND: Playback completes successfully
```

##### Implementation Notes:
- Create scenario with one failing action
- Verify non-blocking behavior
- Verify all 5 executed (despite error)
- Verify error array populated

---

### Module 4: Log Diffing Engine

**File:** `cmd/browser-agent/log_diff_test.go` (new)

#### Test Case 4.1: Compare Identical Logs
```
GIVEN: Original logs (test_boundary: 'original-checkout'):
  [
    {type: "network", timestamp_ms: 1000, url: "POST /api/cart", status: 200},
    {type: "console", timestamp_ms: 2000, level: "info", message: "Cart updated"}
  ]
AND: Replay logs (test_boundary: 'replay-checkout') identical
WHEN: log-diff compares both
THEN: Result: {status: "match", newErrors: [], missingEvents: [], changedValues: []}
```

##### Implementation Notes:
- Create two identical log sets
- Compare via log-diff
- Verify "match" status

#### Test Case 4.2: Detect New Errors (Regression)
```
GIVEN: Original logs: POST /api/cart → 200 OK
AND: Replay logs: POST /api/cart → 404 Not Found
WHEN: log-diff compares
THEN: Result includes:
  newErrors: [{type: "network", url: "POST /api/cart", status: 404, message: "Not Found"}]
AND: status: "regression"
```

##### Implementation Notes:
- Original: 200 response
- Replay: 404 response
- Verify new error detected
- Verify categorization as regression

#### Test Case 4.3: Detect Missing Events (Broken Feature)
```
GIVEN: Original logs include: {type: "console", message: "Payment processed"}
AND: Replay logs do NOT include it
WHEN: log-diff compares
THEN: Result includes:
  missingEvents: [{type: "console", message: "Payment processed"}]
AND: status: "regression" (feature broken)
```

##### Implementation Notes:
- Original has success event
- Replay missing it
- Verify detection
- Verify categorization

#### Test Case 4.4: Detect Value Changes
```
GIVEN: Original logs:
  POST /api/order → {orderId: 123, status: "confirmed"}
AND: Replay logs:
  POST /api/order → {orderId: 456, status: "pending"}
WHEN: log-diff compares
THEN: Result includes:
  changedValues: [{event: "orderId", original: "123", replay: "456"}]
AND: status: "unknown" (manual review needed)
```

##### Implementation Notes:
- Compare response values
- Detect differences
- Verify value change logged

#### Test Case 4.5: Detect Fixed Regressions
```
GIVEN: Original logs: POST /api/order → 500 Error
AND: Replay logs: POST /api/order → 200 OK
WHEN: log-diff compares
THEN: Result:
  missingEvents: [{status: 500, message: "Error"}]
  newErrors: []
AND: status: "fixed" (regression resolved)
```

##### Implementation Notes:
- Original has error
- Replay succeeds
- Verify status = "fixed"

#### Test Case 4.6: Category Errors (Network, Console, DOM)
```
GIVEN: Mixed log types:
  - Network: POST /api/cart → 404
  - Console: error "Missing API key"
  - DOM: querySelector(".pay-btn") → null
WHEN: log-diff categorizes
THEN: Errors grouped by type:
  newErrors: [
    {type: "network", ...},
    {type: "console", ...},
    {type: "dom", ...}
  ]
```

##### Implementation Notes:
- Include all three log types
- Verify categorization
- Verify all types detected

#### Test Case 4.7: Extract Error Context
```
GIVEN: Error log: POST /api/order → 404
WHEN: log-diff extracts context
THEN: Error includes:
  - message: "404 Not Found"
  - url: "POST /api/order"
  - statusCode: 404
  - timestamp: (unix milliseconds)
  - affected_action: 5 (if matching action index)
```

##### Implementation Notes:
- Verify error details extracted
- Verify correlation to action
- Verify complete context

---

### Module 5: Extension Recording

**File:** `tests/extension/recording.test.js` (new)

#### Test Case 5.1: Start Recording
```
GIVEN: Extension popup open
WHEN: User clicks "Start Recording" button
AND: Enters name: "checkout-flow"
AND: Enters URL: "https://shop.example.com"
THEN: Recording state changed to "recording"
AND: Button changes to "Stop Recording"
AND: MCP action sent: configure({what: 'recording_start', name: 'checkout-flow', ...})
AND: Server responds with recording_id
```

##### Implementation Notes:
- Mock popup UI
- Mock button click
- Verify MCP call
- Verify UI state change

#### Test Case 5.2: Stop Recording
```
GIVEN: Recording in progress
WHEN: User clicks "Stop Recording"
THEN: Recording state changed to "stopped"
AND: MCP action sent: configure({what: 'recording_stop', recording_id: '...'})
AND: Button changes back to "Start Recording"
AND: Recording saved to disk (via server)
```

##### Implementation Notes:
- Verify stop action sent
- Verify state reset
- Verify server receives stop

#### Test Case 5.3: Capture Click Action
```
GIVEN: Recording active
WHEN: User clicks element with [data-testid=add-to-cart]
THEN: Action captured:
  {
    type: "click",
    selector: "[data-testid=add-to-cart]",
    x: 520,
    y: 300,
    timestamp_ms: 1234567890
  }
AND: Sent to recording storage
```

##### Implementation Notes:
- Mock click event
- Verify action structure
- Verify selector extraction
- Verify coordinates captured

#### Test Case 5.4: Capture Type Action
```
GIVEN: Recording active
AND: User focused on input#email
WHEN: User types "test@example.com"
THEN: Action captured:
  {
    type: "type",
    selector: "input#email",
    text: "test@example.com",
    x: 500,
    y: 450,
    timestamp_ms: 1234567900
  }
AND: If sensitive_data_enabled=false: text: "[redacted]"
```

##### Implementation Notes:
- Mock input element
- Mock keypress events
- Verify text captured (or redacted)
- Verify input selector extracted

#### Test Case 5.5: Capture Navigation
```
GIVEN: Recording active
WHEN: Page navigates to "https://shop.example.com/checkout"
THEN: Action captured:
  {
    type: "navigate",
    url: "https://shop.example.com/checkout",
    timestamp_ms: 1234568000
  }
AND: Sent to recording storage
```

##### Implementation Notes:
- Mock navigation event
- Verify URL captured
- Verify timestamp

#### Test Case 5.6: Screenshot on Page Load
```
GIVEN: Recording active
AND: Recording: {when_captured: ["page-load"]}
WHEN: Page finishes loading (network idle detected)
THEN: Screenshot taken automatically
AND: Stored at: ~/.kaboom/recordings/{id}/screenshots/{ts}-page-load.jpg
AND: Reference stored in action metadata
```

##### Implementation Notes:
- Mock page load
- Verify screenshot triggered
- Verify stored correctly

#### Test Case 5.7: Recording Metadata Auto-Generated Name
```
GIVEN: User starts recording without entering name (empty)
WHEN: Recording created
THEN: Name auto-generated: "{adjective}-{noun}-{adjective}-{ISO8601}"
  Example: "swift-badger-hammer-20260130T143022Z"
AND: ID matches generated name
```

##### Implementation Notes:
- Mock empty name input
- Verify auto-generation logic
- Verify ID format

---

## Integration Tests (End-to-End)

**File:** `cmd/browser-agent/integration_test.go` (new)

### Integration Test 1: Full Recording → Playback → Diff Workflow

```
SCENARIO: Record a shopping flow, simulate fix, replay, and verify regression is fixed

GIVEN:
  - Test server running with sample shopping app
  - Kaboom server running on localhost:3001
  - WebSocket connected

WHEN:
  Step 1: Record original flow
    configure({what: 'test_boundary_start', test_id: 'original-shop'})
    User: click "Product", type "Apple", click "Add to Cart", click "Checkout"
    configure({what: 'test_boundary_end', test_id: 'original-shop'})
    → All actions + logs captured

  Step 2: Simulate bug fix in shopping app
    (Change POST /api/cart endpoint from /api/order to /api/checkout)

  Step 3: Replay the recorded flow
    interact({
      action: 'playback',
      recording: 'shopping-flow-20260130T...',
      test_id: 'replay-shop'
    })
    → Playback executes same 4 actions
    → New logs captured under test_id: 'replay-shop'

  Step 4: Compare logs
    observe({what: 'logs', test_id: 'original-shop'})
    observe({what: 'logs', test_id: 'replay-shop'})
    → Log diffing detects: POST /api/checkout now returns 200 (was 404)

THEN:
  - All 4 actions executed successfully
  - No new errors in replay
  - Regression is FIXED ✓
  - LLM can see: "POST /api/checkout now succeeds (was failing)"
```

#### Success Criteria:
- ✓ All 4 actions captured in original
- ✓ All 4 actions executed in replay
- ✓ Logs show progression from error → fix
- ✓ Zero missing events
- ✓ Diff clearly shows regression fixed

#### Validation Checklist:
- [ ] Original recording has 4 actions + logs
- [ ] Playback summary: "4/4 actions executed"
- [ ] Replay logs include success response
- [ ] Log diff status = "fixed"
- [ ] No timeouts or crashes

---

### Integration Test 2: LLM-Generated Variation

```
SCENARIO: Record checkout, LLM generates variation with different coupon, compare results

GIVEN:
  - Recording: "checkout-flow" with action: {type: "type", selector: "input#coupon", text: "WELCOME20"}
  - Original execution successful

WHEN:
  Step 1: LLM reads original actions
    observe({what: 'recording_actions', recording_id: 'checkout-flow'})
    → Returns: [{type: "navigate", ...}, {type: "type", text: "WELCOME20", ...}, ...]

  Step 2: LLM generates variation
    Modified action: {type: "type", text: "SUMMER2026"} (different coupon)

  Step 3: LLM executes variation
    interact({
      action: 'playback',
      actions: [...modified_actions...],
      test_id: 'variation-summer'
    })

  Step 4: Compare results
    observe({what: 'logs', test_id: 'original-checkout'})
    observe({what: 'logs', test_id: 'variation-summer'})

THEN:
  - Variation executed with new coupon code
  - Logs show discount applied: -$5.00 (SUMMER2026)
  - Both flows successful
  - LLM can see: "Both coupons work, SUMMER2026 gives $5 off"
```

#### Success Criteria:
- ✓ Variation executes successfully
- ✓ Different coupon code used
- ✓ Discount amount reflected in logs
- ✓ Both flows complete without errors

---

### Integration Test 3: Selector Fragility Detection

```
SCENARIO: Button moves across 3 playback runs; system detects fragility

GIVEN:
  - Recording: "checkout" with action: {type: "click", selector: "[data-testid=submit]", x: 500, y: 200}
  - Each playback run, button moves slightly

WHEN:
  Run 1: Playback → Button found at x: 500, y: 200
  Run 2: Playback → Button moved to x: 505, y: 210 (self-healing recovers)
  Run 3: Playback → Button moved to x: 512, y: 215 (self-healing recovers)
  Run 4: Playback → Button at x: 520, y: 220 (4th location)

THEN:
  - Runs 1-4 all succeed (self-healing works)
  - After 3 moves: Warning logged: "Fragile selector: element moved 3 times"
  - Screenshot taken with issue type: "moved-selector"
  - LLM notified: "Consider adding data-testid or updating selector for stability"
```

#### Success Criteria:
- ✓ All 4 runs execute successfully
- ✓ Self-healing recovers moved elements
- ✓ Fragility warning triggered after 3 moves
- ✓ Recommendation provided to LLM

---

## UAT Steps (Manual Testing)

**Executor:** QA engineer or developer with Kaboom extension

### Environment Setup:
```bash
# Start server
go run ./cmd/browser-agent/main.go

# Load extension in Chrome
- chrome://extensions → Load unpacked → extension/ directory
- Verify extension icon appears
- Verify "AI Web Pilot" toggle enabled
```

### UAT 1: Record a Real Web Flow

**Objective:** Verify recording captures realistic user interactions

#### Steps:
1. Open https://example.com (or test app)
2. Click extension icon → "Start Recording"
3. Enter name: "real-flow-test"
4. Perform: Click header link → Type email → Click submit → Wait for success message
5. Click "Stop Recording"

#### Expected Result:
- ✓ Extension shows "Recording saved"
- ✓ Recording visible in extension popup (with timestamp, action count)
- ✓ Recording metadata file exists: `~/.kaboom/recordings/real-flow-test-*/metadata.json`
- ✓ All 4 actions captured: click, type, click, wait
- ✓ Screenshot files created in screenshots/ subdirectory
- ✓ Timestamps make sense (ascending, reasonable intervals)

#### Validation:
```bash
# Check recording exists
ls -lh ~/.kaboom/recordings/real-flow-test-*/metadata.json

# Inspect metadata (verify JSON is valid)
jq . ~/.kaboom/recordings/real-flow-test-*/metadata.json
```

---

### UAT 2: Playback Recorded Flow

**Objective:** Verify playback executes recorded actions

#### Steps:
1. Have recording from UAT 1
2. Open browser to fresh session of <https://example.com>
3. In Claude Code or console:
   ```javascript
   interact({
     action: 'playback',
     recording: 'real-flow-test-...',
     test_id: 'replay-test'
   })
   ```
4. Watch playback execute (should take < 10 seconds)

#### Expected Result:
- ✓ Playback executes all 4 actions in order
- ✓ Playback completes successfully
- ✓ Response: `{status: "ok", actions_executed: 4, errors: []}`
- ✓ Screenshot taken (if error occurs)
- ✓ Duration reasonable (< 10 seconds)

#### Validation:
```bash
# Check replay logs captured
observe({what: 'logs', test_id: 'replay-test'})
# Should return array of logs during playback
```

---

### UAT 3: Compare Original vs Replay Logs

**Objective:** Verify log diffing shows differences (or match)

#### Steps:
1. Have recording from UAT 1 + replay from UAT 2
2. Fetch original logs:
   ```javascript
   observe({what: 'logs', test_id: 'original-real-flow-test'})
   ```
3. Fetch replay logs:
   ```javascript
   observe({what: 'logs', test_id: 'replay-test'})
   ```
4. Analyze: Do they match? Any new errors? Missing events?

#### Expected Result (Best Case):
- ✓ Both logs nearly identical
- ✓ Same API calls, same success/error messages
- ✓ Timestamps differ (slightly) but order preserved
- ✓ No regressions detected

#### Expected Result (If Bug Fix Worked):
- ✓ Original logs: POST /api/submit → 500 Error
- ✓ Replay logs: POST /api/submit → 200 OK
- ✓ Log diff: "Regression FIXED ✓"

---

### UAT 4: Sensitive Data Toggle

**Objective:** Verify credential recording is optional and safe by default

#### Steps:
1. Start recording with sensitive_data_enabled: false (default)
2. Click password field
3. Type: "my_secret_password"
4. Stop recording
5. Inspect metadata.json

#### Expected Result:
- ✓ Metadata contains: `{type: "type", text: "[redacted]", ...}`
- ✓ Original password never stored
- ✓ Metadata flag: `sensitive_data_enabled: false`

#### Steps (Opt-In):
1. Start recording with sensitive_data_enabled: true
2. Extension shows warning: "You are recording credentials..."
3. User clicks: "I confirm this is test data"
4. Type password: "test_password_123"
5. Stop recording
6. Inspect metadata.json

#### Expected Result:
- ✓ Metadata contains: `{type: "type", text: "test_password_123", ...}`
- ✓ Full password captured
- ✓ Metadata flag: `sensitive_data_enabled: true`
- ✓ File location: `~/.kaboom/recordings/`

---

### UAT 5: Storage Quota Enforcement

**Objective:** Verify 1GB limit is enforced

#### Steps (Prep):
```bash
# Create dummy recordings to reach 800MB
cd ~/.kaboom/recordings
dd if=/dev/zero of=dummy1.bin bs=1M count=800
cd -
```

1. Try to start new recording
2. Extension should show warning: "Recording storage at 80%"
3. Recording proceeds (non-blocking)

## Steps (Quota Full):
```bash
# Create 200MB more to hit 1GB limit
dd if=/dev/zero of=dummy2.bin bs=1M count=200
```

1. Try to start new recording
2. Expect error: "Recording storage at capacity (1GB). Delete old recordings..."
3. Attempt fails

## Expected Result:
- ✓ Warning at 80% (continues)
- ✓ Error at 100% (blocks)
- ✓ User must manually delete to proceed
- ✓ Next attempt after cleanup succeeds

---

### UAT 6: WebSocket Connection (Developer View)

**Objective:** Verify WebSocket is active and efficient

#### Steps:
1. Open Chrome DevTools → Network tab
2. Filter to "WS" (WebSocket)
3. Verify: `ws://localhost:3001/api/stream` connected
4. Start recording
5. Perform 10 actions (click, type, click, etc.)
6. Watch Network tab

#### Expected Result:
- ✓ WebSocket shows single persistent connection (no polls)
- ✓ Events arrive within 100ms of occurrence
- ✓ No "pending" queries in background
- ✓ Extension shows: "Connected via WebSocket"

#### Fallback Test:
1. In Chrome DevTools, simulate network throttling
2. Simulate offline (DevTools → Network → Offline)
3. Wait 5 seconds
4. Go back online

#### Expected Result:
- ✓ Extension detects loss, switches to polling
- ✓ Popup shows: "Polling (WebSocket unavailable)"
- ✓ Continues capturing (non-blocking)
- ✓ After reconnect, back to WebSocket

---

### UAT 7: End-to-End Regression Testing Workflow

**Objective:** Full workflow from bug report to fix verification

#### Scenario:
```
1. QA reports: "Checkout button broken in Safari, works in Chrome"
2. Engineer wants to reproduce and verify fix
3. Kaboom captures reproduction
4. Engineer fixes bug
5. Engineer re-runs to verify fix
```

#### Steps:

#### Phase 1: Reproduce Bug
```
1. Open test shopping app
2. configure({what: 'test_boundary_start', test_id: 'bug-report'})
3. Click "Add to Cart" → Select product → Click "Checkout"
4. Observe error: "400 Bad Request - Invalid session"
5. configure({what: 'test_boundary_end', test_id: 'bug-report'})
```

#### Phase 2: Engineer Investigates
```
1. observe({what: 'logs', test_id: 'bug-report'})
2. LLM sees: "POST /api/checkout → 400, message: 'Invalid session'"
3. Engineer: "Ah, session cookie not passed in checkout request"
4. Engineer: Fix: Add cookie to checkout POST
5. Engineer: Redeploy app
```

#### Phase 3: Verify Fix
```
1. interact({
     action: 'playback',
     recording: 'shopping-flow-20260130T...',
     test_id: 'fix-verification'
   })
2. observe({what: 'logs', test_id: 'fix-verification'})
3. LLM: "POST /api/checkout → 200 OK, checkout successful ✓"
4. LLM: "Regression FIXED"
```

#### Expected Result:
- ✓ Original logs show 400 error
- ✓ Replay logs show 200 success
- ✓ LLM can clearly trace: "Session cookie was missing → now included"
- ✓ Fix verified in < 2 minutes (vs. 30 min manual QA)

---

## Test Summary

| Module | Unit Tests | Lines | Pass Rate | Coverage |
|--------|-----------|-------|-----------|----------|
| WebSocket | 5 test cases | ~150 LOC | ✓ | 95%+ |
| Recording Storage | 9 test cases | ~250 LOC | ✓ | 90%+ |
| Playback | 9 test cases | ~280 LOC | ✓ | 92%+ |
| Log Diffing | 7 test cases | ~200 LOC | ✓ | 90%+ |
| Extension | 7 test cases | ~180 LOC | ✓ | 88%+ |
| **Integration** | 3 scenarios | ~150 LOC | ✓ | 85%+ |
| **UAT** | 7 workflows | Manual | ✓ | Complete |
| **TOTAL** | 47 test cases | ~1,210 LOC | **Target: 100%** | **≥ 90%** |

---

## Success Criteria (Gate 4)

✅ **Test Completeness:**
- [ ] All 47 unit test cases defined with inputs/outputs
- [ ] 3 integration test scenarios cover end-to-end workflows
- [ ] 7 UAT steps cover manual testing
- [ ] Tests are executable without implementation knowledge

✅ **Test Independence:**
- [ ] Each test case is standalone (no dependencies on other tests)
- [ ] Mock objects defined for external dependencies
- [ ] Cleanup/teardown procedures specified

✅ **Coverage Targets:**
- [ ] Recording: ≥ 90% code coverage
- [ ] Playback: ≥ 92% code coverage
- [ ] WebSocket: ≥ 95% code coverage
- [ ] Log Diffing: ≥ 90% code coverage
- [ ] Extension: ≥ 88% code coverage
- [ ] **Overall: ≥ 90%**

✅ **Feasibility:**
- [ ] QA engineer can execute without asking "how do we...?"
- [ ] Test setup/teardown is clear
- [ ] Expected results are specific and measurable
- [ ] Mock objects are documented

---

## Next Phase: Implementation (Gate 5)

Once this QA_PLAN is approved:

1. **Write Failing Tests FIRST**
   ```bash
   make test → Tests fail ✗
   ```

2. **Implement Code to Make Tests Pass**
   - Week 1: WebSocket + Recording storage
   - Week 2: Playback + Self-healing
   - Week 3: Log Diffing
   - Week 4: Integration & UAT

3. **All Tests Pass**
   ```bash
   make test → All tests pass ✓
   make ci-local → Full CI passes ✓
   ```

4. **Commit**
   ```bash
   git commit -m "feat: Implement Flow Recording & Playback MVP"
   ```

---

**QA Plan Status:** READY FOR APPROVAL

Are you satisfied with the test plan? Once approved, implementation begins (TDD).

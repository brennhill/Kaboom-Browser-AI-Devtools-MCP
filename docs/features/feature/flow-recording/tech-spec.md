---
feature: Flow Recording & Playback
status: in-progress
version: 0.7.12
doc_type: tech-spec
feature_id: feature-flow-recording
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Flow Recording & Playback (Regression Testing)

## Overview

Flow Recording & Playback enables developers to regression test by recording a user flow once, replaying it after fixing a bug, and automatically comparing logs to detect what changed.

### Core MVP Components:
1. **Extension Recording** (TypeScript/JS) — Capture user interactions + browser state
2. **Server Storage** (Go) — Store recordings + logs with test boundary tagging
3. **Playback Engine** (Go + Extension) — Replay actions with self-healing selectors
4. **Log Diffing** (Go) — Compare original vs replay logs, detect regressions
5. **LLM Integration** (MCP) — Feed diffs to Claude for analysis

---

## Architecture

### Recording Flow

```
User Interacts                Extension Captures            Server Stores
─────────────────            ─────────────────            ──────────────

[Click button]      →        POST /query {
                               type: 'click',
                               selector: '.btn',
                               x: 500, y: 200,
                               timestamp: 1234567890
                             }
                                     ↓
                             Buffer entry:
                             - Add to memory ring buffer
                             - Tag with test_id
                             - Flush to disk periodically

[Type text]         →        POST /query {
                               type: 'type',
                               selector: 'input#name',
                               text: 'Alice',
                               timestamp: 1234567900
                             }
                                     ↓
                             Buffer entry + screenshot

[Navigate]          →        POST /query {
                               type: 'navigate',
                               url: 'https://...',
                               timestamp: 1234567950
                             }
```

### Playback & Log Diffing Flow

```
LLM Invokes Playback         Playback Executes            Log Diffing
────────────────────        ─────────────────            ────────────

interact({                   start_boundary('replay-X')
  action: 'playback',            ↓
  recording: 'checkout-X',   replay action sequence:
  test_id: 'replay-X'            ├─ navigate (wait 5s)
})                               ├─ click (find selector)
                                 ├─ type (verify input)
                                 └─ capture logs + network
                                       ↓
                                 end_boundary('replay-X')
                                       ↓
                            observe({what: 'logs',
                             test_id: 'original-X'}) → logs A
                            observe({what: 'logs',
                             test_id: 'replay-X'}) → logs B
                                       ↓
                            Diff logs A vs B:
                            ├─ New errors in B?
                            ├─ Missing events in B?
                            ├─ Different responses?
                            └─ Return structured diff
                                       ↓
                            LLM analyzes diff:
                            "Regression: 404 on /api/order"
                            "Likely cause: endpoint renamed"
                            "Suggested fix: use /api/orders"
```

---

## Data Structures

### Recording Format (Metadata)

**File:** `~/.kaboom/recordings/{recording-id}/metadata.json`

```json
{
  "id": "shopping-checkout-20260130T143022Z",
  "name": "shopping-checkout",
  "created_at": "2026-01-30T14:30:22Z",
  "duration_ms": 45000,
  "action_count": 8,
  "start_url": "https://example.com/shop",
  "viewport": {"width": 1920, "height": 1080},
  "sensitive_data_enabled": false,
  "actions": [
    {
      "type": "navigate",
      "timestamp_ms": 0,
      "url": "https://example.com/shop"
    },
    {
      "type": "click",
      "timestamp_ms": 1200,
      "selector": "[data-testid=product-1]",
      "data_testid": "product-1",
      "x": 500,
      "y": 300,
      "screenshot_path": "screenshots/001-click.jpg"
    },
    {
      "type": "type",
      "timestamp_ms": 3400,
      "selector": "input#quantity",
      "text": "3",
      "x": 523,
      "y": 456,
      "screenshot_path": "screenshots/002-type.jpg"
    },
    {
      "type": "type",
      "timestamp_ms": 5200,
      "selector": "input#password",
      "text": "[redacted]",
      "x": 520,
      "y": 500,
      "screenshot_path": "screenshots/003-type.jpg"
    }
  ]
}
```

### Action Type

```go
type RecordingAction struct {
    Type              string    // "navigate", "click", "type"
    Timestamp         int64     // Unix milliseconds
    URL               string    // for navigate actions
    Selector          string    // CSS selector
    DataTestID        string    // data-testid value
    Text              string    // for type actions; "[redacted]" if sensitive_data_enabled=false
    X                 int       // x coordinate
    Y                 int       // y coordinate
    ScreenshotPath    string    // relative path to screenshot
}

// Sensitive Data Handling:
// - If sensitive_data_enabled=false (default): typed text recorded as "[redacted]" in JSON
// - If sensitive_data_enabled=true (opt-in): full typed text recorded (e.g., "test123")
// - metadata.json includes flag: "sensitive_data_enabled": true|false for audit
```

### Log Diff Result

```go
type LogDiff struct {
    Status         string       // "match", "regression", "fixed", "unknown"
    NewErrors      []LogEntry   // errors in replay not in original
    MissingEvents  []LogEntry   // events in original not in replay
    ChangedValues  []ValueDiff  // different responses/messages
    Summary        string       // human-readable diff summary
}

type LogEntry struct {
    Type       string    // "network", "console", "dom"
    Timestamp  int64     // Unix ms
    Message    string    // error message or event
    URL        string    // for network errors
    StatusCode int       // for network errors
    Stack      string    // for errors
}

type ValueDiff struct {
    Event       string // event name
    Original    string // original value
    Replay      string // replay value
    Timestamp   int64
}
```

---

## MCP API Design

### Recording Actions

#### Start Recording:
```javascript
configure({
  action: 'recording_start',
  name?: 'shopping-checkout',
  url?: 'https://example.com/shop',
  sensitive_data_enabled?: false
})
```

Response: `{status: "ok", recording_id: "shopping-checkout-20260130T..."}`

#### Stop Recording:
```javascript
configure({
  action: 'recording_stop',
  recording_id?: 'shopping-checkout-20260130T...'
})
```

Response: `{status: "ok", action_count: 8, duration_ms: 45000}`

### Recording Queries

#### List Recordings:
```javascript
observe({
  what: 'recordings',
  limit?: 10
})
```

Response:
```json
{
  "recordings": [
    {
      "id": "shopping-checkout-20260130T143022Z",
      "name": "shopping-checkout",
      "created_at": "2026-01-30T14:30:22Z",
      "action_count": 8,
      "url": "https://example.com/shop"
    }
  ]
}
```

#### Get Recording Actions:
```javascript
observe({
  what: 'recording_actions',
  recording_id: 'shopping-checkout-20260130T143022Z'
})
```

Response: `{recording_id: "...", actions: [...]}`

### Playback

#### Execute Playback (Recorded Flow):
```javascript
interact({
  action: 'playback',
  recording: 'shopping-checkout-20260130T143022Z',  // Load from saved recording
  test_id?: 'replay-shopping-checkout',
  sequence_mode?: true,
  timeout_ms?: 30000
})
```

#### Execute Playback (LLM-Generated Variation):
```javascript
interact({
  action: 'playback',
  actions: [  // Custom action array (LLM-generated)
    {type: "navigate", url: "https://example.com/shop", timestamp_ms: 0},
    {type: "click", selector: "[data-testid=product-2]", x: 500, y: 300, timestamp_ms: 1000},
    {type: "type", selector: "input#coupon", text: "SUMMER2026", x: 520, y: 450, timestamp_ms: 3000}
  ],
  test_id?: 'variation-summer-coupon',
  sequence_mode?: true,
  timeout_ms?: 30000
})
```

**Note:** Either `recording` (string ID) or `actions` (array) required; not both.

Response:
```json
{
  "status": "ok",
  "recording_id": "shopping-checkout-20260130T143022Z",
  "test_id": "replay-shopping-checkout",
  "actions_executed": 8,
  "errors": [
    {
      "action_index": 3,
      "type": "selector_not_found",
      "selector": "[data-testid=product-1]",
      "message": "Element not found after self-healing"
    }
  ],
  "duration_ms": 12000
}
```

### Test Generation (LLM-Generated Variations)

#### LLM Can Generate Action Variations:

Playback accepts both recorded AND LLM-generated action sequences. LLM can synthesize variations by modifying the action array JSON:

```javascript
// Original recorded actions
observe({what: 'recording_actions', recording_id: 'checkout-flow'})
// Returns: {recording_id: "checkout-flow", actions: [...]}

// LLM generates variation (e.g., different coupon code)
// LLM modifies actions array and invokes playback with custom actions:
interact({
  action: 'playback',
  actions: [  // Instead of 'recording', pass custom action array
    {type: "navigate", url: "https://example.com/shop", timestamp_ms: 0},
    {type: "click", selector: "[data-testid=product-2]", x: 500, y: 300, timestamp_ms: 1000},
    {type: "type", selector: "input#coupon", text: "SUMMER2026", x: 520, y: 450, timestamp_ms: 3000},
    {type: "click", selector: "[data-testid=checkout-btn]", x: 600, y: 500, timestamp_ms: 5000}
  ],
  test_id: 'variation-coupon-summer2026'
})
```

#### Use Cases:
- Different inputs: Try coupon "SUMMER2026" instead of "WELCOME20"
- Different selectors: If selector moved, test with new coordinates
- Different flows: Remove intermediate steps, add new ones
- Different user states: Simulate logged-in vs guest checkout

#### Implementation:
- Playback function accepts either `recording: string` (ID) OR `actions: array` (custom)
- Response identical: `{status: "ok", actions_executed, errors, duration}`
- Logs tagged same way (test_boundary_id captures all events)
- LLM can generate, playback, and compare results

#### MVP Scope:
- [x] Playback accepts custom action arrays
- [x] No special "test generation" feature needed (LLM generates JSON directly)
- [x] Variations logged and compared like recorded flows

### Log Diffing & Regression Detection

#### Compare Logs:
```javascript
// LLM reads both sets of logs via test boundaries
observe({what: 'logs', test_boundary: 'original-checkout'})
observe({what: 'logs', test_boundary: 'replay-checkout'})
observe({what: 'logs', test_boundary: 'variation-coupon-summer2026'})

// LLM internally diffs and analyzes
// Kaboom provides structured diff via logs themselves
```

---

## Implementation Details

### Extension Recording (JavaScript)

**Files:** `extension/background/recording.js` (new), modify `extension/inject/recording.ts` (new)

#### Server-Side Storage (`extension/background/recording.js`):
- Manage recording lifecycle (start/stop)
- Buffer actions in memory
- Persist recording to server via POST /query
- Metadata management (name, timestamp, duration)
- Size target: ~150 LOC

#### Page-Side Capture (`extension/inject/recording.ts`):
- **REUSE existing infrastructure:** inject.js already has `installActionCapture`, `recordEnhancedAction`, `handleClick`, `handleInput`, `handleKeydown`
- Extend existing action capture to include recording-specific metadata (selector, x/y, screenshot_path)
- Send actions to background via postMessage (existing pattern)
- Size target: ~150 LOC (mostly integration with existing capture)

#### Screenshot Capture:
- Reuse existing screenshot functionality (used by error detection)
- Capture on: page load, selector failure, error
- Store locally at `~/.kaboom/recordings/{recording_id}/screenshots/`

#### Why Minimal New Code:
- Extension already captures click/input/keydown/navigation via `inject/action-capture.ts`
- We extend that to add recording metadata (selector, x/y, screenshots)
- Existing infrastructure: message passing, storage, screenshot, selector extraction
- Recording = filtering + tagging existing action stream

### Server Recording Storage (Go)

**File:** `cmd/browser-agent/recording.go` (new)

#### Responsibilities:
- In-memory recording buffer (ring, max 100 recordings)
- Persist recordings to disk (~/.kaboom/recordings/)
- Query recordings by ID/name
- Load recordings from disk on startup
- Cleanup old recordings (manual, not auto)

**Size Target:** ~200 LOC

#### Data:
- `Recording` struct with actions array
- `RecordingBuffer` with ring buffer + mutex
- File I/O for metadata.json

### Playback Engine (Go)

**File:** `cmd/browser-agent/playback.go` (new)

#### Responsibilities:
- Load recording by ID
- Execute actions in sequence (fast-forward)
- Find elements using self-healing strategy (R4)
- Capture logs/network during playback
- Tag all captured events with test_boundary_id
- Return playback result with error list

**Size Target:** ~250 LOC

#### Algorithm:
```
1. Load recording
2. For each action:
   a. Navigate: wait for page to load (5s timeout)
      - Wait for 0 active HTTP/XHR requests OR timeout
      - Non-blocking: if timeout reached, log warning and continue
   b. Click: find element via self-healing, scroll if needed, click
   c. Type: find element, verify is input, type text
   d. Take screenshot on error
3. Capture all logs under test boundary
4. Return {actions_executed, errors, duration}
```

#### Network Idle Definition (Fast-Forward Playback):
- "Network idle" = 0 active HTTP/XHR requests
- Polling interval: check every 100ms (non-blocking)
- Hard timeout: 5 seconds
- If timeout: log warning "Navigation timeout (5s)" and continue playback
- Rationale: In sequence mode (fast-forward), we execute actions rapidly and wait for prior network to settle before next action (not for user think-time)

### WebSocket Migration (Go + TypeScript) — Phase 1

**Files:** `cmd/browser-agent/websocket.go` (new), `extension/src/ws.ts` (new)

**Problem:** Current polling-based recording (extension polls GET /pending-queries every 200ms) introduces timing inaccuracy (~100-200ms jitter). Recording needs millisecond precision for accurate playback.

**Solution:** WebSocket streaming for real-time event propagation.

#### Architecture:
```
Extension                    Server
─────────────────           ──────────
POST /api/ws-connect        → Check API key + session
  ↓
WebSocket upgrade           ← Upgrade to WS:// (ws://localhost:3001/api/stream)
  ↓
Connect & subscribe         → Join broadcast channel (global stream)
  ↓
Real-time events           ← Server sends logs/network/WebSocket/actions as they occur
 (< 1ms latency)             (push model, not polling)
  ↓
Buffer overflow?           → Server: drop oldest, log warning to extension
                            Extension: show warning icon in popup
  ↓
Connection drop?           → Fall back to polling (graceful degradation)
```

#### Implementation Details:
- Server: WebSocket upgrade handler (ws package for Go)
- Server: Broadcast buffer with ring (max 10,000 events)
- Server: Drop oldest on overflow, log warning
- Extension: Establish WS connection on startup
- Extension: Listen to stream, update ring buffers
- Extension: Fall back to polling if WS unavailable (5s timeout)
- Extension: Reconnect logic (exponential backoff on failure)

**Size Target:** ~300 LOC total (150 Go, 150 TypeScript)

#### Testing:
- Test normal WS flow (events received in real-time)
- Test buffer overflow (oldest events dropped)
- Test connection drop + polling fallback
- Test reconnection after network outage

#### Success Criteria:
- Recording timestamps accurate to < 10ms
- No gaps in event stream during recording
- Graceful fallback to polling if WS unavailable
- Buffer overflow handled without data loss (oldest dropped, not newest)

### Element Selector Self-Healing (Go)

**File:** `cmd/browser-agent/playback.go` (in Playback section)

#### Strategy (Priority Order):
1. Try data-testid match: `querySelector('[data-testid=X]')`
2. Try CSS selector: `querySelector(original_selector)`
3. Search nearby: Look for element near old x/y coordinates
4. OCR recovery: (Phase 2) Use screenshot OCR to find by visible text
5. Fallback: Use last-known x/y with warning

**Size Target:** ~150 LOC

### Log Diffing Engine (Go)

**File:** `cmd/browser-agent/log-diff.go` (new)

#### Responsibilities:
- Query logs from both test boundaries
- Compare error counts
- Detect new errors (in replay, not in original)
- Detect missing events (in original, not in replay)
- Detect value changes (response different)
- Return structured diff

**Size Target:** ~150 LOC

#### Algorithm:
```
1. Fetch logs from test_boundary A (original)
2. Fetch logs from test_boundary B (replay)
3. Categorize all entries: network, console, dom
4. For each error in B:
   - Is it in A? (expected)
   - New in B? (regression)
5. For each event in A:
   - Is it in B? (expected)
   - Missing in B? (broken feature)
6. Compare values (response codes, messages)
7. Return LogDiff with summary
```

### Test Boundary Tagging (Go)

**File:** `cmd/browser-agent/types.go` (modify existing)

Already implemented in prior work:
- `Capture.activeTestIDs map[string]bool`
- `NetworkBody.TestIDs []string`
- `WebSocketEvent.TestIDs []string`
- `EnhancedAction.TestIDs []string`
- Filter structs with `TestID` field

**No changes needed** — reuse existing test boundary infrastructure.

---

## Screenshots & Artifacts

### Naming Convention:
```
~/.kaboom/recordings/{recording-id}/screenshots/
├── {date}-{recording-id}-{action-index}-{issue-type}.jpg
├── 20260130-shopping-checkout-001-page-load.jpg
├── 20260130-shopping-checkout-003-moved-selector.jpg
├── 20260130-shopping-checkout-005-error.jpg
```

### Issue Types:
- `page-load` — after navigation, page fully loaded
- `moved-selector` — element selector failed, had to use fallback
- `error` — error/timeout occurred, visual evidence

**Compression:** JPEG 85%, max 500KB per image

---

## Performance Targets (MVP)

| Component | Target | Notes |
|-----------|--------|-------|
| Extension recording overhead | < 5% CPU | Non-intrusive |
| Playback speed | 10+ actions/sec | Fast-forward mode (sequence mode) |
| Screenshot capture | < 100ms per image | Async, non-blocking |
| Log diff query | < 500ms | Typical: 100-500 log entries |
| Recording storage | 1GB hard limit | Manual cleanup, enforced |

**Note:** Performance targets are aspirational for Phase 1. Actual measurement and optimization deferred if needed.

---

## Error Handling

### Recording Errors

#### Screenshot Compression:
- Target: JPEG 85% compression, typically 50-200KB per screenshot
- If > 500KB: Log debug note, keep screenshot (500KB is acceptable)
- Rationale: Typical page screenshots are well under 500KB; if exceeds, it's informational but not a blocker

#### Storage Quota Enforcement (1GB Hard Limit):
- At 80% capacity (800MB): Log warning to user: "Recording storage at 80%. Consider deleting old recordings."
- At 100% capacity (1GB): `recording_start` returns error: "Recording storage at capacity (1GB). Delete old recordings to continue."
- User must manually delete old recordings: `rm ~/.kaboom/recordings/{recording_id}`
- Next `recording_start` call still fails if capacity not freed
- Design: No auto-delete (data loss risk per PRODUCT_SPEC)

Implementation in server:
```go
// Check storage quota before starting new recording
func (c *Capture) RecordingStart(name, url string) error {
  totalSize := calculateRecordingStorageUsage() // sum all ~/.kaboom/recordings/

  if totalSize >= 1.0 * GB {
    return fmt.Errorf("recording_storage_full: Recording storage at capacity (1GB). Delete old recordings via file system to continue.")
  }
  if totalSize >= 0.8 * GB {
    log.Warn("recording_storage_warning: Recording storage at 80% capacity")
  }

  // Create new recording...
  return nil
}
```

#### Selector Fragility:
- Selector unstable (moved 3+ times) → log warning, take screenshot with issue type `moved-selector`
- Recommend to LLM: "Add data-testid=X to improve test stability"

### Playback Errors (Non-Blocking)
- Element not found → use self-healing, log error, continue
- Navigation timeout (5s) → log error, continue
- Type in non-input → log error, continue
- Click outside viewport → scroll then click

**Design:** Playback MUST complete even if some actions fail. LLM needs full log diff to analyze.

---

## Security & Privacy

### Recording Data:
- Stored locally at `~/.kaboom/recordings/` only
- Not transmitted to cloud

### Sensitive Data Toggle (Credential Recording):

When `sensitive_data_enabled=false` (default - SAFE):
- All typed text in recordings recorded as `[redacted]` in JSON
- Metadata flag: `sensitive_data_enabled: false`
- Use case: Normal testing without credentials
- Example: `{type: "type", text: "[redacted]"}`

When `sensitive_data_enabled=true` (explicit opt-in - FOR LOGIN TESTING):
- Full typed text recorded (e.g., "test123")
- ⚠️ Warning popup: "You are recording all text. Ensure credentials are TEST DATA only, not production. Recordings stored on localhost only."
- Metadata flag: `sensitive_data_enabled: true`
- Use case: Testing login flows with test account credentials on local development machine
- Example: `{type: "type", text: "test_password_123"}`

Implementation in extension:
```javascript
// When recording_start called with sensitive_data_enabled=true:
1. Show warning popup
2. Require user to acknowledge: "I confirm this is test data, not production credentials"
3. Only then proceed with full text recording
4. Mark metadata.json with sensitive_data_enabled=true
```

### Selector Recommendations:
- Log fragile selectors (moved 3+ times)
- Suggest using `data-testid` instead
- Help LLM improve test robustness

---

## Test Boundary Integration

### Orthogonal Concerns (But Coordinated):

1. **Recording** captures action metadata (click, type, navigate, selector, x/y, screenshot)
   - Stored to disk at `~/.kaboom/recordings/{recording_id}/metadata.json`
   - Independent of test boundaries

2. **Test Boundaries** tag LOGS, NETWORK, WEBSOCKET, ACTIONS with test context
   - Used to group related events for analysis
   - Enable filtering: "show me all logs from test X"

3. **How They Work Together:**
   - User starts `test_boundary_start('original-checkout')`
   - User performs flow (extension captures actions via recording)
   - Events (logs, network, WebSocket) tagged with `test_id: 'original-checkout'`
   - User stops `test_boundary_end('original-checkout')`
   - Recording saved to disk with all captured actions
   - Later: replay invokes `test_id: 'replay-checkout'` → captures new logs under that boundary
   - Compare: fetch logs for 'original-checkout' vs 'replay-checkout'

### Example Flow:
```javascript
// Option A: Record + playback within test boundaries (typical)
configure({what: 'test_boundary_start', test_id: 'original-checkout'})
// user performs flow, extension captures actions + logs
configure({what: 'test_boundary_end', test_id: 'original-checkout'})

// Later: replay
interact({
  action: 'playback',
  recording: 'shopping-checkout-20260130T...',
  test_id: 'replay-checkout'
})

// Compare
observe({what: 'logs', test_id: 'original-checkout'})  // original logs
observe({what: 'logs', test_id: 'replay-checkout'})    // replay logs

// Option B: Record without explicit test boundary
configure({what: 'recording_start', name: 'checkout-flow'})
// user performs flow, extension captures (no test_id tagging)
configure({what: 'recording_stop', recording_id: '...'})

// Later: replay with explicit test boundary
configure({what: 'test_boundary_start', test_id: 'replay-1'})
interact({what: 'playback', recording: 'checkout-flow', test_id: 'replay-1'})
configure({what: 'test_boundary_end', test_id: 'replay-1'})
```

**Key Design:** Recording captures actions; test boundaries tag everything else. They're independent but compose naturally.

---

## Files to Create/Modify

### New Files

| File | Purpose | Size |
|------|---------|------|
| `extension/background/recording.js` | Recording lifecycle + metadata + server communication | ~150 LOC |
| `extension/inject/recording.ts` | Extend action capture with recording metadata | ~150 LOC |
| `extension/background/ws.js` | WebSocket client + fallback to polling | ~150 LOC |
| `cmd/browser-agent/recording.go` | Store + query recordings (disk I/O, file management) | ~250 LOC |
| `cmd/browser-agent/websocket.go` | WebSocket upgrade + broadcast + buffer management | ~150 LOC |
| `cmd/browser-agent/playback.go` | Replay actions + self-healing selectors | ~250 LOC |
| `cmd/browser-agent/log-diff.go` | Compare logs, detect regressions | ~180 LOC |
| `cmd/browser-agent/recording_test.go` | Unit tests (storage, playback, diffing) | ~450 LOC |
| `tests/extension/recording-lifecycle/recording.test.js` | Extension tests (capture, playback, metadata) | ~300 LOC |

### Modified Files

| File | Changes | Impact |
|------|---------|--------|
| `cmd/browser-agent/tools_core.go` | Add recording_start, recording_stop, playback actions | ~50 LOC |
| `cmd/browser-agent/queries.go` | Add handlers for recordings, recording_actions, WebSocket upgrade | ~60 LOC |
| `cmd/browser-agent/main.go` | Initialize WebSocket router | ~20 LOC |
| `extension/inject/action-capture.ts` | Extend with recording metadata (selector, x/y, screenshots) | ~30 LOC |
| `extension/background/index.js` | Initialize recording module | ~15 LOC |

**Total New Code:** ~1,930 LOC (reuses existing action capture infrastructure)

---

## Phase 1 Success Criteria

✅ **Functional:**
- LLM can start/stop recordings via extension or MCP
- All interactions (click, type, navigate) recorded with selectors + x/y
- Screenshots captured at key points (page load, error, sampling)
- Recordings queryable via MCP
- Playback replays actions in sequence
- Self-healing selectors work (data-testid → CSS → x/y)
- Moved elements logged with screenshots
- Log diffing detects new errors + missing events
- Regression alerts shown to LLM

✅ **Quality:**
- Recording overhead < 5% CPU
- Playback 10+ actions/sec
- All tests pass (recording, playback, selector matching, log diffing)
- Zero regressions (existing tests still pass)
- Screenshots < 500KB
- Error handling non-blocking (playback completes)

✅ **Performance:**
- Log diff query < 500ms
- Recording storage 1GB limit enforced
- No cloud dependencies

---

## Phase 2 (Deferred)

- Git integration (find commits touching affected files)
- Visual element recovery (OCR on screenshots)
- Test coverage analytics (which code paths tested)
- Performance regression detection (timing changes)

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Selector fragility (UI changes break tests) | Self-healing strategy + data-testid recommendation |
| Long recordings (100+ actions) | Guidance on typical flow size, manual cleanup |
| Log comparison complexity | Structured diff, error categorization |
| Extension overhead | < 5% CPU target, async screenshots |
| Storage explosion | 1GB hard limit, manual user management |

---

## Dependencies & Integration

**Zero External Dependencies** (production runtime)

### Existing Kaboom Infrastructure Used:
- Test boundaries (activeTestIDs, TestIDs tagging)
- Ring buffers (logs, network, WebSocket, actions)
- MCP tools (observe, interact, configure)
- Extension message passing
- Screenshot functionality

### New MCP Extensions:
- `configure` action: `recording_start`, `recording_stop`
- `interact` action: `playback`
- `observe` what: `recordings`, `recording_actions`

---

## Reference Implementation Order

1. **Phase 1a** (Week 1): WebSocket Migration + Recording Infrastructure
   - Implement `websocket.go` (server-side WebSocket upgrade + broadcast)
   - Implement `ws.js` (extension client + polling fallback)
   - Extend `action-capture.ts` with recording metadata (selector, x/y, screenshot_path)
   - Implement `recording.js` (extension recording lifecycle: start/stop)
   - Implement `recording.go` (server storage: disk I/O, file management)
   - Add MCP actions (recording_start/stop, recordings, recording_actions)
   - Tests for WebSocket, action capture extension, storage layer
   - **Note:** Reuses existing action capture infrastructure; minimal new code for capture logic

2. **Phase 1b** (Week 2): Playback + Self-Healing + Test Generation
   - Implement `playback.go` (action replay, both recorded + LLM-generated)
   - Implement selector self-healing logic
   - Add playback MCP action (with `recording` or `actions` param support)
   - Tests for playback, selector matching, LLM-generated variations

3. **Phase 1c** (Week 3): Log Diffing + Regression Detection
   - Implement `log-diff.go` (comparison)
   - Hook into test boundary logs
   - Regression detection (new errors, missing events, value changes)
   - Tests for log diffing

4. **Integration & UAT** (Week 4)
   - End-to-end test: record → replay → diff → analyze
   - WebSocket + polling fallback testing
   - Performance testing (< 5% CPU, 10+ actions/sec)
   - Manual UAT with LLM (real regression workflow)

---

## Next Steps

1. **Principal Review** — Validate technical approach against product spec
2. **QA Plan** — Define test cases (unit + integration + UAT)
3. **Implementation** — TDD, write failing tests first
4. **Release** — Tag as v6.0, deploy with existing Kaboom


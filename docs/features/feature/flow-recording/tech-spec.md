---
feature: Flow Recording & Playback
status: in-progress
version: 0.7.12
doc_type: tech-spec
feature_id: feature-flow-recording
last_reviewed: 2026-11-23
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

> Target design. Today the extension does not forward actions into flow recordings (see Implementation Details).

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

configure({                  start_boundary('replay-X')
  what: 'playback',              ↓
  recording_id: 'checkout-X' replay action sequence:
})                               ├─ navigate (wait 5s)
                                 ├─ click (find selector)
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

> Shipped surface (verified 2026-11-21). Recording lifecycle, playback and log diff are `configure` modes; queries are `observe` modes. Handlers: `internal/tools/recording/handler.go`, registered in `internal/composition/toolruntime/tools_configure.go` and `internal/tools/observe/handler/dispatcher.go`.

### Recording Actions

#### Start Recording:
```javascript
configure({
  what: 'event_recording_start',
  name?: 'shopping-checkout',
  url?: 'https://example.com/shop',   // defaults to about:blank
  sensitive_data_enabled?: false
})
```

Response includes `recording_id` (format `name-YYYYMMDDTHHMMSSZ`). Starting while a recording is active returns an "already recording" error.

#### Stop Recording:
```javascript
configure({
  what: 'event_recording_stop',
  recording_id?: 'shopping-checkout-20260130T...'  // defaults to the active recording
})
```

Response includes `action_count` and `duration_ms`.

### Recording Queries

#### List Recordings:
```javascript
observe({
  what: 'recordings',
  limit?: 10
})
```

Response: recording summaries (`id`, `name`, `created_at`, `start_url`, `duration_ms`, `action_count`, `sensitive_data_enabled`, `test_id`), bounded to a response budget.

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
configure({
  what: 'playback',
  recording_id: 'shopping-checkout-20260130T143022Z'
})
```

Results are stored per recording and can be re-read with `observe({what: 'playback_results', recording_id})`.

**Not implemented:** an inline `actions` array (LLM-generated variation), `test_id`, `sequence_mode` and `timeout_ms` parameters. Playback only accepts `recording_id`.

### Test Generation (LLM-Generated Variations)

> **Status: proposed, not implemented.** The shipped `playback` mode only accepts `recording_id`.

#### LLM Can Generate Action Variations:

Playback would accept both recorded AND LLM-generated action sequences. LLM can synthesize variations by modifying the action array JSON:

```javascript
// Original recorded actions
observe({what: 'recording_actions', recording_id: 'checkout-flow'})
// Returns: {recording_id: "checkout-flow", actions: [...]}

// LLM generates variation (e.g., different coupon code)
// LLM modifies actions array and invokes playback with custom actions:
configure({
  what: 'playback',
  actions: [  // proposed: custom action array instead of recording_id
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
- [ ] Playback accepts custom action arrays (not implemented)
- [x] No special "test generation" feature needed (LLM generates JSON directly)
- [ ] Variations logged and compared like recorded flows (depends on the above)

### Log Diffing & Regression Detection

#### Compare Logs:
```javascript
configure({
  what: 'log_diff',
  original_id: 'shopping-checkout-20260130T143022Z',
  replay_id: 'shopping-checkout-20260131T090000Z'
})
// Re-read the stored report later:
observe({what: 'log_diff_report', original_id: '...', replay_id: '...'})
```

The diff compares two recordings (`internal/recording/logdiff/compare.go`: action counts, new errors, fixed errors, value changes, overall status). Logs can also be read per test boundary via `configure({what: 'test_boundary_start' | 'test_boundary_end'})` tagging.

---

## Implementation Details

> Verified 2026-11-21 against the code. Sections marked **not built** remain design.

### Extension Action Capture (TypeScript)

- Page-side action capture already exists in `src/lib/page/actions.ts` (`handleClick`, input/scroll handlers) calling `recordEnhancedAction` in `src/lib/page/reproduction.ts`; these feed the general `observe({what: 'actions'})` buffer.
- `src/background/recording/` is the **video** (tab capture) recording module, not flow-event recording.
- **Built (2026-11-23):** every AI `interact` action is forwarded into the active flow recording. `actionlog.Recorder` (the single funnel for interact actions) feeds `RecordingManager.RecordIfActive` while a recording is active; actions without a reproduction type are kept with a `dom_` prefix. Typed text (`type`/`input`) is redacted unless `sensitive_data_enabled`.
- **Not built:** recording-specific metadata (screenshot_path per action, x/y), and forwarding *human* browser actions (captured by the extension into telemetry) into the flow recording.

### Server Recording Storage (Go)

**Files:** `internal/recording/manager.go`, `internal/recording/manager_store.go`, `internal/recording/manager_storage.go`, `internal/recording/types.go`

- `RecordingManager` holds recordings in memory (map keyed by ID) with a single active recording
- `StartRecording` / `StopRecording` / `AddRecordingAction` manage the lifecycle
- Recordings persist as `metadata.json` under the Kaboom state recordings directory and are loaded from disk
- Storage capped at 1GB (`RecordingStorageMax`); start fails with `recording_storage_full` when exceeded
- `DeleteRecording` / `GetStorageInfo` for manual cleanup (exposed over HTTP via `HandleRecordingStorage` in `internal/ingestion/http/handlers.go`)

### Playback Engine (Go)

**Files:** `internal/recording/playback/session.go`, `internal/recording/playback/actions.go`, `internal/recording/playback/fragile.go`

- `playback.Execute` loads a recording and runs `executeAction` for each action, producing a `Session` with per-action `Result`s
- Current execution is **server-side simulation only**: navigate/type/scroll are marked ok, and click runs the self-healing strategy against the recorded metadata. It does not yet drive the browser through the extension, wait for network idle, or take screenshots on error.
- `DetectFragileSelectors` flags selectors that fail across sessions

The algorithm and network-idle rules below are the target design:

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

### WebSocket Migration — not built

A server push stream (`/api/stream`) with an extension WebSocket client and polling fallback was proposed to reduce timing jitter. No such endpoint or extension client exists; the extension still talks to the server over HTTP.

### Element Selector Self-Healing (Go)

**File:** `internal/recording/playback/actions.go` (`executeClickWithHealing`)

Shipped strategy (priority order):
1. data-testid match: `[data-testid=X]`
2. Recorded CSS selector
3. Recorded x/y coordinates (`nearby_xy`)
4. Last-known position when a screenshot path exists (`last_known`)
5. Otherwise `selector_not_found`

OCR recovery remains Phase 2.

### Log Diffing Engine (Go)

**Files:** `internal/recording/logdiff/compare.go`, `internal/recording/logdiff/report.go`, `internal/recording/logdiff/types.go`

- `logdiff.Compare(src, originalID, replayID)` compares two recordings
- `compareActions` compares action counts; `detectRegressions` / `detectFixes` find errors new in the replay or gone from it; `detectValueChanges` flags changed values
- `determineStatus` sets the overall verdict

### Test Boundary Tagging (Go)

Already implemented and reused unchanged:
- Active boundary set: `activeTestIDs` in `internal/sync/runtime/runtime.go`
- `TestIDs []string` on network bodies, WebSocket events and enhanced actions, plus `TestID` filter fields, in `internal/platform/types/network.go`

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
- At 100% capacity (1GB): `event_recording_start` returns error: "Recording storage at capacity (1GB). Delete old recordings to continue."
- User must manually delete old recordings: `rm ~/.kaboom/recordings/{recording_id}`
- Next `event_recording_start` call still fails if capacity not freed
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
// When event_recording_start called with sensitive_data_enabled=true:
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

// Later: replay inside a boundary (playback itself takes only recording_id)
configure({what: 'test_boundary_start', test_id: 'replay-checkout'})
configure({what: 'playback', recording_id: 'shopping-checkout-20260130T...'})
configure({what: 'test_boundary_end', test_id: 'replay-checkout'})

// Compare
observe({what: 'logs', test_id: 'original-checkout'})  // original logs
observe({what: 'logs', test_id: 'replay-checkout'})    // replay logs

// Option B: Record without explicit test boundary
configure({what: 'event_recording_start', name: 'checkout-flow'})
// user performs flow, extension captures (no test_id tagging)
configure({what: 'event_recording_stop', recording_id: '...'})

// Later: replay with explicit test boundary
configure({what: 'test_boundary_start', test_id: 'replay-1'})
configure({what: 'playback', recording_id: 'checkout-flow-20260130T...'})
configure({what: 'test_boundary_end', test_id: 'replay-1'})
```

**Key Design:** Recording captures actions; test boundaries tag everything else. They're independent but compose naturally.

---

## Files

### Shipped

| File | Purpose |
|------|---------|
| `internal/recording/manager.go` | Recording lifecycle, active recording, storage cap |
| `internal/recording/manager_store.go` / `internal/recording/manager_storage.go` | Disk persistence and storage accounting |
| `internal/recording/types.go` | `Recording`, `RecordingAction`, summaries |
| `internal/recording/playback/` | Playback sessions, self-healing click, fragile-selector detection |
| `internal/recording/logdiff/` | Recording comparison and reports |
| `internal/recording/actionlog/recorder.go` | Server-side recorder for the general actions buffer |
| `internal/tools/recording/handler.go` | MCP handlers (`EventRecordingStart/Stop`, `Recordings`, `RecordingActions`, `Playback`, `PlaybackResults`, `LogDiff`, `LogDiffReport`) |
| `internal/composition/toolruntime/tools_configure.go` | Registers the configure modes |
| `internal/tools/observe/handler/dispatcher.go` | Registers the observe modes |
| `src/lib/page/actions.ts` | Page-side action capture (reused) |

### Tests

- `internal/recording/manager_test.go`, `internal/recording/manager_active_test.go`, `internal/recording/types_test.go`
- `internal/recording/playback/playback_test.go`, `internal/recording/logdiff/logdiff_test.go`
- `internal/tools/recording/handler_test.go`, `internal/tools/recording/playback_result_test.go`
- `tests/extension/recording-lifecycle/` (extension recording lifecycle tests)

### Not built

- Extension wiring that streams captured actions into the active flow recording
- WebSocket push stream and extension client
- Browser-driven playback (extension execution, network idle waits, screenshots on error)
- Inline `actions` array playback (LLM variations)

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

### MCP Modes (shipped):
- `configure` what: `event_recording_start`, `event_recording_stop`, `playback`, `log_diff`
- `observe` what: `recordings`, `recording_actions`, `playback_results`, `log_diff_report`

---

## Remaining Implementation Order

1. Wire human (extension-captured) actions into the active flow recording, with recording metadata (x/y, screenshot_path). AI interact actions are already wired.
2. Drive playback through the extension (real clicks/typing, network-idle waits, screenshots on error)
3. Accept inline `actions` arrays for LLM-generated variations
4. Optional: WebSocket push stream if HTTP timing jitter proves to matter
5. End-to-end UAT: record → replay → diff → analyze

---

## Next Steps

1. **Principal Review** — Validate technical approach against product spec
2. **QA Plan** — Define test cases (unit + integration + UAT)
3. **Implementation** — TDD, write failing tests first
4. **Release** — Tag as v6.0, deploy with existing Kaboom

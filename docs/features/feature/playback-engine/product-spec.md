---
feature: Playback Engine
status: proposed
tool: configure, observe
mode: playback, playback_results
version: 0.7.12
doc_type: product-spec
feature_id: feature-playback-engine
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Playback Engine

## Problem Statement

### Recorded flows can't actually be replayed in the browser.

`internal/capture/playback.go` is a stub. It defines types and a self-healing selector cascade, but `executeAction` returns `"ok"` without dispatching any real browser commands. This blocks the core value proposition of flow recording: regression testing through replay.

Today:
1. **Recording works** — clicks, typing, navigation captured with selectors, coordinates, and timestamps
2. **Playback is fake** — `ExecutePlayback()` iterates actions but never touches the browser
3. **No regression testing** — without real replay, developers can't verify fixes against recorded flows
4. **No synthetic flows** — LLMs can't execute ad-hoc action sequences for exploratory testing

**Result:** Flow recording captures data that can never be used. The regression testing workflow documented in the flow-recording spec is blocked entirely.

---

## Solution

Wire `executeAction` to dispatch real browser commands through the existing PendingQuery/interact system. Playback = automated `interact()` calls driven by a recording or synthetic action array.

### The workflow:
1. **LLM starts playback** via `configure({action: "playback", recording_id: "checkout-flow"})` or passes a synthetic action array
2. **Server returns immediately** with `{status: "running", playback_id: "..."}` — playback is asynchronous
3. **Server iterates actions** — each `RecordingAction` maps to an `interact()` primitive (navigate, click, type, scroll, select, check, key_press)
4. **Extension executes** — same dispatch path as manual `interact()` calls, via PendingQuery
5. **Results accumulate** — each action produces a status (ok/failed/healed), duration, and error detail
6. **LLM queries results** — `observe({what: "playback_results", playback_id: "..."})` returns partial or final report

### Why configure, not interact:
- Playback is a **session-level operation** (start, monitor, query results) — same pattern as `event_recording_start`/`event_recording_stop`
- `interact()` is for individual atomic actions; playback orchestrates many
- Aligns with the existing `configure()` session management surface

### Architecture constraint: RecordingAction ↔ EnhancedAction parity

The extension captures rich action data as `EnhancedAction` (6 selector strategies, source tracking, scroll position, key data, selected values). The current `RecordingAction` struct discards most of this when persisting. Playback accuracy depends on field parity between these types. The tech spec MUST address propagating `EnhancedAction` fields into `RecordingAction` before implementation begins. Key fields to propagate:

- `Selectors map[string]any` — multi-strategy selector map (testId, ariaLabel, role, id, text, cssPath)
- `Source string` — `"human"` or `"ai"` action origin
- `Key string` — keypress key name
- `SelectedValue string` / `SelectedText string` — dropdown selection data
- `ScrollY int` — scroll position at action time
- `ViewportWidth int` / `ViewportHeight int` — viewport dimensions per action

---

## User Workflows

### Workflow 1: Replay a Saved Recording

```
1. LLM: configure({action: "playback", recording_id: "checkout-flow-20260218T1400Z"})
2. Returns immediately: {status: "running", playback_id: "pb-abc123"}
3. Server iterates actions in background:
   - navigate to https://example.com/checkout
   - click [data-testid=email-field]  (subtitle: "Playback [2/4]: click email-field")
   - type "user@test.com"
   - click [data-testid=submit-btn]
4. Each action dispatched via PendingQuery → extension executes in browser
5. LLM: observe({what: "playback_results", playback_id: "pb-abc123"})
6. Returns:
   {
     status: "completed",
     playback_id: "pb-abc123",
     actions_executed: 3,
     actions_failed: 1,
     actions_healed: 0,
     duration_ms: 4200,
     results: [...]
   }
```

### Workflow 2: Execute Synthetic Flow

```
1. LLM constructs action array from context (no saved recording needed):
   configure({action: "playback", actions: [
     {action: "navigate", url: "https://example.com/login"},
     {action: "click", selector: "[data-testid=login-btn]"},
     {action: "type", selector: "#email", text: "test@example.com"},
     {action: "click", selector: "[data-testid=submit]"}
   ]})
2. Same execution engine as recording playback
3. Results queryable via observe({what: "playback_results"})
```

### Workflow 3: Regression Test with Test Boundary

```
1. LLM: configure({action: "playback", recording_id: "checkout-flow",
         test_id: "checkout-regression", auto_boundary: true})
2. Server auto-starts test boundary "checkout-regression"
3. Actions execute, logs captured under test boundary
4. Server auto-ends test boundary on playback completion
5. LLM: observe({what: "playback_results", playback_id: "..."})
6. LLM compares original recording logs vs replay logs
7. Regression detected or fix verified
```

Manual boundary control is still supported (omit `auto_boundary`, manage boundaries separately).

### Workflow 4: Monitor Playback Progress

```
1. LLM starts playback (long flow, 50+ actions)
   → Returns {status: "running", playback_id: "pb-abc123"}
2. While running: observe({what: "playback_results", playback_id: "pb-abc123"})
   → Returns {status: "running", actions_executed: 12, actions_total: 50, ...}
3. On completion: {status: "completed", ...} with full report
```

### Workflow 5: Compose Video Recording with Playback

```
1. LLM: interact({action: "screen_recording_start", name: "regression-replay"})
2. LLM: configure({action: "playback", recording_id: "checkout-flow"})
3. LLM polls observe({what: "playback_results", playback_id: "..."}) until status: "completed"
4. LLM: interact({action: "screen_recording_stop"})
→ Narrated regression test video (subtitles auto-captured during playback)
```

---

## Core Requirements

### R1: Action Execution

Map each `RecordingAction` to an `interact()` primitive:

| Recording Action | Interact Primitive | Behavior |
|------------------|--------------------|----------|
| `navigate` | `navigate` to URL | Wait for page load (R6) |
| `click` | `click` with selector | Self-healing cascade (R3) |
| `type` | `type` text into element | Clear + type with selector; handle redacted text (R9) |
| `select` | `select` with value | Select dropdown option by value or text |
| `check` | `check` / uncheck | Toggle checkbox by selector |
| `key_press` | `key_press` with key | Send keyboard event (Enter, Tab, Escape, etc.) |
| `scroll` | `scroll_to` position | Scroll to x/y coordinates |
| `screenshot` | `screenshot` (observe-only) | Capture screenshot checkpoint, do not "execute" as action |

Unsupported action types: return `status: "skipped"` with `error: "unsupported_action_type: {type}"` (not `"error"` — distinguish from real failures).

- [ ] Sequential execution: wait for each action result before proceeding to next
- [ ] Each action dispatched through PendingQuery system (same path as direct `interact()`)
- [ ] Action timeout: 10s per action (configurable)
- [ ] Visual feedback: inject subtitle for each action during playback, e.g. `"Playback [3/12]: click [data-testid=submit-btn]"` — subtitles are captured automatically if tab recording is active

### R2: Timing Model

Two modes for inter-action timing:

- [ ] **Fast-forward** (default, `timing: "fast"`): no delays, execute ASAP after previous completes
- [ ] **Recorded timing** (`timing: "recorded"`): replay with original inter-action delays from `TimestampMs` deltas
  - Max inter-action delay capped at 30s (prevents long pauses from recording gaps)
  - Negative deltas (out-of-order timestamps) treated as zero
  - All timestamps within a recording must come from the same clock source (extension-side `Date.now()`)

Fast-forward is the default because regression testing prioritizes speed over visual fidelity.

### R3: Self-Healing Selectors

For click, type, select, and check actions, attempt element location with fallback strategies. Requires `Selectors map[string]any` from `EnhancedAction` to be propagated into `RecordingAction` (see architecture constraint above).

Selector cascade (priority order):

1. **data-testid** — `[data-testid=value]` (most reliable for dynamic UIs)
2. **aria-label** — `[aria-label="value"]` (accessibility-stable)
3. **role + name** — `role=button, name="Submit"` (semantic match)
4. **id** — `#element-id` (if present)
5. **text content** — visible text match (for clickable elements)
6. **CSS path** — computed CSS path with dynamic class filtering
7. **Coordinate fallback** — click at recorded x/y with viewport scaling (see R10)

- [ ] On self-healing (strategy 2+ succeeds): mark result as `status: "healed"`, log which strategy worked AND the actual selector string that succeeded (`healed_selector` field)
- [ ] On failure (all strategies exhausted): mark `status: "failed"`, capture screenshot, continue playback
- [ ] On healed actions: capture screenshot with visual highlight of the element actually matched
- [ ] Track selector failures per concrete selector string across runs for fragile detection

### R4: Error Handling

Three error modes, controlled by `on_error` parameter:

- [ ] **`"continue"`** (default): action failures logged, playback continues
- [ ] **`"skip_dependent"`**: if a `click` or `navigate` fails, skip subsequent `type`/`select`/`check` actions until the next successful `click` or `navigate` (prevents typing into wrong elements)
- [ ] **`"stop"`**: halt playback on first failure (for strict regression testing)

Each action result includes:

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | `"ok"` \| `"failed"` \| `"healed"` \| `"skipped"` |
| `duration_ms` | number | Execution time |
| `error` | string | Human-readable error detail (empty on success) |
| `error_code` | string | Machine-readable enum: `selector_not_found`, `element_hidden`, `element_disabled`, `navigation_timeout`, `page_error`, `url_mismatch`, `redacted_value`, `unsupported_action_type`, `skipped_dependency` |
| `likely_cause` | string | Optional context hint, e.g. `"auth_redirect"` when selector fails after a URL mismatch |
| `selector_used` | string | Which strategy succeeded (for click/type) |
| `healed_selector` | string | Actual selector string that succeeded on heal |
| `page_url` | string | Actual page URL when action executed |
| `screenshot_path` | string | Screenshot path (captured on `"failed"` and `"healed"`) |

Final playback report:

- `actions_executed`: count of ok + healed
- `actions_failed`: count of failures
- `actions_healed`: count of self-healed actions
- `actions_skipped`: count of skipped actions (dependency or unsupported)
- `duration_ms`: total playback time
- `screenshots`: list of all screenshot paths captured

### R5: MCP API

#### Start Playback

`configure({action: "playback", ...})` — returns immediately, playback runs asynchronously.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `recording_id` | string | one of | ID of saved recording to replay |
| `actions` | array | one of | Synthetic action array (R8) |
| `timing` | string | no | `"fast"` (default) or `"recorded"` |
| `on_error` | string | no | `"continue"` (default), `"skip_dependent"`, or `"stop"` |
| `test_id` | string | no | Test boundary ID for log correlation |
| `auto_boundary` | bool | no | Auto-manage test boundary start/end around playback |
| `tab_id` | number | no | Target tab (default: active tab) |
| `value_overrides` | object | no | Map of action index → replacement text for redacted values |

Must provide either `recording_id` or `actions`, not both.

Returns: `{status: "running", playback_id: "pb-abc123"}`

If a playback is already running: `{error: "PLAYBACK: Already running. Query or wait for current playback to complete."}`

#### Query Results

`observe({what: "playback_results", playback_id: "pb-abc123"})` returns:

```json
{
  "status": "running",
  "playback_id": "pb-abc123",
  "recording_id": "checkout-flow-20260218T1400Z",
  "actions_executed": 5,
  "actions_failed": 0,
  "actions_healed": 1,
  "actions_skipped": 0,
  "actions_total": 8,
  "duration_ms": 2800,
  "results": [
    {
      "index": 0,
      "action": "navigate",
      "status": "ok",
      "duration_ms": 1200,
      "page_url": "https://example.com/checkout",
      "selector_used": "navigate"
    },
    {
      "index": 1,
      "action": "click",
      "status": "healed",
      "duration_ms": 350,
      "selector_used": "aria_label",
      "healed_selector": "[aria-label=\"Log in\"]",
      "page_url": "https://example.com/checkout",
      "screenshot_path": "/path/to/healed-1.png"
    }
  ]
}
```

Status transitions: `"running"` → `"completed"` | `"failed"` (if `on_error: "stop"` and an action failed) | `"not_found"` (invalid playback_id)

### R6: Navigation Waits

After `navigate` actions, wait for page readiness before proceeding:

- [ ] Wait for page load event (network idle heuristic or `load` event)
- [ ] SPA detection: if no `load` event fires within 1s, fall back to DOM stability check (no mutations for 500ms)
- [ ] Timeout: 5s — if page hasn't loaded, continue with warning
- [ ] Log navigation timing in action result
- [ ] Redirects: follow automatically; log original URL vs final URL in action result
- [ ] URL validation: if final URL differs from recorded URL, set `page_url` in result so LLM can detect unexpected redirects (e.g., auth wall)

### R7: Tab Targeting

- [ ] Default: active tab (same as `interact()` default)
- [ ] Optional: `tab_id` parameter targets a specific tab
- [ ] All actions in a playback session target the same tab

### R8: Synthetic Flows

LLMs can pass an action array directly, without a saved recording:

```json
{
  "action": "playback",
  "actions": [
    {"action": "navigate", "url": "https://example.com"},
    {"action": "click", "selector": "[data-testid=login]"},
    {"action": "type", "selector": "#email", "text": "user@test.com"},
    {"action": "click", "selector": "[data-testid=submit]"}
  ]
}
```

- [ ] Same execution engine as recording playback — no separate code path
- [ ] Actions are not persisted (fire-and-forget execution)
- [ ] Results queryable via `observe({what: "playback_results"})` same as recording playback

### R9: Redacted Value Handling

Default recordings redact all `type` action text to `"[redacted]"` (when `sensitive_data_enabled` is false). The playback engine MUST detect this and handle it explicitly rather than typing literal `"[redacted]"` into form fields.

- [ ] Detect `[redacted]` text in `type` actions before execution
- [ ] If `value_overrides` provides a replacement for the action index, use the override value
- [ ] If no override is provided, skip the action with `status: "skipped"`, `error_code: "redacted_value"`, and a clear message: `"Action requires redacted credential. Provide value_overrides or re-record with sensitive_data_enabled."`
- [ ] Never type the literal string `"[redacted]"` into any element

Example with overrides:
```json
{
  "action": "playback",
  "recording_id": "login-flow",
  "value_overrides": {
    "3": "test@example.com",
    "5": "test-password-123"
  }
}
```

### R10: Recording Format Requirements

The `RecordingMetadata` and `RecordingAction` structs must be extended before playback implementation begins.

#### Schema versioning:
- [ ] Add `schema_version: 1` to `RecordingMetadata`
- [ ] Loader must check version and reject incompatible recordings with a clear error
- [ ] Increment version on any breaking field change

#### Per-action viewport and scroll context:
- [ ] `viewport_width`, `viewport_height` per action (detect resize events mid-recording)
- [ ] `scroll_x`, `scroll_y` per action (required for coordinate normalization)
- [ ] Coordinates (`x`, `y`) are viewport-relative (`event.clientX/clientY`)
- [ ] During playback coordinate fallback: scale coordinates by `replay_viewport / recording_viewport` ratio

#### Environment metadata (per recording):
- [ ] `browser` and `browser_version` — from extension's `navigator.userAgent`
- [ ] `device_pixel_ratio` — from `window.devicePixelRatio` (critical for coordinate scaling on HiDPI displays)
- [ ] `extension_version` and `server_version`

#### Action source tracking:
- [ ] `source` field on `RecordingAction`: `"human"` (from content script listener) or `"ai"` (from `interact()` PendingQuery path)
- [ ] Propagated from `EnhancedAction.Source` which already captures this

### R11: Playback Prerequisites and Environment

Playback assumes the browser is in a compatible state. The engine validates what it can and warns about what it cannot.

#### Viewport check:
- [ ] On playback start, compare current viewport to recording's viewport dimensions
- [ ] If dimensions differ by >20%, log a warning in the playback results: `"viewport_mismatch: Recording was 1920x1080, current is 1366x768. Coordinate fallback may be inaccurate."`
- [ ] Do not block playback — selector-based strategies are viewport-independent

#### Authentication and browser state:

Playback does NOT capture or restore cookies, localStorage, or session state. Auth failures are the most common cause of playback failure in real-world usage. The engine detects and surfaces these clearly so the LLM can respond.

**Detection — URL mismatch after navigate:**
- [ ] After every `navigate` action, compare the actual page URL to the recorded URL
- [ ] If URLs differ (e.g., expected `/checkout`, landed on `/login` or `/sso/authorize`), add a diagnostic to the action result:
  - `error_code: "url_mismatch"`
  - `error: "Expected https://app.com/checkout, landed on https://app.com/login. Browser may need authentication."`
  - `page_url: "https://app.com/login"` (the actual URL)
- [ ] The action status remains `"ok"` (navigation itself succeeded) but the diagnostic is present for the LLM to act on

**Detection — selector failures after navigate mismatch:**
- [ ] If a `navigate` produced a `url_mismatch` and the subsequent action(s) fail with `selector_not_found`, the engine annotates: `"likely_cause: "auth_redirect"` on those failures
- [ ] This helps the LLM distinguish "the button moved" from "we're on the wrong page entirely"

**Behavior — what happens when auth is broken:**
- [ ] With `on_error: "continue"` (default): playback runs all actions, most fail, results clearly show the auth wall pattern (URL mismatch → cascade of selector failures)
- [ ] With `on_error: "skip_dependent"`: after the first selector failure post-auth-redirect, subsequent dependent actions are skipped — faster failure with clearer signal
- [ ] With `on_error: "stop"`: playback halts at the first failed action after auth redirect

**LLM recovery pattern** (documented for LLM consumption, not engine behavior):
```
1. LLM starts playback → gets url_mismatch on first navigate
2. LLM recognizes auth pattern from error_code + page_url
3. LLM uses interact() to log in (type credentials, click submit)
4. LLM re-starts playback from the failed action using synthetic flow (R8)
   with the remaining actions from the original recording
```

**Prerequisites:**
- [ ] The LLM or user must ensure the browser is in the correct authentication state before starting playback of authenticated flows
- [ ] Future enhancement: optional cookie/storage snapshot capture and restore at recording time

#### Dynamic content:
- [ ] Playback does not handle timestamps, CSRF tokens, random IDs, or other dynamic content in form fields
- [ ] For flows with dynamic content, use synthetic flows (R8) with LLM-constructed action arrays that supply fresh values
- [ ] `value_overrides` (R9) can also replace specific action values at replay time

---

## Known Limitations (MVP)

These are deliberate scope boundaries, not bugs. Each is documented so the LLM can reason about them and suggest workarounds.

| Limitation | Workaround |
|-----------|------------|
| No iframe support | Selectors inside iframes will not resolve. Use coordinate fallback or re-record with top-level elements. Shadow DOM is supported — `interact()` uses `querySelectorDeep()` which traverses shadow roots automatically. |
| No cookie/auth state capture | Log in manually or via synthetic flow before starting playback. |
| No dynamic content handling | Use synthetic flows with fresh values, or `value_overrides`. |
| CSS-in-JS class instability | Use `data-testid` attributes. Recording should filter dynamic class names (tech spec concern). |
| Single concurrent playback | One playback session at a time. Second start returns error. |

---

## Out of Scope (MVP)

- **Visual/OCR element recovery** — coordinate fallback with viewport scaling is sufficient for MVP; visual matching deferred
- **Parallel action execution** — sequential-only; parallel adds complexity with no clear MVP need
- **Cross-tab flows** — single tab per playback session
- **CI/CD integration** — playback is MCP-triggered; CI integration is a separate feature
- **Playback pause/resume** — once started, playback runs to completion (LLM can use synthetic flows to re-run from a specific action)
- **Action editing during playback** — modify the action array before starting, not mid-flight
- **Conditional branching** — no if/else logic in action sequences; LLM handles orchestration
- **Assertion actions** — no `assert_text`, `assert_url`, `assert_element_visible` in action sequences; LLM queries results after playback and performs its own assertions
- **iframe support** — selectors scoped to top frame only; cross-frame support deferred (Shadow DOM IS supported via `querySelectorDeep()`)
- **Cookie/storage state capture** — browser state not captured or restored; user/LLM handles auth prerequisites
- **Concurrent playback sessions** — one at a time; multi-session support deferred

---

## Success Criteria

### Functional
- Recorded flow replays end-to-end in a real browser via interact dispatch
- Synthetic action arrays execute correctly through the same engine
- All mapped action types work: navigate, click, type, select, check, key_press, scroll, screenshot
- Self-healing selector cascade uses all available strategies (7-level cascade)
- Healed and failed actions produce screenshots with context
- Redacted values detected and handled (skip or override, never type `"[redacted]"`)
- Results queryable via `observe({what: "playback_results", playback_id: "..."})` with per-action detail
- Test boundary integration works with `auto_boundary` for streamlined regression workflow
- All three error modes work: `continue`, `skip_dependent`, `stop`
- Both `timing: "fast"` and `timing: "recorded"` modes work correctly
- Playback subtitles visible in browser during execution
- Viewport mismatch and URL mismatch warnings produced when applicable

### Non-Functional
- Action dispatch overhead: < 50ms per action (excluding browser execution time)
- Fast-forward playback: limited only by browser execution speed
- Memory: playback session state < 1MB for 100-action flows
- No new production dependencies

### Integration
- Works with existing PendingQuery/interact dispatch system
- Works with test boundaries for regression log comparison
- Works with tab recording for narrated replay videos
- Works with `observe()` for result querying (partial and final results)
- Async execution model: start returns immediately, poll for results
- MCP API consistent with configure/observe patterns
- Recording format includes schema version for forward compatibility

---

## Dependencies

### Internal
- **PendingQuery system** (`internal/capture/queries.go`) — dispatches commands to extension
- **interact tool** (`cmd/browser-agent/tools_interact.go`) — existing action primitives
- **configure tool** (`cmd/browser-agent/tools_configure.go`) — playback start entry point
- **observe tool** (`cmd/browser-agent/tools_observe.go`) — playback results query
- **Recording types** (`internal/recording/types.go`) — `RecordingAction` struct
- **Test boundaries** (`internal/capture/test_boundary.go`) — log correlation

### Existing Stub
- `internal/capture/playback.go` — types (`PlaybackResult`, `PlaybackSession`, `Coordinates`) and self-healing skeleton already defined; implementation will replace stub logic

---

## Next Steps

1. **Tech Spec** — architecture for PendingQuery integration, action mapping, session lifecycle
2. **QA Plan** — test scenarios for each action type, self-healing, timing modes, error cases
3. **Implementation** — wire executeAction to PendingQuery dispatch, add configure/observe handlers

---
feature: Flow Recording & Playback
status: proposed
tool: interact, observe, configure
mode: recording, playback, test-generation
version: 0.7.12
doc_type: product-spec
feature_id: feature-flow-recording
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Flow Recording & Playback (Regression Testing)

## Problem Statement

### When bugs are discovered in production, developers spend hours reproducing them, analyzing root causes, and verifying fixes — manually.

Regression testing today is **slow, manual, and error-prone**:

1. **Reproducibility Gap:** "Works on my machine" — reproducing user-reported bugs requires exact environment replication
2. **Root Cause Blindness:** Logs are opaque; developers manually trace through to understand what broke
3. **Fix Verification:** After coding a fix, QA manually re-tests the entire flow (15-30 min per bug)
4. **Regression Risk:** No automated way to re-test all historical flows; some regressions re-appear multiple times
5. **Incident Response:** On-call engineers waste time on reproduction; they need answers in minutes, not hours

**Result:** Critical bugs take 2-4 hours to fix + verify; non-critical regressions accumulate; team loses confidence in releases.

---

## Solution

**Kaboom Flow Recording & Playback** is the **AI-powered regression testing tool for developers**.

### The workflow:
1. **QA records** a user's reported flow once (e.g., "checkout fails on coupon code entry")
   - Recording captures: clicks, typing, navigation, network calls, console errors, DOM state
2. **Developer fixes** the bug in code
3. **LLM invokes Kaboom** to replay the exact same flow (against fixed code)
4. **Kaboom compares** original recording logs vs replay logs:
   - What changed? (error message now present, API response different, element location moved?)
   - What's new? (404 error, console warning, network timeout?)
   - What's missing? (cleanup event, success message?)
5. **LLM analyzes** the diff and tells developer: "Regression is fixed ✓" or "Still broken, here's why"
6. **Developer verifies** the fix in <5 minutes (not 30 minutes of manual testing)

### Why Kaboom:
- **Purpose-built for regression testing**, not general test automation
- **Structured log diffing** — record a flow once, replay later, automatically detect what changed (the core product)
- **Error detection** — identify 404s, timeouts, missing elements, log changes automatically
- **AI-ready data pipeline** — logs, diffs, errors fed directly to Claude for analysis
- **Local-first** — runs entirely on your machine (no cloud, no shared state)
- **Zero dependencies** — lean, fast, audit-friendly

---

## User Workflows

### Workflow 1: Record a Flow (QA Engineer)

```
1. QA clicks "Start Recording" in extension (or calls configure() via MCP)
2. Browser navigates to target URL
3. QA performs flow: login → add to cart → checkout → submit
4. Extension records:
   - URL changes (timestamps)
   - Clicks (x/y, selectors)
   - Typing (field selector, text, x/y)
   - Screenshots at key points
5. QA clicks "Stop Recording"
→ Recording saved as "shopping-checkout-{timestamp}"
→ Recording is now queryable via MCP
```

### Workflow 2: Replay & Verify Fix (LLM + CI)

```
1. Bug found: "Checkout button missing error message"
2. Dev fixes bug in code
3. LLM says: "replay shopping-checkout recording"
4. Kaboom:
   - Navigates to original URL
   - Replays all clicks/typing in sequence
   - Captures new logs alongside original
5. LLM compares original logs vs new logs:
   - Error message missing in original ✗
   - Error message present in new ✓
   - Regression fixed ✓
6. Test passes, CI proceeds
```

### Workflow 3: Generate Variations (LLM Synthesized)

```
1. LLM reads recorded action sequence:
   [navigate → click product → type qty:1 → checkout]
2. LLM generates variations:
   - qty: 1 → 5
   - product: item-1 → item-3
   - Variation: Add coupon code
3. Each variation replayed, logs compared
4. All variations pass → broader regression coverage
```

### Workflow 4: Root Cause Analysis & Auto-Fix (LLM + Git)

```
1. Playback detects regression (new logs differ from original)
2. LLM invokes /kaboom-fix skill:
   "Analyze logs and suggest fixes for checkout-flow regression"
3. Kaboom:
   - Diffs original vs new logs
   - Identifies error types (404, timeout, DOM change, etc.)
   - Suggests root cause ("Missing field 'cvv' in form")
   - If git available, finds commits that changed related code
   - Suggests code fixes (e.g., "Add cvv field validation")
4. LLM reviews suggestions and applies fixes to codebase
5. Re-runs playback to verify fix works
6. Skill reports: "✓ Fix verified, regression resolved"
```

---

## Core Requirements

### R1: Flow Recording

#### What Gets Recorded:
- [ ] **URL changes** (timestamp, new URL)
- [ ] **Clicks** (x/y coordinates, element selector, timestamp)
- [ ] **Typing** (field selector, text typed, x/y, timestamp)
- [ ] **Screenshots** (at key points: page load, after click, after type)
- [ ] **Page metadata** (viewport width/height, timestamp)

#### Storage:
- [ ] Recordings stored as **JSON action sequence** (for LLM consumption)
- [ ] Screenshots stored as **JPEG/PNG files on disk**
- [ ] Metadata: name, creation time, duration, action count
- [ ] Queryable via MCP: `observe({what: 'recordings'})`

#### Naming:
- [ ] User-provided name (e.g., "shopping-checkout")
- [ ] OR auto-generated: "{adjective}-{noun}-{adjective}-{ISO8601}" (e.g., "magic-badger-hammer-20260130T143022Z")
- [ ] Name + timestamp used in file paths/IDs

#### Recording Policy:
- [ ] Full text typed captured (necessary for regression testing with real data, including login flows)
- [ ] **Sensitive Data Toggle:** User can enable/disable recording of credentials
  - Default: Disabled (safe)
  - If enabled: ⚠️ **Warning:** "You are recording credentials. Ensure this is test data, not production credentials. Recordings stored only on localhost."
- [ ] Use case: Testing login flows on local dev environment requires recording test account credentials
- [ ] Recordings stored locally only; not transmitted to cloud

### R2: Recording UI

#### Extension Popup:
- [ ] Button: "Start Recording" (opens dialog for name + URL)
- [ ] Dialog: name field (optional, auto-generates if empty), URL field (optional)
- [ ] Button: "Stop Recording" (visible after start)
- [ ] List: recent recordings with timestamp, action count

#### MCP Actions:
- [ ] `configure({action: 'recording_start', name?: 'shopping-cart', url?: 'https://...'})`
- [ ] `configure({action: 'recording_stop', recording_id: 'shopping-cart-20260130T...'})` (auto-generates ID if not provided)
- [ ] Browser auto-navigates to URL if provided on `recording_start`

### R3: Screenshot Management

#### Format & Compression:
- [ ] Format: JPEG (85% compression)
- [ ] Max file size: 500KB per screenshot
- [ ] Storage: Disk (local file system)
- [ ] Naming: `{date}-{recording_name}-{action_index}-{issue_type}.jpg`
  - Example: `20260130-shopping-checkout-003-page-load.jpg`
  - Issue types: `page-load` (after navigation), `moved-selector` (element not found), `error` (assertion/timeout/network)

#### When Captured:
- [ ] On page load (after navigation)
- [ ] When element selector fails or moves
- [ ] On error/timeout
- [ ] Sampling: every N actions (configurable, default 5)

### R4: Element Matching & Self-Healing (Robust Selector Recovery)

#### For Playback, Match Elements Using (Priority Order):
1. **data-testid** attribute (most reliable for dynamic content)
2. **x/y coordinates + context** (if selector fails, search nearby elements)
3. **Visual recovery** (if above fails, use OCR on screenshots to find element by visible text)

#### Self-Healing on Selector Failure:
- [ ] Primary: Try exact data-testid match
- [ ] Secondary: Try recorded CSS selector
- [ ] Tertiary: Check if element moved (nearby search based on old x/y)
- [ ] Quaternary: Use OCR on screenshot to find by visible text
- [ ] Final: Use last-known x/y coordinates with warning

#### If Element is Fragile (Moved Multiple Times):
- [ ] Log warning: "Fragile selector: element moved 3 times across test runs"
- [ ] Screenshot with issue type: `moved-selector`
- [ ] Recommend using `data-testid` instead of selectors
- [ ] Suggest code change: "Add data-testid=product-card-1 to improve test stability"
- [ ] Report to LLM for debugging

#### Competitive Advantage:
- Kaboom has access to real browser context (logs, network, visual state)
- Can detect selector fragility and suggest fixes proactively
- Unlike cloud-based tools, we see the actual user environment

---

### R5: Playback & Sequence Execution

#### Execution:
- [ ] **Sequence mode**: Execute actions in order, ignoring timing (fast-forward)
  - Navigate → click → type → navigate → click
  - Useful for regression testing (speed is priority)
  - Target: 10+ actions/second

#### On Page Load During Playback:
- [ ] Wait for page to load (network idle or timeout: 5 sec)
- [ ] If selector not found, attempt self-healing (R4)
- [ ] If self-healing fails, log error and take screenshot
- [ ] Continue playback (non-blocking)

#### Error Handling (Non-Blocking):
- [ ] Selector not found (after self-healing) → Log + screenshot, continue
- [ ] Click outside viewport → Scroll to element, then click
- [ ] Type in non-input → Log error, continue
- [ ] Navigation timeout → Log, continue
- [ ] Network error → Log, continue

#### Graceful Degradation:
- Playback completes even if some actions fail (important for regression analysis)
- All failures logged and visible to LLM for debugging

---

### R6: MCP Integration

#### Recording Management:
- [ ] `observe({what: 'recordings'})` → List all recordings
  - Returns: `[{id, name, created_at, duration, action_count, url}]`
- [ ] `observe({what: 'recording_actions', recording_id: 'shopping-cart'})` → Action sequence
  - Returns: `[{action, selector, text, x, y, timestamp, screenshot_path}]`

#### Playback:
- [ ] `interact({action: 'playback', recording: 'shopping-cart-{id}', test_id: 'replay-shopping-cart'})`
  - Records playback logs under test boundary `replay-shopping-cart`
  - Returns: `{status, actions_executed, errors, duration}`

#### Test Boundary Integration:
- [ ] Playback runs under `test_boundary_id` so logs can be correlated
- [ ] LLM can query: `observe({what: 'logs', test_boundary: 'replay-shopping-cart'})`
- [ ] Compare to original logs: `observe({what: 'logs', test_boundary: 'original-shopping-cart'})`

---

### R7: Log Streaming (WebSocket Migration)

**Current Issue:** Polling introduces timing inaccuracy. Recording needs millisecond precision.

**Solution:** Migrate extension from polling to **WebSocket** connection.

#### Behavior:
- [ ] Extension connects to server via WebSocket on startup
- [ ] Logs/events streamed in real-time with timestamps
- [ ] Server buffers with timestamps (last 10 seconds)
- [ ] If buffer overflows, drop oldest, log warning icon in popup
- [ ] LLM receives logs with millisecond accuracy for action correlation

#### Backward Compatibility:
- [ ] Polling still works (fallback if WebSocket unavailable)
- [ ] Health check: if WebSocket stale > 3 sec, warn user

---

### R8: Test Generation (LLM Synthesis)

#### LLM Capability:
- [ ] Read recorded action sequence (JSON format)
- [ ] Understand action parameters (selector, text, coordinates)
- [ ] Generate variations:
  - Different input values (cart quantity: 1 → 3, 5, 10)
  - Different selectors (product A → B → C)
  - Different flows (skip coupon vs apply coupon)
  - Different user states (logged in vs guest)
- [ ] Generate new recordings as JSON (not stored, executed on-the-fly)

#### Format:
```json
[
  {"action": "navigate", "url": "https://example.com/shop"},
  {"action": "click", "selector": "[data-testid=product-qty-selector]"},
  {"action": "type", "selector": "input#quantity", "text": "3", "x": 523, "y": 456},
  {"action": "click", "selector": "[data-testid=add-to-cart-btn]", "x": 600, "y": 300},
  {"action": "navigate", "url": "https://example.com/checkout"}
]
```

#### Execution:
- [ ] LLM-generated variations executed same as recorded flows
- [ ] Logs captured, compared to original
- [ ] Results reported to LLM

---

### R9: Recording Storage & Management

#### Storage:
- [ ] **Max storage:** 1GB total on disk (warn at 80%, error at 100%)
- [ ] **Concurrent:** Only 1 active recording at a time
- [ ] Storage location: `~/.kaboom/recordings/` (configurable)

#### Guidance (Not Hard Limits):
- Typical flow: 5-30 minutes, 20-100 actions
- If recording approaches 1GB, user should manage manually:
  - Delete old recordings
  - Or expand storage
- Kaboom doesn't auto-delete recordings (data loss risk)

### R10: Log Diffing & Regression Detection

#### What Kaboom Provides (Core MVP):

#### Log Diffing:
- [ ] Compare original recording logs vs replay logs (structured diff)
- [ ] Show what's different:
  - New errors (404, 500, timeout, missing element)
  - Missing events (expected network call didn't happen, console message absent)
  - Changed values (success response different, form validation error different)
  - Timing changes (action took 2x longer, load delayed)
- [ ] Extract error context: error message, stack trace, affected URL, affected action, timestamp

#### Regression Detection:
- [ ] Categorize: Is this a regression? (present in replay but NOT in original recording)
- [ ] Alert LLM: "Regression detected: [error type] on [action]"
- [ ] Take screenshot when error detected (visual evidence)
- [ ] Mark errors as:
  - Regression (new in replay) = BUG
  - Expected (in both original + replay) = Known issue
  - Fixed (was in original, gone in replay) = SUCCESS

#### What Claude Does (With Kaboom Data):
- [ ] Review the log diff from Kaboom
- [ ] Analyze error patterns: "404 on /api/checkout" → likely cause is "endpoint missing"
- [ ] Propose code fixes: "Update client to call /api/orders instead"
- [ ] Rank confidence (HIGH/MEDIUM/LOW) based on error clarity
- [ ] Never auto-apply; developer reviews and applies fixes

#### Phase 2 (Optional):
- [ ] Git integration: Find commits that touched affected files
- [ ] Show commit history for context
- [ ] Test coverage analytics: Which code paths are untested

---

## Out of Scope

- Mobile/native app testing (web only, Kaboom doesn't run in mobile)
- Video replay (screenshots + logs sufficient)
- Real-time collaboration (async sharing via recordings)
- Multi-user conflict resolution (one recording at a time)
- Auto-apply fixes without review (Kaboom proposes, LLM reviews and applies)

---

## Success Criteria

### Functional
- ✅ LLM can start/stop recordings via `configure()` and UI
- ✅ All user interactions (click, type, navigate) recorded with selectors + x/y
- ✅ Screenshots captured and stored on disk
- ✅ Recordings queryable via `observe()`
- ✅ Playback replays actions in correct sequence
- ✅ Element selector matching works (data-testid → CSS → x/y fallback)
- ✅ Moved elements logged with screenshots
- ✅ LLM can read action sequences and generate variations
- ✅ Generated flows execute and logs are captured
- ✅ LLM can invoke `/kaboom-fix` skill to analyze regressions
- ✅ Root cause analysis detects error types (404, timeout, missing element, etc.)
- ✅ Fix suggestions identify affected files and propose code changes
- ✅ Git integration finds related commits (if repo available)
- ✅ Suggestions ranked by confidence (high/medium/low)

### Non-Functional
- ✅ Recording overhead: < 5% CPU, < 20MB memory per active recording
- ✅ Playback speed: 10+ actions per second (sequence mode)
- ✅ Log accuracy: ±100ms timestamp deviation (WebSocket streaming)
- ✅ Storage: 1GB limit enforced, cleanup automatic
- ✅ Screenshot size: < 500KB per image
- ✅ Zero data loss (buffer with warnings before drop)

### Integration
- ✅ Works with test boundaries (logs tagged with test_id)
- ✅ Works with log comparison (original vs replay)
- ✅ Works in CI/CD (automated playback)
- ✅ MCP API consistent with observe/interact patterns

---

## Integration & Dependencies

### Internal
- **observe()**: Extended to support `what: 'recordings'`, `what: 'recording_actions'`
- **interact()**: Extended with `action: 'playback'`
- **configure()**: Extended with `action: 'recording_start'`, `action: 'recording_stop'`
- **Test Boundaries**: Playback runs under test boundary for log correlation
- **WebSocket**: Requires migration from polling for accurate timing
- **Claude Skill**: New `/kaboom-fix` skill for LLM to invoke root cause analysis
- **Git Integration**: Optional (read-only) for finding related commits

### External
- **Browser APIs:** `chrome.webRequest` (capture URLs), Mutation Observer (detect clicks/typing), Intersection Observer (screenshots)
- **File System:** Local disk storage for screenshots/metadata
- **Database:** Optional (for large-scale recording management, defer to Phase 2)

---

## Success Stories

### Story 1: Regression Testing
```
QA records 5-minute checkout flow.
Week later, dev fixes "missing error message on invalid card".
LLM: "Replay checkout recording"
→ Playback executes, logs captured
→ LLM compares original vs replay logs
→ Error message now present ✓
→ Regression test passes, no manual testing needed
```

### Story 2: Automated Variations
```
LLM reads checkout recording (add item → checkout).
Generates 3 variations:
  - Different product (shoes → shirt)
  - Different quantity (1 → 5)
  - Different coupon (none → SUMMER20)
All variations run, logs compared.
→ All pass → broader regression coverage
→ Results reported to CI/CD
```

### Story 3: Fragile Selector Debugging
```
Recording: "Click 'Add to Cart' button"
UI changed: button moved from (500, 200) to (505, 210)
Playback:
  - Tries [data-testid=add-to-cart] ✓ (found, uses new coords)
  - Logs: "Element moved: old=[500,200] new=[505,210]"
  - Screenshot: moved-selector issue
→ LLM notified, suggested update selector for robustness
```

### Story 4: Root Cause Analysis (Kaboom + Claude)

```
Original: Checkout flow works (records clean logs).
Week later: Playback detects regression → POST /api/order returns 404.

LLM invokes /kaboom-fix skill:

Kaboom provides:
  - Error detected: "404 Not Found" on POST /api/order
  - Affected action: Step 5 (click checkout button)
  - Related commits: [abc123 "Refactor API endpoints", def456 "Fix endpoint"]
  - Affected files: src/api/checkout.ts, src/handlers/order.ts
  - Screenshot: shows the error page

Claude analyzes all this and responds:
  - Root cause: "POST /api/order endpoint was renamed to /api/orders in commit abc123"
  - Confidence: HIGH (git context + error type match perfectly)
  - Suggested fix: "In src/api/checkout.ts line 45, change fetch('/api/order') to fetch('/api/orders')"

Developer reviews & applies:
  - "Got it, the endpoint was renamed. I'll update the client call."
  - Applies fix: checkout.ts calls /api/orders instead
  - Re-runs playback
  → ✓ Regression resolved, logs match original
```

**Key:** Kaboom (data collection) + Claude (analysis) = Fast root cause + fix verification

---

## Metrics & Observability

### What Gets Logged:
- Recording start/stop (timestamp, name, action count)
- Playback execution (duration, actions executed, errors)
- Element matching (selector tried, success/failure, location shift)
- Screenshots (why, when, issue type)
- WebSocket events (connected, disconnected, buffer overflow, timestamp)

### LLM Observability:
- `observe({what: 'recordings'})` to list
- `observe({what: 'recording_actions'})` to read actions
- `observe({what: 'logs', test_boundary: 'replay-X'})` to compare

---

## Claude Skill: Analyze Regression (Kaboom Data → Claude Analysis)

### When to Use:
When playback detects a regression (replay logs differ from original), use this workflow to analyze the diff and suggest fixes.

### How to Use (Workflow, not API):
```
1. LLM reads original recording logs
2. LLM reads replay logs
3. LLM analyzes diff: "What changed? What's broken?"
4. LLM suggests fixes: "Try changing X to Y in file Z"
5. Developer applies fix
6. LLM replays recording to verify
```

### What Kaboom Provides:
- `observe({what: 'logs', test_boundary: 'original-checkout'})` → Original logs
- `observe({what: 'logs', test_boundary: 'replay-checkout'})` → Replay logs with regression
- `observe({what: 'recording_actions', recording_id: 'checkout'})` → The exact flow that was recorded
- Screenshots showing errors (visual evidence)
- Structured error detection: "404 on /api/order", "Missing element: .pay-btn", etc.

### What Claude Does:
- Compares the two log sets
- Detects what changed: new errors, missing calls, different responses
- Analyzes patterns: "404 on /api/order → endpoint renamed or removed"
- Suggests fixes: "Try /api/orders instead"
- Ranks confidence: HIGH (obvious match), MEDIUM (pattern-based), LOW (speculative)

### Example Flow:

```
Original logs:
  POST /api/order → 200 OK
  Response: {orderId: 123, status: "confirmed"}

Replay logs:
  POST /api/order → 404 Not Found
  Error: "Cannot POST /api/order"

Claude's analysis:
  Regression: 404 on /api/order (was working in original)
  Root cause: Endpoint renamed or removed
  Suggested fix: Change fetch('/api/order') to fetch('/api/orders')
  Confidence: HIGH (error message is explicit)

Developer applies fix and re-runs playback → ✓ Verified
```

### Phase 2 (Optional):
- Git context: Show commits that touched affected files
- Test coverage: Which code paths are being tested
- Performance: Detect if regression is also a performance issue

---

## Next Steps

1. **Tech Spec:** Architecture, data structures, API contracts, skill implementation
2. **QA Plan:** Unit tests (recording, playback, selector matching, root cause analysis), UAT steps
3. **Implementation Phase 1:** Basic recording/playback + skill scaffolding
4. **Implementation Phase 2:** Root cause analysis + git integration, test generation, large-scale storage


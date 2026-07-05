---
feature: Kaboom CI Infrastructure
doc_type: qa-plan
feature_id: feature-ci-infrastructure
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Kaboom CI Infrastructure

> Comprehensive testing strategy. Code testing + human UAT + edge case analysis + issues found.

## CRITICAL ISSUES IDENTIFIED (Pre-Implementation)

### Issue 1: CLARIFY Mock Scope Behavior
**Severity:** HIGH
**Confusion:** When multiple mocks for same endpoint, what happens?

**Current spec says:** "Later mock overwrites earlier (LIFO stack)"
**Problem:** If AI mocks `/api/order → 200`, then immediately mocks `/api/order → 500`, which one applies?

#### Questions that need answers:
- Does LIFO mean last-registered wins? Or first-registered wins?
- What if mocks are registered in rapid succession (race condition)?
- Should mock scope be per-test or per-request?
- Can AI query currently-registered mocks?

**Recommendation:** Add explicit mock ID system:
```
mock1 = await kaboom.mock({endpoint, response})  // Returns ID
mock2 = await kaboom.mock({endpoint, response})  // Returns ID
await kaboom.unmock(mock1)                       // Clear specific mock
```

**Test Case Needed:** Verify ordering + cleanup behavior under concurrency

---

### Issue 2: UNCLEAR Snapshot Size Limits
**Severity:** MEDIUM
**Confusion:** Spec says "keep snapshots <10MB each", but doesn't define what happens if exceeded.

#### Questions:
- Does system reject snapshot if >10MB? Or truncate?
- What's truncated? (DOM, network, logs?)
- What warning/error does AI see?
- Is there a hard limit, or just a warning at 10MB?

**Recommendation:** Define explicit behavior:
- **Hard limit:** 50MB per snapshot (fail with error if exceeded)
- **Soft warning:** Alert if snapshot > 10MB
- **Truncation strategy:** If DOM > 5MB, prune non-essential nodes (hidden elements, framework internals)

**Test Case Needed:** Capture large DOM (10K+ nodes), verify behavior

---

### Issue 3: CONFUSING Test Boundary Cleanup
**Severity:** MEDIUM
**Confusion:** Spec says "auto-cleanup on test end", but doesn't specify WHEN.

#### Questions:
- Is cleanup immediate (afterEach hook)?
- Is cleanup deferred (end of all tests)?
- If developer forgets to call `testBoundary()`, are ALL logs marked as test-specific?
- What if test crashes mid-execution?

#### Example that's confusing:
```javascript
test('test1', async ({ kaboom }) => {
  await kaboom.testBoundary('test1');
  // logs marked test1
  console.log('test1 log');
  // test1 crashes here
  // Is testBoundary cleaned up?
});

test('test2', async ({ kaboom }) => {
  // Are test2 logs still marked as test1? BUG?
  console.log('test2 log');
});
```

#### Recommendation:
- **Explicit cleanup:** `await kaboom.endBoundary()`
- **Or auto-cleanup:** Tie to test lifecycle (afterEach)
- **Or scoped:** `async with boundary('test1'): ...` (context manager)

**Test Case Needed:** Verify cleanup happens in all scenarios (success, failure, crash)

---

### Issue 4: AMBIGUOUS "Ephemeral" Snapshots
**Severity:** MEDIUM
**Confusion:** Spec says snapshots are "ephemeral" but doesn't say HOW LONG they live.

#### Questions:
- Do snapshots live for entire test suite execution?
- Do they live for single test only?
- Are they cleared on CI job end?
- Can AI query snapshots from previous tests?

#### Timeline that's confusing:
```
Test 1 runs
  → Snapshot captured ('snapshot1')
  → Test fails
  
AI queries snapshots
  → Can it get snapshot1? (probably yes)
  
Test 2 runs
  → Snapshot captured ('snapshot2')
  → AI queries snapshots
  → Can it get snapshot1? Or only snapshot2?
```

**Recommendation:** Define explicit lifetime:
```
Snapshot Lifetime:
- Born: When created (snapshot()) or test fails
- Accessible: For duration of test + 10 min after (for AI analysis)
- Cleared: On CI job end OR 10 min idle (whichever first)
- Not persisted: No on-disk storage (memory only)
```

**Test Case Needed:** Verify snapshots expire correctly, no memory leak

---

### Issue 5: RACE CONDITION with Async Operations
**Severity:** HIGH
**Confusion:** What happens if AI queues 5 async test re-runs concurrently?

#### Scenario:
```javascript
// AI does this (in parallel):
result1 = await kaboom.async(() => rerunTest('test1.spec.ts'))  // 30 sec
result2 = await kaboom.async(() => rerunTest('test2.spec.ts'))  // 30 sec
result3 = await kaboom.async(() => rerunTest('test3.spec.ts'))  // 30 sec
// ... all queued at once
```

#### Questions:
- Do they run in parallel (true concurrency) or queued (serial)?
- If parallel: Are they isolated (separate browser contexts)?
- If serial: What's the queue order?
- What happens if system runs out of resources?

#### Recommendation:
```
Concurrency Model:
- Max 3 concurrent test re-runs (configurable)
- If 5 queued, first 3 run, other 2 wait
- Queue is FIFO
- Each gets isolated test context (separate browser window)
- If system OOM, fail gracefully (return error to AI)
```

**Test Case Needed:** Queue 5 test re-runs, verify ordering + resource usage

---

### Issue 6: SILENT FAILURES in Mock Registration
**Severity:** HIGH
**Confusion:** What if mock endpoint is malformed?

#### Example:
```javascript
await kaboom.configure({
  action: 'mock',
  endpoint: 'api/order',  // Missing leading slash
  response: { statusCode: 200, body: {...} }
});

// Later:
fetch('/api/order')  // Will this match 'api/order'?
```

#### Questions:
- Does system validate endpoint format (must start with /)?
- Does system reject invalid endpoints, or silently ignore?
- What error message does AI see?
- Does mock partially match (e.g., 'order' matches '/api/order')?

**Recommendation:** Strict validation:
```
Mock Validation Rules:
1. Endpoint must start with '/'
2. Endpoint must not contain query string (e.g., '?foo=bar')
3. Method must be uppercase (GET, POST, etc.)
4. Response must have statusCode (200-599)
5. Response body must be JSON or string

If invalid:
  - Reject immediately
  - Return error: "Invalid endpoint format: 'api/order' (must start with /)"
```

**Test Case Needed:** Try invalid endpoints, verify error handling

---

### Issue 7: AMBIGUOUS Snapshot Restoration
**Severity:** MEDIUM
**Confusion:** What does "restore to snapshot" actually do?

#### Questions:
- Does it reload the page to match snapshot state?
- Does it revert DOM mutations?
- Does it restore network state (undo pending requests)?
- Or is it read-only (just for comparison)?

**Spec says:** "AI can restore to pre-failure state for comparison/analysis"
**Ambiguity:** "restore" could mean:
- Option A: Reload page to exact state
- Option B: Read-only comparison (snapshot shows what state WAS, but doesn't change current state)

**Recommendation:** Clarify as read-only:
```
Snapshot Restoration:
- Not a mutation (doesn't change current page state)
- Is a read operation (AI can see "what was the state before failure?")
- Use for comparison (before_snapshot vs after_snapshot)
- Example: 
  - Before snapshot: {DOM: loading spinner, network: pending}
  - After snapshot: {DOM: success button, network: completed}
  - AI compares: "After API completed, spinner removed and success shown"
```

**Test Case Needed:** Capture 2 snapshots, compare them, verify no mutations

---

### Issue 8: CONFUSING Test Boundary with Multiple Boundaries
**Severity:** MEDIUM
**Confusion:** Can a test have multiple boundaries?

#### Example:
```javascript
test('complex test', async ({ kaboom }) => {
  await kaboom.testBoundary('setup');
  // ... setup code ...
  
  await kaboom.testBoundary('main-flow');  // NEW boundary?
  // ... main test code ...
  
  await kaboom.testBoundary('cleanup');    // ANOTHER boundary?
  // ... cleanup code ...
});
```

#### Questions:
- Does second `testBoundary()` REPLACE the first, or create nested boundary?
- Can logs be tagged with multiple boundaries?
- What happens if AI queries with `boundary: 'setup'`?

**Recommendation:** Single boundary per test:
```
Rule: Only ONE active boundary per test
- Second call to testBoundary() replaces first
- Each test should have exactly ONE boundary (test name)
- If developer calls testBoundary() twice:
  - Log warning: "Replacing boundary 'setup' with 'main-flow'"
  - Only 'main-flow' logs are tagged
```

**Test Case Needed:** Try multiple boundaries, verify only last one applies

---

## Testing Strategy

### Code Testing (Automated)

#### Unit Tests: Snapshot Capture

- [ ] **Snapshot Creation:** Create snapshot, verify all data collected (DOM, network, logs, perf)
- [ ] **Snapshot Size:** Verify compression works (gzip reduces size ~80%)
- [ ] **Snapshot Limits:** Try snapshot >10MB, verify behavior (reject/truncate/warn)
- [ ] **DOM Pruning:** Verify non-essential nodes removed (framework internals)
- [ ] **Performance Metrics:** Verify LCP/FCP/load_time captured correctly
- [ ] **Timestamp Accuracy:** Verify ISO8601 timestamps match actual time

#### Unit Tests: Test Boundaries

- [ ] **Boundary Creation:** Create boundary, verify tags applied to logs
- [ ] **Boundary Cleanup:** Verify cleanup removes tag (next logs untagged)
- [ ] **Boundary Nesting:** Try nested boundaries, verify last one wins
- [ ] **Automatic Cleanup:** Verify cleanup happens in afterEach hook
- [ ] **Crash Recovery:** Test crashes mid-execution, verify boundary cleaned up
- [ ] **Noise Filtering:** Verify analytics/metrics logs excluded (80% reduction)
- [ ] **Boundary Query:** Filter logs by boundary, verify only matching logs returned

#### Unit Tests: Network Mocking

- [ ] **Mock Registration:** Register mock, verify stored in registry
- [ ] **Mock Matching:** Request `/api/order` with mock registered, verify intercepted
- [ ] **Mock Response:** Verify response body returned correctly
- [ ] **Mock Status Codes:** Try 200, 400, 500, verify all work
- [ ] **Partial Mocks:** Mock `/api/order`, request `/api/users`, verify `/api/users` hits real backend
- [ ] **Mock Cleanup:** Verify mocks cleared after test (next request hits real backend)
- [ ] **Mock Conflicts:** Register 2 mocks for same endpoint, verify ordering (LIFO or explicit error)
- [ ] **Invalid Endpoints:** Try `api/order` (missing /), verify rejected with error

#### Unit Tests: Async Execution

- [ ] **Async Operation:** Queue async operation, verify returns immediately (non-blocking)
- [ ] **Async Result:** Wait for async operation, verify result returned
- [ ] **Async Timeout:** Queue operation, wait 5 min, verify timeout + error
- [ ] **Async Status:** Query operation status (pending/running/completed/failed)
- [ ] **Async Concurrency:** Queue 5 async operations, verify max 3 concurrent (config)
- [ ] **Async Cleanup:** Verify operations cleaned up after completion (no memory leak)

#### Unit Tests: Output Formats

- [ ] **HAR Generation:** Collect network calls, generate HAR, verify format valid
- [ ] **HAR Parsing:** Parse generated HAR in browser DevTools, verify readable
- [ ] **SARIF Generation:** Create test violation, generate SARIF, verify format valid
- [ ] **SARIF GitHub:** Post SARIF to GitHub API, verify inline PR comments appear
- [ ] **Screenshot Capture:** Take screenshot at snapshot, verify PNG created
- [ ] **Screenshot Attachment:** Attach screenshot to CI artifact, verify accessible

#### Integration Tests: Full Flow

- [ ] **Flow 1: Test Fails → Snapshot Captured → AI Diagnoses → Fix Applied → Re-run Passes**
  - Run failing test
  - Verify snapshot auto-created
  - AI retrieves snapshot
  - AI identifies root cause
  - AI mocks slow endpoint
  - AI re-runs test in CI
  - Verify test passes
  
- [ ] **Flow 2: Works Locally, Fails in CI → Mismatch Detected → Mock Updated → Passes**
  - Run with PostgreSQL 12 schema (local)
  - Test passes
  - Run with PostgreSQL 14 schema (CI)
  - Test fails
  - AI detects schema mismatch
  - AI updates mock
  - AI re-runs
  - Test passes
  
- [ ] **Flow 3: Multiple Tests → Each Isolated → Snapshots Don't Leak**
  - Run test1 → snapshot1 created
  - Run test2 → snapshot2 created
  - Verify snapshot1 doesn't affect test2
  - Verify snapshot2 doesn't affect test1
  - Run test3 → snapshot3 created
  - Verify all 3 isolated (no cross-pollution)

- [ ] **Flow 4: Concurrent Async Ops → Queue → Execute → Results Correct**
  - Queue 5 async test re-runs
  - Verify first 3 execute immediately
  - Verify 4-5 queued
  - Wait for completion
  - Verify all results correct (no race conditions)

### Security/Compliance Testing

#### Sensitive Data Handling

- [ ] **Auth Token Redaction:** Snapshot captures request with Authorization header, verify token redacted in output
- [ ] **Password Masking:** Snapshot captures form input with password, verify masked
- [ ] **Session ID Hiding:** Snapshot captures session cookie, verify hidden
- [ ] **PII Redaction:** Snapshot captures user email, verify redacted (depends on redaction rules)

#### Isolation Testing

- [ ] **Test Isolation:** Run test1 with mock, run test2, verify mock not applied to test2
- [ ] **Boundary Isolation:** Run test1 with boundary, run test2, verify logs not tagged with test1's boundary
- [ ] **Async Isolation:** Run async op1, run async op2, verify results not mixed

#### Resource Testing

- [ ] **Memory Cleanup:** Run 100 snapshots, verify memory freed (ephemeral)
- [ ] **Snapshot Expiry:** Create snapshot, wait 10 min, verify expired/cleared
- [ ] **Concurrent Limits:** Try 10 async operations, verify max 3 concurrent (queue others)

---

## Human UAT Walkthrough

### Scenario 1: Developer Pushes Code, Test Fails, AI Fixes

#### Setup:
1. Clone Kaboom repo
2. Create simple Playwright test that fails (timeout)
3. Push to GitHub
4. GitHub Actions runs tests with Kaboom CI Infrastructure

#### Steps:

1. [ ] **Test Fails:** Test fails with "Timeout waiting for element"
2. [ ] **Snapshot Captured:** Kaboom auto-captures snapshot (DOM, network, logs)
3. [ ] **AI Analyzes:** AI retrieves snapshot via `observe({what: 'snapshots'})`
4. [ ] **AI Diagnoses:** AI says "Element selector timing out; API call pending"
5. [ ] **AI Proposes Fix:** AI suggests increasing timeout OR mocking slow endpoint
6. [ ] **AI Applies Fix:** AI mocks `/api/endpoint` to return immediately
7. [ ] **AI Re-runs:** AI re-runs test in CI (via async operation)
8. [ ] **Test Passes:** Test passes after mock applied
9. [ ] **GitHub Comment:** Auto-generated comment appears on PR with:
   - Root cause analysis
   - Proposed fix
   - Verification evidence (re-run passed)
10. [ ] **Engineer Reviews:** Engineer sees comment, approves, merges

**Expected Outcome:** Fix applied autonomously in <5 minutes (vs 45 minutes manual)

---

### Scenario 2: Works Locally, Fails in CI (Environment Mismatch)

#### Setup:
1. Create test that assumes PostgreSQL 12 schema
2. Local machine: PostgreSQL 12 (test passes)
3. CI container: PostgreSQL 14 (test fails with schema error)

#### Steps:

1. [ ] **Test Passes Locally:** Engineer runs `npm test` locally, passes
2. [ ] **Test Fails in CI:** GitHub Actions runs same test, fails: "Column 'user_metadata' not found"
3. [ ] **Snapshot Captured:** Kaboom captures snapshot showing DB error
4. [ ] **AI Diagnoses:** AI analyzes snapshot, detects schema mismatch
5. [ ] **AI Identifies Root Cause:** "PostgreSQL schema version mismatch (12 vs 14)"
6. [ ] **AI Proposes Fix:** Update mock to match PG14 schema
7. [ ] **AI Re-runs:** Test re-runs with updated mock
8. [ ] **Test Passes:** Test passes in CI (no local reproduction needed)
9. [ ] **GitHub Comment:** Shows fix with evidence
10. [ ] **No Manual Debugging:** Engineer never has to reproduce in CI

**Expected Outcome:** Schema mismatch fixed without local reproduction (saves 2+ hours)

---

### Scenario 3: Compliance Review (Enterprise)

#### Setup:
1. Enterprise team wants to enable AI to modify code
2. CISO requires: "Proof that fix works before merge"
3. Compliance team needs: "Full audit trail"

#### Steps:

1. [ ] **AI Proposes Fix:** AI suggests code change to test file
2. [ ] **Snapshot Provided:** AI includes snapshot context (what was broken)
3. [ ] **Fix Verified:** AI re-runs test in CI, confirms pass
4. [ ] **Evidence Generated:** HAR + SARIF + screenshots generated
5. [ ] **GitHub Comment:** Comment includes:
   - Root cause (with snapshot data)
   - Proposed fix
   - Verification proof (test re-run results)
   - Links to HAR, SARIF, screenshots
6. [ ] **Compliance Review:** CISO + Compliance team see:
   - Snapshot: "What was broken"
   - Fix: "What changed"
   - Verification: "Test passes after fix"
   - Audit Trail: "All steps logged"
7. [ ] **Approval:** Compliance team approves "AI-verified" fix
8. [ ] **Merge:** Engineer merges with confidence

**Expected Outcome:** Compliance team signs off on AI-modified code (enables enterprise adoption)

---

## Regression Testing

### Existing Features Must Not Break

- [ ] **Network Capture:** Verify normal network capture still works (non-mocked requests)
- [ ] **Log Capture:** Verify log capture still works (non-boundary logs)
- [ ] **observe() API:** Verify existing `observe()` calls work unchanged
- [ ] **configure() API:** Verify existing `configure()` calls work (except new actions)
- [ ] **interact() API:** Verify existing `interact()` calls work unchanged
- [ ] **Buffer Clearing:** Verify existing buffer clear operations work (v5.3)
- [ ] **Pagination:** Verify pagination still works for large datasets (v5.3)
- [ ] **Redaction:** Verify existing redaction rules still applied to snapshots
- [ ] **Performance:** Verify Kaboom doesn't degrade (snapshot capture <500ms overhead)

---

## Performance/Load Testing

### Latency Targets

- [ ] **Snapshot Capture:** <500ms (target: <300ms with compression)
- [ ] **Snapshot Retrieval:** <100ms (in-memory lookup)
- [ ] **Mock Registration:** <10ms (add to registry)
- [ ] **Mock Interception:** <5ms per request (fast HTTP interception)
- [ ] **Async Queue:** <50ms to queue operation (metadata only)
- [ ] **Async Execution:** Non-blocking (other requests still processed)
- [ ] **Output Generation:** <1 second (HAR + SARIF + screenshot)

### Scale Testing

- [ ] **Large Snapshots:** Capture 10K-node DOM, verify <10MB (with compression)
- [ ] **Many Mocks:** Register 100 mocks, verify all matched correctly
- [ ] **Many Boundaries:** Create 100 test boundaries, verify cleanup works
- [ ] **Concurrent Snapshots:** Queue 5 snapshot captures, verify not blocked
- [ ] **Concurrent Async:** Queue 10 async operations, verify max 3 concurrent

### Memory Testing

- [ ] **Snapshot Memory:** Capture 100 snapshots, verify memory freed after cleanup (no leak)
- [ ] **Mock Memory:** Register 1000 mocks, verify cleared properly
- [ ] **Boundary Memory:** Create 1000 boundaries, verify no memory growth
- [ ] **Long-Running CI:** Run 100-test suite, verify memory stable (no growth)

---

## Issues Found & Recommendations

### Summary Table

| ID | Issue | Severity | Status | Recommendation |
|----|----|----------|--------|-----------------|
| 1 | Unclear mock scope/ordering | HIGH | BLOCKING | Define explicit mock ID system |
| 2 | Vague snapshot size limits | MEDIUM | BLOCKING | Define hard limit + truncation strategy |
| 3 | Confusing boundary cleanup | MEDIUM | BLOCKING | Clarify lifecycle (immediate vs deferred) |
| 4 | Ambiguous "ephemeral" lifetime | MEDIUM | BLOCKING | Define exact snapshot TTL |
| 5 | Race conditions in async queuing | HIGH | BLOCKING | Define concurrency model (max 3 concurrent) |
| 6 | Silent failures in mock registration | HIGH | BLOCKING | Add strict validation + error messages |
| 7 | Ambiguous snapshot restoration | MEDIUM | BLOCKING | Clarify read-only (comparison only) |
| 8 | Multiple boundaries per test | MEDIUM | BLOCKING | Define: single boundary per test |

---

## Sign-Off

**Status:** BLOCKED - AWAITING CLARIFICATIONS

### Cannot proceed with implementation until these 8 critical issues are resolved:

1. [ ] **Mock Scope** — Define LIFO vs explicit mock IDs
2. [ ] **Snapshot Size Limits** — Define hard limit + truncation behavior
3. [ ] **Boundary Cleanup** — Define lifecycle (immediate vs deferred)
4. [ ] **Snapshot Lifetime** — Define TTL (10 min, or full test suite?)
5. [ ] **Async Concurrency** — Define max concurrent + queue order
6. [ ] **Mock Validation** — Define strict rules for endpoint format
7. [ ] **Snapshot Restore** — Clarify read-only (comparison) vs mutation
8. [ ] **Multiple Boundaries** — Define single boundary per test rule

**QA Sign-Off:** Cannot certify until issues clarified
**Next Step:** Product team + Engineers align on answers to 8 questions


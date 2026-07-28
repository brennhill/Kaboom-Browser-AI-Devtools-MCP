---
feature: Kaboom CI Infrastructure
status: proposed
tool: observe, configure, interact
mode: ci-cd, autonomous-repair, snapshots
version: 0.7.12
doc_type: product-spec
feature_id: feature-ci-infrastructure
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Kaboom CI Infrastructure

## Problem Statement

CI/CD pipelines are where AI debugging breaks down:

1. **Context Lost:** Test fails with error "Expected 'Success', got 'Loading'" — no DOM/network/logs visible
2. **Environment Gap:** Local ≠ CI (different Node versions, dependencies, database schemas, secrets)
3. **Reproduction Friction:** Engineers must clone repo, install deps, set up environment, reproduce locally (30-45 min)
4. **No Autonomy:** AI sees error text only; can't inspect browser state or propose verified fixes
5. **Verification Loop:** Fix applied locally, re-run in CI, back-and-forth (time-consuming)
6. **Knowledge Silos:** Each engineer debugs independently; no shared context or learnings

**Result:** CI test failures waste **$750,000/year per 100-person engineering team** in lost productivity.

## Solution

**Kaboom CI Infrastructure** brings full browser observability to CI pipelines:

1. **Snapshots:** Capture browser state (DOM, network, logs, console) when test fails
2. **Test Isolation:** Mark which logs/network calls are "test-specific" vs "background noise"
3. **Network Mocking:** AI controls API responses to test edge cases and error paths
4. **Async Execution:** Long-running operations (test re-runs) don't block MCP server
5. **Verification in CI:** AI re-runs tests in same container, with same hardware + environment
6. **Compliance Output:** HAR (network), SARIF (violations), screenshots for audit trails

**Result:** Test failures diagnosed + repaired autonomously in 5 minutes (vs 45 minutes manual).

## Why Kaboom vs Chrome DevTools Protocol

Chrome has DevTools Protocol (CDP), but Kaboom solves a different problem. Here's why both exist:

| Aspect | Chrome DevTools Protocol | Kaboom CI Infrastructure |
|--------|--------------------------|---------------------------|
| **Designed for** | Human developers with DevTools UI | AI agents operating autonomously |
| **Telemetry capture** | On-demand (you request what you want) | Auto-buffered (everything captured continuously) |
| **Storage** | Live streaming only | Ring buffers (queryable by time, URL, endpoint, error) |
| **Prerequisites** | Know what to capture BEFORE failure happens | Failure happens, telemetry already there |
| **Query style** | Imperative ("give me network") | Declarative ("show me failures on /api/checkout since 10:15") |
| **CI environment** | Requires special setup + management | Works headless, auto-loaded with extension |
| **Correlation** | Manual reading of logs | Automatic (failure → network call → code location) |
| **Semantic queries** | "CDP.Network.getAll()" | `observe({what: 'network_bodies', url: '/api/checkout', status_min: 400})` |

### Concrete Example:

With CDP alone:
```javascript
// You must KNOW to capture before test runs
page.on('response', response => {
  // Save response manually
  savedResponses.push(response);
});

// Test fails
// You search through savedResponses manually
// You try to correlate error to network call
// Slow, error-prone
```

With Kaboom:
```javascript
// Test fails (no pre-planning needed)
// AI queries: observe({what: 'network_bodies', url: '/api/checkout'})
// → Returns all responses to that endpoint in last 60 seconds
// → AI immediately sees: "POST /api/checkout returned 400, body: {error: 'missing field version'}"
// → Fast, precise, automated
```

### Why not just use Chrome CDP via MCP?

Chrome DevTools Protocol *can* be exposed via MCP, but Kaboom solves the operational problems:

1. **Auto-capture:** Kaboom buffers everything automatically. CDP requires you to set up listeners BEFORE the failure happens.
2. **No port forwarding:** Kaboom uses stdio (same as MCP). CDP requires exposing port 9222, which is a security/networking concern in CI.
3. **Launch overhead:** CDP requires launching Chrome with `--remote-debugging-port`, waiting for port readiness, then connecting. Kaboom auto-loads as an extension.
4. **Semantic queries:** Kaboom provides high-level queries (`observe({what: 'network_bodies', url: '/api/x', status_min: 400})`). CDP gives low-level primitives that agents must interpret.
5. **Single protocol:** Agents already use MCP (observe, configure, interact). Kaboom extends that same protocol. CDP would require learning a second protocol.
6. **Test boundaries:** Kaboom understands test structure (mark test start/end) and can filter noise automatically. CDP has no concept of "this log is test-specific."
7. **Verification context:** When a test fails, Kaboom knows exactly which snapshot/boundary is relevant. CDP shows all Chrome events without context.

**Bottom Line:** CDP is low-level infrastructure. Kaboom is a purpose-built high-level abstraction for AI agents in test automation. They work together, not instead of each other.

## Requirements

### Core Capabilities

#### 1. Test Snapshots
- **Capture:** Full browser state at named checkpoint (`kaboom.snapshot('name')`)
  - DOM tree (HTML structure)
  - Network calls (requests + responses, timing)
  - Console logs (info, warn, error, debug)
  - Performance metrics (load time, paint time, LCP)
  - User interactions timeline
- **Storage:** Snapshots stored ephemeral (in CI artifacts, not persisted)
- **Retrieval:** AI can query snapshots via `observe({what: 'snapshots'})`
- **Restore:** AI can restore to pre-failure state for comparison/analysis
- **Size:** Keep snapshots <10MB each (compress, prune non-essential data)

#### 2. Test Boundaries
- **Mark:** Developers call `kaboom.testBoundary('test-name')` to mark test-specific logs
- **Isolation:** Logs/network calls after boundary marked are tagged as "test-specific"
- **Filtering:** When AI queries logs, filter out background noise (analytics, scheduled jobs)
- **Noise Reduction:** 80%+ reduction in irrelevant logs (target: <50 relevant logs per test)
- **Cleanup:** Automatic cleanup when test boundary ends

#### 3. Network Mocking
- **Mock API:** AI calls `configure({what: 'mock', endpoint: '/api/x', response: {...}})` to mock response
- **Interception:** Kaboom intercepts requests to endpoint, returns mocked response
- **Error Testing:** Can mock error responses (4xx, 5xx, timeouts) to test error handling
- **Partial Mocks:** Can mock specific endpoints while others hit real backend
- **Verification:** Re-run test with mocked endpoint to verify fix works
- **Scope:** Applies only during test (automatically reverted after)

#### 4. Test Snapshots for Fixtures
- **Playwright:** Provide fixture to expose Kaboom API in Playwright tests
- **Jest:** Similar fixture for Jest test setup
- **Cypress:** Plugin for Cypress tests
- **Setup/Teardown:** Initialize Kaboom on test start, cleanup on test end
- **API Access:** Full Kaboom API available in tests (snapshot, boundary, mock, observe)

#### 5. Async Command Execution
- **Prevent Hangs:** Long-running operations (re-run test: 30+ sec) wrapped in async handler
- **Server Responsiveness:** MCP server never blocked; can handle multiple requests concurrently
- **Timeout Protection:** Operations that exceed timeout (5 min) fail gracefully
- **Concurrency:** Multiple tests can re-run simultaneously without contention

#### 6. CI Output Formats
- **HAR (HTTP Archive):**
  - All network calls + responses captured in HAR 1.2 format
  - Timing information (request duration, DNS lookup, TLS handshake)
  - Request/response bodies
  - Can view in browser DevTools HAR viewer
  - Attached to CI artifacts for debugging
  
- **SARIF (Static Analysis Results):**
  - Code violations + exact locations (file, line, column)
  - Severity levels (critical, high, medium, low, info)
  - GitHub/GitLab can display inline PR comments
  - Full audit trail for compliance
  
- **Screenshots:**
  - Visual state of page at failure moment
  - Multiple screenshots (if snapshots taken at multiple points)
  - Attached to GitHub/GitLab PR artifacts
  - Engineers can see exactly what DOM looked like when test failed

### Integration Points

#### Test Runners
- **Cypress:** `cy.kaboom.snapshot()`, `cy.kaboom.testBoundary()` commands
- **Playwright:** Fixture-based API in tests
- **Jest:** Similar fixture-based API
- **Vitest:** Same as Jest

#### CI Platforms
- **GitHub Actions:** Native support via GitHub Actions runner
- **GitLab CI:** Native support via GitLab Runner
- **CircleCI:** Via CircleCI Docker executor
- **Self-hosted:** Works with any CI that supports Docker containers with browser

#### AI Integration
- **Self-Healing Tests (#33):** Consumes snapshots for root-cause diagnosis
- **Context Streaming (#5):** Real-time snapshot push notifications
- **MCP Server:** Kaboom MCP server is extended with snapshot/mock/boundary commands

### Output & Reporting

#### AI Proposes Fix with Evidence
```
GitHub Comment (Auto-Generated):
┌─────────────────────────────────────┐
│ ✅ Fix Applied & Verified           │
│                                     │
│ ROOT CAUSE:                         │
│ API timeout (15s) > test timeout    │
│ (5s)                                │
│                                     │
│ PROPOSED FIX:                       │
│ - File: checkout.spec.ts:45         │
│ - Change: cy.get(...).click()       │
│ + Change: cy.get(..., {timeout:     │
│   20000}).click()                   │
│                                     │
│ VERIFICATION:                       │
│ ✅ Re-run passed                    │
│ ✅ No regressions (45 tests pass)   │
│ ✅ Snapshot diff analysis           │
│                                     │
│ [View HAR] [View Screenshot] [etc]  │
└─────────────────────────────────────┘
```

#### Engineer Reviews & Merges
- Comment includes links to evidence (HAR, screenshots, SARIF)
- Engineer reviews root cause + proposed fix
- Engineer approves + merges
- Fix is verified (already passed test re-run)

## Out of Scope

- **Custom CI Systems:** Focus on GitHub Actions, GitLab CI, CircleCI (others Phase 2)
- **Container Orchestration:** Assume Kubernetes support later
- **Parallel Test Isolation:** Multi-worker test parallelization deferred to Phase 2
- **Cost Optimization:** Artifact storage de-duplication deferred to Phase 2
- **Video Replay:** Screenshots + HAR sufficient for debugging
- **Real-time Collaboration:** Snapshots provide asynchronous sharing

## Success Criteria

- ✅ **Snapshot Capture:** <500ms to capture full browser state
- ✅ **Snapshot Restore:** <1 second to restore to pre-failure state
- ✅ **Test Isolation:** 80%+ reduction in irrelevant logs (noise filtering)
- ✅ **Network Control:** Any endpoint can be mocked; error paths testable
- ✅ **Fixture Integration:** Copy-paste Playwright fixture, zero friction adoption
- ✅ **Async Safety:** Long operations don't hang MCP server; timeout protection
- ✅ **Output Formats:** HAR, SARIF, screenshots generated automatically
- ✅ **CI Integration:** Works in GitHub Actions, GitLab CI, CircleCI (one-click setup)
- ✅ **Autonomous Repair:** Test failure → snapshot → AI diagnose → fix → verify (all in CI)
- ✅ **Performance:** Full cycle (capture + diagnose + repair + verify) <5 minutes

## User Workflows

### Workflow 1: Developer Pushes Code (Automatic)

```
1. Developer pushes feature branch
   ↓
2. GitHub Actions (CI) triggers test suite
   ↓
3. Test fails: "Expected 'Order Confirmed', got 'Loading'"
   ↓
4. [AUTOMATIC] Kaboom captures snapshot
   - DOM at failure moment
   - Network calls (pending POST /api/order)
   - Console logs
   - Performance metrics
   ↓
5. [AUTOMATIC] AI analyzes snapshot
   - "POST /api/order timeout (15s) > test timeout (5s)"
   - Confidence: 95%
   ↓
6. [AUTOMATIC] AI proposes fix + mocks endpoint
   - Change: timeout 5s → 20s
   - Mock: /api/order returns 200 with success payload
   ↓
7. [AUTOMATIC] AI re-runs test in CI
   - Test passes ✓
   ↓
8. [AUTOMATIC] GitHub PR comment appears
   - Root cause analysis
   - Proposed fix
   - Verification evidence
   ↓
9. Developer reviews comment, approves merge
```

### Workflow 2: "Works Locally, Fails in CI" (Environment Mismatch)

```
Developer:
  ✅ Tests pass locally (PostgreSQL 12, Node 18)
  
CI/CD:
  ❌ Tests fail (PostgreSQL 14, Node 20)
  Error: "Column 'user_metadata' not found"

With CI Infrastructure:
  1. Snapshot captured (database error in logs)
  2. AI diagnoses: "Schema mismatch: PG12 vs PG14"
  3. AI updates mock to reflect PG14 schema
  4. Test re-runs with updated mock ✓
  5. GitHub comment: "Fixed schema mismatch"
  
Result: No local reproduction needed; diagnosed + fixed in 5 minutes
```

### Workflow 3: Compliance Review (Enterprise)

```
Compliance Team Requirements:
  - "AI can modify code, but need proof it works"
  - "Audit trail: failure → fix → verification"
  - "Read-only approval: AI proposes, human approves"

CI Infrastructure Provides:
  1. Full snapshot context (failure captured)
  2. Proposed fix with rationale
  3. Test re-run proof (verification evidence)
  4. HAR + SARIF for audit trail
  5. GitHub comment for approval workflow
  
Result: Compliance team approves "AI-verified" fixes
```

## Examples

### Example 1: Selector Timeout

#### Test Code:
```javascript
test('checkout', async ({ page }) => {
  await page.goto('/checkout');
  await page.fill('input[name="email"]', 'test@example.com');
  await page.click('[data-testid="submit"]');  // ← Fails here
  await expect(page).toContainText('Success');
});

// Timeout: selector not found in 5 seconds
```

#### Snapshot Shows:
- DOM: Loading spinner visible, form inputs disabled
- Network: POST /api/order pending (not completed)
- Logs: "Processing payment..." (stuck)

#### AI Diagnosis:
"API call to /api/order is slow (15+ seconds). Test timeout is 5 seconds. Either:
1. Increase test timeout to 20 seconds, OR
2. Mock slow endpoint to return immediately"

#### AI Fix:
```javascript
// Mock the slow endpoint
await kaboom.configure({
  action: 'mock',
  endpoint: '/api/order',
  response: { statusCode: 200, body: { success: true } }
});

// Increase timeout as backup
await page.click('[data-testid="submit"]', { timeout: 20000 });
```

**Verification:** Test re-runs, passes ✓

---

### Example 2: Schema Mismatch (Local ≠ CI)

#### Local Database (working):
```
PostgreSQL 12:
  users table: (id, name, email, ...)
  No "user_metadata" column
```

#### CI Database (failing):
```
PostgreSQL 14:
  users table: (id, name, email, ...)
  user_info table: (user_id, metadata, ...)  ← NEW structure
```

#### Test Fails in CI:
```
Error: "Column 'user_metadata' does not exist in users"
```

#### Snapshot Shows:
- Error: Database error (column not found)
- Network: SELECT query failed
- Logs: "User query failed"

#### AI Diagnosis:
"Schema mismatch detected. Test expects old PG12 schema, but CI has PG14. Column 'user_metadata' moved to separate 'user_info' table."

#### AI Fix:
```javascript
// Update mock to reflect PG14 schema
const mockUser = {
  id: 1,
  name: 'Test User',
  email: 'test@example.com'
};

// Instead of user_metadata, use user_info relationship
await kaboom.configure({
  action: 'mock',
  endpoint: '/api/users/1',
  response: { 
    statusCode: 200, 
    body: { 
      ...mockUser, 
      user_info: { metadata: 'test' }  // ← Updated structure
    } 
  }
});
```

**Verification:** Test re-runs, passes ✓

---

## Integration & Dependencies

### Internal (Kaboom)
- **observe():** Extended to support `what: 'snapshots'`
- **configure():** Extended to support `action: 'mock'`, `action: 'boundary'`
- **interact():** Can restore to snapshot, re-run test
- **Buffer-Specific Clearing (v5.3):** Isolates test logs from background
- **MCP Server:** All new commands exposed via MCP

### External (Test Environment)
- **Cypress/Jest/Playwright:** Test runner event hooks
- **Chrome DevTools Protocol:** For DOM inspection + network interception
- **GitHub Actions/GitLab Runner:** CI container with browser

## Technical Approach (High-Level)

### Snapshot Capture
1. Developer calls `kaboom.snapshot('name')`
2. Kaboom collects: DOM, network calls, logs, perf metrics
3. Data compressed, stored in memory (ephemeral)
4. AI can retrieve via `observe({what: 'snapshots'})`

### Test Boundary
1. Developer calls `kaboom.testBoundary('test-name')`
2. Subsequent logs/network tagged as "test-specific"
3. Background noise (analytics, jobs) not tagged
4. AI queries only "test-specific" logs

### Network Mocking
1. AI calls `configure({what: 'mock', endpoint, response})`
2. Kaboom intercepts requests to endpoint
3. Returns mocked response instead of real request
4. Test continues with mocked data

### Async Execution
1. Wrap long operations in `kaboom.async(() => {...})`
2. Operations run without blocking MCP server
3. Results returned asynchronously

## Notes

- **Related Features:** Self-Healing Tests (#33), Context Streaming (#5)
- **Marketing Message:** "AI closes the feedback loop, even in CI/CD pipelines"
- **Enterprise Value:** Compliance teams approve "AI-verified" fixes
- **Competitive Advantage:** Only solution with autonomous repair in CI


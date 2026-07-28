---
feature: Kaboom CI Infrastructure
status: proposed
doc_type: tech-spec
feature_id: feature-ci-infrastructure
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Kaboom CI Infrastructure

> Plain language architecture. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

CI Infrastructure extends Kaboom with three new capabilities:

```
Test Execution (CI)
  ├─ Snapshot Capture (on test failure or checkpoint)
  ├─ Test Boundary Marking (isolate test logs from noise)
  └─ Network Mocking (AI controls API responses)
        ↓
    MCP Server (Kaboom)
  ├─ observe({what: 'snapshots'})
  ├─ configure({what: 'mock'})
  ├─ interact({what: 'restore'})
  └─ async() wrapper for long operations
        ↓
    AI Agent (Claude)
  ├─ Retrieve snapshot context
  ├─ Diagnose root cause
  ├─ Propose + apply fix
  └─ Verify via re-run
        ↓
    CI Output
  ├─ HAR (network trace)
  ├─ SARIF (violations)
  ├─ Screenshots
  └─ GitHub/GitLab PR comment
```

## Key Components

### 1. Snapshot Capture Engine
**What it does:** Records full browser + network + logs state at named checkpoints

#### Inside the engine:
- **DOM Serializer:** Walks DOM tree, captures structure + attributes (but prunes non-essential data like internal framework props)
- **Network Interceptor:** Captures requests + responses (matching existing Kaboom network logging)
- **Log Collector:** Gathers console.log/warn/error (already done by Kaboom)
- **Performance Metrics:** LCP, FCP, load time, paint duration (from Performance API)
- **Compression:** Snapshots gzipped before storage (reduce size ~80%)
- **Storage:** In-memory (ephemeral), tied to test execution lifecycle

#### Data model:
```
Snapshot {
  id: uuid
  name: string (e.g., "before-checkout")
  timestamp: ISO8601
  test_name: string
  
  dom: {
    html: string (serialized DOM)
    size_bytes: number
  }
  
  network: {
    requests: [{ url, method, headers, body, timestamp }]
    responses: [{ url, status, body, headers, duration_ms }]
  }
  
  logs: [{ level, message, timestamp }]
  
  performance: {
    lcp_ms: number
    fcp_ms: number
    load_time_ms: number
  }
}
```

### 2. Test Boundary Manager
**What it does:** Tags logs/network calls as "test-specific" or "background"

#### How it works:
- Developer calls `kaboom.testBoundary('test-name')`
- Kaboom adds tag to subsequent log entries: `{level, message, boundary: 'test-name'}`
- Developer ends test (Jest afterEach, Playwright teardown, etc.)
- Kaboom removes tag for next test
- When AI queries logs, can filter: `logs.filter(l => l.boundary === 'test-name')`

#### Noise filtering:
- Exclude log sources: /analytics, /metrics, /telemetry, /sentry, background jobs, timers
- Exclude patterns: "Analytics event", "Heartbeat", "Background sync"
- Result: 80%+ reduction in irrelevant logs (from 1000 lines → 50-100 relevant lines)

#### Data model:
```
LogEntry {
  level: 'info' | 'warn' | 'error' | 'debug'
  message: string
  timestamp: ISO8601
  boundary: string | null  // 'test-name' or null (background)
  source: string           // e.g., "console.log", "network", "framework"
}
```

### 3. Network Mocking Service
**What it does:** Intercepts HTTP requests, returns mocked responses

#### Implementation approach:
- Leverage existing Kaboom network interception (already intercepts Fetch + XHR)
- Add mock registry: `{endpoint: '/api/users', response: {...}}`
- On request to /api/users:
  - Check if endpoint in mock registry
  - If yes: return mocked response immediately
  - If no: forward to real backend
- Automatic cleanup: mocks cleared after test ends

#### Capabilities:
- Mock any endpoint (request path + method)
- Return custom status code (200, 400, 500, etc.)
- Return custom response body
- Simulate error paths (timeouts, partial responses)
- Delay response (simulate slow API)

#### Data model:
```
Mock {
  endpoint: string          // e.g., '/api/order'
  method: 'GET' | 'POST'... // optional, defaults to match any method
  response: {
    statusCode: number
    headers: {[key]: value}
    body: JSON | string
    delay_ms?: number
  }
  scope: 'test' | 'session'  // auto-reverted after test if scope='test'
}
```

### 4. Async Command Executor
**What it does:** Wraps long-running operations so they don't block MCP server

#### Problem it solves:
- `kaboom.rerunTest('checkout.spec.ts')` takes 30+ seconds
- If called synchronously, MCP server blocked (no other requests processed)
- LLM can't send new queries while waiting

#### Solution:
- Wrap in async handler: `await kaboom.async(() => rerunTest())`
- Operation runs in background thread/goroutine
- MCP server remains responsive
- Result returned asynchronously via callback or polling

#### Timeout protection:
- Operations exceeding 5 minutes fail gracefully (return error, don't hang)
- MCP server never permanently blocked

#### Concurrency:
- Multiple async operations can run concurrently
- Each gets its own execution context (no shared state)
- Results independent per operation

#### Data model:
```
AsyncOperation {
  id: uuid
  status: 'pending' | 'running' | 'completed' | 'failed'
  result: any
  error: string | null
  timeout_ms: number (default 5 min)
  created_at: ISO8601
  completed_at: ISO8601 | null
}
```

### 5. CI Output Generator
**What it does:** Creates HAR, SARIF, screenshots for GitHub/GitLab integration

#### HAR (HTTP Archive) Generator
- Collects all network calls from test execution
- Formats as HAR 1.2 standard
- Includes: timing, headers, bodies, compression
- Output: `.har` JSON file, attached to CI artifacts
- Tools: Can be viewed in browser DevTools HAR viewer

#### SARIF Generator (Static Analysis Results)
- Formats test failures + violations in SARIF format
- Each violation: file, line, column, message, severity
- GitHub/GitLab parse SARIF, show as inline PR comments
- Example:
  ```
  {
    "ruleId": "test-failed",
    "message": "Expected 'Success', got 'Loading'",
    "locations": [{
      "physicalLocation": {
        "artifactLocation": { "uri": "checkout.spec.ts" },
        "region": { "startLine": 45 }
      }
    }],
    "level": "error"
  }
  ```

#### Screenshot Capture
- Take screenshot at snapshot points (if visual snapshot enabled)
- Save PNG images
- Attach to CI artifacts
- Link in GitHub/GitLab PR

### 6. Fixture Adapter Layer
**What it does:** Exposes Kaboom API in test frameworks with zero friction

#### Playwright Fixture
```
// Define fixture in test setup
export const test = base.extend({
  kaboom: async ({ page }, use) => {
    const g = new KaboomClient();
    await g.initialize();
    await use(g);
    await g.cleanup();
  }
});

// Use in test
test('my test', async ({ page, kaboom }) => {
  await kaboom.snapshot('before');
  // ... test code ...
});
```

##### How it works:
- Base fixture extends @playwright/test
- Kaboom fixture initializes on test start
- Provides `kaboom` object with all APIs
- Auto-cleanup on test end

#### Jest Fixture
Similar pattern using Jest beforeEach/afterEach hooks.

#### Cypress Plugin
Expose as Cypress commands: `cy.kaboom.snapshot()`, `cy.kaboom.boundary()`.

## Data Flows

### Flow 1: Snapshot Capture
```
Test Code:
  await kaboom.snapshot('before-action')
    ↓
  Kaboom Extension (browser):
    - Serialize DOM
    - Collect network calls (from existing buffer)
    - Collect logs (from existing buffer)
    - Get performance metrics
    - Compress data
    ↓
  Kaboom Server (Go):
    - Receive snapshot data
    - Store in memory (ephemeral)
    - Return snapshot ID
    ↓
  Test continues...
    ↓
  If test fails:
    - Test failure handler triggered
    - Automatic snapshot created (if not already)
    - AI retrieves snapshot via observe({what: 'snapshots'})
```

### Flow 2: Network Mocking
```
AI Agent:
  await configure({
    action: 'mock',
    endpoint: '/api/order',
    response: { statusCode: 200, body: {...} }
  })
    ↓
  Kaboom Server:
    - Register mock in registry
    - Store: endpoint → response mapping
    ↓
  Test Code:
    - Makes request: fetch('/api/order', ...)
    ↓
  Kaboom Network Interceptor:
    - Check: is '/api/order' mocked?
    - Yes: return mocked response immediately
    - No: forward to real backend
    ↓
  Test continues with mocked data...
    ↓
  Test finishes:
    - Cleanup: clear mocks from registry
```

### Flow 3: Test Isolation (Boundary Marking)
```
Test Starts:
  await kaboom.testBoundary('checkout-test')
    ↓
  Kaboom Server:
    - Tag: subsequent logs with boundary='checkout-test'
    ↓
  Test Code:
    console.log('Starting checkout')  → tagged: boundary='checkout-test'
    fetch('/api/order')               → tagged: boundary='checkout-test'
    console.log('Order submitted')    → tagged: boundary='checkout-test'
    fetch('/analytics/track')         → NOT tagged (background service)
    ↓
  AI Queries Logs:
    observe({what: 'logs', boundary: 'checkout-test'})
    → Returns only checkout-test-specific logs (80% less noise)
```

### Flow 4: Async Execution
```
AI Agent:
  const result = await kaboom.async(() => {
    return rerunTest('checkout.spec.ts')
  })
    ↓
  Kaboom Server:
    - Create AsyncOperation {id, status: 'pending'}
    - Return operation ID immediately
    - Spawn background thread
    ↓
  Background Thread (non-blocking):
    - Run test (30+ seconds)
    - Update operation status: 'completed'
    - Store result
    ↓
  MCP Server:
    - Still responsive during test execution
    - Can handle other requests
    ↓
  AI Agent:
    - Polls or waits for result (callback)
    - Receives completed result
```

## Implementation Strategy

### Phase 1: Core Snapshot Infrastructure (Week 1-2)
1. **Snapshot Capture:** Extend existing network/log buffering to support snapshots
2. **Storage:** In-memory ephemeral storage (tied to test execution)
3. **Retrieval:** Add `observe({what: 'snapshots'})` support
4. **Compression:** Gzip snapshots to keep size reasonable

### Phase 2: Test Boundaries & Mocking (Week 2-3)
1. **Test Boundary:** Add tagging system to logs/network
2. **Network Mocking:** Extend network interceptor to support mocks
3. **Mock Registry:** Store + retrieve mocks per test

### Phase 3: Async & Fixtures (Week 3-4)
1. **Async Executor:** Wrap long operations in background threads
2. **Timeout Protection:** Fail gracefully after 5 minutes
3. **Playwright Fixture:** Create reusable fixture
4. **Jest/Cypress:** Adapt for other frameworks

### Phase 4: Output Formats & CI Integration (Week 4-5)
1. **HAR Generator:** Format network trace as HAR
2. **SARIF Generator:** Format violations as SARIF
3. **Screenshot Capture:** Capture + attach screenshots
4. **GitHub Actions Integration:** Auto-upload artifacts, post PR comments
5. **GitLab CI Integration:** Similar for GitLab

## Edge Cases & Assumptions

### Edge Case 1: Snapshot During Failed Network Request
**Scenario:** Test makes request, server returns 500, test fails
**Handling:** Snapshot captures partial request + 500 response (already in buffer)
**Result:** AI sees "API returned 500" clearly

### Edge Case 2: Mock Conflicts (Multiple Mocks for Same Endpoint)
**Scenario:** AI mocks `/api/order` twice with different responses
**Handling:** Later mock overwrites earlier (LIFO stack)
**Alternative:** Reject if already mocked (safer)

### Edge Case 3: Test Timeout During Snapshot Creation
**Scenario:** Snapshot() itself times out (DOM too large, network busy)
**Handling:** Return partial snapshot with warning
**Fallback:** Use most recent complete snapshot

### Edge Case 4: Concurrent Tests (Parallel Execution)
**Scenario:** Multiple tests run in parallel, each with own snapshots/mocks
**Handling:** Each test gets isolated context (separate boundaries, mock scopes)
**Assumption:** Test runner provides unique test ID per execution

### Edge Case 5: Sensitive Data in Snapshots
**Scenario:** Snapshot captures request body with auth token
**Handling:** Redact sensitive fields (Authorization, passwords, tokens)
**Implementation:** Use existing Kaboom redaction rules

### Edge Case 6: Network Mock + Real Backend Race
**Scenario:** Mock registered, but real request already in flight
**Handling:** Mock applies to new requests only, doesn't affect in-flight
**Result:** May see mix of real + mocked (test must be designed to handle)

### Edge Case 7: Test Boundary End Without Explicit Call
**Scenario:** Developer forgets to end test boundary
**Handling:** Auto-cleanup on test end (Jest afterEach, Playwright teardown)
**Result:** Subsequent test not polluted by previous test's boundary

## Assumptions

1. **Tests are isolated:** Each test run gets clean environment
2. **Network is intercepted:** Kaboom already intercepts Fetch/XHR
3. **Logs are centralized:** All logs feed into Kaboom buffer
4. **Snapshots ephemeral:** Stored in-memory, not persisted
5. **CI environment has browser:** Assumes containerized browser (Chrome, Playwright)
6. **Test failure triggers snapshot:** Automatic capture on assert failure

## Risks & Mitigations

### Risk 1: Snapshot Size Bloat
**Problem:** Large DOM (10K+ nodes) → snapshot > 100MB
**Mitigation:** Compression + DOM pruning (remove non-essential nodes)
**Target:** Keep snapshots <10MB each

### Risk 2: Mock Conflicts Break Tests
**Problem:** Overlapping mock definitions cause unexpected behavior
**Mitigation:** Clear mocks per test (auto-cleanup), explicit mock naming
**Test:** Verify mock cleanup doesn't leak between tests

### Risk 3: Async Operations Timeout
**Problem:** Test re-run exceeds 5-minute timeout (slow CI hardware)
**Mitigation:** Increase timeout to 10 minutes for enterprise; fail gracefully
**Monitoring:** Log timeout events, alert if common

### Risk 4: Network Mock Doesn't Cover All Requests
**Problem:** Test makes request to unmocked endpoint, hits real backend
**Mitigation:** Document which endpoints should be mocked, provide defaults
**Test:** Verify all external API calls are mocked or explicitly allowed

### Risk 5: Snapshot Data Inconsistency
**Problem:** Snapshot captured at different times shows different state
**Mitigation:** Snapshot lifecycle tied to test execution; clear on test end
**Test:** Verify snapshots don't leak between tests

## Dependencies

### Internal (Kaboom)
- **Network Interception:** Already working, extend with mock support
- **Log Buffering:** Already working, extend with boundary tagging
- **MCP Server:** Extend with new commands (snapshot, mock, boundary)
- **Extension:** Already has DOM access, extend with snapshot logic

### External (Test Environment)
- **Chrome DevTools Protocol:** For DOM serialization + network control
- **Test Framework Hooks:** Jest beforeEach/afterEach, Playwright hooks, Cypress plugins
- **CI Platform APIs:** GitHub REST API (for PR comments), GitLab APIs

## Performance Considerations

- **Snapshot Capture:** <500ms (target: <300ms with compression)
- **Snapshot Retrieval:** <100ms (in-memory lookup)
- **Mock Registration:** <10ms (add to registry)
- **Network Mocking Overhead:** <5ms per request (fast interception)
- **Async Operation:** Non-blocking, doesn't affect other requests
- **Snapshot Storage:** Ephemeral, auto-cleanup on test end (no memory leak)

## Security Considerations

- **Snapshot Data:** May contain auth tokens, passwords, user data
  - Mitigation: Redact sensitive fields before storage
  - Comply with existing Kaboom redaction rules
  
- **Mock Responses:** Could be manipulated by attacker
  - Mitigation: Mocks only apply to test context, not production
  - Test isolation ensures mocks don't affect real users

- **Async Operations:** Could expose system internals if error messages verbose
  - Mitigation: Catch errors, return sanitized error messages
  - Don't expose stack traces to AI


---
status: proposed
scope: feature/test-execution-capture
ai-priority: high
tags: [v7, testing, test-integration, ears]
relates-to: [../custom-event-api.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-test-execution-capture
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Execution Capture

## Overview
Test Execution Capture integrates Kaboom directly into test frameworks (Jest, Playwright, Mocha, Pytest, etc.), capturing the complete test execution context and correlating it with frontend behavior and backend logs. When a test runs, Kaboom records not just pass/fail, but the entire lifecycle: setup, page loads, user interactions, assertions, and cleanup. A failed assertion doesn't just show "expect(x).toBe(5)" in test output—it shows the exact browser state, network requests, backend logs, and custom events that led to that failure. Developers debug failing tests 10x faster because they see the full stack trace: browser state + network + backend.

## Problem
Current test runners (Jest, Playwright) provide excellent test output and failure messages, but are isolated from system observability:
- A test fails because a button doesn't appear, but developers don't see the network request that failed to fetch the page
- An API test times out, but developers can't correlate it with backend logs to see why the server hung
- Test flakiness is mysterious because there's no visibility into intermediate states
- Debugging requires manually re-running the test with logging enabled, then correlating with other systems
- Test infrastructure events (setup, teardown, retry) are invisible to the observation system

## Solution
Test Execution Capture integrates test frameworks with Kaboom's event system:
1. **Test Framework Integration:** Adapters for Jest, Playwright, Mocha, Pytest emit test lifecycle events (test:start, test:end, assertion)
2. **Session Correlation:** Each test run establishes a Kaboom session with a unique trace_id
3. **Complete Context Capture:** Frontend state, network requests, backend logs, and custom events are all tagged with the test's trace_id
4. **Assertion Linking:** Failed assertions emit events with stack traces, expected/actual values
5. **Unified Report:** On test completion, Kaboom generates a report showing the entire execution timeline

## User Stories
- As a QA engineer, I want to see the complete browser state and network requests when a test fails so that I can immediately understand the cause
- As a frontend developer, I want failed E2E tests to include backend logs so that I can see if the failure was due to a service error
- As a DevOps engineer, I want to verify tests run in isolation and don't interfere with each other through shared state
- As a test maintainer, I want to analyze test execution time and identify slow operations (setup, assertions, navigation)
- As a CI/CD engineer, I want to automatically capture Kaboom sessions on test failure and attach them to CI logs

## Acceptance Criteria
- [ ] Test framework adapters (Jest, Playwright) emit test lifecycle events
- [ ] Each test run has unique trace_id, correlation across all observables (network, logs, events)
- [ ] Failed assertions emit event with stack trace, expected, actual, and browser state at failure
- [ ] Test setup/teardown time captured and reported
- [ ] Each test's observables (logs, network, events) are queryable as a group
- [ ] Kaboom generates HTML report on test completion with timeline, flamegraph, and links to detailed logs
- [ ] Performance: test instrumentation adds <10% overhead
- [ ] Support for test retries: each retry is a separate trace_id, linked to parent test

## Not In Scope
- Test execution (that's the test framework's job)
- Test authoring (users write tests normally)
- Result aggregation across multiple test runs (per-run analysis only)
- Visual regression testing (that's Percy, etc.)
- Test optimization recommendations

## Data Structures

### Test Lifecycle Event
```go
type TestEvent struct {
    Timestamp   time.Time
    Type        string          // "test:started", "test:completed", "assertion"
    TraceID     string          // Unique per test run
    TestName    string          // "should render dashboard"
    FilePath    string          // "src/__tests__/dashboard.spec.js"
    SuiteName   string          // "Dashboard"
    Status      string          // "passed", "failed", "skipped"
    Duration    int64           // milliseconds
    ErrorMsg    string          // Failure message if status == "failed"
    StackTrace  string          // Full stack trace
    RetryCount  int             // 0 for first run

    // For assertions specifically:
    AssertionMsg string          // "expect(x).toBe(5)"
    Expected     string          // Serialized expected value
    Actual       string          // Serialized actual value
    AssertFile   string          // File + line of assertion
}
```

### Test Session Report
```json
{
  "trace_id": "test-run-12345",
  "test_name": "should process payment",
  "file_path": "src/e2e/checkout.spec.js",
  "status": "failed",
  "duration_ms": 2340,
  "started_at": "2026-01-31T10:15:23.456Z",
  "completed_at": "2026-01-31T10:15:25.796Z",
  "events": [
    {
      "timestamp": "2026-01-31T10:15:23.500Z",
      "type": "network:request",
      "method": "GET",
      "url": "/api/products",
      "status": 200,
      "duration_ms": 45
    },
    {
      "timestamp": "2026-01-31T10:15:23.800Z",
      "type": "backend:log",
      "level": "INFO",
      "message": "Product listing retrieved",
      "service": "api-server"
    }
  ],
  "failure": {
    "assertion": "expect(button.textContent).toBe('Pay Now')",
    "expected": "Pay Now",
    "actual": "Loading...",
    "stack_trace": "at checkout.spec.js:23:5"
  }
}
```

## Examples

### Example 1: E2E Test with Network Failure
#### Test code:
```javascript
it('should process payment', async () => {
  await page.goto('http://localhost:3000/checkout');
  await page.fill('input[name=amount]', '99.99');
  await page.click('button.pay');
  await expect(page).toHaveSelector('.success-message');
});
```

#### Kaboom Timeline (on failure):
```
[10:15:23.100] test:started (should process payment, trace-123)
[10:15:23.200] page:load (checkout page)
[10:15:23.300] network:request GET /api/config (200)
[10:15:23.400] backend:log INFO: Config loaded
[10:15:23.500] user:input fill amount field
[10:15:23.600] user:click pay button
[10:15:23.700] network:request POST /api/payments (500)
[10:15:23.750] backend:log ERROR: Payment gateway timeout
[10:15:24.000] page:timeout waiting for .success-message
[10:15:24.100] assertion:failed (expected: .success-message, actual: none)
[10:15:24.200] test:completed (FAILED)
```

**Developer sees:** "Oh, the payment API returned 500 because the gateway timed out. I should increase the timeout or check gateway health."

### Example 2: Test Setup/Teardown Timing
#### Test framework adapter emits:
```javascript
emit({ type: 'test:setup:started', trace_id, file_path });
// ... setup code ...
emit({ type: 'test:setup:completed', trace_id, duration_ms: 450 });

emit({ type: 'test:started', trace_id, test_name });
// ... test code ...
emit({ type: 'test:completed', trace_id, status, duration_ms });

emit({ type: 'test:teardown:started', trace_id });
// ... teardown code ...
emit({ type: 'test:teardown:completed', trace_id, duration_ms: 100 });
```

Kaboom report shows: "Setup took 450ms, test took 1800ms, teardown took 100ms. Consider optimizing setup (database seeding?)."

### Example 3: Assertion with Screenshot
#### Test fails:
```javascript
await expect(page).toHaveSelector('.order-confirmation');
// → assertion:failed event emitted with:
//   - Expected: selector '.order-confirmation' exists
//   - Actual: selector not found
//   - page.url(): 'http://localhost/checkout?error=payment_failed'
//   - Backend logs show: ERROR: Payment processing failed
```

Kaboom captures the entire context, making debugging instant.

## Integration Points
- **Jest:** `expect()` overrides to emit assertion events
- **Playwright:** Page event listeners for load, request, response
- **Mocha:** Hook listeners (before, after, it)
- **Cypress:** Command logging and error handlers
- **Pytest:** Pytest hooks (setup, teardown, assert)

## MCP Changes
New observable type:
```javascript
observe({
  what: 'test-execution',
  trace_id: 'test-run-12345',  // Query specific test run
  include: ['network', 'backend-logs', 'assertions', 'timings']
})
```

New report generation:
```javascript
generateTestReport({
  trace_id: 'test-run-12345',
  format: 'html'  // Returns URL to HTML report in Kaboom UI
})
```

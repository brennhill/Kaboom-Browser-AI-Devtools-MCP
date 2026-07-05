---
status: proposed
scope: feature/test-execution-capture
ai-priority: high
tags: [v7, testing, test-integration]
relates-to: [product-spec.md, ../custom-event-api/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-test-execution-capture
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Execution Capture — Technical Specification

## Architecture

### Data Flow
```
Jest/Playwright/Mocha (test framework)
    ↓ Adapter emits lifecycle events
Custom Event API (trace_id = test_id)
    ↓ events indexed by trace_id
Kaboom Event Store
    ↓ on observe('test-execution', trace_id)
Test Report Generator
    ↓ HTML timeline + flamegraph
Frontend Extension / CI
```

### Components
1. **Test Framework Adapters** (npm packages)
   - `@kaboom/jest-adapter`: Hook into Jest test lifecycle
   - `@kaboom/playwright-adapter`: Hook into Playwright page events
   - `@kaboom/mocha-adapter`: Hook into Mocha hooks and tests
   - Emit test:started, test:completed, assertion:failed, etc.
   - Each test gets unique trace_id

2. **Test Session Correlator** (`server/test-session.go`)
   - Tracks active test runs by trace_id
   - Associates all events (network, logs, custom) with test's trace_id
   - Collects metadata: file path, suite name, retry info
   - Generates report on test completion

3. **Test Report Generator** (`server/reports/test-report.go`)
   - Generates HTML report with:
     - Timeline view (events in chronological order)
     - Flamegraph (time spent in different phases: setup, test, assertion, teardown)
     - Failure details (assertion, expected, actual, screenshot if available)
     - Network waterfall
     - Backend logs correlated with test timeline
   - Supports exporting to CI systems

### Test Session Store
```go
type TestSession struct {
    TraceID         string
    TestName        string
    FilePath        string
    SuiteName       string
    Status          string          // "running", "passed", "failed", "skipped"
    StartedAt       time.Time
    CompletedAt     time.Time
    DurationMS      int64
    RetryCount      int
    AssociatedEvents []string        // Indexes into custom-events store
    Failure         *TestFailure
}

type TestFailure struct {
    AssertionMsg    string
    Expected        string
    Actual          string
    StackTrace      string
    Screenshot      []byte          // PNG/JPEG if available
    FailureTime     time.Time
}
```

## Implementation Plan

### Phase 1: Core Framework (Week 1)
1. Define test lifecycle event schema
2. Implement Jest adapter (`@kaboom/jest-adapter`)
   - Hook into beforeEach, afterEach, test() callback
   - Emit test:started, test:completed, assertion:failed
   - Allocate trace_id, propagate to browser via window.kaboom_trace_id
3. Test with simple Jest project

### Phase 2: Browser Integration (Week 2)
1. Frontend extension listens for `window.kaboom_trace_id`
2. Include trace_id in all frontend events (network, dom, logs)
3. Frontend SDK sends trace_id with all custom events
4. Implement correlation in Kaboom server

### Phase 3: Report Generation (Week 3)
1. Implement test session store (in-memory, indexed by trace_id)
2. Implement HTML report generator
3. Create timeline view (D3.js or similar)
4. Create flamegraph for timing analysis
5. Link assertion failure to relevant network requests and backend logs

### Phase 4: Additional Adapters (Week 4)
1. Implement Playwright adapter
2. Implement Mocha adapter
3. Test with real test projects
4. Performance profiling (ensure <10% overhead)

## API Changes

### Jest Adapter Example
```javascript
// In jest.config.js
module.exports = {
  setupFilesAfterEnv: ['@kaboom/jest-adapter/setup.js'],
};

// Adapter automatically hooks into:
// - expect() for assertions
// - test() for test lifecycle
// - Emits to Kaboom via MCP or HTTP
```

### Playwright Adapter Example
```javascript
const { kaboom } = require('@kaboom/playwright-adapter');

test.beforeEach(async ({ page }) => {
  const traceId = await kaboom.startTest(page, 'should load page');
  page.context().traceId = traceId;
});

test.afterEach(async ({ page }) => {
  await kaboom.endTest(page, page.context().traceId, 'passed');
});
```

### Backend SDK Example (Node.js)
```javascript
const kaboom = require('@kaboom/sdk');

// Middleware to extract trace_id from request headers
app.use((req, res, next) => {
  const traceId = req.headers['x-test-trace-id'] || req.headers['x-trace-id'];
  res.locals.traceId = traceId;
  next();
});

// Log with trace_id
logger.info('Processing request', { traceId: res.locals.traceId });

// Emit custom events with trace_id
kaboom.emit('payment:authorized', { traceId: res.locals.traceId, amount: 99.99 });
```

### Test Report API
```go
// In handlers.go
func handleTestReport(req *TestReportRequest) (*TestReportResponse, error) {
    // req.TraceID: "test-run-12345"
    // req.Format: "html" or "json"

    // Returns:
    // - HTML report with timeline, flamegraph, failure details
    // - JSON with all events and metadata
}
```

## Code References
- **Jest adapter:** `/Users/brenn/dev/kaboom/js-adapters/jest-adapter/src` (new)
- **Playwright adapter:** `/Users/brenn/dev/kaboom/js-adapters/playwright-adapter/src` (new)
- **Test session store:** `/Users/brenn/dev/kaboom/server/test-session.go` (new)
- **Report generator:** `/Users/brenn/dev/kaboom/server/reports/test-report.go` (new)
- **Backend SDK:** `/Users/brenn/dev/kaboom/sdk/node-sdk/src` (modified)
- **Go SDK:** `/Users/brenn/dev/kaboom/sdk/go-sdk/sdk.go` (modified)

## Performance Requirements
- **Adapter overhead:** <10% slowdown on test execution
- **Jest assertion hook:** <1ms per assertion
- **Trace_id propagation:** <0.1ms per event
- **Report generation:** <500ms for 1000-event test
- **Memory per test session:** <5MB

## Testing Strategy

### Unit Tests (Adapters)
- Test Jest adapter hooks with mock tests
- Test trace_id allocation and propagation
- Test assertion capture (pass/fail/error)
- Test exception handling

### Integration Tests
- Run real Jest/Playwright tests with adapter enabled
- Verify all events have correct trace_id
- Verify session metadata captured correctly
- Verify report generation works

### E2E Tests
- Run full test suite with Kaboom enabled
- Verify frontend and backend events correlated
- Verify HTML report is generated and links work
- Test with real backend services

### Performance Tests
- Measure Jest execution time with/without adapter
- Measure memory footprint of test session store
- Profile report generation time

## Dependencies
- **Test frameworks:** Jest 27+, Playwright 1.20+, Mocha 9+
- **Browser extension:** Already provides window.kaboom API
- **Custom event API:** Must be enabled to emit test events
- **Backend SDKs:** Must propagate trace_id from headers

## Risks & Mitigation
1. **Adapter overhead slowing tests**
   - Mitigation: Async event emission, batch processing
2. **Test isolation issues** (shared state between tests)
   - Mitigation: Each test gets unique trace_id, clear session state on teardown
3. **Report generation performance** (large test suites)
   - Mitigation: Stream report generation, lazy-load details
4. **Flaky tests from adapter code**
   - Mitigation: Comprehensive testing of adapter itself, error boundaries
5. **False negatives** (adapter failure masks test failure)
   - Mitigation: Graceful degradation, test adapter itself

## Backward Compatibility
- Adapters are opt-in (setup files)
- Kaboom works without adapters (no test integration)
- No changes to test code required (adapters are transparent)
- Existing tests run unmodified

## Future Extensions
- CI/CD integration: Attach Kaboom reports to GitHub Actions, Jenkins, etc.
- Test flakiness analysis: Detect which operations cause flakiness
- Performance budgets: Assert test execution time is within limits
- Visual regression: Screenshot comparison (integration with Percy)

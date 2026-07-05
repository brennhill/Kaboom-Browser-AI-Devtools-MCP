---
status: proposed
scope: feature/test-execution-capture
ai-priority: high
tags: [v7, testing]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-test-execution-capture
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Execution Capture — QA Plan

## Test Scenarios

### Scenario 1: Simple Test Lifecycle
#### Setup:
- Jest with Kaboom adapter installed
- Simple test: `it('should add 2+2', () => expect(2+2).toBe(4))`

#### Steps:
1. Run Jest
2. Adapter emits test:started event
3. Test executes
4. Adapter emits assertion event
5. Test completes, adapter emits test:completed
6. Query `observe({what: 'test-execution', trace_id: 'test-123'})`

#### Expected Result:
- All test events captured with correct trace_id
- Assertion shows: expected=4, actual=4, passed=true
- Timeline shows setup, test, teardown phases

#### Acceptance Criteria:
- [ ] test:started event has trace_id
- [ ] assertion event captured with expected/actual
- [ ] test:completed event has status=passed
- [ ] Events queryable via observe()

---

### Scenario 2: Failed Assertion with Details
#### Setup:
- Jest test that fails: `expect(5).toBe(10)`

#### Steps:
1. Run Jest
2. Assertion fails
3. Adapter captures failure with stack trace
4. Query test execution by trace_id
5. Verify failure details are complete

#### Expected Result:
- Failure captured with:
  - Assertion message: "expect(5).toBe(10)"
  - Expected: 10
  - Actual: 5
  - Stack trace with file and line number
- Status shows failed

#### Acceptance Criteria:
- [ ] assertion:failed event has expected, actual, message
- [ ] Stack trace includes file and line number
- [ ] Test status is failed
- [ ] Report shows failure clearly

---

### Scenario 3: E2E Test with Network Requests
#### Setup:
- Playwright test with network activity
- Test loads page, clicks button, waits for response

#### Steps:
1. Playwright adapter installed
2. Test starts (trace_id assigned)
3. page.goto() triggers network requests
4. Backend receives request, logs with trace_id header
5. Test completes
6. Query test execution including network events

#### Expected Result:
- Network requests have same trace_id as test
- Backend logs have trace_id
- Timeline shows: page load → network request → backend processing → response
- All correlated under same trace_id

#### Acceptance Criteria:
- [ ] Network requests have trace_id
- [ ] Backend logs have trace_id
- [ ] All events grouped under test's trace_id
- [ ] Timeline view shows complete flow

---

### Scenario 4: Multiple Assertions in Single Test
#### Setup:
- Jest test with 5 assertions

#### Steps:
1. Test runs with 5 expect() calls
2. All assertions pass
3. Query test execution
4. Verify all 5 assertions captured

#### Expected Result:
- 5 assertion events emitted
- Each shows expected, actual, pass/fail
- Total duration accounts for all assertions
- Timeline shows assertion order

#### Acceptance Criteria:
- [ ] All 5 assertions emitted as events
- [ ] Each assertion event is ordered
- [ ] Test status is passed (all assertions passed)
- [ ] Flamegraph shows time spent per assertion

---

### Scenario 5: Test Retry on Failure
#### Setup:
- Jest with retry configured
- Flaky test that fails first, passes on retry

#### Steps:
1. First test run fails
2. Adapter emits failure event, retry count = 0
3. Jest retries test
4. Second run passes
5. Adapter emits success event, retry count = 1
6. Query both runs by parent trace_id

#### Expected Result:
- Both runs captured separately
- First run shows failure, second shows pass
- Linked via parent test_id
- Final status is passed (retry succeeded)

#### Acceptance Criteria:
- [ ] Each retry gets unique trace_id
- [ ] Both runs are linked to parent test
- [ ] Retry count incremented on retry
- [ ] Final status reflects last run

---

### Scenario 6: Test with Setup and Teardown
#### Setup:
- Jest with beforeEach/afterEach hooks
- Setup connects to test database (200ms)
- Test runs (100ms)
- Teardown cleans database (150ms)

#### Steps:
1. Test runs
2. Adapter captures all phases: setup, test, teardown
3. Each phase timestamped
4. Query test execution
5. Generate flamegraph showing phase breakdown

#### Expected Result:
- Setup time: ~200ms captured
- Test time: ~100ms captured
- Teardown time: ~150ms captured
- Flamegraph shows 200ms (setup) + 100ms (test) + 150ms (teardown)

#### Acceptance Criteria:
- [ ] Each phase has separate event
- [ ] Phase timings are accurate (±10ms)
- [ ] Flamegraph shows 3 phases with correct proportions
- [ ] Report suggests optimization (setup is slow)

---

### Scenario 7: Trace ID Propagation to Backend
#### Setup:
- Frontend test: Playwright
- Backend: Node.js with Kaboom SDK
- Test triggers API request

#### Steps:
1. Playwright test starts with trace_id = "test-abc"
2. Frontend extension sets header: X-Test-Trace-ID: test-abc
3. Backend receives request, logs with trace_id
4. Backend emits custom event with trace_id
5. Query all events with trace_id = test-abc

#### Expected Result:
- Frontend network request has trace_id header
- Backend logs have trace_id field
- Backend custom events have trace_id
- All events grouped in timeline

#### Acceptance Criteria:
- [ ] X-Test-Trace-ID header sent to backend
- [ ] Backend logs include trace_id
- [ ] Backend events have trace_id
- [ ] Timeline shows all events correlated

---

### Scenario 8: Report Generation
#### Setup:
- Test execution completed
- Multiple events captured (100+ events over 5 second test)

#### Steps:
1. Call `generateTestReport({trace_id: "test-123", format: "html"})`
2. Kaboom generates HTML report
3. Open report in browser
4. Verify all sections present and correct

#### Expected Result:
- HTML report generated with:
  - Test name, status, duration
  - Timeline view (events in chronological order)
  - Flamegraph (time distribution)
  - Network waterfall
  - Backend logs section
  - Failure details (if any)
- All sections clickable and interactive

#### Acceptance Criteria:
- [ ] Report generates in <500ms
- [ ] Timeline view shows all events
- [ ] Flamegraph is accurate
- [ ] Network waterfall shows requests in order
- [ ] Report is downloadable/shareable

---

### Scenario 9: Multiple Concurrent Tests
#### Setup:
- Jest running with `--maxWorkers=4` (4 concurrent tests)
- 4 different test files running simultaneously

#### Steps:
1. Jest runs 4 tests in parallel
2. Each test has unique trace_id
3. Adapter for each test emits events independently
4. Query events for each trace_id separately

#### Expected Result:
- Events don't mix between tests
- Each test's events are correctly isolated
- No race conditions in event emission
- Each test's report is independent

#### Acceptance Criteria:
- [ ] Each test has unique trace_id
- [ ] Events don't leak between tests
- [ ] Query by trace_id returns only that test's events
- [ ] No data corruption under concurrent load

---

### Scenario 10: Adapter Overhead Measurement
#### Setup:
- Same test suite run without and with Kaboom adapter

#### Steps:
1. Run test suite 3 times without adapter
2. Measure average execution time (baseline)
3. Install Kaboom adapter
4. Run same test suite 3 times with adapter
5. Calculate overhead percentage

#### Expected Result:
- Overhead <10% (test suite slowdown)
- Memory footprint <5MB per test session
- No test failures due to adapter

#### Acceptance Criteria:
- [ ] Adapter overhead <10%
- [ ] No memory leaks (<5MB per test)
- [ ] 0 test failures attributable to adapter
- [ ] Adapter handles errors gracefully

---

## Acceptance Criteria (Overall)
- [ ] All 10 scenarios pass
- [ ] Trace ID propagation works frontend-to-backend
- [ ] Report generation is fast and complete
- [ ] Multiple concurrent tests don't interfere
- [ ] Adapter overhead is <10%
- [ ] Failed assertions show complete context

## Test Data

### Fixture: Passed Test Events
```json
[
  {
    "type": "test:started",
    "trace_id": "test-xyz",
    "test_name": "should render dashboard",
    "file_path": "src/__tests__/dashboard.spec.js",
    "suite_name": "Dashboard",
    "timestamp": "2026-01-31T10:15:23.000Z"
  },
  {
    "type": "assertion",
    "trace_id": "test-xyz",
    "assertion_msg": "expect(true).toBe(true)",
    "expected": "true",
    "actual": "true",
    "passed": true,
    "timestamp": "2026-01-31T10:15:23.050Z"
  },
  {
    "type": "test:completed",
    "trace_id": "test-xyz",
    "status": "passed",
    "duration_ms": 150,
    "timestamp": "2026-01-31T10:15:23.150Z"
  }
]
```

### Fixture: Failed Test Events
```json
[
  {
    "type": "test:started",
    "trace_id": "test-abc",
    "test_name": "should load user profile",
    "timestamp": "2026-01-31T10:15:30.000Z"
  },
  {
    "type": "network:request",
    "trace_id": "test-abc",
    "method": "GET",
    "url": "/api/user/123",
    "status": 404,
    "duration_ms": 45,
    "timestamp": "2026-01-31T10:15:30.100Z"
  },
  {
    "type": "assertion",
    "trace_id": "test-abc",
    "assertion_msg": "expect(response.status).toBe(200)",
    "expected": "200",
    "actual": "404",
    "passed": false,
    "stack_trace": "at user.spec.js:45:5",
    "timestamp": "2026-01-31T10:15:30.150Z"
  },
  {
    "type": "test:completed",
    "trace_id": "test-abc",
    "status": "failed",
    "duration_ms": 250,
    "timestamp": "2026-01-31T10:15:30.250Z"
  }
]
```

### Load Test Data
- Run 50 tests concurrently (Jest workers)
- Mix of fast tests (50ms), medium (500ms), slow (2000ms)
- 20% of tests fail (assertion failures)
- Each test emits 5-20 events
- Measure: memory usage, event loss, report generation time

## Regression Tests

### Adapter Behavior
- [ ] test:started always emitted before assertions
- [ ] test:completed always emitted after all assertions
- [ ] assertion events have correct expected/actual/message
- [ ] Failed assertions don't prevent subsequent assertions

### Trace ID Integrity
- [ ] Each test has unique trace_id
- [ ] Trace ID propagates to all observable events
- [ ] Trace ID doesn't leak between tests
- [ ] Trace ID format is consistent

### Report Quality
- [ ] Report loads without errors
- [ ] Timeline view sorts events by timestamp
- [ ] Flamegraph percentages add up to 100%
- [ ] Network waterfall shows correct timing
- [ ] Failure details are complete

### Performance
- [ ] Assertion hook adds <1ms per call
- [ ] Report generation <500ms for typical test
- [ ] Memory per test session <5MB
- [ ] No memory leaks over 100-test run

### Integration
- [ ] Network requests correlated via trace_id
- [ ] Backend logs correlated via trace_id
- [ ] Custom events correlated via trace_id
- [ ] Cross-layer timeline is complete and ordered

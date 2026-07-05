---
feature: Self-Healing Tests
status: shipped
tool: observe, generate, interact
mode: test-automation, autonomous-repair
version: 0.7.12
doc_type: product-spec
feature_id: feature-self-healing-tests
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Self-Healing Tests

## Problem Statement

Test failures are the primary feedback mechanism for AI coding assistants. However, traditional test failures often leave AI agents confused:

- **Ambiguous failures:** Error messages don't explain what broke or why
- **Multi-step fixes:** A single failure may require changes across code, mocks, and test setup
- **No verification:** AI suggests fixes but doesn't autonomously confirm they work
- **No root-cause analysis:** Tests fail, but the underlying reason remains opaque

**Result:** Tests become a bottleneck instead of a feedback loop. Human developers must interpret failures and guide AI toward fixes.

## Solution

Self-Healing Tests enable AI to autonomously repair test failures by:

1. **Observe** the test failure via Kaboom (logs, network calls, DOM state, performance)
2. **Diagnose** the root cause (API changed? CSS broken? Selector stale? Mock mismatch?)
3. **Repair** the code or test (update mocks, fix selectors, adjust assertions)
4. **Verify** the fix works (re-run test, confirm pass, no regressions)

The AI acts as a self-healing agent: test fails → diagnose via context → fix → verify → done. No human intervention required.

## Requirements

### Core Flow
- **Autonomous Test Repair:** When a test fails, AI diagnoses via Kaboom context (network, logs, DOM, performance) and proposes fixes
- **Multi-Path Diagnostics:** Support repairs across code, tests, mocks, selectors, assertions
- **Closed-Loop Verification:** AI re-runs tests after fix to confirm success before marking resolved
- **Root-Cause Linking:** Connect test failure → DOM/network/log anomaly → code location requiring fix
- **Regression Prevention:** Verify that fix doesn't break other tests or introduce new failures

### Test Coverage
- **Unit Tests:** Component logic, state mutations
- **Integration Tests:** Multi-component workflows, API interactions
- **E2E Tests:** Full user journeys, cross-feature interactions
- **Mock/Stub Tests:** Verify mocks match actual API behavior

### Output
- **Repair Proposals:** Concrete code/test changes with rationale
- **Verification Evidence:** Test re-run results showing fix worked
- **Confidence Metrics:** % certainty of diagnosis + fix quality
- **Rollback Safety:** If fix fails verification, propose alternatives or escalate

## Out of Scope

- **Manual Test Coverage:** Tests must already exist; AI doesn't create new test suites from scratch (Tier 4 feature)
- **Non-Deterministic Flakes:** Fixes for intermittent/environment-dependent failures deferred to Phase 2
- **Load/Stress Testing:** Performance degradation diagnosis deferred to v6.6 (Performance Audit)
- **Security Test Repair:** Security assertion failures handled by human review

## Success Criteria

- ✅ **Autonomy:** 80%+ of test failures repaired without human intervention
- ✅ **Speed:** Test failure → diagnosis → repair → verification in <5 minutes (vs 30+ minutes manual)
- ✅ **Accuracy:** Repairs result in genuine fixes, not false positives (95%+ pass on re-run)
- ✅ **Multi-Path:** Can diagnose and repair across code, mocks, selectors, assertions, API contracts
- ✅ **Evidence:** Repair proposals include root-cause analysis + verification results
- ✅ **Regression-Free:** Fixes don't introduce new test failures (verify full suite)
- ✅ **Battle-Tested:** UAT at ~/dev/kaboom-demos: 34 bugs autonomously fixed with zero human guidance

## User Workflow

### Scenario 1: Code Logic Failure
1. Developer runs test suite
2. Test fails: "Expected button to be disabled, but was enabled"
3. AI observes failure via Kaboom
   - Fetches logs, network calls, DOM state at failure moment
   - Detects: No API error response, but state update didn't trigger
4. AI diagnoses: "Component state mutation is missing in action handler"
5. AI repairs: Adds state update to fix logic
6. AI verifies: Re-runs test, confirm pass
7. Result: Test fixed autonomously, developer alerted

### Scenario 2: Selector Breakage
1. Test fails: "Cannot find element with ID 'submit-button'"
2. UI was refactored: Button ID changed to `primary-action-submit`
3. AI observes failure + current DOM
4. AI diagnoses: "Selector is stale; element exists with different ID"
5. AI repairs: Updates selector to use stable attribute (role-based, ARIA label)
6. AI verifies: Re-runs test, confirm pass
7. Result: Test resilient to future refactors

### Scenario 3: API Contract Change
1. Test fails: "Expected status 200, got 400"
2. Backend API changed: New required field in request body
3. AI observes failure + network traffic
4. AI diagnoses: "Request missing field 'version'; API contract updated"
5. AI repairs: Updates mock/request to include required field, OR updates assertion if intentional API change
6. AI verifies: Re-runs test, confirm pass
7. Result: Test synced with new API contract

### Scenario 4: Mock/Stub Mismatch
1. Test passes locally but fails in CI
2. Mock doesn't match actual API response shape
3. AI observes failure + compares mock vs actual response
4. AI diagnoses: "Mock is missing field 'pagination'; actual response includes it"
5. AI repairs: Updates mock to match actual API shape
6. AI verifies: Re-runs test, confirm pass
7. Result: Test reliable in both local and CI environments

## Examples

### Example 1: Repair Proposal Output

```
Test Failure: checkout.spec.ts:45 - "Complete purchase flow"
└─ Error: Expected total $99.99, got $0.00

ROOT CAUSE ANALYSIS
─────────────────────
1. Test step: Click "Apply Coupon" button
   ↓ No DOM update detected
2. Network: POST /api/coupons/apply returned 400
   ↓ Missing 'coupon_code' in request body
3. Mock expectation: Mock expects {amount: 99.99}
   ↓ Actual API always returns {subtotal, tax, total, discount}

PROPOSED FIX
────────────
File: checkout.spec.ts:35
BEFORE:
  cy.get('[data-testid="coupon-input"]').type('SAVE10')
  cy.get('[data-testid="apply-coupon"]').click()

AFTER:
  cy.get('[data-testid="coupon-input"]').type('SAVE10')
  cy.get('[data-testid="apply-coupon"]').click()
  cy.intercept('POST', '/api/coupons/apply', {
    statusCode: 200,
    body: { discount: 10, total: 89.99 }  // ← Updated mock response
  })

VERIFICATION
─────────────
✅ Test re-run: PASS
✅ Full suite: 45 tests pass, 0 failures
✅ No new regressions detected
```

## Integration Points

- **Kaboom Context:** Uses `observe()` to fetch logs, network, DOM, performance data at failure moment
- **Code Analysis:** Parses test files to identify selectors, assertions, API expectations
- **Test Runner:** Integration with Cypress, Jest, Playwright to capture failures and re-run
- **CI/CD:** Kaboom CI Infrastructure (Wave 1 #2) enables autonomous repair in pipelines

## Dependencies

- ✅ **Kaboom Core:** observe() tool provides full browser/network context
- ✅ **Buffer-Specific Clearing (v5.3):** Isolates test-specific logs from noise
- ⏳ **Context Streaming (Wave 1 #3):** Real-time error context improves diagnosis speed
- ⏳ **Kaboom CI Infrastructure (Wave 1 #2):** Enables automated repair loops in CI/CD

## Notes

- **UAT Evidence:** ~/dev/kaboom-demos site demonstrates 34 bugs autonomously fixed with zero human guidance
- **Related Specs:**
  - [Kaboom CI Infrastructure](../ci-infrastructure/product-spec.md) — Enables autonomous loops in CI/CD
  - [Context Streaming](../context-streaming/product-spec.md) — Real-time diagnostics
- **Marketing Message:** "AI autonomously fixes tests, not just suggests fixes. Closed-loop verification: failure → diagnosis → repair → confirm → done."

---
feature: test-generation
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation — User Acceptance Testing Guide

## Overview

This guide provides step-by-step instructions for human testers to verify the test generation feature works correctly in real-world development workflows.

**Estimated Time:** 60-90 minutes

### Required Setup:
- Kaboom extension installed and connected
- Kaboom server running
- Claude or MCP-compatible AI client
- A web application with forms/interactions for testing
- Playwright installed (`npm init playwright@latest`)

---

## Prerequisites & Setup

### Environment Checklist

- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and shows "Connected" in popup
- [ ] AI Web Pilot toggle is ON
- [ ] MCP client (Claude Desktop, Cursor, etc.) connected
- [ ] Test web application running locally (e.g., todo app, login form)
- [ ] Playwright test runner available: `npx playwright test --version`

### Test Application Requirements

You need a simple web application with:
1. A form that can produce validation errors
2. Clickable buttons and inputs
3. Network requests (API calls)

---

## Happy Path Tests

### HP-1: Generate Test from Console Error

**Objective:** Generate a Playwright test that reproduces a captured error

#### Steps:
1. Navigate to a page with a form
2. Trigger a validation error (e.g., submit empty form)
3. Confirm error appears in Kaboom capture (`observe {what: "errors"}`)
4. Ask AI: "Generate a test that reproduces the validation error I just triggered"

#### Expected AI Actions:
- AI calls `generate {format: "test_from_context", context: "error"}`

#### Verification:
- [ ] Response includes valid Playwright test code
- [ ] Test imports `{ test, expect }` from '@playwright/test'
- [ ] Test includes navigation to the form page
- [ ] Test includes actions that reproduce the error
- [ ] Test includes assertion for the error condition
- [ ] Selectors use stable attributes (data-testid preferred)

---

### HP-2: Generate Test from User Interactions

**Objective:** Generate a test from recorded user actions

#### Steps:
1. Enable action recording (AI Web Pilot ON)
2. Perform a multi-step workflow:
   - Navigate to login page
   - Fill in username
   - Fill in password
   - Click submit
3. Ask AI: "Generate a test from my recent interactions"

#### Expected AI Actions:
- AI calls `generate {format: "test_from_context", context: "interaction"}`

#### Verification:
- [ ] Test includes all performed actions in order
- [ ] Input values are captured (or marked as [user-provided] if sensitive)
- [ ] Navigation steps included
- [ ] Click actions have correct selectors

---

### HP-3: Test with Network Mocks

**Objective:** Generate a test with network request mocking

#### Steps:
1. Perform actions that trigger API calls
2. Ask AI: "Generate a test with network mocking for the API calls"

#### Expected AI Actions:
- AI calls `generate {format: "test_from_context", context: "interaction", include_mocks: true}`

#### Verification:
- [ ] Test includes `page.route()` for API endpoints
- [ ] Mock responses match captured response shapes
- [ ] Test assertions check API response status

---

### HP-4: Heal Single Broken Selector

**Objective:** Auto-repair a broken selector in a test file

#### Steps:
1. Create a test file with an intentionally broken selector:
   ```typescript
   await page.locator('#old-button-id').click();
   ```
2. On the page, ensure the element has a new identifier
3. Ask AI: "Fix the broken selector '#old-button-id' in my test"

#### Expected AI Actions:
- AI calls `generate {format: "test_heal", action: "repair", broken_selectors: ["#old-button-id"]}`

#### Verification:
- [ ] Response includes healed selector with new value
- [ ] Confidence score provided (>= 0.7 for valid match)
- [ ] Strategy indicates how match was found
- [ ] Updated test content provided

---

### HP-5: Analyze Test for Broken Selectors

**Objective:** Identify broken selectors in a test file

#### Steps:
1. Create a test file with multiple selectors (some broken)
2. Ask AI: "Analyze my test file for broken selectors"

#### Expected AI Actions:
- AI calls `generate {format: "test_heal", action: "analyze", test_file: "path/to/test.spec.ts"}`

#### Verification:
- [ ] Response lists all selectors in the test
- [ ] Broken selectors identified with explanations
- [ ] Working selectors confirmed as valid

---

### HP-6: Classify Test Failure - Selector Broken

**Objective:** Correctly classify a selector-related failure

#### Steps:
1. Run a test that fails due to missing element
2. Capture the error message: "Timeout waiting for selector '#nonexistent'"
3. Ask AI: "Why did this test fail?"

#### Expected AI Actions:
- AI calls `generate {format: "test_classify", action: "failure", failure: {...}}`

#### Verification:
- [ ] Classification: `selector_broken`
- [ ] Confidence >= 0.8
- [ ] Evidence explains selector not in DOM
- [ ] Suggested fix: heal selector
- [ ] is_real_bug: false

---

### HP-7: Classify Test Failure - Real Bug

**Objective:** Correctly identify an actual application bug

#### Steps:
1. Run a test that fails on assertion (e.g., "Expected 'Welcome' to be 'Welcme'")
2. Ask AI: "Is this test failure a real bug?"

#### Expected AI Actions:
- AI calls `generate {format: "test_classify", action: "failure", failure: {...}}`

#### Verification:
- [ ] Classification: `real_bug`
- [ ] is_real_bug: true
- [ ] Evidence points to assertion mismatch
- [ ] Recommended action: "fix_application"

---

### HP-8: Classify Test Failure - Timing/Flaky

**Objective:** Identify a flaky timing issue

#### Steps:
1. Create a test that sometimes fails on slow networks
2. Error: "Timeout waiting for selector" but element exists
3. Ask AI: "This test is flaky, can you diagnose it?"

#### Expected AI Actions:
- AI calls `generate {format: "test_classify", action: "failure", failure: {...}}`

#### Verification:
- [ ] Classification: `timing_flaky`
- [ ] is_flaky: true
- [ ] Suggested fix includes adding wait or increasing timeout

---

### HP-9: Batch Heal Multiple Tests

**Objective:** Heal selectors across multiple test files

#### Steps:
1. Create a directory with 3+ test files with broken selectors
2. Ask AI: "Fix all broken selectors in my tests directory"

#### Expected AI Actions:
- AI calls `generate {format: "test_heal", action: "batch", test_dir: "tests/"}`

#### Verification:
- [ ] All test files analyzed
- [ ] Summary shows total broken, healed, unhealed counts
- [ ] High-confidence fixes applied
- [ ] Low-confidence fixes flagged for review

---

## Error Path Tests

### EP-1: No Error Context Available

**Objective:** Handle case when no errors captured

#### Steps:
1. Clear Kaboom state or use fresh session
2. Ask AI: "Generate a test for the error I triggered"

#### Verification:
- [ ] Error: `no_error_context`
- [ ] Message explains no errors captured
- [ ] Retry guidance: "Trigger an error first, then retry"

---

### EP-2: No User Actions Captured

**Objective:** Handle case when no interactions recorded

#### Steps:
1. Fresh session, no page interactions
2. Ask AI: "Generate a test from my interactions"

#### Verification:
- [ ] Error: `no_actions_captured`
- [ ] Guidance: "Interact with the page first"

---

### EP-3: Test File Not Found

**Objective:** Handle invalid test file path

#### Steps:
1. Ask AI: "Heal selectors in /nonexistent/path/test.spec.ts"

#### Verification:
- [ ] Error: `test_file_not_found`
- [ ] Message includes the path that wasn't found

---

### EP-4: Invalid Selector Syntax

**Objective:** Handle unparseable selectors

#### Steps:
1. Ask AI to heal: "[[[invalid selector syntax"

#### Verification:
- [ ] Error: `selector_not_parseable`
- [ ] Guidance to use valid CSS selector

---

### EP-5: Classification with Insufficient Context

**Objective:** Handle ambiguous failures

#### Steps:
1. Provide minimal failure info (just "Test failed")
2. Ask AI to classify

#### Verification:
- [ ] Classification: `unknown` or low confidence
- [ ] Evidence list empty or minimal
- [ ] Guidance to provide more context

---

### EP-6: Path Outside Project Directory

**Objective:** Prevent path traversal

#### Steps:
1. Ask AI: "Heal selectors in /etc/passwd"

#### Verification:
- [ ] Error: `path_not_allowed`
- [ ] Security boundary enforced

---

## Edge Case Tests

### EC-1: Very Long Test File

**Objective:** Handle large test files

#### Steps:
1. Create a test file with 500+ lines
2. Request selector analysis

#### Verification:
- [ ] Analysis completes within timeout
- [ ] Results paginated or summarized if large
- [ ] No memory issues

---

### EC-2: Test with No Selectors

**Objective:** Handle tests without selectors

#### Steps:
1. Create a test that only does navigation (no locators)
2. Request selector healing

#### Verification:
- [ ] Response indicates no selectors found
- [ ] Graceful handling (not an error)

---

### EC-3: Multiple Elements Match Healed Selector

**Objective:** Handle ambiguous selector matches

#### Steps:
1. Page has multiple buttons with same text
2. Request healing for broken selector targeting one

#### Verification:
- [ ] Lower confidence score due to ambiguity
- [ ] Evidence mentions multiple matches
- [ ] Suggestion to use more specific selector

---

### EC-4: Framework Detection

**Objective:** Generate tests in different frameworks

#### Steps:
1. Ask AI: "Generate a Vitest unit test for this error"
2. Ask AI: "Generate a Jest test for this interaction"

#### Verification:
- [ ] Vitest test uses `describe`, `it`, `expect` syntax
- [ ] Jest test uses appropriate syntax
- [ ] Imports match framework

---

### EC-5: Secret Detection in Generated Test

**Objective:** Redact sensitive values

#### Steps:
1. Fill in a form with value that looks like API key: "sk_test_abc123"
2. Generate test from interactions

#### Verification:
- [ ] Sensitive value redacted or replaced
- [ ] Comment indicates redaction
- [ ] Test still valid (placeholder used)

---

### EC-6: Generated Test with Dynamic Content

**Objective:** Handle dynamically generated selectors

#### Steps:
1. Page has elements with UUID-based IDs: `#item-a1b2c3d4-e5f6`
2. Generate test including interactions with these

#### Verification:
- [ ] Test uses stable selector (not UUID)
- [ ] Fallback to text or data-testid
- [ ] Warning if no stable selector available

---

### EC-7: Concurrent Heal Requests

**Objective:** Handle multiple simultaneous requests

#### Steps:
1. Request batch heal on directory A
2. Immediately request batch heal on directory B

#### Verification:
- [ ] Both complete successfully
- [ ] No data corruption
- [ ] Results are correct for each

---

## Performance Validation

### PV-1: Test Generation Timing

#### Test Matrix:

| Context Type | Expected Time | Actual Time | Pass? |
|--------------|---------------|-------------|-------|
| Single error | < 3s | _____ | [ ] |
| 10 actions | < 5s | _____ | [ ] |
| 50 actions | < 10s | _____ | [ ] |

---

### PV-2: Selector Healing Timing

#### Test Matrix:

| Scenario | Expected Time | Actual Time | Pass? |
|----------|---------------|-------------|-------|
| 1 selector | < 1s | _____ | [ ] |
| 10 selectors | < 5s | _____ | [ ] |
| Batch (10 files) | < 30s | _____ | [ ] |

---

### PV-3: Classification Timing

#### Test Matrix:

| Scenario | Expected Time | Actual Time | Pass? |
|----------|---------------|-------------|-------|
| Single failure | < 2s | _____ | [ ] |
| Batch (10 failures) | < 15s | _____ | [ ] |

---

## Real-World Workflow Tests

### WF-1: Complete Debug-Test-Fix Loop

**Objective:** Verify full autonomous debugging workflow

#### Steps:
1. Developer: "I'm getting an error when I submit the form"
2. Let AI:
   - Capture the error
   - Generate a test
   - Run the test (verify it fails)
   - (Developer fixes the bug)
   - Re-run the test (verify it passes)

#### Verification:
- [ ] AI captures error context automatically
- [ ] Generated test reproduces the error
- [ ] Test is valid and runnable
- [ ] After fix, test passes

---

### WF-2: Test Maintenance After Refactor

**Objective:** Heal tests after UI changes

#### Steps:
1. Have existing passing tests
2. Refactor UI (change element IDs, classes)
3. Tests now fail
4. Ask AI: "My tests are failing after the refactor, fix them"

#### Verification:
- [ ] AI identifies broken selectors
- [ ] AI heals with high confidence
- [ ] After healing, tests pass

---

### WF-3: Flaky Test Diagnosis

**Objective:** Identify and fix flaky tests

#### Steps:
1. Have a flaky test (passes sometimes, fails sometimes)
2. Run it and capture a failure
3. Ask AI: "This test is flaky, help me fix it"

#### Verification:
- [ ] AI classifies as timing_flaky or network_flaky
- [ ] Suggested fix addresses root cause
- [ ] After fix, test is stable

---

### WF-4: Regression Test from Baseline

**Objective:** Generate regression test from analyze baseline

#### Steps:
1. Run `analyze {action: "regression", scope: "baseline"}`
2. Make a change to the application
3. Ask AI: "Generate a regression test for the changes since baseline"

#### Verification:
- [ ] Test captures before/after state
- [ ] Assertions check for regressions
- [ ] Test name indicates regression context

---

## Sign-Off Checklist

### Tester Information

| Field | Value |
|-------|-------|
| Tester Name | |
| Date | |
| Browser/Version | |
| Extension Version | |
| Server Version | |

### Test Results Summary

| Category | Passed | Failed | Blocked |
|----------|--------|--------|---------|
| Happy Path (HP-1 to HP-9) | | | |
| Error Path (EP-1 to EP-6) | | | |
| Edge Cases (EC-1 to EC-7) | | | |
| Performance (PV-1 to PV-3) | | | |
| Workflows (WF-1 to WF-4) | | | |

### Failed Tests

| Test ID | Issue Description | Severity |
|---------|-------------------|----------|
| | | |

### Blocked Tests

| Test ID | Blocking Reason |
|---------|-----------------|
| | |

### Notes

_Additional observations, edge cases discovered, or suggestions:_

---

### Final Verdict

- [ ] **PASS** - All critical tests pass, ready for release
- [ ] **CONDITIONAL PASS** - Minor issues, can release with known issues documented
- [ ] **FAIL** - Critical issues found, must fix before release

**Approved By:** __________________________ **Date:** __________

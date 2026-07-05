---
feature: test-generation
version: 0.7.12
doc_type: qa-plan
feature_id: feature-test-generation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation — Comprehensive QA Plan

## Testing Strategy Overview

### Test Pyramid

| Layer | Count | Focus |
|-------|-------|-------|
| Unit (Go) | ~55 tests | Request validation, response schemas, generation logic |
| Integration | ~15 tests | End-to-end flows, cross-component interactions |
| Edge Cases | ~35 tests | Boundary conditions, error paths, unusual inputs |
| Performance | ~12 tests | Timeout scenarios, large inputs, batch operations |
| Security | ~10 tests | Path traversal, injection, secret detection |

### Test Location

- **Go Server:** `cmd/browser-agent/testgen_test.go`

---

## Unit Tests: Go Server

### 1. Tool Registration & Schema Validation

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-UNIT-001 | `generate` tool accepts `format: "test_from_context"` | Request dispatched to GenerateTestFromContext | P0 |
| TG-UNIT-002 | `generate` tool accepts `format: "test_heal"` | Request dispatched to HealSelector | P0 |
| TG-UNIT-003 | `generate` tool accepts `format: "test_classify"` | Request dispatched to ClassifyFailure | P0 |
| TG-UNIT-004 | Invalid `type` value rejected | Error: "unknown_mode" | P0 |
| TG-UNIT-005 | Missing `type` parameter rejected | Error: "missing_param" | P0 |

### 2. test_from_context Mode - Request Validation

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-UNIT-010 | `context: "error"` with valid error | Test generated from error context | P0 |
| TG-UNIT-011 | `context: "interaction"` accepted | Test generated from recorded actions | P0 |
| TG-UNIT-012 | Invalid context value rejected | Error: "invalid_param" | P0 |
| TG-UNIT-013 | `framework: "playwright"` (default) | Playwright test syntax | P0 |
| TG-UNIT-014 | `framework: "vitest"` accepted | Vitest test syntax | P1 |
| TG-UNIT-015 | `base_url` overrides origin | URLs use provided base_url | P1 |

### 3. test_from_context.error - Generation Logic

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-UNIT-030 | Generate test from single console error | Valid Playwright test with error assertion | P0 |
| TG-UNIT-031 | Generated test includes actions within ±5s of error | Actions timeline captured | P0 |
| TG-UNIT-032 | Generated test uses testId selector priority | getByTestId used when available | P0 |
| TG-UNIT-033 | Generated test uses role selector fallback | getByRole used | P1 |
| TG-UNIT-034 | Generated test includes error comment | Comment with error message | P0 |
| TG-UNIT-035 | No actions captured returns error | Error: "no_actions_captured" | P0 |
| TG-UNIT-036 | No error context available | Error: "no_error_context" | P0 |

### 4. test_from_context.interaction - Generation Logic

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-UNIT-050 | Generate test from click actions | await page.locator().click() | P0 |
| TG-UNIT-051 | Generate test from input actions | await page.locator().fill() | P0 |
| TG-UNIT-052 | Generate test from select actions | await page.locator().selectOption() | P1 |
| TG-UNIT-053 | Redacted password values replaced | [user-provided] placeholder | P0 |
| TG-UNIT-054 | Generated test has valid syntax | No syntax errors | P0 |

### 5. test_heal Mode - Request Validation

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-UNIT-070 | `action: "analyze"` with valid test_file | Broken selectors identified | P0 |
| TG-UNIT-071 | `action: "repair"` with broken_selectors | Healed selectors returned | P0 |
| TG-UNIT-072 | Invalid action rejected | Error: "invalid_param" | P0 |
| TG-UNIT-073 | test_file path not found | Error: "test_file_not_found" | P0 |

### 6. test_heal.repair - Healing Logic

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-UNIT-090 | Heal selector by testid_match strategy | confidence >= 0.9 | P0 |
| TG-UNIT-091 | Heal selector by aria_match strategy | confidence ~0.7 | P0 |
| TG-UNIT-092 | Heal selector by text_match strategy | Visible text matched | P1 |
| TG-UNIT-093 | No match found | Selector in unhealed list | P0 |
| TG-UNIT-094 | updated_content includes all fixes | Full file with replacements | P0 |

### 7. test_classify Mode - Classification Logic

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-UNIT-130 | "Timeout waiting for selector" + selector missing | category: "selector_broken" | P0 |
| TG-UNIT-131 | "Timeout waiting for selector" + selector exists | category: "timing_flaky" | P0 |
| TG-UNIT-132 | "net::ERR_" error | category: "network_flaky" | P0 |
| TG-UNIT-133 | "Expected X to be Y" assertion | category: "real_bug", is_real_bug: true | P0 |
| TG-UNIT-134 | Unknown error pattern | category: "unknown", low confidence | P1 |
| TG-UNIT-135 | suggested_fix provided for selector_broken | type: "selector_update" | P0 |

---

## Integration Tests

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-INT-001 | End-to-end: capture error, generate test | Test reproduces error | P0 |
| TG-INT-002 | End-to-end: heal broken selector, verify fixed | Test passes after healing | P0 |
| TG-INT-003 | End-to-end: classify real test failure | Correct category identified | P0 |
| TG-INT-004 | No extension connected | Error: "extension_timeout" | P0 |

---

## Edge Cases

### Input Boundary Conditions

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-EDGE-001 | Empty error message | Test generated with minimal context | P1 |
| TG-EDGE-002 | Error message > 80 characters | Truncated in test name | P1 |
| TG-EDGE-003 | Selector with special characters | Properly escaped | P0 |
| TG-EDGE-004 | Selector with quotes | Escaped correctly | P0 |
| TG-EDGE-005 | Empty selectors map | Fallback comment | P1 |

### Large Input Handling

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-EDGE-020 | Generate test from 100 actions | Script capped at 50KB | P1 |
| TG-EDGE-021 | DOM with 10,000 elements | Healing completes | P1 |

### Selector Edge Cases

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-EDGE-030 | Selector matches 0 elements | In unhealed list | P0 |
| TG-EDGE-031 | Invalid CSS selector syntax | Error: "selector_not_parseable" | P0 |

---

## Performance Tests

| Test ID | Metric | Target | Priority |
|---------|--------|--------|----------|
| TG-PERF-001 | test_from_context.error generation | < 3s | P0 |
| TG-PERF-002 | test_from_context.interaction (10 actions) | < 2s | P0 |
| TG-PERF-003 | test_heal.analyze single file | < 1s | P0 |
| TG-PERF-004 | test_heal.repair single selector | < 1s | P0 |
| TG-PERF-005 | test_classify.failure single | < 2s | P0 |

---

## Security Tests

| Test ID | Test Case | Expected Result | Priority |
|---------|-----------|-----------------|----------|
| TG-SEC-001 | test_file path traversal (../) | Error: "path_not_allowed" | P0 |
| TG-SEC-002 | Selector with script injection | Sanitized | P0 |
| TG-SEC-003 | Generated test contains API key | Value redacted | P0 |
| TG-SEC-004 | Generated test contains password | Value redacted | P0 |
| TG-SEC-005 | Generated test contains Bearer token | Value redacted | P0 |

---

## Error Code Tests

| Test ID | Error Code | Trigger Condition | Priority |
|---------|------------|-------------------|----------|
| TG-ERR-001 | no_error_context | Generate test with no errors | P0 |
| TG-ERR-002 | no_actions_captured | Generate test with no actions | P0 |
| TG-ERR-003 | test_file_not_found | Heal non-existent file | P0 |
| TG-ERR-004 | selector_not_parseable | Heal invalid selector | P0 |
| TG-ERR-005 | dom_query_failed | Extension returns error | P0 |

---

## Discovered Edge Cases (Not in Specs)

| Test ID | Edge Case | Recommended Handling | Priority |
|---------|-----------|---------------------|----------|
| TG-DISC-001 | Action sequence includes page reload | Handle state reset | P1 |
| TG-DISC-002 | Alert/confirm/prompt dialogs | Generate page.on('dialog') | P1 |
| TG-DISC-003 | Test healing TypeScript vs JavaScript | Preserve file extension | P1 |
| TG-DISC-004 | Dynamic class names (CSS-in-JS) | Prefer testId/aria | P1 |
| TG-DISC-005 | Unicode characters in selectors | Proper encoding | P1 |

---

## Coverage Targets

| Component | Line Coverage | Branch Coverage |
|-----------|--------------|-----------------|
| testgen.go | >= 85% | >= 80% |
| testgen_heal.go | >= 85% | >= 80% |
| testgen_classify.go | >= 85% | >= 80% |

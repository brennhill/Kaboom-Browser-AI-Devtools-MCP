---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# {Feature Name} — Test Plan

**Status:** [ ] Product Tests Defined | [ ] Tech Tests Designed | [ ] Tests Generated | [ ] All Tests Passing

---

## Product Tests

High-level scenarios that prove the feature works correctly across all states and edge cases.

### Valid State Tests

- **Test:** Describe what should happen in a valid state
  - **Given:** Initial conditions
  - **When:** Action taken
  - **Then:** Expected outcome

- **Test:** Another valid scenario
  - **Given:** ...
  - **When:** ...
  - **Then:** ...

### Edge Case Tests (Negative)

- **Test:** What happens when [edge case 1]
  - **Given:** ...
  - **When:** ...
  - **Then:** Feature handles gracefully with [expected behavior]

- **Test:** What happens when [edge case 2]
  - **Given:** ...
  - **When:** ...
  - **Then:** ...

### Concurrent/Race Condition Tests

- **Test:** What happens when [operation A] and [operation B] happen simultaneously
  - **Given:** ...
  - **When:** ...
  - **Then:** ...

### Failure & Recovery Tests

- **Test:** What happens when [failure scenario]
  - **Given:** ...
  - **When:** ...
  - **Then:** System recovers by [recovery method]

---

## Technical Tests

How the product tests will be implemented and validated.

### Unit Tests

#### Coverage Areas:
- Core logic function: `functionName()` with inputs (valid, boundary, invalid)
- State transitions in `StateManager`
- Data validation in `DataValidator`

**Test File:** `test/features/{feature-name}.test.ts`

### Integration Tests

#### Scenarios:
- End-to-end flow: initialization → action → state change → verification
- Service interactions: MCP server ↔ Extension ↔ Browser
- Concurrent client connections under load

**Test File:** `test/integration/{feature-name}.integration.test.ts`

### UAT/Acceptance Tests

**Framework:** [Playwright/custom bash scripts]

#### Scenarios:
- Browser automation: user clicks button → feature executes → result visible
- Data persistence: state survives browser reload
- Error handling: invalid input shown to user correctly

**Test File:** `scripts/tests/{feature-name}-uat.sh` or `test/uat/{feature-name}.spec.ts`

### Manual Testing (if applicable)

#### Steps:
1. Open browser with extension enabled
2. Navigate to [URL]
3. Verify [behavior]
4. Check browser console for errors
5. Verify data in [location]

---

## Test Status

### Links to generated test files (update as tests are created):

| Test Type | File | Status | Notes |
|-----------|------|--------|-------|
| Unit | TBD | ⏳ Pending | Awaiting implementation |
| Integration | TBD | ⏳ Pending | Awaiting implementation |
| UAT | TBD | ⏳ Pending | Awaiting implementation |
| Manual | TBD | ⏳ Pending | Awaiting implementation |

**Overall:** All product test scenarios must pass before feature is considered complete.

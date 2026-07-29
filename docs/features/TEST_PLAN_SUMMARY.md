---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Kaboom Feature Test Plans — Summary Report

**Generated:** 2026-02-09
**Status:** ✅ 5 Test Plans Created | ⏳ 40+ Recommended Additional Tests Identified

---

## Overview

This document summarizes the comprehensive test plans created for 5 major Kaboom features with existing UAT test coverage. Each feature now has a dedicated `test-plan.md` that:

1. **Documents all product-level test scenarios** (valid states, edge cases, concurrency, failure recovery)
2. **Maps UAT tests to actual test files** (with line numbers and test counts)
3. **Identifies test gaps** (scenarios in specs not yet covered by UAT)
4. **Recommends additional UAT tests** (new test categories to fill gaps)

---

## Features & Test Plan Status

### 1. Link Health (`feature/link-health/`)

**Location:** `/Users/brenn/dev/kaboom/docs/features/feature/link-health/test-plan.md`

**UAT Tests:** 19 tests in `cat-19-link-health.sh`
- Group A (5): Basic link checking (correlation_id, parameters, status=queued, hint)
- Group B (3): Analyzer dispatcher (routing, missing params, invalid mode)
- Group C (3): Error handling (invalid JSON, invalid timeout_ms, unknown params)
- Group D (2): Concurrency (5 concurrent calls, 50 repeated calls)
- Group E (3): Response structure (format, content blocks, valid JSON)
- Group F (3): Integration (tool discovery, MCP compliance, tools/call)

#### Status:
- ✅ API Contract Tests: **PASSING** (19/19)
- ⏳ HTTP Logic Tests: **TODO** (10 gaps identified)
  - Actual HTTP requests to URLs
  - Status code categorization accuracy
  - Persistent storage validation
  - Crash recovery (warm start)
  - External vs internal link tracking
  - Worker concurrency
  - Timeout handling
  - CORS detection edge cases
  - Session ID collision prevention
  - Disk full error handling

#### Recommended New Tests:
- `cat-19-extended`: 10 tests for HTTP logic, persistence, recovery

---

### 2. Noise Filtering (`feature/noise-filtering/`)

**Location:** `/Users/brenn/dev/kaboom/docs/features/feature/noise-filtering/test-plan.md`

**UAT Tests:** 10 tests in `cat-20-noise-persistence.sh`
- Test 1: Rule creation with user_N ID
- Test 2-3: File persistence and schema validation
- Test 4-5: Restart resilience and ID uniqueness
- Test 6-8: Removal persistence and reset functionality
- Test 9-10: Corruption handling and built-in rule freshness

#### Status:
- ✅ Persistence Tests: **PASSING** (10/10)
- ⚠️ **CRITICAL DATA LEAK TESTS: NOT IMPLEMENTED**
  - DL-1: 401/403 responses must NOT be filtered ⚠️ **CRITICAL**
  - DL-2: App errors never auto-detected as noise ⚠️ **CRITICAL**
  - DL-3: Dismissed patterns don't suppress security events ⚠️ **CRITICAL**
- ⏳ Filtering Logic Tests: **TODO** (10+ gaps)
  - Built-in rule matching (console, network, WebSocket)
  - Framework detection (React, Vite, Next.js)
  - Auto-detect confidence thresholds
  - Periodicity detection (infrastructure)
  - Entropy scoring
  - Dismiss noise rule creation
  - Statistics accuracy
  - Concurrent RWMutex behavior

#### Recommended New Tests:
- `cat-20-security`: 5 tests for **CRITICAL data leak protection**
- `cat-20-filtering-logic`: 6 tests for built-in rule matching
- `cat-20-auto-detect`: 5 tests for auto-detection heuristics

**⚠️ BLOCKING ISSUE:** Data leak tests (DL-1, DL-2, DL-3) must be implemented before production release.

---

### 3. AI Web Pilot (`feature/ai-web-pilot/`)

**Location:** `/Users/brenn/dev/kaboom/docs/features/feature/ai-web-pilot/test-plan.md`

**UAT Tests:** 12 tests across 3 categories
- **Cat-13** (3 tests): Pilot state contract tests
  - Test 13.1: navigate fails when pilot OFF (regression guard)
  - Test 13.2: /sync accepts both pilot ON and OFF
  - Test 13.3: execute_js fails when pilot OFF

- **Cat-14** (5 tests): Extension startup sequence
  - Test 14.1-14.5: /sync payload validation, pilot transitions, version compatibility, command results

- **Cat-15** (4 tests): Pilot-gated actions success path
  - Test 15.1-15.4: navigate/execute_js/highlight success, pilot OFF→ON transition

#### Status:
- ✅ Core Gating Logic Tests: **PASSING** (12/12)
- ⏳ Edge Case Tests: **TODO** (8 gaps)
  - Rapid state toggles (ON→OFF→ON)
  - Concurrent sessions (different states per session)
  - Session persistence across syncs
  - Default state (uninitialized = false)
  - Failed action logging
  - Data not captured if blocked
  - Extension reconnect scenario
  - Long-running actions during state change

#### Recommended New Tests:
- `cat-15-extended`: 8 tests for state transitions and edge cases

---

### 4. Flow Recording (`feature/flow-recording/`)

**Location:** `/Users/brenn/dev/kaboom/docs/features/feature/flow-recording/test-plan.md`

**UAT Tests:** 7 tests in `cat-18-recording.sh`
- Test 18.1-18.2: screen_recording_start/screen_recording_stop JSON-RPC validation
- Test 18.3: observe(saved_videos) structure validation
- Test 18.4-18.7: Audio parameter handling and validation

#### Status:
- ✅ API Contract Tests: **PASSING** (7/7)
- ⏳ Recording Logic Tests: **TODO** (8 gaps)
  - Action recording (navigate, click, type, select, keypress)
  - Selector capture (data-testid, CSS, x/y)
  - Screenshot capture on errors
  - File storage verification

- ⏳ Playback Logic Tests: **TODO** (7 gaps)
  - Action execution in sequence
  - Self-healing on moved selector
  - Non-blocking error handling
  - Data-testid, CSS, x/y fallbacks
  - Element move detection
  - Test boundary logging

- ⏳ Log Diffing Tests: **TODO** (5 gaps)
  - Original vs replay comparison
  - Regression detection (new errors)
  - Missing call detection
  - Timing change detection
  - Clean pass (no changes)

#### Recommended New Tests:
- `cat-18-recording-logic`: 8 tests for recording action capture
- `cat-18-playback-logic`: 7 tests for action execution
- `cat-18-log-diffing`: 5 tests for log comparison

---

### 5. Test Generation (`feature/test-generation/`)

**Location:** `/Users/brenn/dev/kaboom/docs/features/feature/test-generation/test-plan.md`

**UAT Tests:** 6 tests in `cat-29-reproduction.sh`
- Test 17.1: Seed actions via HTTP POST (5 actions)
- Test 17.2: Export kaboom format (natural language)
- Test 17.3-17.6: Playwright export, Vitest export, round-trip validation, error handling

#### Status:
- ✅ Action Export Tests: **PASSING** (6/6)
- ⏳ Generation Logic Tests: **TODO** (6 gaps)
  - Generate Playwright test from error context
  - Generate Vitest test from context
  - Generate Jest test from context
  - Assertion generation accuracy
  - Framework template variations
  - Generated test syntax validation

- ⏳ Selector Healing Tests: **TODO** (7 gaps)
  - Identify broken selectors
  - Repair with data-testid strategy
  - Repair with ARIA label strategy
  - Repair with text strategy
  - Repair with attribute strategy
  - Confidence scoring accuracy
  - Batch healing

- ⏳ Classification Tests: **TODO** (6 gaps)
  - Classify timeout + latency spike as environmental
  - Classify timeout + normal latency as real bug
  - Classify selector move as flaky
  - Classify assertion mismatch as real bug
  - Classify element not found timing as flaky
  - Classify API timeout as real bug

#### Recommended New Tests:
- `cat-17-generation`: 6 tests for test code generation
- `cat-17-healing`: 7 tests for selector repair
- `cat-17-classification`: 6 tests for failure analysis

---

## Test Coverage Summary

### By Feature

| Feature | API Contract | Logic | Persistence | Security | Total Existing | Total Recommended | Total Target |
|---------|--------|-------|-------------|----------|-----------------|------------------|--------------|
| **link-health** | ✅ 19 | ⏳ 0 | ⏳ 0 | N/A | **19** | **10** | **29** |
| **noise-filtering** | ✅ 10 | ⏳ 0 | ✅ 10 | ⚠️ 0 | **10** | **16** | **26** |
| **ai-web-pilot** | ✅ 12 | ⏳ 0 | N/A | N/A | **12** | **8** | **20** |
| **flow-recording** | ✅ 7 | ⏳ 0 | N/A | N/A | **7** | **20** | **27** |
| **test-generation** | ✅ 6 | ⏳ 0 | N/A | N/A | **6** | **19** | **25** |
| **TOTALS** | **✅ 54** | **⏳ 0** | **✅ 10** | **⚠️ 0** | **54** | **73** | **127** |

### Test Type Breakdown

| Test Type | Existing | Recommended | Gaps |
|-----------|----------|-------------|------|
| Unit | ~25 | ~15 | ~40 |
| Integration | ~15 | ~20 | ~35 |
| **UAT/Acceptance** | **54** | **73** | **127** |
| Manual | 0 | 5+ | N/A |

---

## Critical Gaps Identified

### Blocking (Must Implement Before Release)

1. **Noise Filtering: Data Leak Tests** ⚠️ **CRITICAL**
   - DL-1: 401/403 responses never filtered
   - DL-2: App errors never auto-detected
   - DL-3: Dismissed patterns safe
   - **Impact:** Security vulnerability if not tested
   - **Effort:** Medium (new cat-20-security with 5 tests)

### High Priority (Should Implement)

2. **Link Health: HTTP Logic Tests**
   - Actual network requests, status categorization, persistence
   - **Impact:** Feature untested for core functionality
   - **Effort:** Medium (new cat-19-extended with 10 tests)

3. **Flow Recording: Recording/Playback/Diffing**
   - Recording action capture, playback execution, log diffing
   - **Impact:** Feature untested for end-to-end workflow
   - **Effort:** High (3 new test categories, 20 tests total)

4. **Test Generation: Generation/Healing/Classification**
   - Test code generation, selector repair, failure analysis
   - **Impact:** Feature untested for core functionality
   - **Effort:** High (3 new test categories, 19 tests total)

### Medium Priority (Nice to Have)

5. **AI Web Pilot: Edge Case Tests**
   - Rapid toggles, concurrent sessions, defaults
   - **Impact:** Edge case coverage
   - **Effort:** Low (1 new test category, 8 tests)

6. **Noise Filtering: Framework Detection Tests**
   - React, Vite, Next.js detection
   - **Impact:** Framework-specific rule activation coverage
   - **Effort:** Medium (new cat-20-filtering-logic with 6 tests)

---

## Recommended Implementation Timeline

### Phase 1 (Week 1) — CRITICAL Security Tests
- [ ] `cat-20-security` — Noise filtering data leak tests (5 tests)
  - Status: **BLOCKING** for production release
  - Effort: Medium (~4 hours)
  - Why: DL-1, DL-2, DL-3 are security-critical

### Phase 2 (Week 2-3) — Core Logic Tests
- [ ] `cat-19-extended` — Link health HTTP logic (10 tests)
- [ ] `cat-20-filtering-logic` — Noise filtering built-in rules (6 tests)
- [ ] `cat-15-extended` — AI Web Pilot edge cases (8 tests)

### Phase 3 (Week 4-6) — Feature Completeness
- [ ] `cat-18-recording-logic` — Flow recording capture (8 tests)
- [ ] `cat-18-playback-logic` — Flow recording playback (7 tests)
- [ ] `cat-18-log-diffing` — Flow recording diffing (5 tests)
- [ ] `cat-17-generation` — Test generation from context (6 tests)
- [ ] `cat-17-healing` — Selector repair (7 tests)
- [ ] `cat-17-classification` — Failure classification (6 tests)

### Phase 4 (Ongoing) — Auto-Detection & Advanced
- [ ] `cat-20-auto-detect` — Noise filtering auto-detection (5 tests)
- [ ] Manual testing suites for browser-based features

---

## Files Created

### New Test Plan Documents

1. **`/Users/brenn/dev/kaboom/docs/features/feature/link-health/test-plan.md`** (382 lines)
   - Product tests: 4 valid, 5 edge, 2 concurrent, 4 recovery
   - Tech tests: Unit (link extraction, categorization), Integration (workflows), UAT (19 tests)
   - Gaps: 10 HTTP logic tests needed
   - Status: ✅ Comprehensive

2. **`/Users/brenn/dev/kaboom/docs/features/feature/noise-filtering/test-plan.md`** (534 lines)
   - Product tests: 5 valid, 5 edge, 2 concurrent, 2 recovery
   - Tech tests: Unit (rule matching, persistence), Integration (auto-detect), UAT (10 tests)
   - Gaps: **CRITICAL DL-1/2/3, 10 filtering logic tests**
   - Status: ⚠️ Persistence validated, data leak tests **NOT IMPLEMENTED**

3. **`/Users/brenn/dev/kaboom/docs/features/feature/ai-web-pilot/test-plan.md`** (395 lines)
   - Product tests: 5 valid (pilot ON), 3 invalid (pilot OFF), 2 transitions, 3 startup, 2 concurrent
   - Tech tests: Unit (gating, state cache), Integration (transitions, sessions), UAT (12 tests)
   - Gaps: 8 edge case tests (rapid toggles, concurrent sessions, defaults)
   - Status: ✅ Core logic validated, edge cases recommended

4. **`/Users/brenn/dev/kaboom/docs/features/feature/flow-recording/test-plan.md`** (411 lines)
   - Product tests: 3 recording, 3 playback, 2 diffing
   - Tech tests: Unit (actions, selectors, diffing), Integration (workflows), UAT (7 tests)
   - Gaps: 8 recording logic, 7 playback logic, 5 diffing tests
   - Status: ⏳ API contract validated, **core logic NOT TESTED**

5. **`/Users/brenn/dev/kaboom/docs/features/feature/test-generation/test-plan.md`** (402 lines)
   - Product tests: 3 generation, 3 healing, 2 classification
   - Tech tests: Unit (generation, healing, classification), Integration (workflows), UAT (6 tests)
   - Gaps: 6 generation, 7 healing, 6 classification tests
   - Status: ⏳ Action export validated, **core logic NOT TESTED**

### Summary Document

6. **`/Users/brenn/dev/kaboom/docs/features/TEST_PLAN_SUMMARY.md`** (This file)
   - Executive summary
   - Feature-by-feature status
   - Test coverage matrix
   - Critical gaps analysis
   - Implementation timeline

---

## Usage Guide

### For QA Engineers

1. **Review existing UAT tests:**
   ```bash
   # Link health API contract (19 tests)
   ./scripts/tests/browser/cat-19-link-health.sh 7890 /tmp/cat-19-results.txt

   # Noise filtering persistence (10 tests)
   ./scripts/tests/capture/cat-20-noise-persistence.sh 7890 /tmp/cat-20-results.txt

   # AI Web Pilot contracts (12 tests across 3 files)
   ./scripts/tests/browser/cat-13-pilot-contract.sh 7900 /tmp/cat-13-results.txt
   ./scripts/tests/browser/cat-14-extension-startup.sh 7901 /tmp/cat-14-results.txt
   ./scripts/tests/browser/cat-15-pilot-success-path.sh 7902 /tmp/cat-15-results.txt
   ```

2. **Identify test gaps:**
   - Open each feature's `test-plan.md`
   - Review "Test Gaps & Coverage Analysis" section
   - Prioritize by severity (CRITICAL > HIGH > MEDIUM)

3. **Implement recommended tests:**
   - Follow patterns in existing `cat-XX-*.sh` files
   - Create new test categories (e.g., `cat-20-security.sh`)
   - Integrate into comprehensive suite

### For Developers

1. **Understand feature scope:**
   - Read product tests (high-level scenarios)
   - Check tech tests (unit/integration design)

2. **Verify implementation:**
   - Run UAT tests: `./scripts/test-all-tools-comprehensive.sh`
   - Review failures against test-plan gaps
   - Implement missing logic if tests fail

3. **Debug failures:**
   - Check test-plan for edge cases
   - Verify security invariants (esp. noise-filtering DL-1/2/3)
   - Add unit tests for core logic

### For Product Managers

1. **Track feature completeness:**
   - Green (✅): API contract fully tested
   - Yellow (⏳): Core logic tests needed
   - Red (⚠️): CRITICAL tests missing

2. **Prioritize work:**
   - Blocking: Noise filtering data leak tests
   - High: Link health HTTP logic, flow recording workflows
   - Medium: Edge cases and advanced scenarios

3. **Communicate status:**
   - "Feature X has 19 API tests passing, needs 10 additional tests for complete coverage"
   - "Feature Y has CRITICAL security tests missing — must fix before release"

---

## Next Steps

### Immediate (This Sprint)

1. **Review noise-filtering data leak risks** — Understand DL-1, DL-2, DL-3 requirements
2. **Schedule security testing** — Implement cat-20-security tests
3. **Assess link-health priority** — Decide if HTTP logic tests needed for v6.0 or v6.1

### Short-term (Next 2 Sprints)

4. **Implement Phase 1 tests** — Blocking data leak tests first
5. **Implement Phase 2 tests** — Core logic coverage
6. **Review test-plan diffs** — Ensure all gaps documented

### Medium-term (Future Sprints)

7. **Build Phase 3 tests** — Feature completeness
8. **Establish testing standards** — All features should have test-plans
9. **Automate test generation** — Reduce manual UAT scripting

---

## Appendix: Test Plan Template

All test-plans follow the standard template in `/docs/features/_template/template-test-plan.md`:

- Product Tests (valid, edge, concurrent, failure/recovery)
- Technical Tests (unit, integration, UAT)
- Test Gaps & Coverage Analysis
- Recommended Additional Tests
- Test Status Summary
- Running Instructions
- Known Limitations
- Sign-Off

---

## Document Info

- **Author:** Claude Code
- **Date:** 2026-02-09
- **Scope:** 5 major Kaboom features with UAT test coverage
- **Status:** ✅ Test plans created, gaps identified, recommendations prioritized
- **Next Review:** After Phase 1 (data leak tests) implementation

---

## Quick Reference: Test File Locations

| Feature | Product Spec | Tech Spec | Test Plan | UAT Tests | Smoke Tests |
|---------|--------------|-----------|-----------|-----------|------------|
| **link-health** | ❌ None | ✅ [tech-spec.md](feature/link-health/tech-spec.md) | ✅ [test-plan.md](feature/link-health/test-plan.md) | ✅ [cat-19-link-health.sh](../../scripts/tests/browser/cat-19-link-health.sh) | ✅ [link-health-smoke.sh](../../scripts/smoke-tests/link-health-smoke.sh) |
| **noise-filtering** | ✅ [product-spec.md](feature/noise-filtering/product-spec.md) | ✅ [tech-spec.md](feature/noise-filtering/tech-spec.md) | ✅ [test-plan.md](feature/noise-filtering/test-plan.md) | ✅ [cat-20-noise-persistence.sh](../../scripts/tests/capture/cat-20-noise-persistence.sh) | ❌ None |
| **ai-web-pilot** | ✅ [product-spec.md](feature/ai-web-pilot/product-spec.md) | ✅ [tech-spec.md](feature/ai-web-pilot/tech-spec.md) | ✅ [test-plan.md](feature/ai-web-pilot/test-plan.md) | ✅ cat-13/14/15 (12 tests) | ❌ None |
| **flow-recording** | ✅ [product-spec.md](feature/flow-recording/product-spec.md) | ✅ [tech-spec.md](feature/flow-recording/tech-spec.md) | ✅ [test-plan.md](feature/flow-recording/test-plan.md) | ✅ [cat-18-recording.sh](../../scripts/tests/capture/cat-18-recording.sh) | ❌ None |
| **test-generation** | ✅ [product-spec.md](feature/test-generation/product-spec.md) | ✅ [tech-spec.md](feature/test-generation/tech-spec.md) | ✅ [test-plan.md](feature/test-generation/test-plan.md) | ✅ [cat-29-reproduction.sh](../../scripts/tests/workflows/cat-29-reproduction.sh) | ❌ None |

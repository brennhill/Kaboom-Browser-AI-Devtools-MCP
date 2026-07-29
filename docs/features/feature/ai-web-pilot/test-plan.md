---
feature: ai-web-pilot
status: active
tool: interact
mode: [navigate, execute_js, highlight, click, type, etc.]
version: 0.7.12
last-updated: 2026-02-09
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# AI Web Pilot — Test Plan

**Status:** ✅ Product Tests Defined | ✅ Tech Tests Designed | ✅ UAT Tests Implemented (12 tests across 3 categories)

---

## Overview: AI Web Pilot State Machine

The AI Web Pilot enables AI agents to control the browser. State transitions determine which actions are allowed:

```
Extension/Browser State:
  pilot_enabled: boolean (controlled by user/extension settings)

Server-side Gating:
  - Actions like navigate, execute_js, highlight require pilot_enabled=true
  - Actions fail with "pilot_disabled" error when false
  - Pilot state communicated via /sync endpoint from extension
  - State cached in session memory
```

---

## Product Tests

### Valid State Tests (Pilot ON)

- **Test:** navigate succeeds when pilot ON
  - **Given:** Extension reports `settings.pilot_enabled=true` via /sync
  - **When:** User/AI calls `interact({what: 'navigate', url: 'https://example.com'})`
  - **Then:** Action executed, browser navigates, data captured in buffers

- **Test:** execute_js succeeds when pilot ON
  - **Given:** Extension reports `settings.pilot_enabled=true` via /sync
  - **When:** User/AI calls `interact({what: 'execute_js', script: 'console.log(1)'})`
  - **Then:** Script executes in browser context, result returned

- **Test:** highlight succeeds when pilot ON
  - **Given:** Extension reports `settings.pilot_enabled=true` via /sync
  - **When:** User/AI calls `interact({what: 'highlight', selector: 'body'})`
  - **Then:** Element highlighted visually in browser

- **Test:** click succeeds when pilot ON
  - **Given:** Extension reports `settings.pilot_enabled=true` via /sync
  - **When:** User/AI calls `interact({what: 'click', selector: 'button'})`
  - **Then:** Button clicked, event fired, logs captured

- **Test:** type succeeds when pilot ON
  - **Given:** Extension reports `settings.pilot_enabled=true` via /sync, input focused
  - **When:** User/AI calls `interact({what: 'type', selector: 'input', text: 'hello'})`
  - **Then:** Text typed into input, value updated

### Invalid State Tests (Pilot OFF)

- **Test:** navigate fails when pilot OFF
  - **Given:** Extension reports `settings.pilot_enabled=false` (or uninitialized) via /sync
  - **When:** User/AI calls `interact({what: 'navigate', url: 'https://example.com'})`
  - **Then:** Error returned: `isError=true`, content includes "pilot_disabled" or "not enabled"

- **Test:** execute_js fails when pilot OFF
  - **Given:** Extension reports `settings.pilot_enabled=false` via /sync
  - **When:** User/AI calls `interact({what: 'execute_js', script: '...'})`
  - **Then:** Error returned with "pilot_disabled" message

- **Test:** highlight fails when pilot OFF
  - **Given:** Extension reports `settings.pilot_enabled=false` via /sync
  - **When:** User/AI calls `interact({what: 'highlight', selector: 'body'})`
  - **Then:** Error returned, no DOM manipulation

### State Transition Tests

- **Test:** Pilot OFF → ON enables previously-failing actions
  - **Given:** Extension starts with `pilot_enabled=false`
  - **When:** First navigate call fails (pilot OFF) → extension toggles to ON → second navigate called
  - **Then:** First fails with "pilot_disabled", second succeeds (state change respected)

- **Test:** Pilot ON → OFF disables previously-succeeding actions
  - **Given:** Extension starts with `pilot_enabled=true`
  - **When:** navigate succeeds → extension toggles to OFF → second navigate called
  - **Then:** First succeeds, second fails with "pilot_disabled"

### Startup & Extension Initialization Tests

- **Test:** Extension /sync payload with pilot_enabled required
  - **Given:** Extension sends POST to /sync with `settings: {pilot_enabled: false}`
  - **When:** /sync processed
  - **Then:** Server accepts payload, state cached

- **Test:** /sync accepts both pilot ON and OFF
  - **Given:** Two /sync payloads: one with `pilot_enabled: true`, one with `false`
  - **When:** Both sent to server
  - **Then:** Both accepted without error

- **Test:** Version mismatches handled gracefully
  - **Given:** Extension sends `/sync` with old version (5.7.0) or future version (5.9.0)
  - **When:** /sync processed
  - **Then:** Server accepts request regardless of version

### Concurrent Action Tests

- **Test:** Multiple pilot-gated actions in sequence respect state
  - **Given:** Pilot enabled, execute 5 actions (navigate, click, execute_js, type, highlight)
  - **When:** All actions called in sequence
  - **Then:** All succeed; no state corruption between calls

- **Test:** Rapid pilot state toggles don't cause race conditions
  - **Given:** Extension toggles pilot_enabled on/off rapidly (10 times)
  - **When:** Concurrent actions submitted
  - **Then:** Server state consistent, no undefined behavior

---

## Technical Tests

### Unit Tests

#### Coverage Areas:
- Pilot state cache initialization
- Pilot state updates from /sync payloads
- Pilot gating logic (is action allowed given pilot_enabled?)
- State transitions (OFF → ON, ON → OFF)
- Error message generation ("pilot_disabled" format)

**Test File:** `internal/pilot/pilot_test.go`

#### Key Test Cases:
1. `TestPilotStateInitialization` — Cache starts empty/disabled
2. `TestPilotStateFromSyncPayload` — Parse /sync to update pilot_enabled
3. `TestNavigateGatingLogic` — navigate blocked if pilot=false
4. `TestExecuteJsGatingLogic` — execute_js blocked if pilot=false
5. `TestHighlightGatingLogic` — highlight blocked if pilot=false
6. `TestStatePersistencePerSession` — Pilot state per session_id
7. `TestStateTransitions` — OFF→ON→OFF transitions handled
8. `TestErrorMessageFormat` — Error includes "pilot_disabled"
9. `TestConcurrentStateUpdates` — No race conditions with RWMutex
10. `TestDefaultState` — Uninitialized sessions have pilot=false (safe default)

### Integration Tests

#### Scenarios:

1. **Extension startup sequence:**
   - Extension boots, sends /sync with `pilot_enabled: false`
   - Server caches state for session
   - Server handlers check pilot state on action calls
   - → navigate call fails (pilot OFF)

2. **Pilot state toggle:**
   - Extension sends /sync: `pilot_enabled: false`
   - navigate call → fails
   - Extension sends /sync: `pilot_enabled: true`
   - navigate call → succeeds
   - → State change immediately respected

3. **Concurrent sessions (two users):**
   - User A: session_a, pilot ON
   - User B: session_b, pilot OFF
   - User A's navigate → succeeds
   - User B's navigate → fails
   - → States isolated per session

4. **Version compatibility:**
   - Extension v5.7.0 sends /sync
   - Server processes (ignores version mismatch)
   - Actions work as expected
   - → No version-gating required

**Test File:** `tests/integration/ai_web_pilot.integration.ts`

### UAT Tests

**Framework:** Bash scripts across 3 categories

**Total:** 12 tests (3 + 5 + 4)

#### Category 13: Pilot State Contract Tests (3 tests)

**File:** `/Users/brenn/dev/kaboom/scripts/tests/browser/cat-13-pilot-contract.sh`

| Cat | Test | Line | Scenario |
|-----|------|------|----------|
| 13.1 | navigate fails when pilot OFF (regression guard) | 15-42 | Would have CAUGHT the pilot state regression |
| 13.2 | Sync endpoint accepts both pilot ON and OFF | 44-72 | Contract: server accepts pilot state |
| 13.3 | execute_js fails when pilot OFF (double-check gating) | 74-94 | Ensures all pilot-dependent actions gated |

**Note:** These are regression tests for the "pilot state cache not initialized" bug. Cat-13 would have caught it.

#### Category 14: Extension Startup Sequence (5 tests)

**File:** `/Users/brenn/dev/kaboom/scripts/tests/browser/cat-14-extension-startup.sh`

| Cat | Test | Line | Scenario |
|-----|------|------|----------|
| 14.1 | Extension /sync payload has required fields | 15-47 | Contract: pilot_enabled, tracking_enabled, session_id |
| 14.2 | Extension transitions: pilot OFF → ON | 50-77 | Server handles state transitions |
| 14.3 | Extension tracking_enabled changes accepted | 80-107 | Tracking state updates |
| 14.4 | Server handles version mismatches gracefully | 110-137 | Old/new extension versions both work |
| 14.5 | Extension command result payload format | 140-150+ | Results sent in correct format |

#### Category 15: Pilot-Gated Actions Success Path (4 tests)

**File:** `/Users/brenn/dev/kaboom/scripts/tests/browser/cat-15-pilot-success-path.sh`

| Cat | Test | Line | Scenario |
|-----|------|------|----------|
| 15.1 | navigate succeeds when pilot ON + data captured | 16-50 | Action execution and data flow |
| 15.2 | execute_js succeeds when pilot ON | 53-79 | Pilot-gated action works in success case |
| 15.3 | highlight succeeds when pilot ON | 82-107 | DOM interaction enabled when pilot=true |
| 15.4 | navigate fails (OFF) then succeeds (ON) | 110-150+ | Pilot OFF→ON transition respected |

---

## Test Gaps & Coverage Analysis

### Scenarios NOT YET covered by cat-13/14/15 UAT:

The existing tests focus on **state caching, extension startup, and success paths**. Missing:

| Gap | Scenario | Severity | Recommended UAT Test |
|-----|----------|----------|----------------------|
| GH-1 | Rapid state toggles (ON→OFF→ON) | MEDIUM | Test 10 rapid /sync toggles, verify action gating changes |
| GH-2 | Concurrent sessions (user A ON, user B OFF) | HIGH | Two sessions, verify states isolated |
| GH-3 | Session persistence across multiple syncs | MEDIUM | Same session_id, multiple /sync calls, state updates properly |
| GH-4 | Default state (no /sync sent) | HIGH | Verify pilot=false by default (safe) |
| GH-5 | Action failures logged correctly | MEDIUM | Verify "pilot_disabled" error in logs |
| GH-6 | Data not captured if action blocked | MEDIUM | Verify failed navigate doesn't create logs |
| GH-7 | Extension reconnect (disconnect then reconnect) | HIGH | Verify state re-synced after extension reconnect |
| GH-8 | Long-running actions (navigate with slow page) | MEDIUM | Verify pilot state checked at start, not re-gated mid-action |

---

## Recommended Additional UAT Tests (cat-15-extended or separate)

### cat-15-pilot-state-transitions (NEW)

```
15.5 - Rapid state toggles: ON→OFF→ON→OFF (10x) → state always respected
15.6 - Long-running action starts with pilot ON, stays gated (not re-checked mid-action)
15.7 - Session state isolation: session_A pilot ON, session_B pilot OFF
15.8 - Default state: uninitialized session has pilot OFF (safe default)
15.9 - /sync updates state for same session_id (persistence of session)
```

### cat-15-pilot-failure-cases (NEW)

```
15.10 - Failed navigate (pilot OFF) doesn't create logs
15.11 - Failed execute_js logs include "pilot_disabled" message
15.12 - Error response has isError=true, content includes helpful message
15.13 - Multiple rapid failures (50x navigate calls with pilot OFF) all fail consistently
```

---

## State Machine Validation

The AI Web Pilot implements this state machine:

```
╔════════════════════════════════════════════════════════════════╗
║                     AI Web Pilot State Machine                 ║
╚════════════════════════════════════════════════════════════════╝

Pilot-Dependent Actions:
  - navigate(url)
  - execute_js(script)
  - highlight(selector)
  - click(selector)
  - type(selector, text)
  - key_press(key)

State (per session):
  pilot_enabled: boolean (from /sync extension payload)

Gating Logic:
  if action in [navigate, execute_js, highlight, ...]:
    if not pilot_enabled:
      return error("pilot_disabled", ...)
    else:
      execute action

State Transitions:
  Extension sends /sync: {settings: {pilot_enabled: true/false}}
  → Server caches state for session
  → Next action checks new state
  → State change is immediate (no buffering)

Safety:
  - Default: pilot_enabled = false (safe)
  - Only extension can change state (no user override)
  - State isolated per session (no cross-contamination)
```

### Tests validate:
- ✅ Gating logic (actions blocked when pilot=false)
- ✅ State transitions (OFF→ON→OFF)
- ✅ Session isolation (multiple sessions, different states)
- ⏳ Edge cases (rapid toggles, concurrent actions, defaults)

---

## Test Status Summary

| Test Type | Count | Status | Pass Rate | Coverage |
|-----------|-------|--------|-----------|----------|
| Unit | ~10 | ✅ Implemented | TBD | Gating logic, state cache |
| Integration | ~4 | ✅ Implemented | TBD | Session management, state transitions |
| **UAT/Acceptance** | **12** | ✅ **PASSING** | **100%** | **Contract, startup, success paths** |
| **Missing UAT** | **8+** | ⏳ **TODO** | **0%** | **Rapid toggles, concurrent sessions, defaults** |
| Manual Testing | N/A | ⏳ Manual step required | N/A | Browser + extension verification |

**Overall:** ✅ **Core Gating Logic Validated** | ⏳ **Edge Cases & Concurrency Tests Recommended**

---

## Running the Tests

### UAT (Pilot State Contract & Extension Startup)

```bash
# Run cat-13 (Pilot state contract tests)
./scripts/tests/browser/cat-13-pilot-contract.sh 7900 /dev/null

# Run cat-14 (Extension startup sequence)
./scripts/tests/browser/cat-14-extension-startup.sh 7901 /dev/null

# Run cat-15 (Pilot-gated actions success path)
./scripts/tests/browser/cat-15-pilot-success-path.sh 7902 /dev/null
```

### Full Test Suite

```bash
# Run comprehensive suite (all categories)
./scripts/test-all-tools-comprehensive.sh
```

---

## Known Limitations

1. **No user override** — Only extension controls pilot_enabled (by design, security)
2. **No action-level gating** — All pilot-dependent actions use same on/off switch
3. **No pilot timeout** — Once ON, stays ON until extension toggles OFF
4. **No audit trail** — Pilot state changes not logged (no "AI turned on at T" record)

---

## Sign-Off

| Area | Status | Notes |
|------|--------|-------|
| Product Tests Defined | ✅ | Valid states (ON/OFF), transitions, concurrent sessions |
| Tech Tests Designed | ✅ | Unit, integration, UAT frameworks identified |
| UAT Tests Implemented | ✅ | **12 tests across cat-13/14/15 (100% passing)** |
| **Edge Case Tests** | ⏳ | **Recommended: cat-15-extended for rapid toggles, concurrent sessions** |
| **Overall Readiness** | ✅ | **Core state machine validated. Edge cases optional but recommended.** |


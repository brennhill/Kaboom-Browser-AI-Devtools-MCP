---
feature: noise-filtering
status: active
tool: configure
mode: noise_rule
version: 0.7.12
last-updated: 2026-02-09
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Noise Filtering — Test Plan

**Status:** ✅ Product Tests Defined | ✅ Tech Tests Designed | ✅ UAT Tests Implemented (10 tests)

---

## Product Tests

### Valid State Tests

- **Test:** Built-in noise rules applied by default
  - **Given:** Server starts, no custom rules configured
  - **When:** User calls `configure({what: 'noise_rule', noise_action: 'list'})`
  - **Then:** Response contains ~50 built-in rules (prefixed with 'builtin_'); chrome-extension, favicon, HMR, analytics rules all present

- **Test:** Custom rule added and filters subsequent entries
  - **Given:** User calls `configure({what: 'noise_rule', noise_action: 'add', rules: [{category: 'console', match_spec: {message_regex: 'test.*pattern'}, classification: 'test'}]})`
  - **When:** Console entry matching "test.*pattern" is logged
  - **Then:** Entry is filtered from `observe` responses; statistics show 1 filtered

- **Test:** User rules survive session restart
  - **Given:** User adds custom rule, daemon is killed and restarted
  - **When:** User calls `list` again
  - **Then:** Custom rule still present (persisted to ~/.kaboom/noise/rules.json)

- **Test:** Framework detection activates framework rules
  - **Given:** Network requests contain `/@vite/client` or `_next/static` signatures
  - **When:** Framework auto-detected
  - **Then:** Vite or Next.js specific noise rules become active, filtering framework overhead

- **Test:** Auth responses (401/403) never filtered
  - **Given:** Network response returns 401 on URL matching analytics noise rule pattern
  - **When:** Response processed
  - **Then:** Entry appears in `observe` output (NOT filtered) despite URL match

### Edge Case Tests (Negative)

- **Test:** Invalid regex pattern silently skipped (no panic)
  - **Given:** User calls `configure({what: 'noise_rule', noise_action: 'add', rules: [{category: 'console', match_spec: {message_regex: '[invalid'}, classification: 'test'}]})`
  - **When:** Rule added and later entries tested
  - **Then:** Rule has nil regex, never matches, server doesn't crash

- **Test:** Max 100 rules enforced
  - **Given:** User adds rules until count reaches 100, then attempts to add 1 more
  - **When:** 101st rule added
  - **Then:** Rule silently dropped, count remains 100

- **Test:** Built-in rules cannot be removed
  - **Given:** User calls `configure({what: 'noise_rule', noise_action: 'remove', rule_id: 'builtin_favicon'})`
  - **When:** Removal attempted
  - **Then:** Error returned: "Cannot remove built-in rule: builtin_favicon"

- **Test:** Application errors from localhost never auto-detected
  - **Given:** 20+ identical console errors from localhost:3000
  - **When:** `configure({what: 'noise_rule', noise_action: 'auto_detect'})` called
  - **Then:** No rule proposed for localhost error pattern (protected by DL-2)

- **Test:** Empty buffer with noise rules
  - **Given:** Buffers empty, noise rules configured
  - **When:** `configure({what: 'noise_rule', noise_action: 'auto_detect'})` called
  - **Then:** Returns empty proposals, no crash

### Concurrent/Race Condition Tests

- **Test:** Concurrent rule add and buffer read
  - **Given:** One goroutine adding rules, another reading buffers for filtering
  - **When:** Both operations simultaneous
  - **Then:** No race condition, RWMutex prevents data corruption

- **Test:** Reset during active filtering
  - **Given:** Filtering in progress, user calls `configure({what: 'noise_rule', noise_action: 'reset'})`
  - **When:** Reset completes (removes all user/auto rules)
  - **Then:** Only built-ins remain; subsequent entries not affected by deleted rules

### Failure & Recovery Tests

- **Test:** Reset clears persisted user rules
  - **Given:** User has added 5 custom rules, daemon restarted
  - **When:** `configure({what: 'noise_rule', noise_action: 'reset'})` called, daemon killed and restarted
  - **Then:** Rules file reset to empty; only built-ins present after restart

- **Test:** Corrupted persisted rules file handled gracefully
  - **Given:** Manually corrupt ~/.kaboom/noise/rules.json with invalid JSON
  - **When:** Daemon starts
  - **Then:** Server starts successfully, skips corrupted file, uses only built-ins

- **Test:** Dismiss noise creates rule correctly
  - **Given:** User calls `configure({what: 'dismiss_noise', pattern: 'specific-app-warning', reason: 'Noisy during testing'})`
  - **When:** Rule created
  - **Then:** Rule has ID starting with 'dismiss_', classification='dismissed', immediately filters matching entries

---

## Technical Tests

### Unit Tests

#### Coverage Areas:
- Built-in rule initialization (all ~50 rules compiled)
- Pattern matching (regex compiled once, matching fast)
- Rule add/remove operations (ID generation, persistence)
- Auto-detection heuristics (frequency, source, periodicity, entropy)
- Security invariants (auth responses, app errors protection)
- Framework detection (React, Next.js, Vite signatures)

**Test File:** `internal/noise/noise_test.go`

#### Key Test Cases:
1. `TestNewNoiseConfigHasAllBuiltins` — Verify ~50 built-in rules present
2. `TestConsoleNoiseMatching` — chrome-extension:// sources, HMR messages, framework overhead
3. `TestNetworkNoiseMatching` — favicon.ico, analytics URLs, CORS preflights
4. `TestWebSocketNoiseMatching` — HMR WebSocket URLs
5. `TestAuth401Never Filtered` — 401/403 always preserved
6. `TestApplicationErrors NotAutoDetected` — localhost errors protected
7. `TestAutoDetectFrequency` — 15 identical messages → confidence > 0.7
8. `TestAutoDetectPeriodicity` — Regular intervals ±10% jitter → infrastructure
9. `TestMaxRulesEnforced` — 100 rule limit
10. `TestInvalidRegexSkipped` — No panic on [invalid regex
11. `TestFrameworkDetection` — React/Next.js/Vite signature detection
12. `TestPersistenceRoundTrip` — Rules saved and loaded from ~/.kaboom/noise/rules.json

### Integration Tests

#### Scenarios:

1. **Built-in rules active without user config:**
   - Server starts (no user rules)
   - Browser generates chrome-extension warnings, favicon 404s, HMR messages, analytics pings
   - `observe` tool called
   - → Built-in rules filter appropriately; real app errors NOT filtered

2. **User adds rule → filters new entries:**
   - `configure({what: 'noise_rule', noise_action: 'add', rules: [custom_rule]})`
   - Generate new entry matching rule pattern
   - `observe` called
   - → New entry filtered, statistics show count increased

3. **Auto-detect proposes and auto-applies rules:**
   - Generate 15 identical console messages
   - `configure({what: 'noise_rule', noise_action: 'auto_detect'})`
   - → Proposals returned with confidence scores
   - → Rules with confidence >= 0.9 automatically added
   - → Subsequent matching entries filtered

4. **Persistence round-trip:**
   - Add custom rules
   - Kill daemon
   - Restart daemon
   - `configure({what: 'noise_rule', noise_action: 'list'})`
   - → Custom rules still present (loaded from file)

5. **Security invariants (no data leak):**
   - Add rule matching analytics domain
   - Send 401 response on that domain
   - `configure({what: 'noise_rule', noise_action: 'list'})` + count metrics
   - → 401 not filtered (security protected)
   - → Statistics don't expose sensitive details from filtered entries

**Test File:** `tests/integration/noise_filtering.integration.ts`

### UAT Tests

**Framework:** Bash scripts (see cat-20-noise-persistence.sh)

**File:** `/Users/brenn/dev/kaboom/scripts/tests/capture/cat-20-noise-persistence.sh`

#### 10 Tests Implemented:

| Cat | Test | File | Line | Scenario |
|-----|------|------|------|----------|
| 20.1 | configure/add creates rule with user_N ID | cat-20 | 24-46 | Rules assignable IDs and persisted |
| 20.2 | Rules persisted to .kaboom/noise/rules.json | cat-20 | 49-63 | File creation and valid JSON |
| 20.3 | Persisted file schema correct (version, next_user_id, rules) | cat-20 | 66-82 | File structure validates |
| 20.4 | No built-in rules in persisted file | cat-20 | 85-98 | Built-ins always fresh from code |
| 20.5 | Rules survive server restart | cat-20 | 101-126 | Rules reloaded on cold start |
| 20.6 | ID counter prevents collisions | cat-20 | 129-155 | Unique IDs after restart (user_2, not user_1 again) |
| 20.7 | RemoveRule persists immediately | cat-20 | 158-201 | Deleted rules stay deleted after restart |
| 20.8 | Reset removes all user rules and persists empty state | cat-20 | 204-244 | Reset clears persistence |
| 20.9 | Corrupted file handled gracefully | cat-20 | 247-264 | Server starts, skips invalid rules |
| 20.10 | Built-in rules always fresh (~45 rules) | cat-20 | 267-291 | Built-ins loaded from code after corruption recovery |

#### Test Coverage:
- Persistence: rule add, removal, reset
- Restart resilience: rule survival, ID uniqueness
- Corruption handling: invalid JSON tolerance
- Built-in freshness: always loaded from code

---

## Test Gaps & Coverage Analysis

### Scenarios in Tech Spec NOT YET covered by cat-20 UAT:

The cat-20 tests focus on **persistence & restart scenarios**. They don't test the actual noise-filtering logic (matching). Missing:

| Gap | Scenario | Severity | Recommended UAT Test |
|-----|----------|----------|----------------------|
| GH-1 | Actual console/network filtering | CRITICAL | Test with real browser noise (chrome-ext, analytics, HMR) + verify filtered |
| GH-2 | Framework detection (React, Vite, Next.js) | CRITICAL | Verify framework-specific rules activate on signature detection |
| GH-3 | Auth response protection (401/403) | **CRITICAL** | Verify 401/403 NEVER filtered regardless of rules |
| GH-4 | App error protection (localhost) | **CRITICAL** | Verify console errors from app code never auto-detected |
| GH-5 | Auto-detect confidence thresholds | HIGH | Test confidence >= 0.9 auto-applies, < 0.9 suggests only |
| GH-6 | Periodicity detection (infrastructure) | MEDIUM | Test regular intervals ±10% detected as infrastructure |
| GH-7 | Entropy scoring (low-entropy filtering) | MEDIUM | Test repetitive static messages flagged with low entropy |
| GH-8 | Dismiss noise rule creation | MEDIUM | Test dismiss_noise creates rule with dismiss_* prefix |
| GH-9 | Statistics accuracy | MEDIUM | Test filtered counts per rule in list response |
| GH-10 | Concurrent read/write with RWMutex | HIGH | Verify no race conditions (go test -race) |

### **CRITICAL DATA LEAK TESTS NOT YET IMPLEMENTED:**

Per QA Plan section DL-1 through DL-3 (marked CRITICAL):

- [ ] **DL-1:** 401/403 responses on analytics URLs must NOT be filtered
- [ ] **DL-2:** Console errors from localhost/app domain never auto-detected as noise
- [ ] **DL-3:** Dismiss noise with broad regex (e.g., `.*error.*`) doesn't suppress security events

**Recommended:** New test category cat-20-security with explicit data leak verification

---

## Recommended Additional UAT Tests (cat-20-extended or separate)

### cat-20-filtering-logic (NEW)

```
20.11 - Built-in chrome-extension rule filters browser warnings
20.12 - Built-in favicon rule filters 404s on /favicon.ico
20.13 - Built-in analytics rule filters segment.io requests
20.14 - Built-in HMR rule filters [vite] console messages
20.15 - Framework detection: React detected → React rules active
20.16 - Framework detection: Vite detected → Vite rules active
```

### cat-20-security (NEW - DATA LEAK PROTECTION)

```
20.20 - 401 response on analytics URL NOT filtered
20.21 - 403 response on any noise-matching URL NOT filtered
20.22 - Console error from localhost never auto-detected
20.23 - Auto-detect skips application source errors
20.24 - 5xx error never filtered as noise
```

### cat-20-auto-detect (NEW)

```
20.30 - 15 identical messages → auto-detect proposes rule
20.31 - High-confidence (>= 0.9) rules auto-applied
20.32 - Low-confidence (< 0.9) rules returned as suggestions
20.33 - Periodicity detection: requests at 10s intervals → infrastructure
20.34 - Existing rules not duplicated by auto-detect
```

---

## Test Status Summary

| Test Type | Count | Status | Pass Rate | Coverage |
|-----------|-------|--------|-----------|----------|
| Unit | ~12 | ✅ Implemented | TBD | Rule logic, matching, persistence |
| Integration | ~5 | ✅ Implemented | TBD | Workflows (add, remove, reset, auto-detect) |
| **UAT/Acceptance** | **10** | ✅ **PASSING** | **100%** | **Persistence, restart resilience, file schema** |
| **Missing UAT** | **10+** | ⏳ **TODO** | **0%** | **Filtering logic, security, auto-detect** |
| Manual Testing | N/A | ⏳ Manual step required | N/A | Browser verification of filtered entries |

**Overall:** ✅ **Persistence Tests Complete** | ⚠️ **Data Leak Tests CRITICAL** | ⏳ **Filtering Logic Tests Needed**

---

## Data Leak Analysis (Security)

### CRITICAL Tests (DL-1, DL-2, DL-3) **NOT YET IMPLEMENTED IN UAT**

| Test | Risk | Current Status | Priority |
|------|------|-----------------|----------|
| **DL-1** | Auth failures (401/403) hidden by noise rules | ⏳ **NOT TESTED** | **CRITICAL** |
| **DL-2** | Application errors suppressed by auto-detect | ⏳ **NOT TESTED** | **CRITICAL** |
| **DL-3** | Dismissed patterns hiding security events | ⏳ **NOT TESTED** | **CRITICAL** |
| DL-4 | Noise statistics leaking internal URLs | ⏳ Not tested | High |
| DL-5 | Built-in rules hiding real failures | ✅ Mitigated by rule design | High |
| DL-6 | Filtered data still in raw buffers | ✅ By design (filtering read-time only) | High |
| DL-7 | Auto-detect confidence too low | ⏳ Not tested | Medium |
| DL-8 | Custom regex capturing too broadly | ⏳ Not tested | Medium |
| DL-9 | Noise rule reason leaking sensitive info | ⏳ Not tested | Low |

**Action Required:** Implement cat-20-security tests immediately before feature release.

---

## Running the Tests

### UAT (Persistence & Restart)

```bash
# Run all 10 persistence tests
./scripts/tests/capture/cat-20-noise-persistence.sh 7890 /dev/null

# Or with output to file
./scripts/tests/capture/cat-20-noise-persistence.sh 7890 ./cat-20-results.txt
```

### Full Test Suite

```bash
# Run comprehensive suite (all categories)
./scripts/test-all-tools-comprehensive.sh
```

---

## Known Limitations (v5.8.1)

1. **User rules only persisted** — Built-ins always loaded from code (by design)
2. **Max 100 rules** — Additional rules silently dropped (no dynamic expansion)
3. **No cross-session queries** — Built-in JSONL; v2 may add bbolt database for indexed queries
4. **No rule scheduling** — Rules always active; no time-based activation

---

## Sign-Off

| Area | Status | Notes |
|------|--------|-------|
| Product Tests Defined | ✅ | Valid states, edge cases, concurrency, recovery |
| Tech Tests Designed | ✅ | Unit, integration, UAT frameworks identified |
| UAT Tests Implemented | ✅ | **10 tests in cat-20 (100% passing)** |
| **Data Leak Tests** | ⚠️ | **CRITICAL: DL-1, DL-2, DL-3 NOT YET TESTED** |
| **Filtering Logic Tests** | ⏳ | **Recommended: New cat-20-filtering-logic (10+ tests)** |
| **Overall Readiness** | ⚠️ | **Persistence validated. CRITICAL data leak tests required before release.** |

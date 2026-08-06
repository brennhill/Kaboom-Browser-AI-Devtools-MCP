---
feature: link-health
status: active
tool: analyze
mode: link_health
version: 0.7.12
last-updated: 2026-02-09
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Link Health Checker — Test Plan

**Status:** ✅ Product Tests Defined | ✅ Tech Tests Designed | ✅ UAT Tests Implemented (19 tests)

---

## Product Tests

### Valid State Tests

- **Test:** Check links on current page returns results grouped by status
  - **Given:** Browser on a webpage with 20 mixed links (200 OK, 404 broken, 403 auth-required, 301 redirect)
  - **When:** User calls `analyze({what: 'link_health'})`
  - **Then:** Server returns session ID, correlation_id for async tracking, summary with counts (ok, broken, requires_auth, redirect, timeout)

- **Test:** Distinguish auth-required (403) from genuinely broken (404)
  - **Given:** Page with two links: one returns 403, one returns 404
  - **When:** Link check completes
  - **Then:** 403 link marked with `code='requires_auth'`, 404 link marked with `code='broken'` (different categories)

- **Test:** Detect redirect chains (v1: report first, v2: follow)
  - **Given:** Link returns 301 redirect with Location header
  - **When:** Link check completes
  - **Then:** Response includes `redirect_to` field with target URL, code='redirect'

- **Test:** Track external vs internal links separately
  - **Given:** Page with 15 internal links (same domain), 5 external links (different domain)
  - **When:** Link check completes
  - **Then:** Results include `is_external` flag; summary shows separate counts

- **Test:** Session ID uniqueness
  - **Given:** User starts two concurrent link checks on different pages
  - **When:** Both checks initiated
  - **Then:** Each gets unique session ID (timestamp + domain); sessions don't interfere

### Edge Case Tests (Negative)

- **Test:** Timeout on slow/unreachable links
  - **Given:** Link points to unreachable host (e.g., invalid domain)
  - **When:** Check timeout threshold (15s per link) exceeded
  - **Then:** Result marked with `code='timeout'`, status=null, error message provided

- **Test:** DNS failure treated as broken
  - **Given:** Link domain doesn't resolve
  - **When:** Link check executes
  - **Then:** Logged as timeout (treated as broken for practical purposes)

- **Test:** Large number of links (1000+) handled efficiently
  - **Given:** Page with 1000 links
  - **When:** Link check requested
  - **Then:** Check completes without OOM, results streamed to disk, no stalling UI

- **Test:** Same link appears multiple times on page
  - **Given:** Page links to `/about` in 3 different places
  - **When:** Link check completes
  - **Then:** URL checked once, but all 3 occurrences reported in results (with deduplicated check)

- **Test:** Port blocking (corporate firewall)
  - **Given:** Link to port that's blocked by network policy
  - **When:** Link check attempts connection
  - **Then:** Logged as timeout, user can see "connection refused" in error

### Concurrent/Race Condition Tests

- **Test:** Multiple workers checking links concurrently
  - **Given:** 20 concurrent workers processing 100-link queue
  - **When:** Each worker pulls next link and appends result to disk
  - **Then:** All results appended without data corruption, counts match 100 total

- **Test:** Concurrent session isolation
  - **Given:** Two users start link checks simultaneously on different domains
  - **When:** Both sessions write to disk concurrently
  - **Then:** Session directories separate, no file contention, both complete successfully

- **Test:** Queue depletion race
  - **Given:** Last few links being processed by multiple workers
  - **When:** Queue becomes empty mid-check
  - **Then:** Workers gracefully exit, final results written, no dangling processes

### Failure & Recovery Tests

- **Test:** Crash recovery (warm start)
  - **Given:** Server crashed after checking 15/42 links (state and visited.jsonl on disk)
  - **When:** User calls `observe('link_health', {action: 'resume', session_id: '...'})`
  - **Then:** Server rebuilds state from disk, continues with remaining 27 links, merges results

- **Test:** Disk full during append
  - **Given:** Disk runs out of space while appending to results.jsonl
  - **When:** Write fails
  - **Then:** Error logged to stderr, session marked "degraded", results still in memory/response, check continues

- **Test:** Corrupted state.json on resume
  - **Given:** state.json file exists but corrupted (malformed JSON)
  - **When:** User attempts resume
  - **Then:** Server detects corruption, creates new session ID with different timestamp, starts fresh check

- **Test:** User cancels mid-check
  - **Given:** Link check in progress, 25/42 links completed
  - **When:** User calls `observe('link_health', {action: 'cancel', session_id: '...'})`
  - **Then:** Workers drain queue (no new pops), results so far returned, state marked "cancelled"

---

## Technical Tests

### Unit Tests

#### Coverage Areas:
- URL extraction from DOM (selector matching, deduplication)
- Session ID generation (timestamp + domain format)
- Status code categorization (200/3xx/4xx/5xx → code values)
- Worker pool management (queue push/pop, thread safety)
- Persist logic (append-only JSONL, atomic writes)

**Test File:** `internal/link_health/link_health_test.go`

#### Key Test Cases:
1. `TestExtractLinksFromDOM` — Parse various link formats (href, data-attr, invalid)
2. `TestSessionIDGeneration` — Unique IDs, collision detection
3. `TestStatusCodeToCategoryCode` — 200→'ok', 404→'broken', 403→'requires_auth', 301→'redirect', timeout→'timeout'
4. `TestWorkerQueueThreadSafety` — Concurrent push/pop without races
5. `TestAppendOnlyPersistence` — JSONL writes atomic, no partial lines
6. `TestCrashRecovery` — Rebuild state from corrupted files

### Integration Tests

#### Scenarios:

1. **End-to-end workflow (cold start):**
   - observe('link_health', {links: [url1, url2, ...], domain: 'example.com'})
   - → Server creates session, extracts links, spawns 20 workers
   - → Workers check links in parallel
   - → Results appended to results.jsonl
   - → State checkpointed every 10 results
   - → observe tool returns summary + partial results (async)

2. **Resume after crash:**
   - observe('link_health') → session 1 starts, checks 15/42
   - Server crashes, disk state preserved
   - Daemon restarts
   - observe('link_health', {action: 'resume', session_id: 'sess1'})
   - → State rebuilt from disk (visited.jsonl + results.jsonl)
   - → Queue repopulated with remaining 27 links
   - → Results merged and returned

3. **Concurrent sessions (two users, different domains):**
   - User A calls observe('link_health', {domain: 'site-a.com'}) → sess_a created
   - User B calls observe('link_health', {domain: 'site-b.com'}) → sess_b created
   - Both run in parallel
   - → Directory structure: ~/.kaboom/crawls/sess_a/, ~/.kaboom/crawls/sess_b/
   - → Results independent, no interference

4. **Large link set (1000+ links):**
   - observe('link_health', {links: [url1, ..., url1000]})
   - → Check completes without OOM
   - → Results streamed to disk incrementally
   - → No single point of buffer bloat

**Test File:** `tests/integration/link_health.integration.ts`

### UAT Tests

**Framework:** Bash scripts (see cat-19-link-health.sh)

**File:** `/Users/brenn/dev/kaboom/scripts/tests/browser/cat-19-link-health.sh`

#### 19 Tests Implemented:

| Cat | Test | File | Line | Scenario |
|-----|------|------|------|----------|
| 19.1 | analyze returns correlation_id | cat-19-link-health.sh | 19-37 | Verify link_health returns valid correlation_id for async tracking |
| 19.2 | accepts timeout_ms parameter | cat-19-link-health.sh | 43-57 | Optional parameters don't cause errors |
| 19.3 | accepts max_workers parameter | cat-19-link-health.sh | 63-73 | Worker count parameter validation |
| 19.4 | status='queued' response | cat-19-link-health.sh | 79-93 | Async operation correctly marked as queued |
| 19.5 | returns hint for command_result usage | cat-19-link-health.sh | 99-117 | Response includes guidance for async result tracking |
| 19.6 | dispatcher routes correctly | cat-19-link-health.sh | 125-133 | analyze tool correctly dispatches to link_health handler |
| 19.7 | rejects missing 'what' param | cat-19-link-health.sh | 139-153 | Validates required parameters |
| 19.8 | rejects invalid mode | cat-19-link-health.sh | 159-173 | Invalid modes return error with valid mode list |
| 19.9 | handles invalid JSON | cat-19-link-health.sh | 181-195 | Malformed JSON caught and reported clearly |
| 19.10 | handles invalid timeout_ms | cat-19-link-health.sh | 201-213 | Non-numeric values handled gracefully |
| 19.11 | ignores unknown parameters | cat-19-link-health.sh | 219-227 | Lenient parsing allows extensibility |
| 19.12 | 5 concurrent calls | cat-19-link-health.sh | 235-258 | Stress test: multiple simultaneous requests |
| 19.13 | 50 repeated calls | cat-19-link-health.sh | 264-280 | Memory leak detection via iteration |
| 19.14 | correlation_id format check | cat-19-link-health.sh | 288-302 | Verify 'link_health_' prefix convention |
| 19.15 | response has content blocks | cat-19-link-health.sh | 308-322 | MCP protocol structure validation |
| 19.16 | response is valid JSON | cat-19-link-health.sh | 328-336 | JSON parse validation |
| 19.17 | link_health listed as valid mode | cat-19-link-health.sh | 344-359 | Tool discovery via error response |
| 19.18 | MCP protocol compliance | cat-19-link-health.sh | 365-379 | Required fields (isError, content) present |
| 19.19 | tools/call invocation works | cat-19-link-health.sh | 385-394 | Standard MCP method invocation |

#### Smoke Tests:

**File:** `/Users/brenn/dev/kaboom/scripts/smoke-tests/links/link-health-smoke.sh`

(Status: Exists with additional quick-check scenarios)

---

## Test Gaps & Coverage Analysis

### Scenarios in Tech Spec NOT YET covered by cat-19 UAT:

The cat-19 tests focus on **API contract & MCP protocol validation**. They don't test the actual link-checking logic. Missing:

| Gap | Scenario | Severity | Recommended UAT Test |
|-----|----------|----------|----------------------|
| GH-1 | Actual HTTP requests to real URLs | CRITICAL | New test: Start daemon, call analyze, verify network requests made to test URLs |
| GH-2 | Status code categorization accuracy | CRITICAL | Verify 200→'ok', 404→'broken', 403→'requires_auth', 301→'redirect' |
| GH-3 | Persistent storage (results.jsonl) | HIGH | Verify files created in ~/.kaboom/crawls/{session_id}/ |
| GH-4 | Crash recovery (warm start) | HIGH | Kill daemon mid-check, restart, verify resume works |
| GH-5 | External vs internal link tracking | MEDIUM | Verify is_external flag correctly set |
| GH-6 | Worker concurrency (20 workers) | MEDIUM | Monitor concurrent HTTP requests, verify queue management |
| GH-7 | Timeout handling (15s per link) | MEDIUM | Test with slow endpoint, verify timeout behavior |
| GH-8 | CORS detection edge cases | HIGH | Test cross-origin requests, CORS preflights |
| GH-9 | Session ID collision prevention | MEDIUM | Start multiple checks, verify unique session IDs |
| GH-10 | Disk full error handling | LOW | Mock disk full, verify graceful degradation |

### Suggested New UAT Tests (cat-19-extended):

1. **cat-19-links-links-actual** — Integration test with mock HTTP server
   - Start mock server with test endpoints (200, 404, 403, 301, timeout)
   - Call analyze('link_health') with those URLs
   - Verify results.jsonl created with correct categories

2. **cat-19-links-crash-recovery** — Warm start scenario
   - Start check, kill daemon after 5 requests
   - Restart daemon
   - Call resume on same session_id
   - Verify remaining links checked and results merged

3. **cat-19-links-persistence** — File system validation
   - Start check, wait for completion
   - Verify ~/.kaboom/crawls/{session_id}/ exists with:
     - visited.jsonl (one URL per line)
     - results.jsonl (one JSON result per line)
     - state.json (valid JSON with status, counts, queue)
     - metadata.json (domain, start_url, extracted_at)

---

## Test Status Summary

| Test Type | Count | Status | Pass Rate | Coverage |
|-----------|-------|--------|-----------|----------|
| Unit | ~6 | ✅ Implemented | TBD | Core functions |
| Integration | ~4 | ✅ Implemented | TBD | Workflows (cold start, resume, concurrent) |
| **UAT/Acceptance** | **19** | ✅ **PASSING** | **100%** | **API contract, MCP protocol** |
| **Missing UAT** | **10** | ⏳ **TODO** | **0%** | **HTTP logic, persistence, recovery** |
| Manual Testing | N/A | ⏳ Manual step required | N/A | Browser-based verification |

**Overall:** ✅ **API Contract Tests Complete** | ⏳ **HTTP Logic Tests Needed**

---

## Running the Tests

### UAT (API Contract Validation)

```bash
# Run all 19 link-health API contract tests
./scripts/tests/browser/cat-19-link-health.sh 7890 /dev/null

# Or with output to file
./scripts/tests/browser/cat-19-link-health.sh 7890 ./cat-19-results.txt
```

### Smoke Tests

```bash
# Quick sanity checks
./scripts/smoke-tests/links/link-health-smoke.sh
```

### Full Test Suite

```bash
# Run comprehensive suite (all categories)
./scripts/uat/runners/test-all-tools-comprehensive.sh
```

---

## Data Leak Analysis (Security)

**Risk Areas:** None identified for v1 (current page only, no persistent cross-session storage)

---

## Known Limitations (v1)

1. **Current page only** — Does not crawl linked pages (v2 feature)
2. **First 301 reported** — Redirects not followed (v2 feature)
3. **No authentication** — 403 responses logged as-is (can't retry with credentials)
4. **No HEAD requests** — Full GET for all checks (no optimization for large files)

---

## Sign-Off

| Area | Status | Notes |
|------|--------|-------|
| Product Tests Defined | ✅ | Valid states, edge cases, concurrency, recovery |
| Tech Tests Designed | ✅ | Unit, integration, UAT frameworks identified |
| UAT Tests Implemented | ✅ | **19 tests in cat-19-link-health.sh (100% passing)** |
| **HTTP Logic Tests** | ⏳ | **Recommended: Add cat-19-extended tests for persistence & recovery** |
| **Overall Readiness** | ✅ | **API contract validated. Link-checking logic needs UAT coverage.** |


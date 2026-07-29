---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Test Quality Audit Report

**Project:** Kaboom MCP
**Date:** 2026-02-14
**Auditor:** Senior QA Engineer (Automated)
**Scope:** All test suites -- smoke tests, Go unit tests, regression tests

---

## Summary

| Severity | Count |
|----------|-------|
| CRITICAL | 8     |
| HIGH     | 16    |
| MEDIUM   | 22    |
| LOW      | 11    |
| **Total** | **57** |

**Test Suites Audited:**
- 15 smoke test files (`scripts/smoke-tests/01-*.sh` through `15-*.sh`)
- 2 test framework files (`scripts/tests/framework/framework.sh`, `scripts/smoke-tests/framework-smoke.sh`)
- ~100 Go unit test files across `cmd/browser-agent/` and `internal/`
- 13 regression test files (`tests/regression/`)
- 29 UAT category test files (`scripts/tests/cat-*.sh`)
- 0 TypeScript test files found (none exist in `src/`)

---

## Part 1: Smoke Tests (`scripts/smoke-tests/`)

### File: `scripts/smoke-tests/01-bootstrap.sh`

**[MEDIUM] Test 1.1 -- Fixed sleep instead of polling for port release**
Line 15: `sleep 0.5` after `kill_server` before checking port availability. On slow machines or under load, 0.5s may not be enough for the OS to release the port. This creates a flaky test that can false-fail on busy CI machines.

**[LOW] Test 1.2 -- Unexplained `sleep 2` before health check**
Line 63: There is a hard `sleep 2` before `wait_for_health` even though the daemon was already started and verified healthy during test 1.1 restart. This wastes 2 seconds per run and masks a potential startup timing issue.

**[MEDIUM] Test 1.4 -- Loose URL matching**
Line 145: `grep -qi "example.com"` would match any URL containing "example.com" as a substring, including `not-example.com`, `example.com.evil.tld`, or URLs in error messages that mention example.com. Should use an exact URL match or at least anchor the regex.

---

### File: `scripts/smoke-tests/02-core-telemetry.sh`

**[HIGH] Test 2.2 -- Fallback pass that weakens the assertion**
Lines 79-80: If the injected button's ID is not found in the response, the test falls back to a generic `grep -qi "click"` and passes. The word "click" could appear anywhere in the response text (e.g., in a description, timestamp format, or unrelated log). This fallback means the test can pass even if the button click was never captured -- it only proves the daemon returned _something_ containing "click."

**[HIGH] Test 2.3 -- Same fallback pattern as 2.2**
Lines 110-111: Falls back to `grep -qi "input\|change"` and passes. Same weakness: the test does not confirm the specific injected input was tracked.

**[MEDIUM] Test 2.4 -- Extremely loose success check**
Line 131: `grep -qi "complete\|success\|highlighted"` accepts any response containing any of these common English words. This provides almost no signal about whether the highlight action actually executed.

**[MEDIUM] Test 2.7 -- Count-based assertion without specificity**
Lines 376-377: Uses `grep -ci "input\|change\|focus"` to count matches case-insensitively. This counts occurrences of common words, not specific form actions. A response containing "No input data available" would still increment the counter and lead to a pass.

---

### File: `scripts/smoke-tests/03-observe-modes.sh`

**[LOW] Test 3.1 -- Fixed 3-second sleep for Web Vitals**
Uses `sleep 3` waiting for vitals to be collected. On slow connections or complex pages, vitals may take longer. On fast machines, this wastes time. Should poll with backoff.

**[MEDIUM] Test 3.4 -- Only checks bundle count, not content**
Verifies `error_bundles` returned a count > 0 but never verifies the bundle content matches the errors seeded in test 2.1. Could pass with stale bundles from a previous run.

---

### File: `scripts/smoke-tests/04-network-websocket.sh`

**[CRITICAL] Test 4.1 -- Depends on external service (binance.com)**
The WebSocket capture test navigates to `binance.com` and waits for WebSocket messages. This test is fundamentally fragile:
1. Binance may be unavailable (downtime, geo-blocking, rate limiting)
2. Binance may change their WebSocket behavior
3. Uses `sleep 5` fixed delay -- no guarantee WS messages arrive in time
4. CI environments may not have internet access

Any external dependency in a test suite is a ticking time bomb. This should use a local WebSocket server.

**[MEDIUM] Test 4.2 -- Regex-based field presence check**
Uses regex grep on the response text instead of proper JSON parsing via `jq` to verify network waterfall fields. Can false-match on field names embedded in URLs or values.

---

### File: `scripts/smoke-tests/05-interact-dom.sh`

**[HIGH] Tests 5.x -- Error detection via grep on response text**
Multiple tests check for errors using `grep -qi "error\|failed"`. This pattern false-positives on legitimate content containing these words (e.g., "No errors found", "0 failed assertions", or an element with id="error-container").

**[LOW] Tests 5.x -- Fixed sleeps between operations**
Tests use `sleep 0.3` to `sleep 3` between DOM operations. These are timing-dependent and will be flaky on slower machines.

---

### File: `scripts/smoke-tests/06-interact-state.sh`

**[HIGH] Test 6.1 -- Fallback pass without verification**
When the save_state response does not contain the snapshot name, the test falls back to passing if the response is not an error. This means the test passes even if the state was not actually saved -- it only proves the daemon did not crash.

**[HIGH] Test 6.3 -- Second pass path accepts silence as success**
When load_state does not return a clear success indicator, the test passes with "no error returned." The test cannot distinguish between "state loaded successfully" and "command was silently ignored."

---

### File: `scripts/smoke-tests/07-generate-formats.sh`

**[MEDIUM] Test 7.1 -- Loose Playwright pattern matching**
Line checks for `page.|await.*goto|.click(` using regex. These patterns would match any JavaScript code, not specifically Playwright-formatted reproduction steps. For example, a response saying "No page data available" would match `page.`.

**[MEDIUM] Test 7.2 -- Test format check matches any JS code**
Checks for `test(|describe(|it(` which would match any JavaScript function call containing these common words, not specifically a test framework structure.

**[LOW] Test 7.6 -- CSP syntax not validated**
Only checks for CSP directive keywords (`default-src|script-src|style-src`). Does not validate the generated CSP policy is syntactically correct or safe.

---

### File: `scripts/smoke-tests/09-perf-analysis.sh`

**[MEDIUM] Test 9.1 -- 3-second sleep between navigations**
Uses `sleep 3` between page navigations for performance comparison. This is both wasteful and insufficient depending on page load speed.

**[LOW] Test 9.5 -- Regex-based JSON field check**
Uses `grep -qE '"verdict":\s*"(improved|regressed|mixed|unchanged)"'` for JSON parsing. Should use `jq` which is available in the environment.

---

### File: `scripts/smoke-tests/10-recording.sh`

**[CRITICAL] Tests 10.x -- Depend on external service (YouTube)**
Recording tests navigate to YouTube. Same external dependency issues as test 4.1 -- YouTube may change, be blocked, or be down.

**[MEDIUM] Test 10.1 -- No video integrity verification**
Only checks recording name appears in `saved_videos`. Does not verify the video file exists, is non-empty, or is playable.

---

### File: `scripts/smoke-tests/12-cross-cutting.sh`

**[HIGH] Test 12.1 -- Pagination overlap check is insufficient**
Verifies consecutive pages have "different text" but does not check for duplicate entries across pages, which is the primary risk in cursor-based pagination. Two pages could contain different subsets of the same data with overlapping entries.

**[MEDIUM] Test 12.2 -- Error recovery weakened by second pass**
Error recovery test has a fallback that passes when "some responses not structured." This means partial failures are silently accepted.

---

### File: `scripts/smoke-tests/13-draw-mode.sh`

**[HIGH] Tests 13.5, 13.7-13.9 -- Require manual interaction**
These tests use `read -r` to wait for human input, making them impossible to run in CI. They are permanently stuck as manual tests without any automation path documented.

---

### File: `scripts/smoke-tests/14-stability-shutdown.sh`

**[LOW] Test 14.1 -- Only checks URL presence**
Post-barrage stability check only verifies `observe(page)` returns a URL. Does not check for data corruption, memory growth, or buffer integrity after the full test barrage.

**[LOW] Test 14.2 -- Only checks health status=ok**
Does not inspect memory usage, goroutine count, or connection count metrics that the health endpoint likely exposes. A server could be "ok" with a massive memory leak.

---

### File: `scripts/smoke-tests/15-file-upload.sh`

**[MEDIUM] Tests 15.x -- Heavy reliance on Python test server**
The upload test suite spawns a Python HTTP server for file upload testing. If Python is not available or the server fails to start, all 18 tests are skipped rather than failing. This means upload testing silently disappears from the suite.

---

## Part 2: Test Frameworks

### File: `scripts/tests/framework/framework.sh`

**[CRITICAL] `check_not_error` -- Only checks isError flag, not content**
Lines 214-219: `check_not_error` only verifies `isError != "true"`. It does not verify the response has any content, that the content is valid JSON, or that the content contains expected data. Tests relying solely on `check_not_error` can pass with empty or garbage responses.

**[MEDIUM] `check_matches` -- Case-insensitive by default**
Line 258: `grep -qiE` is case-insensitive, which weakens pattern matching. If a test checks for a specific casing (e.g., JSON field names), this helper silently ignores case mismatches.

**[LOW] `send_mcp` -- Retry on "starting up" masks real errors**
Lines 151-155: The retry logic on "starting up" is useful for cold starts but could mask real startup failures. If the daemon is truly broken, it will retry 2 times with 2-second sleeps before returning the error, adding 4 seconds of latency.

---

### File: `scripts/smoke-tests/framework-smoke.sh`

**[MEDIUM] `interact_and_wait` -- Fixed 0.5s polling interval**
The polling helper uses a fixed 0.5-second sleep between polls. On fast operations, this wastes time. On slow operations, 15 polls x 0.5s = 7.5s max wait may be insufficient. Should use exponential backoff.

---

## Part 3: Go Unit Tests (`cmd/browser-agent/`)

### File: `cmd/browser-agent/tools_contract_test.go`

**[MEDIUM] Generate tool contracts only check non-error, not content shape**
Lines 132-178: Tests for `generate reproduction`, `generate test`, `generate pr_summary`, `generate har`, `generate csp`, `generate sri`, `generate sarif` all use `assertNonErrorResponse` which only checks: (1) not an error, (2) has content blocks, (3) text is non-empty. None verify the generated output contains the expected format-specific structure (e.g., HAR has `log.entries`, SARIF has `runs[].results`).

---

### File: `cmd/browser-agent/tools_observe_contract_test.go`

This file is well-structured with proper field-level type checking via `assertResponseShape` and `assertObjectShape`. Good use of nested object validation (e.g., checking `errors[0]` shape). One area for improvement:

**[LOW] No boundary value tests for observe modes with data**
Tests load either full data or empty data. Missing tests for: single entry, buffer at capacity, entries with missing optional fields set to null vs. absent.

---

### File: `cmd/browser-agent/server_reliability_test.go`

**[CRITICAL] `TestReliability_ResourceLeaks_Goroutines` is permanently skipped**
Line 236: `t.Skip("Skipped: flaky in parallel test runs; works in isolation")`. The goroutine leak detection test -- arguably the most important reliability test -- is permanently skipped. The comment says "TODO: Investigate root cause of flakiness" but there is no tracking issue referenced. This means goroutine leaks can ship to production undetected.

**[MEDIUM] Goroutine leak test measures test process, not server**
Lines 271/298: `runtime.NumGoroutine()` measures goroutines in the _test process_, not in the _server process_ which runs as a separate binary. This test can only detect leaks in the HTTP client connection pooling, not actual server-side goroutine leaks.

**[MEDIUM] Stress test success threshold is 99%, not 100%**
Line 139: `if successRate < 99.0` allows up to 1% of 1,000 requests to fail silently. Under 50 concurrent connections with 20 requests each, that is up to 10 failed requests that are accepted as normal. In production, 1% failure rate is significant.

---

### File: `cmd/browser-agent/upload_security_test.go`

This file is exemplary. Thorough security testing with:
- Denylist matching for SSH keys, AWS credentials, .env files, key files, git config, system files
- Positive AND negative tests (safe paths that should NOT match)
- Path traversal detection
- Symlink resolution to sensitive directories
- Case-insensitive denylist on macOS
- SSRF protection for unspecified addresses
- Hardlink detection
- CRLF injection in form fields
- DNS fail-closed behavior

**[LOW] Missing test: Unicode normalization attacks on paths**
No test for Unicode path normalization attacks (e.g., using fullwidth characters or Unicode look-alikes for `/`, `.`).

---

### File: `cmd/browser-agent/ssrf_transport_test.go`

Excellent coverage with boundary IP tests for all RFC 1918 ranges, IPv6 unique-local, link-local, cloud metadata endpoints, and DNS rebinding protection. Well-structured with proper negative tests.

No significant findings.

---

### File: `cmd/browser-agent/tools_interact_dom_test.go`

**[MEDIUM] Two tests permanently skipped without regression alternative**
Lines 200-206: `TestDOMPrimitive_Click_ReturnsCorrelationID` and `TestDOMPrimitive_ListInteractive_ReturnsCorrelationID` are skipped with "covered by shell UAT." However, shell UAT requires a browser extension to be connected -- meaning this test path is only covered in manual smoke tests, not in automated CI.

---

### File: `cmd/browser-agent/integration_test.go`

**[HIGH] `TestIntegration_AllMCPToolsReturnValidResponses` accepts error responses**
Lines 243-244: When a tool returns an error response (non-"not implemented"), the test only logs a warning (`t.Logf`) and does not fail. This means tools that return runtime errors (e.g., "no data", "connection failed") are silently accepted as passing. The test claims to verify "ALL exposed MCP tools return valid responses" but actually only catches "not implemented" stubs.

---

### File: `cmd/browser-agent/stdio_silence_test.go`

**[CRITICAL] All 3 stdio silence tests are permanently skipped**
Lines 35, 169, 254: `TestStdioSilence_NormalConnection`, `TestStdioSilence_MultiClientSpawn`, and `TestStdioSilence_ConnectionRetry` are all skipped with "uses test binary which lacks -port flag; use shell UAT instead." The test comments say this is a "CRITICAL INVARIANT TEST" and contains "DO NOT remove or weaken this test" -- yet the test is permanently disabled. Stdio silence violations could ship to production.

---

### File: `cmd/browser-agent/golden_test.go`

**[HIGH] Golden test writes but never compares**
`TestUpdateGoldenToolsList` writes the current schema to a golden file but never compares it against a previous version. This is a _generator_, not a _test_. Running it always "passes" because it only writes. There is no test that reads the golden file and compares it against the current output to detect schema drift.

---

### File: `cmd/browser-agent/mcp_protocol_test.go` (from session summary)

Marked "DO NOT MODIFY" -- good. Integration test that builds binary and starts server. Verified to check response newlines, notification handling, JSON-RPC structure. No findings.

---

## Part 4: Go Unit Tests (`internal/`)

### File: `internal/buffers/ring_buffer_test.go` (from session summary)

Well-structured tests with good edge case coverage (capacity 1, multiple wraparounds, empty buffer, concurrent access).

**[LOW] Missing: negative capacity test**
No test for `NewRingBuffer(-1)` or `NewRingBuffer(0)`. Should verify the constructor handles invalid capacity gracefully.

**[LOW] Missing: ReadLast with negative n**
No test for `ReadLast(-5)`. Should verify this returns empty slice or panics predictably.

---

### File: `internal/security/security_test.go` (from session summary)

Strong security test suite with fuzz testing, false positive mitigation, and evidence redaction. Good negative tests for localhost skips.

No significant findings.

---

### File: `internal/redaction/redaction_test.go` (from session summary)

Comprehensive pattern tests with exact string matching. Thread safety verified. Performance benchmarks included.

No significant findings.

---

### File: `internal/capture/circuit_breaker_test.go`

**[MEDIUM] `TestCircuitBreaker_LifecycleCallback` uses timing-dependent assertion**
Line 167: `time.Sleep(50 * time.Millisecond)` to wait for goroutine-based callbacks. On slow CI machines, 50ms may not be sufficient, causing flaky failures.

**[LOW] `TestCircuitBreaker_StreakOpensCircuit` accesses internal mutex directly**
Lines 74-85: Test reaches into `cb.mu.Lock()` and directly manipulates internal state. This is tightly coupled to the implementation and will break on any internal refactor. However, this is acceptable for a state machine test where the API does not expose fine-grained state control.

---

### File: `internal/session/sessions_test.go` (from session summary)

Uses mock `CaptureStateReader`. Thorough verdict logic tests. Tests limits, case sensitivity, JSON serialization.

**[MEDIUM] Mock may diverge from real implementation**
The `CaptureStateReader` mock is defined in the test file. If the real interface changes, the mock must be manually updated. No compile-time check ensures the mock matches the production interface (though Go interfaces provide implicit implementation).

---

### File: `internal/pagination/pagination_test.go`

Well-structured cursor pagination tests with sequence number verification and monotonicity checks.

No significant findings.

---

## Part 5: Regression Tests (`tests/regression/`)

### File: `tests/regression/lib/assertions.sh`

Good assertion library with proper error reporting. Includes `assert_mcp_success`, `assert_mcp_error`, `assert_json_path`, `assert_json_equals`, and `assert_json_structure` with snapshot comparison. Well-designed.

No significant findings.

---

### File: `tests/regression/02-mcp-protocol/test-error-handling.sh`

**[MEDIUM] Test 3 (`test_observe_missing_what`) has conflicting assertion**
Lines 45-51: Calls `assert_mcp_success` expecting a successful response, then checks the content for the word "what" to confirm it mentioned the missing parameter. This means the test expects a _successful_ response with _error content_ -- a confusing contract. If the server changes to return a proper MCP error for missing params, this test breaks.

---

### File: `tests/regression/06-zombie-prevention/test-zombie-prevention.sh`

**[MEDIUM] Fixed sleeps throughout (sleep 1, sleep 2)**
Tests 1-6 all use fixed `sleep` calls ranging from 1 to 2 seconds. On fast machines, these waste time. On slow machines or under load, they may be insufficient, causing flaky results.

**[LOW] Test 4 uses fixed `sleep 4` for MCP cold start**
Line 129: Waits 4 seconds for MCP client to spawn a server, then kills the process. If the server takes longer than 4 seconds to start, the test may falsely fail.

---

## Part 6: Missing Coverage

### CRITICAL: No TypeScript tests exist

**[CRITICAL] Zero TypeScript test files found**
Searched for `*.test.ts`, `*.spec.ts`, and `__tests__/` directories in `src/` and throughout the project. No TypeScript tests were found. The project has TypeScript source code in `src/` (Chrome Extension MV3) but zero unit or integration tests for it. This means:
- Content script injection logic is untested
- Extension popup UI is untested
- Background service worker is untested
- WebSocket message handling from extension side is untested
- Action capture (click/form/navigate) listeners are untested

### CRITICAL: Skipped test inventory

**[CRITICAL] 7 tests are permanently skipped with no tracking**
The following tests are skipped and have no associated issue or timeline for re-enabling:

1. `TestReliability_ResourceLeaks_Goroutines` -- goroutine leak detection
2. `TestStdioSilence_NormalConnection` -- MCP stdio purity
3. `TestStdioSilence_MultiClientSpawn` -- multi-client stdio purity
4. `TestStdioSilence_ConnectionRetry` -- retry stdio purity
5. `TestDOMPrimitive_Click_ReturnsCorrelationID` -- DOM click correlation
6. `TestDOMPrimitive_ListInteractive_ReturnsCorrelationID` -- list interactive correlation
7. Smoke test 5.15 -- permanently skipped DOM interaction

Each of these represents a gap in automated verification. Some are labeled "CRITICAL INVARIANT" in comments while being disabled in practice.

### HIGH: Missing negative/error path tests in smoke suite

**[HIGH] No test for concurrent MCP clients in smoke tests**
The smoke test suite runs all tests sequentially with a single MCP client. There is no test for what happens when two MCP clients connect simultaneously, which is a documented supported use case.

**[HIGH] No test for daemon restart resilience in smoke tests**
No test verifies that a daemon restart mid-session preserves buffer state or handles in-flight requests gracefully. The stability tests in 14.x only check health after the barrage, not recovery after a crash.

---

## Part 7: Patterns of Concern

### Pattern 1: Fallback-Pass Anti-Pattern (HIGH)

Multiple smoke tests follow this pattern:
```bash
if echo "$response" | grep -q "SPECIFIC_THING"; then
    pass "Found specific thing"
elif echo "$response" | grep -qi "generic_keyword"; then
    pass "Found generic keyword (specific thing not in response, but close enough)"
else
    fail "Nothing found"
fi
```

Files affected: `02-core-telemetry.sh` (tests 2.2, 2.3), `06-interact-state.sh` (tests 6.1, 6.3), `12-cross-cutting.sh` (test 12.2).

This pattern creates false confidence. The fallback `pass` accepts a weaker signal that does not prove the feature works, but the overall test suite reports it as passing.

### Pattern 2: Fixed Sleep Instead of Polling (MEDIUM)

At least 25 instances of `sleep N` where N ranges from 0.3 to 5 seconds. These are scattered across all 15 smoke test files and the regression tests. Total wasted time is estimated at 30-60 seconds per smoke test run, and the fixed delays introduce flakiness on slower machines.

### Pattern 3: grep-based JSON Parsing (MEDIUM)

Multiple tests use `grep` or `grep -qE` to check JSON field values instead of using `jq` which is available and used elsewhere in the same test suite. This risks false matches when field values contain field names, or when JSON formatting changes.

Files affected: `04-network-websocket.sh`, `09-perf-analysis.sh`, `12-cross-cutting.sh`.

### Pattern 4: Permanently Skipped "Critical" Tests (CRITICAL)

Tests marked with comments like "CRITICAL INVARIANT", "DO NOT MODIFY", or "RELEASE GATE" that are then immediately followed by `t.Skip()`. This creates a dangerous illusion of coverage -- the test comments suggest thorough verification, but the test never runs. Developers scanning test names or comments will believe these areas are covered.

---

## Recommendations (Priority Order)

1. **Enable or replace the 7 permanently skipped Go tests.** Each represents a real gap. If they cannot run in CI, create alternative tests that can.

2. **Add TypeScript tests for the Chrome Extension.** The extension is half the product and has zero automated tests.

3. **Eliminate the fallback-pass pattern in smoke tests.** If the specific assertion fails, the test should fail -- not fall back to a weaker assertion and pass.

4. **Replace external service dependencies (Binance, YouTube) with local test servers.** Use `netcat` or a simple Go/Python server to simulate WebSocket and media streams.

5. **Make the golden test actually compare against a baseline.** Currently it only writes, never reads.

6. **Fix the goroutine leak test.** This is a release gate test that never runs. Either fix the flakiness or replace it with a different detection mechanism.

7. **Replace fixed sleeps with polling/backoff.** The framework already has `wait_for_health` with exponential backoff -- apply this pattern to all timing-dependent checks.

8. **Use `jq` consistently for JSON field checks.** The framework already imports `jq` -- there is no reason to use `grep` for JSON parsing.

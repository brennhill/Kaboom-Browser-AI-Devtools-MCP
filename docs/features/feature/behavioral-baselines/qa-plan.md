---
status: proposed
scope: feature/behavioral-baselines/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-behavioral-baselines
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Behavioral Baselines

> QA plan for the Behavioral Baselines feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Response shape extraction leaks values | Verify shape extraction records only field names and types (`{"id": "number"}`), never actual values (`{"id": 42}`) | critical |
| DL-2 | Baseline JSON on disk contains response bodies | Verify `~/.kaboom/baselines/<name>.json` stores shapes, not raw response content | critical |
| DL-3 | API endpoint paths reveal internal architecture | Verify normalized paths (`/api/users/{uuid}/posts`) do not expose internal routing patterns beyond what the browser already sees | medium |
| DL-4 | Console error fingerprints contain PII | Verify known-error fingerprints are hashed or normalized, not raw messages containing user data | high |
| DL-5 | WebSocket URL patterns expose internal services | Verify WebSocket baseline records URL patterns without leaking auth tokens in query strings | high |
| DL-6 | Comparison results expose raw error text | Verify regression descriptions use normalized summaries, not raw console messages with user data | high |
| DL-7 | Baseline name/description user-provided fields | Verify agent-provided names and descriptions are stored as-is but do not echo sensitive data in list responses | medium |
| DL-8 | Disk persistence accessible by other users | Verify `~/.kaboom/baselines/` directory has appropriate permissions (user-only read/write) | high |
| DL-9 | Latency data reveals internal API topology | Verify latency baselines do not include internal hostnames or IP addresses, only URL paths | medium |

### Negative Tests (must NOT leak)
- [ ] No raw JSON response bodies stored in baseline files on disk
- [ ] No authentication headers or tokens in network baseline endpoint records
- [ ] No raw console error messages with PII in baseline fingerprints
- [ ] No WebSocket auth tokens in connection URL patterns
- [ ] No absolute file system paths in baseline storage location references
- [ ] No response values (only types) in shape comparison output

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | "match" vs "no data" distinction | AI understands `status: "match"` means "current state matches baseline" not "no comparison data available" | [ ] |
| CL-2 | Regression severity levels | AI understands regression severities (error for status code changes, warning for latency) are actionable priorities | [ ] |
| CL-3 | Tolerance factor meaning | AI understands `timing_factor: 3.0` means "3x slower is the threshold" not "3 second tolerance" | [ ] |
| CL-4 | Endpoint not observed vs endpoint removed | AI distinguishes between "endpoint not visited yet" (no regression) and "endpoint returning 404" (regression) | [ ] |
| CL-5 | Version number semantics | AI understands baseline version increments on overwrite, indicating update history, not schema versions | [ ] |
| CL-6 | "Improvement" vs "regression" | AI understands improvements are noted but not flagged as issues; regressions are actionable | [ ] |
| CL-7 | Known errors vs new errors | AI understands errors present in baseline are "expected" and only new errors trigger console regressions | [ ] |
| CL-8 | Shape comparison output format | AI can interpret `{"id": "number", "name": "string"}` as a structural schema, not actual data | [ ] |
| CL-9 | Empty baseline list | AI understands `[]` with `count: 0` means no baselines saved, not that comparison is impossible | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might confuse `timing_factor: 3.0` with an absolute threshold in seconds -- verify documentation says "multiplier"
- [ ] AI might interpret "status: match" as "identical values" rather than "within tolerance" -- verify comparison description
- [ ] AI might assume not-observed endpoints are healthy -- verify the output explains why they are skipped
- [ ] AI might think `allow_additional_network: true` means new endpoints are always OK -- verify it just suppresses warnings for them
- [ ] AI might confuse baseline `version` with the software version -- verify field naming is clear

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Save a baseline | 1 step: `save_baseline(name: "login-flow")` | No -- already minimal |
| Compare against baseline | 1 step: `compare_baseline(name: "login-flow")` | No -- already minimal |
| Full save-change-compare cycle | 3 steps: save baseline, make changes, compare | No -- inherently requires before/after |
| List existing baselines | 1 step: `list_baselines()` | No -- already minimal |
| Delete a stale baseline | 1 step: `delete_baseline(name: "old")` | No -- already minimal |
| Custom tolerance comparison | 2 steps: understand tolerance params, call compare with config | Yes -- defaults should cover 90% of cases |

### Default Behavior Verification
- [ ] `save_baseline` works with only `name` parameter (description, url_scope optional)
- [ ] `compare_baseline` works with only `name` parameter (default tolerance applied)
- [ ] Default timing_factor of 3.0 is sensible for most apps
- [ ] Default `allow_additional_network: true` prevents false positives on new endpoints
- [ ] Default `allow_additional_console_info: true` prevents noise from info-level logs
- [ ] Baselines persist across server restarts without user action

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Save baseline with 5 network bodies | 5 network bodies in capture buffer | Baseline with 5 endpoint records, correct shapes | must |
| UT-2 | Save baseline with console errors | 2 errors + 1 warning in server entries | Baseline with error_count: 2, warning_count: 1, fingerprints | must |
| UT-3 | Save baseline with WebSocket connections | 1 open WS connection | Baseline with WS record, URL pattern, state: open | must |
| UT-4 | Save when name exists, overwrite=false | Existing baseline named "test" | Error: baseline already exists | must |
| UT-5 | Save with overwrite=true | Existing baseline "test" v1 | Baseline updated, version: 2 | must |
| UT-6 | Save at max baselines (50) | 50 baselines already exist | Error: maximum baselines reached | must |
| UT-7 | Path normalization - UUID | `/api/users/550e8400-e29b-41d4-a716-446655440000/posts` | `/api/users/{uuid}/posts` | must |
| UT-8 | Path normalization - numeric ID | `/api/items/12345/comments` | `/api/items/{id}/comments` | must |
| UT-9 | Response shape extraction - flat JSON | `{"id": 1, "name": "test", "active": true}` | `{"id": "number", "name": "string", "active": "boolean"}` | must |
| UT-10 | Response shape extraction - nested JSON | `{"user": {"id": 1}, "items": [1,2]}` | `{"user": "object", "items": "array"}` | must |
| UT-11 | Response shape extraction - non-JSON | HTML response body | Shape is nil (no shape recorded) | must |
| UT-12 | Compare with no changes | Same state as baseline | `status: "match"`, empty regressions | must |
| UT-13 | Compare - status code regression | Endpoint was 200, now 500 | Regression: category "network", severity "error" | must |
| UT-14 | Compare - latency regression (>3x) | 100ms baseline, 400ms current | Regression: category "timing" | must |
| UT-15 | Compare - latency within tolerance (<3x) | 100ms baseline, 250ms current | No regression | must |
| UT-16 | Compare - new console errors | Error not in baseline fingerprints | Regression: category "console" | must |
| UT-17 | Compare - known error still present | Error matching baseline fingerprint | Not flagged as regression | must |
| UT-18 | Compare - WebSocket regression | WS was open, now closed | Regression: category "websocket" | must |
| UT-19 | Compare - improvement | Endpoint was 500, now 200 | Noted as improvement, not regression | must |
| UT-20 | Compare - custom tolerance factor 2.0 | 100ms baseline, 250ms current, factor 2.0 | Regression flagged (250ms > 200ms) | must |
| UT-21 | Compare - nonexistent baseline | `compare_baseline(name: "doesnt-exist")` | Error: not found | must |
| UT-22 | List with no baselines | Empty baseline store | `[]`, count: 0, bytes: 0 | should |
| UT-23 | List with baselines | 3 baselines saved | Array of 3 summaries with correct metadata | should |
| UT-24 | Delete removes from memory and disk | Delete existing baseline | Not in list, file removed from disk | must |
| UT-25 | Baseline exceeding 100KB | Baseline with very large network data | Error: exceeds size limit, not written | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Save reads from live capture buffer | Extension -> Capture -> Baseline save | Baseline contains current network/console/WS state | must |
| IT-2 | Compare reads live state and baseline | Capture (current) + BaselineStore (saved) | Accurate comparison of current vs saved | must |
| IT-3 | Disk persistence round-trip | Save -> server restart -> list baselines | Baseline loaded from disk, metadata correct | must |
| IT-4 | Concurrent save and compare | Two agents: one saves, one compares simultaneously | No race conditions (RWMutex works) | must |
| IT-5 | Baseline + push regression integration | Save baseline, trigger regression, check alerts | Regression detected against saved baseline | should |
| IT-6 | Multiple baselines for different URLs | Save "login-flow" and "dashboard-load" for different pages | Both baselines independent, compare scoped correctly | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Baseline capture speed | Time to capture baseline from buffers | < 30ms | must |
| PT-2 | Comparison speed | Time to compare current state against baseline | < 20ms | must |
| PT-3 | Disk persistence speed | Time to write baseline JSON to disk | < 50ms | must |
| PT-4 | Startup load speed | Time to load 50 baselines from disk | < 200ms | must |
| PT-5 | Memory per baseline | Memory footprint of one baseline | < 100KB | must |
| PT-6 | Total baseline memory | Memory for 50 baselines | < 5MB | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Endpoint not observed in current session | Baseline has endpoint `/api/users`, current session has not visited it | Not flagged as regression (might not have navigated there) | must |
| EC-2 | Non-JSON response body | HTML or XML response | Shape is nil, no shape comparison | must |
| EC-3 | Concurrent access to baseline store | Multiple save/compare/list operations overlapping | RWMutex prevents data corruption | must |
| EC-4 | Baseline file corruption | Manually corrupt `~/.kaboom/baselines/test.json` | Graceful error on load, other baselines unaffected | should |
| EC-5 | Baseline with empty network data | No network requests captured at save time | Baseline saved with empty network section, compare skips network | should |
| EC-6 | Very long URL path | URL with 50+ segments | Path normalization handles without crash or truncation | should |
| EC-7 | Null/undefined JSON values | Response body with `{"key": null}` | Shape records `{"key": "null"}` | must |
| EC-8 | Total storage exceeding 5MB | Try to save 51st baseline that would exceed limit | Error returned, baseline not saved | must |
| EC-9 | Baseline name with special characters | Name: `"my baseline / v2 (test)"` | File saved with sanitized filename, retrieved correctly | should |
| EC-10 | Server crash during baseline write | Simulated crash mid-write | No partial/corrupt file left on disk | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web app with API endpoints, console logging, and optionally WebSocket connections
- [ ] `~/.kaboom/baselines/` directory exists (created automatically on first save)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Navigate to test app's main page, interact to generate network traffic | Network tab shows API calls completing | Several API responses captured | [ ] |
| UAT-2 | `{"tool": "configure", "arguments": {"action": "save_baseline", "name": "main-page", "description": "Main page working state"}}` | AI confirms baseline saved | Response includes name, version: 1, endpoint count, URL scope | [ ] |
| UAT-3 | `{"tool": "configure", "arguments": {"action": "list_baselines"}}` | AI receives baseline list | List contains "main-page" with correct metadata | [ ] |
| UAT-4 | `{"tool": "configure", "arguments": {"action": "compare_baseline", "name": "main-page"}}` | No changes made to app | Response: `status: "match"`, no regressions | [ ] |
| UAT-5 | Break an API endpoint (e.g., return 500 instead of 200) | Human modifies backend or mocks a failure | API now returns error | [ ] |
| UAT-6 | Reload the page, then: `{"tool": "configure", "arguments": {"action": "compare_baseline", "name": "main-page"}}` | AI receives comparison | Response: `status: "regression"`, network regression listing the broken endpoint | [ ] |
| UAT-7 | Fix the API endpoint back to normal | Human restores normal behavior | API returns 200 again | [ ] |
| UAT-8 | Reload and compare: `{"tool": "configure", "arguments": {"action": "compare_baseline", "name": "main-page"}}` | AI receives comparison | Response: `status: "match"` or `status: "improved"` | [ ] |
| UAT-9 | Add a console.error that was not in the baseline | Human adds `console.error("new bug")` to app code | Error visible in DevTools console | [ ] |
| UAT-10 | `{"tool": "configure", "arguments": {"action": "compare_baseline", "name": "main-page"}}` | AI receives comparison | Console regression detected for new error | [ ] |
| UAT-11 | `{"tool": "configure", "arguments": {"action": "save_baseline", "name": "main-page", "overwrite": true}}` | AI overwrites baseline | Response: version: 2, updated baseline | [ ] |
| UAT-12 | `{"tool": "configure", "arguments": {"action": "delete_baseline", "name": "main-page"}}` | AI confirms deletion | Baseline removed from list, file deleted from disk | [ ] |
| UAT-13 | Restart Kaboom server, then list baselines | Human restarts `./dist/kaboom` | Baselines loaded from disk (excluding deleted one) | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No response values in baseline | Inspect `~/.kaboom/baselines/main-page.json` on disk | Only field names and types in shape data, no actual values | [ ] |
| DL-UAT-2 | No auth headers in baseline | Inspect baseline file for "Authorization", "Cookie", "Bearer" | None present | [ ] |
| DL-UAT-3 | Compare output hides raw error text | Trigger comparison with new console error containing PII-like text | Regression description uses fingerprint, not raw message | [ ] |
| DL-UAT-4 | Disk file permissions | `ls -la ~/.kaboom/baselines/` | Files owned by current user, not world-readable | [ ] |

### Regression Checks
- [ ] Existing `observe(what: "network")` still works independently of baselines
- [ ] Existing `observe(what: "errors")` unaffected by baseline save/compare
- [ ] Server startup time not significantly impacted by loading 10+ baselines
- [ ] Feature disabled by default (no baselines = no comparisons, no errors)

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |

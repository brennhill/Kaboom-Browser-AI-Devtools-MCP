---
status: proposed
scope: feature/compressed-diffs/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-compressed-diffs
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Compressed State Diffs

> QA plan for the Compressed State Diffs feature (`get_changes_since`). Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Compressed diffs summarize console errors, network failures, WebSocket disconnections, and user actions. Deduplication fingerprinting, endpoint normalization, and checkpoint persistence all create surface area for sensitive data retention.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Console error messages containing PII | Verify console error text (truncated to 200 chars) does not expose user data (e.g., "Error for user <john@example.com>") | high |
| DL-2 | Network endpoint URLs with auth tokens | Verify URL path normalization strips query parameters (which may contain tokens) | critical |
| DL-3 | Checkpoint persistence storing sensitive state | Verify named checkpoints stored to disk do not contain raw log/network data, only buffer positions | critical |
| DL-4 | User action tracking details | Verify actions diff does not expose typed text content (e.g., passwords entered in forms) | critical |
| DL-5 | WebSocket message content in disconnect reasons | Verify WS disconnection entries do not include last message content | medium |
| DL-6 | Endpoint status tracking revealing internal APIs | Verify `known_endpoints` map in checkpoint does not expose full API surface to persisted storage | medium |
| DL-7 | Fingerprinting preserving sensitive patterns | Verify UUID/number normalization in fingerprints does not preserve enough structure to reconstruct original data | low |
| DL-8 | Summary text containing raw values | Verify the one-line summary contains counts and categories, not specific error messages or URLs | medium |
| DL-9 | Token count estimation leaking response size | Verify token count is an approximation that does not reveal exact payload sizes of sensitive responses | low |
| DL-10 | Named checkpoint names revealing session info | Verify checkpoint names follow naming conventions (no sensitive data in names like `before_user_john_fix`) | low |

### Negative Tests (must NOT leak)
- [ ] Console error messages with PII must have PII redacted or truncated before inclusion in diff
- [ ] Network endpoint URLs must not include query parameters in the diff output
- [ ] User action diffs must not include typed text content (only action type and target element)
- [ ] Persisted checkpoints must contain only buffer indices and timestamps, not raw data
- [ ] The `known_endpoints` map must store only path patterns and status codes, not request/response data
- [ ] Fingerprinted messages must not be reversible to original content
- [ ] WebSocket error messages in diffs must not contain full message payloads

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Severity is unambiguous | "error", "warning", "clean" are the only three values | [ ] |
| CL-2 | Summary is actionable | One-line summary like "2 new console error(s), 1 network failure(s)" | [ ] |
| CL-3 | Checkpoint timestamps are clear | ISO 8601 with timezone for from/to | [ ] |
| CL-4 | Diff categories are distinct | Console, network, websocket, actions are clearly separated sections | [ ] |
| CL-5 | Deduplication is transparent | Entries show `count` field when messages are deduplicated | [ ] |
| CL-6 | "Clean" vs. "empty" distinction | "Clean" means no notable changes; empty buffers also return "clean" with appropriate note | [ ] |
| CL-7 | Auto-checkpoint advancement is clear | Response or documentation explains that calling without named checkpoint advances position | [ ] |
| CL-8 | Named vs. auto checkpoint behavior | Named checkpoint queries do NOT advance auto-checkpoint -- this must be unambiguous | [ ] |
| CL-9 | Network failure context | New failures include `previous_status` showing what the endpoint used to return | [ ] |
| CL-10 | Token count estimate | `estimated_token_count` helps LLM understand response size impact | [ ] |
| CL-11 | Nil vs. empty sections | Omitted categories are `nil` (absent), not empty objects -- LLM must not misinterpret absence as error | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM might interpret "clean" as "no data captured" rather than "no notable changes" -- verify the summary explains this
- [ ] LLM might call `get_changes_since` repeatedly without realizing auto-checkpoint advances, seeing "clean" on second call -- verify the first response notes checkpoint advancement
- [ ] LLM might confuse `new_endpoints` (never seen before) with `new_failures` (status code changed to error) -- verify field names are distinct
- [ ] LLM might not understand deduplication count=5 means "this error happened 5 times" -- verify the count field is contextually clear
- [ ] LLM might pass a stale named checkpoint and get a very large diff without realizing the checkpoint was from long ago -- verify from/to timestamps are prominent

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Check for changes since last call | 1 step: call `observe(what: "changes")` | No -- already minimal |
| Create named checkpoint | 1 step: use checkpoint naming in configure tool | No |
| Check changes against named checkpoint | 1 step: add `checkpoint` parameter | No |
| Filter to errors only | 1 step: add `severity: "errors_only"` | No |
| Filter to specific categories | 1 step: add `include: ["console", "network"]` | No |
| Edit-wait-check feedback loop | 3 steps: edit code, wait, call observe(changes) | No -- this is the minimal feedback loop |

### Default Behavior Verification
- [ ] Feature works with zero configuration (auto-checkpoint, all categories, all severities)
- [ ] First call returns all buffered data and sets initial checkpoint
- [ ] Auto-checkpoint advances automatically on each call
- [ ] All four categories (console, network, websocket, actions) included by default
- [ ] Default severity is "all" (no filtering)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Empty server | No data in any buffer | Severity "clean", all diffs empty, summary "No significant changes" | must |
| UT-2 | New console errors | 3 error-level log entries after checkpoint | Console diff has 3 entries, severity "error" | must |
| UT-3 | Deduplicated errors | Same error message 5 times | 1 entry with count=5 | must |
| UT-4 | Endpoint status change | `/api/users` was 200, now 500 | Network failures entry with `previous_status: 200` | must |
| UT-5 | New endpoint discovery | `/api/new-endpoint` first seen | Appears in `new_endpoints` list | must |
| UT-6 | Degraded endpoint detection | Endpoint latency > 3x baseline | Appears in degraded endpoints | should |
| UT-7 | WebSocket disconnection | WS close event after checkpoint | Appears in disconnections | must |
| UT-8 | Auto-checkpoint advancement | First call sees errors, second call sees nothing new | Second call returns "clean" | must |
| UT-9 | Named checkpoint stability | Create named checkpoint, call auto twice | Named checkpoint query always returns same window | must |
| UT-10 | Severity filter - errors_only | Mix of errors and warnings | Only errors in response | must |
| UT-11 | Category filter | `include: ["console", "network"]` | WebSocket and actions sections are nil | must |
| UT-12 | Buffer wrap past checkpoint | Checkpoint index evicted from ring buffer | Best-effort from available entries, no error | must |
| UT-13 | Timestamp as checkpoint | ISO 8601 timestamp | Entries after that time returned | must |
| UT-14 | Token count approximation | Known response JSON | Estimated tokens close to JSON length / 4 | should |
| UT-15 | Max entries cap | 100 errors after checkpoint | Capped at 50 in response | must |
| UT-16 | UUID normalization in fingerprint | "Error loading user abc123-def456" | UUID replaced with `{uuid}` | must |
| UT-17 | Number normalization in fingerprint | "Request 12345 failed" | Number replaced with `{n}` | must |
| UT-18 | Timestamp normalization | "Error at 2026-01-24T10:30:00Z" | Timestamp replaced with `{ts}` | should |
| UT-19 | URL path extraction | `/api/users?page=1&sort=name` | Path is `/api/users`, query params stripped | must |
| UT-20 | Severity hierarchy | Console error + WS warning | Overall severity is "error" (worst wins) | must |
| UT-21 | Summary formatting | 2 errors, 1 network failure | "2 new console error(s), 1 network failure(s)" | must |
| UT-22 | User actions tracking | Click, navigation, typing events | Actions diff lists events with types and targets | should |
| UT-23 | Noise filtering integration | Console entries matching active noise rules | Filtered entries do not appear in diff | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end diff via MCP | MCP client -> `observe(what: "changes")` -> server | Valid diff response with severity, summary, sections | must |
| IT-2 | Named checkpoint create and query | Create checkpoint -> trigger events -> query named checkpoint | Events since named checkpoint returned | must |
| IT-3 | Feedback loop simulation | Make changes -> call changes -> fix -> call changes | First call shows errors, second shows "clean" | must |
| IT-4 | Concurrent reads and writes | Call `get_changes_since` while new events arriving | Consistent diff, no race conditions | must |
| IT-5 | Session health integration | `configure(what: "health")` uses same checkpoint data | Health check reflects same state as changes diff | should |
| IT-6 | Persistence across sessions | Named checkpoint persists, new session queries it | Checkpoint data survives server restart | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Checkpoint resolution | Wall clock time | Under 1ms | must |
| PT-2 | Console diff for 1000 entries | Wall clock time | Under 10ms | must |
| PT-3 | Network diff for 100 entries | Wall clock time | Under 5ms | must |
| PT-4 | WebSocket diff for 500 entries | Wall clock time | Under 5ms | must |
| PT-5 | Actions diff for 50 entries | Wall clock time | Under 2ms | must |
| PT-6 | Total tool response time | Wall clock time | Under 25ms | must |
| PT-7 | Response size | Byte count | Under 2KB typical | should |
| PT-8 | Checkpoint memory | Memory for 20 checkpoints | Under 100KB | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | First call (no prior checkpoint) | Server just started, buffers have data | Returns everything in buffers, sets auto-checkpoint | must |
| EC-2 | All buffers empty | Server just started, no data | "Clean" with empty sections | must |
| EC-3 | Max 20 named checkpoints | Create 21 named checkpoints | Oldest named checkpoint evicted or error returned | must |
| EC-4 | Invalid checkpoint name | Non-existent named checkpoint | Clear error in MCP response | must |
| EC-5 | Invalid timestamp format | Malformed ISO 8601 string | Clear error in MCP response | must |
| EC-6 | Concurrent checkpoint creation | Two simultaneous `create_checkpoint` calls | Both succeed with correct positions | must |
| EC-7 | Very noisy page (>50 entries per category) | Page generating hundreds of errors | Each section capped at 50, total entries noted | must |
| EC-8 | Page navigation during diff | URL changes between checkpoint and query | Diff includes events from both URLs | should |
| EC-9 | Checkpoint name too long (>50 chars) | 60-character name | Error or truncation with clear message | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page loaded that can generate console errors and network failures on demand (e.g., a page with buttons that trigger API calls to non-existent endpoints)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "observe", "arguments": {"what": "changes"}}` | Initial page state | First call returns buffered data, sets auto-checkpoint | [ ] |
| UAT-2 | Trigger a console error on the page (e.g., click a button that calls undefined function) | Error appears in browser console | -- | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "changes"}}` | Compare to UAT-1 | Severity "error", console diff shows the new error, summary mentions it | [ ] |
| UAT-4 | `{"tool": "observe", "arguments": {"what": "changes"}}` | No new errors since UAT-3 | Severity "clean", summary "No significant changes" (auto-checkpoint advanced) | [ ] |
| UAT-5 | `{"tool": "configure", "arguments": {"what": "store", "checkpoint": "before_test"}}` | Named checkpoint created | Confirmation response | [ ] |
| UAT-6 | Trigger 3 more errors, then: `{"tool": "observe", "arguments": {"what": "changes", "checkpoint": "before_test"}}` | 3 errors since named checkpoint | Severity "error", console diff has 3 entries | [ ] |
| UAT-7 | `{"tool": "observe", "arguments": {"what": "changes", "checkpoint": "before_test"}}` | Same query again | Same result (named checkpoint does NOT advance) | [ ] |
| UAT-8 | `{"tool": "observe", "arguments": {"what": "changes", "severity": "errors_only"}}` | Mix of errors and warnings on page | Only errors appear in response | [ ] |
| UAT-9 | `{"tool": "observe", "arguments": {"what": "changes", "include": ["console"]}}` | Network and WS events also occurred | Only console section present, others are absent | [ ] |
| UAT-10 | Trigger the same error 10 times rapidly, then query changes | Repeated error in console | One deduplicated entry with count=10 | [ ] |
| UAT-11 | Make a successful API call, then make the same endpoint return 500, then query changes | Endpoint status changed | Network failures shows endpoint with `previous_status: 200` | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Console errors do not leak full PII | Trigger error containing an email address | Error text is truncated to 200 chars, fingerprint normalizes dynamic content | [ ] |
| DL-UAT-2 | Network URLs strip query params | Make request with `?token=secret` | Diff shows URL path only, no query parameters | [ ] |
| DL-UAT-3 | User actions do not contain typed text | Type a password in a form field | Actions diff shows "typing" event with target element, NOT the typed content | [ ] |
| DL-UAT-4 | Named checkpoint does not store raw data | Inspect persisted checkpoint file/memory | Only buffer positions and timestamps, no log messages or network data | [ ] |

### Regression Checks
- [ ] Existing `observe` console/network/websocket tools still return full data (diffs are a separate view)
- [ ] Buffer ring eviction is unaffected by checkpoint tracking
- [ ] Noise filtering rules still apply in diff output
- [ ] Server memory does not grow with repeated diff calls (checkpoints are bounded)
- [ ] Concurrent access to buffers (from both diff queries and new data ingestion) is race-free

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

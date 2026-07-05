---
status: proposed
scope: feature/error-clustering/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-error-clustering
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Error Clustering

> QA plan for the Error Clustering feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Stack traces expose absolute file paths | Verify cluster `root_cause` and `instances[].source` use relative paths only, never absolute system paths like `/Users/dev/project/...` | critical |
| DL-2 | Error messages contain user PII | Check that message normalization replaces emails, tokens, and PII-like strings with placeholders, not just UUIDs/IDs/URLs | high |
| DL-3 | Stack frames reveal internal infrastructure | Ensure framework-internal frames (node_modules paths) do not expose private package registry URLs or internal dependency names | medium |
| DL-4 | Cluster summary leaks raw variable values | Verify the `summary` field uses normalized templates, not raw error messages with user data embedded | high |
| DL-5 | Instance list accumulates sensitive error context | Confirm instance storage is capped at 20 and older instances with potentially sensitive data are properly evicted | medium |
| DL-6 | Alert payloads expose raw stack content | Verify compound alerts generated at 3+ instances use normalized messages, not raw error text | high |
| DL-7 | Normalized message retains short quoted strings | Strings under 20 chars are NOT replaced by `{string}` -- verify these cannot contain passwords, tokens, or secrets | high |

### Negative Tests (must NOT leak)
- [ ] No absolute file paths appear in any cluster field (root_cause, instances, affected_components)
- [ ] No auth tokens, API keys, or passwords appear in normalized error messages
- [ ] No raw request/response bodies appear in cluster data
- [ ] No user email addresses or PII in cluster summaries
- [ ] No internal network hostnames or IP addresses in stack frame source fields
- [ ] No environment variables or config values in error message templates

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Cluster vs individual error distinction | AI can distinguish between a cluster (grouped errors) and a single error from `observe(what: "errors")` | [ ] |
| CL-2 | Root cause meaning is unambiguous | The `root_cause` field clearly identifies a function+file location, not just a message | [ ] |
| CL-3 | Instance count vs total errors | AI understands `instance_count: 4` means 4 related errors, not 4 total errors in the session | [ ] |
| CL-4 | Unclustered errors are explained | The `unclustered_errors: 2` field clearly indicates errors that did not match any cluster | [ ] |
| CL-5 | Severity inheritance is clear | AI understands cluster severity comes from the highest-severity instance, not the majority | [ ] |
| CL-6 | Time range semantics | `first_seen` / `last_seen` clearly indicates the cluster's lifetime, not the session lifetime | [ ] |
| CL-7 | Affected components meaning | AI understands `affected_components` are source files, not UI components or microservices | [ ] |
| CL-8 | No data vs no clusters | When response has `clusters: []`, AI understands "no related error groups found" not "no errors at all" | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might confuse `instance_count` with unique error types -- verify field description disambiguates
- [ ] AI might treat `root_cause` as confirmed diagnosis rather than hypothesis -- verify language uses "likely" or "inferred"
- [ ] AI might assume `unclustered_errors: 0` means no errors exist -- verify `total_errors` field clarifies
- [ ] AI might think cluster expiry (5 min timeout) means the error is fixed -- verify no "resolved" language in expired clusters
- [ ] AI might not realize normalized messages are templates -- verify placeholder format `{uuid}`, `{id}` is self-documenting

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| View error clusters | 1 step: `analyze(target: "errors")` | No -- already minimal |
| View individual (unclustered) errors | 1 step: `observe(what: "errors")` | No -- existing workflow unchanged |
| Get alerted about new cluster | 0 steps: automatic via alert piggyback | No -- zero-config is optimal |
| Investigate root cause of cluster | 2 steps: read cluster, look at root_cause field | No -- data is inline |

### Default Behavior Verification
- [ ] Feature works with zero configuration (clusters form automatically)
- [ ] No opt-in or enable step required
- [ ] Cluster alerts generate automatically at 3+ instances without configuration
- [ ] Cluster expiry at 5 minutes requires no manual cleanup
- [ ] Max 50 active clusters enforced without user intervention

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Stack frame parsing | `"at UserProfile.render (user-profile.js:42:10)"` | `{Function: "UserProfile.render", File: "user-profile.js", Line: 42, Col: 10}` | must |
| UT-2 | Framework frame detection | `"at React.createElement (node_modules/react/...)"` | Frame flagged as framework, excluded from similarity | must |
| UT-3 | Message normalization - UUID | `"Failed for 550e8400-e29b-41d4-a716-446655440000"` | `"Failed for {uuid}"` | must |
| UT-4 | Message normalization - numeric ID | `"User 12345 not found"` | `"User {id} not found"` | must |
| UT-5 | Message normalization - URL | `"Failed to fetch https://api.example.com/data"` | `"Failed to fetch {url}"` | must |
| UT-6 | Message normalization - file path | `"ENOENT: /usr/local/lib/file.js"` | `"ENOENT: {path}"` | must |
| UT-7 | Message normalization - timestamp | `"Error at 2026-01-24T15:30:00Z"` | `"Error at {timestamp}"` | should |
| UT-8 | Message normalization - long string | `"Cannot read 'averylongpropertyname...'"` (>20 chars) | `"Cannot read {string}"` | should |
| UT-9 | Two-signal match: stack + message | Two errors with shared frames and same normalized message | Clustered together | must |
| UT-10 | Two-signal match: stack + temporal | Two errors with shared frames within 2 seconds | Clustered together | must |
| UT-11 | Two-signal match: message + temporal | Two errors with same normalized message within 2 seconds | Clustered together | must |
| UT-12 | Single-signal rejection | Two errors with only stack similarity (different message, >2s apart) | NOT clustered | must |
| UT-13 | Root cause inference | Cluster with 4 instances, deepest common app-code frame at `render()` | `root_cause` = that frame | must |
| UT-14 | Root cause fallback to message | Cluster with message-only similarity (no common frames) | `root_cause` = normalized message | should |
| UT-15 | Representative error selection | Cluster with varying stack depth | Most informative (deepest stack) chosen as representative | should |
| UT-16 | Instance cap at 20 | Add 25 errors to one cluster | `instances` array has 20 entries, `instance_count` is 25 | must |
| UT-17 | Active cluster cap at 50 | Create 51 clusters | Oldest cluster evicted, 50 remain | must |
| UT-18 | Cluster expiry after 5 min | Cluster with no new instances for 5+ minutes | Cluster removed from active set | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end cluster formation via WebSocket | Extension sends errors -> Server clusters them | Clusters visible via `analyze(target: "errors")` | must |
| IT-2 | Alert piggyback on observe | Cluster forms with 3+ instances -> next `observe` call | Response includes error_cluster alert in `_alerts` | must |
| IT-3 | Cluster + unclustered coexistence | Mix of clusterable and unique errors | Both `clusters` array and `unclustered_errors` count correct | must |
| IT-4 | Memory pressure eviction | System under memory pressure + active clusters | Clusters evicted before individual error buffer entries | should |
| IT-5 | Server restart clears clusters | Active clusters exist -> server restart | All clusters gone, fresh clustering starts | must |
| IT-6 | Cross-tab clustering | Errors from tab A and tab B with same root cause | Errors clustered together regardless of source tab | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Cluster matching latency | Time to match one error against 50 clusters | < 1ms | must |
| PT-2 | Stack frame parsing speed | Time to parse one error's stack trace | < 0.5ms | must |
| PT-3 | Message normalization speed | Time to normalize one error message | < 0.1ms | must |
| PT-4 | Memory per cluster | Memory footprint of one cluster with 20 instances | < 5KB | must |
| PT-5 | High error rate (100 errors/sec) | Memory usage during error storm | Stable, no OOM | must |
| PT-6 | Cluster matching at scale | 50 active clusters, 1000 errors processed | All within 1ms per match | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Error without stack trace | `{message: "Network error", stack: ""}` | Clustered by message similarity only (single-signal allowed) | must |
| EC-2 | Minified stack trace | `"at a (a.js:1:2345)"` | Frames still comparable across instances from same bundle | must |
| EC-3 | Single error (never clusters) | Only one error in session | `clusters: []`, error only in unclustered count | must |
| EC-4 | Error with only framework frames | All frames are React/Vue/Angular internals | No app-code root cause; falls back to message | should |
| EC-5 | Very long error message (>10KB) | Extremely verbose error with massive stack | Message truncated, normalization still works | should |
| EC-6 | Unicode in error messages | `"TypeError: \u2019name\u2019 is undefined"` | Normalization handles Unicode correctly | should |
| EC-7 | Empty error message | `{message: "", stack: "at foo.js:1"}` | Clustered by stack only; empty message not used for similarity | must |
| EC-8 | Concurrent error arrival | 50 errors arrive simultaneously via WebSocket | No race conditions, all processed, clusters form correctly | must |
| EC-9 | Cluster with errors from different error types | TypeError + ReferenceError with same root cause | Clustered if 2 signals match, different types noted | should |
| EC-10 | Rapid cluster formation and expiry | Burst of errors, then 5+ min silence, then new burst | Old cluster expired, new cluster formed independently | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web app open that can trigger JavaScript errors (e.g., a React app with intentional bugs)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Trigger 4 related errors in the test app (e.g., click a button that calls an undefined function in 4 components) | Console shows 4 TypeError messages in DevTools | Errors appear in browser console | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"what": "errors"}}` | AI receives individual error list | Response shows 4+ error entries | [ ] |
| UAT-3 | `{"tool": "analyze", "arguments": {"target": "errors"}}` | AI receives cluster analysis | Response contains `clusters` array with at least 1 cluster containing the related errors | [ ] |
| UAT-4 | Verify cluster structure | Review AI's cluster output | Cluster has `representative_error`, `root_cause`, `instance_count >= 4`, `affected_components`, `severity` | [ ] |
| UAT-5 | Verify root cause identification | Check `root_cause` field | Points to deepest common app-code function, not a framework function | [ ] |
| UAT-6 | Trigger 1 unrelated error (e.g., network timeout) | Console shows a different kind of error | Error visible in DevTools | [ ] |
| UAT-7 | `{"tool": "analyze", "arguments": {"target": "errors"}}` | AI receives updated analysis | `unclustered_errors` count increased by 1; new error NOT in existing cluster | [ ] |
| UAT-8 | Wait 5+ minutes without triggering errors | No new console errors | Idle period passes | [ ] |
| UAT-9 | `{"tool": "analyze", "arguments": {"target": "errors"}}` | AI receives analysis after expiry window | Previous clusters should be expired/removed if no new instances arrived | [ ] |
| UAT-10 | Trigger the same error pattern again | Console shows related errors again | New cluster forms (not the old expired one) | [ ] |
| UAT-11 | `{"tool": "observe", "arguments": {"what": "errors"}}` | Check for alert piggyback | If cluster has 3+ instances, an `_alerts` section with `error_cluster` type should appear | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No absolute paths in cluster output | Inspect `root_cause` and `instances[].source` in `analyze(target: "errors")` response | Only relative paths like `user-profile.js:42`, no `/Users/...` or `C:\...` | [ ] |
| DL-UAT-2 | Message normalization active | Trigger errors containing UUIDs or URLs, inspect cluster `representative_error` | Variable content replaced with `{uuid}`, `{url}`, etc. | [ ] |
| DL-UAT-3 | No raw request bodies in clusters | Trigger errors caused by API failures with JSON bodies | No request/response body content in any cluster field | [ ] |
| DL-UAT-4 | Alert payload uses normalized data | Check `_alerts` content for `error_cluster` alert | Alert `message` uses normalized text, not raw error strings with user data | [ ] |

### Regression Checks
- [ ] Existing `observe(what: "errors")` still returns individual errors unchanged
- [ ] Existing `observe` response format not broken by alert piggyback (no alerts = single content block)
- [ ] Server memory usage stable after processing 100+ errors
- [ ] Extension performance not degraded (errors forwarded at same speed)

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

---
status: proposed
scope: feature/temporal-graph/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-temporal-graph
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Cross-Session Temporal Graph

> QA plan for the Cross-Session Temporal Graph feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Error event descriptions contain raw stack traces with PII | Verify error events store normalized/fingerprinted messages, not raw messages with user data | critical |
| DL-2 | JSONL file on disk is world-readable | Verify `.kaboom/history/events.jsonl` has user-only file permissions | high |
| DL-3 | Event descriptions accumulate user behavior patterns over 90 days | Verify event deduplication and fingerprinting prevent granular user activity reconstruction | high |
| DL-4 | Deploy events expose CI/CD secrets | Verify deploy events from CI webhooks and resource changes store only public commit refs and timestamps, not CI tokens or environment variables | critical |
| DL-5 | Agent-recorded events contain arbitrary text | Verify `configure(action: "record_event")` sanitizes or limits description length, preventing storage of arbitrary sensitive data by a misconfigured AI | medium |
| DL-6 | Causal links expose internal system relationships | Verify relationship links between events do not reveal infrastructure topology beyond what browser data shows | medium |
| DL-7 | Pattern detection reveals long-term user habits | Verify pattern descriptions are behavioral summaries (e.g., "recurring error") not user activity profiles | medium |
| DL-8 | Event metadata contains raw metric values | Verify metric values in regression/baseline-shift events are performance numbers only, not user-identifying data | low |
| DL-9 | Evicted event content remains readable in JSONL file | Verify eviction rewrites the file, not just marks entries as deleted (leaving data on disk) | high |
| DL-10 | Agent name in origin="agent" events reveals client identity | Verify MCP client name stored does not expose private infrastructure names beyond the tool's own identifier | medium |

### Negative Tests (must NOT leak)
- [ ] No raw error messages with user PII in event descriptions
- [ ] No CI/CD tokens, API keys, or environment variables in deploy events
- [ ] No absolute file system paths in event source fields
- [ ] No reconstructable user navigation history from event log
- [ ] No evicted event data accessible after rewrite (no ghost data on disk)
- [ ] No private infrastructure names in agent-origin events
- [ ] No raw request/response bodies in any event metadata

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Event types are unambiguous | AI distinguishes error/regression/resolution/baseline_shift/deploy/fix event types | [ ] |
| CL-2 | "explicit" vs "inferred" confidence | AI understands explicit links are confirmed by AI/user, inferred links are automatic hypotheses | [ ] |
| CL-3 | Event status meanings | AI distinguishes "active" (still happening), "resolved" (fixed), "superseded" (replaced by newer) | [ ] |
| CL-4 | Origin distinction: system vs agent | AI understands system events are ground truth from browser, agent events are AI interpretations | [ ] |
| CL-5 | "No history recorded yet" | AI understands empty history is expected on first use, not an error | [ ] |
| CL-6 | Orphaned links (target_evicted) | AI understands the referenced event was removed by retention policy, not that the link is broken | [ ] |
| CL-7 | Recurring pattern meaning | AI understands "recurring error seen 3 times in 30 days" as a pattern, not 3 individual events | [ ] |
| CL-8 | Time window semantics | AI understands `since: "7d"` returns events from the last 7 days, not events created 7 days ago | [ ] |
| CL-9 | Deduplication vs count | AI understands one event with `occurrence_count: 100` means 100 instances, not 100 separate events | [ ] |
| CL-10 | "possibly_caused_by" vs "caused_by" | AI treats "possibly" as hypothesis needing verification, not confirmed causality | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might treat inferred causal links as confirmed -- verify confidence field is prominent
- [ ] AI might assume "resolved" means permanently fixed -- verify it means "no longer occurring", could recur
- [ ] AI might confuse event timestamp with when the event was recorded vs when the condition started -- verify semantics
- [ ] AI might assume all events in the 7d window are related -- verify no grouping is implied beyond explicit links
- [ ] AI might not realize pattern detection is computed on-demand, not real-time -- verify no staleness issues

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Query recent error history | 1 step: `analyze(target: "history", query: {type: "error", since: "7d"})` | No -- already minimal |
| Record a fix event | 1 step: `configure(action: "record_event", event: {type: "fix", ...})` | No -- already minimal |
| Find related events | 1 step: `analyze(target: "history", query: {related_to: "evt_123"})` | No -- already minimal |
| Get full history overview | 1 step: `analyze(target: "history")` (no filters) | No -- already minimal |
| Search events by description | 1 step: `analyze(target: "history", query: {pattern: "UserProfile"})` | No -- already minimal |
| Understand why regression happened | 2 steps: query history for regression, follow causal links | Yes -- could auto-include linked events, but current approach is clear |

### Default Behavior Verification
- [ ] Events recorded automatically (errors, regressions, resolutions, baseline shifts, deploys)
- [ ] No explicit opt-in required for event recording
- [ ] 90-day retention applied automatically on server startup
- [ ] Pattern detection runs automatically on query
- [ ] Automatic correlation (temporal proximity) works without configuration
- [ ] JSONL file created automatically on first event

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Error event recorded on first fingerprint | New error fingerprint seen | Event with type: "error", origin: "system" appended to JSONL | must |
| UT-2 | Repeat error increments count, no new event | Same error fingerprint seen 5 more times | Original event's occurrence_count updated, no new event | must |
| UT-3 | Regression event on threshold breach | Performance metric exceeds baseline by threshold | Event with type: "regression", metric values in metadata | must |
| UT-4 | Resolution event after 5 min absence | Error absent for 5+ minutes during active browsing | Event with type: "resolution", status: "resolved" | must |
| UT-5 | Baseline shift event on update | Baseline updated with new values | Event with type: "baseline_shift", old/new values | must |
| UT-6 | Deploy event on resource change | Resource list changes significantly | Event with type: "deploy", origin: "system" | must |
| UT-7 | Deploy event on CI webhook | CI webhook receives deploy notification | Event with type: "deploy", source info included | must |
| UT-8 | AI-recorded fix event | `configure(action: "record_event", event: {type: "fix", ...})` | Event with origin: "agent", agent name recorded | must |
| UT-9 | AI-recorded event with explicit link | Event with `related_to: "evt_error_123"` | Link stored with confidence: "explicit" | must |
| UT-10 | AI-recorded event with invalid related_to | `related_to: "evt_nonexistent"` | Event recorded WITHOUT link, warning in response | must |
| UT-11 | AI-recorded event missing required fields | `{type: "fix"}` without description | Error response listing valid fields | must |
| UT-12 | Automatic correlation: regression after resource change | Regression detected within 30s of new script | "possibly_caused_by" link created, confidence: "inferred" | must |
| UT-13 | Automatic correlation: error resolved after reload | Error disappears within 60s of resource list change | "possibly_resolved_by" link created, confidence: "inferred" | must |
| UT-14 | Query with type filter | Query `{type: "error"}` | Only error events returned | must |
| UT-15 | Query with since filter | Query `{since: "1d"}` | Only events from last 24 hours | must |
| UT-16 | Query with related_to filter | Query `{related_to: "evt_1"}` | Events linked to evt_1 returned | must |
| UT-17 | Query with pattern filter | Query `{pattern: "UserProfile"}` | Events with "UserProfile" in description | must |
| UT-18 | Pattern detection: recurring error | Same error fingerprint appears, resolves, reappears 3 times | Pattern: "Recurring error seen 3 times in 30 days" | must |
| UT-19 | 90-day eviction | Events from 91 days ago and 89 days ago | 91-day event evicted, 89-day event retained | must |
| UT-20 | Corrupted JSONL line handling | One malformed JSON line in events.jsonl | Line skipped, subsequent lines read correctly | must |
| UT-21 | Empty history file | No events recorded | Empty response, no error, "No history recorded yet" | must |
| UT-22 | Orphaned link after eviction | Event references target evicted by retention | Link preserved, marked "target_evicted" in query | must |
| UT-23 | Event ID generation | Two events recorded in sequence | Unique IDs with format `evt_{timestamp}_{random}` | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Error -> Event -> Query round-trip | Extension error -> Server detects new fingerprint -> Event recorded -> Query returns it | Event visible in history query | must |
| IT-2 | Regression -> Alert -> Event chain | Performance regression -> push alert fires -> temporal event recorded | Both alert and event exist for same regression | must |
| IT-3 | Disk persistence round-trip | Record events -> server restart -> query history | Events loaded from JSONL file, all present | must |
| IT-4 | Cross-session pattern detection | Session 1: error appears + resolves. Session 2: same error recurs | Pattern detected: "recurring error" | must |
| IT-5 | Configure record_event via MCP | AI calls `configure(action: "record_event")` via MCP protocol | Event stored with origin="agent", agent name from MCP client | must |
| IT-6 | Concurrent event recording + querying | Background event recording while AI queries history | No race conditions, consistent results | must |
| IT-7 | Eviction on startup | Events.jsonl has old entries -> server starts | Old entries removed, file rewritten | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Event append speed | Time to append one event to JSONL | < 1ms | must |
| PT-2 | Full history query (90 days) | Time to scan 5000 events | < 50ms | must |
| PT-3 | Pattern detection speed | Time to compute patterns on query | < 100ms | must |
| PT-4 | Pattern cache effectiveness | Second query within 60s uses cached patterns | Cache hit, < 5ms | should |
| PT-5 | Startup eviction speed | Time to read, filter, rewrite 5000 events | < 200ms | must |
| PT-6 | Disk space for 90 days | File size for ~500 events/month * 3 months | < 1MB | must |
| PT-7 | Memory for query results | Memory footprint of query response | < 500KB | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No history file exists | First query on fresh install | File created, empty response, no error | must |
| EC-2 | Corrupted single JSONL line | One bad line in middle of file | Skipped with warning, other lines read correctly | must |
| EC-3 | Very long session (1000+ potential events) | 1000 errors of the same type | Deduplicated to 1 event with occurrence_count: 1000 | must |
| EC-4 | Concurrent server instances | Two servers reading same JSONL file | Second instance reads only, no corruption | must |
| EC-5 | Clock skew between sessions | System clock changed between sessions | Events sorted by recorded timestamp regardless | must |
| EC-6 | Large history file (>10MB) | Abnormal usage producing >10MB file | Eviction runs immediately, not just at startup | should |
| EC-7 | All events evicted (>90 days old) | Only very old events in file | File rewritten as empty, clean response | must |
| EC-8 | Query with all filters combined | `{type: "error", since: "7d", pattern: "UserProfile"}` | Filters applied as AND (all must match) | must |
| EC-9 | Agent records event with very long description | 10KB description text | Stored (within limits), no truncation or crash | should |
| EC-10 | Relationship cycle | Event A links to B, B links to A | No infinite loop in query traversal | must |
| EC-11 | Pattern with time-of-day correlation | Errors consistently at 2 PM across sessions | Time-of-day pattern noted in response | should |
| EC-12 | Deploy-correlated regressions pattern | 3 deploys each followed by regression | Pattern: "Deploy-correlated regressions" detected | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web app capable of generating errors and performance changes
- [ ] `.kaboom/history/` directory accessible (created automatically)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Trigger a JavaScript error in the test app | Console shows TypeError in DevTools | Error appears in browser | [ ] |
| UAT-2 | Wait 5 seconds for event recording | No user action needed | Server detects new error fingerprint | [ ] |
| UAT-3 | `{"tool": "analyze", "arguments": {"target": "history", "query": {"type": "error", "since": "1h"}}}` | AI receives history | At least 1 error event with correct description and timestamp | [ ] |
| UAT-4 | Record a fix event: `{"tool": "configure", "arguments": {"action": "record_event", "event": {"type": "fix", "description": "Fixed null user in UserProfile", "related_to": "<evt_id_from_step_3>"}}}` | AI confirms event recorded | Fix event stored with explicit link to the error event | [ ] |
| UAT-5 | `{"tool": "analyze", "arguments": {"target": "history", "query": {"related_to": "<evt_id_from_step_3>"}}}` | AI receives linked events | Both error event and fix event returned, linked by relationship | [ ] |
| UAT-6 | Trigger a performance regression (add slow script) | Page loads noticeably slower | Extension captures slow performance snapshot | [ ] |
| UAT-7 | Wait for regression detection | No user action | Server detects regression against baseline | [ ] |
| UAT-8 | `{"tool": "analyze", "arguments": {"target": "history", "query": {"type": "regression", "since": "1h"}}}` | AI receives history | Regression event with metric values and severity | [ ] |
| UAT-9 | Deploy new code (reload with different resources) | Human changes app code and reloads | New resources loaded in browser | [ ] |
| UAT-10 | `{"tool": "analyze", "arguments": {"target": "history", "query": {"type": "deploy", "since": "1h"}}}` | AI receives history | Deploy event recorded (resource change detected) | [ ] |
| UAT-11 | Check for automatic correlation | Query history for regression event | Regression has "possibly_caused_by" link to deploy event (if within 30s) | [ ] |
| UAT-12 | Fix the error (remove the buggy code), wait 5+ minutes | Error stops occurring | No more errors in console | [ ] |
| UAT-13 | `{"tool": "analyze", "arguments": {"target": "history", "query": {"since": "1h"}}}` | AI receives full history | Resolution event with "possibly_resolved_by" link | [ ] |
| UAT-14 | Restart Kaboom server, then query history | Human restarts server | Events persist from JSONL file | [ ] |
| UAT-15 | `{"tool": "analyze", "arguments": {"target": "history"}}` | AI receives history | All events from before restart are present with correct timestamps | [ ] |
| UAT-16 | Search by pattern: `{"tool": "analyze", "arguments": {"target": "history", "query": {"pattern": "UserProfile"}}}` | AI receives filtered results | Only events mentioning "UserProfile" in description | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No raw PII in error event descriptions | Inspect error events in history response | Fingerprinted/normalized messages, no user emails or tokens | [ ] |
| DL-UAT-2 | JSONL file permissions | `ls -la .kaboom/history/events.jsonl` | File owned by current user, not world-readable | [ ] |
| DL-UAT-3 | Deploy events have no CI secrets | Trigger CI webhook or resource-change deploy event | No tokens, env vars, or CI credentials in event | [ ] |
| DL-UAT-4 | Eviction removes data from disk | Set retention to 1 day, add old events, restart server | Old events physically removed from file (not just hidden) | [ ] |
| DL-UAT-5 | Agent-recorded events do not contain arbitrary secrets | Record fix event with sensitive-looking description | Description stored but not broadcast; only accessible via explicit query | [ ] |

### Regression Checks
- [ ] Existing error observation (`observe(what: "errors")`) unaffected by temporal graph recording
- [ ] Existing performance monitoring unaffected by event recording overhead
- [ ] Server startup time acceptable with 1000+ events in JSONL file
- [ ] No additional memory usage beyond query results when history is idle
- [ ] JSONL file does not grow unboundedly (eviction works)

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

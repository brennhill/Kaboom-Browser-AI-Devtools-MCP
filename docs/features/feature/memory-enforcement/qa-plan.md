---
status: proposed
scope: feature/memory-enforcement/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-memory-enforcement
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Memory Enforcement

> QA plan for the Memory Enforcement feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Memory enforcement evicts buffer data under pressure -- the primary risks are that eviction order leaks data priority assumptions, that memory status responses reveal internal architecture details, and that evicted data leaves traces.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Evicted data leaves memory traces | After eviction, verify that evicted entries are truly dereferenced and not accessible through any alternate buffer reference or cached response | critical |
| DL-2 | Memory status exposes internal byte-level details | The `memory` status response includes `total_bytes`, per-buffer bytes -- verify this does not reveal sensitive data patterns (e.g., large network body sizes hinting at specific API responses) | medium |
| DL-3 | Minimal mode flag observable to agents | Agents can learn the server entered critical memory state -- verify this does not reveal information about captured traffic volume that could hint at application behavior | low |
| DL-4 | Eviction counters reveal traffic patterns | `total_evictions` and `evicted_entries` counters could reveal how much traffic an application generates -- verify this is acceptable for localhost-only use | low |
| DL-5 | Network body rejection (429) leaks state | When memory-exceeded flag is set, network body POSTs return 429 -- verify the error response does not include memory details or buffer contents | high |
| DL-6 | Evicted entries accessible during eviction window | Between eviction trigger and completion, a concurrent read could see partially-evicted buffers | critical |
| DL-7 | Extension-side memory enforcement leaks to server | Extension sends memory state info to server -- verify this does not include page content or user data | medium |

### Negative Tests (must NOT leak)
- [ ] After eviction, evicted entries cannot be retrieved by any MCP tool call
- [ ] Memory status response contains only aggregate byte counts, not entry-level details
- [ ] 429 rejection response for network bodies includes only the error message, no buffer contents
- [ ] Concurrent reads during eviction see a consistent (pre- or post-eviction) snapshot, never partial state
- [ ] Extension memory data sent to server contains only buffer sizes, not entry content

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Three threshold levels are clearly named | Response uses "soft", "hard", "critical" consistently and includes the byte values (20MB, 50MB, 100MB) | [ ] |
| CL-2 | Memory units are unambiguous | All memory values are in bytes with field names ending in `_bytes`, not ambiguous "size" or "memory" fields | [ ] |
| CL-3 | Minimal mode explanation | When `minimal_mode: true`, the response explains what it means (halved capacities, persists until restart) | [ ] |
| CL-4 | Eviction vs. rejection distinction | AI understands eviction (removing oldest entries) vs. rejection (refusing new data) -- terms used consistently | [ ] |
| CL-5 | Memory-exceeded flag semantics | AI understands this flag means network body capture is paused, not that the server is down or all capture stopped | [ ] |
| CL-6 | Eviction priority order is documented | Response or documentation makes clear: network bodies first, then WebSocket, then actions | [ ] |
| CL-7 | Eviction percentage clarity | AI understands "25% of each buffer" means oldest 25% of entries, not 25% of total memory | [ ] |
| CL-8 | Threshold values are absolute, not relative | AI understands 20MB/50MB/100MB are fixed values, not percentages of available system RAM | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might interpret "memory exceeded" as a server crash or outage -- verify status messages clarify the server continues operating
- [ ] AI might think minimal mode can be disabled via MCP tool call -- verify documentation states it persists until restart
- [ ] AI might confuse server-side and extension-side memory enforcement -- verify responses clearly label which side is reporting
- [ ] AI might think eviction means data loss across the entire system -- verify it is clear eviction is from in-memory buffers only (disk persistence unaffected)
- [ ] AI might interpret `total_evictions: 3` as "3 entries evicted" vs. "3 eviction cycles" -- verify field name is unambiguous (use `eviction_cycles` or clarify in description)

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (passive feature -- no user action required)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Check memory status | 1 step: `configure(what: "health")` or observe diagnostics | No -- already minimal |
| Respond to memory pressure | 0 steps (automatic) | N/A -- feature is fully automatic |
| Exit minimal mode | 1 step: restart the server | No -- intentionally requires manual intervention |
| Understand current memory state | 1 step: read memory section of health response | No -- already minimal |

### Default Behavior Verification
- [ ] Memory enforcement is active with zero configuration -- thresholds are built-in (20/50/100 MB)
- [ ] No CLI flags or environment variables are required to enable memory enforcement
- [ ] Eviction happens automatically without any MCP tool call
- [ ] Memory status is included in health responses without opt-in

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Memory below soft limit -- no eviction | `calcTotalMemory()` returns 15MB | No eviction triggered, all buffers unchanged | must |
| UT-2 | Memory at 21MB triggers soft eviction | `calcTotalMemory()` returns 21MB | Oldest 25% of each buffer evicted | must |
| UT-3 | Memory at 51MB triggers hard eviction | `calcTotalMemory()` returns 51MB | Oldest 50% evicted, memory-exceeded flag set | must |
| UT-4 | Memory at 101MB triggers critical | `calcTotalMemory()` returns 101MB | ALL buffers cleared, `minimalMode = true` | must |
| UT-5 | Minimal mode halves buffer capacities | Enter minimal mode | WS capacity = 250, NB = 50, Actions = 100 | must |
| UT-6 | Minimal mode persists after memory drops | Enter minimal mode, then memory drops to 5MB | `minimalMode` still true | must |
| UT-7 | Memory-exceeded flag set at hard limit | Cross hard limit | `isMemoryExceeded()` returns true | must |
| UT-8 | Memory-exceeded flag cleared below hard limit | Memory drops below 50MB | `isMemoryExceeded()` returns false | must |
| UT-9 | Network bodies evicted first | Memory at 25MB with mixed buffer content | Network bodies lose more entries than WS or actions | must |
| UT-10 | Eviction cooldown (1 second) | Two ingests within 500ms, both above soft limit | Only one eviction cycle occurs | must |
| UT-11 | `calcTotalMemory()` sums all buffers | Known buffer sizes | Returns correct sum of WS + NB + actions memory | must |
| UT-12 | `calcWSMemory()` estimates correctly | 10 WS events with known data lengths | Returns `sum(200 + len(data))` for each event | must |
| UT-13 | `calcNBMemory()` estimates correctly | 5 network bodies with known request/response sizes | Returns `sum(300 + len(req) + len(resp))` for each | must |
| UT-14 | After eviction, oldest entries gone, newest preserved | Buffer with entries [1..100], evict 25% | Entries [1..25] removed, [26..100] remain | must |
| UT-15 | Ring buffer rotation works after eviction | Evict, then add new entries past capacity | New entries added correctly, oldest rotate out | must |
| UT-16 | AddWebSocketEvents at hard limit rejects | Memory at 52MB, try adding WS events | Events rejected (not added) | must |
| UT-17 | Minimal mode + ingest at reduced capacity | In minimal mode, add entries | Data added at half capacity (250 WS max) | should |
| UT-18 | Memory status response format | Query memory status | Response includes all fields: total_bytes, per-buffer, limits, mode, counters | should |
| UT-19 | Eviction with empty buffer | One buffer empty, others full, memory above soft limit | No crash on empty buffer, other buffers evicted normally | should |
| UT-20 | Enhanced actions 500 bytes per entry estimate | 10 enhanced actions | `calcActionsMemory()` returns 5000 | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Ingest triggers eviction when above soft limit | HTTP ingest endpoint + Capture + memory check | POST network bodies, memory crosses 20MB, oldest entries evicted | must |
| IT-2 | Periodic check triggers eviction | Background goroutine + Capture | Memory above soft limit detected by 10s periodic check, eviction occurs | must |
| IT-3 | Memory status in health endpoint | Health HTTP handler + Capture | GET /v4/health includes memory section with correct values | must |
| IT-4 | Memory status in MCP tool response | MCP configure/health handler + Capture | MCP tool response includes memory state | must |
| IT-5 | Network body POST rejected at hard limit | HTTP ingest + memory-exceeded flag | POST returns 429 when flag is set | must |
| IT-6 | Eviction under concurrent ingest | Multiple goroutines ingesting + eviction | No race condition (go test -race passes) | must |
| IT-7 | Extension memory enforcement independent | Extension memory check + server memory check | Both operate independently without conflict | should |
| IT-8 | Minimal mode survives continued operation | Enter minimal mode, continue ingesting | Server operates at half capacity without crash | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Memory calculation time | Latency of `calcTotalMemory()` | < 1ms | must |
| PT-2 | Eviction of 25% of 500 entries | Eviction time | < 0.5ms | must |
| PT-3 | Periodic check overhead | Time per check cycle | < 1ms | must |
| PT-4 | No allocations during eviction | Allocation count | 0 allocations (reslicing only) | should |
| PT-5 | Eviction of 50% of max buffers | Eviction time for hard limit scenario | < 1ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | All buffers empty but periodic check runs | Memory = 0, periodic check fires | No eviction, no crash | must |
| EC-2 | Single giant network body (100KB) | One 100KB body in otherwise empty buffer | Body truncation at capture time prevents this; if it occurs, next check evicts it | should |
| EC-3 | Rapid eviction cycling at soft limit boundary | Sustained load holding memory at ~20MB | Max 1 eviction per second due to cooldown | must |
| EC-4 | Race condition during eviction | Concurrent AddWebSocketEvents and eviction | Mutex ensures serialized access, no data corruption | must |
| EC-5 | Extension and server both evicting simultaneously | Extension reduces sends while server evicts | Both operate independently, additive protection | should |
| EC-6 | Memory drops after critical clear | After critical clear, new data arrives slowly | Buffers accept data at half capacity, memory stays low | should |
| EC-7 | Eviction ratio results in zero entries to remove | Buffer has 1 entry, evict 25% = 0.25 entries | At least 1 entry removed (round up) or buffer left alone if too small | should |
| EC-8 | Memory exceeded flag cleared by periodic check | Flag set, then memory drops below hard limit between ingests | Periodic check clears the flag | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application that generates significant network traffic (e.g., an app making many API calls with large JSON responses)
- [ ] Access to server logs to observe eviction events

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"what": "health"}}` | Review the memory section of the response | Response includes `memory` object with `total_bytes`, per-buffer breakdown, `soft_limit`, `hard_limit`, `critical_limit`, `minimal_mode: false`, `total_evictions: 0` | [ ] |
| UAT-2 | Generate heavy network traffic (open a page making 100+ API calls with large bodies) | Watch server logs for eviction messages | If memory crosses 20MB, server logs show eviction activity | [ ] |
| UAT-3 | `{"tool": "configure", "arguments": {"what": "health"}}` | Compare memory values to UAT-1 | Memory values increased; if eviction occurred, `total_evictions > 0` and `evicted_entries > 0` | [ ] |
| UAT-4 | `{"tool": "observe", "arguments": {"what": "get_network"}}` | Compare entry count to what browser shows | If eviction occurred, oldest network entries are missing from Kaboom output but visible in browser Network tab history | [ ] |
| UAT-5 | Continue generating traffic to push memory above 50MB (hard limit) | Check server logs for hard limit messages | Server logs "hard limit exceeded", memory-exceeded flag set | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"what": "health"}}` | Check memory status | `minimal_mode` may be false but memory should show reduced counts due to 50% eviction | [ ] |
| UAT-7 | Verify network body capture is paused when memory-exceeded | Try to trigger new network body captures | Server rejects network body POSTs (extension receives 429 or equivalent) | [ ] |
| UAT-8 | Let traffic subside, wait for memory to drop | Monitor health status | Memory drops below hard limit, memory-exceeded flag clears, network body capture resumes | [ ] |
| UAT-9 | `{"tool": "observe", "arguments": {"what": "get_console_logs"}}` | Check that console entries are unaffected by network body eviction | Console entries still present and correctly ordered (eviction targets network bodies first) | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Evicted entries not retrievable | After eviction, call observe for the time range of evicted entries | No evicted entries appear in response | [ ] |
| DL-UAT-2 | Memory status is aggregate only | Inspect health response memory section | Only byte counts and counters, no entry-level details | [ ] |
| DL-UAT-3 | 429 rejection contains no data | Monitor the 429 response when memory-exceeded | Response body contains only error message, no buffer contents | [ ] |
| DL-UAT-4 | Consistent snapshot during eviction | Read buffers while eviction is occurring (timing-dependent) | Either pre- or post-eviction data, never a mix | [ ] |

### Regression Checks
- [ ] All MCP tools continue to work normally when memory is below soft limit
- [ ] Buffer reads return correct data after eviction (no off-by-one errors)
- [ ] WebSocket connection tracking is unaffected by memory eviction
- [ ] Server startup time is not affected by memory enforcement initialization
- [ ] Existing memory-related CLI flags (if any) continue to work

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

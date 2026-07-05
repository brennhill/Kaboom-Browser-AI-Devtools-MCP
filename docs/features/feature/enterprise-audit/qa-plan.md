---
status: proposed
scope: feature/enterprise-audit/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-enterprise-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Enterprise Audit & Governance

> QA plan for the Enterprise Audit & Governance feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification. This feature spans four tiers: AI Audit Trail, Data Governance, Operational Safety, and Multi-Tenant & Access Control.

---

## 1. Data Leak Analysis

**Goal:** Verify the enterprise audit features do NOT expose data they are designed to protect. The audit trail itself must not become a data leak vector -- it must store metadata about access, never the accessed data itself.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Audit log stores full request parameters | Audit entries must contain only parameter SUMMARY (tool name + key params), NOT full request body | critical |
| DL-2 | Audit log stores full response bodies | Audit entries must contain only response SIZE and STATUS, NOT response content | critical |
| DL-3 | Redaction audit entries store redacted content | Redaction log entries must store pattern name + char count, NEVER the matched sensitive data | critical |
| DL-4 | Client identity exposes auth credentials | Session records must contain client name/version, NOT authentication tokens | high |
| DL-5 | Session ID is predictable | Session IDs must use cryptographic randomness (`crypto/rand`), not sequential counters | high |
| DL-6 | `export_data` includes unredacted captures | Export scope `captures` must apply active redaction patterns before serializing | critical |
| DL-7 | `export_data` audit scope leaks sensitive metadata | Audit export must not include any fields that were intentionally omitted from individual queries | high |
| DL-8 | TTL bypass via export | `export_data` must respect TTL -- entries older than TTL must not appear in exports | high |
| DL-9 | Configuration profiles expose API key | Profile info in health endpoint must not include the API key value | high |
| DL-10 | Health metrics expose captured data | `configure({action:"health"})` must report buffer SIZES and COUNTS, not buffer CONTENTS | high |
| DL-11 | Tool allowlist/blocklist reveals hidden tools | Hidden tools must not appear in `tools/list` or error messages that confirm their existence | medium |
| DL-12 | Read-only mode bypass via MCP | AI agent must not be able to disable read-only mode via any tool call | critical |
| DL-13 | Project isolation breach | Data captured in project A must never appear when querying project B | critical |
| DL-14 | Auth attempt audit includes API key | Auth audit entries must log success/failure, NOT the key that was submitted | critical |
| DL-15 | Config file secrets in health output | Health endpoint must not expose config file path contents or API key values | high |

### Negative Tests (must NOT leak)
- [ ] `get_audit_log` entries contain no `request_body` or `response_body` fields
- [ ] `get_audit_log` with `type: "redaction"` entries contain no `matched_content` or `original_text` fields
- [ ] `export_data` with `scope: "captures"` has redaction patterns applied to all exported data
- [ ] `export_data` with `scope: "audit"` contains no sensitive captured data
- [ ] `configure({action:"health"})` response contains no buffer contents, only sizes and utilization metrics
- [ ] Session records contain no auth token fields
- [ ] Calling a blocked tool returns generic "method not found" -- does not reveal the tool exists but is blocked
- [ ] Read-only mode cannot be disabled by any MCP tool call
- [ ] Project A data is not accessible from project B queries

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading audit, health, and governance responses can unambiguously understand what they mean and act appropriately.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Audit entry fields are self-documenting | `tool_name`, `status`, `duration_ms`, `response_size_bytes`, `redaction_count` are clear | [ ] |
| CL-2 | Session ID format is human-readable | Format `s_<base36timestamp>_<random>` is recognizable as a session ID | [ ] |
| CL-3 | Client identity values match known AI tools | "claude-code", "cursor", "windsurf" -- not internal codenames | [ ] |
| CL-4 | TTL behavior is clear | When data is TTL-expired, responses indicate "0 entries" not "error" | [ ] |
| CL-5 | Profile names are self-documenting | `short-lived`, `restricted`, `paranoid` convey severity intuitively | [ ] |
| CL-6 | Rate limit error includes retry info | Error response has `retry_after_seconds` field, not just "rate limited" | [ ] |
| CL-7 | Health metrics have units | `memory_bytes`, `duration_ms`, `uptime_seconds` -- units in field names | [ ] |
| CL-8 | Export format is documented | `export_data` returns JSON Lines with `type` field per line | [ ] |
| CL-9 | Audit log pagination is clear | `limit` and `offset` parameters, total count in response | [ ] |
| CL-10 | Read-only error message is actionable | Error says "read-only mode is active -- this operation is disabled" not generic "forbidden" | [ ] |
| CL-11 | Tool allowlist error is standard MCP | Hidden tool returns `-32601 Method not found` per JSON-RPC spec | [ ] |
| CL-12 | Project isolation is observable | Health metrics show per-project buffer utilization | [ ] |
| CL-13 | Redaction audit entries are distinguishable | Audit entry `type: "redaction"` is clearly distinct from `type: "tool_call"` | [ ] |
| CL-14 | Failure status values are enumerated | `status` field: `success`, `error`, `rate-limited`, `redacted` -- not free-form | [ ] |

### Common LLM Misinterpretation Risks
- [ ] Risk: LLM interprets "0 entries" from TTL expiry as "no data was captured" -- verify health metrics show buffer eviction counts
- [ ] Risk: LLM thinks `export_data` writes to disk -- verify response clarifies data is returned as tool response, not written to filesystem
- [ ] Risk: LLM tries to call `configure` to disable read-only mode -- verify error clearly states read-only is a server-start flag
- [ ] Risk: LLM confuses per-tool rate limits with global HTTP rate limits -- verify different error codes and messages
- [ ] Risk: LLM assumes "unknown" client identity means unauthorized -- verify "unknown" means MCP client did not send clientInfo
- [ ] Risk: LLM treats `redaction_count: 0` as "no sensitive data" when it might mean "redaction patterns not configured" -- verify context

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium-High (many features, but each individually simple)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Enable audit trail | 0 steps: always active | No -- zero-config by design |
| Query audit log | 1 MCP call: `get_audit_log` with optional filters | No -- already minimal |
| Set TTL | 1 step: `--ttl=1h` flag | No -- already minimal |
| Use a profile | 1 step: `--profile=restricted` flag | No -- already minimal |
| Export session data | 1 MCP call: `export_data` with scope | No -- already minimal |
| Enable read-only mode | 1 step: `--read-only` flag | No -- already minimal |
| Configure tool allowlist | 1 step: `--tools-allow="observe,analyze"` flag | No -- already minimal |
| Full enterprise setup | 3 steps: set profile + API key + redaction config | Could offer `--enterprise` meta-flag |
| Check health metrics | 1 MCP call: `configure({action:"health"})` | No -- already minimal |
| Create project isolation | 1 HTTP call: `POST /projects` | No -- already minimal |

### Default Behavior Verification
- [ ] Audit trail is always active (zero configuration) but zero-cost when unused
- [ ] TTL defaults to unlimited (existing behavior preserved)
- [ ] No redaction occurs unless configured
- [ ] No per-tool rate limits unless configured
- [ ] All tools available unless allowlist/blocklist configured
- [ ] Read-only mode is off by default
- [ ] Single "default" project when no projects created
- [ ] Profiles override individual settings, but individual flags override profiles
- [ ] Config priority: CLI flags > env vars > config file > defaults

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Audit entry creation on tool call | MCP tool call dispatched | Entry with timestamp, session_id, tool_name, status | must |
| UT-2 | Audit entry records duration | Tool takes 50ms | `duration_ms: 50` (approximately) | must |
| UT-3 | Audit entry records response size | Tool returns 1KB response | `response_size_bytes: ~1024` | must |
| UT-4 | Audit entry does NOT store request body | Tool called with large params | No `request_body` or `params` field in entry | must |
| UT-5 | Audit entry does NOT store response body | Tool returns large response | No `response_body` field in entry | must |
| UT-6 | Audit ring buffer eviction | Insert 10001 entries (buffer size 10000) | Oldest entry evicted | must |
| UT-7 | `get_audit_log` time range filter | Entries at T1, T2, T3; filter T2-T3 | Only T2 and T3 returned | must |
| UT-8 | `get_audit_log` tool name filter | Entries for observe, generate; filter observe | Only observe entries returned | must |
| UT-9 | `get_audit_log` status filter | Success + error entries; filter errors | Only error entries returned | must |
| UT-10 | `get_audit_log` reverse chronological order | Multiple entries | Most recent first | must |
| UT-11 | `get_audit_log` pagination | 100 entries, limit 10 | First 10 returned with pagination info | must |
| UT-12 | Session ID generation: uniqueness | Generate 10000 IDs | All unique | must |
| UT-13 | Session ID generation: format | Generate ID | Matches `s_<base36>_<random6>` pattern | must |
| UT-14 | Session ID generation: sortability | IDs from T1 < T2 | T1 ID sorts before T2 ID lexicographically | must |
| UT-15 | Client identity from MCP initialize | `clientInfo: {name: "claude-code", version: "1.0"}` | Stored as session client identity | must |
| UT-16 | Client identity missing | No clientInfo in initialize | Labeled as "unknown" | must |
| UT-17 | Redaction audit entry created | Pattern matches in response | Sub-entry with pattern_name, field_path, char_count | must |
| UT-18 | Redaction audit does NOT store content | Pattern matches "Bearer secret123" | Entry has `chars_redacted: 14`, no content | must |
| UT-19 | TTL: entries older than TTL skipped on read | TTL=5min, entry at T-10min | Entry invisible in tool responses | must |
| UT-20 | TTL: entries within TTL returned | TTL=5min, entry at T-2min | Entry visible in tool responses | must |
| UT-21 | TTL: unlimited (default) | TTL not set | All entries returned regardless of age | must |
| UT-22 | TTL: minimum 1 minute enforced | `--ttl=30s` | Error or clamped to 1 minute | must |
| UT-23 | Profile `short-lived` sets correct values | `--profile=short-lived` | TTL=15min, specific redaction patterns enabled | must |
| UT-24 | Profile `restricted` sets correct values | `--profile=restricted` | TTL=30min, all builtins enabled, rate limits set, read-only | must |
| UT-25 | Profile `paranoid` sets correct values | `--profile=paranoid` | TTL=5min, all builtins, strict rate limits, read-only, tool allowlist | must |
| UT-26 | Profile values overridden by explicit flags | `--profile=restricted --ttl=1h` | TTL=1h (flag wins), rest from profile | must |
| UT-27 | `export_data` audit scope | Ring buffer with entries | JSON Lines output with all audit entries | must |
| UT-28 | `export_data` captures scope | Buffers with data | JSON Lines with `type` field per entry | must |
| UT-29 | `export_data` respects TTL | TTL active, old entries | Old entries excluded from export | must |
| UT-30 | `export_data` applies redaction | Redaction patterns active | Exported data has patterns applied | must |
| UT-31 | Read-only mode blocks clear | `configure` with `action: "clear"` | Error: "read-only mode active" | must |
| UT-32 | Read-only mode blocks noise rules | `configure` with `action: "noise_rule"` | Error: "read-only mode active" | must |
| UT-33 | Read-only mode allows observe | `observe` call | Normal response | must |
| UT-34 | Read-only mode allows generate | `generate` call | Normal response | must |
| UT-35 | Read-only mode allows health checks | `configure({action:"health"})` call | Normal response | must |
| UT-36 | Tool allowlist hides tools | `--tools-allow="observe"` | `tools/list` returns only `observe` | must |
| UT-37 | Tool blocklist hides tools | `--tools-block="configure"` | `tools/list` returns everything except `configure` | must |
| UT-38 | Hidden tool returns method not found | Call hidden tool directly | `-32601` error, no "tool exists but blocked" hint | must |
| UT-39 | Allowlist takes priority over blocklist | Both specified | Allowlist used, blocklist ignored | must |
| UT-40 | Per-tool rate limit enforced | `analyze` at 21/min (limit 20) | 21st call returns error with retry hint | must |
| UT-41 | Rate limit sliding window resets | Wait 60 seconds after hitting limit | Calls accepted again | must |
| UT-42 | Rate limit error format | Rate limited call | `code: -32029`, includes `retry_after_seconds` | must |
| UT-43 | Health metrics: memory breakdown | Active session with data | Per-buffer memory sizes reported | must |
| UT-44 | Health metrics: tool call counts | Multiple tool calls | Accurate per-tool breakdown | must |
| UT-45 | Health metrics: active profile | `--profile=restricted` | `profile: "restricted"` in health | must |
| UT-46 | Config priority: CLI > env > file | Same param in all three | CLI value used | must |
| UT-47 | JSON config file parsing | Valid config file | All values loaded correctly | must |
| UT-48 | JSON config file: invalid JSON | Malformed file | Clear error on startup | must |
| UT-49 | API key NOT accepted from config file | Key in JSON config | Ignored with warning; must use env var or flag | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full tool call audit trail | MCP dispatcher + audit log | Every tool call creates audit entry with all metadata | must |
| IT-2 | Session lifecycle | MCP initialize + tool calls + disconnect | Session created, all calls logged with session ID, close logged with summary | must |
| IT-3 | Redaction + audit integration | Redaction engine + audit log | Redaction events appear as sub-entries of tool calls | must |
| IT-4 | TTL + observe integration | TTL configured + data older than TTL | Observe returns only fresh data | must |
| IT-5 | TTL + export integration | TTL + export_data | Export excludes TTL-expired entries | must |
| IT-6 | Profile + rate limit integration | Profile sets rate limits + tool calls exceed limit | Rate limited per profile settings | must |
| IT-7 | Read-only + all tools | Read-only mode + exercise all tools | Read tools work, write tools blocked | must |
| IT-8 | Tool allowlist + tools/list | Allowlist configured + list tools | Only allowed tools visible | must |
| IT-9 | Health metrics + active session | Running session + configure health | Accurate real-time metrics | must |
| IT-10 | Export + redaction + TTL combined | All three active + export_data | Exported data is TTL-filtered and redacted | must |
| IT-11 | Config file + env + CLI priority | Values set in all three | Correct priority applied | must |
| IT-12 | Auth attempt audit + get_audit_log | API key auth + audit query | Auth events queryable with correct metadata | must |
| IT-13 | Project isolation buffers | Two projects + different data | Queries in each project return only their data | should |
| IT-14 | Project memory sharing | Two projects near memory limit | Largest-first eviction triggers | should |
| IT-15 | Project minimum guarantee | Create project that would drop below 2MB minimum | Creation fails with clear error | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Audit logging overhead per tool call | Latency added | < 0.1ms | must |
| PT-2 | Audit ring buffer at capacity | Insert/query at 10000 entries | No degradation | must |
| PT-3 | TTL check overhead on buffer read | Latency added | < 0.05ms per entry | must |
| PT-4 | Rate limit check overhead | Latency per check | < 0.01ms | must |
| PT-5 | Health metrics computation | Latency | < 10ms | must |
| PT-6 | Export serialization: 10000 audit entries | Latency | < 500ms | should |
| PT-7 | Export serialization: full capture buffers | Latency | < 2s | should |
| PT-8 | Total overhead with all Tier 1-3 features | Per-tool-call latency added | < 3ms | must |
| PT-9 | Config file parsing at startup | Time | < 50ms | should |
| PT-10 | Session ID generation | Time per ID | < 0.1ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Audit buffer exactly at capacity | Insert entry at exactly 10000 | Entry added, oldest evicted | must |
| EC-2 | `get_audit_log` with no entries | Query before any tool calls | Empty array, not error | must |
| EC-3 | TTL of exactly 1 minute (minimum) | `--ttl=1m` | Accepted, entries expire at 60s | must |
| EC-4 | TTL below minimum | `--ttl=30s` | Error or clamp to 1 minute | must |
| EC-5 | Profile with all overrides | `--profile=paranoid` + override every value | All overrides applied | must |
| EC-6 | Export during active data ingestion | Export while extension pushes data | Consistent snapshot, no partial entries | must |
| EC-7 | Read-only mode: tool tries to clear via different parameter name | Creative mutation attempt | Still blocked | must |
| EC-8 | Rate limit at exact boundary | Exactly 20 calls in 60s for 20/min limit | 20th succeeds, 21st fails | must |
| EC-9 | Rate limit across minute boundary | 19 calls, wait until window resets, 20 more | All succeed (separate windows) | must |
| EC-10 | Health metrics on fresh server | Query immediately after start | All zeros/defaults, no errors | must |
| EC-11 | Multiple concurrent audit log queries | 10 parallel `get_audit_log` calls | All return consistent data | must |
| EC-12 | Config file with unknown keys | JSON with extra unrecognized fields | Ignored gracefully, no error | should |
| EC-13 | Config file missing | `--config=/nonexistent` | Clear error on startup | must |
| EC-14 | Session disconnect detection | Kill MCP client process | Session closed, summary logged | must |
| EC-15 | Stdio EOF handling | Pipe closed unexpectedly | Graceful shutdown, no panic | must |
| EC-16 | Project deletion during active query | Delete project while tool call in progress | Query completes or returns clear error | should |
| EC-17 | 10 projects at memory limit | Max projects, all near limit | Largest-first eviction works correctly | should |
| EC-18 | Empty profile name | `--profile=` | Error: "unknown profile" | must |
| EC-19 | Invalid profile name | `--profile=nonexistent` | Clear error: "unknown profile: nonexistent" | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890 --ttl=30m`
- [ ] Chrome extension installed and connected
- [ ] A web application available for browsing

### Tier 1: Audit Trail UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | AI calls: `{"tool": "observe", "params": {"category": "logs"}}` | MCP response | Normal observe response | [ ] |
| UAT-2 | AI queries audit log: `{"tool": "observe", "params": {"category": "audit_log"}}` | MCP response | Entry for the UAT-1 observe call with timestamp, tool_name, duration_ms, response_size_bytes, status | [ ] |
| UAT-3 | Verify audit entry has no request/response body | Inspect all fields of audit entry | No `request_body`, `response_body`, or `params` fields | [ ] |
| UAT-4 | AI calls multiple tools: observe, generate, configure | MCP responses | Each produces an audit entry | [ ] |
| UAT-5 | AI queries audit with tool filter: `{"tool": "observe", "params": {"category": "audit_log", "filter_tool": "observe"}}` | MCP response | Only observe entries returned | [ ] |
| UAT-6 | Verify session ID consistency | Compare session_id across all audit entries | Same session ID on all entries | [ ] |
| UAT-7 | Verify client identity | Check audit entry or health for client info | Client name matches AI tool being used (e.g., "claude-code") | [ ] |

### Tier 2: Data Governance UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-8 | Human browses web app to generate logs | Browser activity | Console logs captured | [ ] |
| UAT-9 | AI calls observe for recent logs | MCP response | Logs visible (within TTL window) | [ ] |
| UAT-10 | Wait for TTL to expire (or restart server with `--ttl=2m` and wait 2+ minutes) | Timer | TTL window elapsed | [ ] |
| UAT-11 | AI calls observe again | MCP response | Old entries no longer visible (TTL expired) | [ ] |
| UAT-12 | AI calls export: `{"tool": "observe", "params": {"category": "export", "scope": "audit"}}` | MCP response | JSON Lines output with audit entries | [ ] |
| UAT-13 | AI calls export: `{"tool": "observe", "params": {"category": "export", "scope": "captures"}}` | MCP response | JSON Lines with captured data, redaction applied | [ ] |
| UAT-14 | Verify export respects TTL | Compare export to direct observe | Same TTL filtering applied | [ ] |

### Tier 3: Operational Safety UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-15 | Restart server with `--profile=restricted --api-key=test123` | Server startup | Server starts with restricted profile | [ ] |
| UAT-16 | AI checks health: `{"tool": "observe", "params": {"category": "health"}}` | MCP response | Profile: "restricted", TTL, rate limits, redaction status shown | [ ] |
| UAT-17 | AI attempts mutation: `{"tool": "configure", "params": {"action": "clear"}}` | MCP response | Error: "read-only mode is active" | [ ] |
| UAT-18 | AI calls observe (read operation) | MCP response | Works normally | [ ] |
| UAT-19 | AI calls observe rapidly (exceed rate limit) | MCP responses | After N calls, rate limit error with retry_after_seconds | [ ] |
| UAT-20 | Wait for rate limit window to reset | Timer | Window resets | [ ] |
| UAT-21 | AI calls observe again | MCP response | Works normally again | [ ] |

### Tool Allowlist UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-22 | Restart server with `--tools-allow="observe"` | Server startup | Only observe tool available | [ ] |
| UAT-23 | AI requests tools list | MCP response | Only `observe` appears | [ ] |
| UAT-24 | AI attempts: `{"tool": "generate", "params": {...}}` | MCP response | Error: method not found (-32601) | [ ] |
| UAT-25 | AI calls observe | MCP response | Works normally | [ ] |

### Profile Verification UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-26 | Start server with `--profile=short-lived` | Server log | TTL=15min confirmed in startup | [ ] |
| UAT-27 | AI checks health | MCP response | `profile: "short-lived"`, TTL=15m, specific redaction patterns listed | [ ] |
| UAT-28 | Start server with `--profile=paranoid` | Server log | All restrictions active | [ ] |
| UAT-29 | AI checks health | MCP response | TTL=5min, read-only, limited tool list, strict rate limits | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Audit entries have no request/response bodies | AI queries audit log, inspect all fields | Only metadata fields present | [ ] |
| DL-UAT-2 | Redaction audit has no sensitive content | Trigger redaction match, query redaction audit entries | Pattern name + char count only | [ ] |
| DL-UAT-3 | Export applies redaction | Call export with captures scope when redaction active | Exported data has redaction applied | [ ] |
| DL-UAT-4 | Health metrics have no buffer contents | Call `configure({action:"health"})` | Sizes and counts only, no data excerpts | [ ] |
| DL-UAT-5 | Hidden tool does not confirm existence | Call blocked tool | Generic "method not found", no "tool blocked" message | [ ] |
| DL-UAT-6 | API key not in health output | Call `configure({action:"health"})` with auth enabled | `auth_enabled: true`, no key value | [ ] |
| DL-UAT-7 | TTL prevents access to old data | Wait for TTL + observe | No old entries returned | [ ] |

### Regression Checks
- [ ] Default server (no enterprise flags) behaves identically to pre-feature behavior
- [ ] Audit trail has zero overhead when no audit queries are made (passive logging only)
- [ ] Existing MCP tools (`observe`, `generate`, `configure`, `interact`) are unaffected
- [ ] Extension works unchanged with all enterprise features enabled
- [ ] No new MCP tools violate the 5-tool maximum (new modes added under existing tools)
- [ ] Config file format is JSON only (no TOML/YAML dependencies added)
- [ ] All implementation uses Go stdlib only (zero external dependencies)
- [ ] Performance within SLOs: < 3ms total overhead per tool call with all features active

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

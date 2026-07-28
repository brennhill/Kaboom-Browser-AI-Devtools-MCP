---
status: proposed
scope: feature/ttl-retention/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-ttl-retention
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: TTL Retention

> QA plan for the TTL-Based Retention feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. TTL filtering is read-time only -- entries exist in memory past their TTL until ring buffer rotation removes them. The primary risk is that TTL-expired data remains accessible through alternate paths, or that TTL configuration reveals application behavior.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | TTL-expired entries still in raw memory | Entries past TTL are excluded from responses but remain in ring buffers until overwritten -- verify no MCP tool bypasses TTL filtering to expose expired data | critical |
| DL-2 | TTL stats reveal filtered entry count | `filtered_by_ttl` count could hint at traffic volume during periods the agent was not observing -- verify this is acceptable for localhost use | medium |
| DL-3 | Oldest/newest entry timestamps in stats | `oldest_entry` and `newest_entry` timestamps could reveal when application activity occurred -- verify no sensitive timing information is exposed | low |
| DL-4 | TTL config reveals retention strategy | An agent reading the TTL config could infer what data the user considers sensitive (short TTL on network = sensitive API responses) -- acceptable for localhost | low |
| DL-5 | TTL pressure multiplier shortens retention unexpectedly | Under memory pressure with `--ttl-pressure-aware`, TTL could be halved or quartered, causing agents to miss recent data they expected to see | high |
| DL-6 | TTL=0 (unlimited) retains data indefinitely | With unlimited TTL, sensitive data captured early in a session persists until ring buffer rotation or memory eviction -- verify this is clearly communicated | medium |
| DL-7 | Per-buffer TTL creates inconsistent data windows | Network TTL=5m but console TTL=30m means an agent sees errors but not the network requests that caused them -- verify response helps agents understand the mismatch | medium |
| DL-8 | TTL config change does not retroactively redact | Setting a shorter TTL does not delete entries from buffers; it only hides them at read time -- a subsequent TTL increase would re-expose previously hidden entries | high |

### Negative Tests (must NOT leak)
- [ ] No MCP tool returns entries older than the effective TTL for that buffer
- [ ] TTL-expired entries are not included in `get_changes_since` diffs
- [ ] Health endpoint shows TTL configuration but not the content of expired entries
- [ ] Changing TTL from short to long does not expose previously-filtered entries in a security-sensitive way (document this behavior)
- [ ] TTL stats show counts only, never entry content

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | TTL duration format is human-readable | Values shown as "5m", "1h", "15m" not raw milliseconds or seconds | [ ] |
| CL-2 | "unlimited" is explicit | When TTL=0, the response says "unlimited" not "0" or "0s" | [ ] |
| CL-3 | Effective TTL resolution is clear | Response shows both the configured TTL and the resolved TTL (e.g., "15m (global)" vs. "5m (buffer-specific)") | [ ] |
| CL-4 | Per-buffer vs. global precedence | AI understands buffer-specific TTL overrides global, and both 0 means unlimited | [ ] |
| CL-5 | "Filtered by TTL" semantics | AI understands these entries still exist in memory but are excluded from responses | [ ] |
| CL-6 | Minimum TTL enforcement | AI understands values below 1 minute are rejected, receives clear error message | [ ] |
| CL-7 | Pressure-aware TTL explanation | If enabled, response explains that effective TTL is currently reduced due to memory pressure | [ ] |
| CL-8 | TTL preset names are descriptive | Presets "debug", "ci", "monitor", "unlimited" clearly convey their intended use case | [ ] |
| CL-9 | Stats `entries_in_ttl` vs `total_entries` | AI understands `total_entries` is the ring buffer count, `entries_in_ttl` is the number the agent will actually see | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might think TTL=0 means "delete immediately" instead of "unlimited retention" -- verify error messages and documentation clarify
- [ ] AI might expect TTL to physically delete entries -- verify response explains TTL is read-time filtering, entries remain until ring buffer rotation
- [ ] AI might confuse buffer-specific TTL with global TTL -- verify the response always shows which is in effect and why
- [ ] AI might think setting a shorter TTL will immediately delete data -- verify the response explains the filtering-only behavior
- [ ] AI might not understand why an entry it just saw disappeared -- verify TTL expiration is mentioned when entries age out between consecutive reads

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Set global TTL | 1 step: `configure(what: "ttl", ttl_action: "set", ttl_config: {global: "15m"})` | No -- already minimal |
| Set per-buffer TTL | 1 step: same call with buffer-specific fields | No -- already minimal |
| Check current TTL config | 1 step: `configure(what: "ttl", ttl_action: "get")` | No -- already minimal |
| Reset to defaults | 1 step: `configure(what: "ttl", ttl_action: "reset")` | No -- already minimal |
| Apply a TTL preset | 1 step: `configure(what: "ttl", ttl_action: "set", preset: "debug")` | No -- already minimal |
| Configure at startup | 1 step: CLI flag `--ttl 15m` | No -- already minimal |

### Default Behavior Verification
- [ ] Feature works with zero configuration -- default TTL is unlimited (no filtering)
- [ ] No MCP tool call is required to start using TTL; it can be set via CLI flags
- [ ] Setting a TTL does not require restarting the server
- [ ] Removing TTL (reset) takes effect immediately

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Parse valid duration "15m" | `parseTTLDuration("15m")` | 15 * time.Minute | must |
| UT-2 | Parse valid duration "1h" | `parseTTLDuration("1h")` | time.Hour | must |
| UT-3 | Parse valid duration "2h30m" | `parseTTLDuration("2h30m")` | 2.5 * time.Hour | must |
| UT-4 | Parse invalid duration "abc" | `parseTTLDuration("abc")` | Error returned | must |
| UT-5 | Parse zero duration "" (unlimited) | `parseTTLDuration("")` | 0 (unlimited) | must |
| UT-6 | Enforce minimum TTL (reject < 1m) | `parseTTLDuration("30s")` | Error: minimum TTL is 1 minute | must |
| UT-7 | Enforce maximum TTL (reject > 24h) | `parseTTLDuration("25h")` | Error: maximum TTL is 24 hours | must |
| UT-8 | EffectiveTTL with buffer-specific set | `TTLConfig{Global: 30m, Network: 5m}`, query "network" | 5m (buffer-specific) | must |
| UT-9 | EffectiveTTL with only global set | `TTLConfig{Global: 30m}`, query "network" | 30m (global) | must |
| UT-10 | EffectiveTTL with both zero | `TTLConfig{}`, query "network" | 0 (unlimited) | must |
| UT-11 | Read-time filter excludes old entries | 10 WS events, 5 older than TTL | `GetWebSocketEvents()` returns 5 entries | must |
| UT-12 | Read-time filter includes entries within TTL | 10 entries all within TTL | All 10 returned | must |
| UT-13 | Boundary: entry exactly at TTL age | Entry timestamp = now - TTL | Entry excluded (boundary is exclusive) | must |
| UT-14 | TTL=0 returns all entries | Unlimited TTL, mix of old and new entries | All entries returned regardless of age | must |
| UT-15 | Per-buffer independence | Console TTL=5m, Network TTL=30m | Console filters at 5m, Network filters at 30m independently | must |
| UT-16 | Changing one buffer TTL does not affect others | Set network TTL, check console TTL | Console TTL unchanged | must |
| UT-17 | SetTTLConfig thread safety | Concurrent SetTTLConfig and GetTTLConfig | No race condition (mutex protects access) | must |
| UT-18 | TTL stats calculation | Buffer with 100 entries, 30 past TTL | `TTLStats.EntriesInTTL=70, FilteredByTTL=30` | should |
| UT-19 | TTL stats with empty buffer | Empty buffer | `TTLStats.TotalEntries=0, EntriesInTTL=0` | should |
| UT-20 | Pressure-aware TTL at normal pressure | Base TTL=10m, pressure=normal | Effective TTL=10m | should |
| UT-21 | Pressure-aware TTL at soft pressure | Base TTL=10m, pressure=soft, feature enabled | Effective TTL=5m (halved) | should |
| UT-22 | Pressure-aware TTL at critical pressure | Base TTL=10m, pressure=critical, feature enabled | Effective TTL=2.5m (quartered) | should |
| UT-23 | Pressure-aware with unlimited TTL | TTL=0, any pressure level | Effective TTL=0 (unlimited stays unlimited) | must |
| UT-24 | TTL preset "debug" applies correct values | Apply preset "debug" | Global=15m, Network=5m, WebSocket=30m, Actions=15m | should |
| UT-25 | TTL preset "ci" applies correct values | Apply preset "ci" | Global=5m, Network=2m, WebSocket=5m, Actions=5m | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | MCP configure tool with action "ttl", ttl_action "get" | MCP dispatcher + Capture.GetTTLConfig() | Returns current TTL config and stats | must |
| IT-2 | MCP configure tool with action "ttl", ttl_action "set" | MCP dispatcher + Capture.SetTTLConfig() | Updates TTL config, subsequent reads use new TTL | must |
| IT-3 | MCP configure tool with action "ttl", ttl_action "reset" | MCP dispatcher + Capture.SetTTLConfig() | All TTL values reset to 0 (unlimited) | must |
| IT-4 | CLI flag --ttl sets global TTL | main.go flag parsing + Capture init | Server starts with global TTL set | must |
| IT-5 | CLI per-buffer flags override global | main.go flag parsing + TTLConfig resolution | Buffer-specific flags take precedence | must |
| IT-6 | Health endpoint includes TTL section | HTTP /v4/health handler + TTLConfig | Response JSON includes `ttl.config` and `ttl.effective` | must |
| IT-7 | observe tool respects TTL | observe handler + read-time filtering | observe returns only entries within TTL | must |
| IT-8 | get_changes_since respects TTL | diff handler + read-time filtering | Diff results exclude TTL-expired entries | must |
| IT-9 | Environment variable KABOOM_TTL works | env var + main.go | Server starts with TTL from env var | should |
| IT-10 | TTL change during active reads | Concurrent TTL update + buffer reads | Read uses consistent TTL for entire query (RLock) | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | TTL filtering 1000 entries | Time to filter | < 1ms | must |
| PT-2 | TTL config read | Latency | < 0.01ms | must |
| PT-3 | TTL config write | Latency | < 0.1ms | should |
| PT-4 | TTL stats calculation on 1000-entry buffer | Computation time | < 1ms | should |
| PT-5 | Config file read on startup | Parse time for TTL flags | < 5ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Empty buffer with TTL set | Query buffer with TTL=5m, buffer empty | Returns empty array, no crash | must |
| EC-2 | Ring buffer wrap with TTL | Buffer at capacity, oldest entries TTL-expired, new entries added | Expired entries not returned; ring rotation also removes them | must |
| EC-3 | TTL change during read | One goroutine reads, another sets TTL | Read uses consistent TTL for entire query (RLock held) | must |
| EC-4 | All entries expired | 100 entries all older than TTL | Returns empty array | must |
| EC-5 | TTL set to exactly 1 minute (minimum) | `parseTTLDuration("1m")` | Accepted, entries older than 1 minute filtered | should |
| EC-6 | TTL set to exactly 24 hours (maximum) | `parseTTLDuration("24h")` | Accepted | should |
| EC-7 | Negative duration in input | `parseTTLDuration("-5m")` | Error: invalid duration | must |
| EC-8 | TTL with memory pressure interaction disabled | Memory above soft limit, --ttl-pressure-aware not set | TTL unchanged, pressure does not affect filtering | should |
| EC-9 | Rapid TTL changes | Set TTL 5 times in 1 second | Each change takes effect immediately, no crash or race | should |
| EC-10 | TTL reset while entries exist | Buffer has 100 entries, set TTL from 5m to unlimited | All 100 entries now visible (including previously filtered) | must |
| EC-11 | Console buffer timestamps vs. server time | Console entries captured with slight clock skew | TTL filtering uses server-side addedAt timestamps, not entry timestamps | should |
| EC-12 | TTL config survives within session but not across restarts | Set TTL, query mid-session, restart server | Mid-session: TTL active. After restart: TTL reset to defaults (or CLI flags) | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application running locally that generates continuous activity (console logs, network requests)
- [ ] Ability to wait 5+ minutes between steps to verify TTL expiration

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"what": "ttl", "ttl_action": "get"}}` | Review TTL configuration | Response shows `config` with all zeros (unlimited) and `stats` for each buffer | [ ] |
| UAT-2 | `{"tool": "configure", "arguments": {"what": "ttl", "ttl_action": "set", "ttl_config": {"global": "5m"}}}` | None | Response confirms global TTL set to "5m" | [ ] |
| UAT-3 | `{"tool": "configure", "arguments": {"what": "ttl", "ttl_action": "get"}}` | Verify effective TTLs | All buffers show effective_ttl = "5m (global)" | [ ] |
| UAT-4 | Generate browser activity for 2 minutes, then call `{"tool": "observe", "arguments": {"what": "get_console_logs"}}` | Compare to browser DevTools | Only entries from the last 5 minutes appear (all current entries, since only 2 minutes have passed) | [ ] |
| UAT-5 | Wait 6 minutes from the first activity, then call `{"tool": "observe", "arguments": {"what": "get_console_logs"}}` | Note the entry count | Entries older than 5 minutes are no longer in the response | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"what": "ttl", "ttl_action": "set", "ttl_config": {"global": "5m", "network": "2m"}}}` | None | Response confirms global=5m, network=2m | [ ] |
| UAT-7 | `{"tool": "configure", "arguments": {"what": "ttl", "ttl_action": "get"}}` | Verify per-buffer resolution | Console shows effective_ttl="5m (global)", Network shows effective_ttl="2m (buffer-specific)" | [ ] |
| UAT-8 | Wait 3 minutes, then call observe for both console and network | Compare counts | Console has more entries (5m window) than network (2m window) | [ ] |
| UAT-9 | `{"tool": "configure", "arguments": {"what": "ttl", "ttl_action": "set", "ttl_config": {"global": "30s"}}}` | None | Error: minimum TTL is 1 minute | [ ] |
| UAT-10 | `{"tool": "configure", "arguments": {"what": "ttl", "ttl_action": "reset"}}` | None | Response confirms all TTLs reset to unlimited | [ ] |
| UAT-11 | `{"tool": "observe", "arguments": {"what": "get_console_logs"}}` | Check entry count | All entries in ring buffer now visible again (unlimited TTL) | [ ] |
| UAT-12 | `{"tool": "configure", "arguments": {"what": "ttl", "ttl_action": "set", "preset": "debug"}}` | None | Response confirms debug preset applied: global=15m, network=5m, websocket=30m, actions=15m | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | TTL-expired entries not in observe response | Set short TTL, wait for entries to expire, call observe | Expired entries not returned | [ ] |
| DL-UAT-2 | TTL-expired entries not in get_changes_since | Set TTL, wait for expiry, call get_changes_since with old timestamp | Expired entries not in diff | [ ] |
| DL-UAT-3 | TTL stats show counts only | Call TTL get action | Stats contain entry counts and timestamps, no entry content | [ ] |
| DL-UAT-4 | Re-exposing entries via TTL increase | Set TTL=2m, wait 3m, then set TTL=unlimited | Previously hidden entries reappear (document this behavior as expected) | [ ] |
| DL-UAT-5 | Health endpoint shows effective TTL | GET /v4/health | TTL section shows config and effective values, no entry content | [ ] |

### Regression Checks
- [ ] Existing buffer read functionality works when TTL is unlimited (default behavior unchanged)
- [ ] Existing `--ttl` CLI flag continues to work as global TTL
- [ ] Ring buffer rotation still works correctly alongside TTL filtering
- [ ] Memory enforcement interacts correctly with TTL (both eviction strategies compose cleanly)
- [ ] No observable latency increase on observe tool responses with TTL filtering active

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

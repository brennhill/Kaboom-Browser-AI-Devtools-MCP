---
status: proposed
scope: feature/ai-capture-control/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-ai-capture-control
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: AI Capture Control

> QA plan for the AI Capture Control feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. AI Capture Control allows the AI to change what data the extension captures (e.g., enabling WebSocket message payloads, full network bodies). This elevates capture scope, which increases the risk of capturing sensitive data.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | AI escalates `ws_mode` to `messages` — captures WebSocket auth tokens | When AI sets `ws_mode: "messages"`, verify that captured WS payloads containing auth tokens are subject to the same header/body stripping rules | critical |
| DL-2 | AI enables `network_bodies: true` — response bodies contain PII | When AI enables body capture, verify bodies are stored in memory only (ring buffer), not written to the JSONL file on disk | critical |
| DL-3 | AI sets `log_level: "all"` — verbose logs expose secrets | When log_level is "all", console.log() calls that print secrets (e.g., `console.log("token:", token)`) are captured; verify they stay in ring buffer, not disk | high |
| DL-4 | Audit log leaks setting values | Audit log at `~/.kaboom/audit.jsonl` records setting changes; verify it logs "from"/"to" values (e.g., `ws_mode: lifecycle -> messages`) but NOT captured data content | high |
| DL-5 | `/settings` endpoint exposes override map externally | Verify `/settings` is localhost-only; the override map should not contain captured data, only setting names and values | medium |
| DL-6 | `observe({what: "page"})` override display leaks timing info | Override metadata in page response includes `changed_at` timestamps; verify no additional metadata (client IP, agent identity) leaks | medium |
| DL-7 | Audit log rotation leaves orphaned files with sensitive data | When audit log rotates, verify old files (`.1`, `.2`, `.3`) are properly managed and files beyond the 3-file limit are deleted | medium |
| DL-8 | Agent identity in audit log exposes MCP client internals | `agent` field from `clientInfo.name` is logged; verify only the name string is captured, not the full `initialize` request | low |
| DL-9 | AI enables `screenshot_on_error: true` — screenshots contain sensitive page content | Screenshots captured on error may show password fields, PII, or confidential data; verify they stay in memory buffer only | high |
| DL-10 | Session scoping failure — overrides persist across server restarts | Verify overrides are truly in-memory only; a server restart clears all capture overrides | medium |

### Negative Tests (must NOT leak)
- [ ] Audit log entries must NOT contain captured telemetry data (only setting change metadata)
- [ ] `/settings` response must NOT include any captured data — only setting names and override values
- [ ] Override settings must NOT be persisted to disk (session-scoped, memory-only)
- [ ] Changing capture settings must NOT retroactively apply to already-captured data
- [ ] Audit log file must NOT be accessible via any HTTP endpoint

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Setting change confirmation is explicit | Response after `configure(what: "capture")` confirms which settings changed, with old and new values | [ ] |
| CL-2 | Invalid setting name provides valid options | Error response for unknown setting includes the list of valid settings | [ ] |
| CL-3 | Invalid setting value provides valid values | Error response for bad value includes the enum of valid values for that setting | [ ] |
| CL-4 | Rate limit error is distinguishable | Rate limit error includes "Rate limited" text AND the time until next allowed change | [ ] |
| CL-5 | Reset confirmation is clear | `configure(what: "capture", settings: "reset")` returns confirmation that ALL overrides were cleared | [ ] |
| CL-6 | Override status in `observe({what: "page"})` shows current vs default | Override display shows both the current value and the default value so the AI knows what changed | [ ] |
| CL-7 | Alert piggyback on setting change | The info-level alert emitted on change is visible in the next `observe` response via `_alerts` | [ ] |
| CL-8 | Session scope is communicated | Response or documentation communicates that changes are session-scoped (reset on server restart) | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI may assume `network_bodies: true` means bodies are already available in the next `observe` call — verify there is latency (extension must poll `/settings` first, then the next request captures bodies)
- [ ] AI may call `configure(what: "capture")` with no settings object — verify error message explains that `settings` is required
- [ ] AI may confuse `settings: "reset"` (clear overrides) with setting individual values to their defaults — verify reset is an atomic "clear all" operation
- [ ] AI may not realize the 5-second poll delay between setting change and extension behavior change — verify response includes a note about propagation delay

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Change one capture setting | 1 step: `configure(what: "capture", settings: { ... })` | No — already minimal |
| Change multiple settings at once | 1 step: single call with multiple settings | No — already batched |
| Check current overrides | 1 step: `observe(what: "page")` includes override state | No — piggybacks on existing call |
| Reset all overrides | 1 step: `configure(what: "capture", settings: "reset")` | No — already minimal |
| Verify setting took effect | 2 steps: change setting, then wait ~5s and observe data | Could add immediate confirmation, but 5s poll is architectural |

### Default Behavior Verification
- [ ] All capture settings use sensible defaults (log_level: error, ws_mode: lifecycle, network_bodies: true, screenshot_on_error: false, action_replay: true)
- [ ] No configuration needed for basic operation — AI only changes settings when it needs richer data
- [ ] Extension behavior is unchanged when no overrides are active

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Store single override | `configure(what: "capture", settings: { log_level: "all" })` | Override stored in memory map; response confirms change | must |
| UT-2 | Store multiple overrides in one call | `settings: { ws_mode: "messages", log_level: "all" }` | Both stored; both confirmed; one rate limit credit consumed | must |
| UT-3 | Invalid setting name | `settings: { invalid_setting: "value" }` | Error with list of valid settings | must |
| UT-4 | Invalid setting value | `settings: { log_level: "verbose" }` | Error: "Invalid value 'verbose' for log_level. Valid: error, warn, all" | must |
| UT-5 | Reset all overrides | `settings: "reset"` | Override map cleared; response confirms reset | must |
| UT-6 | Rate limit enforcement | Two calls within 1 second | Second call returns rate limit error | must |
| UT-7 | Rate limit recovery | Call, wait 1.1 seconds, call again | Second call succeeds | must |
| UT-8 | Override survives extension reconnect | Set override, simulate extension disconnect/reconnect | Override still present in `/settings` response | should |
| UT-9 | Server restart clears overrides | Set override, restart server process | Override map is empty; `/settings` has no overrides | must |
| UT-10 | Alert emitted on change | Change `ws_mode` from lifecycle to messages | Alert buffer contains info-level alert with correct from/to values | must |
| UT-11 | Audit log entry written | Change any setting | `~/.kaboom/audit.jsonl` has new line with correct JSON structure | must |
| UT-12 | Audit log includes agent identity | Change setting from known MCP client | Audit entry `agent` field matches clientInfo.name | should |
| UT-13 | Audit log rotation at 10MB | Write entries until file exceeds 10MB | File rotates to `.1`; new file created; max 3 rotated files | should |
| UT-14 | Audit log failure is non-blocking | Set audit log path to read-only directory | Setting change still succeeds; warning logged | must |
| UT-15 | `/settings` response includes overrides | Set overrides, then GET /settings | Response includes `capture_overrides` field with active overrides | must |
| UT-16 | `observe({what: "page"})` includes overrides | Set overrides, then observe page | Response includes `capture_overrides` with value/default/changed_at | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end override propagation | Server MCP handler, /settings endpoint, extension background.js | AI sets override via MCP; extension polls /settings and receives override; extension applies new capture behavior | must |
| IT-2 | Override affects captured data | Server, extension, capture pipeline | Set `log_level: "all"` and trigger console.log(); verify log appears in `observe({what: "logs"})` when it would not with default `error` level | must |
| IT-3 | ws_mode override changes WS capture | Server, extension WS monitor | Set `ws_mode: "messages"` and trigger WS messages; verify payloads appear in `observe({what: "websocket_events"})` | must |
| IT-4 | Reset propagates to extension | Server, /settings, extension | Reset overrides; extension picks up empty overrides on next poll; capture reverts to user defaults | must |
| IT-5 | Alert appears in observe response | Server alert buffer, observe handler | After setting change, next `observe` call includes the alert about the change | should |
| IT-6 | Concurrent override changes | Two MCP clients changing settings | Last writer wins; both changes audited; no crash or corruption | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Setting change latency | Time from MCP call to override stored | < 0.1ms | must |
| PT-2 | Settings poll response time | Time to serialize /settings response with overrides | < 0.05ms | must |
| PT-3 | Audit log write latency | Time for json.Marshal + file write | < 1ms | must |
| PT-4 | Audit log rotation latency | Time for file rename chain | < 10ms | should |
| PT-5 | Override memory footprint | Memory used by 5 active overrides | < 1KB | must |
| PT-6 | Rate limit check overhead | Time to check and enforce rate limit | < 0.01ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Setting same value as current | Set `log_level: "error"` when default is already `error` | Accepted (no-op); alert shows same from/to; audit logged | should |
| EC-2 | Empty settings object | `configure(what: "capture", settings: {})` | Error: no settings specified, or no-op with empty confirmation | should |
| EC-3 | Rate limit with multiple settings | Call with 3 settings, then another with 1 setting within 1 second | First call succeeds (all 3 stored); second call rate-limited | must |
| EC-4 | Audit log directory does not exist | `KABOOM_AUDIT_LOG=/nonexistent/path/audit.jsonl` | Directory created via `os.MkdirAll`; log file created | should |
| EC-5 | Audit log disk full | Disk is full when audit write attempted | Write fails silently; capture control still works | must |
| EC-6 | Override then reset then override | Set log_level: "all", reset, set ws_mode: "messages" | After reset, only ws_mode override is active | must |
| EC-7 | Extension disconnected when override set | Set override with no extension connected | Override stored server-side; extension picks it up on reconnection | should |
| EC-8 | Boolean setting as string | `settings: { network_bodies: "true" }` | Accepted (coerced to boolean) or error with type hint | should |
| EC-9 | Very rapid toggling | Enable/disable/enable log_level 100 times in 100 seconds | Each change beyond 1/second is rate-limited; audit log has exactly the accepted entries | should |
| EC-10 | Concurrent reads during write | `/settings` GET while override map is being written | RWMutex ensures consistent read; no partial data | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web page open with console activity and network requests (e.g., a dev server)
- [ ] DevTools console open to manually trigger log messages

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"what": "capture", "settings": {"log_level": "all"}}}` | Extension popup shows "AI-controlled" indicator (if implemented) | AI receives confirmation: `log_level` changed from `error` to `all` | [ ] |
| UAT-2 | Human types `console.log("test message")` in DevTools | Log message sent to console | Message visible in DevTools console | [ ] |
| UAT-3 | Wait ~5s for extension poll, then: `{"tool": "observe", "arguments": {"what": "logs"}}` | No visual change | AI receives logs including "test message" (would NOT appear with default `error` level) | [ ] |
| UAT-4 | `{"tool": "observe", "arguments": {"what": "page"}}` | No visual change | Response includes `capture_overrides` showing `log_level: { value: "all", default: "error", changed_at: "..." }` | [ ] |
| UAT-5 | `{"tool": "configure", "arguments": {"what": "capture", "settings": {"ws_mode": "messages"}}}` | No visual change | AI receives confirmation: `ws_mode` changed from `lifecycle` to `messages` | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"what": "capture", "settings": {"invalid_setting": "value"}}}` | No visual change | AI receives error listing valid settings | [ ] |
| UAT-7 | `{"tool": "configure", "arguments": {"what": "capture", "settings": {"log_level": "verbose"}}}` | No visual change | AI receives error listing valid values for log_level | [ ] |
| UAT-8 | Call configure twice within 1 second: first `{"settings": {"network_bodies": false}}` then `{"settings": {"action_replay": false}}` | No visual change | First call succeeds; second call returns rate limit error | [ ] |
| UAT-9 | Wait 2 seconds, then: `{"tool": "configure", "arguments": {"what": "capture", "settings": "reset"}}` | Extension popup "AI-controlled" indicator disappears | AI receives confirmation that all overrides cleared | [ ] |
| UAT-10 | `{"tool": "observe", "arguments": {"what": "page"}}` | No visual change | Response shows no `capture_overrides` (or empty overrides) | [ ] |
| UAT-11 | Human checks audit log file: `cat ~/.kaboom/audit.jsonl` | File contains JSONL entries | Each setting change from above steps has a corresponding audit entry with ts, event, setting, from, to, source, agent fields | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Audit log contains no captured data | `cat ~/.kaboom/audit.jsonl \| jq .` | Each line has only metadata (ts, event, setting, from, to, source, agent) — no log content, network bodies, or WS payloads | [ ] |
| DL-UAT-2 | `/settings` endpoint is localhost-only | `curl http://<external-ip>:7890/settings` from another machine | Connection refused (server binds to 127.0.0.1) | [ ] |
| DL-UAT-3 | Overrides cleared after server restart | Stop and restart the Kaboom server | `observe({what: "page"})` shows no `capture_overrides` | [ ] |

### Regression Checks
- [ ] Extension capture behavior unchanged when no overrides are active
- [ ] User settings in extension popup still editable and respected when no AI overrides
- [ ] `observe` tool responses unchanged in format when overrides are inactive
- [ ] Server startup and shutdown unaffected by audit log configuration

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

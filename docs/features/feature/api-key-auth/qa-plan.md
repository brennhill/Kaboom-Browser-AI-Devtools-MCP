---
status: proposed
scope: feature/api-key-auth/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-api-key-auth
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: API Key Authentication

> QA plan for the API Key Authentication feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to API keys appearing in logs, error responses, or MCP tool outputs.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | API key echoed in error responses | 401 response must NOT include the provided (invalid) key | critical |
| DL-2 | API key logged in plaintext to audit trail | Audit entries must log success/failure + header used, never the key value | critical |
| DL-3 | API key exposed in MCP tool responses | `get_audit_log` results must NOT contain API key values in any field | critical |
| DL-4 | API key visible in health endpoint | `/health` and `configure({action:"health"})` must not expose configured keys | high |
| DL-5 | API key leaked in server startup logs | Server log output on startup must not print the configured key(s) | high |
| DL-6 | Key file contents exposed via error messages | If `--api-key-file` path is invalid, error must not dump file contents | medium |
| DL-7 | Extension stores key in accessible location | Chrome extension must store key in `chrome.storage.local` only, not in plaintext DOM or logs | high |
| DL-8 | Key transmitted over non-localhost HTTP | Extension must only send API key to configured server URL (default localhost) | critical |
| DL-9 | Key appears in `configure` tool output | When querying server configuration via MCP, key values must be masked or absent | high |
| DL-10 | Bearer token in Authorization header captured by network observer | Kaboom's own auth header from extension requests should be stripped from captured network data | high |

### Negative Tests (must NOT leak)
- [ ] 401 error response body does not contain the submitted key value
- [ ] Audit log entries for `auth_attempt` events contain no `key` or `secret` field
- [ ] `get_audit_log` with `type: "auth_attempt"` returns entries without key material
- [ ] Server startup output with `--api-key=secret123` does not print `secret123`
- [ ] Extension options page masks the key field (password input type)
- [ ] Network body captures of requests TO the Kaboom server strip the `X-API-Key` and `Authorization` headers
- [ ] `configure({action:"health"})` tool response contains no key values, only a boolean `auth_enabled` indicator

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand authentication status and errors without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | 401 error message is actionable | Response includes `hint` field explaining which headers to use | [ ] |
| CL-2 | Auth attempt audit entries distinguish failure reasons | `reason` field uses clear enums: `no_key_provided`, `invalid_key`, `malformed_header` | [ ] |
| CL-3 | `header_used` field is self-documenting | Values are literal header names (`X-API-Key`, `Authorization`), not codes | [ ] |
| CL-4 | Success vs failure is unambiguous | `success` boolean field, not a status string that could be misread | [ ] |
| CL-5 | Localhost exemption is observable | When exempted, audit log clearly shows exemption reason rather than appearing as unauthenticated | [ ] |
| CL-6 | Auth disabled state is clear | When no keys configured, health/status clearly indicates auth is not active, not "auth failed" | [ ] |
| CL-7 | Multi-key rotation state is observable | Health or audit data indicates how many valid keys are configured (count, not values) | [ ] |
| CL-8 | WWW-Authenticate header present on 401 | Standard `Bearer realm="kaboom"` header helps LLM understand it is a Bearer auth scheme | [ ] |

### Common LLM Misinterpretation Risks
- [ ] Risk: LLM interprets "no auth configured" as "auth failed" -- verify health endpoint distinguishes these states
- [ ] Risk: LLM tries to extract key from 401 error response -- verify no key material in error body
- [ ] Risk: LLM confuses `X-API-Key` and `Authorization: Bearer` priority -- verify documentation in tool descriptions
- [ ] Risk: LLM misreads `header_used: ""` (empty) as "all headers" rather than "no header provided" -- verify empty string semantics are documented
- [ ] Risk: LLM assumes localhost exemption means "no auth needed anywhere" -- verify exemption is clearly scoped to loopback addresses only

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Enable auth (CLI flag) | 1 step: `--api-key=<key>` | No -- already minimal |
| Enable auth (env var) | 1 step: `export KABOOM_API_KEY=<key>` | No -- already minimal |
| Enable auth (key file) | 2 steps: create file + `--api-key-file=<path>` | No -- file creation is inherent |
| Configure extension | 2 steps: open options, paste key | No -- standard UX |
| Key rotation | 4 steps: generate new key, add to server, update clients, remove old key | Could add `--api-key-rotate` CLI but manual is clearer |
| Query auth events | 1 MCP call: `get_audit_log` with type filter | No -- already minimal |
| Verify auth status | 1 MCP call: `configure({action:"health"})` | No -- already minimal |

### Default Behavior Verification
- [ ] Feature works with zero configuration: auth disabled by default, all requests pass through
- [ ] Adding `--api-key` is the ONLY step needed to enable auth (no additional config required)
- [ ] Extension works without key when server has no auth configured
- [ ] Localhost exemption requires explicit opt-in (`--api-key-localhost-exempt`)
- [ ] Key file supports comments (`#`) and blank lines for readability

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | `validateKey` with correct single key | `AuthConfig{Keys: {"secret": {}}}`, key="secret" | `true` | must |
| UT-2 | `validateKey` with wrong key | `AuthConfig{Keys: {"secret": {}}}`, key="wrong" | `false` | must |
| UT-3 | `validateKey` with empty key | `AuthConfig{Keys: {"secret": {}}}`, key="" | `false` | must |
| UT-4 | `validateKey` with close-but-wrong key | `AuthConfig{Keys: {"secret123": {}}}`, key="secret1234" | `false` | must |
| UT-5 | `validateKey` with multiple keys | `AuthConfig{Keys: {"k1": {}, "k2": {}, "k3": {}}}`, key="k2" | `true` | must |
| UT-6 | `validateKey` constant-time smoke test | 3 keys, time 1000 validations of first vs last | Difference < 10% | should |
| UT-7 | `extractAPIKey` with X-API-Key | Request with `X-API-Key: mykey` | `("mykey", "X-API-Key")` | must |
| UT-8 | `extractAPIKey` with Bearer | Request with `Authorization: Bearer mykey` | `("mykey", "Authorization")` | must |
| UT-9 | `extractAPIKey` with both headers (X-API-Key wins) | Both headers set | X-API-Key value returned | must |
| UT-10 | `extractAPIKey` with Basic auth (not Bearer) | `Authorization: Basic dXNlcjpwYXNz` | `("", "")` | must |
| UT-11 | `extractAPIKey` with no headers | Empty request | `("", "")` | must |
| UT-12 | `extractAPIKey` with deprecated X-Kaboom-Key | `X-Kaboom-Key: oldkey` | `("oldkey", "X-Kaboom-Key")` | should |
| UT-13 | `isLocalhost` with 127.0.0.1:port | "127.0.0.1:54321" | `true` | must |
| UT-14 | `isLocalhost` with ::1:port | "[::1]:54321" | `true` | must |
| UT-15 | `isLocalhost` with "localhost" | "localhost:54321" | `true` | must |
| UT-16 | `isLocalhost` with remote IP | "192.168.1.1:54321" | `false` | must |
| UT-17 | `isLocalhost` with IPv6 mapped IPv4 | "[::ffff:192.168.1.1]:54321" | `false` | must |
| UT-18 | `respondUnauthorized` response format | Call function | JSON with `error`, `message`, `hint` fields; 401 status; `WWW-Authenticate` header | must |
| UT-19 | Key file parsing with comments and blanks | File with `#comment\n\nkey1\nkey2` | Keys: {"key1", "key2"} | must |
| UT-20 | Key file parsing with trailing whitespace | File with `key1 \n key2\t` | Keys trimmed: {"key1", "key2"} | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Middleware disabled (no keys) | `APIKeyMiddleware` + handler | Request passes through, returns 200 | must |
| IT-2 | Middleware with valid key | `APIKeyMiddleware` + handler | Request authenticated, returns 200 | must |
| IT-3 | Middleware with invalid key | `APIKeyMiddleware` + handler | Returns 401 with JSON error body | must |
| IT-4 | Middleware with missing key | `APIKeyMiddleware` + handler | Returns 401 with JSON error body | must |
| IT-5 | Middleware with localhost exemption | `APIKeyMiddleware` + handler | Localhost request passes without key | must |
| IT-6 | Middleware without localhost exemption | `APIKeyMiddleware` + handler | Localhost request requires key | must |
| IT-7 | Audit logging on success | `APIKeyMiddleware` + `AuditLog` | Audit entry with `success: true`, correct `header_used` | must |
| IT-8 | Audit logging on failure | `APIKeyMiddleware` + `AuditLog` | Audit entry with `success: false`, correct `reason` | must |
| IT-9 | Audit logging records no key material | `APIKeyMiddleware` + `AuditLog` | No field in audit entry contains key value | must |
| IT-10 | Key merging from CLI + env + file | Config parser | All sources merged into single key set | should |
| IT-11 | CLI flags override env vars | Config with both CLI and env | CLI flag key used; env key also valid | should |
| IT-12 | Extension sends X-API-Key header | Extension background.js + server | Server authenticates, returns 200 | must |
| IT-13 | Extension handles 401 gracefully | Extension background.js + server | Extension enters backoff, records auth error | must |
| IT-14 | MCP stdio bypasses auth | MCP connection over stdio | No auth check, tools work normally | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Auth middleware overhead per request | Latency added | < 0.1ms | must |
| PT-2 | Constant-time key comparison (single key) | Time variance across 10000 checks | < 10% variance | should |
| PT-3 | Constant-time key comparison (10 keys) | Time variance across 10000 checks | < 10% variance | should |
| PT-4 | 1000 requests/sec with auth enabled | Throughput | No degradation vs no-auth | should |
| PT-5 | Audit logging overhead | Time per audit entry write | < 0.05ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Very long API key (10KB) | `X-API-Key: <10KB string>` | Rejected gracefully, no OOM | must |
| EC-2 | Unicode in API key | `X-API-Key: key-with-emoji` | Compared correctly via constant-time compare | should |
| EC-3 | Empty Authorization header | `Authorization: ` | Treated as "no key provided" | must |
| EC-4 | Bearer with extra spaces | `Authorization:  Bearer  key` | Handle gracefully (trim or reject) | should |
| EC-5 | Multiple X-API-Key headers | Two `X-API-Key` headers in same request | First header value used | should |
| EC-6 | Key file does not exist | `--api-key-file=/nonexistent` | Clear error on startup, server refuses to start | must |
| EC-7 | Key file with only comments/blanks | `--api-key-file=empty.txt` | No keys loaded, auth effectively disabled | should |
| EC-8 | Null bytes in key | Key containing `\x00` | Handled safely, no truncation | should |
| EC-9 | Concurrent requests with different keys | 100 parallel requests, mixed valid/invalid | Each correctly authenticated independently | must |
| EC-10 | Key file with BOM | UTF-8 BOM at start of file | BOM stripped, keys parsed correctly | could |
| EC-11 | Server restart with new key during extension backoff | Extension in backoff, server restarts with rotated key | Extension eventually retries, gets 401, user updates key | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890 --api-key=test-secret-123`
- [ ] Chrome extension installed and connected
- [ ] Extension API key NOT yet configured (to test rejection first)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Open a webpage in Chrome and trigger console logs | Extension status icon | Extension shows connection error (401) since no key configured | [ ] |
| UAT-2 | AI calls `observe` tool: `{"tool": "observe", "params": {"category": "logs"}}` | MCP response | Tool works (MCP over stdio bypasses auth) but returns no data (extension cannot push) | [ ] |
| UAT-3 | Human configures extension with API key `test-secret-123` in options page | Extension options page | Key field accepts input, shows masked value | [ ] |
| UAT-4 | Reload webpage to trigger console logs | Extension status icon | Extension status shows connected (green) | [ ] |
| UAT-5 | AI calls `observe` tool: `{"tool": "observe", "params": {"category": "logs"}}` | MCP response | Console logs now visible in response | [ ] |
| UAT-6 | AI calls audit log: `{"tool": "observe", "params": {"category": "audit_log", "type": "auth_attempt"}}` | MCP response | Shows successful auth entries with `header_used: "X-API-Key"` | [ ] |
| UAT-7 | Human changes extension key to `wrong-key-456` | Extension options | Key field updated | [ ] |
| UAT-8 | Reload webpage to trigger console logs | Extension status | Extension shows connection error (401) | [ ] |
| UAT-9 | AI calls audit log again with failure filter | MCP response | Shows failed auth entries with `reason: "invalid_key"` | [ ] |
| UAT-10 | Verify no key material in audit entries | Inspect all audit response fields | No field contains `test-secret-123` or `wrong-key-456` | [ ] |
| UAT-11 | Human restores correct key in extension | Extension options | Key field restored | [ ] |
| UAT-12 | Verify data flow resumes | Extension status + MCP observe | Logs flowing again, no data loss during auth error window | [ ] |

### Localhost Exemption Test

| # | Step | Human Observes | Expected Result | Pass |
|---|------|----------------|-----------------|------|
| UAT-13 | Restart server with `--api-key=secret --api-key-localhost-exempt` | Server startup | Server starts successfully | [ ] |
| UAT-14 | Remove API key from extension settings | Extension options | Key field cleared | [ ] |
| UAT-15 | Reload webpage and trigger logs | Extension + MCP | Logs flow because extension connects from localhost (exempted) | [ ] |
| UAT-16 | AI calls `observe` to confirm data received | MCP response | Console logs present | [ ] |

### Key Rotation Test

| # | Step | Human Observes | Expected Result | Pass |
|---|------|----------------|-----------------|------|
| UAT-17 | Start server with `--api-key=old-key --api-key=new-key` | Server startup | Accepts both keys | [ ] |
| UAT-18 | Configure extension with `old-key` | Extension options | Extension connected | [ ] |
| UAT-19 | Update extension to `new-key` | Extension options | Extension still connected (no interruption) | [ ] |
| UAT-20 | Restart server with only `--api-key=new-key` | Server restart | Extension with `new-key` still works; `old-key` would be rejected | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | API key not in 401 response | `curl -H "X-API-Key: wrong" http://localhost:7890/logs` | Response body contains `unauthorized` error, NOT `wrong` | [ ] |
| DL-UAT-2 | API key not in audit log | AI calls `observe` with audit_log filter for auth_attempt | No `key`, `secret`, or `token_value` fields in entries | [ ] |
| DL-UAT-3 | API key not in server stdout | Review terminal output where server is running | No key values printed at startup or during operation | [ ] |
| DL-UAT-4 | Extension masks key in options | Inspect extension options page DOM | Input type is `password`, no plaintext key visible | [ ] |

### Regression Checks
- [ ] Existing functionality works when auth is NOT configured (server started without `--api-key`)
- [ ] Extension without API key setting works against server without auth
- [ ] MCP tools all function normally when auth is enabled (stdio bypasses auth)
- [ ] `X-Kaboom-Key` header still works for backward compatibility
- [ ] Network body capture does not include Kaboom's own auth headers in captured data

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

---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Comprehensive UAT Test Plan

> Goal: If this script passes, ship with 100% confidence.

## Design Principles

1. **Test the wire, not the internals** — Every assertion validates what MCP clients actually receive
2. **Fail-first verification** — Each test must be able to fail. If removing the feature wouldn't fail the test, the test is worthless
3. **No mocks** — Tests run against the real `kaboom-agentic-browser` binary from PATH (the npm-installed version)
4. **Deterministic** — No timing-dependent assertions. Use retries with backoff, not sleeps
5. **Self-diagnosing** — On failure, print the actual response so you can debug without re-running

## Trust Model

Every test below has a **"Trust because"** field. This answers: "Why should I believe this test passing means the feature works?" The answer must never be "because the test checks the output" — it must explain what *specific contract* is being verified.

---

## Category 1: Protocol Compliance

These tests verify MCP JSON-RPC 2.0 correctness. Failures here mean no MCP client can talk to us.

### 1.1 — Initialize returns capabilities

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Send `initialize` request, verify response has `capabilities.tools` |
| **Assert** | `response.result.capabilities.tools` exists, `response.result.serverInfo.name == "kaboom-browser-devtools"`, `response.result.serverInfo.version` matches VERSION file |
| **Trust because** | If capabilities are wrong, Claude/Cursor won't know what tools exist. Version mismatch means wrong binary is installed. |

### 1.2 — tools/list returns exactly 5 tools

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Send `tools/list`, extract tool names |
| **Assert** | Exactly `["observe", "generate", "configure", "interact", "analyze"]` — no more, no less. Count == 5. |
| **Trust because** | Exact-match means no extras sneak in and no tools are missing. |

### 1.3 — tools/list schema shapes are valid

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | For each tool in tools/list, verify `inputSchema` has correct `required` fields and `properties` |
| **Assert** | `observe` requires `what`, `generate` requires `format`, `configure` requires `action`, `interact` requires `action`. Each has `format: "object"`. |
| **Trust because** | Schema errors mean MCP clients send wrong params. This is the contract AI models use to call tools. |

### 1.4 — Response IDs match request IDs

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Send 3 requests with IDs 1, 2, 3. Verify each response has matching ID. |
| **Assert** | `response[i].id == request[i].id` for all three |
| **Trust because** | ID mismatch means responses get routed to wrong callers. This is a JSON-RPC 2.0 requirement. |

### 1.5 — Stdout purity (only valid JSON-RPC)

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Capture all stdout from a session with 5 tool calls. Every line must be valid JSON with `"jsonrpc": "2.0"`. |
| **Assert** | Line-by-line: `jq -e '.jsonrpc == "2.0"'` succeeds on every line. Zero non-JSON lines. |
| **Trust because** | Any non-JSON on stdout breaks the MCP transport. This is the #1 production failure mode. |

### 1.6 — Unknown method returns error -32601

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Send `{"method": "bogus/method", "id": 99}` |
| **Assert** | Response has `error.code == -32601` and `id == 99` |
| **Trust because** | Without this, unknown methods silently succeed or crash the server. |

### 1.7 — Malformed JSON returns parse error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Send `{not valid json` on stdin |
| **Assert** | Response has `error.code == -32700` (parse error) |
| **Trust because** | Malformed input from buggy clients must not crash the server. |

---

## Category 2: Observe Tool (23 modes)

Each mode must return a valid response shape, even with no data.

### 2.1 — observe(page) returns URL and title

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Call `observe` with `what: "page"` |
| **Assert** | Response is not an error. Content text is parseable. Contains page data or "no tracked tab" message. |
| **Trust because** | This is the most basic observe call. If this fails, no observe works. |

### 2.2 — observe(tabs) returns tab array

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "tabs"` |
| **Assert** | Response content contains JSON with `tabs` array (may be empty). `tracking_active` field exists as boolean. |
| **Trust because** | MCP clients use this to know what's being tracked. Shape must be stable. |

### 2.3 — observe(logs) with no data returns empty

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Clear buffers, then call `observe` with `what: "logs"` |
| **Assert** | Response has `count: 0` or empty logs array. NOT an error response. |
| **Trust because** | Empty state must be distinguishable from error state. Returning an error for "no data" breaks AI workflows. |

### 2.4 — observe(logs) with min_level filter

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Call `observe` with `what: "logs"`, `min_level: "error"` |
| **Assert** | Response is valid, no error. If logs exist, all have level >= error. |
| **Trust because** | Filter params that silently fail mean AI gets wrong data and makes wrong decisions. |

### 2.5 — observe(network_waterfall) returns entries array

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "network_waterfall"` |
| **Assert** | Response contains `entries` array and `count` number. No error. |
| **Trust because** | Network waterfall is the most-used observe mode. Shape breakage affects every user. |

### 2.6 — observe(network_waterfall) with limit parameter

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Call with `what: "network_waterfall"`, `limit: 5` |
| **Assert** | Response entries array has <= 5 items. |
| **Trust because** | Limit is critical for keeping MCP context windows manageable. If it silently ignores limit, AI context overflows. |

### 2.7 — observe(errors) returns error array

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "errors"` |
| **Assert** | Response contains `errors` array and `count` number. |
| **Trust because** | Error detection is core functionality. |

### 2.8 — observe(vitals) returns metrics shape

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "vitals"` |
| **Assert** | Response contains `metrics` object with `has_data` boolean. |
| **Trust because** | Web Vitals is the performance monitoring surface. Shape must be stable. |

### 2.9 — observe(actions) returns entries

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "actions"` |
| **Assert** | Response contains `entries` array and `count`. |
| **Trust because** | Actions feed test generation and reproduction. |

### 2.10 — observe(websocket_events) returns events

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "websocket_events"` |
| **Assert** | Response contains events array. No error. |
| **Trust because** | WebSocket capture is a key differentiator. |

### 2.11 — observe(websocket_status) returns connection status

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "websocket_status"` |
| **Assert** | Response is valid, not error. Contains connection state data. |
| **Trust because** | Status endpoint must always respond, even with no WebSocket connections. |

### 2.12 — observe(extension_logs) returns logs

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "extension_logs"` |
| **Assert** | Response contains logs data. No error. |
| **Trust because** | Extension debugging depends on this. |

### 2.13 — observe(pilot) returns pilot status

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "pilot"` |
| **Assert** | Response contains pilot enabled/disabled state. |
| **Trust because** | AI Web Pilot gate — all interact commands check this. |

### 2.14 — observe(performance) returns snapshots

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "performance"` |
| **Assert** | Response is valid, not error. |
| **Trust because** | Performance snapshots feed Web Vitals reporting. |

### 2.15 — observe(timeline) returns unified entries

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "timeline"` |
| **Assert** | Response contains `entries` array and `count`. |
| **Trust because** | Timeline is the unified view across all buffer types. |

### 2.16 — observe(error_clusters) returns clusters

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "error_clusters"` |
| **Assert** | Response contains `clusters` array and `total_count`. |
| **Trust because** | Error clustering reduces noise for AI. Shape must be stable. |

### 2.17 — observe(history) returns navigation history

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "history"` |
| **Assert** | Response contains `entries` array and `count`. |
| **Trust because** | Navigation history is used by reproduction and test generation. |

### 2.18 — observe(accessibility) returns audit data

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "accessibility"` |
| **Assert** | Response is valid. Contains audit results or "no extension" message. |
| **Trust because** | A11y audits feed SARIF export. |

### 2.19 — observe(security_audit) returns findings

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "security_audit"` |
| **Assert** | Response is valid, contains findings data. |
| **Trust because** | Security audit is a paid-tier feature. |

### 2.20 — observe(third_party_audit) returns analysis

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "third_party_audit"` |
| **Assert** | Response is valid, contains third-party analysis. |
| **Trust because** | Third-party tracking is compliance-critical for enterprise users. |

### 2.21 — observe(pending_commands) returns command queues

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "pending_commands"` |
| **Assert** | Response contains `pending`, `completed`, `failed` arrays. |
| **Trust because** | Async command tracking is the interact tool's feedback loop. |

### 2.22 — observe(failed_commands) returns failures

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `observe` with `what: "failed_commands"` |
| **Assert** | Response contains `commands` array and `count`. |
| **Trust because** | Failed command visibility prevents silent failures. |

### 2.23 — observe with invalid "what" returns structured error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `observe` with `what: "nonexistent_mode"` |
| **Assert** | Response has `isError: true`. Error message mentions valid options or "unknown mode". |
| **Trust because** | Typos in mode names must produce helpful errors, not empty success responses. |

### 2.24 — observe with missing "what" returns error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `observe` with `{}` (no what parameter) |
| **Assert** | Response has error about missing required parameter. |
| **Trust because** | Missing required params must fail loudly. |

---

## Category 3: Generate Tool (7 formats)

### 3.1 — generate(reproduction) returns script

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `generate` with `format: "reproduction"` |
| **Assert** | Response is not error. Content contains a script (even if empty/placeholder). |
| **Trust because** | Reproduction scripts are the primary debugging output. |

### 3.2 — generate(test) returns Playwright test

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `generate` with `format: "test"` |
| **Assert** | Response is not error. Content contains test code. |
| **Trust because** | Test generation is a core feature. |

### 3.3 — generate(pr_summary) returns summary

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `generate` with `format: "pr_summary"` |
| **Assert** | Response is not error. Content contains summary text. |
| **Trust because** | PR summaries are used in CI workflows. |

### 3.4 — generate(sarif) returns valid SARIF JSON

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `generate` with `format: "sarif"` |
| **Assert** | Response contains SARIF data with `version: "2.1.0"` and `$schema` URL. |
| **Trust because** | SARIF is consumed by GitHub Code Scanning. Invalid format = silent CI failure. |

### 3.5 — generate(har) returns HAR structure

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `generate` with `format: "har"` |
| **Assert** | Response contains HAR data with `log.version` and `log.entries` array. |
| **Trust because** | HAR is consumed by Chrome DevTools, Charles Proxy, etc. Invalid format = import fails. |

### 3.6 — generate(csp) returns policy string

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `generate` with `format: "csp"` |
| **Assert** | Response contains `status` ("ok" or "unavailable"), `mode` field, and `policy` string. |
| **Trust because** | CSP generation is security-critical. Wrong policy = XSS or broken site. |

### 3.7 — generate(sri) returns hashes

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `generate` with `format: "sri"` |
| **Assert** | Response contains `resources` array. |
| **Trust because** | SRI hashes prevent supply-chain attacks. |

### 3.8 — generate with invalid format returns error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `generate` with `format: "docx"` |
| **Assert** | Response has `isError: true` with helpful message listing valid formats. |
| **Trust because** | Invalid format must not silently return empty success. |

### 3.9 — generate with missing format returns error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `generate` with `{}` |
| **Assert** | Response has error about missing required parameter. |
| **Trust because** | Same as 2.24 — required params must fail loudly. |

---

## Category 4: Configure Tool (12 actions)

### 4.1 — configure(health) returns server health

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `configure` with `action: "health"` |
| **Assert** | Response contains `status` ("ok"/"degraded"/"unhealthy"), `version`, `daemon_uptime`, `extension_connected` (boolean), `buffers` object. |
| **Trust because** | Health is the liveness probe. Every field is consumed by monitoring. |

### 4.2 — configure(clear) resets buffers

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Call `configure` with `action: "clear"`, then call `observe(logs)` |
| **Assert** | After clear, observe returns count: 0 or empty array. |
| **Trust because** | Clear is used between test runs. If it doesn't actually clear, tests leak state. The two-step verification (clear then observe) proves the clear worked. |

### 4.3 — configure(clear) with specific buffer

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Call `configure` with `action: "clear"`, `buffer: "network"` |
| **Assert** | Network waterfall is empty, but logs are preserved. |
| **Trust because** | Selective clear is used for targeted debugging. Must not clear everything. |

### 4.4 — configure(store) save and load roundtrip

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Save `{key: "test", data: {"foo": "bar"}}`, then load same key |
| **Assert** | Loaded data exactly matches saved data: `{"foo": "bar"}` |
| **Trust because** | Roundtrip proves serialization/deserialization works. If data is silently corrupted, this catches it. |

### 4.5 — configure(store) list shows saved keys

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Save a key, then call store with `store_action: "list"` |
| **Assert** | List includes the saved key. |
| **Trust because** | List must reflect actual state, not cached/stale data. |

### 4.6 — configure(noise_rule) add and list roundtrip

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Add a noise rule, then list rules |
| **Assert** | Listed rules contain the one we added, with matching pattern and category. |
| **Trust because** | Noise rules affect all observe responses. If add silently fails, data is noisy. |

### 4.7 — configure(audit_log) returns tool call history

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Make 3 tool calls, then call `configure` with `action: "audit_log"` |
| **Assert** | Audit log contains >= 3 entries. Each has `tool_name`, `timestamp`, `session_id`. |
| **Trust because** | Audit trail is compliance-critical. If it's empty after tool calls, auditing is broken. |

### 4.8 — configure(streaming) enable and status

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Enable streaming, then check status |
| **Assert** | Status shows streaming is enabled with configured event types. |
| **Trust because** | Streaming state must persist within a session. |

### 4.9 — configure(test_boundary_start/end) roundtrip

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Start a test boundary with `test_id: "uat-1"`, end it |
| **Assert** | Both return success. No error on start or end. |
| **Trust because** | Test boundaries isolate CI test runs. If they error, CI tests can't use the feature. |

### 4.10 — analyze(what:"dom") with selector

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call with `what: "dom"`, `selector: "body"` |
| **Assert** | Response is valid (may be "no extension" or actual DOM result). Not a crash. |
| **Trust because** | DOM queries are sent to extension via pending queries. Must not crash without extension. |

### 4.11 — configure with invalid action returns error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `configure` with `action: "destroy_everything"` |
| **Assert** | Response has `isError: true` with helpful message. |
| **Trust because** | Invalid actions must not silently succeed. |

---

## Category 5: Interact Tool (11 actions)

### 5.1 — interact(list_states) returns array

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Call `interact` with `action: "list_states"` |
| **Assert** | Response contains `states` array and `count`. Not an error. |
| **Trust because** | list_states doesn't require pilot. Must always work. |

### 5.2 — interact(save_state/load_state) roundtrip

| Field | Value |
|-------|-------|
| **Type** | Functional |
| **What** | Save state "uat-test", list states, verify it appears, load it, delete it, verify it's gone |
| **Assert** | Save returns "saved", list includes "uat-test", load returns the state, delete returns "deleted", final list excludes "uat-test". |
| **Trust because** | Full CRUD roundtrip. If any step fails, state management is broken. |

### 5.3 — interact(execute_js) without pilot returns pilot disabled error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `interact` with `action: "execute_js"`, `script: "1+1"` (no extension connected) |
| **Assert** | Response has `isError: true` with pilot-disabled error code. |
| **Trust because** | Pilot-gated actions must fail clearly when pilot is off. Silent success with no execution is worse. |

### 5.4 — interact(navigate) without pilot returns error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `interact` with `action: "navigate"`, `url: "https://example.com"` |
| **Assert** | Response has pilot-disabled error. |
| **Trust because** | Same gate as 5.3. |

### 5.5 — interact(highlight) without pilot returns error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `interact` with `action: "highlight"`, `selector: "body"` |
| **Assert** | Response has pilot-disabled error. |
| **Trust because** | Same gate as 5.3. |

### 5.6 — interact with invalid action returns error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `interact` with `action: "fly_to_moon"` |
| **Assert** | Response has `isError: true` with helpful message. |
| **Trust because** | Invalid actions must not crash. |

### 5.7 — interact(save_state) without name returns error

| Field | Value |
|-------|-------|
| **Type** | Negative |
| **What** | Call `interact` with `action: "save_state"` (no snapshot_name) |
| **Assert** | Error about missing required parameter. |
| **Trust because** | Required param validation. |

---

## Category 6: Server Lifecycle

### 6.1 — Cold start: first tool call works

| Field | Value |
|-------|-------|
| **Type** | Lifecycle |
| **What** | Kill any running daemon. Send `initialize` + `tools/call` (observe page). |
| **Assert** | Response arrives within 10 seconds. Is valid JSON-RPC. Not a timeout error. |
| **Trust because** | Cold start is the most fragile path. Daemon must auto-spawn and respond. |

### 6.2 — Server persists across client disconnects

| Field | Value |
|-------|-------|
| **Type** | Lifecycle |
| **What** | Connect client 1, make a call, disconnect. Connect client 2, make a call. |
| **Assert** | Client 2 gets a valid response. Server didn't die when client 1 disconnected. |
| **Trust because** | Daemon mode is persistent. Client disconnect must not kill the server. |

### 6.3 — Graceful shutdown via --stop

| Field | Value |
|-------|-------|
| **Type** | Lifecycle |
| **What** | Start daemon, verify health, run `kaboom-agentic-browser --stop --port $PORT` |
| **Assert** | --stop exits 0. Port is freed (nothing listening). PID file is cleaned up. |
| **Trust because** | Ungraceful shutdown leaves orphan processes. PID file check prevents stale state. |

### 6.4 — Daemon health endpoint responds

| Field | Value |
|-------|-------|
| **Type** | Lifecycle |
| **What** | `curl http://localhost:$PORT/health` |
| **Assert** | Returns JSON with `status`, `version`, `daemon_uptime`. Status code 200. |
| **Trust because** | Health endpoint is the liveness probe for monitoring. |

### 6.5 — Version matches VERSION file

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | Read VERSION file. Call configure(health). Compare version field. Also check initialize response serverInfo.version. |
| **Assert** | All three match exactly. |
| **Trust because** | Version mismatch means the wrong binary is running. This is the "did npm publish work?" test. |

---

## Category 7: Concurrency & Resilience

### 7.1 — 10 concurrent clients get valid responses

| Field | Value |
|-------|-------|
| **Type** | Load |
| **What** | Fork 10 processes, each sends `tools/list` simultaneously |
| **Assert** | All 10 get valid responses with 5 tools each. Zero failures. |
| **Trust because** | Real usage has multiple MCP clients (Claude + Cursor + CI). Must handle concurrency. |

### 7.2 — Rapid sequential tool calls don't crash

| Field | Value |
|-------|-------|
| **Type** | Load |
| **What** | Send 20 tool calls in a tight loop (observe page, observe logs, observe network, ...) |
| **Assert** | All 20 return valid responses. No crashes, no hangs. |
| **Trust because** | AI agents make rapid-fire tool calls. Server must not accumulate state that causes failure. |

### 7.3 — Large response doesn't crash (network waterfall with high limit)

| Field | Value |
|-------|-------|
| **Type** | Resilience |
| **What** | Call `observe` with `what: "network_waterfall"`, `limit: 10000` |
| **Assert** | Response is valid JSON-RPC. No truncation errors. May be empty, but not malformed. |
| **Trust because** | Large limit values must not cause OOM or buffer overflow. |

---

## Category 8: Security

### 8.1 — Extension endpoints reject requests without X-Kaboom-Client header

| Field | Value |
|-------|-------|
| **Type** | Security |
| **What** | `curl http://localhost:$PORT/sync` (no X-Kaboom-Client header) |
| **Assert** | Returns 403 Forbidden with error about missing header. |
| **Trust because** | extensionOnly middleware must actually block. If it doesn't, any local process can inject data. |

### 8.2 — Extension endpoints accept valid X-Kaboom-Client header

| Field | Value |
|-------|-------|
| **Type** | Security |
| **What** | `curl -H 'X-Kaboom-Client: kaboom-extension/5.8.0' http://localhost:$PORT/sync` |
| **Assert** | Does NOT return 403. (May return 400 for missing body, but not 403.) |
| **Trust because** | The middleware must not over-block. Valid extension requests must pass. |

### 8.3 — CORS rejects non-localhost origins

| Field | Value |
|-------|-------|
| **Type** | Security |
| **What** | `curl -H 'Origin: https://evil.com' http://localhost:$PORT/health` |
| **Assert** | Returns 403 or response has no `Access-Control-Allow-Origin: https://evil.com` header. |
| **Trust because** | DNS rebinding / CORS bypass is the primary attack vector for local servers. |

### 8.4 — Host header validation rejects non-localhost

| Field | Value |
|-------|-------|
| **Type** | Security |
| **What** | `curl -H 'Host: evil.com' http://127.0.0.1:$PORT/health` |
| **Assert** | Returns 403. |
| **Trust because** | DNS rebinding requires spoofed Host header. Must reject. |

---

## Category 9: HTTP Endpoints (Daemon Direct)

### 9.1 — /health returns complete health object

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | `curl http://localhost:$PORT/health` |
| **Assert** | JSON with keys: `status`, `version`, `daemon_uptime`, `extension_connected`, `buffers`. `buffers` has: `console_entries`, `network_waterfall`, `websocket_events`, `enhanced_actions`, `extension_logs`. All are numbers. |
| **Trust because** | Health shape is consumed by monitoring dashboards. Missing fields break integrations. |

### 9.2 — /diagnostics returns detailed info

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | `curl http://localhost:$PORT/diagnostics` |
| **Assert** | Returns JSON (not 404, not 500). Contains system info. |
| **Trust because** | Diagnostics is the debugging escape hatch. Must not be broken when you need it most. |

### 9.3 — /mcp POST accepts JSON-RPC

| Field | Value |
|-------|-------|
| **Type** | Contract |
| **What** | `curl -X POST -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' http://localhost:$PORT/mcp` |
| **Assert** | Returns JSON-RPC response with tools array. |
| **Trust because** | HTTP transport is the alternative to stdio. Must work for web-based MCP clients. |

### 9.4 — /shutdown POST stops the server

| Field | Value |
|-------|-------|
| **Type** | Lifecycle |
| **What** | `curl -X POST http://localhost:$PORT/shutdown` |
| **Assert** | Returns success. Server stops within 5 seconds. Port is freed. |
| **Trust because** | Programmatic shutdown is used by CI cleanup. |

---

## Category 10: Regression Guards

These tests exist because of specific bugs we've hit before.

### 10.1 — No stub tools in tools/list (regression: v5.7.5)

| Field | Value |
|-------|-------|
| **Type** | Regression |
| **What** | Get tools/list, check for "analyze" tool |
| **Assert** | "analyze" is NOT in the list. Only observe, generate, configure, interact. |
| **Trust because** | We shipped stub tools that returned errors. This must never happen again. |

### 10.2 — Empty buffers don't crash observe (regression: nil pointer)

| Field | Value |
|-------|-------|
| **Type** | Regression |
| **What** | Immediately after cold start (no extension data), call all 23 observe modes |
| **Assert** | All 23 return valid responses (success or graceful "no data"). Zero panics. |
| **Trust because** | Nil pointer on empty state was a real bug (bridge.go:143). This runs all modes in the worst-case state. |

### 10.3 — Version in health matches binary version (regression: stale build)

| Field | Value |
|-------|-------|
| **Type** | Regression |
| **What** | `kaboom-agentic-browser --version` output vs health endpoint version |
| **Assert** | Both match. |
| **Trust because** | We shipped binaries where --version and health reported different versions due to stale compilation. |

---

## Execution Order

```
1. Kill any running daemon (clean slate)
2. Run Category 1 (Protocol) — establishes server works at all
3. Run Category 6.1 (Cold start) — proves daemon spawning works
4. Run Category 2 (Observe) — all 24 tests
5. Run Category 3 (Generate) — all 9 tests
6. Run Category 4 (Configure) — all 11 tests
7. Run Category 5 (Interact) — all 7 tests
8. Run Category 7 (Concurrency) — 3 tests
9. Run Category 8 (Security) — 4 tests
10. Run Category 9 (HTTP) — 4 tests
11. Run Category 6.2-6.5 (Remaining lifecycle)
12. Run Category 10 (Regressions) — 3 tests
13. Graceful shutdown and verify clean exit
```

## Summary

| Category | Tests | Type |
|----------|------:|------|
| Protocol Compliance | 7 | Contract + Negative |
| Observe Tool | 24 | Contract + Functional + Negative |
| Generate Tool | 9 | Contract + Negative |
| Configure Tool | 11 | Contract + Functional + Negative |
| Interact Tool | 7 | Contract + Functional + Negative |
| Server Lifecycle | 5 | Lifecycle |
| Concurrency | 3 | Load + Resilience |
| Security | 4 | Security |
| HTTP Endpoints | 4 | Contract + Lifecycle |
| Regression Guards | 3 | Regression |
| **Total** | **77** | |

Current UAT has **8 tests**. This plan has **77 tests** covering every tool mode, every error path, and every regression we've hit.

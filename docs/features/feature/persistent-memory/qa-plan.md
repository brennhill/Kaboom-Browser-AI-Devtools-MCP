---
status: proposed
scope: feature/persistent-memory/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-persistent-memory
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Persistent Memory

> QA plan for the Persistent Cross-Session Memory feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Persistent memory writes data to disk in `.kaboom/` -- the primary risks are that persisted data contains sensitive information, that the key-value store allows access to arbitrary files, that data survives longer than expected, and that file permissions are too permissive.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Persisted data contains unsanitized sensitive content | Data written to `.kaboom/` must have the same sanitization applied as in-memory data (auth headers stripped, passwords redacted) -- verify sanitization happens BEFORE persistence | critical |
| DL-2 | Namespace/key path traversal | `session_store(action: "save", namespace: "../../etc", key: "passwd")` could write outside `.kaboom/` -- verify path validation prevents traversal | critical |
| DL-3 | Key path traversal | `session_store(action: "load", namespace: "baselines", key: "../meta")` could read arbitrary files in `.kaboom/` -- verify key validation prevents traversal | critical |
| DL-4 | `.kaboom/` not gitignored | If `.kaboom/` is not added to `.gitignore`, persisted data (baselines, noise rules, error history) could be committed to version control and pushed to remote repos | critical |
| DL-5 | File permissions too permissive | Files at 0644 and directories at 0755 are user-readable but also group/world-readable -- verify this is acceptable or if 0600/0700 would be more appropriate | high |
| DL-6 | Error history contains sensitive error messages | Error fingerprints and messages could contain PII, API keys, or internal URLs that appeared in console errors | high |
| DL-7 | API schema data persists endpoint details | Inferred API schemas in `.kaboom/api_schema/` reveal the application's API structure, endpoints, and parameter types | medium |
| DL-8 | Session metadata reveals usage patterns | `meta.json` with session count and timestamps reveals when the developer was working -- acceptable for local tool | low |
| DL-9 | Load action returns data from arbitrary namespace/key | An AI agent could read any persisted namespace by guessing key names -- verify this is intentional (namespaces are not access-controlled) | medium |
| DL-10 | Stats action reveals total storage size | `session_store(action: "stats")` shows total bytes and entry counts per namespace -- acceptable for diagnostics | low |
| DL-11 | Concurrent server instances -- file locking bypass | If flock fails silently, two instances could corrupt persisted data or both write simultaneously | high |
| DL-12 | Corrupted file returns partial/garbage data | If a JSON file is partially written (crash during save), loading it could return malformed data that confuses the AI agent | medium |

### Negative Tests (must NOT leak)
- [ ] `session_store(namespace: "../../etc", key: "passwd")` is rejected with path traversal error
- [ ] `session_store(namespace: "baselines", key: "../meta")` is rejected with path traversal error
- [ ] `session_store(namespace: "baselines", key: "../../.env")` is rejected with path traversal error
- [ ] `.kaboom/` is automatically added to `.gitignore` on first use
- [ ] Data persisted to disk has auth headers stripped and passwords redacted (same sanitization as in-memory)
- [ ] Error history entries do not contain full stack traces with file paths to system directories
- [ ] `session_store(action: "save", data: <1.5MB>)` is rejected (exceeds 1MB limit)
- [ ] Total `.kaboom/` directory size is enforced at 10MB maximum
- [ ] No file in `.kaboom/` contains raw HTTP Authorization header values, Bearer tokens, or API keys

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | `load_session_context` response structure | AI understands this is a summary of ALL persisted data, not raw data -- each section is a summary (counts, names, timestamps) | [ ] |
| CL-2 | Session count semantics | AI understands `session_count` is cumulative across all restarts, not concurrent sessions | [ ] |
| CL-3 | Namespace and key terminology | AI understands `namespace` is a logical grouping (like "baselines", "noise") and `key` is a specific entry within it | [ ] |
| CL-4 | Error history entry fields | AI understands `fingerprint` is a deduplicated message hash, `occurrences` is total count across sessions, `resolved` means manually marked resolved | [ ] |
| CL-5 | "Not found" error clarity | When loading a non-existent key, the error clearly states it was not found and does not suggest a server error | [ ] |
| CL-6 | Stats response clarity | `stats` action returns storage-level information (bytes, counts), not the actual data content | [ ] |
| CL-7 | Shutdown persistence guarantee | AI understands that dirty data is flushed on shutdown, so no data loss on graceful stop | [ ] |
| CL-8 | First session behavior | AI understands `session_count: 1` and empty summaries on first use is normal, not an error | [ ] |
| CL-9 | `list` returns keys only | `session_store(action: "list")` returns key names without `.json` extension, not the data itself | [ ] |
| CL-10 | Save action confirmation | Save response confirms what was saved (namespace, key, size) without echoing the full data back | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might think `load_session_context` loads raw data into memory -- verify response explains it returns summaries and applies config (like noise rules)
- [ ] AI might try to save very large data blocks (full buffer dumps) -- verify the 1MB limit is clearly communicated in error messages
- [ ] AI might not understand that `session_store` is a general-purpose KV store usable for any purpose -- verify tool description makes this clear
- [ ] AI might think "resolved" errors have been fixed -- verify documentation explains "resolved" means "marked as resolved by the agent" not "confirmed fixed"
- [ ] AI might try to use `session_store` to persist capture overrides between sessions -- verify this works OR document that capture overrides are session-scoped
- [ ] AI might confuse `load_session_context` (summary of everything) with `session_store(action: "load")` (load specific key) -- verify both tool descriptions are unambiguous

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Start new session with context | 1 step: `load_session_context()` | No -- already minimal |
| Save a baseline | 1 step: `session_store(action: "save", namespace: "baselines", key: "login", data: {...})` | No -- already minimal |
| Load a baseline | 1 step: `session_store(action: "load", namespace: "baselines", key: "login")` | No -- already minimal |
| List all baselines | 1 step: `session_store(action: "list", namespace: "baselines")` | No -- already minimal |
| Delete old data | 1 step: `session_store(action: "delete", namespace: "baselines", key: "login")` | No -- already minimal |
| Check storage usage | 1 step: `session_store(action: "stats")` | No -- already minimal |
| Full session restore | 1 step: `load_session_context()` restores noise rules and returns all summaries | No -- already minimal |

### Default Behavior Verification
- [ ] Feature works with zero configuration -- `.kaboom/` directory created automatically on first use
- [ ] `.gitignore` entry added automatically without user action
- [ ] `load_session_context()` on first-ever session returns fresh context with session_count=1 (no error)
- [ ] Background flush goroutine starts automatically, no opt-in needed
- [ ] Shutdown flush happens automatically on graceful server stop

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | SessionStore creation at `.kaboom/` | `NewSessionStore(projectDir)` | `.kaboom/` directory created, `meta.json` initialized | must |
| UT-2 | `.gitignore` updated on first use | Create store in project with `.gitignore` not containing `.kaboom/` | `.kaboom/` line added to `.gitignore` | must |
| UT-3 | `.gitignore` not duplicated | Create store in project where `.gitignore` already has `.kaboom/` | No duplicate line added | must |
| UT-4 | Save then load returns identical data | `Save("baselines", "login", data)` then `Load("baselines", "login")` | Loaded data matches saved data exactly | must |
| UT-5 | Load nonexistent key returns error | `Load("baselines", "nonexistent")` | Error: key not found | must |
| UT-6 | List returns all keys | Save 3 keys to "baselines", then `List("baselines")` | Returns 3 key names without `.json` extension | must |
| UT-7 | Delete removes file | `Delete("baselines", "login")` then `Load("baselines", "login")` | Load returns not-found error | must |
| UT-8 | Stats returns correct sizes | Save known-size data, call `Stats()` | Correct total bytes and per-namespace counts | must |
| UT-9 | File exceeding 1MB rejected | `Save("big", "huge", data_1.5MB)` | Error: file exceeds 1MB limit | must |
| UT-10 | Session count increments on restart | Create store, shutdown, create store again | meta.json shows session_count=2 | must |
| UT-11 | Fresh store returns session_count=1 | `NewSessionStore` on empty directory | meta.json: session_count=1 | must |
| UT-12 | load_session_context summary | Store with baselines, noise, errors | Returns combined context with counts for each namespace | must |
| UT-13 | Noise config restored on load_session_context | Save noise config, restart, `load_session_context` | Noise rules become active on the server | must |
| UT-14 | Namespace path traversal rejected | `Save("../../etc", "passwd", data)` | Error: invalid namespace (path traversal) | must |
| UT-15 | Key path traversal rejected | `Save("baselines", "../meta", data)` | Error: invalid key (path traversal) | must |
| UT-16 | Concurrent reads/writes no corruption | Multiple goroutines saving/loading | Mutex prevents corruption, go test -race passes | must |
| UT-17 | Shutdown flushes dirty data | Mark data dirty, call `Shutdown()` | Dirty data written to disk before shutdown completes | must |
| UT-18 | Background flush at interval | Mark data dirty, wait > 30 seconds | Data flushed by background goroutine | should |
| UT-19 | Error history capped at 500 | Add 501 error entries | Oldest entry evicted, count stays at 500 | should |
| UT-20 | Error entries older than 30 days evicted | Entry with timestamp > 30 days ago | Entry removed on cleanup | should |
| UT-21 | Corrupted JSON file skipped on load | Place invalid JSON in `.kaboom/baselines/bad.json` | File skipped, other files loaded normally | must |
| UT-22 | Missing directories created on save | Save to namespace that doesn't exist yet | Subdirectory created via MkdirAll | must |
| UT-23 | Store not initialized error | Call `load_session_context` before store is ready | Specific error message returned, no crash | must |
| UT-24 | Total directory size limit (10MB) | Save data until total exceeds 10MB | Error: storage limit exceeded | should |
| UT-25 | Read-only filesystem graceful degradation | Store on read-only filesystem | Save returns error, server continues in-memory-only | should |
| UT-26 | File permissions on creation | Save a file, check permissions | File: 0644, directory: 0755 | should |
| UT-27 | meta.json records project path | Create store | meta.json includes project path (CWD) | should |
| UT-28 | meta.json records timestamps | Create store, perform operations | `first_created` and `last_session` timestamps present and correct | should |
| UT-29 | Concurrent server instances -- file lock | Two SessionStores on same directory | Second gets read-only mode (flock on meta.json) | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | MCP load_session_context round-trip | MCP dispatcher + SessionStore | First call returns fresh context, subsequent calls return accumulated data | must |
| IT-2 | MCP session_store save/load | MCP dispatcher + SessionStore + filesystem | Save via MCP, load via MCP, data matches | must |
| IT-3 | MCP session_store list | MCP dispatcher + SessionStore | List returns all saved keys for a namespace | must |
| IT-4 | MCP session_store delete | MCP dispatcher + SessionStore + filesystem | Delete via MCP, file removed from disk | must |
| IT-5 | MCP session_store stats | MCP dispatcher + SessionStore | Returns total bytes and per-namespace counts | must |
| IT-6 | Server shutdown persists noise config | V4Server + SessionStore | Shutdown writes noise config to `.kaboom/noise/config.json` | must |
| IT-7 | Server shutdown persists baselines | V4Server + SessionStore | Shutdown writes baselines to `.kaboom/baselines/` | must |
| IT-8 | Server restart restores session data | V4Server lifecycle | Restart loads meta.json, increments session count, noise rules available on load_session_context | must |
| IT-9 | Background flush goroutine lifecycle | SessionStore + goroutine management | Goroutine starts on init, runs every 30s, exits cleanly on shutdown | must |
| IT-10 | Concurrent MCP operations | Multiple MCP calls + SessionStore mutex | No race condition under concurrent tool calls | must |
| IT-11 | Noise rules restored and active | SessionStore + NoiseConfig | After load_session_context, previously saved noise rules actively filter entries | should |
| IT-12 | Error history across sessions | SessionStore + error tracking | Errors accumulated across 3 sessions, counts correct | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Store creation (dirs + read meta) | Latency | < 100ms | must |
| PT-2 | load_session_context (all namespaces) | Latency | < 200ms | must |
| PT-3 | Single file save | Latency | < 50ms | must |
| PT-4 | Single file load | Latency | < 20ms | must |
| PT-5 | Shutdown flush (all dirty data) | Latency | < 500ms | must |
| PT-6 | Periodic flush cycle | Latency | < 100ms | should |
| PT-7 | Memory overhead (metadata + handles) | Memory | < 1MB | should |
| PT-8 | List with 100 keys in namespace | Latency | < 50ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Save empty data | `Save("ns", "key", {})` | Empty JSON object saved, loadable | must |
| EC-2 | Save null data | `Save("ns", "key", null)` | Error or null stored, no crash | should |
| EC-3 | Namespace with underscore and hyphen | `Save("my-namespace_v2", "key", data)` | Accepted if alphanumeric + hyphens + underscores | should |
| EC-4 | Key with dots | `Save("ns", "my.key.v2", data)` | Accepted (file becomes `my.key.v2.json`) | should |
| EC-5 | Very long key name | Key with 255+ characters | Rejected (filesystem path limit) or truncated | should |
| EC-6 | Unicode namespace name | `Save("", "key", data)` | Rejected: invalid characters | must |
| EC-7 | Crash during save (simulated) | Kill server mid-write | Next load either gets old data or skips corrupted file | should |
| EC-8 | `.gitignore` does not exist | Project has no `.gitignore` file | File created with `.kaboom/` entry | must |
| EC-9 | `.gitignore` is read-only | `.gitignore` exists but is read-only | Warning logged, server continues (user may need to add manually) | should |
| EC-10 | Multiple namespaces with same key name | `Save("baselines", "login", data1)` and `Save("errors", "login", data2)` | Both saved independently in separate directories | must |
| EC-11 | Load after delete | `Delete("ns", "key")` then `Load("ns", "key")` | Error: not found | must |
| EC-12 | Background flush with no dirty data | 30s passes with no changes | Flush completes quickly, no I/O | should |
| EC-13 | Concurrent save to same key | Two goroutines save to same namespace/key | Last write wins, no corruption (mutex) | must |
| EC-14 | Store created in temp directory | `NewSessionStore("/tmp/test-project")` | Store works normally in any writable directory | should |
| EC-15 | Error history entry resolved then reoccurs | Resolve error, then same error occurs again | Entry un-resolved, occurrence count continues | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application running locally
- [ ] Write access to the project directory

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"action": "load"}}` (load_session_context) | Check project directory for `.kaboom/` | `.kaboom/` directory created, `meta.json` exists with `session_count: 1` | [ ] |
| UAT-2 | Check `.gitignore` in project root | Open `.gitignore` | `.kaboom/` line present | [ ] |
| UAT-3 | `{"tool": "configure", "arguments": {"action": "load"}}` | Review response | Response shows session context with session_count, empty baselines/noise/errors summaries | [ ] |
| UAT-4 | `{"tool": "configure", "arguments": {"action": "store", "namespace": "baselines", "key": "login-page", "data": {"load_ms": 1200, "requests": 15}}}` | Check `.kaboom/baselines/` | File `login-page.json` created with correct JSON content | [ ] |
| UAT-5 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "load", "namespace": "baselines", "key": "login-page"}}` | None | Response returns `{"load_ms": 1200, "requests": 15}` matching saved data | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "list", "namespace": "baselines"}}` | None | Response lists `["login-page"]` | [ ] |
| UAT-7 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "stats"}}` | None | Response shows total bytes > 0, baselines namespace has 1 entry | [ ] |
| UAT-8 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "save", "namespace": "baselines", "key": "dashboard", "data": {"load_ms": 2500, "requests": 45}}}` | None | Second baseline saved | [ ] |
| UAT-9 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "list", "namespace": "baselines"}}` | None | Response lists `["dashboard", "login-page"]` (or `["login-page", "dashboard"]`) | [ ] |
| UAT-10 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "delete", "namespace": "baselines", "key": "dashboard"}}` | Check `.kaboom/baselines/` | File `dashboard.json` removed | [ ] |
| UAT-11 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "load", "namespace": "baselines", "key": "dashboard"}}` | None | Error: key not found | [ ] |
| UAT-12 | Restart the Kaboom server | Server restarts | Server starts without error | [ ] |
| UAT-13 | `{"tool": "configure", "arguments": {"action": "load"}}` | Check `meta.json` | `session_count` incremented to 2, `last_session` timestamp updated | [ ] |
| UAT-14 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "load", "namespace": "baselines", "key": "login-page"}}` | None | Previously saved baseline still available after restart | [ ] |
| UAT-15 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "save", "namespace": "../../etc", "key": "test"}}` | None | Error: invalid namespace (path traversal rejected) | [ ] |
| UAT-16 | `{"tool": "configure", "arguments": {"action": "store", "store_action": "save", "namespace": "test", "key": "../meta", "data": {"hack": true}}}` | Check `.kaboom/meta.json` | meta.json unchanged; error returned for path traversal | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Path traversal for namespace prevented | Try save with namespace `../../etc` | Rejected with path traversal error | [ ] |
| DL-UAT-2 | Path traversal for key prevented | Try save with key `../meta` | Rejected with path traversal error | [ ] |
| DL-UAT-3 | `.gitignore` updated automatically | Check `.gitignore` after first server start | `.kaboom/` entry present | [ ] |
| DL-UAT-4 | File size limit enforced | Try saving data > 1MB | Rejected with size limit error | [ ] |
| DL-UAT-5 | Persisted data does not contain auth headers | Save some data, then inspect `.kaboom/` files manually | No Authorization, Bearer, Cookie values in any file | [ ] |
| DL-UAT-6 | Error history sanitized | Generate errors with stack traces, check `.kaboom/errors/history.json` | Sensitive paths and values redacted | [ ] |
| DL-UAT-7 | No raw sensitive data on disk | Search all files in `.kaboom/` for common secrets patterns | No API keys, tokens, passwords found | [ ] |

### Regression Checks
- [ ] All MCP tools work normally when `.kaboom/` directory does not yet exist (first run)
- [ ] Server starts and operates normally on a read-only filesystem (without persistence)
- [ ] Existing noise filtering, memory enforcement, and TTL features work with persistence enabled
- [ ] Server shutdown completes within 500ms even with pending dirty data
- [ ] No performance degradation in tool response times with persistence active

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

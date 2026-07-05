---
status: proposed
scope: feature/interception-deferral/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-interception-deferral
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Interception Deferral

> QA plan for the Interception Deferral feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Deferral timing data in MCP response | `get_page_info` reports injection timestamp, Phase 2 timestamp, and pre-existing WebSocket counts. These are diagnostic metrics, not sensitive data. Verify no PII or page content is exposed. | low |
| DL-2 | Pre-existing WebSocket URL exposure | Retroactive WebSocket discovery via `performance.getEntriesByType('resource')` reports WebSocket URLs. These URLs may contain tokens (e.g., `wss://api.example.com?token=abc`). | high |
| DL-3 | "Capture from page start" toggle state | Stored in `chrome.storage.local`. The toggle value (boolean) is not sensitive, but verify no additional data is stored alongside it. | low |
| DL-4 | Early console logs lost (privacy benefit) | Console logs during page load are NOT captured when deferral is active. This is actually a privacy benefit -- framework initialization noise with potential PII is not captured. | low (positive) |
| DL-5 | Early fetch requests via Performance API | `performance.getEntriesByType('resource')` captures URLs of all network requests. Verify only the URL and timing data are reported, not request/response bodies or headers. | medium |
| DL-6 | Settings message between content.js and inject.js | `KABOOM_SETTINGS` message passes deferral preference via `window.postMessage`. Verify the message contains only the boolean flag, not other settings or user data. | medium |
| DL-7 | Data transmission path | Deferral is entirely extension-side logic. Verify no new data is sent to external servers. Diagnostic data in `get_page_info` flows over localhost only. | critical |

### Negative Tests (must NOT leak)
- [ ] WebSocket URLs reported as "pre-existing" do NOT include authentication tokens in the logged URL (or if they do, data stays on localhost only)
- [ ] `KABOOM_SETTINGS` message contains ONLY the deferral boolean, no other user data
- [ ] "Capture from page start" toggle stores ONLY a boolean in `chrome.storage.local`
- [ ] No early console logs are captured during deferral (logs before Phase 2 are intentionally lost)
- [ ] Performance API resource entries used for retroactive discovery do not include request/response bodies
- [ ] No deferral-related data is sent to external servers

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Deferral active meaning | LLM understands that when deferral is active, early events (console logs, fetch calls before Phase 2) are intentionally not captured. This is NOT a bug or data loss. | [ ] |
| CL-2 | `missedEvents` semantics | `{ consoleLogs: true, fetchCalls: true, wsConnections: true }` means these event types WERE potentially missed during the deferral window. LLM should not interpret `true` as "captured". | [ ] |
| CL-3 | Phase 2 timing context | `injectionTimestamp` and `phase2Timestamp` relative to page load -- LLM should understand the gap represents the deferral window during which Kaboom was passive. | [ ] |
| CL-4 | Pre-existing WebSocket meaning | "pre-existing connection (not intercepted)" means the WebSocket was opened before Kaboom's wrapper installed. LLM should know messages on this connection are NOT being captured. | [ ] |
| CL-5 | "Capture from page start" toggle | LLM should understand this toggle exists as an escape hatch for debugging initialization issues. Default OFF means deferral is active. | [ ] |
| CL-6 | Performance data accuracy | LLM should know that FCP, LCP, TTFB metrics are NOT inflated by Kaboom when deferral is active. These are trustworthy measurements. | [ ] |
| CL-7 | Retroactive fetch discovery | Fetch calls before Phase 2 are visible via Performance API resource entries, NOT via Kaboom's fetch wrapper. LLM should understand these have URL and timing but no response body capture. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM interprets missing early console logs as a bug rather than intentional deferral behavior -- verify `get_page_info` deferral section explains the capture window clearly
- [ ] LLM does not realize pre-existing WebSocket connections are NOT being monitored for messages -- verify the "not intercepted" label is unambiguous
- [ ] LLM assumes all fetch requests are captured because Performance API shows them -- verify the distinction between "URL visible" and "body captured" is clear
- [ ] LLM tells the user to enable "Capture from page start" without understanding the performance tradeoff -- verify the diagnostic context explains the impact

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (transparent, zero-configuration by default)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Use deferral (default) | 0 steps: enabled by default | Already at zero |
| Check if deferral is active | 1 step: `get_page_info` and read `kaboom.deferral` section | No -- single tool call |
| Disable deferral for debugging | 1 step: toggle "Capture from page start" in extension options | No -- single toggle |
| Understand capture window | 1 step: read Phase 1/Phase 2 timestamps from `get_page_info` | No -- data is in response |

### Default Behavior Verification
- [ ] Deferral is ON by default (no user action needed)
- [ ] Phase 1 runs immediately at script injection (API namespace, message listener, PerformanceObservers)
- [ ] Phase 2 runs after `window.load` + 100ms delay
- [ ] No user configuration needed for the default behavior
- [ ] "Capture from page start" toggle is OFF by default
- [ ] Toggle state persists across browser restarts

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Phase 2 installs after load + 100ms | Default deferral | `installPhase2()` called 100ms after `load` event | must |
| UT-2 | `deferralEnabled = false` installs Phase 2 immediately | "Capture from page start" ON | `installPhase2()` called at script load time | must |
| UT-3 | Phase 1 does NOT wrap console | After Phase 1, before Phase 2 | `console.log` is the original native function | must |
| UT-4 | Phase 1 does NOT wrap fetch | After Phase 1, before Phase 2 | `window.fetch` is the original native function | must |
| UT-5 | Phase 1 does NOT wrap WebSocket | After Phase 1, before Phase 2 | `window.WebSocket` is the original constructor | must |
| UT-6 | Phase 1 sets up PerformanceObservers | After Phase 1 | FCP and CLS observers are active | must |
| UT-7 | Phase 1 registers `__kaboom` API | After Phase 1 | `window.__kaboom` exists | must |
| UT-8 | Phase 1 records injection timestamp | After Phase 1 | `injectionTimestamp` is a valid `performance.now()` value | must |
| UT-9 | Phase 2 wraps console methods | After Phase 2 | `console.log`, `.warn`, `.error` are wrapped | must |
| UT-10 | Phase 2 wraps fetch | After Phase 2 | `window.fetch` is Kaboom's wrapper | must |
| UT-11 | Phase 2 wraps WebSocket constructor | After Phase 2 | `window.WebSocket` is Kaboom's wrapper | must |
| UT-12 | Phase 2 installs error handlers | After Phase 2 | `window.onerror` and `unhandledrejection` handlers installed | must |
| UT-13 | Phase 2 installs action capture | After Phase 2 | Click, input, scroll, keydown listeners installed | must |
| UT-14 | Phase 2 wraps navigation methods | After Phase 2 | `pushState`/`popstate` wrapped | must |
| UT-15 | 10-second timeout fallback | Page never fires `load` event | Phase 2 installs at 10s timeout | must |
| UT-16 | `document.readyState === 'complete'` at injection | Page already loaded when inject.js runs | Phase 2 installs immediately (+100ms) | must |
| UT-17 | Double injection guard | Phase 2 already installed, trigger again | Second call is a no-op | must |
| UT-18 | Retroactive WebSocket discovery | WebSocket opened before Phase 2 | Reported as "pre-existing connection" | should |
| UT-19 | `phase2Installed` flag set correctly | After Phase 2 runs | `phase2Installed === true` | must |
| UT-20 | `phase2Timestamp` recorded | After Phase 2 runs | Valid `performance.now()` value, greater than `injectionTimestamp` | must |
| UT-21 | `missedEvents` object populated | Default deferral active | `{ consoleLogs: true, fetchCalls: true, wsConnections: true }` | should |
| UT-22 | Settings message received from content.js | content.js sends `KABOOM_SETTINGS` | `deferralEnabled` updated from message | must |
| UT-23 | SPA navigation does not re-trigger deferral | SPA route change after Phase 2 | Interceptors stay active, no re-deferral | must |
| UT-24 | Console logs before Phase 2 not captured | Log before Phase 2 | Log does NOT appear in Kaboom's log buffer | must |
| UT-25 | Console logs after Phase 2 captured | Log after Phase 2 | Log appears in Kaboom's log buffer | must |
| UT-26 | Background.js stores deferral preference | Toggle "Capture from page start" | Value stored in `chrome.storage.local` | should |
| UT-27 | Background.js responds to `get_settings` | content.js sends `get_settings` | Response includes `deferral` boolean | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Default deferral end-to-end | inject.js Phase 1 -> load event -> Phase 2 | Console wrapping active only after load + 100ms | must |
| IT-2 | Settings flow from options to inject | options.js -> background.js -> content.js -> inject.js | Toggle change propagates to inject.js behavior | must |
| IT-3 | `get_page_info` includes deferral diagnostics | Server -> extension -> inject.js state | Response includes `injectionTimestamp`, `phase2Timestamp`, pre-existing WS count | should |
| IT-4 | Performance API retroactive fetch capture | Fetch before Phase 2, then query | Resource entry visible via Performance API | should |
| IT-5 | Full page load with deferral | Load a real web page | FCP/LCP/TTFB unaffected by Kaboom, Phase 2 installs after load | must |
| IT-6 | Full page load without deferral | "Capture from page start" ON | Phase 2 installs immediately, may affect FCP/LCP | should |
| IT-7 | Deferral toggle persistence | Set toggle, restart browser | Toggle value persists in `chrome.storage.local` | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Phase 1 overhead | Script execution time at injection | < 1ms | must |
| PT-2 | Phase 2 overhead | Interceptor installation time | < 5ms | must |
| PT-3 | FCP impact with deferral ON | Compare FCP with and without extension | No measurable difference (< 1ms) | must |
| PT-4 | LCP impact with deferral ON | Compare LCP with and without extension | No measurable difference (< 5ms) | must |
| PT-5 | TTFB impact | Compare TTFB with and without extension | Zero impact (TTFB is network, not JS) | must |
| PT-6 | PerformanceObserver overhead in Phase 1 | Observer callback latency | Zero overhead (browser-pushed) | should |
| PT-7 | Retroactive WebSocket discovery | `getEntriesByType` execution time | < 2ms | should |
| PT-8 | FCP impact with deferral OFF | Compare FCP with "Capture from page start" ON vs extension disabled | Measurable difference documents the benefit of deferral | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Page fires `load` before inject.js runs | Very fast page | `document.readyState === 'complete'` path: install deferred (+100ms) | must |
| EC-2 | Page never fires `load` (stuck resource) | Broken image blocks load event indefinitely | 10-second timeout installs Phase 2 | must |
| EC-3 | `about:blank` page | Navigate to about:blank | `load` fires immediately, Phase 2 installs at ~100ms | should |
| EC-4 | SPA navigation after Phase 2 | React Router navigation | Interceptors stay active, no re-deferral | must |
| EC-5 | Extension reloaded during Phase 1 wait | Reload extension before Phase 2 fires | `phase2Installed` guard prevents double-wrapping on new inject.js | should |
| EC-6 | Multiple inject.js loads | Extension dev reload | Guard prevents double Phase 2 installation | must |
| EC-7 | Very fast page (<50ms to load) | Simple static page | Phase 2 at ~150ms from nav start. Console logs in first 150ms lost. Acceptable tradeoff. | should |
| EC-8 | Worker contexts | Service worker or web worker | inject.js only runs in main window. Workers unaffected. | should |
| EC-9 | Content Security Policy restrictions | Page with strict CSP | CSP affects injection timing, not deferral behavior. If inject.js loads, deferral works normally. | should |
| EC-10 | Framework WebSocket during hydration | Socket.io connects during React hydration | With deferral: WebSocket constructor is NOT wrapped yet, no interference. Discovered retroactively. | must |
| EC-11 | Heavy page with many early scripts | 50+ scripts loading before `load` | Phase 1 has zero interference with script loading. Phase 2 installs after all are complete. | must |
| EC-12 | Toggle changed while page is loaded | User toggles "Capture from page start" | Takes effect on next page load/navigation. Current page unaffected. | should |
| EC-13 | Pre-existing WebSocket with auth token in URL | `wss://api.example.com?token=secret123` | URL reported via Performance API. Data stays on localhost. | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page that:
  - Calls `console.log("EARLY LOG")` before `DOMContentLoaded`
  - Calls `console.log("LATE LOG")` after `load` event + 200ms
  - Opens a WebSocket connection during page initialization
  - Makes a fetch request during page initialization
- [ ] Tab is being tracked by the extension
- [ ] "Capture from page start" toggle is OFF (default)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| **Default deferral behavior** | | | | |
| UAT-1 | Navigate to test page. `{"tool": "observe", "arguments": {"what": "logs"}}` | Console in browser shows both "EARLY LOG" and "LATE LOG" | Kaboom logs contain "LATE LOG" but NOT "EARLY LOG" (early log was before Phase 2) | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"what": "network"}}` | DevTools Network shows the early fetch request | Kaboom network buffer may not have the early fetch (depends on timing), but Performance API resource entries show it | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "websocket"}}` | DevTools shows WebSocket connection established during load | Kaboom reports the WebSocket as "pre-existing connection (not intercepted)" | [ ] |
| UAT-4 | Check page performance: `{"tool": "observe", "arguments": {"what": "performance"}}` | DevTools Performance tab shows FCP/LCP | Kaboom's FCP/LCP/TTFB metrics match DevTools values (deferral did not inflate them) | [ ] |
| UAT-5 | Check deferral diagnostics (if `get_page_info` is available) | N/A | Response includes `injectionTimestamp`, `phase2Timestamp`, deferral active flag | [ ] |
| **"Capture from page start" toggle** | | | | |
| UAT-6 | Human enables "Capture from page start" in extension options | Options page toggle | Toggle is ON | [ ] |
| UAT-7 | Navigate to test page again. `{"tool": "observe", "arguments": {"what": "logs"}}` | Both logs appear in browser console | Kaboom logs contain BOTH "EARLY LOG" AND "LATE LOG" (no deferral, immediate capture) | [ ] |
| UAT-8 | `{"tool": "observe", "arguments": {"what": "websocket"}}` | WebSocket connection established during load | Kaboom reports the WebSocket as an intercepted connection (NOT "pre-existing") | [ ] |
| UAT-9 | Check page performance | Compare FCP with deferral OFF | FCP may be slightly higher (expected -- Kaboom wrapping adds ~5ms during critical path) | [ ] |
| UAT-10 | Human disables "Capture from page start" (returns to default) | Options page toggle | Toggle is OFF (deferral re-enabled) | [ ] |
| **Timeout fallback** | | | | |
| UAT-11 | Load a page with a stuck resource (e.g., iframe loading forever) | Page never fires `load` event | After ~10 seconds, Kaboom starts capturing (Phase 2 timeout fallback). Late console logs appear in buffer. | [ ] |
| **SPA navigation** | | | | |
| UAT-12 | Navigate within an SPA (React Router, etc.) after initial load. `{"tool": "observe", "arguments": {"what": "logs"}}` | Console logs from SPA navigation | Logs captured normally (Phase 2 active, no re-deferral on SPA nav) | [ ] |
| **Toggle persistence** | | | | |
| UAT-13 | Human enables "Capture from page start", closes and reopens browser | Check options page after restart | Toggle is still ON (persisted in `chrome.storage.local`) | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Settings message contains only boolean | Inspect inject.js console for `KABOOM_SETTINGS` message | Message contains `{ type: 'KABOOM_SETTINGS', deferral: true/false }` only | [ ] |
| DL-UAT-2 | No external data transmission | Monitor all network traffic during page load with deferral | No requests to external servers from deferral logic | [ ] |
| DL-UAT-3 | Pre-existing WebSocket URLs on localhost only | WebSocket URL contains auth token | URL reported in diagnostics but only transmitted to 127.0.0.1:7890 | [ ] |
| DL-UAT-4 | Chrome.storage.local only stores boolean | Inspect `chrome.storage.local` for deferral key | Single boolean value, no additional data | [ ] |
| DL-UAT-5 | Early console logs NOT captured (privacy benefit) | Page logs PII during initialization | PII log NOT in Kaboom buffer (lost during deferral window) | [ ] |

### Regression Checks
- [ ] All existing Kaboom features work after Phase 2 installs (console capture, network capture, WebSocket monitoring, error capture)
- [ ] PerformanceObservers (FCP, LCP, CLS) still collect data from Phase 1 (not delayed to Phase 2)
- [ ] Extension popup shows correct connection status during both Phase 1 and Phase 2
- [ ] Action capture (clicks, inputs, scrolls) works after Phase 2 installs
- [ ] Navigation capture (`pushState`, `popstate`) works after Phase 2 installs
- [ ] No double-wrapping of any API if extension reloads during Phase 1 window
- [ ] Pages with strict CSP still work (deferral does not change injection mechanism)

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

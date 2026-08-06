---
status: shipped
scope: feature/interception-deferral/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-interception-deferral
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-interception-deferral.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Interception Deferral Review](./index.md).

# Technical Spec: Interception Deferral

## Purpose

Kaboom captures browser activity by wrapping native APIs — console, fetch, WebSocket, error handlers, and PerformanceObservers. Today, these wraps execute immediately when `inject.js` loads, which happens before or during the page's critical rendering path. On a fast page, this adds measurable overhead to First Contentful Paint. On a heavy page with dozens of early-loading scripts, the WebSocket constructor replacement can race with libraries that instantiate WebSocket connections during initialization (e.g., Socket.io, Phoenix LiveView, Supabase Realtime).

The core thesis demands that Kaboom be invisible to the app it observes. If the debugger perturbs the system, the AI observes artificial behavior and makes incorrect decisions. An AI that sees a 200ms FCP regression doesn't know whether the app got slower or whether Kaboom's interception added the latency.

Interception deferral solves this by postponing all heavy intercepts until after the page has finished its critical load, ensuring Kaboom never contributes to perceived slowness or initialization races.

---

## Opportunity & Business Value

**AI accuracy**: Performance observations (FCP, LCP, TTFB) made by the performance budget monitor are only trustworthy if Kaboom itself doesn't inflate them. Deferral ensures the AI's performance baselines reflect real app behavior, not instrumentation artifacts.

**Developer trust**: If developers notice their app loads slower with Kaboom installed, they disable it — eliminating the AI's eyes. A zero-impact extension stays installed permanently.

**Framework compatibility**: Modern frameworks initialize WebSocket connections, set up error boundaries, and call console.log during hydration. Wrapping these APIs before the framework is ready can cause subtle timing bugs (e.g., a WebSocket message arriving during React hydration when the handler isn't mounted yet). Deferral eliminates this entire class of issues.

**Competitive position**: Chrome DevTools, React DevTools, and Redux DevTools all affect page performance noticeably. A debug tool that provably adds zero overhead is a unique selling point for AI-assisted development workflows.

---

## How It Works

### Two-Phase Initialization

The inject script splits its work into two phases:

**Phase 1 (Immediate)**: Only lightweight, non-intercepting setup runs at script load time:
- Register the `window.__kaboom` API object (no interceptions, just a namespace)
- Set up the message listener for content script commands
- Start PerformanceObservers for paint timing and CLS (these are passive observers that don't wrap anything — the browser simply calls a callback, no prototype replacement needed)
- Record `performance.now()` as the injection timestamp (for later diagnostics)

**Phase 2 (Deferred)**: Heavy interceptors install after the page has loaded:
- Console method wrapping (replaces `console.log`, `.warn`, `.error`, etc.)
- Fetch wrapping (replaces `window.fetch`)
- WebSocket constructor replacement
- Error handler installation (`window.onerror`, `unhandledrejection`)
- Action capture (click, input, scroll, keydown event listeners)
- Navigation capture (`pushState`/`popstate` wrapping)

### Deferral Trigger

Phase 2 fires on the **later** of:
1. The `window.load` event
2. A 100ms delay after that event

This means: `window.addEventListener('load', () => setTimeout(install, 100))`.

The 100ms buffer accounts for late-firing scripts that initialize during or just after the load event (common in lazy-loaded bundles and async analytics). It also prevents Kaboom's interceptions from appearing in the browser's load waterfall.

If the page takes longer than 10 seconds to fire `load` (a timeout safeguard), Phase 2 installs anyway. This handles pathological cases like pages that never finish loading due to a stuck resource.

### Early Events Are Not Lost

Between Phase 1 and Phase 2, console logs, fetch calls, and WebSocket connections happen without Kaboom observing them. This is acceptable because:

1. **Console logs during load are usually noise** — framework initialization messages, HMR connection logs, etc. The noise filter would suppress most of them anyway.
2. **Fetch requests during load are captured by the Performance API** — `performance.getEntriesByType('resource')` retroactively provides all network requests that happened before fetch wrapping started. The performance snapshot collects these.
3. **WebSocket connections opened during load are retroactively discovered** — After Phase 2 installs, we scan `performance.getEntriesByType('resource')` for entries with `initiatorType === 'websocket'` to detect WebSocket connections we missed. For each discovered URL, we note it as "pre-existing connection (not intercepted)" in the WebSocket status report.
4. **Errors during load are captured by the PerformanceObserver** — Long tasks that caused errors will be visible in the long task data.

If a critical error occurs before Phase 2, it's still captured by the browser's native error console — and the AI can query it later via the `get_browser_errors` tool which reads from the captured error buffer.

### Configuration Escape Hatch

For users who need complete capture from the very first byte (e.g., debugging an initialization race), the extension options page exposes a "Capture from page start" toggle. When enabled, Phase 2 runs immediately (matching current behavior). This is OFF by default.

The toggle is communicated from the background script to the content script, which passes it to inject.js via a message. The inject script checks for this flag before deciding whether to defer.

---

## Data Model

### Extension State

New state in inject.js:
- `deferralEnabled`: Boolean (default true). Set to false by "Capture from page start" option.
- `phase2Installed`: Boolean. Tracks whether heavy interceptors are active.
- `injectionTimestamp`: Number. `performance.now()` at script load time.
- `phase2Timestamp`: Number. `performance.now()` when Phase 2 fires.
- `missedEvents`: Object tracking what was potentially missed: `{ consoleLogs: true, fetchCalls: true, wsConnections: true }`.

### MCP Visibility

The `get_page_info` tool response includes a `kaboom` section that reports:
- Whether deferral is active
- When injection occurred relative to page load
- When Phase 2 installed
- How many pre-existing WebSocket connections were discovered retroactively

This lets the AI understand the capture window and whether early events might be missing from the buffer.

---

## Extension Changes

### inject.js

The initialization block at the bottom of the file changes from:

```
if (typeof window !== 'undefined') {
  install()
  installKaboomAPI()
}
```

To:

```
if (typeof window !== 'undefined') {
  installPhase1()  // Lightweight: API, message listener, perf observers

  if (!deferralEnabled) {
    installPhase2()  // Immediate if configured
  } else {
    const installDeferred = () => setTimeout(installPhase2, 100)
    if (document.readyState === 'complete') {
      installDeferred()  // Already loaded
    } else {
      window.addEventListener('load', installDeferred, { once: true })
      setTimeout(installPhase2, 10000)  // Fallback timeout
    }
  }
}
```

### content.js

Passes the deferral preference from background to inject:

```
// On load, query background for deferral setting
chrome.runtime.sendMessage({ type: 'get_settings' }, (response) => {
  window.postMessage({ type: 'KABOOM_SETTINGS', deferral: response.deferral })
})
```

### background.js

Stores the "Capture from page start" preference in `chrome.storage.local`. Responds to `get_settings` messages.

---

## Edge Cases

- **SPA navigation**: After the initial deferred install, subsequent SPA navigations don't re-trigger deferral. Phase 2 stays active for the lifetime of the page.
- **Page with no `load` event** (e.g., `about:blank`, Chrome internal pages): The 10-second timeout ensures Phase 2 eventually installs. In practice, `about:blank` fires `load` immediately.
- **Multiple inject.js loads** (extension reload during development): The `phase2Installed` guard prevents double-wrapping.
- **Very fast pages** (<50ms to `load`): Phase 2 installs at load+100ms = ~150ms from navigation start. Console logs in the first 150ms are lost. This is an acceptable tradeoff — those logs are almost always framework noise.
- **Worker contexts**: `inject.js` only runs in the main window context. Service workers and web workers are unaffected.
- **Content Security Policy**: The existing injection mechanism (script element with `chrome.runtime.getURL`) is unaffected by deferral timing. CSP applies to the injection itself, not to when its code runs.

---

## Performance Constraints

- Phase 1 overhead: under 1ms (no prototype wrapping, just object assignment)
- Phase 2 overhead: under 5ms (same as current `install()`)
- No impact on FCP, LCP, or TTFB (Phase 2 fires after these are measured)
- PerformanceObservers in Phase 1 have zero overhead (browser-pushed callbacks)
- Retroactive WebSocket discovery: under 2ms (`getEntriesByType` is fast)

---

## Test Scenarios

1. Default behavior: Phase 2 installs after `load` event + 100ms delay
2. `deferralEnabled = false`: Phase 2 installs immediately (same as current behavior)
3. Console logs before Phase 2 → not captured (intentional)
4. Fetch calls before Phase 2 → visible in performance resource entries
5. WebSocket opened before Phase 2 → reported as "pre-existing" in status
6. Page takes 15 seconds to load → Phase 2 installs at 10s timeout
7. SPA navigation after Phase 2 → no re-deferral, interceptors stay active
8. Phase 1 doesn't modify console, fetch, or WebSocket prototypes
9. PerformanceObservers (FCP, LCP, CLS) are active from Phase 1
10. `get_page_info` includes deferral timing diagnostics
11. "Capture from page start" toggle persists across browser restarts
12. Double injection guard prevents Phase 2 from running twice
13. `document.readyState === 'complete'` at injection time → installs immediately (+ 100ms)

---

## File Locations

Extension implementation: Changes to `extension/inject.js`, `extension/content.js`, `extension/background.js`, `extension/options.js`.

Tests: `extension-tests/interception-deferral.test.js`.

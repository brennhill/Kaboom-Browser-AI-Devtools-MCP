---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Network Capture Documentation

**Last Updated:** 2026-01-30
**Version:** v5.2.5+

## Overview

Kaboom provides two complementary tools for observing network activity:

1. **`network_waterfall`** - Captures timing metadata for ALL network requests
2. **`network_bodies`** - Captures request/response bodies for `window.fetch()` calls only

---

## network_waterfall (Complete Coverage)

### What It Captures

**All network requests** made by the browser, including:
- ✅ Browser navigation (`window.location.href`, address bar)
- ✅ `window.fetch()` calls
- ✅ `XMLHttpRequest` (XHR) calls
- ✅ Resource loading (`<script src>`, `<img src>`, `<link href>`)
- ✅ Form submissions
- ✅ WebSocket connections (initial handshake)
- ✅ CSS `@import` and `url()` references
- ✅ Prefetch/preload requests

### What It Provides

**Metadata only** (no request/response bodies):
- URL, method, status code
- Timing breakdown (DNS, connection, TLS, download, etc.)
- Transfer size, content type
- Compression ratio
- Timestamps

### How It Works

Uses the browser's **Performance API** (`performance.getEntriesByType('resource')`), which records all network activity at the browser level.

### Use Cases

- Performance analysis (find slow requests)
- Network debugging (see all traffic)
- API discovery (identify endpoints)
- Dependency analysis (what resources are loaded)

### Example

```javascript
observe({what: "network_waterfall", limit: 50})
```

**Returns:** Markdown table with timing and metadata for all requests.

---

## network_bodies (Fetch-Only, Full Content)

### What It Captures

**ONLY `window.fetch()` calls** made by JavaScript code on the page.

### What It Does NOT Capture

- ❌ Browser navigation (address bar, `window.location.href = "..."`)
- ❌ `XMLHttpRequest` (XHR) calls
- ❌ Resources loaded by `<script>`, `<img>`, `<link>`, `<iframe>` tags
- ❌ Form submissions (`<form method="POST">`)
- ❌ WebSocket messages (use `websocket_events` instead)
- ❌ CSS resource loads

### Why This Limitation?

**Technical Constraint:** Kaboom injects scripts into **page context** (not extension context) to intercept JavaScript APIs. In page context:

- ✅ **Can wrap:** `window.fetch` (JavaScript function)
- ❌ **Cannot intercept:** Browser-level navigation, resource loading, form submissions

**Alternative approach (not used):** Chrome's `webRequest` API with `blocking` mode could capture all requests, but:
- Requires scary permissions (`webRequestBlocking`)
- Violates Chrome Web Store policies
- Slows down ALL network requests on ALL tabs
- Deprecated in Manifest V3 for privacy/performance
- Cannot access response bodies in MV3

### What It Provides

#### Full request and response content:
- Request: URL, method, headers, body (up to 100KB)
- Response: status, headers, body (up to 100KB)
- Timing: duration in milliseconds
- Content-Type metadata

### How It Works

Wraps the native `window.fetch()` function:

```typescript
// src/lib/net/network.ts
export function wrapFetchWithBodies(fetchFn: typeof fetch): any {
  return async function (input: any, init?: RequestInit): Promise<Response> {
    // Clone request/response to capture bodies
    // Send to background script for storage
    // Return original response to caller
  };
}
```

The wrapper:
1. Clones the `Request` to read the body without consuming it
2. Calls the original `fetch()` function
3. Clones the `Response` to read the body without consuming it
4. Sends captured data to background script
5. Returns the original `Response` to the page

### Use Cases

- API debugging (see request/response payloads)
- Schema extraction (analyze API contracts)
- Test generation (capture real API interactions)
- Mock generation (create fixtures from real responses)

### Example

```javascript
observe({what: "network_bodies", limit: 20})
```

**Returns:** JSON array with full request/response content for `fetch()` calls.

---

## Comparison Table

| Feature | network_waterfall | network_bodies |
|---------|-------------------|----------------|
| **Coverage** | ALL network requests | `fetch()` calls only |
| **Navigation** | ✅ Yes | ❌ No |
| **fetch()** | ✅ Metadata only | ✅ Full content |
| **XHR** | ✅ Metadata only | ❌ No |
| **Resources** | ✅ Metadata only | ❌ No |
| **Forms** | ✅ Metadata only | ❌ No |
| **Request bodies** | ❌ No | ✅ Yes (fetch only) |
| **Response bodies** | ❌ No | ✅ Yes (fetch only) |
| **Headers** | ❌ No | ✅ Yes (fetch only) |
| **Timing** | ✅ Detailed breakdown | ✅ Total duration |
| **Body size limit** | N/A | 100KB each |

---

## Common Pitfalls

### ❌ WRONG: Testing network_bodies by navigating to URLs

```javascript
// User navigates to example.com
observe({what: "network_bodies"})
// Returns: [] (empty - navigation is not a fetch() call)
```

**Why:** Browser navigation is not a `fetch()` call. The page load happens at the browser level, not via JavaScript.

### ✅ RIGHT: Testing network_bodies on SPAs that use fetch()

```javascript
// User navigates to a React/Vue/Angular app that makes fetch() calls
observe({what: "network_bodies"})
// Returns: [array of fetch() calls made by the app]
```

#### Examples of sites that use fetch():
- GitHub (loads comments, issues via API)
- Twitter/X (infinite scroll, timeline updates)
- Gmail (message loading)
- Any modern SPA (React, Vue, Angular, Svelte)

---

## Future Enhancements

### Planned for v5.3+

**XHR capture** - Wrap `XMLHttpRequest` to capture legacy AJAX calls:

```typescript
export function wrapXHRWithBodies() {
  const OriginalXHR = window.XMLHttpRequest;
  window.XMLHttpRequest = function() {
    const xhr = new OriginalXHR();
    // Intercept .open(), .send(), .onload, .onerror
    // Capture request/response bodies
    return xhr;
  };
}
```

**Estimated effort:** 4-6 hours
**Value:** Capture request/response bodies for legacy apps using XHR

### Not Planned (Architecture Constraints)

These would require Chrome Extension API changes or violate Web Store policies:

- ❌ Capture navigation request/response bodies
- ❌ Capture form submission bodies
- ❌ Capture resource loading bodies (`<script>`, `<img>`)
- ❌ Use `webRequest` blocking API (deprecated in MV3)

**Workaround:** Use Chrome DevTools Network tab alongside Kaboom for these cases.

---

## Testing network_bodies

### Good Test Sites (use fetch)

1. **JSONPlaceholder API** - `https://jsonplaceholder.typicode.com`
   - Open DevTools console
   - Run: `fetch('https://jsonplaceholder.typicode.com/posts/1').then(r => r.json()).then(console.log)`
   - Check: `observe({what: "network_bodies"})` should show the request

2. **GitHub** - `https://github.com`
   - Navigate to any repo
   - Scroll to comments section (triggers fetch() calls)
   - Check: `observe({what: "network_bodies"})` should show API calls

3. **Any React/Vue/Angular app**
   - Actions that trigger API calls (button clicks, form submissions handled by JS)
   - Check: `observe({what: "network_bodies"})` should capture the fetch() calls

### Bad Test Sites (don't use fetch)

1. **Static HTML sites** - No JavaScript, no fetch() calls
2. **Navigation-only testing** - Just loading URLs in address bar
3. **Old sites using XHR** - Won't be captured until v5.3+

---

## Reference

- **UAT Issue #4:** [UAT-ISSUES-TRACKER.md](../../core/in-progress/UAT-ISSUES-TRACKER.md#-issue-4-network_bodies-no-data-captured-documented)
- **Implementation:** [src/lib/net/network.ts](../../src/lib/net/network.ts) (lines 420-498)
- **MCP Tool:** `observe({what: "network_bodies"})`
- **Related Tools:** `observe({what: "network_waterfall"})`, `observe({what: "api"})`

---

## FAQ

### Q: Why don't I see any network_bodies data?
A: You're likely testing on a page that doesn't use `fetch()`. Try a modern SPA or run `fetch()` from DevTools console.

### Q: Can I capture XHR requests?
A: Not yet. XHR capture is planned for v5.3+.

### Q: Can I capture browser navigation?
A: No, browser navigation cannot be intercepted from page context. Use `network_waterfall` for navigation metadata.

### Q: Why not use webRequest API?
A: It requires scary permissions, violates Chrome policies, and is deprecated in Manifest V3.

### Q: What's the body size limit?
A: 100KB for both request and response bodies. Larger bodies are truncated.

### Q: How do I see ALL network traffic?
A: Use `observe({what: "network_waterfall"})` for metadata on all requests, or Chrome DevTools Network tab for full details.

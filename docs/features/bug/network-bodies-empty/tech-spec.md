---
feature: network-bodies-empty
status: in-progress
doc_type: tech-spec
feature_id: bug-network-bodies-empty
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Network Bodies Empty (Bug Fix)

> Plain language only. No code. Describes HOW to fix the empty network bodies issue.

## Architecture Overview

Kaboom captures network traffic through fetch/XHR interception in inject.js:
1. Extension intercepts fetch() and XMLHttpRequest calls in page context
2. For each request, captures metadata (url, method, headers) immediately
3. For completed responses, optionally captures response body
4. Body data is posted to server's /network-bodies endpoint
5. Server stores bodies in a ring buffer (100 entries, 8MB max)
6. MCP tool `observe({what: "network_bodies"})` retrieves from ring buffer

**The Bug:** Step 3 or 4 is not happening. Bodies are either not being captured by the extension, or not being posted to the server, or not being stored in the ring buffer.

## Key Components

- **inject.js:** Fetch/XHR interceptors that should capture response bodies
- **Network body capture toggle:** Configuration flag that may default to OFF
- **Body serialization:** Logic to read and serialize response bodies (text, JSON)
- **/network-bodies POST endpoint:** Server endpoint that receives body data
- **Network bodies ring buffer:** In-memory storage (100 entries, 8MB limit)
- **observe tool handler:** Code that retrieves bodies from ring buffer

## Data Flows

### Current (Broken) Flow - Hypothesis 1: Capture Not Enabled
```
Page makes fetch() → inject.js intercepts → body capture flag is OFF
→ Body not read from response → No POST to /network-bodies
→ Ring buffer stays empty → observe returns empty array
```

### Current (Broken) Flow - Hypothesis 2: Capture Enabled But Bodies Not Stored
```
Page makes fetch() → inject.js intercepts → body capture enabled
→ Body read from response → POST to /network-bodies fails or is not sent
→ Ring buffer stays empty → observe returns empty array
```

### Current (Broken) Flow - Hypothesis 3: Bodies Stored But Not Retrieved
```
Page makes fetch() → inject.js intercepts → body captured and posted
→ Server receives body → Stored in wrong buffer or wrong key
→ observe queries wrong buffer → returns empty array
```

### Fixed Flow
```
Page makes fetch() → inject.js intercepts → body capture enabled (default or configured)
→ Body read from response → POST to /network-bodies with body data
→ Server stores in network bodies ring buffer → observe retrieves from correct buffer
→ Returns body data to user
```

## Implementation Strategy

### Step 1: Verify Body Capture Is Enabled
Check the configuration state:
- Look for a body capture toggle in extension settings or server config
- Check if it defaults to ON or OFF
- If OFF by default, determine if this is intentional (privacy, performance)
- If OFF, either change default to ON or document how to enable
- Verify the toggle state is accessible to inject.js intercept logic

### Step 2: Trace Body Capture Logic in inject.js
Follow the fetch/XHR interception code:
- Locate where response bodies should be read
- Check if body reading is conditional (gated by flag or response type)
- Verify body is read BEFORE the response is consumed by the app (clone response first)
- Check if body reading handles different response types (text, JSON, blob)
- Verify errors during body read don't silently fail (log errors for debugging)

### Step 3: Verify Body Posting to Server
Check the HTTP POST logic:
- Confirm bodies are posted to `http://127.0.0.1:7890/network-bodies` (or configured server URL)
- Verify POST payload format matches server expectation (JSON with url, method, status, body fields)
- Check if POST happens immediately or in batches
- Verify POST errors are caught and logged (if server unreachable, don't crash)
- Check for self-capture prevention (don't capture bodies for requests to Kaboom server itself)

### Step 4: Verify Server Storage
Check the server-side handler:
- Confirm `/network-bodies` POST endpoint exists and handles body storage
- Verify data is stored in the network bodies ring buffer (not a different buffer)
- Check ring buffer initialization (must be created at startup)
- Verify ring buffer size limits enforced (100 entries, 8MB total)
- Check oldest-first eviction works correctly

### Step 5: Verify Retrieval Logic
Check the observe tool handler:
- Confirm `observe({what: "network_bodies"})` queries the correct ring buffer
- Verify returned data format matches schema (entries array with url, method, status, body)
- Check if retrieval is filtered (by time, by status code, by content type)
- Verify empty array is only returned when buffer is actually empty (not a query error)

### Step 6: Add Diagnostic Logging
Improve debuggability:
- Log when body capture is enabled/disabled
- Log when body is successfully read from response
- Log when body POST to server succeeds/fails
- Log when body is stored in ring buffer
- Log when observe queries return empty (why?)

## Edge Cases & Assumptions

### Edge Case 1: Binary Response Bodies
**Handling:** Binary responses (images, PDFs, videos) should be skipped (not captured). Check Content-Type header; only capture text/*, application/json, application/xml. For binary types, log "binary content skipped" and don't store.

### Edge Case 2: Large Response Bodies
**Handling:** Bodies larger than 5KB should be truncated. Read up to 5120 characters, set `truncated: true` flag in stored data. This prevents memory exhaustion.

### Edge Case 3: Body Already Consumed
**Handling:** Response bodies can only be read once. Use `response.clone()` before reading body to avoid breaking the application. If clone fails, skip body capture and log error.

### Edge Case 4: Streaming Responses
**Handling:** Streaming responses (chunked transfer, incremental HTTP streams) may not have a complete body to capture. Capture what's available at request completion or skip with a note.

### Edge Case 5: CORS Opaque Responses
**Handling:** Opaque responses (no-cors mode) don't expose body or headers. Skip body capture for opaque responses; log "opaque response, body not accessible".

### Edge Case 6: Network Errors
**Handling:** Failed requests (network error, timeout) don't have response bodies. Don't attempt to capture bodies for these; they won't be in the ring buffer.

### Assumption 1: Body Capture Defaults to OFF
We assume body capture is OFF by default for privacy/performance. Verify this assumption and document how to enable.

### Assumption 2: Only Completed Requests Captured
We assume bodies are captured only after the request fully completes. In-flight requests have no body yet.

### Assumption 3: Capture Happens in Page Context
We assume inject.js runs in page context and has access to fetch() and XHR before the app uses them. This requires interception to run early.

## Risks & Mitigations

### Risk 1: Capturing Bodies Breaks Applications
**Mitigation:** Use `response.clone()` to read body without consuming the original. Test on multiple applications to ensure no breakage. If cloning fails, skip capture gracefully.

### Risk 2: Memory Exhaustion from Large Bodies
**Mitigation:** Enforce strict truncation (5KB per body) and ring buffer size limits (8MB total, 100 entries). Drop oldest entries when limits exceeded.

### Risk 3: Sensitive Data in Response Bodies
**Mitigation:** Body capture must be opt-in (OFF by default). Document that response bodies may contain PII. Apply header redaction rules (Authorization, Cookie). Consider adding body redaction patterns.

### Risk 4: Performance Impact from Body Cloning
**Mitigation:** Cloning responses adds CPU cost (5-10ms per request). Test on pages with high request volume (100+ requests). If performance degrades, keep body capture OFF by default.

### Risk 5: POST to Server Fails Silently
**Mitigation:** Log errors when POST to /network-bodies fails. Don't block the page's network activity. Buffer bodies locally and retry or drop if server is unreachable.

## Dependencies

- **Existing:** Fetch/XHR interception in inject.js (likely already exists for network waterfall)
- **Existing:** /network-bodies POST endpoint on server (may exist but unused)
- **Existing:** Network bodies ring buffer in server memory (may not be initialized)
- **New:** Body capture toggle (may need to be added to config)
- **New:** Response cloning logic to avoid breaking app
- **New:** Body truncation logic for large responses

## Performance Considerations

- Body reading time: 1-10ms per request (depends on body size)
- Body cloning time: 1-5ms per request (Response.clone() cost)
- Body serialization: 1-10ms (converting blob/stream to text)
- POST to server: 5-20ms (HTTP overhead)
- Memory per body: 500 bytes - 5KB (after truncation)
- Total ring buffer memory: 8MB max (enforced)
- Impact on page load: < 50ms total for typical page with 20 requests

## Security Considerations

- **Body Capture Opt-In:** Must default to OFF to avoid capturing sensitive data without user consent. Provide clear documentation on how to enable and privacy implications.
- **Sensitive Header Redaction:** Authorization, Cookie, Set-Cookie headers must be redacted before storage (already done for headers; ensure bodies don't leak tokens in JSON).
- **Body Redaction Patterns:** Consider adding configurable patterns to redact tokens, API keys, passwords from response bodies (similar to noise filtering).
- **Localhost-Only:** Server binds to 127.0.0.1 only. Bodies never leave localhost (maintains existing security boundary).
- **No Body Modification:** Captured bodies are read-only copies. No risk of modifying application behavior.
- **Binary Content Skipped:** Binary files (images, PDFs) not captured. Reduces risk of capturing unintended data and saves memory.

## Test Plan Reference

See qa-plan.md for detailed testing strategy. Key test scenarios:
1. Bodies are returned after enabling capture
2. Bodies are empty when capture is disabled (with clear message)
3. Large bodies are truncated correctly
4. Binary responses are skipped
5. Response cloning doesn't break application
6. Sensitive headers in body data are redacted
7. Ring buffer size limits enforced
8. Regression: network waterfall still works

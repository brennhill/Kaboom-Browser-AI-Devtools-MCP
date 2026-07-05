---
feature: network-bodies-empty
doc_type: qa-plan
feature_id: bug-network-bodies-empty
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Network Bodies Empty (Bug Fix)

> How to test the network bodies empty bug fix. Includes code-level testing and human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Body capture logic
- [ ] Response body is read when capture is enabled
- [ ] Response body is NOT read when capture is disabled
- [ ] Response.clone() used before reading body
- [ ] Body truncation works (> 5KB bodies truncated at 5120 chars)
- [ ] Binary responses skipped (Content-Type: image/png)
- [ ] JSON responses captured correctly
- [ ] Text responses captured correctly
- [ ] HTML responses captured correctly
- [ ] POST to /network-bodies includes url, method, status, headers, body
- [ ] POST to server succeeds when server is reachable
- [ ] POST failure handled gracefully when server unreachable

**Integration tests:** End-to-end body capture
- [ ] fetch() request → body captured → stored in ring buffer → retrieved via observe
- [ ] XMLHttpRequest → body captured → stored → retrieved
- [ ] Multiple requests → all bodies captured up to ring buffer limit (100 entries)
- [ ] Oldest bodies evicted when buffer full
- [ ] observe({what: "network_bodies"}) returns correct data format
- [ ] Headers redacted (Authorization: [REDACTED])
- [ ] Body capture toggle: ON → bodies captured, OFF → bodies not captured

**Edge case tests:** Error scenarios
- [ ] Opaque response (no-cors) → body not captured, no error
- [ ] Network error (failed request) → no body to capture
- [ ] Streaming response → partial or no body captured
- [ ] Body clone fails → capture skipped, app still works
- [ ] Very large body (1MB) → truncated correctly
- [ ] Concurrent requests (50+) → all bodies captured without race conditions
- [ ] Self-capture prevention: requests to Kaboom server not captured

### Security/Compliance Testing

**Data leak tests:** Verify sensitive data redacted
- [ ] Authorization header shows [REDACTED] in captured headers
- [ ] Cookie header shows [REDACTED] in captured headers
- [ ] Response body containing "Bearer token" captured as-is (body redaction not yet implemented, but documented)

#### Privacy tests:
- [ ] Body capture defaults to OFF (opt-in for privacy)
- [ ] Clear message when capture disabled: "Network body capture is disabled..."
- [ ] Documentation warns that bodies may contain PII

---

## Human UAT Walkthrough

### Scenario 1: Enable Body Capture and Verify Data
1. Setup:
   - Start Kaboom server: `./dist/kaboom`
   - Load Chrome with extension
   - Enable body capture via: `configure({action: "network_body_capture", enabled: true})` (or default ON if changed)
   - Navigate to <https://jsonplaceholder.typicode.com/users>
2. Steps:
   - [ ] Page loads and makes GET request to API
   - [ ] Call MCP tool: `observe({what: "network_bodies"})`
   - [ ] Observe response
3. Expected Result: Response contains:
   ```json
   {
     "entries": [
       {
         "url": "https://jsonplaceholder.typicode.com/users",
         "method": "GET",
         "status": 200,
         "responseBody": "[{\"id\":1,\"name\":\"Leanne Graham\",...}]",
         "truncated": false
       }
     ]
   }
   ```
4. Verification:
   - [ ] entries array is NOT empty
   - [ ] responseBody contains actual JSON data
   - [ ] url and method populated correctly

### Scenario 2: Body Capture Disabled (Default)
1. Setup:
   - Start Kaboom server
   - Load Chrome with extension
   - DO NOT enable body capture (should default to OFF)
   - Navigate to https://jsonplaceholder.typicode.com/users
2. Steps:
   - [ ] Page loads and makes API requests
   - [ ] Call MCP tool: `observe({what: "network_bodies"})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "entries": [],
     "message": "Network body capture is disabled. Enable via configure({action: 'network_body_capture', enabled: true})"
   }
   ```
4. Verification: Empty array with clear instruction on how to enable

### Scenario 3: Large Body Truncation
1. Setup: Enable body capture, navigate to endpoint with large response (> 5KB)
2. Steps:
   - [ ] Make request to large data endpoint
   - [ ] Call MCP tool: `observe({what: "network_bodies"})`
   - [ ] Observe response
3. Expected Result:
   - [ ] responseBody length is 5120 characters (truncated)
   - [ ] truncated field is true
   - [ ] Body content is valid (not corrupted mid-character)
4. Verification: Large bodies don't cause memory issues

### Scenario 4: Binary Response Skipped
1. Setup: Enable body capture, navigate to page with images
2. Steps:
   - [ ] Page loads image: <https://example.com/image.png>   - [ ] Call MCP tool: `observe({what: "network_bodies"})`
   - [ ] Observe response
3. Expected Result:
   - [ ] No entry for image.png in bodies (or entry with body: null and note: "binary content skipped")
4. Verification: Binary content not captured (saves memory, avoids garbage)

### Scenario 5: Error Response Body Captured
1. Setup: Enable body capture, make request to non-existent endpoint
2. Steps:
   - [ ] Trigger 404 error: fetch('/nonexistent')
   - [ ] Call MCP tool: `observe({what: "network_bodies"})`
   - [ ] Observe response
3. Expected Result:
   - [ ] Entry with status: 404
   - [ ] responseBody contains error message: {"error": "Not found"}
4. Verification: Error responses captured (helpful for debugging)

### Scenario 6: Multiple Requests Captured
1. Setup: Enable body capture, navigate to page with many API calls
2. Steps:
   - [ ] Navigate to complex SPA (e.g., GitHub.com)
   - [ ] Wait for multiple requests to complete
   - [ ] Call MCP tool: `observe({what: "network_bodies"})`
   - [ ] Observe response
3. Expected Result:
   - [ ] entries array has multiple items (up to 100)
   - [ ] Each entry has unique url
   - [ ] Bodies are from most recent requests (ring buffer)
4. Verification: Ring buffer captures multiple requests

### Scenario 7: Sensitive Headers Redacted
1. Setup: Enable body capture, make authenticated request
2. Steps:
   - [ ] Trigger fetch with Authorization header
   - [ ] Call MCP tool: `observe({what: "network_bodies"})`
   - [ ] Observe response
3. Expected Result:
   - [ ] requestHeaders.Authorization shows [REDACTED]
   - [ ] responseHeaders.Set-Cookie (if present) shows [REDACTED]
4. Verification: Sensitive headers not leaked in body captures

---

## Regression Testing

### Must Not Break

- [ ] Network waterfall still works (`observe({what: "network_waterfall"})`)
- [ ] Page navigation still works
- [ ] Fetch/XHR requests in application still function correctly
- [ ] No JavaScript errors in console from response cloning
- [ ] No memory leaks after 100+ requests
- [ ] Extension startup time not significantly impacted

### Regression Test Steps

1. Disable body capture, verify waterfall still works
2. Enable body capture, make 200 requests, verify no memory leak
3. Test on sites with heavy network traffic (e.g., Twitter, GitHub)
4. Verify no "Failed to execute 'clone'" errors in console
5. Test with applications that consume response bodies (ensure cloning doesn't break)

---

## Performance/Load Testing

### Body capture overhead:
- [ ] fetch() with body capture: < 10ms overhead vs without
- [ ] XMLHttpRequest with body capture: < 10ms overhead vs without
- [ ] Body cloning time: < 5ms per response

### Ring buffer operations:
- [ ] Store 100 bodies: < 100ms total
- [ ] Retrieve 100 bodies via observe: < 200ms
- [ ] Eviction of oldest entry: < 1ms

### Memory usage:
- [ ] 100 bodies (average 2KB each): ~200KB memory
- [ ] Ring buffer max size: 8MB enforced
- [ ] No unbounded growth after 500 requests

### High-volume test:
- [ ] Page with 100 concurrent requests
- [ ] All bodies captured without loss
- [ ] No application slowdown
- [ ] Extension remains responsive

---

## Configuration Testing

### Body capture toggle:
- [ ] Default state is OFF (privacy-first)
- [ ] Enable via `configure({action: "network_body_capture", enabled: true})`
- [ ] Disable via `configure({action: "network_body_capture", enabled: false})`
- [ ] Toggle state persists across page navigations
- [ ] Toggle state resets on extension reload (session-only)

### Server endpoint:
- [ ] /network-bodies POST endpoint exists
- [ ] Accepts JSON payload with url, method, status, headers, body
- [ ] Returns 200 OK on successful storage
- [ ] Handles missing fields gracefully

### Ring buffer limits:
- [ ] Maximum 100 entries enforced
- [ ] Maximum 8MB total memory enforced
- [ ] Oldest entries evicted when limits reached
- [ ] observe returns most recent entries first

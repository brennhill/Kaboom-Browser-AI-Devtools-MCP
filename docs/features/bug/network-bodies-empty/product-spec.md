---
feature: network-bodies-empty
status: in-progress
tool: observe
mode: network_bodies
version: 0.7.12
doc_type: product-spec
feature_id: bug-network-bodies-empty
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Network Bodies Empty (Bug Fix)

## Problem Statement

Users calling `observe({what: "network_bodies"})` receive empty arrays despite the browser making network requests that should have been captured. The network waterfall shows requests occurred, but the response bodies are not available.

### Current User Experience:
1. User navigates to a page that makes API calls
2. User calls `observe({what: "network_waterfall"})` and sees requests listed
3. User calls `observe({what: "network_bodies"})` to get response bodies
4. Receives empty array: `{entries: []}`
5. User assumes no network traffic occurred or bodies weren't captured
6. User cannot diagnose API issues or analyze response data

**Root Cause:** Network body capture may be:
- Not enabled by default (opt-in flag not set)
- Enabled but bodies not being stored in the ring buffer
- Stored but not being retrieved correctly
- Filtered out (only 4xx/5xx responses captured, or only specific content types)

## Solution

Fix the network body capture pipeline so that response bodies are actually captured, stored, and returned when requested. Ensure the capture mechanism is properly initialized and body data flows from the extension through to the server's ring buffer.

### Fixed User Experience:
1. User navigates to a page that makes API calls
2. User calls `observe({what: "network_bodies"})`
3. Receives actual response bodies for network requests
4. User can analyze API responses, diagnose issues, verify data shape

## Requirements

1. **Verify Capture Enablement:** Ensure network body capture is enabled (check default config or require explicit opt-in)
2. **Trace Capture Pipeline:** Follow data flow from fetch/XHR interception → extension → server → ring buffer
3. **Fix Storage:** Ensure bodies are stored in the network bodies ring buffer (max 100 entries, 8MB limit)
4. **Fix Retrieval:** Ensure `observe({what: "network_bodies"})` queries the correct ring buffer
5. **Content Type Filtering:** Document which content types are captured (JSON, text, HTML) vs skipped (binary, images)
6. **Status Code Filtering:** Clarify whether all responses are captured or only errors (4xx/5xx)
7. **Schema Compliance:** Results must match the existing network_bodies response schema
8. **Backward Compatibility:** Fix must not break existing network waterfall functionality

## Out of Scope

- Adding new body capture capabilities (e.g., request bodies, binary bodies)
- Changing the network body capture schema
- Performance optimizations beyond fixing the empty results
- Increasing ring buffer size or memory limits
- Real-time body streaming (bodies captured at request completion)

## Success Criteria

1. `observe({what: "network_bodies"})` returns actual response bodies for captured requests
2. Bodies include url, method, status, headers, and body content
3. Body content is properly decoded (not base64 or garbled)
4. Truncation works correctly (bodies > 5KB truncated with indicator)
5. Sensitive headers are redacted (Authorization, Cookie)
6. Ring buffer enforces size limits (100 entries, 8MB total)
7. Clear documentation on which requests are captured (all? only errors? opt-in?)

## User Workflow

### Before Fix:
1. User makes API request via browser
2. User calls `observe({what: "network_bodies"})`
3. Receives empty array
4. User cannot diagnose API issues

### After Fix:
1. User makes API request via browser
2. User calls `observe({what: "network_bodies"})`
3. Receives response bodies with full data
4. User can analyze response structure, debug API issues

## Examples

### Example 1: Successful API Response Body

#### Request:
```json
{
  "tool": "observe",
  "arguments": {
    "what": "network_bodies"
  }
}
```

#### Before Fix Response:
```json
{
  "entries": []
}
```

#### After Fix Response:
```json
{
  "entries": [
    {
      "url": "https://api.example.com/users",
      "method": "GET",
      "status": 200,
      "statusText": "OK",
      "timestamp": "2026-01-28T10:00:00Z",
      "requestHeaders": {
        "Accept": "application/json",
        "Authorization": "[REDACTED]"
      },
      "responseHeaders": {
        "Content-Type": "application/json; charset=utf-8",
        "Content-Length": "1234"
      },
      "responseBody": "{\"users\": [{\"id\": 1, \"name\": \"Alice\"}, {\"id\": 2, \"name\": \"Bob\"}]}",
      "truncated": false
    }
  ]
}
```

### Example 2: Error Response Body

**Request:** Same as Example 1

#### After Fix Response:
```json
{
  "entries": [
    {
      "url": "https://api.example.com/posts/999",
      "method": "GET",
      "status": 404,
      "statusText": "Not Found",
      "timestamp": "2026-01-28T10:05:00Z",
      "requestHeaders": {
        "Accept": "application/json"
      },
      "responseHeaders": {
        "Content-Type": "application/json"
      },
      "responseBody": "{\"error\": \"Post not found\"}",
      "truncated": false
    }
  ]
}
```

### Example 3: Truncated Large Response

**Request:** Same as Example 1

#### After Fix Response:
```json
{
  "entries": [
    {
      "url": "https://api.example.com/large-data",
      "method": "GET",
      "status": 200,
      "statusText": "OK",
      "timestamp": "2026-01-28T10:10:00Z",
      "responseHeaders": {
        "Content-Type": "application/json",
        "Content-Length": "50000"
      },
      "responseBody": "{\"data\": [... 5120 characters ...]",
      "truncated": true
    }
  ]
}
```

### Example 4: No Bodies Captured (When Feature Not Enabled)

**Request:** Same as Example 1

#### Response (If body capture is opt-in and not enabled):
```json
{
  "entries": [],
  "message": "Network body capture is disabled. Enable via configure({what: 'network_body_capture', enabled: true})"
}
```

---

## Notes

- Network body capture may be OFF by default to reduce memory usage and avoid capturing sensitive data
- Body capture configuration may be per-session or global
- Bodies are only captured for completed requests (not in-flight)
- Binary responses (images, PDFs) may be excluded from capture
- The extension's fetch/XHR interceptors must capture response bodies before they're consumed by the application
- Ring buffer limits: 100 entries, 8MB total (oldest evicted first)
- Related specs: See network waterfall feature for request capture architecture

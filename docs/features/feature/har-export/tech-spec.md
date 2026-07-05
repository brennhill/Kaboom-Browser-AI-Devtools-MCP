---
status: proposed
scope: feature/har-export/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-har-export
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-har-export.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Har Export Review](har-export-review.md).

# Technical Spec: HAR Export

## Purpose

Kaboom captures network request and response bodies, timing data, and headers during development. Today this data is only accessible through MCP tools — the AI reads it, the AI acts on it. But sometimes the human needs to inspect the raw network traffic: replaying a failed API call, sharing a bug report with a backend team, profiling waterfall timing in Charles Proxy, or importing into Postman for manual testing.

HAR (HTTP Archive) is the universal interchange format for HTTP traffic. Every browser DevTools panel exports it, every proxy tool imports it, and every performance analysis tool understands it. By exporting Kaboom's captured network data as HAR, the human gets a portable artifact they can open in any tool they already know.

This is the bridge between "AI observed something" and "human can verify it independently."

---

## Opportunity & Business Value

**Universal tool compatibility**: HAR is supported by Chrome DevTools (import/export), Firefox DevTools, Safari Web Inspector, Charles Proxy, Fiddler, mitmproxy, Postman, Insomnia, k6 (load testing), WebPageTest, Lighthouse, GTmetrix, and dozens more. One format, every tool.

**Bug reproduction across teams**: When the AI identifies a backend issue ("API /users returns 500 intermittently"), exporting a HAR file lets the backend team see exactly what was sent and received — headers, body, timing — without needing Kaboom or the frontend running. Attach the HAR to a Jira ticket and the backend engineer opens it in their preferred tool.

**Performance profiling handoff**: The AI detects "load time regressed by 800ms" and the causal diff shows a slow API response. The developer exports a HAR covering the slow page load, opens it in WebPageTest or Charles Proxy, and sees the exact waterfall with server-side timing breakdown. The AI's abstract finding becomes a concrete, inspectable artifact.

**Load testing seed data**: Tools like k6, Gatling, and Locust can replay HAR files as load test scenarios. A captured HAR of a user flow (login → dashboard → edit) becomes the basis for realistic load tests without manually scripting each request.

**Compliance and audit trails**: Financial and healthcare applications sometimes require network traffic logs for audit purposes. HAR provides a standardized, timestamped record of all API communications during a development session.

**Postman collection generation**: Postman can import HAR files directly, creating a collection of all observed API calls with their headers and bodies. This auto-generates API documentation from actual usage — no manual endpoint listing needed.

---

## How It Works

### HAR Structure

The HAR 1.2 specification defines a JSON format with:
- `log.version`: "1.2"
- `log.creator`: Tool name and version
- `log.entries`: Array of HTTP transactions, each containing:
  - `request`: method, URL, headers, query params, body, cookies
  - `response`: status, statusText, headers, body, content type
  - `timings`: blocked, dns, connect, send, wait, receive, ssl
  - `startedDateTime`: ISO 8601 timestamp
  - `time`: Total elapsed time in ms
  - `pageref`: Optional page reference for grouping

### Mapping Kaboom Data to HAR

Kaboom's network body capture stores:
- Request URL, method, headers (sanitized)
- Request body (for POST/PUT/PATCH)
- Response status, headers
- Response body (truncated at capture limit)
- Timestamp
- Duration (from Performance API resource timing)

The HAR export maps these directly:

| Kaboom Field | HAR Field |
|---------------|-----------|
| URL | `request.url` |
| Method | `request.method` |
| Request headers | `request.headers` (excluding sanitized auth headers — noted as redacted) |
| Request body | `request.postData.text` |
| Response status | `response.status` |
| Response headers | `response.headers` |
| Response body | `response.content.text` |
| Content-Type | `response.content.mimeType` |
| Duration | Distributed across `timings` fields |
| Timestamp | `startedDateTime` |

### Timing Distribution

Kaboom's network capture provides total duration from the Performance API's `resource timing`. The HAR format expects granular timing breakdown (DNS, connect, SSL, etc.). Two approaches:

1. **If resource timing entries are available** (captured in the performance snapshot): Use `domainLookupEnd - domainLookupStart` for DNS, `connectEnd - connectStart` for connect, `responseStart - requestStart` for wait, etc.

2. **If only total duration is available**: Report the full duration as `wait` (server response time) and set other phases to 0. This is acceptable — many HAR importers only use total time anyway.

The extension enhances the network body POST to include resource timing fields when available (matched by URL and timestamp proximity).

### MCP Tool: `export_har`

**Parameters**:
- `output_path` (optional): File path to write the HAR file. If omitted, returns the HAR JSON in the response (warning: can be large).
- `url_filter` (optional): Regex to filter which URLs to include (e.g., `/api/` to export only API calls).
- `method_filter` (optional): HTTP method(s) to include (e.g., `["POST", "PUT"]`).
- `status_filter` (optional): Status code range (e.g., `{ "min": 400, "max": 599 }` for only errors).
- `time_range` (optional): `{ "start": "ISO8601", "end": "ISO8601" }` to limit the export window.
- `include_bodies` (optional, boolean, default true): Include request/response bodies. Set to false for a smaller file focusing on timing.
- `redact_headers` (optional, array): Additional headers to redact beyond the default sensitive set.

**Response** (when output_path specified):
```
{
  "file_path": "/path/to/project/.kaboom/reports/capture-2026-01-24.har",
  "entry_count": 47,
  "total_size_bytes": 284000,
  "time_range": { "start": "2026-01-24T10:28:00Z", "end": "2026-01-24T10:35:22Z" },
  "summary": "47 requests exported (12 API, 8 scripts, 5 stylesheets, 22 other). Total payload: 2.1MB captured."
}
```

### Privacy & Security

HAR files contain sensitive data by nature (request bodies, cookies, auth tokens). Kaboom's export applies the same sanitization used for in-memory storage:

1. **Authorization headers**: Replaced with `[REDACTED]` in the HAR output
2. **Cookie values**: Replaced with `[REDACTED]` (cookie names preserved for debugging)
3. **Common auth patterns**: Bearer tokens, API keys in query params, session IDs — all redacted
4. **Custom redaction**: The `redact_headers` parameter allows additional headers to be stripped

A warning is included in the MCP response if the HAR might contain sensitive data that wasn't caught by automatic redaction: "Review the HAR file before sharing — response bodies may contain user-specific data."

---

## Data Model

### HAR Entry Construction

Each network body in Kaboom's buffer becomes one HAR entry. The construction:

1. Parse the stored request/response data
2. Build the `request` object with method, URL, headers, cookies, query params, body
3. Build the `response` object with status, headers, content
4. Compute timings from resource timing (or fallback)
5. Set `startedDateTime` from the capture timestamp
6. Compute `time` as sum of all timing phases

### Pages Section

If multiple page loads occurred during the capture window, the HAR includes a `pages` section grouping entries by page load. Each page has:
- `id`: Sequential identifier
- `title`: Page URL
- `startedDateTime`: Navigation start time
- `pageTimings`: `onLoad` and `onContentLoad` from navigation timing

Entries reference their page via `pageref`. This allows HAR viewers to display per-page waterfalls.

---

## Edge Cases

- **No network bodies captured**: Returns an empty HAR (valid format, zero entries) with a note.
- **Response body truncated** (hit capture size limit): HAR entry includes `response.content.comment: "Body truncated at 100KB capture limit"` and `response.content.size` reflects the original size while `response.content.text` contains the truncated content.
- **Binary response body**: Encoded as base64 in `response.content.text` with `response.content.encoding: "base64"`.
- **Very large export** (>10MB): A warning is returned suggesting the use of filters to reduce size.
- **WebSocket upgrade requests**: Included as regular HTTP entries (the upgrade handshake). The WebSocket messages themselves are NOT in the HAR (HAR doesn't define WebSocket message format). A note suggests using `get_websocket_events` for WS message data.
- **CORS preflight requests**: Included if captured (OPTIONS requests with CORS headers).
- **Redirects**: Each redirect is a separate HAR entry. The `response.redirectURL` field links them.
- **Data URLs**: Excluded (not real network requests).
- **Cached responses** (status 304): Included with the 304 status and empty body.

---

## Performance Constraints

- HAR generation: under 50ms for 100 entries (JSON marshaling + timing computation)
- File write: under 100ms for a 5MB HAR file
- Memory during generation: temporary copy of entries for formatting — max 10MB
- No impact on page performance (server-side operation)
- Filtering reduces generation time proportionally

---

## Test Scenarios

1. Single GET request → valid HAR with correct method, URL, status
2. POST with body → request.postData contains body text
3. Response with JSON body → content.mimeType is "application/json"
4. Multiple entries → all included in chronological order
5. URL filter `/api/` → only matching entries exported
6. Status filter 400-599 → only error responses exported
7. Authorization header → replaced with [REDACTED]
8. Cookie values → replaced with [REDACTED]
9. Resource timing available → granular timing breakdown
10. No resource timing → total duration as `wait`
11. Binary response → base64-encoded with encoding field
12. Truncated body → comment noting truncation
13. Empty buffer → valid HAR with zero entries
14. Output path creates directories
15. Multiple page loads → pages section with pageref linkage
16. HAR validates against HAR 1.2 schema
17. `include_bodies: false` → no body content, smaller file
18. Very large export → warning returned
19. time_range filter limits entries to window
20. Custom redact_headers strips specified headers

---

## File Locations

Server implementation: `cmd/browser-agent/export_har.go` (HAR generation, MCP tool handler).

Tests: `cmd/browser-agent/export_har_test.go`.

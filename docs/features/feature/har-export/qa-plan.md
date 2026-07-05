---
status: proposed
scope: feature/har-export/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-har-export
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: HAR Export

> QA plan for the HAR Export feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. HAR files are notorious for containing sensitive data -- auth tokens, cookies, session IDs, PII in request/response bodies. Kaboom runs on localhost and data must never leave the machine unintentionally, but a HAR file is explicitly designed to be shared. Every field must be audited.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Authorization headers in HAR entries | Verify `Authorization` header values are replaced with `[REDACTED]` in `request.headers` and `response.headers` | critical |
| DL-2 | Cookie values in HAR entries | Verify cookie values are replaced with `[REDACTED]` while cookie names are preserved | critical |
| DL-3 | Bearer tokens in request bodies | Verify tokens in POST body fields (e.g., `access_token`, `refresh_token`) are redacted | critical |
| DL-4 | API keys in query parameters | Verify query params named `api_key`, `apiKey`, `key`, `token`, `secret` have values redacted | critical |
| DL-5 | Session IDs in headers | Verify `X-Session-Id`, `X-Auth-Token`, custom session headers are redacted | high |
| DL-6 | PII in response bodies | Response bodies may contain user emails, names, addresses -- verify `include_bodies: false` omits them entirely | high |
| DL-7 | Custom redact_headers parameter | Verify specifying `redact_headers: ["X-Internal-Key"]` strips those headers from output | high |
| DL-8 | Set-Cookie response headers | Verify `Set-Cookie` response header values are redacted | critical |
| DL-9 | Basic auth in URL | Verify `https://user:pass@host/` has credentials stripped from `request.url` | critical |
| DL-10 | CSRF tokens in headers | Verify `X-CSRF-Token` and similar headers are redacted | high |
| DL-11 | Output file path traversal | Verify `output_path` cannot write outside the project directory (e.g., `../../etc/passwd`) | critical |
| DL-12 | Warning message on sensitive data | Verify MCP response includes warning about reviewing HAR before sharing | medium |

### Negative Tests (must NOT leak)
- [ ] Authorization header value must not appear in any HAR entry
- [ ] Cookie values must not appear in `request.cookies` or `response.cookies` fields
- [ ] Bearer token strings (e.g., `eyJhbG...`) must not appear in any field
- [ ] API key values from query params must not appear in `request.queryString`
- [ ] Session ID values must not appear in any header
- [ ] When `include_bodies: false`, no request or response body text appears
- [ ] Passwords must not appear in any field (already redacted by extension)
- [ ] Internal file system paths of the server must not appear in HAR metadata

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | HAR version identification | Response clearly states HAR 1.2 format | [ ] |
| CL-2 | Entry count is prominent | `entry_count` is a top-level field in the response summary | [ ] |
| CL-3 | Time range is unambiguous | ISO 8601 timestamps with timezone in `time_range` | [ ] |
| CL-4 | Summary is human/AI readable | `summary` field describes count by resource type | [ ] |
| CL-5 | Empty HAR is distinguishable | Zero-entry HAR returns a clear note, not just empty array | [ ] |
| CL-6 | Truncated body is explicit | `response.content.comment` explains truncation with original size | [ ] |
| CL-7 | Redacted fields are obvious | `[REDACTED]` placeholder is consistent and unambiguous | [ ] |
| CL-8 | Binary content encoding | base64-encoded bodies have `encoding: "base64"` set | [ ] |
| CL-9 | File size warning | Large export (>10MB) warning is in the response, not just logged | [ ] |
| CL-10 | WebSocket exclusion note | Response notes that WS messages are not in HAR, suggests alternative tool | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM might interpret `[REDACTED]` as a literal header value -- verify the field name and context make redaction obvious
- [ ] LLM might treat an empty HAR (zero entries) as an error rather than "no captured data" -- verify the note explains this
- [ ] LLM might not realize timing fields of 0 mean "not measured" vs "instant" -- verify fallback timing includes a comment
- [ ] LLM might confuse `time` (total ms) with `startedDateTime` (ISO timestamp) -- verify field naming is unambiguous
- [ ] LLM might not understand `pageref` linkage -- verify pages section and entries are clearly associated

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Export all captured traffic | 1 step: call `generate` with `format: "har"` | No -- already minimal |
| Export filtered by URL | 1 step: add `url_filter` parameter | No |
| Export errors only | 1 step: add `status_filter: { min: 400, max: 599 }` | No |
| Export to file | 1 step: add `output_path` parameter | No |
| Export without bodies | 1 step: add `include_bodies: false` | No |
| Export with custom redaction | 1 step: add `redact_headers` array | No |

### Default Behavior Verification
- [ ] Feature works with zero configuration (all captured traffic exported with default redaction)
- [ ] `include_bodies` defaults to `true` (most useful default)
- [ ] Default redaction covers Authorization, Cookie, and common auth headers without configuration
- [ ] When `output_path` is omitted, HAR JSON is returned inline in MCP response
- [ ] Creator field auto-populates with Kaboom name and version

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Single GET request mapping | One GET network body entry | Valid HAR entry with correct method, URL, status | must |
| UT-2 | POST with body mapping | POST entry with JSON body | `request.postData.text` contains body, `mimeType` set | must |
| UT-3 | Response content type | JSON response body | `response.content.mimeType` is `application/json` | must |
| UT-4 | Authorization header redaction | Entry with `Authorization: Bearer xyz` | Header value is `[REDACTED]` | must |
| UT-5 | Cookie value redaction | Entry with `Cookie: session=abc123` | Cookie value is `[REDACTED]`, name preserved | must |
| UT-6 | Custom header redaction | `redact_headers: ["X-Custom"]` with matching entry | `X-Custom` header value is `[REDACTED]` | must |
| UT-7 | URL filter application | `url_filter: "/api/"` with mixed entries | Only entries matching `/api/` in output | must |
| UT-8 | Method filter application | `method_filter: ["POST"]` with mixed entries | Only POST entries in output | must |
| UT-9 | Status filter application | `status_filter: { min: 400, max: 599 }` | Only 4xx/5xx entries in output | must |
| UT-10 | Time range filter | `time_range` with start/end | Only entries within window | must |
| UT-11 | Chronological ordering | Multiple entries with different timestamps | Entries sorted by `startedDateTime` ascending | must |
| UT-12 | Binary response encoding | Binary response body | base64-encoded with `encoding: "base64"` | must |
| UT-13 | Truncated body comment | Body exceeding 100KB capture limit | `response.content.comment` notes truncation, `size` reflects original | must |
| UT-14 | Empty buffer | No network bodies captured | Valid HAR with empty `entries` array and note | must |
| UT-15 | `include_bodies: false` | Normal entries | No `postData.text` or `content.text` fields | should |
| UT-16 | HAR version field | Any export | `log.version` is `"1.2"` | must |
| UT-17 | Creator field | Any export | `log.creator.name` is `"Kaboom"`, version matches server | must |
| UT-18 | Resource timing mapping | Entry with full Performance API timing | Granular `timings` (dns, connect, ssl, send, wait, receive) | should |
| UT-19 | Timing fallback | Entry with only total duration | `wait` equals total duration, others are 0 | must |
| UT-20 | Data URL exclusion | Data URL entry in buffer | Not included in HAR output | should |
| UT-21 | 304 cached response | 304 status entry | Included with empty body | should |
| UT-22 | Redirect chain | 301 -> 200 entries | Each redirect is separate, `redirectURL` set | should |
| UT-23 | Pages section | Multiple page loads | `pages` array with `pageTimings`, entries have `pageref` | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end export via MCP | MCP client -> `generate(format: "har")` -> server -> file | Valid HAR file written to disk, response has metadata | must |
| IT-2 | Export after network body capture | Extension captures traffic -> server stores -> export | All captured entries appear in HAR | must |
| IT-3 | Output path directory creation | `output_path` with non-existent parent dirs | Directories created via MkdirAll, file written | must |
| IT-4 | Large export warning | >10MB of captured traffic | Response includes size warning | should |
| IT-5 | Concurrent export and capture | Export while new network bodies arriving | Export completes with consistent snapshot (no partial entries) | must |
| IT-6 | HAR import validation | Export HAR -> import into Chrome DevTools or HAR validator | No validation errors | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | HAR generation for 100 entries | Wall clock time | Under 50ms | must |
| PT-2 | File write for 5MB HAR | Wall clock time | Under 100ms | must |
| PT-3 | Memory during generation | Peak memory allocation | Under 10MB temporary | should |
| PT-4 | Filtered export performance | Time with URL filter on 1000 entries | Proportional reduction | should |
| PT-5 | JSON marshaling overhead | Marshal time for 100 entries | Under 30ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | WebSocket upgrade request | HTTP 101 Switching Protocols entry | Included as HTTP entry, note about WS messages | must |
| EC-2 | CORS preflight | OPTIONS request with CORS headers | Included in HAR output | should |
| EC-3 | Very large export (>10MB) | Thousands of entries with bodies | Warning returned, export still completes | must |
| EC-4 | Unicode in response body | JSON with emoji, CJK characters | Correctly encoded in HAR content.text | must |
| EC-5 | Empty response body | 204 No Content response | Entry present with empty content.text, size 0 | must |
| EC-6 | Malformed URL in entry | Entry with invalid URL characters | Gracefully handled, included in HAR | should |
| EC-7 | Non-writable output path | Read-only directory | Error returned in MCP response | must |
| EC-8 | Concurrent filter parameters | `url_filter` + `status_filter` + `time_range` | All filters applied as AND conditions | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page loaded with various network requests (API calls, static assets, errors)
- [ ] At least one POST request with JSON body has been made
- [ ] At least one request returns 4xx or 5xx status

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "generate", "arguments": {"type": "har"}}` | Review inline HAR JSON in MCP response | Valid HAR with `log.version: "1.2"`, entries array, creator field | [ ] |
| UAT-2 | `{"tool": "generate", "arguments": {"type": "har", "output_path": ".kaboom/reports/test.har"}}` | Check file exists at specified path | File created, response shows `file_path`, `entry_count`, `total_size_bytes` | [ ] |
| UAT-3 | `{"tool": "generate", "arguments": {"type": "har", "url_filter": "/api/"}}` | Compare entry count to unfiltered | Only API-path entries included, count is lower | [ ] |
| UAT-4 | `{"tool": "generate", "arguments": {"type": "har", "status_filter": {"min": 400, "max": 599}}}` | Verify all entries are errors | Every entry has status 4xx or 5xx | [ ] |
| UAT-5 | `{"tool": "generate", "arguments": {"type": "har", "include_bodies": false}}` | Open HAR file, check for body content | No `postData.text` or `content.text` fields present | [ ] |
| UAT-6 | `{"tool": "generate", "arguments": {"type": "har", "redact_headers": ["X-Request-Id"]}}` | Search HAR for X-Request-Id values | Header name present but value is `[REDACTED]` | [ ] |
| UAT-7 | Import generated HAR file into Chrome DevTools (Network tab -> Import) | DevTools opens HAR without errors | All entries visible in Network tab waterfall | [ ] |
| UAT-8 | `{"tool": "generate", "arguments": {"type": "har", "method_filter": ["POST", "PUT"]}}` | Check that only mutation requests are in output | No GET, DELETE, or other methods present | [ ] |
| UAT-9 | Navigate to a new page, then export with `time_range` covering only the old page | Check entries | Only entries from the specified time window appear | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Authorization header redacted | Search HAR file for `Bearer`, `Basic`, actual token strings | Not found -- only `[REDACTED]` appears | [ ] |
| DL-UAT-2 | Cookie values redacted | Search HAR for known session cookie values | Not found -- cookie names present but values are `[REDACTED]` | [ ] |
| DL-UAT-3 | API keys in query params | Make request with `?api_key=secret123`, export HAR | `secret123` does not appear in HAR | [ ] |
| DL-UAT-4 | Warning present in response | Check MCP response metadata | Warning about reviewing HAR before sharing is present | [ ] |
| DL-UAT-5 | No server filesystem paths leaked | Search HAR for Go source paths or `/Users/` or `/home/` | No server-internal paths found in HAR metadata | [ ] |

### Regression Checks
- [ ] Existing `observe` network_bodies tool still works after HAR export feature is enabled
- [ ] Existing `generate` tool with other types (reproduction, test, pr_summary) still works
- [ ] Network body capture buffer is not modified or drained by HAR export (export is read-only)
- [ ] Server memory does not spike during export (temporary copy is bounded)

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

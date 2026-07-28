---
status: shipped
scope: feature/best-practices-audit/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-best-practices-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Best Practices Audit

> QA plan for the Best Practices Audit feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

**Note:** No tech-spec.md is available for this feature. This QA plan is based solely on the product-spec.md.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Evidence strings expose full URLs with query parameters containing tokens/session IDs | Verify evidence strings for transport and security checks strip or truncate sensitive query parameters from URLs. Only path portions should appear. | high |
| DL-2 | Security header values may reveal internal infrastructure (e.g., CSP with internal hostnames) | Verify header values in evidence strings are shown as-is. This is acceptable for localhost-only delivery but document the risk. | medium |
| DL-3 | Console error messages may contain user data (e.g., "Failed to load user john@example.com") | Verify evidence strings for console health checks truncate error messages to avoid including PII. Limit message length in evidence. | high |
| DL-4 | DOM metadata query exposes page content | Verify metadata checks only read structural elements (`<head>`, `<html lang>`, `<title>`, `<meta>`) -- not body content, form values, or user data. | medium |
| DL-5 | Remediation strings are static and cannot be injected | Verify all remediation text is hardcoded in the Go handler. No console log content, header values, or URL data is interpolated into remediation strings. | high |
| DL-6 | `page_urls` in data_coverage exposes visited pages | Verify `data_coverage.page_urls` contains only page URLs already known to the MCP client. These are the same URLs visible in network waterfall. | low |
| DL-7 | URL filter parameter does not leak cross-session data | Verify `url` parameter filters data within the current session only. No data from previous sessions is returned. | medium |
| DL-8 | Deprecated API warning messages may contain function names revealing internal code structure | Verify deprecation evidence includes only the browser-generated warning message, not source code snippets. | low |

### Negative Tests (must NOT leak)
- [ ] Evidence strings must NOT contain request/response body content
- [ ] Evidence strings must NOT contain authentication headers (Authorization, Cookie, etc.)
- [ ] Remediation text must NOT be dynamically generated from captured data (must be static)
- [ ] DOM metadata query must NOT read form field values, localStorage, or cookies
- [ ] Audit results must NOT be sent to any external endpoint

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Verdict values are consistent across all checks | All checks use exactly `"pass"`, `"fail"`, `"warn"`, or `"skipped"`. No other values. | [ ] |
| CL-2 | Score percentage calculation is correct | `percentage` = `passed / total * 100` (rounded). Verify this matches the pass count. | [ ] |
| CL-3 | Check IDs are unique and stable | Every check has a unique `id` string. IDs are deterministic across runs. | [ ] |
| CL-4 | Category names match parameter values | The `categories` parameter accepts `"security"`, `"console"`, `"metadata"`, `"transport"`. Each check's `category` field uses these exact strings. | [ ] |
| CL-5 | Remediation is null for passing checks | Passing checks have `"remediation": null`, not an empty string or missing field. | [ ] |
| CL-6 | Evidence string is always present | Every check (pass, fail, warn, skipped) has a non-null `evidence` string explaining the finding. | [ ] |
| CL-7 | Recommendations array is prioritized | The `recommendations` array lists the most impactful items first. Verify ordering matches severity. | [ ] |
| CL-8 | Skipped checks are clearly distinguished from failures | `"verdict": "skipped"` with explanatory evidence is clearly different from `"verdict": "fail"`. | [ ] |
| CL-9 | data_coverage provides context on data quality | `data_coverage` shows how much data was analyzed so the LLM can judge audit completeness. | [ ] |
| CL-10 | Warn vs fail distinction is clear | `warn` indicates advisory/minor issues; `fail` indicates standards violations. Verify the distinction is consistent. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM may treat "skipped" as "pass" -- verify the audit explicitly calls out skipped checks and why they were skipped.
- [ ] LLM may assume 100% score means perfect site -- verify the response notes which data was analyzed (e.g., only 12 console entries, only 47 requests).
- [ ] LLM may not understand that `warn` on `permissions-policy` is advisory, not a compliance failure. Verify severity context in the description.
- [ ] LLM may confuse this audit with the security audit (`observe({what: "security_audit"})`). Verify the response structure is clearly distinct (different field names, different focus).
- [ ] LLM may not realize metadata checks depend on extension connectivity. Verify the response indicates metadata_source and explains if DOM query failed.

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Run full best practices audit | 1 step: `generate({what: "best_practices_audit"})` | No -- already minimal |
| Run audit for specific categories | 1 step with `categories` parameter | No -- single call |
| Run audit scoped to URL | 1 step with `url` parameter | No -- single call |
| Get failures only (fewer tokens) | 1 step with `include_passing: false` | No -- single call |
| Run audit then fix issues | 2+ steps: audit + code changes + re-audit | Natural workflow; cannot simplify |

### Default Behavior Verification
- [ ] `generate({what: "best_practices_audit"})` with no optional params runs all 16 checks
- [ ] Default `include_passing` is `true` -- all checks shown
- [ ] Default `categories` is all categories -- no filtering
- [ ] Default `url` is empty -- all captured data analyzed

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Parse best_practices_audit type | `{type: "best_practices_audit"}` | Handler routes to best practices audit | must |
| UT-2 | Validate categories parameter | `["security", "console"]` | Only security + console checks run | must |
| UT-3 | Reject invalid category | `["security", "invalid_cat"]` | "invalid_cat" silently ignored, security checks run | must |
| UT-4 | All categories invalid | `["foo", "bar"]` | Empty checks array with warning | must |
| UT-5 | HTTPS usage check - all HTTPS | Network entries all HTTPS | `https-usage` verdict: pass | must |
| UT-6 | HTTPS usage check - HTTP detected | One non-localhost HTTP request | `https-usage` verdict: fail with HTTP URL in evidence | must |
| UT-7 | HTTPS usage check - localhost HTTP allowed | HTTP requests only to localhost | `https-usage` verdict: pass | must |
| UT-8 | Mixed content detection | HTTPS page with HTTP resource | `no-mixed-content` verdict: fail with resource URLs | must |
| UT-9 | CSP header present | HTML response with CSP header | `csp-header` verdict: pass | must |
| UT-10 | CSP Report-Only without enforcing | Only CSP-Report-Only header | `csp-header` verdict: warn | must |
| UT-11 | CSP header missing | No CSP on HTML responses | `csp-header` verdict: fail | must |
| UT-12 | HSTS header with adequate max-age | HSTS max-age >= 31536000 | `hsts-header` verdict: pass | must |
| UT-13 | HSTS header with short max-age | HSTS max-age = 3600 | `hsts-header` verdict: warn | must |
| UT-14 | HSTS skipped for localhost | Localhost-only traffic | `hsts-header` verdict: skipped or N/A | must |
| UT-15 | X-Content-Type-Options correct | Header: "nosniff" | `x-content-type-options` verdict: pass | must |
| UT-16 | X-Frame-Options ALLOW-FROM deprecated | X-Frame-Options: ALLOW-FROM | `x-frame-options` verdict: warn | must |
| UT-17 | Referrer-Policy unsafe-url | Referrer-Policy: unsafe-url | `referrer-policy` verdict: warn | should |
| UT-18 | JS error-free check: 0 errors | No exceptions in log buffer | `js-error-free` verdict: pass | must |
| UT-19 | JS error-free check: 3+ errors | 5 exceptions in log buffer | `js-error-free` verdict: fail with error details | must |
| UT-20 | Console noise check: high noise | 250 console entries | `low-console-noise` verdict: fail | must |
| UT-21 | Deprecated API detection | Console warning with "[Deprecation]" prefix | `no-deprecated-apis` verdict: warn or fail | must |
| UT-22 | Document metadata: all present | DOCTYPE, charset, viewport, title, lang all found | All metadata checks pass | must |
| UT-23 | Document metadata: title empty | Empty `<title>` element | `has-title` verdict: warn | must |
| UT-24 | Score calculation: all pass | 16/16 pass | percentage: 100 | must |
| UT-25 | Score calculation: mixed results | 12 pass, 2 warn, 2 fail | percentage: 75 | must |
| UT-26 | include_passing=false filters output | 14 pass, 2 fail, include_passing=false | Only 2 failing checks in response | must |
| UT-27 | URL filter scopes all data sources | url="/api" with mixed traffic | Only /api traffic analyzed across all checks | must |
| UT-28 | Recommendations array generated | 2 failures + 1 warning | Recommendations list prioritized by severity | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full audit with populated buffers | Go server log buffer + network bodies + waterfall | Complete audit report with all 16 checks | must |
| IT-2 | Audit with DOM query for metadata | Go server + Extension analyze({what: "dom"}) pipeline | Metadata checks populated from DOM query results | must |
| IT-3 | Audit with extension disconnected | Go server without extension | Transport + security + console checks run; metadata checks skipped | must |
| IT-4 | Audit with empty buffers | Fresh server, no browsing activity | All checks skipped with "No data captured" evidence | must |
| IT-5 | Audit with only API traffic (no HTML) | JSON API responses only | Header checks skipped ("No HTML responses"); transport checks still run | should |
| IT-6 | Concurrent audit requests | Two simultaneous audit requests | Both complete without blocking each other (read locks on buffers) | should |
| IT-7 | CSP remediation cross-references generate tool | CSP header missing | Remediation suggests `generate({what: 'csp'})` | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Audit without DOM query | Response time | < 50ms | must |
| PT-2 | Audit with DOM query | Response time | < 2500ms (2s DOM query timeout) | must |
| PT-3 | Audit with 1000 log entries | Buffer iteration time | < 5ms | must |
| PT-4 | Memory allocation during audit | Heap allocations | < 500KB | must |
| PT-5 | Full response token cost (all passing) | Token count | < 2000 tokens | should |
| PT-6 | Failures-only response token cost | Token count | < 500 tokens | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Empty buffers (no data captured) | Fresh server start, immediate audit request | All checks "skipped", score 0/0, percentage 0 | must |
| EC-2 | Extension disconnected during metadata query | DOM query times out | Metadata checks "skipped" with explanation; other checks run | must |
| EC-3 | Only API traffic (no HTML responses) | All network bodies are JSON responses | Security header checks "skipped" ("No HTML responses"); transport checks still run | should |
| EC-4 | Localhost-only traffic | All requests to localhost | HSTS skipped; HTTPS check passes for localhost HTTP | must |
| EC-5 | Very large log buffer (1000+ entries) | 1000 console log entries | Audit completes within 50ms (server-side only) | must |
| EC-6 | All categories invalid | `categories: ["x", "y"]` | Empty checks array, warning returned | must |
| EC-7 | Mixed HTTP/HTTPS origins | Some origins HTTPS, some HTTP | Per-origin analysis; mixed content correctly identified | should |
| EC-8 | Concurrent audits | Two audit requests at same time | Both succeed; read locks prevent conflicts | should |
| EC-9 | DOM query returns partial metadata | Only some head elements found | Present checks pass/fail, missing checks report appropriately | should |
| EC-10 | Non-standard deprecation warning format | Warning without "[Deprecation]" prefix but containing "deprecated" | Detected by substring matching | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web page with known best practices issues loaded (e.g., missing CSP, console errors, no viewport meta tag)
- [ ] Browse the page to populate log and network buffers

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "generate", "arguments": {"type": "best_practices_audit"}}` | Server processes audit | Response contains structured JSON with all 16 checks, verdicts, evidence, and score | [ ] |
| UAT-2 | Inspect score in response | Compare score.passed + score.warned + score.failed to score.total | Counts add up correctly; percentage matches passed/total * 100 | [ ] |
| UAT-3 | Inspect a failing check | Look at a check with `verdict: "fail"` | Has non-null `evidence` and `remediation` strings; evidence is specific | [ ] |
| UAT-4 | Inspect a passing check | Look at a check with `verdict: "pass"` | Has `evidence` string; `remediation` is null | [ ] |
| UAT-5 | `{"tool": "generate", "arguments": {"type": "best_practices_audit", "categories": ["security"]}}` | Only security category checks | Response contains only 6 security header checks, no console/metadata/transport | [ ] |
| UAT-6 | `{"tool": "generate", "arguments": {"type": "best_practices_audit", "include_passing": false}}` | Failures-only report | Response contains only failing and warning checks; passing checks omitted | [ ] |
| UAT-7 | `{"tool": "generate", "arguments": {"type": "best_practices_audit", "url": "/nonexistent"}}` | URL filter with no matching traffic | Most checks "skipped" due to no matching data | [ ] |
| UAT-8 | Inspect recommendations array | Top 3-5 recommendations listed | Recommendations are actionable and ordered by impact/severity | [ ] |
| UAT-9 | Inspect data_coverage section | Data coverage stats shown | Console entries, network requests, HTML responses, and page URLs counts are accurate | [ ] |
| UAT-10 | Disconnect extension, then run audit | Extension disconnected | Transport and security checks still run; metadata checks show "skipped" | [ ] |
| UAT-11 | Fix a failing issue (e.g., add CSP header), reload page, re-run audit | Score improves | Previously failing check now passes; score percentage increases | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Evidence strings do not contain request bodies | Inspect all evidence strings in full audit response | No request/response body content in evidence | [ ] |
| DL-UAT-2 | Evidence strings do not contain auth headers | Load a page with Authorization headers, run audit | No Authorization/Cookie header values in evidence (header names OK, values redacted) | [ ] |
| DL-UAT-3 | Remediation text is static | Compare remediation strings across multiple runs with different pages | Same check always produces same remediation template | [ ] |
| DL-UAT-4 | DOM metadata query is structural only | Review data_coverage.metadata_source and metadata check evidence | Only structural HTML elements referenced (title, meta, html lang) -- no form values or body content | [ ] |
| DL-UAT-5 | No external requests during audit | Monitor network during audit | No outbound requests; audit is purely local analysis | [ ] |

### Regression Checks
- [ ] Existing `generate` modes (csp, reproduction, test, har, sarif) still work correctly
- [ ] Existing `observe({what: "security_audit"})` still works independently
- [ ] Log buffer and network buffer not modified by audit (read-only access)
- [ ] Server performance not degraded for non-audit MCP calls

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

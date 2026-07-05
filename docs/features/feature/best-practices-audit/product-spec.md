---
feature: best-practices-audit
status: proposed
version: null
tool: generate
mode: best_practices_audit
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-best-practices-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Best Practices Audit

> Generates a structured best practices audit from captured browser telemetry, evaluating HTTPS usage, security headers, deprecated API usage, console health, document metadata completeness, and mixed content -- producing LLM-consumable pass/fail verdicts with actionable recommendations.

## Problem

When an AI coding agent is debugging or reviewing a web application, it lacks a systematic way to evaluate whether the application follows web development best practices. Today, the agent can observe individual signals -- console errors via `observe({what: "errors"})`, security headers via `observe({what: "security_audit"})`, network traffic via `observe({what: "network_waterfall"})` -- but there is no unified assessment that combines these signals into a holistic best practices verdict.

This matters for AI coding agents because:

1. **Fragmented data requires multiple tool calls.** An agent must call 4-5 different observe modes, mentally correlate the results, and synthesize a best practices assessment. This wastes tokens and is error-prone.

2. **No pass/fail structure.** Existing observe modes return raw data. The agent must interpret "is this good or bad?" for each data point. A structured audit with explicit verdicts eliminates this interpretation burden.

3. **Missed checks.** Without a checklist, agents skip checks they do not think to make. A best practices audit covers a canonical set of checks every time, ensuring nothing is overlooked.

4. **No actionable output.** Raw console logs and header lists do not tell the agent what to fix. A best practices audit pairs each failing check with a specific remediation recommendation that the agent can act on immediately.

This feature is inspired by Lighthouse's best practices category, adapted for Kaboom's architecture: instead of running a synthetic page load, it analyzes data already captured during real browsing sessions.

## Solution

A new `generate` mode (`best_practices_audit`) that aggregates data from Kaboom's existing capture buffers -- console logs, network bodies (with response headers), network waterfall entries, and page URLs -- and runs a series of best practices checks against them. Each check produces a pass/fail/warning verdict with a description, evidence, and remediation. The output is a structured JSON report designed for LLM consumption.

The audit composes data that Kaboom already captures. Most checks require no new extension-side collection:

### Already captured (server-side analysis only):
- Console error and warning counts (log buffer)
- Security headers: CSP, HSTS, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy (network bodies with `ResponseHeaders`)
- HTTPS usage and mixed content (network waterfall + network bodies)
- JavaScript error rate (log buffer, source=exception)
- Console noise level (log buffer, all levels)
- Browser error log analysis (log buffer, level=error)

### Requires new extension-side collection:
- Deprecated API usage detection (console warnings from the browser that match deprecation patterns)
- Document metadata completeness (`<title>`, `<meta charset>`, `<meta viewport>`, `<meta description>`, `<html lang>`, `<!DOCTYPE html>`)

For document metadata, the audit leverages the existing `analyze({what: "dom"})` async command infrastructure to query the page's `<head>` element on demand. Deprecated API warnings are already captured in the console log buffer -- the server-side audit simply needs to pattern-match against known deprecation message formats from Chrome and Firefox.

## User Stories

- As an AI coding agent, I want to run a single best practices audit so that I get a complete checklist of what the application does well and what needs fixing, without making multiple observe calls.
- As an AI coding agent, I want each failing check to include a specific remediation so that I can generate a fix immediately.
- As an AI coding agent, I want pass/fail verdicts so that I can quickly skip passing checks and focus on failures.
- As a developer using Kaboom, I want the AI to proactively identify best practices violations so that I catch issues before they reach production.
- As an AI coding agent, I want a summary score so that I can report "your app passes 14/16 best practices checks" in a PR summary or deployment review.

## MCP Interface

**Tool:** `generate`
**Mode/Action:** `best_practices_audit`

### Request

```json
{
  "tool": "generate",
  "arguments": {
    "type": "best_practices_audit",
    "url": "/app",
    "categories": ["security", "console", "metadata", "transport"],
    "include_passing": true
  }
}
```

#### Parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `type` | string | required | Must be `"best_practices_audit"` |
| `url` | string | `""` | URL filter. Only analyze data matching this URL substring. Empty = all captured data. |
| `categories` | string[] | all | Which check categories to run. Options: `"security"`, `"console"`, `"metadata"`, `"transport"`. Empty = run all. |
| `include_passing` | bool | `true` | Whether to include passing checks in the response. Set `false` for a failures-only report (fewer tokens). |

### Response

```json
{
  "audit": "best_practices",
  "url_scope": "/app",
  "timestamp": "2026-01-28T14:30:00Z",
  "score": {
    "passed": 12,
    "warned": 2,
    "failed": 2,
    "total": 16,
    "percentage": 75
  },
  "checks": [
    {
      "id": "https-usage",
      "category": "transport",
      "title": "Uses HTTPS",
      "verdict": "pass",
      "description": "All non-localhost network requests use HTTPS.",
      "evidence": "47/47 external requests use HTTPS",
      "remediation": null
    },
    {
      "id": "no-mixed-content",
      "category": "transport",
      "title": "No mixed content",
      "verdict": "fail",
      "description": "HTTPS page loads resources over HTTP.",
      "evidence": "2 HTTP resources on HTTPS page: http://cdn.example.com/legacy.js, http://img.example.com/logo.png",
      "remediation": "Update resource URLs to use HTTPS. If the resource server does not support HTTPS, find an alternative CDN or host the resource yourself."
    },
    {
      "id": "csp-header",
      "category": "security",
      "title": "Content-Security-Policy header present",
      "verdict": "fail",
      "description": "No Content-Security-Policy header found on HTML responses.",
      "evidence": "0/3 HTML responses include a CSP header",
      "remediation": "Add a Content-Security-Policy header. Use generate({format: 'csp'}) to generate a policy based on observed traffic."
    },
    {
      "id": "hsts-header",
      "category": "security",
      "title": "Strict-Transport-Security header present",
      "verdict": "pass",
      "description": "HSTS header is set on HTTPS responses.",
      "evidence": "Strict-Transport-Security: max-age=31536000; includeSubDomains",
      "remediation": null
    },
    {
      "id": "x-content-type-options",
      "category": "security",
      "title": "X-Content-Type-Options header present",
      "verdict": "pass",
      "description": "X-Content-Type-Options: nosniff is set.",
      "evidence": "X-Content-Type-Options: nosniff",
      "remediation": null
    },
    {
      "id": "x-frame-options",
      "category": "security",
      "title": "X-Frame-Options or CSP frame-ancestors present",
      "verdict": "warn",
      "description": "X-Frame-Options is set but uses the deprecated ALLOW-FROM directive.",
      "evidence": "X-Frame-Options: ALLOW-FROM https://partner.example.com",
      "remediation": "Replace X-Frame-Options: ALLOW-FROM with CSP frame-ancestors directive: Content-Security-Policy: frame-ancestors 'self' https://partner.example.com"
    },
    {
      "id": "referrer-policy",
      "category": "security",
      "title": "Referrer-Policy header present",
      "verdict": "pass",
      "description": "Referrer-Policy is configured.",
      "evidence": "Referrer-Policy: strict-origin-when-cross-origin",
      "remediation": null
    },
    {
      "id": "permissions-policy",
      "category": "security",
      "title": "Permissions-Policy header present",
      "verdict": "warn",
      "description": "No Permissions-Policy header found. Browser features are unrestricted.",
      "evidence": "0/3 HTML responses include a Permissions-Policy header",
      "remediation": "Add a Permissions-Policy header to restrict access to sensitive browser features (camera, microphone, geolocation). Example: Permissions-Policy: camera=(), microphone=(), geolocation=()"
    },
    {
      "id": "js-error-free",
      "category": "console",
      "title": "No JavaScript errors",
      "verdict": "fail",
      "description": "JavaScript errors detected in console.",
      "evidence": "3 errors: TypeError: Cannot read property 'map' of undefined (x2), ReferenceError: config is not defined (x1)",
      "remediation": "Fix the TypeError by adding null checks before calling .map(). Fix the ReferenceError by ensuring 'config' is defined or imported before use."
    },
    {
      "id": "low-console-noise",
      "category": "console",
      "title": "Low console noise",
      "verdict": "pass",
      "description": "Console output is within acceptable levels.",
      "evidence": "12 total console entries (3 log, 5 info, 2 warn, 2 error). Noise ratio: 25% non-error entries.",
      "remediation": null
    },
    {
      "id": "no-deprecated-apis",
      "category": "console",
      "title": "No deprecated API usage",
      "verdict": "pass",
      "description": "No deprecated API warnings detected in console.",
      "evidence": "0 deprecation warnings found",
      "remediation": null
    },
    {
      "id": "has-doctype",
      "category": "metadata",
      "title": "Document has DOCTYPE",
      "verdict": "pass",
      "description": "Page declares <!DOCTYPE html>.",
      "evidence": "<!DOCTYPE html> present",
      "remediation": null
    },
    {
      "id": "has-charset",
      "category": "metadata",
      "title": "Character encoding declared",
      "verdict": "pass",
      "description": "Page declares character encoding via <meta charset>.",
      "evidence": "<meta charset=\"UTF-8\">",
      "remediation": null
    },
    {
      "id": "has-viewport",
      "category": "metadata",
      "title": "Viewport meta tag present",
      "verdict": "pass",
      "description": "Page includes a viewport meta tag for responsive design.",
      "evidence": "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">",
      "remediation": null
    },
    {
      "id": "has-title",
      "category": "metadata",
      "title": "Page has title",
      "verdict": "pass",
      "description": "Page has a non-empty <title> element.",
      "evidence": "Title: \"Dashboard - MyApp\"",
      "remediation": null
    },
    {
      "id": "has-lang",
      "category": "metadata",
      "title": "HTML lang attribute present",
      "verdict": "pass",
      "description": "The <html> element has a lang attribute.",
      "evidence": "<html lang=\"en\">",
      "remediation": null
    }
  ],
  "recommendations": [
    "Fix 2 JavaScript errors before they affect users. Use observe({what: 'errors'}) for full stack traces.",
    "Add a Content-Security-Policy header. Run generate({format: 'csp'}) to auto-generate one from observed traffic.",
    "Update 2 HTTP resource URLs to HTTPS to eliminate mixed content."
  ],
  "data_coverage": {
    "console_entries_analyzed": 12,
    "network_requests_analyzed": 47,
    "html_responses_analyzed": 3,
    "page_urls": ["https://myapp.example.com/app/dashboard"],
    "metadata_source": "dom_query"
  }
}
```

## Check Catalog

The audit runs the following checks, organized by category:

### Transport (2 checks)

| ID | Check | Verdict Logic |
|----|-------|---------------|
| `https-usage` | All non-localhost requests use HTTPS | **pass**: all external requests use HTTPS. **fail**: any non-localhost HTTP request found. |
| `no-mixed-content` | No HTTP resources loaded on HTTPS pages | **pass**: no HTTP resources on HTTPS pages. **fail**: any HTTP resource loaded from an HTTPS page. |

### Security Headers (6 checks)

| ID | Check | Verdict Logic |
|----|-------|---------------|
| `csp-header` | Content-Security-Policy present on HTML responses | **pass**: CSP header found. **warn**: CSP-Report-Only found but no enforcing CSP. **fail**: no CSP. |
| `hsts-header` | Strict-Transport-Security present on HTTPS responses | **pass**: HSTS header with max-age >= 31536000. **warn**: HSTS present but max-age < 31536000. **fail**: no HSTS on HTTPS. N/A for HTTP-only or localhost. |
| `x-content-type-options` | X-Content-Type-Options: nosniff | **pass**: header present with value "nosniff". **fail**: header missing or wrong value. |
| `x-frame-options` | X-Frame-Options or CSP frame-ancestors | **pass**: X-Frame-Options (DENY or SAMEORIGIN) or CSP frame-ancestors present. **warn**: ALLOW-FROM used (deprecated). **fail**: neither present. |
| `referrer-policy` | Referrer-Policy present | **pass**: any valid Referrer-Policy. **warn**: set to "unsafe-url" or "no-referrer-when-downgrade" (weak). **fail**: not present. |
| `permissions-policy` | Permissions-Policy present | **pass**: header present. **warn**: not present (advisory, lower severity). |

### Console Health (3 checks)

| ID | Check | Verdict Logic |
|----|-------|---------------|
| `js-error-free` | No uncaught JavaScript errors or exceptions | **pass**: zero errors with source=exception. **warn**: 1-2 errors. **fail**: 3+ errors. |
| `low-console-noise` | Console output within acceptable levels | **pass**: < 50 entries total and < 20 warnings. **warn**: 50-200 entries or 20-50 warnings. **fail**: > 200 entries or > 50 warnings. |
| `no-deprecated-apis` | No deprecated API usage detected | **pass**: no deprecation warnings in console. **warn**: 1-3 deprecation warnings. **fail**: 4+ deprecation warnings. |

### Document Metadata (5 checks)

| ID | Check | Verdict Logic |
|----|-------|---------------|
| `has-doctype` | DOCTYPE declaration present | **pass**: `<!DOCTYPE html>` present. **fail**: missing. |
| `has-charset` | Character encoding declared | **pass**: `<meta charset>` present. **fail**: missing. |
| `has-viewport` | Viewport meta tag present | **pass**: `<meta name="viewport">` present. **fail**: missing. |
| `has-title` | Non-empty page title | **pass**: `<title>` element with content. **warn**: empty title. **fail**: no title element. |
| `has-lang` | HTML lang attribute present | **pass**: `<html lang="...">` present with value. **fail**: missing lang attribute. |

**Total: 16 checks** across 4 categories.

## Data Sources and Reuse

A key design principle: the best practices audit does NOT run its own data collection. It reads from existing Kaboom buffers and leverages existing capture infrastructure.

| Data Needed | Source | Already Captured? |
|-------------|--------|-------------------|
| Console errors, warnings, info logs | Log ring buffer | Yes |
| JavaScript exceptions (uncaught) | Log ring buffer (source=exception) | Yes |
| Deprecation warnings | Log ring buffer (level=warn, message pattern) | Yes (pattern matching needed) |
| Response headers (CSP, HSTS, etc.) | Network bodies (`ResponseHeaders` field) | Yes |
| HTTPS vs HTTP usage | Network waterfall entries + network bodies | Yes |
| Mixed content detection | Network waterfall (page URL vs resource URL schemes) | Yes |
| Page URLs | CSP generator page list + network waterfall | Yes |
| Document metadata (DOCTYPE, charset, viewport, title, lang) | DOM query via async command | Partially -- uses existing `analyze({what: "dom"})` infrastructure but needs a targeted metadata query |

### Document Metadata Collection

Document metadata checks require reading the page's `<head>` content. Two approaches:

**Option A (preferred): Server-side metadata query.** The audit handler issues an internal `analyze({what: "dom"})` command targeting `head` and `html[lang]` to retrieve metadata. This reuses the existing async command infrastructure (`/pending-queries` -> extension polls -> executes -> `/dom-result`). The audit waits up to 2 seconds for the result. If the extension is disconnected or times out, metadata checks return `"verdict": "skipped"` with `"evidence": "Extension not connected; metadata checks require an active browser tab."`.

**Option B (fallback): Console log injection.** The extension could inject a small script that logs metadata to the console in a structured format. The audit would then parse it from the log buffer. This is less clean and not preferred.

### Deprecated API Detection

Chrome and Firefox emit console warnings when deprecated APIs are used. These are already captured in the log buffer. The audit matches against known patterns:

- `[Deprecation]` prefix (Chrome)
- `is deprecated` substring
- `will be removed` substring
- `deprecated` keyword in warning-level messages

No new extension-side code is needed for this check.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Run all 16 best practices checks from captured telemetry data | must |
| R2 | Return structured JSON with pass/fail/warn verdicts per check | must |
| R3 | Include evidence string showing what was found (or not found) for each check | must |
| R4 | Include remediation string for each failing/warning check | must |
| R5 | Calculate a summary score (passed/warned/failed/total/percentage) | must |
| R6 | Support `url` parameter to scope the audit to a URL substring | must |
| R7 | Support `categories` parameter to run a subset of checks | must |
| R8 | Support `include_passing` parameter to omit passing checks from the response | must |
| R9 | Generate a prioritized `recommendations` list (top 3-5 actionable items) | must |
| R10 | Include `data_coverage` section showing how much data was analyzed | must |
| R11 | Detect deprecated API usage from console warning patterns | should |
| R12 | Retrieve document metadata via the existing `analyze({what: "dom"})` async command infrastructure | should |
| R13 | Gracefully degrade metadata checks to "skipped" when extension is disconnected | should |
| R14 | Cross-reference CSP check with the CSP generator: suggest `generate({format: 'csp'})` in remediation when CSP is missing | should |
| R15 | Cross-reference security headers with the security audit: avoid duplicating findings already available via `observe({what: 'security_audit'})` | should |
| R16 | Include a `data_coverage.metadata_source` field indicating whether metadata came from DOM query or was unavailable | should |
| R17 | Support returning only failed checks for minimal-token responses | could |

## Non-Goals

- This feature does NOT run a Lighthouse audit or invoke Lighthouse in any way. It is inspired by Lighthouse's best practices category but operates entirely on Kaboom's own captured data. There is no headless browser invocation, no page load simulation, and no dependency on the Lighthouse npm package.

- This feature does NOT duplicate the security audit (`observe({what: "security_audit"})`). The security audit focuses on vulnerability detection (credentials in URLs, PII leakage, cookie flags). The best practices audit focuses on compliance with web standards and development hygiene. There is intentional overlap on security headers -- both check for their presence -- but the best practices audit provides a pass/fail verdict while the security audit provides vulnerability findings.

- This feature does NOT perform performance auditing. Performance is covered by `observe({what: "performance"})`, `observe({what: "vitals"})`, and the performance budget feature. The best practices audit may reference performance data in the future but does not assess it in this version.

- This feature does NOT score or rank sites comparatively. The score is an absolute count (12/16 passed), not a relative ranking. There is no database of scores across sites.

- Out of scope: accessibility best practices. Accessibility is a separate concern covered by `observe({what: "accessibility"})` which integrates with axe-core. The best practices audit does not include WCAG checks.

- Out of scope: SEO checks (canonical URLs, structured data, Open Graph tags). These are valuable but belong in a separate `generate({format: "seo_audit"})` mode if added later.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Audit response time (without DOM query) | < 50ms |
| Audit response time (with DOM query) | < 2500ms (includes 2s DOM query timeout) |
| Memory impact | < 500KB (analysis is stateless; no persistent allocations) |
| Token cost of full response (16 checks, all passing) | < 2000 tokens |
| Token cost of failures-only response | < 500 tokens |

The audit handler performs in-memory iteration over existing buffers with O(n) complexity where n is the number of log entries + network bodies. No new data structures are allocated beyond the response.

## Security Considerations

- **No new data capture.** The audit reads only from existing buffers (log entries, network bodies, network waterfall). It does not capture new data or expand the attack surface.

- **No sensitive data in output.** Evidence strings reference URLs and header values, which are already visible in other Kaboom outputs. No request/response bodies are included in the audit output.

- **DOM query for metadata.** The metadata query reads `<head>` content and the `<html lang>` attribute. This is non-sensitive structural information. The DOM query is executed through the existing `analyze({what: "dom"})` infrastructure with the same timeout and security constraints.

- **Remediation strings are static.** Remediation recommendations are hardcoded in the Go handler, not derived from captured data. An LLM cannot inject content into remediation text via manipulated console logs or headers.

- **URL filter applies to all data sources.** When a `url` parameter is provided, all buffers are filtered consistently. The audit cannot be tricked into analyzing data from a different URL scope.

## Edge Cases

- **No data captured yet.** If log and network buffers are empty, the audit returns all checks as `"verdict": "skipped"` with `"evidence": "No data captured. Browse the application to generate telemetry."`. The score shows 0/0 and percentage is 0.

- **Extension disconnected (metadata checks).** If the DOM query times out or the extension is not connected, metadata checks (has-doctype, has-charset, has-viewport, has-title, has-lang) return `"verdict": "skipped"` with an explanation. Transport and security checks still run using server-side data.

- **Only API traffic captured (no HTML responses).** Security header checks apply only to HTML responses. If only API (JSON) responses are in the buffer, header checks return `"verdict": "skipped"` with `"evidence": "No HTML responses captured. Navigate to an HTML page to evaluate security headers."`.

- **Localhost-only traffic.** HSTS checks are skipped for localhost (as in the existing security scanner). The `https-usage` check passes for localhost HTTP traffic since HTTPS is not expected locally.

- **Very large log buffer (1000 entries).** The audit iterates all entries but performs O(1) per-entry classification (level check, pattern match). With 1000 entries, this completes in < 5ms.

- **Mixed HTTP/HTTPS origins.** The audit evaluates per-origin, not per-request. If origin `https://api.example.com` has all HTTPS traffic but one request to `http://cdn.legacy.com`, the audit correctly identifies the mixed content.

- **Concurrent audit requests.** The audit handler takes read locks on shared buffers (RWMutex). Multiple concurrent audits do not block each other or data ingestion.

- **Invalid `categories` parameter.** Unknown category strings are silently ignored. If all specified categories are invalid, the audit returns an empty checks array with a warning.

## Dependencies

- **Depends on:**
  - Log ring buffer (shipped) -- Console entries for error rate, noise, deprecation detection.
  - Network bodies with ResponseHeaders (shipped) -- Security header evaluation.
  - Network waterfall (shipped) -- HTTPS/mixed content analysis.
  - CSP generator page list (shipped) -- Page URL enumeration.
  - `analyze({what: "dom"})` async command (shipped) -- Document metadata retrieval.
  - Security scanner header checks (shipped) -- Reuses `requiredSecurityHeaders` list and `isHTMLResponse` / `isLocalhostURL` helpers from `security.go`.

- **Depended on by:**
  - None currently. The audit is a standalone analysis output. Future features (e.g., CI quality gates, PR summary enrichment) may reference audit results.

## Assumptions

- A1: The extension is connected and tracking a tab when metadata checks are requested. If not, metadata checks degrade to "skipped."
- A2: The log buffer contains representative browsing activity. An audit run immediately after server start (empty buffers) produces no meaningful results.
- A3: Network body capture is enabled for at least HTML document responses, so that ResponseHeaders are available for security header checks.
- A4: The browser emits deprecation warnings in the console for deprecated API usage (Chrome and Firefox do this; Safari may not).
- A5: The DOM query infrastructure (`analyze({what: "dom"})`) is functional and the extension can execute queries against the page's DOM.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should the audit cache its results for a configurable TTL to avoid redundant computation on repeated calls? | open | Useful if the AI calls the audit multiple times in quick succession (e.g., before and after a fix). However, caching adds complexity and stale-data risk. The audit is cheap (< 50ms without DOM query), so caching may be premature. |
| OI-2 | Should metadata checks use a synchronous DOM query (blocking the audit response) or return metadata checks as "pending" and require a follow-up call? | open | Current design uses the async analyze({what: "dom"}) infrastructure with a 2s wait. An alternative is to make the audit fully synchronous (skip metadata if not immediately available) and suggest the AI run `analyze({what: "dom"})` separately. |
| OI-3 | Should the check thresholds (e.g., console noise: 50 entries = warning, 200 = fail) be configurable via parameters? | open | Configurable thresholds add parameter complexity. Fixed thresholds aligned with industry norms (similar to Lighthouse) are simpler and more consistent across AI agents. |
| OI-4 | Should the audit integrate with the SARIF exporter so that best practices violations can appear as GitHub Code Scanning annotations? | open | This would allow `generate({format: "sarif"})` to include best practices findings alongside security findings. Requires defining SARIF rule IDs for each check. Natural future extension. |
| OI-5 | Should the audit support a `format` parameter for output variants (e.g., `"markdown"` for PR comments, `"json"` for programmatic use)? | open | JSON-only is simplest and most LLM-friendly. Markdown formatting can be done by the LLM from JSON data. |

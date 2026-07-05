---
status: proposed
scope: feature/seo-audit/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-seo-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: SEO Audit

> QA plan for the SEO Audit feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

**Note:** No tech-spec.md is available for this feature. This QA plan is based solely on the product-spec.md.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Meta description may contain sensitive marketing/business info | Verify meta description is shown as-is. This is publicly visible HTML metadata, acceptable for localhost tooling. | low |
| DL-2 | Canonical URLs may expose internal URL structure with query parameters | Verify canonical URL values are shown as-is from the DOM. These are public SEO signals, acceptable. Document exposure. | low |
| DL-3 | Open Graph tags may contain internal image URLs or preview text | Verify OG tag values are shown as-is. These are designed for public sharing. Acceptable for localhost. | low |
| DL-4 | JSON-LD structured data may contain sensitive business data (prices, inventory, internal IDs) | Verify JSON-LD blocks are returned in full for validation. This is publicly visible page content, but document that developers who embed sensitive data in JSON-LD will see it in the audit. | medium |
| DL-5 | Link URLs may contain tracking IDs or session tokens in query parameters | Verify link URLs are shown as extracted from DOM. The existing privacy layer (URL scrubbing via configure) applies to network telemetry but NOT to DOM-extracted URLs. Document this distinction. | medium |
| DL-6 | Heading text may contain sensitive content (e.g., personalized headings with user names) | Verify heading text is extracted as-is. This is visible page content. Acceptable for localhost dev tooling. | low |
| DL-7 | Image src URLs may point to authenticated or signed CDN URLs | Verify image src URLs are shown as-is from the DOM. Document that signed URLs (with token query params) will appear in audit output. | medium |
| DL-8 | SEO audit runs against DOM injected by JS (React Helmet, Next.js Head) | Verify audit reads rendered DOM, which includes JS-injected meta tags. This is correct behavior for SPA SEO but means audit output reflects runtime state, not source HTML. | low |
| DL-9 | Large page collection hits truncation limits, potentially leaking more data from initial pages | Verify truncation is consistent (200 images, 500 links, 50 headings). All collected data stays on localhost. | low |

### Negative Tests (must NOT leak)
- [ ] SEO audit must NOT make any external HTTP requests (no Google API calls, no SEO service calls)
- [ ] SEO audit must NOT access `document.cookie`, `localStorage`, or `sessionStorage`
- [ ] SEO audit must NOT read form field values
- [ ] SEO audit must NOT modify the page DOM
- [ ] SEO audit results must NOT be sent to any external endpoint
- [ ] DOM collection script must NOT execute user scripts or eval arbitrary code
- [ ] Cross-origin iframe content must NOT be accessed or audited

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Severity values are consistent | All issues use exactly `"error"`, `"warning"`, or `"info"`. No other values. | [ ] |
| CL-2 | Issue codes are machine-readable and unique | Each issue has a `code` field (e.g., `META_DESC_MISSING`) that is unique and deterministic. | [ ] |
| CL-3 | Selector field is actionable | Every issue includes a `selector` or `selectors` field that targets the affected DOM element(s). | [ ] |
| CL-4 | current_value and expected are always present | Every issue has both `current_value` (what was found, or null) and `expected` (what should be). | [ ] |
| CL-5 | Suggestion field is concrete | Every issue has a `suggestion` string with a specific fix (e.g., "Add `<meta name='description'...>`"). | [ ] |
| CL-6 | Summary counts are accurate | `summary.total_issues` = `summary.errors` + `summary.warnings` + `summary.info`. | [ ] |
| CL-7 | dimensions_audited reflects actual scope | `summary.dimensions_audited` lists only the dimensions that were run (matches `scope` parameter). | [ ] |
| CL-8 | Heading structure includes selector paths | Each heading in `structure` array includes `level`, `text`, and `selector`. | [ ] |
| CL-9 | Structured data validation is type-specific | Issues reference the detected schema type (e.g., "Product") and the missing properties specific to that type. | [ ] |
| CL-10 | Status fields per metadata item are unambiguous | Each metadata item has `status`: `"pass"`, `"error"`, or `"warning"`. | [ ] |
| CL-11 | Scoped audit response is structurally consistent | A scoped audit (e.g., `scope: "metadata"`) returns the same response structure but only populates the requested dimension(s). | [ ] |
| CL-12 | Null current_value means missing (not empty) | When a meta tag is absent, `current_value: null`. When present but empty, `current_value: ""`. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM may confuse SEO audit heading checks with accessibility audit heading checks. Verify the issues focus on SEO impact (document outline, search indexing) not WCAG compliance.
- [ ] LLM may not realize JSON-LD validation covers only the top 10 common types. Verify unknown types are flagged without deep validation.
- [ ] LLM may assume the audit checks robots.txt accessibility. Verify the technical dimension only checks for `<meta name="robots">` and `<link>` tags, not HTTP-level robots.txt responses.
- [ ] LLM may not understand that `scope: "metadata"` still returns the full response structure with other dimensions empty. Verify structural consistency.
- [ ] LLM may treat `info` severity as ignorable. Verify `info` issues still have actionable suggestions (e.g., hreflang for multilingual sites).
- [ ] LLM may assume the audit handles Microdata/RDFa structured data. Verify only JSON-LD is validated in v1 and this is documented in the response.

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Run full SEO audit | 1 step: `generate({format: "seo_audit"})` | No -- already minimal |
| Run specific dimension | 1 step: `generate({format: "seo_audit", scope: "metadata"})` | No -- single call |
| Fix issues then re-audit | 2 steps: fix code + re-audit | Natural workflow |
| Audit after navigation | 2 steps: navigate + audit | Could auto-audit on navigate but 2 steps is acceptable |
| Multi-page audit | N steps: navigate + audit per page | Cannot simplify -- single-page by design |

### Default Behavior Verification
- [ ] `generate({format: "seo_audit"})` with no optional params runs full audit (all 6 dimensions)
- [ ] Default `scope` is `"full"` -- all dimensions audited
- [ ] Default `url` uses the currently tracked tab's URL
- [ ] Audit runs against the current rendered DOM (including JS-injected content)
- [ ] No configuration required before first use

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Parse seo_audit type | `{type: "seo_audit"}` | Routes to SEO audit handler | must |
| UT-2 | Validate scope parameter | `"full"`, `"metadata"`, `"headings"`, `"links"`, `"images"`, `"structured_data"`, `"technical"` | All accepted | must |
| UT-3 | Reject invalid scope | `"performance"`, `""` | Error listing valid scope values | must |
| UT-4 | Title: present and valid length | `<title>Widget Store</title>` (13 chars) | status: pass | must |
| UT-5 | Title: too long | Title with 80 characters | issue: TITLE_TOO_LONG | must |
| UT-6 | Title: missing | No `<title>` element | issue: TITLE_MISSING, severity: error | must |
| UT-7 | Title: empty | `<title></title>` | issue: TITLE_EMPTY, severity: error | must |
| UT-8 | Meta description: present and valid | Description with 120 characters | status: pass | must |
| UT-9 | Meta description: missing | No description meta tag | issue: META_DESC_MISSING, severity: error | must |
| UT-10 | Meta description: too short | Description with 30 characters | issue: META_DESC_SHORT, severity: warning | must |
| UT-11 | Meta description: too long | Description with 200 characters | issue: META_DESC_LONG, severity: warning | must |
| UT-12 | Canonical: present | `<link rel="canonical" href="...">` | status: pass | must |
| UT-13 | Canonical: missing | No canonical link | issue: CANONICAL_MISSING, severity: warning | must |
| UT-14 | Open Graph: all core tags present | og:title, og:description, og:image, og:url, og:type | status: pass | must |
| UT-15 | Open Graph: partial tags | Only og:title and og:type | issue: OG_INCOMPLETE with missing list | must |
| UT-16 | Robots: noindex present | `<meta name="robots" content="noindex">` | issue: ROBOTS_NOINDEX, severity: warning | should |
| UT-17 | Heading: single H1 | One H1 element | h1_count: 1, no H1 issues | must |
| UT-18 | Heading: missing H1 | No H1 element | issue: H1_MISSING, severity: error | must |
| UT-19 | Heading: multiple H1s | Two H1 elements | issue: MULTIPLE_H1, severity: warning | must |
| UT-20 | Heading: skipped level | H2 followed by H4 | issue: HEADING_SKIP_LEVEL | must |
| UT-21 | Heading: empty text | `<h2></h2>` | issue: HEADING_EMPTY | should |
| UT-22 | Link: empty href | `<a href="#">` | issue: LINK_EMPTY_HREF, severity: info | should |
| UT-23 | Link: no anchor text | `<a href="/page"></a>` | issue: LINK_NO_TEXT, severity: warning | should |
| UT-24 | Link: count internal vs external | Mix of internal and external links | Correct counts | should |
| UT-25 | Image: missing alt text | `<img src="..." >` | issue: IMG_MISSING_ALT, severity: error | must |
| UT-26 | Image: missing dimensions | `<img src="..." alt="...">` (no width/height) | issue: IMG_MISSING_DIMENSIONS, severity: warning | must |
| UT-27 | Image: decorative alt="" | `<img src="..." alt="">` | Not flagged as missing alt (decorative) | must |
| UT-28 | Structured data: valid Product JSON-LD | Complete Product schema | types_detected: ["Product"], no issues | should |
| UT-29 | Structured data: missing required fields | Product without offers | issue: SCHEMA_MISSING_FIELD | should |
| UT-30 | Structured data: no JSON-LD blocks | No script[type="application/ld+json"] | blocks_found: 0 (not necessarily an error) | should |
| UT-31 | Structured data: invalid JSON in LD block | Malformed JSON in script tag | issue: SCHEMA_INVALID_JSON, severity: error | should |
| UT-32 | Technical: viewport present | `<meta name="viewport" content="...">` | status: pass | should |
| UT-33 | Technical: lang attribute present | `<html lang="en">` | status: pass | should |
| UT-34 | Technical: hreflang present | `<link rel="alternate" hreflang="es">` | Detected, no NO_HREFLANG issue | could |
| UT-35 | Summary calculation | 2 errors, 4 warnings, 2 info | total_issues: 8, errors: 2, warnings: 4, info: 2 | must |
| UT-36 | Scoped audit: metadata only | `scope: "metadata"` | Only metadata dimension populated; others empty/absent | must |
| UT-37 | Twitter Card validation | twitter:card, twitter:title present | Validated alongside OG tags | should |
| UT-38 | Truncation for large page | 300 images | First 200 collected; truncation flag in response | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full SEO audit end-to-end | Go server + Extension content script + DOM collection | Complete audit with all 6 dimensions, correct issues, summary | must |
| IT-2 | Scoped audit (metadata only) | Go server + Extension | Only metadata collected and analyzed | must |
| IT-3 | Extension disconnected | Go server timeout | Error: extension not connected, no DOM data available | must |
| IT-4 | SPA with JS-injected meta tags | Extension reads rendered DOM | JS-injected tags (React Helmet, Next.js Head) correctly detected | should |
| IT-5 | Page with multiple JSON-LD blocks | Extension DOM collection | All blocks detected, each validated individually | should |
| IT-6 | Async DOM collection pipeline | Go server async commands + Extension | DOM data collected via existing analyze({what: "dom"}) infrastructure | must |
| IT-7 | Blank/loading page | Extension reads minimal DOM | Appropriate issues flagged (H1 missing, meta missing, etc.) | should |
| IT-8 | Page with iframes | Extension content script | Only top-level document audited; iframe content not included | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Full audit response time | End-to-end | < 500ms | must |
| PT-2 | Scoped audit (single dimension) | End-to-end | < 200ms | must |
| PT-3 | Extension DOM collection time | Content script execution | < 300ms | must |
| PT-4 | Server processing time | Parsing and validation | < 100ms | must |
| PT-5 | Main thread blocking (extension) | Page thread impact | 0ms (content script is async) | must |
| PT-6 | Memory impact | Transient data during collection | < 1MB | must |
| PT-7 | Large page: 200 images, 500 links | Collection time | < 500ms | should |
| PT-8 | Page with 10 JSON-LD blocks | Validation time | < 100ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No `<head>` element (malformed HTML) | Missing head | Metadata dimension: all fields null, status error. Body-based dimensions still work. | must |
| EC-2 | SPA with client-side routing | React/Vue/Angular SPA | Audit reads rendered DOM (correct for SPA SEO) | should |
| EC-3 | Blank page / loading spinner | Page not yet loaded | Minimal findings; issues flagged for missing elements | should |
| EC-4 | Multiple JSON-LD blocks | 3 separate LD+JSON scripts | All detected; types_detected lists all types; issues per block | should |
| EC-5 | Very large page (200+ images, 1000+ links) | Content-heavy page | Truncation at limits (200 images, 500 links); `truncated` flag set | must |
| EC-6 | Extension disconnected | Extension offline | Error indicating no DOM data available | must |
| EC-7 | Cross-origin iframes | Page with embedded iframes | Top-level document only; iframe content not audited | should |
| EC-8 | Microdata instead of JSON-LD | Structured data via itemprop | Not detected in v1 (JSON-LD only); no false positives | should |
| EC-9 | Empty heading text | `<h2>   </h2>` (whitespace only) | issue: HEADING_EMPTY with text "" | should |
| EC-10 | Title in wrong encoding | Non-UTF8 title characters | Title extracted as-is from DOM (browser handles encoding) | could |
| EC-11 | Invalid scope parameter | `scope: "seo"` | Error with valid scope values listed | must |
| EC-12 | Concurrent audit requests | Two SEO audits at same time | Both complete (stateless DOM collection) | should |
| EC-13 | Page with no links | Static page with no anchor tags | Links dimension: total 0, no issues (not an error) | should |
| EC-14 | Page with no images | Text-only page | Images dimension: total 0, no issues | should |
| EC-15 | OG tags with empty values | `<meta property="og:title" content="">` | Detected as present but flagged for empty value | should |
| EC-16 | Robots meta with multiple directives | `<meta name="robots" content="noindex, nofollow">` | Both directives parsed and reported | should |
| EC-17 | Duplicate meta descriptions | Two `<meta name="description">` tags | Issue: duplicate meta description | could |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web page with known SEO issues loaded (e.g., missing meta description, no canonical, missing alt text, skipped heading levels)
- [ ] Page fully rendered (wait for client-side JS to inject meta tags if SPA)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "generate", "arguments": {"type": "seo_audit"}}` | Server processes audit | Response with all 6 dimensions, summary with issue counts | [ ] |
| UAT-2 | Inspect metadata dimension | Compare to actual `<head>` content | Title, description, canonical, OG tags correctly detected; issues match actual page state | [ ] |
| UAT-3 | Inspect headings dimension | Compare to actual heading structure | Heading hierarchy matches page; skipped levels and missing H1 correctly flagged | [ ] |
| UAT-4 | Inspect images dimension | Compare to actual images on page | Missing alt text and dimensions correctly identified; selectors match actual elements | [ ] |
| UAT-5 | Inspect links dimension | Compare to page links | Internal/external counts accurate; empty hrefs flagged | [ ] |
| UAT-6 | Inspect structured_data dimension | View page source for JSON-LD blocks | JSON-LD blocks detected; schema types identified; missing properties flagged | [ ] |
| UAT-7 | Inspect technical dimension | Check viewport, lang, hreflang in page source | Viewport and lang correctly detected; hreflang presence/absence noted | [ ] |
| UAT-8 | Verify summary counts | Add up errors + warnings + info | total_issues matches sum of severity counts | [ ] |
| UAT-9 | Verify issue structure | Pick any issue | Has: severity, code, message, selector, current_value, expected, suggestion | [ ] |
| UAT-10 | `{"tool": "generate", "arguments": {"type": "seo_audit", "scope": "metadata"}}` | Only metadata | Response has metadata dimension populated; other dimensions empty or absent | [ ] |
| UAT-11 | `{"tool": "generate", "arguments": {"type": "seo_audit", "scope": "headings"}}` | Only headings | Response has headings dimension populated; other dimensions empty or absent | [ ] |
| UAT-12 | Fix an issue (e.g., add meta description), reload page, re-run audit | Issue count decreases | Previously flagged META_DESC_MISSING is gone; summary counts updated | [ ] |
| UAT-13 | Disconnect extension, run audit | Extension offline | Error: extension not connected, no DOM data | [ ] |
| UAT-14 | Navigate to SPA page with React Helmet, run audit | SPA with JS-injected meta | JS-injected meta tags correctly detected in audit | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No external requests during audit | Monitor all network traffic during audit | No outbound requests; audit is purely local DOM analysis | [ ] |
| DL-UAT-2 | DOM collection is read-only | Check page state before and after audit | No DOM modifications, no form submissions, no navigation | [ ] |
| DL-UAT-3 | No cookie/localStorage access | Verify extension content script does not read cookies | Only DOM elements (meta tags, headings, links, images, scripts) accessed | [ ] |
| DL-UAT-4 | Cross-origin iframes not accessed | Page with embedded cross-origin iframe | Only top-level document content in audit; no iframe data | [ ] |
| DL-UAT-5 | JSON-LD content is from page source | Inspect structured_data in response | JSON-LD matches what is in the page's `<script type="application/ld+json">` tags | [ ] |
| DL-UAT-6 | Truncation limits respected | Audit large page with 300+ images | Response has at most 200 images; truncated flag present | [ ] |

### Regression Checks
- [ ] Existing `generate` modes (csp, test, reproduction, har, sarif, best_practices_audit, performance_audit) still work
- [ ] Existing `observe({what: "page"})` still works independently
- [ ] Existing `observe({what: "accessibility"})` heading checks are independent from SEO heading checks
- [ ] DOM query infrastructure not affected for other features
- [ ] Extension performance not degraded for non-SEO audit operations

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

---
feature: seo-audit
status: proposed
version: null
tool: generate
mode: seo_audit
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-seo-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# SEO Audit

> Generates a structured on-page SEO audit from live browser telemetry, enabling AI coding agents to diagnose and fix SEO issues as part of the development workflow.

## Problem

SEO issues are among the most common, highest-impact problems that silently ship to production. Developers and AI agents working on web applications routinely make changes that break SEO without realizing it: removing a canonical tag during a refactor, introducing duplicate H1s, shipping images without alt text, breaking structured data markup, or losing Open Graph tags during a component rewrite.

Today, discovering these problems requires:

1. **Switching to a separate tool.** The developer (or AI agent) must leave the coding environment, open Lighthouse or an external SEO audit service, scan the page, then manually correlate findings back to their code changes.
2. **Interpreting unstructured output.** Lighthouse and similar tools produce reports designed for human consumption -- HTML dashboards, PDF reports, color-coded scores. An AI agent cannot easily parse these into actionable code changes.
3. **Running audits too late.** SEO problems are typically discovered after deployment, during a periodic audit, or when search rankings drop. By then, the damage is done and the root cause is harder to trace.

**Current state:** Kaboom already captures the live DOM, network traffic, and page metadata from the active browser tab. The `observe` tool can read page state and the `generate` tool can produce structured artifacts (tests, PRs, security reports). But there is no mode that synthesizes captured browser state into a structured SEO assessment that an AI agent can act on during development.

**The gap:** An AI coding agent working on a frontend feature should be able to request an SEO audit of the current page and receive machine-readable findings with specific issues, locations, and fix suggestions -- all without leaving the development loop.

## Solution

SEO Audit adds a new mode to the `generate` tool: `generate({format: "seo_audit"})`. When invoked, the server instructs the extension to collect on-page SEO signals from the active tab and returns a structured JSON report covering six audit dimensions:

1. **Metadata** -- title, meta description, canonical URL, Open Graph tags, Twitter Card tags, robots directives
2. **Heading structure** -- H1-H6 hierarchy, missing H1, multiple H1s, skipped levels
3. **Link structure** -- internal vs external links, nofollow attributes, empty hrefs, anchor text quality
4. **Image optimization** -- missing alt text, missing dimensions, oversized images, modern format usage
5. **Structured data** -- JSON-LD blocks, schema.org validation, required property checks
6. **Mobile & technical** -- viewport meta tag, font size legibility, tap target sizing, robots.txt/sitemap hints

Each finding is classified by severity (error, warning, info), includes the affected element or selector, and provides a machine-readable suggestion the AI agent can translate into a code fix.

The output is designed for LLM consumption: flat issue arrays with consistent schema, deterministic field names, and explicit context (selector paths, current values, expected values) so the AI can generate targeted fixes without additional DOM queries.

## User Stories

- As an AI coding agent, I want to audit the current page for SEO issues so that I can identify and fix problems before they reach production.
- As an AI coding agent, I want each SEO finding to include the affected element's selector and current value so that I can generate a targeted code fix without additional DOM queries.
- As an AI coding agent, I want findings categorized by severity (error, warning, info) so that I can prioritize the most impactful issues first.
- As a developer using Kaboom, I want the AI to catch SEO regressions (e.g., missing canonical tag after a refactor) during development so that I do not discover them after deployment.
- As an AI coding agent, I want structured data validation results so that I can fix JSON-LD markup errors that affect rich search results.
- As a developer using Kaboom, I want the SEO audit output as structured JSON so that I can integrate it into CI checks or automated review workflows.

## MCP Interface

**Tool:** `generate`
**Mode:** `seo_audit`

### Request

```json
{
  "tool": "generate",
  "arguments": {
    "type": "seo_audit",
    "scope": "full"
  }
}
```

#### Parameters:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | yes | Must be `"seo_audit"` |
| `scope` | string | no | Audit scope: `"full"` (all dimensions), `"metadata"`, `"headings"`, `"links"`, `"images"`, `"structured_data"`, `"technical"`. Defaults to `"full"`. |
| `url` | string | no | Override URL context for the report header. Defaults to the currently tracked tab's URL. Does not navigate; the audit always runs against the current DOM. |

### Response

```json
{
  "seo_audit": {
    "url": "https://example.com/products/widget",
    "audited_at": "2026-01-28T14:30:00Z",
    "scope": "full",
    "summary": {
      "total_issues": 8,
      "errors": 2,
      "warnings": 4,
      "info": 2,
      "dimensions_audited": ["metadata", "headings", "links", "images", "structured_data", "technical"]
    },
    "metadata": {
      "title": {
        "value": "Widget | Example Store",
        "length": 24,
        "status": "pass",
        "issues": []
      },
      "description": {
        "value": null,
        "length": 0,
        "status": "error",
        "issues": [
          {
            "severity": "error",
            "code": "META_DESC_MISSING",
            "message": "No meta description found. Search engines will auto-generate a snippet, which may not represent the page well.",
            "selector": "head",
            "current_value": null,
            "expected": "A meta description between 50-160 characters",
            "suggestion": "Add <meta name=\"description\" content=\"...\"> to the <head>"
          }
        ]
      },
      "canonical": {
        "value": null,
        "status": "warning",
        "issues": [
          {
            "severity": "warning",
            "code": "CANONICAL_MISSING",
            "message": "No canonical URL specified. This page may be treated as a duplicate if accessible via multiple URLs.",
            "selector": "head",
            "current_value": null,
            "expected": "A <link rel=\"canonical\"> pointing to the preferred URL",
            "suggestion": "Add <link rel=\"canonical\" href=\"https://example.com/products/widget\"> to the <head>"
          }
        ]
      },
      "og_tags": {
        "present": ["og:title", "og:type"],
        "missing": ["og:description", "og:image", "og:url"],
        "issues": [
          {
            "severity": "warning",
            "code": "OG_INCOMPLETE",
            "message": "Open Graph tags are incomplete. Missing: og:description, og:image, og:url. Social media previews will be degraded.",
            "selector": "head",
            "current_value": { "present": ["og:title", "og:type"] },
            "expected": "All core OG tags: og:title, og:description, og:image, og:url, og:type",
            "suggestion": "Add the missing og: meta tags to <head>"
          }
        ]
      },
      "robots": {
        "value": null,
        "status": "pass",
        "issues": []
      }
    },
    "headings": {
      "structure": [
        { "level": 1, "text": "Widget Product Page", "selector": "main > h1" },
        { "level": 2, "text": "Features", "selector": "main > section:nth-child(2) > h2" },
        { "level": 2, "text": "Reviews", "selector": "main > section:nth-child(3) > h2" },
        { "level": 4, "text": "Top Review", "selector": "main > section:nth-child(3) > div > h4" }
      ],
      "h1_count": 1,
      "issues": [
        {
          "severity": "warning",
          "code": "HEADING_SKIP_LEVEL",
          "message": "Heading level skipped: H2 -> H4. Skipping levels breaks the document outline and harms accessibility and SEO.",
          "selector": "main > section:nth-child(3) > div > h4",
          "current_value": "h4 (\"Top Review\")",
          "expected": "h3 after h2",
          "suggestion": "Change the <h4> to <h3> to maintain heading hierarchy"
        }
      ]
    },
    "links": {
      "total": 42,
      "internal": 35,
      "external": 7,
      "nofollow": 2,
      "issues": [
        {
          "severity": "info",
          "code": "LINK_EMPTY_HREF",
          "message": "1 link has an empty or '#' href attribute.",
          "selector": "footer > nav > a:nth-child(3)",
          "current_value": "href=\"#\"",
          "expected": "A valid URL or meaningful fragment identifier",
          "suggestion": "Replace empty href with a valid destination or use a <button> for interactive elements"
        }
      ]
    },
    "images": {
      "total": 12,
      "missing_alt": 3,
      "missing_dimensions": 5,
      "issues": [
        {
          "severity": "error",
          "code": "IMG_MISSING_ALT",
          "message": "3 images are missing alt text. Screen readers cannot describe these images, and search engines cannot index them.",
          "selectors": [
            "main > section:nth-child(2) > img:nth-child(1)",
            "main > section:nth-child(2) > img:nth-child(2)",
            "aside > img"
          ],
          "current_value": null,
          "expected": "Descriptive alt attribute on all non-decorative images",
          "suggestion": "Add alt=\"...\" to each image. Use alt=\"\" only for purely decorative images."
        },
        {
          "severity": "warning",
          "code": "IMG_MISSING_DIMENSIONS",
          "message": "5 images are missing explicit width/height attributes, which contributes to Cumulative Layout Shift (CLS).",
          "selectors": [
            "main > section:nth-child(2) > img:nth-child(1)",
            "main > section:nth-child(2) > img:nth-child(2)",
            "main > section:nth-child(3) > img",
            "aside > img",
            "footer > img"
          ],
          "current_value": "No width/height attributes",
          "expected": "Explicit width and height attributes on <img> elements",
          "suggestion": "Add width and height attributes to prevent layout shifts during loading"
        }
      ]
    },
    "structured_data": {
      "blocks_found": 1,
      "types_detected": ["Product"],
      "issues": [
        {
          "severity": "warning",
          "code": "SCHEMA_MISSING_FIELD",
          "message": "Product schema is missing recommended properties: 'offers', 'aggregateRating'. Rich results may not appear.",
          "selector": "script[type='application/ld+json']",
          "current_value": { "@type": "Product", "name": "Widget", "description": "A great widget" },
          "expected": "Product schema with name, description, offers, aggregateRating, image",
          "suggestion": "Add 'offers' (with price, priceCurrency, availability) and 'aggregateRating' to the JSON-LD block"
        }
      ]
    },
    "technical": {
      "viewport": {
        "present": true,
        "value": "width=device-width, initial-scale=1",
        "status": "pass"
      },
      "lang_attribute": {
        "present": true,
        "value": "en",
        "status": "pass"
      },
      "issues": [
        {
          "severity": "info",
          "code": "NO_HREFLANG",
          "message": "No hreflang tags detected. If this page is available in multiple languages, hreflang tags help search engines serve the correct version.",
          "selector": "head",
          "current_value": null,
          "expected": "<link rel=\"alternate\" hreflang=\"...\"> tags for each language variant",
          "suggestion": "Add hreflang tags if the site has multilingual content. Skip if single-language."
        }
      ]
    }
  }
}
```

#### Response fields:

| Field | Type | Description |
|-------|------|-------------|
| `seo_audit.url` | string | The URL of the audited page |
| `seo_audit.audited_at` | string (ISO 8601) | Timestamp of the audit |
| `seo_audit.scope` | string | The scope that was audited |
| `seo_audit.summary` | object | Counts of issues by severity and list of dimensions audited |
| `seo_audit.summary.total_issues` | number | Total number of issues found |
| `seo_audit.summary.errors` | number | Count of error-severity issues |
| `seo_audit.summary.warnings` | number | Count of warning-severity issues |
| `seo_audit.summary.info` | number | Count of informational issues |
| `seo_audit.metadata` | object | Title, description, canonical, OG tags, robots directive audit |
| `seo_audit.headings` | object | H1-H6 structure analysis with hierarchy validation |
| `seo_audit.links` | object | Internal/external link counts and issue detection |
| `seo_audit.images` | object | Alt text, dimension, and format audit |
| `seo_audit.structured_data` | object | JSON-LD / schema.org validation |
| `seo_audit.technical` | object | Viewport, lang, hreflang, and mobile-readiness checks |

#### Issue object schema (consistent across all dimensions):

| Field | Type | Description |
|-------|------|-------------|
| `severity` | enum | `error`, `warning`, `info` |
| `code` | string | Machine-readable issue code (e.g., `META_DESC_MISSING`) |
| `message` | string | Human-readable description of the issue |
| `selector` | string or array | CSS selector(s) for the affected element(s) |
| `current_value` | any | The current value found (or `null` if missing) |
| `expected` | string | What was expected |
| `suggestion` | string | Actionable fix suggestion |

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | `generate({format: "seo_audit"})` returns a structured JSON report covering metadata, headings, links, images, structured data, and technical SEO factors | must |
| R2 | Metadata audit validates: title presence and length (1-60 chars), meta description presence and length (50-160 chars), canonical URL, robots directives | must |
| R3 | Metadata audit validates Open Graph tags (og:title, og:description, og:image, og:url, og:type) and reports missing core tags | must |
| R4 | Heading audit extracts the full H1-H6 hierarchy with text content and selectors, reports missing H1, multiple H1s, and skipped heading levels | must |
| R5 | Image audit identifies images missing alt text and images missing explicit width/height attributes | must |
| R6 | Each issue includes a machine-readable `code`, `severity`, `selector`, `current_value`, `expected`, and `suggestion` | must |
| R7 | Response includes a `summary` object with total issue count and breakdown by severity | must |
| R8 | Link audit counts internal vs external links, identifies empty/placeholder hrefs, and flags links with no anchor text | should |
| R9 | Structured data audit detects JSON-LD blocks, identifies the schema.org type, and validates required/recommended properties for common types (Product, Article, BreadcrumbList, Organization, FAQ) | should |
| R10 | Technical audit checks viewport meta tag, html lang attribute, and hreflang alternate links | should |
| R11 | The `scope` parameter allows limiting the audit to a single dimension (e.g., `"metadata"` only) for faster, targeted checks | should |
| R12 | Twitter Card meta tags (twitter:card, twitter:title, twitter:description, twitter:image) are validated alongside OG tags | should |
| R13 | Image audit detects images served in non-modern formats (not WebP/AVIF) and flags them as optimization opportunities | could |
| R14 | Link audit detects nofollow attributes and reports the ratio of nofollow to total external links | could |
| R15 | Structured data audit validates nested schema objects (e.g., Offer inside Product) for required subfields | could |
| R16 | Technical audit detects whether the page includes a reference to robots.txt and sitemap.xml (via link tags or well-known paths) | could |
| R17 | Mobile-friendliness checks: font size legibility (>= 12px base), tap target sizing (>= 48x48px touch targets) | could |

## Non-Goals

- **This feature does NOT crawl the site.** The audit runs against the single page currently loaded in the tracked browser tab. Multi-page audits, sitemap crawling, and cross-page duplicate content detection are out of scope. The AI agent can invoke the audit on multiple pages sequentially by navigating via `interact({action: "navigate"})`.

- **This feature does NOT check external link reachability.** Detecting broken external links requires outbound HTTP requests, which Kaboom does not make. The link audit covers structural issues (empty hrefs, missing anchor text) but not 404 detection for external URLs. Internal link status can be inferred from network waterfall data if the page has been navigated.

- **This feature does NOT produce a Lighthouse score.** Lighthouse is a comprehensive performance, accessibility, SEO, and best-practices audit tool with its own scoring methodology. Kaboom's SEO audit covers the on-page SEO dimension with a focus on machine-readable output. It does not replicate Lighthouse's scoring system, PWA checks, or JavaScript execution analysis.

- **This feature does NOT replace accessibility audits.** Kaboom already has `observe({what: "accessibility"})` powered by axe-core. The SEO audit touches heading hierarchy and alt text (which overlap with accessibility) but does not duplicate the full WCAG audit. The AI agent should use both tools for comprehensive coverage.

- **This feature does NOT create a new MCP tool.** It adds a new mode (`seo_audit`) to the existing `generate` tool, respecting the 5-tool model.

- **This feature does NOT modify the page.** It is read-only. The AI agent applies fixes using the `interact` tool or by editing source code. Kaboom reports issues; the agent resolves them.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Full audit response time | < 500ms (DOM collection via extension + server processing) |
| Scoped audit response time (single dimension) | < 200ms |
| Extension DOM collection time | < 300ms (must not block main thread; runs in content script) |
| Server processing time | < 100ms (parsing and validating collected data) |
| Memory impact | < 1MB transient (collected DOM data processed and released) |

## Security Considerations

- **No new data capture mechanisms.** The SEO audit reads from the current DOM state using the same `analyze({what: "dom"})` / content script infrastructure that existing features use. It does not introduce new capture channels or expand the attack surface.

- **Page content exposure.** The audit response includes page metadata (title, description, OG tags), heading text, link URLs, image URLs, and JSON-LD content. These are all publicly visible on the rendered page. No authentication tokens, cookies, or server-side data is exposed.

- **URL handling.** URLs in the response (canonical, OG, links, images) are taken directly from the DOM. Query parameters that may contain tracking IDs or session tokens are included as-is. The existing privacy layer (header stripping, URL scrubbing configured via `configure`) applies to network telemetry but not to DOM-extracted URLs. This is acceptable because these URLs are already visible in the page source.

- **Structured data content.** JSON-LD blocks are returned in full for validation purposes. If a developer has embedded sensitive data in JSON-LD (unlikely but possible), it will appear in the audit response. This matches the behavior of `observe({what: "page"})` which already exposes page content.

- **No outbound requests.** The audit is entirely local. No data is sent to external SEO analysis services, Google APIs, or validation endpoints.

## Edge Cases

- **Page has no `<head>` element (e.g., malformed HTML).** Expected behavior: Metadata dimension returns all fields as `null` with `status: "error"` and an issue explaining that no `<head>` was found. Other dimensions (headings, links, images) still audit the `<body>`.

- **Page is a single-page application with client-side routing.** Expected behavior: The audit runs against the current rendered DOM, which includes dynamically inserted content. This is the correct behavior for SPA SEO audits since search engine crawlers with JavaScript rendering see the same DOM.

- **Page has no content (blank page, loading spinner).** Expected behavior: The audit returns minimal findings -- metadata may be present (from the static HTML shell) but headings, links, and images will be empty. Issues are flagged as appropriate (e.g., "No H1 found"). The AI agent should ensure the page has finished loading before requesting an audit.

- **Multiple JSON-LD blocks on the page.** Expected behavior: All blocks are detected and validated individually. The `blocks_found` count reflects the total, and `types_detected` lists all schema types. Issues are reported per block with selectors distinguishing them (e.g., `script[type='application/ld+json']:nth-of-type(2)`).

- **Very large page (hundreds of images, thousands of links).** Expected behavior: The extension collects data up to a reasonable limit (e.g., first 500 images, first 1000 links). The response includes a `truncated` flag if limits were hit. Processing remains within the 500ms SLO by streaming results rather than collecting everything before processing.

- **Extension disconnected.** Expected behavior: The server returns an error indicating that the extension is not connected and no DOM data is available. Same pattern as other features that require extension communication.

- **Page uses iframes with cross-origin content.** Expected behavior: The audit covers only the top-level document. Cross-origin iframe content is inaccessible to the content script. The response does not flag this as an issue (iframed content has its own SEO context).

- **Structured data uses Microdata or RDFa instead of JSON-LD.** Expected behavior: The v1 implementation audits JSON-LD only, which is the format recommended by Google and used by the vast majority of modern sites. Microdata and RDFa support is deferred (see OI-3).

- **Heading text is empty or whitespace-only.** Expected behavior: The heading is included in the structure with `text: ""` and an issue is flagged with code `HEADING_EMPTY`.

## Dependencies

- **Depends on:**
  - **Extension content script DOM access** (shipped) -- Reading meta tags, headings, links, images, and script elements from the current page.
  - **`analyze({what: "dom"})`** (shipped) -- DOM query infrastructure for collecting structured data from the page.
  - **Async command architecture** (shipped) -- The extension collects DOM data asynchronously and posts results back to the server.

- **Optionally composes with:**
  - **`observe({what: "network_waterfall"})`** (shipped) -- Can be used by the AI agent to cross-reference image URLs with actual response sizes and content types for more accurate image optimization findings.
  - **`observe({what: "accessibility"})`** (shipped) -- Complementary audit; the AI agent can run both for comprehensive page quality analysis.
  - **`observe({what: "page"})`** (shipped) -- Provides page metadata that overlaps with the SEO audit. The SEO audit is a specialized, deeper analysis of SEO-specific factors.

- **Depended on by:**
  - None currently. This is a standalone audit capability.

## Assumptions

- A1: The extension is connected and tracking a tab with a loaded page.
- A2: The page has finished loading and rendering (the DOM reflects the final page state). The AI agent should wait for page load before requesting an audit.
- A3: The audit runs against the top-level document only (not iframe content).
- A4: JSON-LD is the primary structured data format for v1. Microdata and RDFa are out of scope.
- A5: Issue severity classifications follow Google's SEO best practices documentation as the reference standard.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should the audit include a "diff from last audit" capability? | open | Useful for the AI to detect SEO regressions after making changes (e.g., "you removed the canonical tag"). This could be a follow-up mode or a parameter on the current mode. Analogous to `diff_sessions` for SEO state. |
| OI-2 | How should structured data validation depth be determined? | open | Full schema.org validation is extensive (hundreds of types, thousands of properties). Propose: validate only the top 10 most common types (Product, Article, BreadcrumbList, Organization, FAQ, HowTo, LocalBusiness, Event, Recipe, VideoObject) and flag unknown types without deep validation. |
| OI-3 | Should Microdata and RDFa structured data formats be supported? | open | JSON-LD is dominant (~80% of structured data on the web), but some legacy sites use Microdata. Propose: JSON-LD only for v1, with Microdata as a "could" for v2. |
| OI-4 | Should the extension-side collection be a single query or multiple targeted queries? | open | A single comprehensive query is simpler but may be slower. Multiple targeted queries (one per dimension) enable scoped audits to skip unnecessary collection. Propose: single collection for v1 with scope-based filtering on the server side. |
| OI-5 | What is the appropriate truncation limit for very large pages? | open | Pages with hundreds of images or thousands of links need a cap to stay within performance SLOs. Propose: 200 images, 500 links, 50 headings. Report truncation in the response. |
| OI-6 | Should issue codes follow an existing standard (e.g., Lighthouse audit IDs)? | open | Using Lighthouse-compatible codes would allow cross-referencing with existing SEO tools. However, Kaboom's audit scope differs. Propose: custom codes with a mapping table to Lighthouse equivalents where applicable. |
| OI-7 | Should the audit detect meta tags injected by JavaScript (e.g., React Helmet, Next.js Head)? | open | Since the audit reads the rendered DOM (not source HTML), JS-injected tags are naturally included. This is correct behavior for SPA SEO audits. Confirm this is the desired behavior and document it explicitly. |

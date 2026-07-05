---
feature: enhanced-wcag-audit
status: proposed
version: null
tool: observe
mode: accessibility
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-enhanced-wcag-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Enhanced WCAG Accessibility Audit

> Extends the existing `observe {what: "accessibility"}` mode with deeper WCAG compliance analysis, structured output by conformance level, and actionable remediation guidance optimized for LLM consumption.

## Problem

The current accessibility audit uses axe-core with default configuration, which catches common violations but leaves significant gaps in WCAG compliance coverage. AI coding agents fixing accessibility issues need:

1. **Conformance-level filtering** -- the ability to audit against a specific WCAG level (A, AA, AAA) rather than receiving an undifferentiated list of violations.
2. **Deeper analysis beyond axe-core defaults** -- color contrast across all text/background combinations, keyboard trap detection, ARIA role hierarchy validation, heading structure analysis, skip link detection, focus indicator visibility, and screen reader text sufficiency.
3. **Remediation guidance** -- each violation needs a severity level, estimated fix effort, and concrete code-level fix suggestions so the LLM can act on findings without additional research.
4. **Structured output by WCAG criterion** -- violations grouped by success criterion (e.g., 1.4.3 Contrast) so the agent can prioritize and batch related fixes.

Without these capabilities, an AI agent must run the basic audit, interpret raw axe-core output, manually research WCAG criteria, and guess at fixes -- a workflow that wastes tokens and produces inconsistent results.

## Solution

Extend the existing `observe {what: "accessibility"}` mode with two new optional parameters:

- **`wcag_level`** (`"A"`, `"AA"`, `"AAA"`) -- filters axe-core rules to the specified conformance level and below, and groups output by WCAG success criterion.
- **`detailed`** (`true`/`false`) -- enables enhanced post-processing that adds remediation guidance, severity classification, effort estimates, and additional checks beyond axe-core's default ruleset.

The existing behavior is fully preserved. Calling `observe {what: "accessibility"}` without these parameters produces identical output to today. The new parameters are additive -- they enhance the response structure without changing the underlying axe-core integration.

**Implementation strategy:** ~200 lines of extension JavaScript (enhanced axe-core configuration + post-processing logic), plus server-side parameter pass-through (~30 lines Go).

## User Stories

- As an AI coding agent, I want to audit a page against WCAG AA specifically so that I can focus fixes on the conformance level the project targets.
- As an AI coding agent, I want remediation guidance with each violation so that I can generate fix code without researching WCAG success criteria separately.
- As an AI coding agent, I want violations grouped by WCAG criterion so that I can batch related fixes (e.g., fix all contrast issues together).
- As a developer using Kaboom, I want to run `observe {what: "accessibility", wcag_level: "AAA", detailed: true}` and get a comprehensive compliance report I can hand to my AI assistant for remediation.
- As an AI coding agent, I want severity levels and effort estimates so that I can prioritize fixes by impact and suggest a remediation order to the developer.

## MCP Interface

**Tool:** `observe`
**Mode:** `accessibility` (existing -- extended with new parameters)

### Request (basic -- unchanged)

```json
{
  "tool": "observe",
  "arguments": {
    "what": "accessibility"
  }
}
```

Response is identical to current behavior. No breaking changes.

### Request (enhanced -- new parameters)

```json
{
  "tool": "observe",
  "arguments": {
    "what": "accessibility",
    "wcag_level": "AA",
    "detailed": true,
    "scope": "#main-content",
    "force_refresh": true
  }
}
```

### Response (basic mode -- unchanged)

```json
{
  "violations": [
    {
      "id": "color-contrast",
      "impact": "serious",
      "description": "Elements must have sufficient color contrast",
      "helpUrl": "https://dequeuniversity.com/rules/axe/4.8/color-contrast",
      "wcag": ["wcag143"],
      "nodes": [
        {
          "selector": ".header-text",
          "html": "<span class=\"header-text\">Welcome</span>",
          "failureSummary": "Fix any of the following: ..."
        }
      ]
    }
  ],
  "summary": {
    "violations": 3,
    "passes": 42,
    "incomplete": 1,
    "inapplicable": 15
  }
}
```

### Response (enhanced mode -- `detailed: true`)

```json
{
  "wcag_level": "AA",
  "violations": [
    {
      "id": "color-contrast",
      "impact": "serious",
      "severity": "high",
      "wcag_criterion": "1.4.3",
      "wcag_criterion_name": "Contrast (Minimum)",
      "wcag_level": "AA",
      "description": "Elements must have sufficient color contrast",
      "helpUrl": "https://dequeuniversity.com/rules/axe/4.8/color-contrast",
      "remediation": {
        "summary": "Increase foreground/background contrast ratio to meet 4.5:1 for normal text or 3:1 for large text",
        "effort": "low",
        "fix_suggestion": "Change color from #767676 to #595959 or darker, or lighten the background"
      },
      "nodes": [
        {
          "selector": ".header-text",
          "html": "<span class=\"header-text\">Welcome</span>",
          "failureSummary": "Element has insufficient color contrast of 4.08 (foreground: #767676, background: #ffffff, required ratio: 4.5:1)",
          "computed": {
            "foreground": "#767676",
            "background": "#ffffff",
            "contrast_ratio": 4.08,
            "required_ratio": 4.5
          }
        }
      ]
    }
  ],
  "checks": {
    "heading_hierarchy": {
      "status": "fail",
      "details": "Heading levels skip from h1 to h3 (missing h2)",
      "headings_found": ["h1", "h3", "h3", "h4"],
      "remediation": "Add h2 headings or restructure to maintain sequential order"
    },
    "skip_links": {
      "status": "pass",
      "details": "Skip navigation link found targeting #main-content"
    },
    "form_labels": {
      "status": "fail",
      "details": "2 form inputs missing associated labels",
      "elements": [
        {"selector": "#email-input", "html": "<input id=\"email-input\" type=\"email\">"},
        {"selector": "#phone", "html": "<input id=\"phone\" type=\"tel\">"}
      ],
      "remediation": "Add <label for=\"...\"> elements or aria-label attributes"
    },
    "keyboard_traps": {
      "status": "pass",
      "details": "No keyboard traps detected in tab order simulation"
    },
    "focus_indicators": {
      "status": "warn",
      "details": "3 interactive elements have outline:none without alternative focus styles",
      "elements": [
        {"selector": ".btn-primary", "html": "<button class=\"btn-primary\">Submit</button>"}
      ],
      "remediation": "Add visible focus styles (outline, box-shadow, or border) to replace removed outlines"
    },
    "aria_validation": {
      "status": "fail",
      "details": "1 element has invalid ARIA role hierarchy",
      "elements": [
        {"selector": "[role='option']", "html": "<div role=\"option\">Item 1</div>", "issue": "role='option' must be owned by role='listbox'"}
      ],
      "remediation": "Wrap role='option' elements in a container with role='listbox'"
    },
    "screen_reader_text": {
      "status": "warn",
      "details": "4 images missing alt text, 1 icon button missing accessible name",
      "elements": [
        {"selector": "img.hero-image", "html": "<img class=\"hero-image\" src=\"/hero.jpg\">"},
        {"selector": ".icon-btn-close", "html": "<button class=\"icon-btn-close\"><svg>...</svg></button>"}
      ],
      "remediation": "Add alt attributes to images; add aria-label to icon-only buttons"
    }
  },
  "summary": {
    "violations": 5,
    "passes": 42,
    "incomplete": 1,
    "inapplicable": 15,
    "wcag_level_tested": "AA",
    "checks_run": 7,
    "checks_passed": 2,
    "checks_failed": 3,
    "checks_warned": 2
  },
  "priority_order": [
    {"id": "color-contrast", "severity": "high", "effort": "low", "reason": "High impact, low effort -- fix first"},
    {"id": "form_labels", "severity": "high", "effort": "low", "reason": "Critical for screen reader users"},
    {"id": "aria_validation", "severity": "medium", "effort": "low", "reason": "Incorrect semantics confuse assistive technology"},
    {"id": "screen_reader_text", "severity": "medium", "effort": "low", "reason": "Missing accessible names"},
    {"id": "heading_hierarchy", "severity": "low", "effort": "low", "reason": "Structural improvement for navigation"},
    {"id": "focus_indicators", "severity": "medium", "effort": "medium", "reason": "Keyboard users cannot see focus"}
  ]
}
```

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | `wcag_level` parameter filters axe-core rules to specified level (A, AA, AAA) and all levels below | must |
| R2 | `detailed: true` enables enhanced post-processing with remediation guidance | must |
| R3 | Existing behavior unchanged when new parameters are omitted (full backward compatibility) | must |
| R4 | Violations grouped and annotated by WCAG success criterion (number + name) | must |
| R5 | Each violation includes severity (`critical`, `high`, `medium`, `low`) derived from axe-core impact + WCAG level | must |
| R6 | Each violation includes remediation object with summary, effort estimate, and fix suggestion | must |
| R7 | Heading hierarchy check: validates sequential heading levels (h1 > h2 > h3, no skips) | must |
| R8 | Form label completeness check: detects inputs without associated labels or aria-label | must |
| R9 | ARIA attribute validation: role hierarchy (e.g., option inside listbox), required attributes | must |
| R10 | Skip link detection: checks for skip navigation links targeting main content | should |
| R11 | Focus indicator visibility: detects `outline:none` / `outline:0` without alternative focus styles | should |
| R12 | Screen reader text sufficiency: images without alt, icon buttons without accessible names | should |
| R13 | Color contrast enhanced data: includes computed foreground/background colors and contrast ratios in node details | should |
| R14 | Keyboard trap detection: simulates tab order and detects elements that trap focus | should |
| R15 | Priority order array: violations sorted by severity (desc) then effort (asc) for LLM-guided remediation | should |
| R16 | Server-side caching respects new parameters (cache key includes `wcag_level` and `detailed` flags) | must |
| R17 | All enhanced checks run within the existing `A11Y_AUDIT_TIMEOUT_MS` (30s) budget | must |
| R18 | Output size stays within reasonable token limits (cap nodes per check, truncate HTML) | must |

## Non-Goals

- This feature does NOT replace axe-core. Axe-core remains the primary audit engine; enhanced checks supplement it with additional analysis.
- This feature does NOT perform automated fixes. It provides remediation guidance that an AI agent or developer acts on.
- This feature does NOT add a new MCP tool. It extends the existing `observe` tool's `accessibility` mode.
- This feature does NOT support custom WCAG success criteria. It uses the standard WCAG 2.1 taxonomy.
- Out of scope: WCAG 2.2 criteria (can be added later as axe-core updates).
- Out of scope: PDF accessibility auditing (Kaboom operates on rendered DOM only).
- Out of scope: multi-page / site-wide crawling (Kaboom audits the current page/scope).

## Performance SLOs

| Metric | Target | Notes |
|--------|--------|-------|
| Basic audit (no new params) | < 5s | Unchanged from current behavior |
| Enhanced audit (`detailed: true`) | < 10s | Post-processing adds overhead; must stay under 10s |
| Enhanced audit total | < 30s | Must complete within existing `A11Y_AUDIT_TIMEOUT_MS` |
| Extension memory impact | < 2MB | Enhanced checks operate on existing DOM; no large data structures |
| Main thread blocking | < 50ms per check | Each enhanced check must yield; never block main thread |
| Response payload size | < 100KB | Capped nodes, truncated HTML, bounded check output |

## Security Considerations

- **Data captured:** DOM structure, computed styles, ARIA attributes, heading text, form field names, CSS selectors. All data already accessible to the existing accessibility audit.
- **Data NOT captured:** Form field values, user input, authentication state. No new sensitive data surfaces.
- **Redaction:** HTML snippets are truncated to `DOM_QUERY_MAX_HTML` (200 chars). No expansion of existing truncation limits.
- **Privacy implications:** None beyond existing audit. All processing happens locally in the browser; results are sent only to the localhost Kaboom server.
- **Attack surface:** No change. No new endpoints, no new extension permissions, no new external requests. The enhanced checks use the same axe-core injection path and the same `/a11y-result` POST endpoint.
- **axe-core version:** Uses the already-bundled axe-core (4.8.4). No new third-party dependencies.

## Edge Cases

- What happens when `wcag_level` is an invalid value (e.g., `"B"`)? Expected behavior: return a structured error with valid enum values (`A`, `AA`, `AAA`).
- What happens when `detailed: true` but the page has no violations? Expected behavior: return the enhanced structure with empty violations array, all checks reporting `pass` status, and empty priority order.
- What happens when axe-core fails to load but `detailed: true` is set? Expected behavior: return the enhanced checks that do not depend on axe-core (heading hierarchy, skip links, form labels, ARIA validation) with a warning that axe-core checks were skipped.
- What happens when a heading hierarchy check runs on a page with no headings? Expected behavior: check status `warn` with details "No headings found on page".
- What happens when `scope` is specified with `detailed: true`? Expected behavior: both axe-core and enhanced checks are scoped to the specified CSS selector.
- What happens when the enhanced audit exceeds `A11Y_AUDIT_TIMEOUT_MS`? Expected behavior: return partial results (whatever completed before timeout) with a `timeout: true` flag and list of checks that did not complete.
- What happens when the extension is disconnected mid-audit? Expected behavior: existing timeout handling applies; server returns the standard extension timeout error.
- What happens when `detailed: true` and `tags` are both specified? Expected behavior: `tags` filters axe-core rules as today; `detailed` adds post-processing and enhanced checks. Both parameters compose.
- What happens when cached results exist for basic mode but `detailed: true` is requested? Expected behavior: cache miss (cache key includes `detailed` flag), fresh audit runs.

## Dependencies

- **Depends on:** Existing axe-core integration (`extension/lib/dom-queries.js`: `loadAxeCore`, `runAxeAudit`, `formatAxeResults`)
- **Depends on:** Existing server-side a11y audit handler (`cmd/browser-agent/queries.go`: `toolRunA11yAudit`, a11y cache infrastructure)
- **Depends on:** Existing pending query / result pipeline (`/pending-queries`, `/a11y-result`)
- **Depended on by:** `generate {format: "sarif"}` -- SARIF export consumes cached a11y results; enhanced results should export with additional metadata when available

## Assumptions

- A1: The extension is connected and actively tracking a tab with a loaded page (standard prerequisite for all `observe` queries).
- A2: axe-core 4.8.4 (bundled) supports tag-based filtering by WCAG level (confirmed: `wcag2a`, `wcag2aa`, `wcag21aa`, `wcag2aaa` tags).
- A3: The DOM is stable at audit time (no in-flight rendering that would change heading structure or ARIA attributes during the check).
- A4: Enhanced checks (heading hierarchy, form labels, ARIA validation, etc.) can be implemented as pure DOM queries without additional library dependencies, consistent with the zero-deps extension constraint.
- A5: The existing `A11Y_AUDIT_TIMEOUT_MS` (30s) budget is sufficient for axe-core + all enhanced checks on typical web pages.
- A6: LLM consumers can parse the enhanced response schema; the priority order array is the primary entry point for remediation workflows.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should `wcag_level` accept `"2.2"` level tags for future-proofing? | open | axe-core 4.8.4 has partial WCAG 2.2 support; could add `wcag22aa` tag mapping |
| OI-2 | Should keyboard trap detection run by default or only with an explicit flag? | open | Tab simulation is the most expensive enhanced check; may need its own opt-in |
| OI-3 | Should enhanced results feed into SARIF export with additional fields? | open | Current SARIF export reads cached a11y results; needs schema alignment |
| OI-4 | Should remediation `fix_suggestion` include concrete CSS/HTML snippets or stay generic? | open | Concrete snippets are more useful for LLMs but harder to generate accurately |
| OI-5 | Maximum number of elements reported per enhanced check | open | Currently using `A11Y_MAX_NODES_PER_VIOLATION` (10); may need separate limit for checks like `screen_reader_text` |

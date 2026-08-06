---
title: "Accessibility Testing"
description: "Run WCAG accessibility audits with KaBOOM's built-in axe-core engine. Scope audits, filter by WCAG tags, export SARIF reports for CI/CD, and track remediation progress."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'accessibility']
---

KaBOOM includes a full accessibility auditing engine powered by [axe-core](https://github.com/dequelabs/axe-core). Your AI assistant can audit pages for WCAG violations, explain what's wrong and why it matters, scope audits to specific components, and export results in SARIF format for CI/CD integration.

## Quick Start

Ask your AI to audit the current page:

```
"Run an accessibility audit on this page."
```

```js
analyze({what: "accessibility"})
```

Returns a list of violations, each with:

| Field | Description |
|-------|-------------|
| `id` | axe-core rule ID (e.g., `color-contrast`, `image-alt`, `label`) |
| `impact` | `critical`, `serious`, `moderate`, `minor` |
| `description` | What the rule checks |
| `help` | How to fix the issue |
| `helpUrl` | Link to axe-core documentation with full explanation |
| `nodes` | List of failing DOM elements with selectors and HTML snippets |

---

## Scoped Audits

Audit a specific section instead of the entire page:

```js
analyze({what: "accessibility", scope: "#main-content"})
analyze({what: "accessibility", scope: ".checkout-form"})
analyze({what: "accessibility", scope: "[role='navigation']"})
```

The `scope` parameter accepts any CSS selector. Only elements within that subtree are audited. Useful for:
- Testing a specific component you just built
- Auditing a modal or dialog
- Checking a form before shipping
- Ignoring third-party widgets you can't control

---

## Filter by WCAG Tags

Target specific WCAG success criteria:

```js
analyze({what: "accessibility", tags: ["wcag2a"]})
analyze({what: "accessibility", tags: ["wcag2aa"]})
analyze({what: "accessibility", tags: ["wcag21aa"]})
analyze({what: "accessibility", tags: ["best-practice"]})
```

| Tag | What It Covers |
|-----|---------------|
| `wcag2a` | WCAG 2.0 Level A (minimum compliance) |
| `wcag2aa` | WCAG 2.0 Level AA (standard compliance target) |
| `wcag21a` | WCAG 2.1 Level A |
| `wcag21aa` | WCAG 2.1 Level AA |
| `wcag22aa` | WCAG 2.2 Level AA |
| `best-practice` | Non-WCAG rules that improve accessibility |
| `section508` | Section 508 requirements |

Combine tags to run multiple rule sets:

```js
analyze({what: "accessibility", tags: ["wcag2aa", "best-practice"]})
```

---

## Force Refresh

Results are cached for performance. To re-run the audit after making changes:

```js
analyze({what: "accessibility", force_refresh: true})
```

This clears the cache and runs a fresh axe-core scan.

---

## SARIF Export

Export results in [SARIF](https://sarifweb.azurewebsites.net/) (Static Analysis Results Interchange Format) for CI/CD integration:

```js
generate({what: "sarif"})
generate({what: "sarif", save_to: "/path/to/a11y-report.sarif"})
generate({what: "sarif", scope: "#main-content", include_passes: true})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `scope` | string | CSS selector to limit audit scope |
| `include_passes` | boolean | Include passing rules (not just violations) |
| `save_to` | string | File path to save the SARIF file |

### CI/CD Integration

Upload SARIF to GitHub Code Scanning:

```yaml
# .github/workflows/a11y.yml
- name: Upload SARIF
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: a11y-report.sarif
```

This puts accessibility violations directly in your PR's "Code scanning alerts" tab — visible to reviewers alongside code changes.

### Other SARIF-Compatible Tools

- **VS Code** — install the SARIF Viewer extension to browse results in your editor
- **Azure DevOps** — upload as a build artifact for security/quality gates
- **SonarQube** — import via the SARIF import plugin

---

## Common Violations and Fixes

### color-contrast (critical/serious)

**What it means**: Text doesn't have enough contrast against its background. Low-vision users can't read it.

**WCAG requirement**: 4.5:1 for normal text, 3:1 for large text (AA level).

**How the AI fixes it**: Reads the computed colors, calculates the contrast ratio, and adjusts either the text or background color to meet the threshold.

### image-alt (critical)

**What it means**: An `<img>` element is missing the `alt` attribute. Screen readers can't describe the image.

**How the AI fixes it**: Examines the image context and suggests appropriate alt text. Decorative images get `alt=""`.

### label (critical)

**What it means**: A form input doesn't have an associated `<label>`. Screen readers can't tell users what the input is for.

**How the AI fixes it**: Adds a `<label for="...">` element or an `aria-label` attribute.

### button-name (critical)

**What it means**: A button has no accessible name. Screen readers announce it as just "button."

**How the AI fixes it**: Adds text content, an `aria-label`, or an `aria-labelledby` attribute.

### link-name (serious)

**What it means**: A link has no accessible name (e.g., `<a href="..."><img src="icon.png"></a>`).

**How the AI fixes it**: Adds `aria-label` to the link or `alt` text to the image inside it.

### heading-order (moderate)

**What it means**: Heading levels skip (e.g., `<h1>` followed by `<h3>` with no `<h2>`). This confuses screen reader navigation.

**How the AI fixes it**: Adjusts heading levels to maintain proper hierarchy.

---

## Workflow: Accessibility Remediation

### 1. Audit the Page

```
"Run a full accessibility audit."
```

```js
analyze({what: "accessibility"})
```

### 2. Prioritize by Impact

Focus on `critical` and `serious` violations first. These are the issues that actually prevent people from using your application.

```
"Show me only the critical and serious violations."
```

### 3. Fix One at a Time

The AI has the exact DOM selector for each failing element. Ask it to fix them:

```
"Fix the color contrast issue on the login form."
```

The AI reads your component source, adjusts the colors, and verifies the contrast ratio meets 4.5:1.

### 4. Re-audit

```
"Run the accessibility audit again — did we fix the contrast issues?"
```

```js
analyze({what: "accessibility", force_refresh: true})
```

### 5. Scope to New Components

As you build new features, audit just the new component:

```
"Audit the accessibility of the checkout form."
```

```js
analyze({what: "accessibility", scope: ".checkout-form"})
```

### 6. Export for CI

```
"Generate a SARIF report and save it."
```

```js
generate({what: "sarif", save_to: "a11y-report.sarif"})
```

---

## Combining with Other Tools

### DOM Queries for Context

If you need more detail about a failing element:

```js
analyze({what: "dom", selector: "#submit-btn"})
```

Returns the element's attributes, classes, computed state, and children — more context for understanding why the violation occurs.

### Highlight Failing Elements

Visually identify where violations are on the page:

```js
interact({what: "highlight", selector: "#failing-element", duration_ms: 5000})
```

### Screenshot for Documentation

Capture the current state before and after fixes:

```js
observe({what: "screenshot"})
```

---

## Tips

**Start with Level AA**. WCAG 2.1 Level AA is the standard compliance target for most organizations and the legal baseline in many jurisdictions.

**Audit early, not late**. Running accessibility checks during development is cheaper than retrofitting an entire application. Ask your AI to audit after every new component.

**Automated audits catch ~30-40% of issues**. axe-core is excellent but can't detect all accessibility problems (e.g., whether alt text is actually meaningful, whether focus order is logical). Use it as a baseline, not a complete audit.

**Use `include_passes` for compliance documentation**. When generating SARIF reports for compliance, including passing rules shows auditors what was tested, not just what failed.

**Scope aggressively**. Full-page audits on complex applications can return hundreds of violations. Scoping to specific components makes the results actionable.

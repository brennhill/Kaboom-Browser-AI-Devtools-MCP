---
status: proposed
scope: feature/sarif-export/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-sarif-export
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-sarif-export.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Sarif Export Review](sarif-export-review.md).

# Technical Spec: SARIF Export

## Purpose

Kaboom's accessibility audit (`run_accessibility_audit`) detects WCAG violations in the running page — missing alt text, low contrast ratios, missing form labels, etc. Today, these results exist only in the MCP tool response. The AI sees them and can fix them, but the human reviewer has no persistent record of what was found, what was fixed, and what remains.

SARIF (Static Analysis Results Interchange Format) is the industry-standard JSON format for communicating tool findings. GitHub Code Scanning, VS Code's Problems panel, Azure DevOps, SonarQube, and dozens of CI tools consume SARIF natively. By exporting accessibility audit results as SARIF, Kaboom's findings integrate directly into the developer's existing code review workflow — violations appear as annotations on the PR diff, just like ESLint errors or TypeScript warnings.

This is the key human verification mechanism: the AI finds and fixes a11y issues, and the human sees the evidence in their familiar PR review interface without needing to run a separate audit tool.

---

## Opportunity & Business Value

**GitHub Code Scanning integration**: Upload a SARIF file via `gh api` or GitHub Actions, and violations appear as inline annotations on the PR diff. Reviewers see "Missing alt attribute on img element" right next to the code that renders the image. No new tools to learn, no new dashboards to check.

**Compliance evidence**: Teams that must demonstrate WCAG 2.1 AA compliance (healthcare, government, finance) need audit records. SARIF provides timestamped, machine-readable evidence that audits were run and what was found. This is auditable documentation, not just console output.

**CI/CD gating**: A GitHub Action that uploads SARIF and sets `fail_on_level: error` blocks merges when critical a11y violations exist. The AI can fix issues during development, and the CI gate ensures nothing slips through.

**Tool-agnostic format**: SARIF is consumed by: GitHub, GitLab (via SAST integration), Azure DevOps, VS Code (SARIF Viewer extension), SonarQube, Snyk, Checkmarx, and any tool supporting OASIS SARIF 2.1.0. One export format reaches the entire ecosystem.

**Differential analysis**: SARIF supports "baseline" comparisons — tools can show only NEW violations introduced in a PR, ignoring pre-existing issues. This makes a11y audits practical on legacy codebases where fixing everything at once is infeasible.

**Persistent artifact**: Unlike MCP tool responses (ephemeral, lost when the session ends), a SARIF file on disk is a permanent record. Teams can track a11y improvement over time by comparing SARIF outputs across commits.

---

## How It Works

### SARIF Structure

A SARIF 2.1.0 log contains:

1. **Tool section**: Identifies Kaboom as the analysis tool, with version and documentation URL
2. **Rules section**: Defines each a11y rule that was checked (axe-core rule IDs map to SARIF rule descriptors)
3. **Results section**: Lists each violation found, with location (file path + source element), severity, and remediation guidance

### Mapping A11y Results to SARIF

Kaboom's `run_accessibility_audit` returns axe-core-compatible results with:
- Rule ID (e.g., `color-contrast`, `image-alt`, `label`)
- Impact level (`critical`, `serious`, `moderate`, `minor`)
- CSS selector of the failing element
- HTML snippet of the failing element
- Help text describing the violation
- Help URL linking to documentation

These map to SARIF fields:

| A11y Field | SARIF Field |
|------------|-------------|
| Rule ID | `result.ruleId` |
| Impact critical/serious | `result.level: "error"` |
| Impact moderate | `result.level: "warning"` |
| Impact minor | `result.level: "note"` |
| CSS selector | `result.locations[0].physicalLocation.region.snippet.text` |
| Help text | `result.message.text` |
| Help URL | `rule.helpUri` |
| HTML snippet | `result.locations[0].physicalLocation.region.snippet.text` |

### Source Location Mapping

The most valuable SARIF feature is source location — pointing to the exact file and line that renders the violating element. Kaboom doesn't have source maps or build-tool integration, so it uses a best-effort heuristic:

1. **Component attribution**: If the HTML snippet contains a `data-component` or `data-testid` attribute, the SARIF location references that component name as a logical location. This is enough for GitHub to create a code annotation.

2. **File path from source maps** (optional): If the page has source maps available and the element was rendered by a known framework (React, Vue, Svelte), the extension can extract the component file path from React DevTools fiber data or framework-specific debug info. This is best-effort — not all apps expose this.

3. **Fallback**: If no source location is determinable, the SARIF result uses a logical location with the CSS selector path: `html > body > main > div.card > img`. This still appears in the SARIF viewer but can't create an inline code annotation.

### MCP Tool: `export_sarif`

**Parameters**:
- `scope` (optional): CSS selector to scope the audit (same as `run_accessibility_audit`)
- `tags` (optional): WCAG tags to test (e.g., `["wcag2a", "wcag2aa"]`)
- `output_path` (optional): File path to write the SARIF file. If omitted, returns the SARIF JSON directly in the response.
- `include_passes` (optional, boolean): Include rules that passed (default false — only violations and incomplete results)

**Behavior**:
1. Runs an accessibility audit (or uses cached results if fresh — within 30s TTL from a11y cache)
2. Transforms results to SARIF 2.1.0 format
3. Writes to file or returns in response

**Response** (when output_path is specified):
```
{
  "file_path": "/path/to/project/.kaboom/reports/a11y-2026-01-24.sarif",
  "violations_count": 7,
  "rules_checked": 85,
  "summary": "7 violations (2 critical, 3 serious, 2 moderate). SARIF file ready for upload."
}
```

### GitHub Upload Workflow

After the agent generates the SARIF file, it can upload it to GitHub Code Scanning:

```bash
gh api /repos/{owner}/{repo}/code-scanning/sarifs \
  -f "commit_sha=$(git rev-parse HEAD)" \
  -f "ref=$(git symbolic-ref HEAD)" \
  -f "sarif=$(gzip -c .kaboom/reports/a11y.sarif | base64)"
```

Or via GitHub Actions:

```yaml
- uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: .kaboom/reports/a11y.sarif
```

The agent can do this as part of the PR creation workflow (see workflow integration spec).

---

## SARIF Output Format

```json
{
  "$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
  "version": "2.1.0",
  "runs": [{
    "tool": {
      "driver": {
        "name": "Kaboom",
        "version": "4.0.0",
        "informationUri": "https://github.com/brennhill/kaboom-agentic-browser-devtools-mcp",
        "rules": [
          {
            "id": "color-contrast",
            "shortDescription": { "text": "Elements must have sufficient color contrast" },
            "helpUri": "https://dequeuniversity.com/rules/axe/4.4/color-contrast",
            "defaultConfiguration": { "level": "error" }
          }
        ]
      }
    },
    "results": [
      {
        "ruleId": "color-contrast",
        "level": "error",
        "message": { "text": "Element has insufficient color contrast of 2.5:1 (foreground: #999, background: #fff). Expected ratio of 4.5:1 for normal text." },
        "locations": [{
          "physicalLocation": {
            "artifactLocation": { "uri": "src/components/Card.tsx" },
            "region": {
              "startLine": 42,
              "snippet": { "text": "<p class=\"text-gray-400\">Card description</p>" }
            }
          }
        }]
      }
    ]
  }]
}
```

---

## Edge Cases

- **No violations found**: SARIF is still generated with an empty `results` array. This is a valid "clean" report.
- **A11y audit not yet run**: Tool triggers an audit first (same as `run_accessibility_audit`). Uses cached results if available.
- **Very many violations** (>100): All are included in the SARIF file. GitHub Code Scanning handles large SARIF files gracefully (up to 5000 results per file).
- **No source location determinable**: Result uses logical location with CSS selector path. GitHub shows it in the "Other" section rather than inline.
- **Output path directory doesn't exist**: Created with MkdirAll.
- **Output path is not writable**: Error returned in MCP response.
- **Stale cached audit** (page has changed since last audit): The `export_sarif` tool always re-runs the audit unless explicitly told to use cache. This ensures the SARIF reflects the current page state.

---

## Performance Constraints

- SARIF generation: under 10ms for 100 violations (JSON marshaling)
- File write: under 50ms (typical SARIF file is 50-200KB)
- No impact on page performance (SARIF is generated server-side from cached results)
- Audit itself is the expensive operation (delegated to the browser via extension) — SARIF is just format conversion

---

## Test Scenarios

1. Single violation → valid SARIF with one result entry
2. Multiple violations → all included in results array
3. Critical/serious impact → SARIF level "error"
4. Moderate impact → SARIF level "warning"
5. Minor impact → SARIF level "note"
6. Rule IDs map correctly to SARIF rule descriptors
7. Help URLs preserved in rule.helpUri
8. CSS selector included in snippet.text
9. Output path creates directories if needed
10. No violations → valid SARIF with empty results
11. Schema validates against SARIF 2.1.0 spec
12. Tool version matches server version
13. `include_passes` adds passed rules to results (kind: "pass")
14. Source location from data-component attribute → correct artifactLocation
15. No source location → logical location with CSS path
16. File returned in response when output_path omitted
17. Cached audit used when fresh (within TTL)
18. Scope parameter limits audit to specific DOM subtree

---

## File Locations

Server implementation: `cmd/browser-agent/export_sarif.go` (SARIF generation, MCP tool handler).

Tests: `cmd/browser-agent/export_sarif_test.go`.

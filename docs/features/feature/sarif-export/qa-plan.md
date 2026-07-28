---
status: proposed
scope: feature/sarif-export/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-sarif-export
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: SARIF Export

> QA plan for the SARIF Export feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. SARIF files contain code locations, HTML snippets, CSS selectors, and file paths. When uploaded to GitHub Code Scanning, this data becomes visible to repository collaborators. Sensitive DOM content, internal file paths, or PII rendered in the page must not leak.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Source file paths from source maps | Verify that local filesystem paths (e.g., `/Users/dev/project/src/`) are either correct relative paths or excluded | high |
| DL-2 | HTML snippets containing PII | Verify HTML snippets in `snippet.text` do not contain user-entered PII (form values, email addresses rendered in DOM) | high |
| DL-3 | CSS selectors revealing internal structure | Verify CSS selectors do not expose sensitive class names or data attributes with PII | medium |
| DL-4 | Page URL in SARIF metadata | Verify localhost development URLs (e.g., `http://localhost:3000/admin/users`) do not expose sensitive routes | medium |
| DL-5 | Tool version leaking internal build info | Verify `tool.driver.version` contains only the public version string, not build hashes or paths | low |
| DL-6 | Data-component attributes with internal names | Verify `data-component` or `data-testid` values used for logical locations do not expose sensitive component naming | medium |
| DL-7 | SARIF output path traversal | Verify `output_path` cannot write outside the project directory | critical |
| DL-8 | Audit scope revealing hidden DOM | Verify scoped audit (`scope` CSS selector) does not inadvertently expose DOM content from outside the scope in violation snippets | medium |
| DL-9 | Help URLs pointing to internal docs | Verify `helpUri` fields point to public Deque University URLs, not internal documentation | low |
| DL-10 | Error messages containing stack traces | Verify error states (e.g., audit failure) do not expose server stack traces or internal state | high |

### Negative Tests (must NOT leak)
- [ ] No absolute filesystem paths (e.g., `/Users/`, `/home/`, `C:\`) appear in SARIF artifact locations
- [ ] No user-entered form data appears in HTML snippets (test with a form containing an email input)
- [ ] No server internal state or Go stack traces appear in SARIF output
- [ ] No authentication tokens or session data appear in any SARIF field
- [ ] Password field values must not appear in HTML snippets
- [ ] No internal IP addresses or port numbers beyond localhost appear in SARIF metadata

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | SARIF version identification | Response clearly states SARIF 2.1.0 format | [ ] |
| CL-2 | Violation count is prominent | `violations_count` is a top-level field in the response summary | [ ] |
| CL-3 | Severity levels are unambiguous | Mapping from a11y impact (critical/serious/moderate/minor) to SARIF levels (error/warning/note) is correct | [ ] |
| CL-4 | Rule IDs are meaningful | SARIF `ruleId` matches axe-core rule IDs (e.g., `color-contrast`, `image-alt`) | [ ] |
| CL-5 | Empty results distinguishable | Zero violations returns valid SARIF with empty `results` array and clear summary | [ ] |
| CL-6 | Summary is actionable | `summary` describes violation count by severity (e.g., "2 critical, 3 serious") | [ ] |
| CL-7 | Help URLs are accessible | `helpUri` links resolve to real documentation pages | [ ] |
| CL-8 | Location types are clear | Physical locations vs. logical locations are distinguishable in the output | [ ] |
| CL-9 | Pass results distinguishable | When `include_passes: true`, passed rules have a different kind/level from violations | [ ] |
| CL-10 | File path vs. URL distinction | SARIF `artifactLocation.uri` uses relative file paths, not page URLs | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM might confuse SARIF `level: "error"` with a system error rather than a critical a11y violation -- verify context makes this clear
- [ ] LLM might treat an empty `results` array as a failure rather than a clean audit -- verify summary message explains "no violations found"
- [ ] LLM might not understand logical locations (CSS selector paths) vs. physical locations (file:line) -- verify both are clearly labeled
- [ ] LLM might attempt to upload SARIF to GitHub without the file existing on disk -- verify response clarifies whether output is inline or file-based
- [ ] LLM might misinterpret `rules_checked: 85` as 85 violations rather than 85 rules evaluated -- verify field naming is unambiguous

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Export all violations as SARIF | 1 step: call `generate` with `format: "sarif"` | No -- already minimal |
| Export scoped audit | 1 step: add `scope` parameter | No |
| Export specific WCAG levels | 1 step: add `tags` parameter | No |
| Export to file for GitHub upload | 1 step: add `output_path` parameter | No |
| Include passing rules | 1 step: add `include_passes: true` | No |
| Full GitHub upload workflow | 2 steps: export SARIF + run `gh api` upload | Could auto-upload but separation is better for review |

### Default Behavior Verification
- [ ] Feature works with zero configuration (full audit, violations only, all WCAG rules)
- [ ] `include_passes` defaults to `false` (smaller output, focuses on issues)
- [ ] Default scope is the entire page (no scoping required)
- [ ] Default tags include all WCAG levels (wcag2a, wcag2aa)
- [ ] Audit is automatically triggered if not recently cached (within 30s TTL)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Single violation mapping | One `color-contrast` violation | Valid SARIF result with correct `ruleId`, `level: "error"`, message | must |
| UT-2 | Multiple violations | 5 different rule violations | All 5 in `results` array with correct rule references | must |
| UT-3 | Critical impact -> error level | `impact: "critical"` violation | `result.level: "error"` | must |
| UT-4 | Serious impact -> error level | `impact: "serious"` violation | `result.level: "error"` | must |
| UT-5 | Moderate impact -> warning level | `impact: "moderate"` violation | `result.level: "warning"` | must |
| UT-6 | Minor impact -> note level | `impact: "minor"` violation | `result.level: "note"` | must |
| UT-7 | Rule descriptor construction | axe-core rule with help URL | SARIF rule with `shortDescription`, `helpUri`, `defaultConfiguration` | must |
| UT-8 | CSS selector in snippet | Violation with CSS selector | `physicalLocation.region.snippet.text` contains selector | must |
| UT-9 | HTML snippet in location | Violation with HTML snippet | Snippet appears in correct SARIF location field | must |
| UT-10 | Source location from data-component | Element with `data-component="Card"` | `artifactLocation.uri` references component file | should |
| UT-11 | Logical location fallback | No source location determinable | Logical location with CSS selector path | must |
| UT-12 | Empty results | No violations found | Valid SARIF with empty `results`, summary says "no violations" | must |
| UT-13 | SARIF schema reference | Any export | `$schema` points to SARIF 2.1.0 schema URL | must |
| UT-14 | Tool version | Any export | `tool.driver.version` matches server version | must |
| UT-15 | Include passes | `include_passes: true` | Passed rules included with appropriate kind | should |
| UT-16 | WCAG tag filtering | `tags: ["wcag2aa"]` | Only wcag2aa rules checked | should |
| UT-17 | Scope filtering | `scope: ".main-content"` | Only violations within scoped element | should |
| UT-18 | Output path file write | `output_path` specified | File written, response has `file_path` and metadata | must |
| UT-19 | Output path directory creation | Non-existent parent directory | Directory created via MkdirAll | must |
| UT-20 | Inline response | `output_path` omitted | Full SARIF JSON in MCP response | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end SARIF via MCP | MCP client -> `generate(what: "sarif")` -> server -> extension audit -> SARIF | Valid SARIF file with real audit results | must |
| IT-2 | Cached audit reuse | Run audit, then export SARIF within 30s | SARIF uses cached results, no re-audit | should |
| IT-3 | Stale cache triggers re-audit | Run audit, wait >30s, export SARIF | Fresh audit triggered before SARIF generation | should |
| IT-4 | SARIF schema validation | Export SARIF -> validate against SARIF 2.1.0 JSON schema | No schema validation errors | must |
| IT-5 | GitHub Code Scanning upload | Export SARIF -> upload via `gh api` | SARIF accepted by GitHub, annotations appear | should |
| IT-6 | Concurrent audit and export | Export while audit is running | Export waits for audit completion or uses latest results | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | SARIF generation for 100 violations | Wall clock time | Under 10ms | must |
| PT-2 | File write for typical SARIF | Wall clock time | Under 50ms | must |
| PT-3 | Audit + SARIF generation combined | Wall clock time | Audit time + under 10ms for SARIF | should |
| PT-4 | Memory during generation | Peak memory allocation | Proportional to violation count | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Very many violations (>100) | Page with 150+ a11y violations | All included in SARIF, no truncation | must |
| EC-2 | Non-writable output path | Read-only directory | Error returned in MCP response with clear message | must |
| EC-3 | Audit not yet run, no cache | First call to export_sarif | Triggers audit automatically, then generates SARIF | must |
| EC-4 | Page with iframes | Cross-origin iframe content | Audit covers main frame only, noted in output | should |
| EC-5 | Unicode in violation messages | CJK characters in element text | Correctly encoded in SARIF JSON | must |
| EC-6 | Very long CSS selector | Deeply nested element (20+ levels) | Selector included without truncation or breakage | should |
| EC-7 | Duplicate rule violations | Same rule violated by 50 elements | All 50 results included, each with unique location | must |
| EC-8 | Mixed physical and logical locations | Some violations with source maps, some without | Both location types correctly represented | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page loaded with known a11y violations (e.g., missing alt text on images, low contrast text)
- [ ] Page has at least one `data-component` or `data-testid` attribute on a violating element

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "generate", "arguments": {"type": "sarif"}}` | Review inline SARIF JSON in MCP response | Valid SARIF 2.1.0 with violations, rules, and tool sections | [ ] |
| UAT-2 | `{"tool": "generate", "arguments": {"type": "sarif", "output_path": ".kaboom/reports/a11y.sarif"}}` | Check file exists at specified path | File created, response shows `file_path`, `violations_count`, `rules_checked` | [ ] |
| UAT-3 | `{"tool": "generate", "arguments": {"type": "sarif", "scope": "main"}}` | Compare violation count to full-page audit | Scoped audit has fewer or equal violations | [ ] |
| UAT-4 | `{"tool": "generate", "arguments": {"type": "sarif", "tags": ["wcag2a"]}}` | Check that only WCAG 2.0 Level A rules are checked | No Level AA-only rules in the rules section | [ ] |
| UAT-5 | `{"tool": "generate", "arguments": {"type": "sarif", "include_passes": true}}` | Look for passed rules in results | Results include both violations and passes, distinguishable by level/kind | [ ] |
| UAT-6 | Open generated SARIF in VS Code with SARIF Viewer extension | VS Code shows violations | Violations appear with correct locations and messages | [ ] |
| UAT-7 | Fix one a11y violation (e.g., add alt text), re-export SARIF | Compare violation counts | New SARIF has one fewer violation | [ ] |
| UAT-8 | Upload SARIF to GitHub: `gh api /repos/{owner}/{repo}/code-scanning/sarifs -f "sarif=$(gzip -c .kaboom/reports/a11y.sarif \| base64)"` | Check GitHub Code Scanning tab | Violations appear as annotations | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No absolute file paths | Search SARIF for `/Users/`, `/home/`, `C:\` | Not found -- only relative paths or logical locations | [ ] |
| DL-UAT-2 | No PII in HTML snippets | Add a form with email input, fill it in, export SARIF | Email address entered by user does not appear in snippets | [ ] |
| DL-UAT-3 | No password values | Add a password field, fill it in, export SARIF | Password value does not appear in any SARIF field | [ ] |
| DL-UAT-4 | No server internals | Search SARIF for Go file paths, goroutine references | Not found | [ ] |
| DL-UAT-5 | Help URLs are public | Click each `helpUri` in the SARIF | All resolve to public Deque University pages | [ ] |

### Regression Checks
- [ ] Existing `observe` accessibility tool still works after SARIF export is enabled
- [ ] Existing `generate` tool with other types (reproduction, test, har) still works
- [ ] A11y cache is not corrupted by SARIF export (subsequent audits produce fresh results)
- [ ] SARIF export does not affect page performance (server-side operation only)

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

---
status: shipped
scope: feature/enhanced-wcag-audit/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-enhanced-wcag-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Enhanced WCAG Accessibility Audit

> QA plan for the Enhanced WCAG Accessibility Audit feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

**Note:** No tech-spec.md is available for this feature. This QA plan is based solely on the product-spec.md.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | HTML snippets in violation nodes may contain user-entered form data | Verify HTML snippets are truncated to `DOM_QUERY_MAX_HTML` (200 chars). Verify form field `value` attributes are stripped or redacted from HTML snippets. | high |
| DL-2 | CSS selectors may reveal internal application structure | Verify selectors only contain structural information (tag names, classes, IDs). This is acceptable for localhost dev tooling. Document the exposure. | low |
| DL-3 | Heading text content may include sensitive information | Verify heading text is extracted as-is from the DOM. This is page-visible content and acceptable for localhost delivery. Confirm no hidden/aria-hidden headings are included. | medium |
| DL-4 | Form label text may reference sensitive fields | Verify `form_labels` check reports label elements and input selectors but not input values. Only `<label>` text and input `type`/`id` are reported. | medium |
| DL-5 | Computed color values could theoretically fingerprint users | Verify color values are standard CSS values (hex/rgb). No additional browser fingerprint data is collected. | low |
| DL-6 | axe-core injection path does not introduce vulnerabilities | Verify axe-core is loaded from the bundled version (4.8.4) and not from an external CDN or URL. No external requests during audit. | critical |
| DL-7 | Enhanced checks access DOM beyond what axe-core reads | Verify enhanced checks (heading hierarchy, form labels, ARIA validation, skip links, focus indicators, keyboard traps, screen reader text) only perform read-only DOM queries. No cookies, localStorage, or sessionStorage access. | high |
| DL-8 | `scope` parameter could target sensitive page sections | Verify `scope` CSS selector limits which DOM subtree is audited but does not enable access to hidden elements or cross-origin iframes. | medium |

### Negative Tests (must NOT leak)
- [ ] HTML snippets must NOT contain `input[type="password"]` value attributes
- [ ] Form label checks must NOT report input field values
- [ ] No external HTTP requests during audit (axe-core loaded locally)
- [ ] Enhanced checks must NOT access `document.cookie`, `localStorage`, or `sessionStorage`
- [ ] Keyboard trap simulation must NOT trigger actual form submissions or navigation
- [ ] Audit results must NOT be sent to any external endpoint
- [ ] `force_refresh` must NOT expose stale cached data from different users/sessions

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Severity values are consistent | All violations use exactly `"critical"`, `"high"`, `"medium"`, or `"low"`. Verify consistency across axe-core violations and enhanced checks. | [ ] |
| CL-2 | WCAG criterion numbers are correct | Each violation annotated with `wcag_criterion` (e.g., "1.4.3") matches the actual WCAG 2.1 success criterion. | [ ] |
| CL-3 | WCAG level filtering works correctly | With `wcag_level: "AA"`, only A and AA violations appear. No AAA-only violations. | [ ] |
| CL-4 | Enhanced check status values are consistent | Each check uses `"pass"`, `"fail"`, or `"warn"`. No other status values. | [ ] |
| CL-5 | Remediation effort estimates are meaningful | `effort` is one of `"low"`, `"medium"`, `"high"`. The LLM can use these to prioritize. | [ ] |
| CL-6 | Priority order array is deterministic | Same page produces same priority ordering across runs. Sorting: severity desc, then effort asc. | [ ] |
| CL-7 | Basic mode response is identical to current behavior | With no new parameters, the response schema matches the existing accessibility audit exactly. | [ ] |
| CL-8 | Enhanced checks are clearly separate from axe-core violations | `violations` array is axe-core output; `checks` object is enhanced checks. The LLM can distinguish them. | [ ] |
| CL-9 | Computed color contrast data is actionable | `computed.foreground`, `computed.background`, `computed.contrast_ratio`, and `computed.required_ratio` give the LLM enough info to suggest a fix. | [ ] |
| CL-10 | Timeout partial results are clearly marked | When audit times out, `timeout: true` flag is present and incomplete checks are listed. | [ ] |
| CL-11 | fix_suggestion is specific enough to act on | Fix suggestions reference concrete CSS properties, HTML attributes, or ARIA attributes the LLM can modify. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM may confuse `impact` (axe-core: minor/moderate/serious/critical) with `severity` (enhanced: low/medium/high/critical). Verify both are present and clearly labeled.
- [ ] LLM may not understand that `wcag_level: "AA"` includes Level A checks too (cumulative). Verify response clearly states the level tested.
- [ ] LLM may assume all enhanced checks are standard axe-core rules. Verify the separation between `violations` (axe-core) and `checks` (enhanced) is clear.
- [ ] LLM may treat `warn` status as `fail` for compliance reporting. Verify `warn` indicates an advisory finding, not a conformance failure.
- [ ] LLM may not realize keyboard trap detection is a simulation and could miss some traps. Verify the check status description notes the detection method.
- [ ] LLM may assume `effort: "low"` means trivial. Verify effort estimates account for the number of affected elements.

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Run basic accessibility audit (unchanged) | 1 step: `observe({what: "accessibility"})` | No -- identical to current behavior |
| Run WCAG AA audit with remediation | 1 step: `observe({what: "accessibility", wcag_level: "AA", detailed: true})` | No -- single call |
| Run scoped audit on specific section | 1 step with `scope` parameter | No -- single call |
| Fix issues and re-audit | 2 steps: fix code + re-audit | Natural workflow |
| Force fresh audit (bypass cache) | 1 step with `force_refresh: true` | No -- single call |

### Default Behavior Verification
- [ ] `observe({what: "accessibility"})` with no new parameters produces identical output to current implementation
- [ ] Adding `detailed: true` enhances the response without breaking existing fields
- [ ] Adding `wcag_level` filters results without removing the standard summary
- [ ] Omitting `scope` audits the entire page (default behavior)
- [ ] `force_refresh: false` uses cached results if available (existing behavior)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Parse wcag_level parameter | `"A"`, `"AA"`, `"AAA"` | Valid values accepted; axe-core tag filter configured | must |
| UT-2 | Reject invalid wcag_level | `"B"`, `"AAAA"`, `""` | Error with valid enum values (A, AA, AAA) | must |
| UT-3 | Parse detailed parameter | `true`, `false` | Boolean parsed correctly; enhanced checks enabled/disabled | must |
| UT-4 | Backward compatibility: no new params | `{what: "accessibility"}` | Identical response to current implementation | must |
| UT-5 | WCAG AA tag filter mapping | `wcag_level: "AA"` | axe-core tags include `wcag2a`, `wcag2aa`, `wcag21aa` | must |
| UT-6 | WCAG A tag filter mapping | `wcag_level: "A"` | axe-core tags include only `wcag2a` | must |
| UT-7 | WCAG AAA tag filter mapping | `wcag_level: "AAA"` | axe-core tags include all levels through AAA | must |
| UT-8 | Heading hierarchy check: valid sequence | h1, h2, h3 | Status: pass | must |
| UT-9 | Heading hierarchy check: skip level | h1, h3 (missing h2) | Status: fail; details mention skipped level | must |
| UT-10 | Heading hierarchy check: no headings | Empty heading list | Status: warn; "No headings found" | must |
| UT-11 | Form label check: all labeled | All inputs have associated labels | Status: pass | must |
| UT-12 | Form label check: missing labels | 2 inputs without labels or aria-label | Status: fail; elements listed with selectors | must |
| UT-13 | ARIA validation: valid hierarchy | role="option" inside role="listbox" | Status: pass | must |
| UT-14 | ARIA validation: invalid hierarchy | role="option" without listbox parent | Status: fail; issue describes required parent | must |
| UT-15 | Skip link detection: present | `<a href="#main">Skip to content</a>` | Status: pass | should |
| UT-16 | Skip link detection: missing | No skip navigation link | Status: fail or warn | should |
| UT-17 | Focus indicator: outline:none with alternative | Element with outline:none but box-shadow on focus | Status: pass (alternative detected) | should |
| UT-18 | Focus indicator: outline:none without alternative | Element with outline:none and no focus style | Status: warn; element listed | should |
| UT-19 | Screen reader text: images with alt | All images have alt attributes | Status: pass | should |
| UT-20 | Screen reader text: icon button without name | `<button><svg>...</svg></button>` with no aria-label | Status: warn; element listed | should |
| UT-21 | Severity derivation from impact + level | axe-core impact "serious" + WCAG level AA | Severity: "high" | must |
| UT-22 | Remediation object structure | Failing violation | remediation has `summary`, `effort`, `fix_suggestion` fields | must |
| UT-23 | Priority order sorting | Multiple violations with mixed severity/effort | Sorted by severity desc, then effort asc | must |
| UT-24 | Cache key includes new parameters | Same URL, different wcag_level | Different cache entries (cache miss) | must |
| UT-25 | HTML snippet truncation | Violation node with 500-char HTML | Truncated to 200 chars (DOM_QUERY_MAX_HTML) | must |
| UT-26 | Color contrast computed data | Element with foreground #767676, background #ffffff | contrast_ratio: 4.48; required_ratio: 4.5 (AA) | should |
| UT-27 | Timeout handling with partial results | Audit exceeds A11Y_AUDIT_TIMEOUT_MS | Partial results with `timeout: true` and list of incomplete checks | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full enhanced audit end-to-end | Go server + Extension + axe-core + enhanced checks | Complete response with violations, checks, summary, priority_order | must |
| IT-2 | Basic audit backward compatibility | Go server + Extension + axe-core | Response identical to pre-feature output | must |
| IT-3 | WCAG level filtering in axe-core | Extension + axe-core tag configuration | Only violations matching specified level returned | must |
| IT-4 | Scoped audit via CSS selector | Extension + axe-core include option | Violations and checks limited to selected DOM subtree | must |
| IT-5 | Cache behavior with new parameters | Go server cache + different wcag_level values | Different params produce different cache keys; fresh audit on cache miss | must |
| IT-6 | axe-core failure with enhanced checks fallback | Extension with axe-core load failure | Enhanced checks run independently; warning about axe-core skip | should |
| IT-7 | Timeout during enhanced audit | Extension with slow enhanced checks | Partial results returned; incomplete checks noted | must |
| IT-8 | Extension disconnected during audit | Go server timeout handling | Standard timeout error | must |
| IT-9 | force_refresh bypasses cache | Cached result exists + force_refresh=true | Fresh audit runs | should |
| IT-10 | Combined tags and detailed parameters | `tags: ["wcag2aa"]` + `detailed: true` | Both parameters compose correctly | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Basic audit (no new params) | Response time | < 5s | must |
| PT-2 | Enhanced audit (detailed: true) | Response time | < 10s | must |
| PT-3 | Total audit within timeout budget | Wall clock time | < 30s (A11Y_AUDIT_TIMEOUT_MS) | must |
| PT-4 | Extension memory during enhanced checks | Heap usage | < 2MB | must |
| PT-5 | Per-check main thread blocking | Per-check execution time | < 50ms | must |
| PT-6 | Response payload size | JSON size | < 100KB | must |
| PT-7 | Heading hierarchy check on page with 200 headings | Check execution time | < 10ms | should |
| PT-8 | Form label check on page with 50 form inputs | Check execution time | < 20ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Invalid wcag_level value | `wcag_level: "B"` | Structured error with valid values (A, AA, AAA) | must |
| EC-2 | detailed=true with no violations | Clean page, all accessible | Enhanced structure with empty violations, all checks pass, empty priority_order | must |
| EC-3 | axe-core fails to load | Library load error | Enhanced checks run without axe-core; warning about skip | should |
| EC-4 | No headings on page | Page without any h1-h6 | heading_hierarchy status: warn, "No headings found" | must |
| EC-5 | Scoped audit with detailed=true | scope="#main" + detailed=true | Both axe-core and enhanced checks scoped to #main subtree | should |
| EC-6 | Audit exceeds timeout | Complex page with many violations | Partial results with timeout: true; list of incomplete checks | must |
| EC-7 | Extension disconnected mid-audit | Network interruption | Standard timeout error from server | must |
| EC-8 | Cached basic result, request detailed | Cache has basic mode result | Cache miss; fresh enhanced audit runs | must |
| EC-9 | Tags and detailed both specified | `tags: ["wcag2a"]` + `detailed: true` | Tags filter axe-core rules; detailed adds post-processing. Both compose. | should |
| EC-10 | Page with deeply nested ARIA roles | 5+ levels of ARIA role nesting | ARIA validation handles arbitrarily deep hierarchy | should |
| EC-11 | Focus indicator check with CSS custom properties | Element uses `var(--focus-color)` for focus styling | Check handles CSS variables correctly (may report as warn if unable to resolve) | could |
| EC-12 | Form inputs with implicit labels | `<label>Email <input type="email"></label>` (wrapping label) | Form label check detects implicit association (no `for` attribute needed) | should |
| EC-13 | Very large number of violations (100+) | Highly inaccessible page | Violations capped per check (A11Y_MAX_NODES_PER_VIOLATION = 10); truncation noted | must |
| EC-14 | Page in dark mode | Dark theme active | Color contrast check uses computed (rendered) colors, not source CSS | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web page with known accessibility issues loaded (e.g., missing alt text, low contrast, skipped heading levels)
- [ ] axe-core 4.8.4 bundled with extension

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "observe", "arguments": {"what": "accessibility"}}` | Extension runs audit | Response matches current (basic) accessibility audit format exactly. No new fields. | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"what": "accessibility", "wcag_level": "AA", "detailed": true}}` | Extension runs enhanced audit | Response includes `wcag_level`, `violations` with WCAG criterion annotations, `checks` object, `summary` with enhanced fields, and `priority_order` array | [ ] |
| UAT-3 | Inspect a violation in enhanced response | Check a color-contrast violation | Violation has `severity`, `wcag_criterion`, `wcag_criterion_name`, `wcag_level`, `remediation` with `summary`/`effort`/`fix_suggestion`, and nodes with `computed` contrast data | [ ] |
| UAT-4 | Inspect heading_hierarchy check | Compare to actual page heading structure | Check correctly identifies skipped levels; headings_found matches page DOM | [ ] |
| UAT-5 | Inspect form_labels check | Look at page forms | Check correctly identifies inputs without labels; selectors match actual elements | [ ] |
| UAT-6 | Inspect aria_validation check | Look at ARIA roles on page | Check identifies invalid ARIA role hierarchies (if any) | [ ] |
| UAT-7 | Inspect priority_order array | Review ordering | Violations sorted by severity desc, then effort asc | [ ] |
| UAT-8 | `{"tool": "observe", "arguments": {"what": "accessibility", "wcag_level": "A", "detailed": true}}` | Only Level A issues | Response does not include AA-only or AAA-only violations | [ ] |
| UAT-9 | `{"tool": "observe", "arguments": {"what": "accessibility", "wcag_level": "AAA", "detailed": true}}` | All levels included | Response includes A, AA, and AAA violations | [ ] |
| UAT-10 | `{"tool": "observe", "arguments": {"what": "accessibility", "detailed": true, "scope": "#main-content"}}` | Scoped audit | Only violations within #main-content subtree reported | [ ] |
| UAT-11 | Fix a violation (e.g., add alt text), then: `{"tool": "observe", "arguments": {"what": "accessibility", "wcag_level": "AA", "detailed": true, "force_refresh": true}}` | Fresh audit after fix | Previously reported violation is gone; summary counts decreased | [ ] |
| UAT-12 | `{"tool": "observe", "arguments": {"what": "accessibility", "wcag_level": "B"}}` | Invalid level | Error response with valid values: A, AA, AAA | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | HTML snippets do not contain password values | Load a page with password fields, run enhanced audit | HTML snippets show `<input type="password">` without value attribute content | [ ] |
| DL-UAT-2 | No external requests during audit | Monitor network during enhanced audit | No outbound requests (axe-core loaded locally) | [ ] |
| DL-UAT-3 | Form label check does not include input values | Run audit on page with filled form fields | Form label check shows selectors and label text, not field values | [ ] |
| DL-UAT-4 | Enhanced checks are read-only | Verify page state before and after audit | No DOM modifications, no form submissions, no navigation triggered by audit | [ ] |
| DL-UAT-5 | Keyboard trap simulation does not trigger actions | Run audit on page with buttons and form submissions | Keyboard trap check does not submit forms or click buttons | [ ] |

### Regression Checks
- [ ] Basic `observe({what: "accessibility"})` produces identical output to pre-feature behavior
- [ ] Existing `tags` parameter still works when `detailed` is not specified
- [ ] a11y cache still works correctly for basic audits
- [ ] Other observe modes (errors, network, page) are unaffected
- [ ] Extension performance not degraded for non-a11y observe calls
- [ ] SARIF export (`generate({what: "sarif"})`) still works with cached basic a11y results

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

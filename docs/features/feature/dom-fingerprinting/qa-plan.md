---
status: proposed
scope: feature/dom-fingerprinting/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-dom-fingerprinting
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: DOM Fingerprinting

> QA plan for the DOM Fingerprinting feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Input field values exposed in interactive elements | Verify that interactive element serialization uses `"[has value]"` or `""` placeholders, NEVER the actual input value. Check `extractInteractive()` output. | critical |
| DL-2 | Password field values in accessible names | Verify password fields do not leak their value through `textContent`, `aria-label`, or any other accessible name resolution path. | critical |
| DL-3 | Visible PII in accessible names | Headings, links, buttons may contain user-visible PII (emails, usernames). The fingerprint captures `text` from accessible name resolution. Since data stays on localhost, document this as expected behavior. | medium |
| DL-4 | CSRF tokens in hidden inputs | `extractInteractive()` includes hidden inputs. Verify only `"[has value]"` is reported, not the actual token value. | high |
| DL-5 | Page state text containing sensitive messages | Error elements and notifications may contain user-specific messages (e.g., "Invalid password for <john@example.com>"). These are captured in `error_elements` and `notifications` text content (truncated to 200 chars). | medium |
| DL-6 | Link hrefs exposing session tokens | Some apps embed session tokens in URLs. The `href` field for links captures the path. Verify only the path portion is captured, not query params containing tokens. | high |
| DL-7 | Form field labels containing PII | `extractContent()` captures form field labels. Labels like "Email: <john@example.com>" could leak PII. This is structural data on localhost. | medium |
| DL-8 | Fingerprint hash reversibility | Verify the 8-character hash cannot be reversed to reconstruct page content. Hash is derived from structure, not content. | low |
| DL-9 | Comparison result leaking baseline data | `compare_dom_fingerprint` returns change descriptions. Verify descriptions do not echo sensitive content from either the current or baseline fingerprint. | medium |
| DL-10 | Data transmission path | Verify all fingerprint data flows only over localhost (127.0.0.1:7890). No external endpoints contacted. | critical |

### Negative Tests (must NOT leak)
- [ ] Actual input values (text typed by user) do NOT appear in the fingerprint -- only `"[has value]"` or `""`
- [ ] Password field values are never included in any part of the fingerprint
- [ ] `document.cookie` and `localStorage` are not accessed or serialized
- [ ] Full URL query parameters (which may contain tokens) are not included in link hrefs
- [ ] The hash cannot be reversed to extract page content
- [ ] No fingerprint data is transmitted to external servers

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Fingerprint vs audit distinction | LLM should understand `get_dom_fingerprint` is structural verification, NOT WCAG compliance. Verify response does not use audit terminology. | [ ] |
| CL-2 | Hash unchanged semantics | `status: "unchanged"` with matching hashes -- LLM should understand this means "page structure is identical", not "page is working correctly". | [ ] |
| CL-3 | Severity levels | `error`, `warning`, `info` -- verify LLM can map these to actionable priorities (error = investigate immediately, warning = review, info = awareness). | [ ] |
| CL-4 | Landmark presence vs content | `present: true` means the landmark element exists. It does not mean the landmark has meaningful content. Verify `contains` list disambiguates. | [ ] |
| CL-5 | Interactive element `enabled` vs `visible` | Both are booleans. LLM must distinguish "visible but disabled" from "enabled but hidden" (note: hidden elements are skipped, so `visible` is always true). | [ ] |
| CL-6 | Depth mode differences | `minimal`, `standard`, `detailed` -- verify the response clearly indicates which depth was used and what was omitted. | [ ] |
| CL-7 | Comparison change types | `error_appeared`, `landmark_missing`, `list_empty`, `element_missing` -- verify each type name is self-descriptive. | [ ] |
| CL-8 | Scope parameter semantics | `"full"`, `"above_fold"`, CSS selector -- verify the response indicates which scope was used so LLM knows the coverage. | [ ] |
| CL-9 | Token count estimate | `tokenEstimate` field -- verify LLM can use this to decide whether to request a more/less detailed fingerprint. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM confuses `get_dom_fingerprint` with `analyze({what: "dom"})` -- test by verifying response formats are clearly different (fingerprint returns structured landmarks/content/state, not raw element arrays)
- [ ] LLM misreads `severity: "clean"` as "all tests passed" when it means "no structural changes" -- test by creating a page with known bugs that do not change structure
- [ ] LLM assumes `hash` match means page is correct, when it only means structure is unchanged -- verify with scenario where content changes but structure does not
- [ ] LLM does not understand `"[has value]"` placeholder means an input has content without revealing what the content is

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (single-shot fingerprint), Medium (comparison workflow)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Get page fingerprint | 1 step: `get_dom_fingerprint()` | No -- already minimal |
| Get fingerprint with scope | 1 step: add `scope` parameter | No -- single call |
| Compare against baseline | 2 steps: (1) get fingerprint with `baseline_name`, or (2) call `compare_dom_fingerprint` separately | The `baseline_name` shortcut reduces to 1 step |
| Check if page structure changed | 1 step: compare hashes | No -- hash comparison is the simplest possible check |
| Filter comparison by severity | 1 step: add `severity_threshold` parameter | No -- single parameter |

### Default Behavior Verification
- [ ] Feature works with zero parameters (defaults to `scope: "full"`, `depth: "standard"`)
- [ ] Default depth (`standard`) provides a useful balance of detail and brevity
- [ ] Hash is always included in the response for quick comparison
- [ ] Token estimate is always included to help agents manage context budgets

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | `extractDOMFingerprint` returns correct structure shape | Standard HTML page | Object with `landmarks`, `content`, `interactive`, `state` sections | must |
| UT-2 | `extractLandmarks` finds semantic HTML landmarks | Page with `<header>`, `<main>`, `<nav>`, `<footer>` | All four landmarks reported with `present: true` | must |
| UT-3 | `extractLandmarks` finds ARIA role landmarks | Page with `role="banner"`, `role="main"` | Landmarks found by role | must |
| UT-4 | `extractInteractive` finds buttons and links | Page with buttons and anchor tags | All interactive elements listed with correct `type` and `text` | must |
| UT-5 | `extractInteractive` caps at max element count | Page with 200 interactive elements | Returns 50 (standard) or 100 (detailed) elements | must |
| UT-6 | `extractInteractive` skips hidden elements | Button with `display: none` | Not included in result | must |
| UT-7 | `extractInteractive` reports `"[has value]"` for inputs | Text input with user-entered value | `value: "[has value]"`, NOT the actual value | must |
| UT-8 | `extractContent` extracts headings | Page with h1-h6 tags | Headings listed with correct `level` and `text` | must |
| UT-9 | `extractContent` counts list items | Page with `<ul>` containing 10 `<li>` | List reported with `items: 10` | should |
| UT-10 | `extractContent` extracts form info | Page with form containing labeled inputs | Form reported with field labels and button labels | should |
| UT-11 | `extractPageState` detects error elements | Page with `role="alert"` element | `error_elements` list is non-empty | must |
| UT-12 | `extractPageState` detects loading indicators | Page with `.spinner` element | `loading_indicators` list is non-empty | must |
| UT-13 | `extractPageState` detects empty states | Page with `.empty-state` element | `empty_states` list is non-empty | should |
| UT-14 | `extractPageState` detects open modals | Page with visible `role="dialog"` | `modals_open` list is non-empty | should |
| UT-15 | Hash is deterministic | Same page fingerprinted twice | Identical 8-char hash | must |
| UT-16 | Hash changes on structural change | Add a button to the page, re-fingerprint | Different hash | must |
| UT-17 | `isVisible` correctly identifies hidden elements | Elements with `display:none`, `visibility:hidden`, `opacity:0` | All identified as not visible | must |
| UT-18 | Accessible name resolution priority | Element with `aria-label`, `textContent`, `title` | `aria-label` takes priority | must |
| UT-19 | Selector generation prefers ID | Element with `id="foo"` | Selector is `#foo` | should |
| UT-20 | Selector generation fallback to data-testid | Element with `data-testid="bar"` and no ID | Selector is `[data-testid="bar"]` | should |
| UT-21 | Comparison: no changes | Two identical fingerprints | `status: "unchanged"`, `severity: "clean"` | must |
| UT-22 | Comparison: error appeared | Baseline has no errors, current has `role="alert"` | Change type `error_appeared`, severity `error` | must |
| UT-23 | Comparison: landmark missing | Baseline has `<nav>`, current does not | Change type `landmark_missing`, severity `error` | must |
| UT-24 | Comparison: list empty | Baseline list has 10 items, current has 0 | Change type `list_empty`, severity `warning` | must |
| UT-25 | `filterBySeverity` filters correctly | Changes with mixed severities, threshold `warning` | Only `warning` and `error` changes returned | should |
| UT-26 | Minimal depth skips content extraction | `depth: "minimal"` | Only landmarks returned, no interactive/content/state | should |
| UT-27 | Detailed depth includes tables | `depth: "detailed"` | Tables with row/column counts included | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full extraction round trip | Go server -> background.js -> content.js -> inject.js -> `extractDOMFingerprint` | Complete fingerprint returned via MCP | must |
| IT-2 | Scope parameter forwarded | Server passes `scope: "#main"` through full chain | Extraction limited to `#main` subtree | must |
| IT-3 | Depth parameter forwarded | Server passes `depth: "minimal"` through full chain | Only landmarks extracted | must |
| IT-4 | Comparison with stored baseline | Server stores baseline, then compares new fingerprint | Comparison result included in response | should |
| IT-5 | Extension timeout handling | Extension disconnected, query sent | 5-second timeout, structured error returned | must |
| IT-6 | Message flow matches pending query pattern | Query created, extension polls, result posted back | Same pattern as `analyze({what: "dom"})` -- validates shared infrastructure | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | `extractDOMFingerprint` total time | Execution time on typical page (<500 nodes) | < 30ms | must |
| PT-2 | `extractLandmarks` execution time | Time to find all landmarks | < 2ms | must |
| PT-3 | `extractInteractive` execution time | Time to find and serialize interactive elements | < 15ms | must |
| PT-4 | `extractContent` execution time | Time to extract headings, lists, forms | < 10ms | should |
| PT-5 | `extractPageState` execution time | Time to check state indicators | < 3ms | should |
| PT-6 | `isVisible` per element | Single visibility check | < 0.1ms | must |
| PT-7 | Response payload size | Typical page fingerprint | < 5KB | should |
| PT-8 | Complex page extraction | Page with 2000+ nodes | < 500ms total | should |
| PT-9 | Performance guard warning | Extraction exceeding 30ms | Console.warn emitted | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Scope element not found | `scope: "#nonexistent"` | Error: "Scope element not found." | must |
| EC-2 | No extension connected | Query sent with no extension | 5-second timeout error | must |
| EC-3 | Massive DOM (10,000+ nodes) | Complex web application | Element lists capped at configured maximums | must |
| EC-4 | Dynamic content still loading | Page with visible spinner | Loading state detected in `loading_indicators` | should |
| EC-5 | Shadow DOM elements | Web Components with shadow roots | Not traversed (light DOM only) | should |
| EC-6 | iframe content | Page with iframes | Only main document fingerprinted | should |
| EC-7 | `above_fold` scope | Viewport-limited extraction | Only visible viewport elements included | should |
| EC-8 | Page with no interactive elements | Static content page | `interactive` list empty, no error | should |
| EC-9 | Page with no landmarks | Div-only page structure | All landmarks `present: false` | should |
| EC-10 | Concurrent fingerprint requests | Two fingerprints requested simultaneously | Both return independently, no cross-contamination | could |
| EC-11 | Elements with generated class names | React/CSS-modules style classes (`_a1b2c3`) | Selector falls back to tag name, skipping `_`-prefixed classes | could |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page loaded with: header, nav, main, footer, a form, a list with items, buttons, links, an error alert, and a hidden element
- [ ] Tab is being tracked by the extension

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"action": "get_dom_fingerprint"}}` | Page has standard structure | Response contains `landmarks`, `content`, `interactive`, `state` sections with correct data | [ ] |
| UAT-2 | `{"tool": "configure", "arguments": {"action": "get_dom_fingerprint", "depth": "minimal"}}` | N/A | Response contains ONLY `landmarks` section. No interactive elements, no content, no page state. | [ ] |
| UAT-3 | `{"tool": "configure", "arguments": {"action": "get_dom_fingerprint", "depth": "detailed"}}` | Page has a table | Response includes table with row/column counts | [ ] |
| UAT-4 | `{"tool": "configure", "arguments": {"action": "get_dom_fingerprint", "scope": "#main"}}` | Main section has specific content | Only elements within `#main` are included | [ ] |
| UAT-5 | `{"tool": "configure", "arguments": {"action": "get_dom_fingerprint", "scope": "#nonexistent"}}` | No such element | Error: "Scope element not found." | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"action": "get_dom_fingerprint", "baseline_name": "test-baseline"}}` | First capture | Fingerprint stored as baseline. Comparison section shows "no baseline exists" or baseline stored. | [ ] |
| UAT-7 | Make a visible change to the page (add an error alert), then: `{"tool": "configure", "arguments": {"action": "compare_dom_fingerprint", "against": "test-baseline"}}` | Error element added to page | Comparison shows `error_appeared` change with severity `error` | [ ] |
| UAT-8 | `{"tool": "configure", "arguments": {"action": "get_dom_fingerprint"}}` -- check hash | Page unchanged from previous step | Hash is different from the baseline hash (structure changed) | [ ] |
| UAT-9 | Verify interactive elements listing | Count buttons and links on page | `interactive` list matches the visible interactive elements on the page | [ ] |
| UAT-10 | Verify input values are redacted | Enter text in a form field, run fingerprint | Input shows `value: "[has value]"`, NOT the actual entered text | [ ] |
| UAT-11 | Verify page state detection | Page has a visible `role="alert"` element | `state.error_elements` is non-empty with the alert's text | [ ] |
| UAT-12 | Verify hash determinism | Run fingerprint twice without page changes | Both responses have identical hash values | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Input values not in fingerprint | Type "secret" in a text field, run fingerprint | `"[has value]"` appears, "secret" does NOT | [ ] |
| DL-UAT-2 | Password values not in fingerprint | Enter password in password field, run fingerprint | Password field appears as interactive element but no value content | [ ] |
| DL-UAT-3 | Link hrefs are path-only | Page has links with query params containing tokens | Only path portion captured, no query string tokens | [ ] |
| DL-UAT-4 | All traffic on localhost | Monitor network during fingerprint capture | Only 127.0.0.1:7890 traffic | [ ] |

### Regression Checks
- [ ] Existing `analyze({what: "dom"})` functionality still works after fingerprinting is enabled
- [ ] Existing `observe({what: "accessibility"})` audit still functions
- [ ] Extension performance is not degraded (fingerprint extraction < 30ms on typical page)
- [ ] Pending query infrastructure handles both `dom` and `dom_fingerprint` query types simultaneously

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

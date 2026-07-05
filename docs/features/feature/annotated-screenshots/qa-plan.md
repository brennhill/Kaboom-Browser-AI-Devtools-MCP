---
status: shipped
scope: feature/annotated-screenshots/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-annotated-screenshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Annotated Screenshots

> QA plan for the Annotated Screenshots feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

**Note:** No tech-spec.md is available for this feature. This QA plan is based solely on the product-spec.md.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Screenshots capture visible passwords in plain-text input fields | Verify that `input[type="password"]` fields showing visible text (e.g., via "show password" toggle) appear in the screenshot. Document that this is an inherent risk of screenshot capture, mitigated by localhost-only delivery. | critical |
| DL-2 | Screenshots capture PII displayed on screen (SSN, credit card numbers, medical data) | Confirm screenshots only travel through localhost MCP responses. Verify no external endpoint receives image data. | critical |
| DL-3 | Annotation metadata exposes internal selectors and data-testid attributes | Verify selector strings in annotation map do not leak to any external service. Confirm selectors stay in MCP response on localhost only. | medium |
| DL-4 | Accessible name / ARIA labels may contain sensitive text (e.g., "Account balance: $45,000") | Verify that `name` and `text` fields in annotations contain only DOM-visible content. Confirm no hidden or aria-hidden content is extracted. | medium |
| DL-5 | Base64 image data persisted to disk unexpectedly | Verify annotated screenshot base64 data is transient (in-memory only during response serialization). Confirm no file is written to disk for annotated screenshots. | high |
| DL-6 | Rate limit bypass allows rapid screenshot extraction | Verify existing rate limits (5s cooldown, 10/session max) apply to annotated screenshots. Confirm no bypass via `annotate_screenshot` parameter. | medium |
| DL-7 | Custom annotation_selector could target hidden elements containing sensitive data | Verify `annotation_target: "custom"` with a selector targeting `[type="hidden"]` or off-screen elements does not extract non-visible content beyond what is in the viewport screenshot. | medium |

### Negative Tests (must NOT leak)
- [ ] Base64 image data must NOT be written to any log file on the server
- [ ] Annotation metadata must NOT be sent to any external HTTP endpoint
- [ ] Screenshot data must NOT persist in server memory after MCP response is sent
- [ ] Auth tokens or session cookies must NOT appear in annotation selector strings
- [ ] `document.cookie` or `localStorage` values must NOT appear in annotation text fields

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Label-to-selector mapping is unambiguous | Each numeric label in the image corresponds to exactly one entry in the `annotations` array. Verify label numbers are sequential with no gaps. | [ ] |
| CL-2 | interactionHint values are exhaustive and mutually exclusive | Verify each element gets exactly one of: `clickable`, `editable`, `selectable`, `toggleable`, `navigable`. No element has multiple hints. | [ ] |
| CL-3 | Bounding box coordinates use consistent units | Verify `bounds.x`, `bounds.y`, `bounds.width`, `bounds.height` are all in viewport pixels, matching the image dimensions. | [ ] |
| CL-4 | Empty annotations array is clearly distinguishable from error | When a page has no interactive elements, verify the response includes an empty `annotations: []` array (not null, not missing). | [ ] |
| CL-5 | Selector format is consistent and actionable | Verify all selectors in the annotation map are valid CSS selectors that can be used with `document.querySelector()`. | [ ] |
| CL-6 | Truncated text is indicated | When `text` exceeds 100 chars, verify it is truncated with an ellipsis or truncation marker so the LLM knows the text is incomplete. | [ ] |
| CL-7 | `total_found` is present when elements are omitted | When more elements exist than `max_annotations`, verify `total_found` is included so the LLM knows the annotation list is incomplete. | [ ] |
| CL-8 | Image and text content blocks are ordered predictably | Verify the image block is always first and the text block (annotation map) is always second in the `content` array. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM may assume label numbers correspond to DOM order (they correspond to viewport position). Verify documentation and response structure clarify ordering.
- [ ] LLM may try to use `bounds` coordinates for clicking (they are viewport-relative, not page-relative for scrolled pages). Verify bounds description clarifies viewport coordinates.
- [ ] LLM may confuse `role` (ARIA role) with `interactionHint` (how to interact). Verify these are clearly distinct fields with different purposes.
- [ ] LLM may assume all page elements are annotated when only interactive ones are (default). Verify response indicates which `annotation_target` was used.

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Get annotated screenshot of current page | 1 step: `observe({what: "page", annotate_screenshot: true})` | No -- already minimal |
| Get screenshot with custom element targeting | 1 step with `annotation_target: "custom"` and `annotation_selector` | No -- single call |
| Get page metadata without screenshot | 1 step: `observe({what: "page"})` (default, unchanged) | No -- backward compatible |
| Annotate and then interact with labeled element | 2 steps: observe + interact | Could be 1 step with future "click by label" feature, but 2 is acceptable |

### Default Behavior Verification
- [ ] Feature works with zero configuration (`annotate_screenshot` defaults to `false`, preserving existing behavior)
- [ ] Setting only `annotate_screenshot: true` uses sensible defaults (`annotation_target: "interactive"`, `max_annotations: 50`)
- [ ] Omitting `annotate_screenshot` produces identical output to current `page` mode (full backward compatibility)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Parse annotate_screenshot parameter (true) | `{"what": "page", "annotate_screenshot": true}` | Parameters struct with AnnotateScreenshot = true | must |
| UT-2 | Parse annotate_screenshot parameter (false/omitted) | `{"what": "page"}` | Parameters struct with AnnotateScreenshot = false | must |
| UT-3 | Validate annotation_target enum | `"interactive"`, `"all_visible"`, `"custom"`, `"invalid"` | Accept first three, reject "invalid" with error | must |
| UT-4 | Validate max_annotations range | `0`, `1`, `50`, `100`, `101`, `-1` | Accept 1-100, reject 0, 101, -1 with error | must |
| UT-5 | Validate custom annotation_selector required when target is "custom" | `{"annotation_target": "custom", "annotation_selector": null}` | Error: annotation_selector required when target is custom | must |
| UT-6 | Selector priority chain produces best selector | Element with data-testid, id, and aria-label | Selector uses `[data-testid]` (highest priority) | must |
| UT-7 | Selector fallback to CSS path | Element with no id, no data-testid, no aria-label | CSS path like `body > div:nth-child(2) > button` | must |
| UT-8 | Annotation ordering (top-to-bottom, left-to-right) | 3 elements at positions (100,200), (100,100), (200,100) | Labels assigned: (100,100)=1, (200,100)=2, (100,200)=3 | must |
| UT-9 | Text truncation at 100 characters | Element with 150-char visible text | `text` field is 100 chars with truncation indicator | should |
| UT-10 | interactionHint classification | button, input, select, checkbox, a[href] | clickable, editable, selectable, toggleable, navigable | should |
| UT-11 | Empty page produces empty annotations | Page with no interactive elements | `annotations: []`, valid image returned | must |
| UT-12 | max_annotations respected | Page with 80 interactive elements, max_annotations=50 | Only 50 annotations returned, `total_found: 80` | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end annotated screenshot capture | Go server + Extension + OffscreenCanvas | MCP response contains image block + text block with annotation map | must |
| IT-2 | Screenshot rate limiting applies to annotated screenshots | Go server + Extension rate limiter | Request within 5s cooldown returns rate limit error | must |
| IT-3 | Extension disconnected during annotation request | Go server + timeout handler | Standard timeout error returned, same as current page mode | must |
| IT-4 | Backward compatibility: observe page without annotate_screenshot | Go server full pipeline | Response identical to pre-feature page mode output | must |
| IT-5 | Mixed content response (image + text blocks) serialization | Go server JSON marshaling | Valid JSON with both `type: "image"` and `type: "text"` content blocks | must |
| IT-6 | Async query pipeline handles annotation data collection | Extension inject.js + background.js + Go server | Element discovery data flows correctly through pending query system | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Element discovery time for 100 elements | inject.js execution time | < 50ms | must |
| PT-2 | Canvas annotation rendering time | OffscreenCanvas rendering | < 100ms | must |
| PT-3 | Total end-to-end latency | MCP request to response | < 500ms | must |
| PT-4 | Base64 response payload size | JPEG quality 80 screenshot size | < 500KB typical | must |
| PT-5 | Memory during annotation | Transient canvas + image decode | < 5MB | must |
| PT-6 | Page main thread not blocked | Main thread blocking duration | 0ms (canvas work in service worker) | must |
| PT-7 | Element discovery with 500 DOM elements | inject.js querySelectorAll scan | < 100ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No interactive elements on page | Blank page or static content page | Empty annotations array, screenshot still returned | must |
| EC-2 | More than max_annotations elements | Page with 200 buttons, max_annotations=50 | First 50 by viewport position annotated, `total_found: 200` | must |
| EC-3 | Element with no usable selector | div with no id, class, or aria-label | CSS path fallback generated | must |
| EC-4 | Extension disconnected | Extension not connected | Timeout error returned | must |
| EC-5 | chrome:// or extension page tab | Tab on chrome://settings | Error: "Cannot capture screenshots of browser internal pages" | must |
| EC-6 | Overlapping elements (z-index) | Two buttons stacked at same position | Both annotated, label positions offset to avoid overlap | should |
| EC-7 | Invalid custom annotation_selector | `annotation_target: "custom"`, `annotation_selector: "[[[invalid"` | Error with invalid selector string and syntax hint | should |
| EC-8 | Rate-limited screenshot request | Request during 5s cooldown | Rate limit error with `nextAllowedIn` field | must |
| EC-9 | Page still loading (readyState != complete) | Page in "loading" or "interactive" state | Annotate available elements, include `document.readyState` in response | should |
| EC-10 | Shadow DOM elements | Web component with open shadow root | Elements may not be discovered; behavior depends on OI-5 resolution | could |
| EC-11 | Elements outside viewport | Scrollable page with elements below fold | Only viewport-visible elements annotated (as per spec) | must |
| EC-12 | Very large bounding boxes | Full-width banner element | Bounding box accurately represents element size, label positioned clearly | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web page with interactive elements loaded in tracked tab (e.g., a form page with buttons, inputs, links)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "observe", "arguments": {"what": "page", "annotate_screenshot": true}}` | Browser captures screenshot (brief flash may occur) | Response contains `image` content block (base64 JPEG) and `text` content block with annotation JSON | [ ] |
| UAT-2 | Inspect annotation map in response | Compare numbered labels on screenshot image to annotation entries | Each numbered label on image has matching entry in annotations array with correct selector, role, and text | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "page", "annotate_screenshot": true, "max_annotations": 3}}` | Page with many interactive elements | Only 3 annotations returned; `total_found` shows actual count | [ ] |
| UAT-4 | `{"tool": "observe", "arguments": {"what": "page", "annotate_screenshot": true, "annotation_target": "all_visible"}}` | Page with headings, paragraphs, and interactive elements | All visible elements with text/role are annotated, not just interactive ones | [ ] |
| UAT-5 | `{"tool": "observe", "arguments": {"what": "page", "annotate_screenshot": true, "annotation_target": "custom", "annotation_selector": "button"}}` | Page with buttons and other elements | Only `<button>` elements are annotated | [ ] |
| UAT-6 | `{"tool": "observe", "arguments": {"what": "page"}}` (no annotate_screenshot) | Normal page metadata request | Response is identical to pre-feature behavior: no image block, standard page metadata only | [ ] |
| UAT-7 | `{"tool": "observe", "arguments": {"what": "page", "annotate_screenshot": true, "annotation_target": "custom", "annotation_selector": "[[[invalid"}}` | Invalid CSS selector provided | Error response with invalid selector string and hint to check syntax | [ ] |
| UAT-8 | Request annotated screenshot twice within 5 seconds | Rate limit behavior | Second request returns rate limit error with `nextAllowedIn` field | [ ] |
| UAT-9 | Navigate to blank page, then: `{"tool": "observe", "arguments": {"what": "page", "annotate_screenshot": true}}` | Page with no interactive elements | Screenshot returned with empty `annotations: []` array; page metadata still included | [ ] |
| UAT-10 | Use annotation selector from response to interact with element | Copy selector from annotation label 1 | Selector successfully targets the correct element on the page | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Screenshot data stays on localhost | Monitor network traffic during annotated screenshot request (browser DevTools Network tab) | No outbound requests containing image data | [ ] |
| DL-UAT-2 | No disk persistence of annotated screenshots | Check server working directory for new image files after annotated screenshot request | No new files created | [ ] |
| DL-UAT-3 | Annotation text does not contain hidden element content | Load page with hidden inputs/elements, request annotated screenshot | Only visible viewport elements appear in annotations | [ ] |
| DL-UAT-4 | Server logs do not contain base64 image data | Check server stdout/stderr logs after annotated screenshot request | No base64 strings in logs (filenames/metadata OK, raw image data not OK) | [ ] |

### Regression Checks
- [ ] Existing `observe({what: "page"})` without `annotate_screenshot` returns identical output to pre-feature behavior
- [ ] Screenshot rate limiting still works for non-annotated screenshots
- [ ] Other observe modes (errors, logs, network) are unaffected
- [ ] Extension performance is not degraded when feature is not in use (no overhead when annotate_screenshot is false/omitted)

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

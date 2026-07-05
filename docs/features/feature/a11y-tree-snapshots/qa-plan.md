---
status: proposed
scope: feature/a11y-tree-snapshots/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-a11y-tree-snapshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: A11y Tree Snapshots

> QA plan for the A11y Tree Snapshots (Text-Based Page Representation) feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Password field values in a11y tree text | Verify password inputs (`type="password"`) have their value replaced with `"[REDACTED]"` per the spec's redaction rule. The accessible name (placeholder text) is fine, but the current value must not appear. | critical |
| DL-2 | Credit card / secret fields exposed | Verify inputs with `autocomplete` hints containing `"password"`, `"cc-"`, or `"secret"` have values replaced with `"[REDACTED]"` per the `SENSITIVE_INPUT_TYPES` constant. | critical |
| DL-3 | User-visible PII in accessible names | Accessible names may contain emails, phone numbers, and usernames displayed on the page (e.g., `text "Welcome, john@example.com"`). Since data stays on localhost, this is acceptable but must be documented. | medium |
| DL-4 | UID map CSS selectors revealing structure | `uidMap` contains CSS selectors like `nav > a:nth-child(1)`. These reveal DOM structure but not sensitive data. Verify selectors do not contain user data. | low |
| DL-5 | Form input current values in tree text | Non-password text inputs may show their current `value=` attribute in the tree. Verify `value=` shows current value for non-sensitive inputs and `"[REDACTED]"` for sensitive ones. | high |
| DL-6 | Link URLs containing session tokens | The tree shows `url=/path` for links. Verify only the pathname is included, not full URLs with query parameters containing tokens. | high |
| DL-7 | Alert/notification text containing PII | The tree captures `text "..."` from all visible elements. Error messages or notifications may contain user-specific data like "Account <john@example.com> locked". | medium |
| DL-8 | localStorage/sessionStorage not accessed | Verify the tree traversal reads only DOM element properties and ARIA attributes, not browser storage APIs. | critical |
| DL-9 | Data transmission path | Verify all a11y tree data flows only over localhost (127.0.0.1:7890). No external network calls. | critical |
| DL-10 | Hidden ARIA elements skipping | Verify `aria-hidden="true"` elements and their descendants are properly skipped, preventing accidental capture of hidden-but-present sensitive data. | high |

### Negative Tests (must NOT leak)
- [ ] Password field values are replaced with `"[REDACTED]"` in the tree text
- [ ] Fields with `autocomplete="cc-number"` have values redacted
- [ ] `document.cookie` and `localStorage` are NOT accessed during traversal
- [ ] Link `url=` attributes show paths only, not full URLs with query params
- [ ] No tree data is transmitted to external servers
- [ ] `aria-hidden="true"` elements and all descendants are excluded from the tree

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Tree text format parsing | LLM can parse indented text format: `role "name" [uid=N] attr=value`. Verify indentation depth maps to nesting. | [ ] |
| CL-2 | UID semantics | LLM understands `[uid=N]` only appears on interactive elements, and can use the `uidMap` to get a CSS selector for follow-up actions. | [ ] |
| CL-3 | Implicit role mapping | LLM understands `heading[1]` means an h1, `textbox` means an input, `link` means an anchor. Verify role names match standard ARIA roles. | [ ] |
| CL-4 | Interactive vs non-interactive distinction | Elements without `[uid=N]` are non-interactive (decorative, structural). LLM should know it cannot click/type on them. | [ ] |
| CL-5 | Truncation indicators | `...(truncated)` and `...(N more nodes truncated)` clearly signal incomplete data. LLM should know to narrow scope or increase depth. | [ ] |
| CL-6 | `maxDepthReached` flag | `true` means the tree was cut short. LLM should request a narrower `scope` or accept partial data. | [ ] |
| CL-7 | Table and list summarization | `table "Name" (24 rows)` and `list "Name" (N items)` -- LLM should understand these are summaries with only first few items shown. | [ ] |
| CL-8 | Scope parameter understanding | `rootSelector: "body"` vs `rootSelector: "#main"` -- LLM should understand which subtree was traversed. | [ ] |
| CL-9 | Token estimate utility | `tokenEstimate.savings: "84%"` -- LLM should use this to justify choosing a11y_tree over analyze({what: "dom"}). | [ ] |
| CL-10 | `[REDACTED]` meaning | LLM should understand `[REDACTED]` means a value exists but was intentionally hidden for security, not that the field is empty. | [ ] |
| CL-11 | Error response for scope not found | `error: "scope_not_found"` -- LLM should suggest trying a different selector or `"body"`. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM confuses `a11y_tree` (semantic tree) with `accessibility` (WCAG audit) -- test by verifying response formats are fundamentally different
- [ ] LLM tries to use UID directly as a CSS selector instead of looking up the `uidMap` -- test by verifying `uidMap` documentation is clear
- [ ] LLM interprets `text "Active users: 1,234"` as an interactive element because it has a number -- verify non-interactive elements lack `[uid=N]`
- [ ] LLM misreads `heading[1]` as "heading number 1" instead of "h1 heading" -- verify naming convention is documented
- [ ] LLM does not realize `interactive_only: true` omits structural context needed to understand page layout

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (single snapshot), Medium (snapshot + follow-up interact)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Get full page tree | 1 step: `observe({what: "a11y_tree"})` | No -- already minimal |
| Get interactive elements only | 1 step: add `interactive_only: true` | No -- single parameter |
| Scope to a subtree | 1 step: add `scope: "#main"` | No -- single parameter |
| Find an element and click it | 2 steps: (1) get tree with UIDs, (2) use uidMap selector in `interact({action: "execute_js"})` | Could be 1 step with future `click_uid` action (OI-2) |
| Re-check page after action | 1 step: re-request `a11y_tree` | No -- snapshots are stateless |

### Default Behavior Verification
- [ ] Feature works with zero parameters (defaults to `scope: "body"`, `interactive_only: false`, `max_depth: 8`)
- [ ] Default output is useful for understanding full page structure
- [ ] UIDs are automatically assigned to interactive elements (no opt-in needed)
- [ ] `uidMap` is always included in the response when UIDs exist

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | `buildA11yTree` returns correct structure | Standard HTML page with nav, main, form | Indented text tree with roles, names, UIDs | must |
| UT-2 | Implicit role mapping for common tags | `<button>`, `<a href>`, `<input type=text>`, `<h1>` | Roles: `button`, `link`, `textbox`, `heading[1]` | must |
| UT-3 | Explicit role override | `<div role="button">` | Role: `button` with UID assigned | must |
| UT-4 | UID assigned to interactive elements only | Mix of interactive and non-interactive elements | Only interactive elements have `[uid=N]` | must |
| UT-5 | UID determinism | Same page tree built twice | Same UIDs assigned to same elements | should |
| UT-6 | Accessible name priority: aria-label first | Element with `aria-label`, `textContent`, `title` | Name from `aria-label` | must |
| UT-7 | Accessible name from aria-labelledby | Element with `aria-labelledby` referencing another element | Name from referenced element's text | must |
| UT-8 | Accessible name from alt text | `<img alt="Logo">` | Name: `"Logo"` | must |
| UT-9 | Accessible name fallback to textContent | Element with only text content | Name from textContent (first 100 chars) | must |
| UT-10 | Text truncation at 100 chars | Element with 200-char textContent | Text truncated with ellipsis | must |
| UT-11 | `aria-hidden="true"` elements skipped | Element with `aria-hidden="true"` | Element and all descendants excluded | must |
| UT-12 | `role="presentation"` elements skipped | Element with `role="presentation"` | Element excluded from tree | must |
| UT-13 | `interactive_only` filter | Page with 50 elements, 10 interactive | Only 10 elements in output | must |
| UT-14 | `max_depth` parameter | Tree deeper than `max_depth` | Traversal stops, `maxDepthReached: true` | must |
| UT-15 | `max_depth` cap at 15 | `max_depth: 20` | Capped to 15 | should |
| UT-16 | `scope` parameter | `scope: "#sidebar"` | Only `#sidebar` subtree traversed | must |
| UT-17 | Scope element not found | `scope: "#nonexistent"` | `error: "scope_not_found"` | must |
| UT-18 | Table summarization | Table with 24 rows | `table "Name" (24 rows)` with first 3 rows | should |
| UT-19 | List summarization | List with 20 items | `list "Name" (20 items)` with first 5 items | should |
| UT-20 | Password field redaction | `<input type="password" value="secret">` | Value shows `"[REDACTED]"` | must |
| UT-21 | Sensitive autocomplete redaction | `<input autocomplete="cc-number" value="4111...">` | Value shows `"[REDACTED]"` | must |
| UT-22 | CSS selector generation for uidMap | Element with ID, element with data-testid, element with only tag | `#id`, `[data-testid]`, positional selector | must |
| UT-23 | Node count cap at 5,000 | Page with 10,000 nodes | Processing stops at 5,000, remainder summarized | must |
| UT-24 | Output size cap at 50KB | Complex page producing large tree | Output truncated at 50KB | must |
| UT-25 | Empty page (about:blank) | `about:blank` | `document "(empty)"`, zero interactive elements | should |
| UT-26 | Attribute output for interactive elements | Button with `disabled`, select with `expanded` | Attributes shown inline: `disabled`, `expanded` | should |
| UT-27 | Link URL attribute | `<a href="/settings">` | `url=/settings` shown in tree | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full a11y tree round trip | Go server (`toolObserveA11yTree`) -> background.js -> content.js -> inject.js -> `buildA11yTree` | Complete tree returned via MCP | must |
| IT-2 | Scope parameter forwarded through chain | Server passes `scope: "#main"` through all hops | Tree scoped to `#main` subtree | must |
| IT-3 | `interactive_only` parameter forwarded | Server passes `interactive_only: true` | Only interactive elements in tree | must |
| IT-4 | Extension timeout handling | Extension disconnected | Timeout error with recovery hint | must |
| IT-5 | New query type `a11y_tree` dispatched | Server creates `PendingQuery{Type:"a11y_tree"}` | Extension correctly routes to `buildA11yTree` handler | must |
| IT-6 | Concurrent a11y_tree and analyze({what: "dom"}) requests | Both in flight simultaneously | Both return correct results with no cross-contamination | should |
| IT-7 | uidMap CSS selectors are valid | AI uses selector from uidMap in subsequent `analyze({what: "dom"})` call | `analyze({what: "dom"})` finds the correct element | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Traversal time on typical page | Execution time for <500 nodes | < 100ms | must |
| PT-2 | Traversal time on complex page | Execution time for 2000+ nodes | < 500ms | must |
| PT-3 | Response size for typical page | Bytes of response | < 10KB | must |
| PT-4 | Response size cap | Complex page | < 50KB | must |
| PT-5 | Memory during traversal | Transient memory allocation | < 2MB | should |
| PT-6 | Main thread blocking | Continuous blocking time | < 50ms | must |
| PT-7 | Token efficiency vs analyze({what: "dom"}) | Compare token counts for same page | a11y_tree is 8-12x more efficient | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Blank page (about:blank) | Navigate to about:blank | `document "(empty)"`, nodeCount: 0 | must |
| EC-2 | Page mid-load | Request during DOM construction | Partial tree reflecting current state | should |
| EC-3 | Extremely deep DOM (depth > 15) | Deeply nested elements | Traversal stops at max_depth, `maxDepthReached: true` | must |
| EC-4 | 10,000+ DOM nodes | Complex web app | Node processing caps at 5,000, remainder summarized | must |
| EC-5 | No interactive elements | Static content page | Empty uidMap, `interactiveCount: 0` | should |
| EC-6 | All elements aria-hidden | Page with `body[aria-hidden=true]` | Nearly empty tree | should |
| EC-7 | Shadow DOM content | Web Components with shadow roots | Not traversed (documented limitation) | should |
| EC-8 | iframe content | Page with iframes | Only top-level document traversed | should |
| EC-9 | Custom elements without roles | `<my-component>` without explicit role | Treated as `generic` role | should |
| EC-10 | Multiple elements with same name | Two buttons both labeled "Submit" | Both appear in tree with unique UIDs | should |
| EC-11 | Concurrent a11y_tree requests | Two requests in flight | Independent traversals, no caching | could |
| EC-12 | Unicode and RTL text in names | Arabic/Hebrew accessible names | Correctly included in tree text | could |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page loaded with: navigation links, headings, a form with text/password/checkbox inputs, a data table, a list, buttons, an error alert, and an `aria-hidden` section
- [ ] Tab is being tracked by the extension

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "observe", "arguments": {"what": "a11y_tree"}}` | Page has standard structure | Indented text tree with roles, names, UIDs for interactive elements. `uidMap` included. | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"what": "a11y_tree", "interactive_only": true}}` | Page has ~10 interactive elements | Tree contains only interactive elements (buttons, links, inputs). Non-interactive content omitted. | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "a11y_tree", "scope": "#main"}}` | Main section has specific content | Only elements within `#main` appear in tree | [ ] |
| UAT-4 | `{"tool": "observe", "arguments": {"what": "a11y_tree", "scope": "#nonexistent"}}` | No such element | `error: "scope_not_found"` | [ ] |
| UAT-5 | `{"tool": "observe", "arguments": {"what": "a11y_tree", "max_depth": 2}}` | Deep page structure | Tree limited to 2 levels of nesting, `maxDepthReached: true` | [ ] |
| UAT-6 | Verify password field redaction | Password field has value entered | `textbox "[REDACTED]"` or `value="[REDACTED]"` appears, not the actual password | [ ] |
| UAT-7 | Verify UID assignment | Count interactive elements visually | `interactiveCount` matches the number of `[uid=N]` markers in the tree text | [ ] |
| UAT-8 | Verify uidMap selector validity -- pick a UID, use its selector | AI uses uidMap selector in `analyze({what: "dom", selector: "..."})` | `analyze({what: "dom"})` finds exactly the element the UID refers to | [ ] |
| UAT-9 | Verify table summarization | Page has a table with 24 rows | Tree shows `table "Name" (24 rows)` with first 3 rows shown | [ ] |
| UAT-10 | Verify list summarization | Page has a list with 20 items | Tree shows `list "Name" (20 items)` with first 5 items shown | [ ] |
| UAT-11 | Verify aria-hidden exclusion | Section with `aria-hidden="true"` | Section and all its children do NOT appear in tree | [ ] |
| UAT-12 | Verify role="presentation" exclusion | Element with `role="presentation"` | Element does NOT appear in tree | [ ] |
| UAT-13 | Verify token estimate | Compare `tokenEstimate.thisResponse` to actual tree text token count | Estimate is within 20% of actual | [ ] |
| UAT-14 | Verify nodeCount accuracy | Count nodes in tree text | `nodeCount` matches the number of lines in the tree (approximately) | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Password values redacted | Enter "MyPassword123" in password field, get tree | `"[REDACTED]"` appears, "MyPassword123" does NOT | [ ] |
| DL-UAT-2 | Credit card field redacted | Enter "4111111111111111" in `autocomplete="cc-number"` field, get tree | `"[REDACTED]"` appears, card number does NOT | [ ] |
| DL-UAT-3 | Link URLs are paths only | Page has link with `?token=abc123` | Tree shows `url=/path` without query params | [ ] |
| DL-UAT-4 | All traffic on localhost | Monitor network during tree capture | Only 127.0.0.1:7890 traffic | [ ] |
| DL-UAT-5 | No storage APIs accessed | Check extension console for storage access | No `localStorage`, `sessionStorage`, or `document.cookie` access | [ ] |

### Regression Checks
- [ ] Existing `observe({what: "logs"})` still works
- [ ] Existing `observe({what: "network"})` still works
- [ ] Existing `observe({what: "accessibility"})` audit still works (different from a11y_tree)
- [ ] Existing `analyze({what: "dom"})` still works
- [ ] Extension performance is not degraded by adding the new `a11y-tree.js` module

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

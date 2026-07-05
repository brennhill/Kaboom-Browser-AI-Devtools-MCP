---
feature: a11y-tree-snapshots
status: proposed
version: null
tool: observe
mode: a11y_tree
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-a11y-tree-snapshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# A11y Tree Snapshots (Text-Based Page Representation)

> Export the accessibility tree as a compact text representation with stable UIDs, enabling AI agents to read page structure and reference interactive elements for subsequent actions.

## Problem

AI coding agents need to understand what is on a web page to interact with it intelligently. Currently, Kaboom offers two page-reading mechanisms:

1. **`analyze {what: "dom"}`** -- Returns structural DOM data (tags, attributes, bounding boxes) for elements matching a CSS selector. This is powerful but requires the AI to already know what to look for (a selector), returns structural rather than semantic data, and produces verbose output that consumes many tokens.

2. **`observe {what: "accessibility"}`** -- Runs an axe-core audit to find WCAG violations. This is an audit tool, not a page-reading tool. It answers "what is broken?" not "what is on the page?"

Neither provides what an AI agent needs most: a concise, semantic overview of the entire page with stable identifiers for interactive elements. Without this, agents must either parse raw HTML (expensive, lossy) or make blind guesses about page structure.

**The gap:** There is no way for an AI to say "show me everything on this page in a way I can understand and act on" and get back a token-efficient, semantically meaningful representation with element references it can use in follow-up `interact` calls.

## Solution

Add `observe {what: "a11y_tree"}` -- a new mode that traverses the page's accessibility tree (via the DOM's `TreeWalker` with ARIA role/label extraction) and returns a compact, indented text representation. Each interactive element is assigned a stable UID that persists across snapshots of the same page, enabling the AI to reference elements in subsequent `interact` calls (e.g., `interact {action: "execute_js", uid: 42}`).

### Key design decisions:

- **Manual traversal, not `chrome.automation`**: The `chrome.automation` API is only available to extensions with the `automation` permission and does not work in content scripts. Instead, we use a `TreeWalker` with ARIA attribute extraction (`role`, `aria-label`, `aria-expanded`, etc.) which runs in inject.js alongside existing DOM query code. This is the same approach used by Playwright's accessibility tree implementation.

- **Stable UIDs via deterministic hashing**: Elements are assigned UIDs based on a combination of their DOM path, tag, role, and name. This makes UIDs stable across repeated snapshots of the same page (same element = same UID), while being unique within a snapshot. UIDs are integers for token efficiency.

- **Text format, not JSON**: The output is an indented text tree, not a JSON structure. This is deliberate -- indented text is 3-5x more token-efficient than equivalent JSON and is easier for LLMs to parse. JSON metadata (page URL, element count, UID map) wraps the text tree.

## User Stories

- As an AI coding agent, I want to get a semantic overview of a page so that I can understand its structure without parsing HTML.
- As an AI coding agent, I want interactive elements labeled with stable UIDs so that I can reference them in follow-up `interact` calls (e.g., "click the login button at uid=42").
- As a developer using Kaboom, I want a token-efficient page representation so that my AI agent stays within context limits on complex pages.
- As an AI coding agent, I want to filter the tree to only interactive elements so that I can quickly find what I can click, type into, or toggle.

## MCP Interface

**Tool:** `observe`
**Mode:** `a11y_tree`

### Request

```json
{
  "tool": "observe",
  "arguments": {
    "what": "a11y_tree",
    "scope": "#main",
    "interactive_only": false,
    "max_depth": 8
  }
}
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `what` | string | (required) | Must be `"a11y_tree"` |
| `scope` | string | `"body"` | CSS selector for root element to start traversal |
| `interactive_only` | boolean | `false` | If true, only emit interactive elements (links, buttons, inputs, etc.) |
| `max_depth` | integer | `8` | Maximum tree depth to traverse (capped at 15) |

### Response

```json
{
  "url": "https://example.com/dashboard",
  "pageTitle": "Dashboard - MyApp",
  "rootSelector": "body",
  "nodeCount": 127,
  "interactiveCount": 23,
  "maxDepthReached": false,
  "tree": "document \"Dashboard - MyApp\"\n  navigation \"Main Nav\"\n    link \"Home\" [uid=1] url=/\n    link \"Settings\" [uid=2] url=/settings\n  main\n    heading[1] \"Dashboard\"\n    region \"Stats\"\n      text \"Active users: 1,234\"\n    form \"Search\" [uid=3]\n      textbox \"Search...\" [uid=4]\n      button \"Go\" [uid=5]\n    table \"Recent Activity\" (24 rows)\n      ...(truncated)\n",
  "uidMap": {
    "1": { "selector": "nav > a:nth-child(1)", "tag": "a", "role": "link" },
    "2": { "selector": "nav > a:nth-child(2)", "tag": "a", "role": "link" },
    "3": { "selector": "main form", "tag": "form", "role": "form" },
    "4": { "selector": "main form input[type=text]", "tag": "input", "role": "textbox" },
    "5": { "selector": "main form button", "tag": "button", "role": "button" }
  },
  "tokenEstimate": {
    "thisResponse": 340,
    "equivalentQueryDom": 2100,
    "savings": "84%"
  }
}
```

#### Tree text format:

```
role "name" [uid=N] attr=value
  child_role "child_name" [uid=M]
    ...
```

- Indentation = 2 spaces per depth level
- `role` = ARIA role or implicit role from tag (e.g., `button`, `link`, `heading[1]`, `textbox`)
- `"name"` = accessible name (aria-label, text content, alt text, title)
- `[uid=N]` = only present on interactive elements
- Additional attributes shown inline: `url=`, `value=`, `checked`, `disabled`, `expanded`, `selected`, `required`
- Tables summarized as `table "Name" (N rows)` with first 3 rows shown
- Lists summarized as `list "Name" (N items)` with first 5 items shown
- Text nodes > 100 chars truncated with ellipsis

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Traverse accessibility tree from scope root using TreeWalker with ARIA attribute extraction | must |
| R2 | Assign stable integer UIDs to interactive elements (links, buttons, inputs, selects, textareas, elements with `role` attribute indicating interactivity) | must |
| R3 | Return indented text tree format (not JSON tree) for token efficiency | must |
| R4 | Include `uidMap` mapping each UID to a CSS selector + tag + role for use by `interact` tool | must |
| R5 | Support `scope` parameter to limit traversal to a subtree | must |
| R6 | Support `interactive_only` filter to show only actionable elements | should |
| R7 | Support `max_depth` parameter (default 8, max 15) to limit tree depth | should |
| R8 | Summarize tables (row count + first 3 rows) and lists (item count + first 5 items) to avoid token explosion | should |
| R9 | Truncate text content > 100 characters | should |
| R10 | Include `tokenEstimate` comparing a11y_tree vs equivalent analyze({what: "dom"}) output size | could |
| R11 | Cap total output at 50KB to prevent oversized responses on complex pages | must |
| R12 | UIDs must be deterministic: same page state produces same UIDs | should |

## Non-Goals

- This feature does NOT run WCAG compliance checks. Use `observe {what: "accessibility"}` for audits.
- This feature does NOT capture visual layout or screenshots. It is a text-only semantic representation.
- This feature does NOT modify the DOM or inject visible elements into the page.
- Out of scope: Shadow DOM traversal. Shadow DOM support may be added in a future iteration.
- Out of scope: iframe traversal. Only the top-level document (or scoped subtree) is traversed.
- This feature does NOT provide a "click by UID" action directly. The `uidMap` provides CSS selectors that the AI uses with existing `interact {action: "execute_js"}` or `interact {action: "highlight"}` calls.

## Performance SLOs

| Metric | Target | Rationale |
|--------|--------|-----------|
| Traversal time (typical page, <500 nodes) | < 100ms | Must not degrade browsing; comparable to DOM query |
| Traversal time (complex page, 2000+ nodes) | < 500ms | Heavy pages should still complete within timeout |
| Response size (typical page) | < 10KB | Token-efficient is the core value proposition |
| Response size (complex page) | < 50KB | Hard cap prevents context window overflow |
| Memory impact during traversal | < 2MB | Transient allocation, GC'd after response |
| Main thread blocking | < 50ms continuous | Must yield via batching if traversal is long |

## Security Considerations

- **Data captured:** Accessible names (text content, labels), ARIA attributes, element roles, and CSS selectors. No raw HTML, no attribute values beyond ARIA-related ones, no computed styles.
- **Redaction:** Input values for password fields (`type="password"`) and fields with `autocomplete` hints containing "password", "cc-", or "secret" are replaced with `"[REDACTED]"`. This follows the existing pattern in Kaboom's sensitive input type detection (`SENSITIVE_INPUT_TYPES` constant).
- **Privacy implications:** The accessible name may contain user-visible text (e.g., email addresses displayed on the page). This is equivalent to what `analyze({what: "dom"})` already exposes. Since data stays on localhost, this is acceptable.
- **Attack surface:** No change. This feature reads the DOM (read-only). It does not execute arbitrary code, inject scripts, or modify page state. The traversal runs in inject.js in the same security context as existing DOM queries.
- **UID stability:** UIDs are derived from DOM structure, not from sensitive content. An attacker cannot learn private data from observing UIDs.

## Edge Cases

- **What happens when the scoped element does not exist?** Expected behavior: Return `{ error: "scope_not_found", message: "No element matches selector '#nonexistent'" }`.
- **What happens on a blank page (about:blank)?** Expected behavior: Return a minimal tree with just `document "(empty)"` and zero interactive elements.
- **What happens when the page is mid-load?** Expected behavior: Return whatever is currently in the DOM. The snapshot reflects the instant of traversal. The AI can re-request after load completes.
- **What happens with extremely deep DOM trees (depth > 15)?** Expected behavior: Traversal stops at `max_depth`. `maxDepthReached` is set to `true`. A `hint` field suggests narrowing scope.
- **What happens with 10,000+ DOM nodes?** Expected behavior: Node processing caps at 5,000 nodes. Remaining nodes are summarized as `...(N more nodes truncated)`. Total output capped at 50KB.
- **What happens when the extension is disconnected?** Expected behavior: Standard timeout error (consistent with existing DOM query and a11y audit timeout behavior).
- **What happens with ARIA `role="presentation"` or `role="none"`?** Expected behavior: These elements are skipped (they are explicitly marked as non-semantic).
- **What happens with `aria-hidden="true"` elements?** Expected behavior: Skipped, along with all descendants. These are intentionally hidden from assistive technology.
- **What happens when concurrent a11y_tree requests arrive?** Expected behavior: Each request creates an independent traversal. No caching for this mode (unlike the a11y audit) since page state changes frequently and the snapshot should always be fresh.

## Dependencies

- **Depends on:** Existing query dispatch infrastructure (`pending-queries` polling, `content.js` message bridge, `inject.js` execution context). Uses the same request/response pattern as `analyze({what: "dom"})` and `a11y` audit.
- **Depends on:** `dom-queries.js` module for shared constants and utilities (element visibility check, text truncation limits).
- **Depended on by:** AI agents that need to identify interactive elements before issuing `interact` commands. The `uidMap` output is designed to bridge `observe` and `interact` tools.

## Assumptions

- A1: The extension is connected and tracking a tab (standard precondition for all on-demand queries).
- A2: The page has a meaningful DOM loaded (not a browser internal page like `chrome://extensions`).
- A3: ARIA roles and labels on the page are reasonably well-authored. Poorly-labeled pages will produce a less useful tree, but the feature will still function (falling back to tag-based implicit roles).
- A4: The `TreeWalker` API is available in all Chromium-based browsers that Kaboom targets (Chrome 90+). This is a well-supported, stable API.
- A5: Integer UIDs (1, 2, 3, ...) are sufficient. We do not need UUIDs or globally unique identifiers since UIDs are only meaningful within a single snapshot and are regenerated on each call.

## Implementation Notes

### Tree Traversal Strategy

Use `document.createTreeWalker()` with `NodeFilter.SHOW_ELEMENT` to walk the DOM tree. For each element:

1. **Compute role:** Check `role` attribute first. Fall back to implicit role mapping (e.g., `<a href>` = `link`, `<button>` = `button`, `<input type="text">` = `textbox`, `<h1>` = `heading[1]`).
2. **Compute accessible name:** Check `aria-label` > `aria-labelledby` (resolve referenced element text) > `alt` > `title` > direct text content (first 100 chars).
3. **Assign UID:** Interactive elements (role in `{link, button, textbox, checkbox, radio, combobox, listbox, menuitem, tab, switch, slider, spinbutton, searchbox}` or elements with `tabindex >= 0`) get an auto-incrementing integer UID.
4. **Build CSS selector:** For UID'd elements, compute a minimal CSS selector (prefer `#id`, then `[name=...]`, then positional `tag:nth-of-type(n)` up to 3 ancestors).
5. **Format output:** Indent by depth, emit `role "name" [uid=N] attributes`.

### Token Efficiency Comparison

Estimated token costs for a typical dashboard page (~200 DOM nodes, ~30 interactive elements):

| Approach | Output size | Est. tokens | Notes |
|----------|------------|-------------|-------|
| `analyze({what: "dom"})` (selector: `*`) | ~45KB | ~12,000 | Includes all attributes, bounding boxes, styles |
| `a11y_tree` (full) | ~4KB | ~1,000 | Roles + names + UIDs only |
| `a11y_tree` (interactive_only) | ~1.5KB | ~400 | Just actionable elements |

The a11y tree is estimated to be **8-12x more token-efficient** than equivalent DOM query output for typical pages. This is its primary value proposition.

### New Extension Files

- `extension/lib/a11y-tree.js` -- Tree traversal, UID assignment, text formatting (~200 lines)
- Constants added to `extension/lib/constants.js` (max nodes, max depth, max text length)

### New Server Handler

- `toolObserveA11yTree()` in `queries.go` -- Creates pending query of type `"a11y_tree"`, waits for result, formats response
- New query type `"a11y_tree"` dispatched through existing `background.js` polling and `content.js` bridge

### Message Flow

```
AI calls observe({what:"a11y_tree", scope:"#main"})
  -> Go server: toolObserveA11yTree() creates PendingQuery{Type:"a11y_tree"}
  -> Extension polls /pending-queries, receives query
  -> background.js dispatches to content.js: {type:"A11Y_TREE_QUERY", params:{scope,max_depth,...}}
  -> content.js forwards to inject.js via postMessage: {type:"KABOOM_A11Y_TREE_QUERY",...}
  -> inject.js: buildA11yTree() traverses DOM, assigns UIDs, formats text
  -> inject.js -> content.js -> background.js -> POST /a11y-result
  -> Go server returns result to AI
```

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should UIDs persist across page navigations (stored in extension memory)? | open | Pros: AI can reference elements across page loads. Cons: DOM changes invalidate UIDs, causing confusion. Current design: UIDs regenerated per snapshot (stateless). |
| OI-2 | Should we add `interact {action: "click_uid", uid: N}` as a convenience? | open | Currently the AI must use `uidMap` to get a selector and then call `execute_js`. A direct `click_uid` action would reduce round-trips but adds complexity. Could be a follow-up feature. |
| OI-3 | Should `a11y_tree` results be cached like `accessibility` audit results? | open | Audit results are expensive (axe-core is slow). Tree traversal is fast (~100ms). Caching may not be needed, but could help if AI requests multiple snapshots rapidly. Leaning toward no cache initially. |
| OI-4 | How should the feature handle Custom Elements / Web Components? | open | Custom elements may not have implicit ARIA roles. Proposal: treat as `generic` role unless explicit `role` attribute is set. Traverse into shadow DOM as a follow-up feature. |

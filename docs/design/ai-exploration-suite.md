# AI Exploration Suite — Design Document

> Compound actions, response enrichments, and composable params that reduce MCP round-trips for AI agents exploring and interacting with web pages.

## 1. Motivation

AI coding agents using Kaboom's MCP tools follow a predictable pattern when working with a web page:

1. Navigate to a URL
2. Understand what is on the page (structure, interactive elements, content)
3. Interact with elements (click, type, select)
4. Observe the effects of those interactions (DOM mutations, new errors, visual changes)
5. Repeat

Each of these steps currently requires multiple tool calls. Understanding a page means calling `observe({what: "page"})`, then `interact({what: "list_interactive"})`, then `interact({what: "get_readable"})`, then `analyze({what: "navigation"})`, then `analyze({what: "page_structure"})`, then `observe({what: "screenshot"})`. That is six round-trips before the agent has even clicked anything.

The AI Exploration Suite reduces this to a single call: `interact({what: "explore_page"})`. It also adds composable enrichment params to individual actions (like `click` and `navigate`) so agents get richer feedback without extra calls.

## 2. Features

### 2.1 `explore_page` — Compound page exploration

A single `interact` action that returns:
- **Page metadata**: URL, title, tab status, favicon, viewport dimensions
- **Interactive elements**: All clickable/typeable elements (same as `list_interactive`)
- **Readable content**: Extracted text content, title, byline, word count (same as `get_readable`)
- **Navigation links**: Discovered links grouped by page region (same as `analyze({what: "navigation"})`)
- **Screenshot**: Inline base64 image content block appended server-side

Optional params: `url` (navigate first), `visible_only`, `limit`.

### 2.2 `observe_mutations` — DOM mutation tracking on actions

Boolean param composable with `click`, `type`, `select`, and other DOM-mutating actions. When `true`, the extension sets up a MutationObserver before executing the action and reports element-level DOM changes in the response.

### 2.3 `include_screenshot` — Inline screenshot after any action

Boolean param composable with any `interact` action. When `true`, a screenshot is captured after the action completes and returned as an inline image content block in the response.

### 2.4 `evidence` — Visual evidence capture mode

String enum (`off`, `on_mutation`, `always`) composable with mutating actions. Captures before/after screenshots or screenshots on detected DOM mutation.

### 2.5 `wait_for_stable` — DOM stability gate

Boolean param composable with `navigate`, `click`, and other actions. Waits for DOM mutations to quiesce (configurable via `stability_ms`, default 500ms) before returning.

### 2.6 `analyze({what: "page_structure"})` — Structural page analysis

Returns framework detection (React, Vue, Next.js, Nuxt, Angular, Svelte), routing type, scroll containers, modal/dialog state, shadow DOM count, and meta tags.

### 2.7 `observe({what: "page_inventory"})` — Combined page + interactive elements

Returns page info and interactive elements in a single call, replacing the need to call `observe({what: "page"})` + `interact({what: "list_interactive"})` separately.

## 3. Response Structures

### 3.1 `explore_page` response

```json
{
  "url": "https://example.com",
  "title": "Example Page",
  "tab_status": "complete",
  "favicon": "https://example.com/favicon.ico",
  "viewport": { "width": 1920, "height": 1080 },
  "interactive_elements": [
    { "tag": "button", "text": "Submit", "selector": "#submit-btn", "visible": true, "element_id": "e_1" }
  ],
  "interactive_count": 42,
  "readable": {
    "title": "Example Page",
    "content": "Main article text...",
    "excerpt": "Main article text (first 300 chars)...",
    "byline": "Author Name",
    "word_count": 1250,
    "url": "https://example.com"
  },
  "navigation": {
    "regions": [
      { "tag": "nav", "role": "navigation", "label": "Main menu", "position": "top", "links": [...] }
    ],
    "unregioned_links": [...],
    "summary": { "total_regions": 3, "total_links": 45, "internal_links": 38, "external_links": 7 }
  }
}
```

Plus an inline image content block (screenshot) appended as a second MCP content block.

### 3.2 `page_structure` response

```json
{
  "frameworks": [
    { "name": "React", "version": "", "evidence": "data-reactroot" },
    { "name": "Next.js", "version": "", "evidence": "window.__NEXT_DATA__" }
  ],
  "routing": { "type": "next", "evidence": "__NEXT_DATA__" },
  "scroll_containers": [
    { "selector": "div#main-content", "scroll_height": 4500, "client_height": 800 }
  ],
  "modals": [
    { "selector": "dialog#cookie-consent", "visible": true, "type": "dialog" }
  ],
  "shadow_roots": 2,
  "meta": {
    "viewport": "width=device-width, initial-scale=1",
    "charset": "utf-8",
    "og_title": "Example Page",
    "description": "A description of the page"
  }
}
```

## 4. Composable Enrichment Params

These params can be added to individual `interact` actions:

| Param | Type | Actions | Default | Effect |
|-------|------|---------|---------|--------|
| `include_screenshot` | boolean | Any | `false` | Append screenshot image block to response |
| `observe_mutations` | boolean | click, type, select, check | `false` | Track and report DOM mutations during action |
| `evidence` | string enum | click, type, select, check | `"off"` | Capture before/after screenshots |
| `wait_for_stable` | boolean | navigate, click | `false` | Wait for DOM quiet before returning |
| `stability_ms` | number | (with wait_for_stable) | 500 | Duration of quiet time required |
| `analyze` | boolean | navigate, refresh, click | `false` | Include perf_diff in response |
| `auto_dismiss` | boolean | navigate | `false` | Dismiss cookie banners after load |

## 5. Tool Descriptions and Discoverability

The `interact` tool description currently reads:

> "Browser actions. Requires AI Web Pilot. [...] Selectors: CSS or semantic [...]"

The `explore_page` action is not mentioned in the tool description. Agents must discover it through the `what` enum or `describe_capabilities`.

## 6. Current Default Values

All enrichment params default to `false` or `"off"`. This means:
- `explore_page` always captures a screenshot (hardcoded in server-side handler)
- `click` does NOT track mutations by default
- `navigate` does NOT wait for stability by default
- `navigate` does NOT auto-dismiss overlays by default
- No action includes a screenshot by default (except `explore_page`)

## 7. Error Handling

When enrichments partially fail in `explore_page`:
- Interactive element extraction failure: `interactive_error` field added, `interactive_elements` is empty array
- Readable content extraction failure: `readable` contains `{ "error": "extraction_failed" }`
- Navigation extraction failure: `navigation` contains `{ "error": "extraction_failed" }`
- Screenshot capture failure: response is returned without image content block (silent omission)

---

## 8. UX Review -- Principal UX Engineer

### Reviewer context

This review evaluates the AI Exploration Suite from the perspective of an AI agent that will call these tools thousands of times. I have read the three schema files (`interact.go`, `observe.go`, `analyze.go`), the `explore_page` server and extension handlers, the `page_structure` analysis handler, the `page_inventory` handler, the `mode_specs.go` capability declarations, the auto-paste-screenshots spec, and the PR preview exploration specs. My focus is on reducing agent cognitive load, improving defaults, tightening response contracts, and making features discoverable.

---

### 8.1 API Surface Consistency

**Finding 1: The dispatch param is inconsistent across tools.**

- `interact` uses `what` (with deprecated `action` alias)
- `observe` uses `what`
- `analyze` uses `what`
- `configure` uses `action`
- `generate` uses... the spec says `format`, the mode_specs says modes are keyed by name

This is a historical artifact, but it means an agent must remember which dispatch key to use for which tool. `what` is used by three out of five tools, but `configure` uses `action` and `generate` uses `format`. This cannot be changed without breaking clients, but it should be explicitly documented in the tool descriptions so agents do not guess wrong.

**Recommendation**: Add a one-line parenthetical to each tool description: `Dispatch key: 'what'` / `Dispatch key: 'action'` / `Dispatch key: 'format'`. This costs almost nothing and prevents a common agent error.

**Finding 2: `explore_page` is in the `interact` tool, but `page_inventory` is in `observe`, and `page_structure` is in `analyze`.**

These three features serve the same purpose -- understanding a page -- but are spread across three tools. An agent that wants a complete page picture must know that:
- `interact({what: "explore_page"})` gives screenshot + elements + readable text + navigation
- `observe({what: "page_inventory"})` gives page info + elements (no readable text, no navigation, no screenshot)
- `analyze({what: "page_structure"})` gives framework detection + routing + modals + scroll containers

There is no single call that gives everything. `explore_page` comes closest but omits `page_structure` data (frameworks, modals, scroll containers).

**Recommendation**: Add an optional `include_structure` boolean param to `explore_page` that, when true, also runs the `page_structure` analysis and includes it in the response under a `structure` key. This turns `explore_page` into the definitive one-call page understanding tool. The param should default to `false` to preserve current payload size, but agents doing first-time page exploration will almost always want it.

Alternatively, consider adding a `depth` param with values `"quick"` (current behavior) and `"full"` (adds page_structure + any future enrichments). This is more extensible than adding individual boolean toggles for each enrichment.

**Finding 3: Naming inconsistency between `include_screenshot` and `include_content`.**

`include_screenshot` is a composable boolean on any interact action. `include_content` is a boolean on `navigate` that returns page content with the navigation response. Both follow the `include_*` pattern, which is good. But `include_url` (on `save_state`) means something completely different -- it means "also save the URL when saving state," not "include the URL in the response."

**Recommendation**: Rename `include_url` on `save_state` to `save_url` or `restore_url` to avoid confusion with the `include_*` response enrichment pattern. If rename is not feasible due to backward compatibility, at least document the distinction in the tool description.

**Finding 4: `observe_mutations` vs `evidence` overlap.**

`observe_mutations` tracks DOM mutations as structured data. `evidence` captures before/after screenshots. Both are about understanding what changed, but they are separate params with different types (boolean vs string enum). An agent that wants full mutation tracking must set both `observe_mutations: true` and `evidence: "always"`.

**Recommendation**: Keep them separate (they serve different purposes), but add a note in the `click` mode spec hint that says: "Pair with observe_mutations for DOM diff and/or evidence for visual before/after." This teaches agents that they are complementary, not alternatives.

---

### 8.2 Default Behavior

**Finding 5: `include_screenshot` should default to `true` for `explore_page` (and it already does, but not via the param).**

The current implementation hardcodes screenshot capture in the Go handler (`appendScreenshotToResponse`), bypassing the `include_screenshot` param entirely. This means `explore_page` always returns a screenshot regardless of the `include_screenshot` param value. This is the right behavior, but it is implemented as a special case rather than a default value.

**Recommendation**: Instead of hardcoding the screenshot in the `explore_page` handler, set `include_screenshot` to default to `true` specifically when `what: "explore_page"`. This makes the behavior explicit in the schema and allows agents to opt out with `include_screenshot: false` if they want a text-only response (useful for token-constrained contexts). Add this to the `explore_page` mode spec hint.

**Finding 6: `observe_mutations` should default to `true` for `click`.**

When an AI agent clicks a button, it almost always wants to know what changed. The entire reason for clicking is to trigger a state change. Currently, the agent must remember to add `observe_mutations: true` to every click call. Almost no agent will want to click a button and NOT know what changed.

**Recommendation**: Default `observe_mutations` to `true` for `click` and `type` actions. Agents can opt out with `observe_mutations: false` for performance-sensitive bulk operations. This is a breaking change in response shape, so it should be gated behind a version flag or announced in a release note.

**Revised recommendation (non-breaking)**: Instead of changing the default, add a new mode spec hint for `click` that says: "Tip: Add observe_mutations:true to see what DOM nodes changed after the click." This teaches agents the pattern without breaking existing clients. Then, in the next major version, change the default.

**Finding 7: `wait_for_stable` should default to `true` for `navigate`.**

When an agent navigates to a URL, it wants to interact with the loaded page. If the DOM is still mutating (SPAs, lazy loading, skeleton screens), subsequent `list_interactive` or `get_readable` calls return incomplete data. The agent must either guess a timeout or explicitly add `wait_for_stable: true`.

**Recommendation**: Default `wait_for_stable` to `true` for `navigate` with a sensible `stability_ms` default (the current 500ms is good). Agents doing rapid navigation (crawling) can opt out with `wait_for_stable: false`.

**Finding 8: `auto_dismiss` should default to `true` for `explore_page`.**

When exploring a page for the first time, cookie consent banners and overlays obscure interactive elements and readable content. The agent must currently remember to add `auto_dismiss: true` or call `interact({what: "auto_dismiss_overlays"})` separately.

**Recommendation**: Default `auto_dismiss` to `true` when `what: "explore_page"`. The whole point of explore_page is first-encounter page understanding, and cookie banners are the primary obstacle to that. Add `auto_dismiss` to the `explore_page` optional params in mode_specs and default it in the handler.

---

### 8.3 Response Structure

**Finding 9: Partial failure is communicated inconsistently in `explore_page`.**

Interactive extraction failure uses `interactive_error` (a top-level string field alongside the empty array). Readable extraction failure uses `{ "error": "extraction_failed" }` inside the `readable` object. Navigation extraction failure uses the same pattern inside `navigation`. Screenshot failure is silent (no image block appended, no error field).

An agent parsing this response must check three different error patterns:
1. Top-level `interactive_error` string field exists?
2. `readable.error` string field exists?
3. `navigation.error` string field exists?
4. Was there a second content block of type `image`? (If not, screenshot failed -- but how would you know?)

**Recommendation**: Standardize on a single error reporting pattern. Add an `_errors` array to the response that lists all partial failures:

```json
{
  "url": "https://example.com",
  "title": "Example Page",
  "interactive_elements": [],
  "readable": null,
  "navigation": null,
  "_errors": [
    { "component": "interactive", "error": "Script injection blocked by CSP" },
    { "component": "readable", "error": "extraction_failed" },
    { "component": "screenshot", "error": "no tracked tab" }
  ]
}
```

This gives agents a single field to check for partial failures. If `_errors` is absent or empty, everything succeeded. The underscore prefix signals it is metadata, not page data.

**Finding 10: `interactive_elements` items lack a consistent shape contract.**

The `explore_page` response includes `interactive_elements` which come from `domPrimitiveListInteractive`. The shape of each element is not documented in any schema -- it comes from whatever the extension returns. Agents must infer the fields from the data.

**Recommendation**: Document the interactive element shape in this design doc and ensure it is stable:

```json
{
  "tag": "string",
  "type": "string | null",
  "text": "string",
  "selector": "string",
  "element_id": "string",
  "visible": "boolean",
  "role": "string | null",
  "aria_label": "string | null",
  "href": "string | null",
  "name": "string | null",
  "placeholder": "string | null",
  "disabled": "boolean",
  "frame_index": "number | null"
}
```

This is critical for agent reliability. If the shape changes between versions, agents that rely on `element_id` for follow-up actions will break silently.

**Finding 11: `explore_page` nesting depth is appropriate but `navigation.regions[].links[]` is deep.**

The response is three levels deep for navigation: `navigation.regions[0].links[0].href`. This is fine for structured data, but agents sometimes have trouble with deeply nested JSON when deciding what to click next.

**Recommendation**: No structural change needed, but add a `navigation.summary.sample_links` field that includes the 5 most prominent internal links (first link from each of the top 5 regions, or the first 5 unregioned links if no regions). This gives agents a quick "what can I click next?" answer without parsing the full region tree.

```json
"navigation": {
  "regions": [...],
  "unregioned_links": [...],
  "summary": {
    "total_regions": 3,
    "total_links": 45,
    "internal_links": 38,
    "external_links": 7,
    "top_links": [
      { "text": "Products", "href": "/products" },
      { "text": "Pricing", "href": "/pricing" },
      { "text": "About", "href": "/about" },
      { "text": "Contact", "href": "/contact" },
      { "text": "Blog", "href": "/blog" }
    ]
  }
}
```

---

### 8.4 Discoverability

**Finding 12: `explore_page` is not mentioned in the `interact` tool description.**

The tool description focuses on sync mode, selectors, draw mode, and the `what` vs `action` deprecation. An agent reading the description has no idea that `explore_page` exists. The `what` enum lists 30+ actions, and `explore_page` is buried near the end.

**Recommendation**: Add a line to the `interact` tool description: "Start with explore_page for a complete page snapshot (screenshot, elements, text, navigation) in one call." This is the single highest-impact discoverability improvement possible. Agents reading the tool description will immediately know to use `explore_page` as their first action on any new page.

Proposed updated description opening:

```
Browser actions. Requires AI Web Pilot.

Start with explore_page for a complete page snapshot (screenshot, interactive elements, readable text, navigation links) in one call. Use list_interactive for element discovery and click/type/select for interaction.

Synchronous Mode (Default): Tools block until the extension returns (up to 15s). Set background:true to return immediately.
[rest unchanged]
```

**Finding 13: `describe_capabilities` is a great discoverability mechanism but agents may not know to call it.**

The `configure({what: "describe_capabilities"})` action returns per-mode param specs, which is exactly what agents need. But agents must know it exists in the first place.

**Recommendation**: Add "Call configure({what:'describe_capabilities'}) for per-action param details." to the end of the `interact` tool description. This creates a breadcrumb trail: agent reads tool description, learns about `explore_page`, and learns how to discover params for every action.

**Finding 14: `page_inventory` and `explore_page` overlap is confusing.**

An agent sees both `page_inventory` (in `observe`) and `explore_page` (in `interact`). Both return page info + interactive elements. The distinction is subtle:
- `page_inventory` = page info + elements (no screenshot, no readable text, no navigation)
- `explore_page` = page info + elements + readable text + navigation + screenshot

Without reading source code, an agent cannot tell which to use.

**Recommendation**: Update the `page_inventory` mode spec hint to say: "Combined page info + interactive elements. For full page exploration including readable text, navigation links, and screenshot, use interact({what:'explore_page'}) instead." This explicitly directs agents to the richer option.

---

### 8.5 Cognitive Load

**Finding 15: The `interact` tool has 30+ actions and 50+ params. This is a high-entropy API.**

When an LLM is deciding what to call, it must consider every param in the schema. The `interact` tool has params for screenshots, recordings, forms, storage, cookies, states, batch execution, DOM actions, navigation, and page exploration. Most params are irrelevant to most actions.

This is a known issue with flat-enum dispatch (all actions share one param namespace). The `mode_specs` / `describe_capabilities` system mitigates this server-side, but the schema sent to the LLM via MCP `tools/list` still includes every param.

**Recommendation (short-term)**: Group the tool description into logical sections with headers. Many MCP clients render tool descriptions as markdown:

```
Browser actions. Requires AI Web Pilot.

**Page understanding:** explore_page (full snapshot), list_interactive, get_readable, get_markdown
**Interaction:** click, type, select, check, hover, focus, scroll_to, key_press, paste
**Navigation:** navigate, back, forward, refresh, new_tab, switch_tab, close_tab
**Forms:** fill_form, fill_form_and_submit
**State:** save_state, load_state, list_states, delete_state
**Advanced:** execute_js, batch, upload, draw_mode_start, screenshot

Dispatch key: 'what'. Start with explore_page for first-time page understanding.
```

**Recommendation (medium-term)**: Consider a `preset` or `mode` meta-param that bundles common enrichment combinations:

```json
// Instead of:
{ "what": "click", "selector": "#btn", "observe_mutations": true, "include_screenshot": true, "wait_for_stable": true }

// Allow:
{ "what": "click", "selector": "#btn", "enrich": "full" }
```

Where `enrich` is an enum:
- `"none"` (default): no enrichments, fastest response
- `"mutations"`: `observe_mutations: true`
- `"visual"`: `include_screenshot: true`
- `"full"`: `observe_mutations: true, include_screenshot: true, wait_for_stable: true`

This reduces the number of params an agent must reason about from 3+ booleans to 1 enum. The individual params still work for fine-grained control.

**Finding 16: Too many ways to target an element.**

An agent can target an element for `click` using:
- `selector` (CSS or semantic)
- `element_id` (from `list_interactive`)
- `index` + `index_generation` (legacy)
- `scope_selector` (container constraint, composable with any of the above)

This is four targeting mechanisms. The mode spec for `click` lists all four as optional params. An agent must decide which to use.

**Recommendation**: The mode spec hint for `click` should include a priority order: "Target elements with element_id (most reliable), selector (most flexible), or index (legacy). Use scope_selector to constrain to a region." This eliminates decision paralysis.

---

### 8.6 Error States and Partial Failures

**Finding 17: Screenshot failure in `explore_page` is silent.**

If `appendScreenshotToResponse` fails (no tracked tab, extension disconnected, capture timeout), the response simply has no image content block. The text content block is complete and correct. But the agent has no way to know whether a screenshot was attempted and failed, or was never attempted.

This matters because agents using `explore_page` may rely on the screenshot for visual understanding. If it silently fails, the agent proceeds with incomplete information and makes worse decisions.

**Recommendation**: When screenshot capture fails in `explore_page`, add a field to the text response:

```json
{
  "screenshot_status": "failed",
  "screenshot_error": "no tracked tab"
}
```

When it succeeds:

```json
{
  "screenshot_status": "captured"
}
```

This costs nothing in the success path (one small field) and prevents agents from wondering where their screenshot went.

**Finding 18: Enrichment timeout handling is not specified.**

If `observe_mutations` is true on a click, what happens if the MutationObserver setup times out? If `wait_for_stable` is true but the page never stops mutating (infinite polling, animation loops), what does the agent see?

**Recommendation**: Document the timeout behavior for every enrichment:
- `observe_mutations`: If the action itself succeeds but mutation collection times out, return the action result plus `"mutations": { "status": "timed_out", "partial_mutations": [...] }`.
- `wait_for_stable`: If stability is not reached within `timeout_ms` (default 5000ms), return `"stability": { "status": "timed_out", "final_mutation_count": N }` instead of blocking forever.
- `include_screenshot`: If capture fails, return the action result plus `"screenshot_status": "failed"` in the text response. Never silently omit.
- `evidence`: If before-screenshot succeeds but after-screenshot fails, return `"evidence": { "before": "captured", "after": "failed" }`.

The principle: **never silently omit data the agent requested**. Always return a status field so the agent knows what happened.

---

### 8.7 Specific Recommendations Summary

| # | Change | Impact | Breaking? |
|---|--------|--------|-----------|
| R1 | Add `include_structure` boolean to `explore_page` | Agents get framework/modal/routing info without a second call | No |
| R2 | Default `include_screenshot` to `true` for `explore_page` via param, not hardcode | Agents can opt out; behavior is explicit | No |
| R3 | Default `auto_dismiss` to `true` for `explore_page` | Cookie banners are the #1 obstacle to page understanding | No |
| R4 | Add `explore_page` callout to `interact` tool description | Single biggest discoverability win | No |
| R5 | Add `describe_capabilities` breadcrumb to tool descriptions | Agents learn how to self-discover params | No |
| R6 | Standardize partial failure reporting with `_errors` array | Agents check one field instead of three patterns | No (additive) |
| R7 | Add `screenshot_status` field to `explore_page` response | Agents know if screenshot was captured or failed | No (additive) |
| R8 | Add `top_links` to navigation summary | Quick "what to click next" without parsing regions | No (additive) |
| R9 | Add `enrich` preset enum to `interact` actions | Reduces boolean param sprawl for common combinations | No (additive) |
| R10 | Update `page_inventory` hint to direct agents to `explore_page` | Reduces confusion between overlapping features | No |
| R11 | Add element targeting priority to `click` mode spec hint | Eliminates decision paralysis on targeting mechanism | No |
| R12 | Document interactive element shape contract | Prevents silent breakage on schema changes | No |
| R13 | Add per-enrichment timeout/failure status fields | Never silently omit requested data | No (additive) |
| R14 | Default `observe_mutations` to `true` for `click` (next major) | Agents almost always want mutation feedback from clicks | Yes (major version) |
| R15 | Default `wait_for_stable` to `true` for `navigate` (next major) | Prevents agents from reading incomplete DOM after navigation | Yes (major version) |
| R16 | Group `interact` tool description into logical sections | Reduces cognitive load on 30+ action enum | No |

### 8.8 Priority Order

**Do now (zero-risk, high-impact):**
1. R4 -- Add `explore_page` callout to interact description
2. R5 -- Add `describe_capabilities` breadcrumb to descriptions
3. R10 -- Update `page_inventory` hint to point to `explore_page`
4. R11 -- Add element targeting priority to `click` hint
5. R16 -- Group interact description into logical sections

**Do next (additive, no breaking changes):**
6. R3 -- Default `auto_dismiss` to `true` for `explore_page`
7. R6 -- Standardize `_errors` array in `explore_page` response
8. R7 -- Add `screenshot_status` to `explore_page` response
9. R8 -- Add `top_links` to navigation summary
10. R1 -- Add `include_structure` param to `explore_page`
11. R12 -- Document interactive element shape contract
12. R13 -- Per-enrichment timeout/failure status fields

**Plan for next major version:**
13. R14 -- Default `observe_mutations` to `true` for `click`
14. R15 -- Default `wait_for_stable` to `true` for `navigate`
15. R9 -- `enrich` preset enum (needs design iteration)
16. R2 -- Refactor `explore_page` screenshot from hardcode to default param

### 8.9 Proposed `interact` Tool Description

```
Browser actions. Requires AI Web Pilot. Dispatch key: 'what'.

**Getting started:** Use explore_page for a complete page snapshot (screenshot,
interactive elements, readable text, navigation links) in one call. Use
list_interactive for element discovery. Use click/type/select for interaction.

**Element targeting:** Prefer element_id (from list_interactive/explore_page) for
reliability, selector for flexibility, or index (legacy). Add scope_selector to
constrain to a page region.

**Enrichments:** Add include_screenshot:true for visual feedback, observe_mutations:true
for DOM change tracking, wait_for_stable:true to wait for DOM to settle.

Synchronous Mode (Default): Tools block until result (up to 15s). Set background:true
to return immediately with a correlation_id.

Selectors: CSS or semantic (text=Submit, role=button, placeholder=Email, label=Name,
aria-label=Close).

Call configure({what:'describe_capabilities', tool:'interact', mode:'click'}) for
per-action param details.
```

### 8.10 Proposed `explore_page` Mode Spec Update

Current hint:
```
"Composite page exploration: screenshot, interactive elements, readable text, navigation links, and metadata in one call"
```

Proposed hint:
```
"Complete page snapshot in one call: screenshot, interactive elements, readable text, navigation links, metadata. Best starting point for first-time page understanding. Auto-dismisses overlays. Add include_structure:true for framework/modal/routing detection."
```

Proposed optional params:
```go
Optional: []string{"url", "visible_only", "limit", "include_structure", "auto_dismiss", "include_screenshot"},
```

This makes `auto_dismiss` and `include_screenshot` explicit (with defaults of `true` for `explore_page`), and adds `include_structure` as an opt-in enrichment.

## 9. Engineering Review -- Principal Engineer

**Reviewer**: Principal Engineer
**Date**: 2026-02-27
**Scope**: Architecture, performance, concurrency, response sizing, backward compatibility, implementation gaps, test coverage

All file paths are relative to the repository root unless noted.

---

### 9.1 Architecture Assessment

**Verdict: Sound layering with two structural concerns.**

The three-layer architecture (MCP schema -> Go server handlers -> Extension command handlers) is clean and well-separated. The command registry pattern in `src/background/commands/registry.ts` with `registerCommand` / `dispatch` is a good replacement for a monolithic if-chain. The Go-side dispatch via `interactDispatch()` map initialized with `sync.Once` (`cmd/browser-agent/tools_interact.go`, line 30) is correct and thread-safe.

**Concern 1: Duplicated `navigationDiscoveryScript` across two files.**

The function `navigationDiscoveryScript` is copy-pasted between:
- `src/background/commands/analyze-navigation.ts` (lines 13-181)
- `src/background/commands/interact-explore.ts` (lines 83-198)

The comment on line 81 of interact-explore.ts acknowledges this: _"Mirrors analyze-navigation.ts navigationDiscoveryScript but as a separate reference for explore_page."_ The two copies have **already diverged**: the `analyze-navigation.ts` version includes `url` and `title` in its return object (lines 170-171), while the `interact-explore.ts` version does not. This means `analyze({what: "navigation"})` returns `url` and `title` at the top level, but the `navigation` sub-object inside `explore_page` does not. This inconsistency will confuse agents that switch between the two APIs.

This is a **divergence bug** that will get worse over time. The justification for duplication is the Chrome `executeScript` constraint: functions must be self-contained (no closures). However, this constraint applies only to the serialization boundary, not to imports.

**Recommended fix**: Extract the canonical `navigationDiscoveryScript` into a shared module (e.g., `src/background/scripts/navigation-discovery.ts`) and import it into both command handlers. `chrome.scripting.executeScript` serializes the function reference at call time by inspecting its `.toString()` output. A named function import from another module works correctly here because esbuild or the native ES module system resolves the import before Chrome serializes. The background service worker uses `"type": "module"` (`extension/manifest.json`, line 22), and command modules are loaded as side-effect imports from the service worker entry point. The function's self-containment requirement (no closure over module-scope variables) is already satisfied by the current implementations.

The same duplication concern applies to `readableContentScript` in `interact-explore.ts` (lines 17-73), which parallels the Go-side `getReadableScript` logic. While the Go and TypeScript versions serve different code paths, the TS version should be extracted to a shared module if any other extension command needs readable content in the future.

**Concern 2: No circular dependencies detected.** The import graph is:
```
registry.ts <-- interact-explore.ts, analyze-*.ts (registerCommand)
dom-primitives-list-interactive.ts <-- interact-explore.ts (domPrimitiveListInteractive)
helpers.ts <-- registry.ts (utility functions)
state.js <-- registry.ts (initReady)
```
This is a clean DAG. No issues.

---

### 9.2 Performance Analysis

**Verdict: `explore_page` latency is acceptable. Batch with per-step screenshots will be problematic.**

#### 9.2.1 explore_page Latency Breakdown

| Operation | Estimated Time | Serial/Parallel | Notes |
|-----------|---------------|-----------------|-------|
| `chrome.tabs.update` + load wait | 0-15000ms | Serial | Only if `url` provided; typical page: 1-3s |
| `chrome.tabs.get` | ~5ms | Serial | Tab metadata lookup |
| `Promise.all` (3 scripts) | 100-500ms | **Parallel** | Wall-clock = max of the three |
| -- `listInteractive` (MAIN, allFrames) | 50-300ms | | Scans DOM across all frames |
| -- `readableContent` (ISOLATED) | 20-100ms | | Text extraction from main frame |
| -- `navigationDiscovery` (ISOLATED) | 30-150ms | | Link scanning and region grouping |
| Server-side `appendScreenshotToResponse` | 500-2000ms | Serial | Screenshot capture + base64 encode |
| **Total (no navigation)** | **~600-2500ms** | | |
| **Total (with navigation)** | **~1600-5500ms** | | |

These are comfortably within the 15s `MaybeWaitForCommand` budget (`cmd/browser-agent/tools_async.go`, line 141: `asyncInitialWait` default). Acceptable.

**Adding `page_structure` to `explore_page`** (the `include_structure` param proposed in Section 8.1 Finding 2) would add `pageStructureScript` as a fourth parallel script. Estimated overhead:

| Additional operation | Estimated Time |
|---------------------|---------------|
| `pageStructureScript` (MAIN with ISOLATED fallback) | 50-200ms |
| CSP fallback penalty (if MAIN world blocked) | +100-300ms |

Since it runs in parallel, wall-clock impact is **0-200ms** (only matters if it becomes the bottleneck). Negligible. **Recommendation: approve `include_structure` on performance grounds.**

**Adding `wait_for_stable` as a default** would insert a serial 500-5000ms wait between navigation completion and data collection. For `explore_page` with `url` provided, this is tolerable (the page load itself dominates). For `explore_page` on the current page (no navigation), this doubles latency from ~600ms to ~1100ms. **Recommendation: make `wait_for_stable` opt-in for `explore_page`, not default. If UX review (Section 8.2, Finding 7) pushes for a default, use a reduced 200ms internal stability check that is separate from the user-configurable `wait_for_stable` composable param.**

**Adding `action_diff` to `explore_page`** makes no architectural sense. `action_diff` captures mutations caused by a preceding action. `explore_page` is an observation operation, not a mutation. Do not add it.

#### 9.2.2 Batch with Per-Step Screenshots

The batch handler (`cmd/browser-agent/tools_interact_batch.go`) executes steps sequentially through `h.toolInteract(req, replayStepArgs)` (line 88). If a step includes `include_screenshot=true`, the `toolInteract` dispatcher (line 205 of `tools_interact.go`) calls `appendScreenshotToResponse`, which calls the page observation owner with a 20-second timeout (`internal/tools/observe/page/page_state.go`). Each screenshot involves:

1. `CreatePendingQueryWithTimeout` (screenshot query)
2. `WaitForResult` up to 20s
3. Extension captures tab, base64 encodes
4. Result returned and parsed

Per-step cost: **500-2000ms for the screenshot alone**, on top of the step action time.

For a 10-step batch with screenshots: **5-20s of screenshot overhead alone**. Combined with step action time (200-1000ms each), total is **7-30s**. The batch's own `step_timeout_ms` is 10s per step (line 50: `defaultStepTimeout = 10000`), and the caller's `MaybeWaitForCommand` timeout is 15s default. **A 10-step batch with screenshots will frequently timeout.**

**Critical implementation issue** (`tools_interact_batch.go`, lines 87-96): The batch handler calls `h.toolInteract()` which returns a `JSONRPCResponse` for each step. When `include_screenshot=true`, the screenshot image block is appended to this per-step response. But the batch handler only extracts `status`, `error`, and `correlation_id` from the step response -- it does **not** extract or preserve the image content blocks. The screenshot data is captured, encoded, appended, and then **silently discarded** when the batch constructs its aggregate response (lines 170-181).

This is wasted work. The extension captures the screenshot, base64 encodes it (CPU-intensive), sends it to the server, the server parses it and appends it to a response object that is immediately thrown away.

**Recommendations:**
1. Add validation in `handleBatch` that rejects `include_screenshot` on batch steps, or strips it from step args before dispatch. This avoids wasted screenshot captures.
2. If per-step screenshots are a desired feature, extend `SequenceStepResult` to include an optional `screenshot_path` field, save screenshots to disk, and return paths. Do not try to return inline base64 images for multiple steps -- the aggregate response would be multi-megabyte.
3. Document that batch steps do not support `include_screenshot` (or `evidence`).

---

### 9.3 Concurrency and Race Conditions

**Verdict: Parallel script execution in `explore_page` is safe. Two edge cases merit attention.**

#### 9.3.1 Parallel Script Safety in explore_page

The three parallel `chrome.scripting.executeScript` calls in `interact-explore.ts` (lines 231-253) are:
1. `listInteractive` -- MAIN world, `allFrames: true`, reads DOM
2. `readableContent` -- ISOLATED world, main frame only, reads DOM
3. `navigationDiscovery` -- ISOLATED world, main frame only, reads DOM

All three are read-only. MAIN and ISOLATED worlds have separate JS contexts but share the same underlying DOM. Chrome does not guarantee atomicity across concurrent `executeScript` calls, so if the page mutates between script executions, each script may see slightly different DOM states. However, this is a theoretical concern -- in practice, the scripts execute within 10-50ms of each other, and DOM mutations during this window are unlikely to cause meaningful inconsistency.

**No race conditions in the current implementation.**

If `page_structure` is added as a fourth parallel script, the same analysis applies -- it is read-only and safe to run concurrently.

#### 9.3.2 Edge Case: Navigation Race in explore_page

When `url` is provided, the extension calls `chrome.tabs.update(ctx.tabId, { url: targetUrl })` then waits for `status === 'complete'` via a `tabs.onUpdated` listener (`interact-explore.ts`, lines 209-225). Race condition:

1. `tabs.update` fires navigation
2. Page loads, status becomes `'complete'` -> listener resolves
3. Page JavaScript executes a client-side redirect (e.g., OAuth flow, A/B test redirect)
4. Parallel scripts execute against the **mid-redirect** page

The 15s safety timeout (line 223) does not help because the initial load already triggered `'complete'`.

**Recommendation**: After the load listener resolves, add a brief delay (50-100ms) and re-check `chrome.tabs.get(ctx.tabId)` to confirm the URL hasn't changed. If it has, wait for the second navigation to complete. This adds 50-100ms latency in the common case (no redirect) but prevents data collection on a transient page.

#### 9.3.3 Batch Mutex

The `replayMu.TryLock()` in `tools_interact_batch.go` (line 54) correctly prevents concurrent batch/replay execution. The mutex variable `replayMu` is declared in `tools_configure_sequence.go` (line 18 area), shared across batch and replay handlers. The non-blocking `TryLock` with an error message is correct -- it prevents deadlock from nested calls (e.g., a batch step that itself calls batch).

**Minor issue**: The cross-file mutex dependency is implicit. Add a comment in both `tools_interact_batch.go` and `tools_configure_sequence.go`:
```go
// replayMu is shared between batch and replay_sequence handlers.
// Declared in tools_configure_sequence.go. Only one can execute at a time.
```

#### 9.3.4 Composable Side-Effect Queuing Order

When multiple composable params are set (`wait_for_stable=true, auto_dismiss=true, include_screenshot=true`), the Go handler queues them in this order (`tools_interact.go`, lines 194-206):
1. `auto_dismiss` (line 195) -- queued as async pending query
2. `wait_for_stable` (line 200) -- queued as async pending query
3. `include_screenshot` (line 206) -- executed synchronously (appendScreenshotToResponse)

The first two are queued as separate pending queries that the extension processes asynchronously. There is no guarantee that `auto_dismiss` completes before `wait_for_stable` starts. Both run on the extension's next poll cycle, but if the extension processes them in FIFO order (which `PendingQuery` queue ordering suggests), the sequence is correct: dismiss overlays first, then wait for stability.

However, `include_screenshot` runs synchronously in the Go handler, capturing the screenshot **before** the queued `auto_dismiss` and `wait_for_stable` have completed on the extension side. This means the screenshot may show the page **before** overlays are dismissed and **before** DOM stabilizes. This is almost certainly not what the caller wants.

**This is a bug.** When `include_screenshot` is combined with `auto_dismiss` or `wait_for_stable`, the screenshot should be captured **after** the queued operations complete, not before.

**Fix**: Move `appendScreenshotToResponse` to after waiting for all queued composable operations. This requires tracking the composable correlation IDs and waiting for them:

```go
// After queuing composable side-effects, wait for them before screenshot
if composableParams.AutoDismiss || composableParams.WaitForStable {
    // Give extension time to process queued operations
    time.Sleep(200 * time.Millisecond) // crude but effective
}
if composableParams.IncludeScreenshot && resp.Error == nil && !isResponseError(resp) {
    resp = h.appendScreenshotToResponse(resp, req)
}
```

A proper fix would wait for the composable correlation IDs, but that requires plumbing the IDs from `queueComposable*` back to the caller.

---

### 9.4 Response Size Analysis

**Verdict: `explore_page` is near the 100KB text limit on complex pages. `ClampResponseSize` has a critical JSON corruption bug.**

#### 9.4.1 explore_page Size Estimates

| Component | Typical Size | Worst Case | Notes |
|-----------|-------------|------------|-------|
| Page metadata | ~300B | ~500B | URL, title, viewport, favicon |
| Interactive elements (100 max) | 15-40KB | 50KB | ~150-400 bytes per element |
| Readable content | 5-15KB | 30KB | Full main content text |
| Navigation links | 5-20KB | 50KB | 20 regions * 50 links max |
| **Text total** | **25-75KB** | **130KB** | |
| Screenshot (image block) | 200-800KB | 1.5MB | Separate content block |

`MaxResponseBytes = 100_000` (`internal/mcp/response.go`, line 194) applies to the serialized `MCPToolResult` JSON, which includes the text content block(s) but **also** includes the image content block (base64 data is inside the same JSON structure). Let me verify this.

Looking at `appendScreenshotToResponse` (`tools_interact.go`, lines 278-306): it appends an `MCPContentBlock` with `Type: "image"` and `Data` containing the base64 string to the `MCPToolResult.Content` array. The full result (text + image blocks) is then serialized to `resp.Result` as `json.RawMessage`. This means the 100KB limit applies to the **entire result including the base64 screenshot**.

A typical JPEG screenshot at 75% quality is 50-200KB base64. Combined with 25-75KB of text content, the total is **75-275KB** -- regularly exceeding `MaxResponseBytes`.

**Wait -- does `ClampResponseSize` actually get called on explore_page responses?** Searching for `ClampResponseSize` usage:

The function is defined but let me verify it is actually applied to explore_page results. Looking at the response path: `handleExplorePage` -> `MaybeWaitForCommand` -> returns response -> `appendScreenshotToResponse` -> returned to `toolInteract` -> returned to caller. `ClampResponseSize` may be called at the transport layer.

**Regardless of whether it is called**: The function (`internal/mcp/response.go`, lines 198-233) truncates the **first text content block** at a byte boundary. If the first content block contains the explore_page JSON payload, truncation at an arbitrary byte position produces **malformed JSON**. For example:

```json
{"url":"https://example.com","interactive_elements":[{"tag":"button","text":"Sub
[truncated: original 150000 bytes, limit 100000 bytes. Use pagination...]
```

The AI agent receives unparsable JSON. This is not a theoretical risk -- it will happen on any moderately complex page with a screenshot.

**Severity: HIGH.**

**Recommendations:**
1. **Immediate fix**: `ClampResponseSize` should detect JSON content (check if text starts with `{` or `[`) and truncate at JSON-aware boundaries -- remove the last N array items from the largest array until the result fits, or replace the deepest nesting level with a `"...truncated"` placeholder.
2. **Architectural fix**: Move inline screenshots to a separate response mechanism that is not subject to text size limits. Options: (a) return screenshot as a file path with `mcp:// ` resource URI, (b) separate the image content block before applying `ClampResponseSize` and re-append after, (c) exempt image blocks from the byte count.
3. **Quick mitigation**: In `appendScreenshotToResponse`, check if adding the image block would push the total past `MaxResponseBytes`. If so, skip the inline screenshot and add a `"screenshot_status": "omitted_size_limit"` field to the text response.

#### 9.4.2 Adding page_structure

Adding `page_structure` data (~5KB) to `explore_page` is **not** the size concern. The concern is the existing text + screenshot combination already exceeding limits. Adding 5KB of structure data is noise compared to the 200KB screenshot. Fix the screenshot size issue first.

---

### 9.5 Extension Bundling Compatibility

**Verdict: Compatible with MV3. Build pipeline is sound.**

The manifest (`extension/manifest.json`) declares:
- Background: `service_worker: "background.js"` with `type: "module"` (lines 21-22)
- Content scripts: `early-patch.bundled.js` (MAIN world), `content.bundled.js` (ISOLATED world)

The build script (`scripts/build/bundle-content.js`) bundles content scripts (IIFE) and inject script (ESM) via esbuild. The background service worker is **not** bundled -- it uses native ES module imports. This is correct for Chromium 120+ targets.

Command handler modules (`interact-explore.ts`, `analyze-page-structure.ts`, etc.) register themselves via `registerCommand()` as import side effects. The background service worker must import all command modules for registration to occur. This works with native modules but would break if the service worker were tree-shaken (dead code elimination would remove "unused" imports that only have side effects).

**Recommendation**: If the build pipeline ever adds service worker bundling, use esbuild's `sideEffects: true` or explicitly import all command modules in the entry point with `import './commands/interact-explore.js'` (side-effect-only imports).

Functions passed to `chrome.scripting.executeScript` (`domPrimitiveListInteractive`, `readableContentScript`, `navigationDiscoveryScript`, `pageStructureScript`) are serialized by Chrome at call time using the function's source code. They must be self-contained (no closures over module-scope variables). All current implementations satisfy this constraint. The proposed shared module extraction (Section 9.1, Concern 1) is compatible because:
1. The shared module exports a named function
2. The importing module references the function by name
3. Chrome serializes the function source at call time
4. The function body is self-contained (no external references)

**No bundling concerns.**

---

### 9.6 Backward Compatibility

**Verdict: No breaking changes in current implementation.**

#### 9.6.1 explore_page is Additive

`explore_page` is a new `what` enum value added to the interact tool. Existing clients that do not use it are unaffected. The `what` enum is an open set (servers accept any value from the dispatch map; clients only see the enum for documentation).

#### 9.6.2 Composable Parameters are Opt-In

All composable enrichment params (`include_screenshot`, `auto_dismiss`, `wait_for_stable`, `action_diff`) default to `false`. Existing `interact` calls without these params behave identically.

#### 9.6.3 Schema Enum Changes Are Non-Breaking

Adding `explore_page` to `interactActions` (`internal/schema/interact/tool.go`, line 28) and `page_structure` / `navigation` to the analyze enum (`internal/schema/analyze.go`, line 19) are additive enum extensions. MCP clients that validate against the enum will accept new values. Clients that do not validate are unaffected.

#### 9.6.4 observe(what="screenshot") Is Unchanged

The UX review (Section 8.2, Finding 5) discusses making `include_screenshot` default to `true` for `explore_page` "via the param instead of hardcoding." If this change were implemented by altering the default in `GetScreenshot` (`internal/tools/observe/page/page_state.go`), it could affect `observe({what: "screenshot"})` callers. However, the proposed implementation only changes the `explore_page` handler, not `GetScreenshot`. **No breaking change.**

#### 9.6.5 The `action` Alias Is Preserved

Top-level tool dispatch uses only the canonical `what` parameter. Batch step
objects retain their own `action` field because that is the batch-step contract,
not a tool-selector alias.

---

### 9.7 Implementation Gaps

#### 9.7.1 `action_diff` Composable Parameter Is Not Wired Up

**Severity: HIGH -- documented feature does not work.**

The design document (Section 4, "Composable Enrichment Params" table, item for `evidence`) and the code in `tools_interact_composable.go` (function `queueComposableActionDiff`, lines 64-81) suggest that `action_diff=true` can be passed as a composable parameter on any interact action. The extension-side implementation exists (`dom-primitives.ts`, lines 2398-2638).

However, the composable parameter extraction in `toolInteract` (`tools_interact.go`, lines 176-183) does **not** include `ActionDiff`:

```go
var composableParams struct {
    Subtitle          *string `json:"subtitle"`
    IncludeScreenshot bool    `json:"include_screenshot"`
    AutoDismiss       bool    `json:"auto_dismiss"`
    WaitForStable     bool    `json:"wait_for_stable"`
    StabilityMs       int     `json:"stability_ms,omitempty"`
    // ActionDiff is MISSING
}
```

There is no code path that calls `queueComposableActionDiff`. The function exists but is never invoked. An AI agent setting `action_diff: true` on a `click` call will have the parameter silently ignored.

**Fix** (in `cmd/browser-agent/tools_interact.go`):

Add `ActionDiff bool \`json:"action_diff"\`` to the composable params struct (after line 182).

Add after line 201:
```go
if composableParams.ActionDiff && resp.Error == nil && !isResponseError(resp) {
    h.queueComposableActionDiff(req)
}
```

Also add `action_diff` to the interact schema (`internal/schema/interact/tool.go`) as a documented parameter:
```go
"action_diff": map[string]any{
    "type":        "boolean",
    "description": "After the action completes, capture a structured mutation summary (overlays, toasts, form errors, text changes, network requests)",
},
```

#### 9.7.2 No URL Validation on explore_page Navigation

**Severity: HIGH -- security concern.**

When `url` is provided to `explore_page`, the extension calls `chrome.tabs.update(ctx.tabId, { url: targetUrl })` (`interact-explore.ts`, line 209) with no validation. An AI agent (or malicious prompt injection) could pass:
- `javascript:alert(document.cookie)` -- executes arbitrary JS
- `data:text/html,<script>...</script>` -- loads arbitrary HTML
- `chrome://settings` -- navigates to internal pages
- `file:///etc/passwd` -- attempts local file access

Chrome blocks most of these for extensions, but `javascript:` URLs may work depending on the Chrome version and extension permissions. The extension has `<all_urls>` host permissions.

**Fix**: Add URL validation in the Go handler (`tools_interact_explore.go`) before creating the pending query:

```go
if params.URL != "" {
    parsed, err := url.Parse(params.URL)
    if err != nil || parsed.Scheme == "" {
        return JSONRPCResponse{...error: "Invalid URL"...}
    }
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return JSONRPCResponse{...error: "Only http/https URLs are allowed"...}
    }
}
```

Also add extension-side validation in `interact-explore.ts` before the `chrome.tabs.update` call:

```typescript
if (targetUrl && !targetUrl.match(/^https?:\/\//)) {
    throw new Error('Only http/https URLs are supported for explore_page navigation')
}
```

Note: The existing `navigate` action handler (`tools_interact.go`) **does** validate URLs via `url.Parse()` (visible in the dispatch chain). But `explore_page` bypasses this validation by forwarding raw params directly to the extension. The extension's `interact-explore.ts` does not call the `navigate` command handler -- it calls `chrome.tabs.update` directly.

#### 9.7.3 explore_page Does Not Include page_structure

The design document (Sections 2 and 8.1) discusses this at length. From an implementation perspective, adding `page_structure` as an optional fourth parallel script in `interact-explore.ts` is straightforward:

```typescript
const [interactiveResults, readableResults, navResults, structureResults] = await Promise.all([
    // ... existing three ...
    ctx.params.include_structure
        ? chrome.scripting.executeScript({
            target: { tabId: ctx.tabId },
            world: 'MAIN',
            func: pageStructureScript,
            args: [true]
          }).catch(() => chrome.scripting.executeScript({
            target: { tabId: ctx.tabId },
            world: 'ISOLATED',
            func: pageStructureScript,
            args: [false]
          })).catch(() => [])
        : Promise.resolve([])
])
```

But this requires importing `pageStructureScript` from `analyze-page-structure.ts`, which circles back to the code duplication concern in Section 9.1. The function should be extracted to a shared module first.

The Go handler needs to forward the `include_structure` param to the extension. Currently, `handleExplorePage` passes `args` directly as `PendingQuery.Params` (line 45 of `tools_interact_explore.go`). The extension reads `ctx.params.include_structure` from the parsed params. No Go-side changes needed beyond adding `include_structure` to the schema.

#### 9.7.4 Missing Error Propagation in explore_page Parallel Scripts

In `interact-explore.ts` (lines 231-253), each parallel script has `.catch(() => [])` which silently swallows errors. The subsequent processing code checks for `res?.success === false` (line 265) on interactive results and captures the first error (line 266), but only surfaces it if `finalElements.length === 0` (line 325).

This means:
- If `listInteractive` fails on frame 1 but succeeds on frame 0 with 3 elements, the failure is silently dropped.
- If `readableContent` throws (CSP blocks ISOLATED world script), `readable` becomes `{ error: "extraction_failed" }` -- but the root cause (CSP) is lost.
- If `navigationDiscovery` throws, same issue.

**Recommendation**: Capture error messages from `.catch()` blocks and include them in the response:

```typescript
const [interactiveResults, readableResults, navResults] = await Promise.all([
    chrome.scripting.executeScript({...}).catch((err) => {
        return [{ result: { success: false, error: err.message } }]
    }),
    // ... same pattern for others
])
```

Then include an `_errors` array in the response payload (aligning with UX Review Finding 9, R6):

```typescript
const errors: Array<{component: string, error: string}> = []
// ... check each result for errors and push to array
if (errors.length > 0) {
    payload._errors = errors
}
```

#### 9.7.5 Navigation Listener Leak on chrome.tabs.update Failure

In `interact-explore.ts` (lines 209-225), if `chrome.tabs.update(ctx.tabId, { url: targetUrl })` rejects (tab closed, invalid tab ID, internal error), the `await` on line 209 throws. The outer try/catch (line 334) handles it, but the `tabs.onUpdated` listener registered on line 218 is **not cleaned up** because the Promise constructor on lines 211-225 hasn't executed yet (the rejection happens before `await` on the navigation promise).

Wait -- actually, re-reading the code: `chrome.tabs.update` is `await`ed on line 209, and the navigation wait promise is constructed inside a separate `await new Promise` on lines 211-225. If `chrome.tabs.update` rejects, execution jumps to the catch block (line 334) without entering the Promise constructor. So the `onUpdated` listener is never registered in this failure case. **No listener leak.**

However, there is a different issue: if `chrome.tabs.update` resolves but the `tabs.onUpdated` event fires `'complete'` before the listener is registered (between lines 209 and 218), the listener will never fire and the 15s timeout is the only escape. This is a classic race between `tabs.update` and `tabs.onUpdated`.

**Recommendation**: Register the `onUpdated` listener **before** calling `chrome.tabs.update`:

```typescript
if (targetUrl) {
    await new Promise<void>((resolve) => {
        const timeout = setTimeout(() => {
            chrome.tabs.onUpdated.removeListener(onUpdated)
            resolve()
        }, 15000)
        const onUpdated = (tabId: number, changeInfo: { status?: string }): void => {
            if (tabId === ctx.tabId && changeInfo.status === 'complete') {
                chrome.tabs.onUpdated.removeListener(onUpdated)
                clearTimeout(timeout)
                resolve()
            }
        }
        chrome.tabs.onUpdated.addListener(onUpdated)
        chrome.tabs.update(ctx.tabId, { url: targetUrl }).catch(() => {
            chrome.tabs.onUpdated.removeListener(onUpdated)
            clearTimeout(timeout)
            resolve() // continue with current page state
        })
    })
}
```

#### 9.7.6 page_structure Performance on Large DOMs

In `analyze-page-structure.ts` (lines 147-169), scroll container detection calls `document.querySelectorAll('*')` and then `getComputedStyle()` on elements where `scrollHeight > clientHeight + 50`. On pages with 10,000+ elements, `getComputedStyle()` forces synchronous layout recalculation.

**Estimates for a 10,000-element page:**
- `querySelectorAll('*')`: ~5-10ms
- Filtering by scrollHeight/clientHeight: ~20-50ms (layout query but no forced reflow if layout is cached)
- `getComputedStyle()` on filtered subset (maybe 50-200 elements): ~50-200ms (forced style recalculation)
- TreeWalker for shadow roots: ~10-30ms

Total: ~85-290ms. Acceptable for typical pages but could spike to 500ms+ on DOM-heavy pages (SPAs with virtual scrolling, complex dashboards).

**Recommendation**: Add a bail-out condition:
```typescript
const allElements = document.querySelectorAll('*')
if (allElements.length > 50000) {
    // Skip expensive scroll container detection
    return { ...result, scroll_containers: [], _note: "scroll_container_detection_skipped_large_dom" }
}
```

#### 9.7.7 Composable Screenshot Timing Bug (see 9.3.4)

As detailed in Section 9.3.4, `include_screenshot` executes synchronously while `auto_dismiss` and `wait_for_stable` are queued asynchronously. The screenshot is captured **before** the queued operations complete. This is a bug when these params are combined.

---

### 9.8 Test Coverage Assessment

**Verdict: Schema and dispatch coverage is adequate. Integration, security, and edge-case coverage has significant gaps.**

#### 9.8.1 What Existing Tests Cover

**`tools_interact_explore_test.go`** (7 tests):
- Dispatch creates correct PendingQuery type and correlation_id prefix
- URL-less calls use current tab (no url in params)
- URL calls include url in params
- Params (visible_only, limit) are forwarded
- Schema enum includes `explore_page`
- Valid actions list includes `explore_page`
- Response structure (snake_case, content blocks)

**`tools_analyze_page_structure_test.go`** (5 tests):
- Dispatch creates correct PendingQuery type
- Schema enum includes `page_structure`
- Response structure
- Invalid JSON arg handling
- tab_id passthrough to PendingQuery

Both test files validate the Go server layer only. No extension-side unit tests are referenced. Extension testing is done via the sharded integration test suite (`test:ext`).

#### 9.8.2 Critical Missing Test Cases

**Priority 1 -- Blocking (security and correctness):**

| Test Case | File to Modify | Reason |
|-----------|---------------|--------|
| `TestExplorePage_JavascriptURL_Rejected` | `tools_interact_explore_test.go` | Section 9.7.2: `javascript:` URLs must be rejected |
| `TestExplorePage_DataURL_Rejected` | `tools_interact_explore_test.go` | Section 9.7.2: `data:` URLs must be rejected |
| `TestExplorePage_ChromeURL_Rejected` | `tools_interact_explore_test.go` | Section 9.7.2: `chrome://` URLs must be rejected |
| `TestBatch_ConcurrentRejection` | `tools_interact_batch_test.go` (create) | Section 9.3.3: two simultaneous batch calls should fail |
| `TestExplorePage_ScreenshotAppend_OnSuccess` | `tools_interact_explore_test.go` | Section 9.2.1: verify screenshot is appended when command completes |
| `TestExplorePage_NoScreenshot_OnError` | `tools_interact_explore_test.go` | Lines 56-58: verify screenshot is NOT appended on error |
| `TestExplorePage_NoScreenshot_OnQueued` | `tools_interact_explore_test.go` | Lines 56-58: verify screenshot is NOT appended when queued |

**Priority 2 -- Important (integration correctness):**

| Test Case | File to Modify | Reason |
|-----------|---------------|--------|
| `TestBatch_WithExplorePageSteps` | `tools_interact_batch_test.go` | Nested explore_page in batch: verify no deadlock |
| `TestBatch_IncludeScreenshot_Stripped` | `tools_interact_batch_test.go` | Section 9.2.2: screenshots in batch steps are wasted |
| `TestComposable_WaitForStable_PlusScreenshot_Ordering` | `tools_interact_composable_test.go` (create) | Section 9.3.4: screenshot timing bug |
| `TestComposable_ActionDiff_WiredUp` | `tools_interact_composable_test.go` | Section 9.7.1: after fix, verify action_diff is queued |
| `TestExplorePage_NavigationDiscoveryScript_URLIncluded` | Extension test | Section 9.1: verify both implementations return same fields |
| `TestPageStructure_LargeDOM_Bailout` | Extension test | Section 9.7.6: verify bail-out on 50k+ elements |

**Priority 3 -- Nice to have:**

| Test Case | Reason |
|-----------|--------|
| `TestExplorePage_PartialFailure_ErrorsPropagated` | Section 9.7.4: verify per-component errors in response |
| `TestBatch_StopAfterStep_Honored` | Verify stop_after_step param |
| `TestBatch_ContinueOnError_False_StopsEarly` | Verify continue_on_error=false stops on first failure |
| `TestPageStructure_CSP_FallbackToIsolated` | Extension test for MAIN->ISOLATED fallback |
| `TestNavigationDiscovery_DuplicateHrefs_Deduped` | Verify seenHrefs deduplication works |

---

### 9.9 Summary Table

| # | Severity | Category | Issue | Location | Recommendation |
|---|----------|----------|-------|----------|----------------|
| 1 | HIGH | Security | No URL validation on explore_page | `tools_interact_explore.go:29`, `interact-explore.ts:209` | Reject non-http/https schemes in both Go and extension |
| 2 | HIGH | Correctness | `action_diff` composable param not wired up | `tools_interact.go:176-183` | Add `ActionDiff bool` to composable struct, add dispatch |
| 3 | HIGH | Data Integrity | `ClampResponseSize` corrupts JSON | `internal/mcp/response.go:224-225` | Implement JSON-aware truncation or exempt image blocks from byte count |
| 4 | HIGH | Correctness | Screenshot captured before queued composable ops complete | `tools_interact.go:194-206` | Reorder: queue composables -> wait -> screenshot |
| 5 | MEDIUM | Performance | Batch with `include_screenshot` wastes work (screenshots discarded) | `tools_interact_batch.go:88-96` | Strip `include_screenshot` from batch step args or return paths |
| 6 | MEDIUM | Maintenance | `navigationDiscoveryScript` duplicated with divergence | `interact-explore.ts:83`, `analyze-navigation.ts:13` | Extract to shared module |
| 7 | MEDIUM | Correctness | explore_page navigation race (listener registered after tabs.update) | `interact-explore.ts:209-225` | Register onUpdated listener before calling tabs.update |
| 8 | MEDIUM | Response Size | explore_page + screenshot regularly exceeds 100KB | Multiple files | Exempt image blocks from ClampResponseSize byte counting |
| 9 | MEDIUM | Test Coverage | No security tests for URL validation | `tools_interact_explore_test.go` | Add javascript:/data:/chrome: URL rejection tests |
| 10 | MEDIUM | Test Coverage | No concurrent batch rejection test | Missing file | Add TryLock contention test |
| 11 | LOW | Performance | page_structure querySelectorAll('*') on huge DOMs | `analyze-page-structure.ts:148` | Add element count bail-out at 50k |
| 12 | LOW | Maintenance | Cross-file `replayMu` dependency undocumented | `tools_interact_batch.go:54`, `tools_configure_sequence.go` | Add cross-reference comments |
| 13 | LOW | Completeness | explore_page missing page_structure data | `interact-explore.ts:231-253` | Add `include_structure` opt-in param |
| 14 | LOW | Completeness | explore_page error reporting inconsistent | `interact-explore.ts:258-327` | Add `_errors` array to response |

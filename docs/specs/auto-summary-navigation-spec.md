---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Tech Spec: Auto-Summarizing Navigation

**Version:** 2.0
**Date:** 2026-02-16
**Status:** Proposed
**Related Issues:** #63, #65

---

## 1. Executive Summary

AI agents spend ~4 tool calls and ~6 seconds orienting after every navigation (navigate, get_page_info, get_text, list_interactive). This feature bundles a compact page summary into the navigate response automatically, reducing orientation to a single turn. The core infrastructure (sync-by-default blocking, long-poll `/sync` endpoint, `pageSummaryScript`) already exists. The actual work is glue: triggering the existing page summary script after navigation completes in the extension, and bundling the result.

---

## 2. Current State

### Already Built

| Component | Location | What It Does |
|-----------|----------|-------------|
| Sync-by-default | `tools_core.go:280` | `maybeWaitForCommand()` blocks 15s for extension result |
| `background` param | `tools_schema.go:512` | Escape hatch for async mode |
| `sync` / `wait` params | `tools_schema.go:504-510` | Additional sync control |
| `pageSummaryScript` | `tools_analyze.go:155-317` | Full page summary (classification, headings, nav links, forms, content preview, word count) |
| `analyze(what="page_summary")` | `tools_analyze.go:319-364` | Standalone page summary via execute query |
| Navigate handler | `tools_interact.go:429-458` | Queues `browser_action` query, waits via `maybeWaitForCommand` |
| Extension navigate | `browser-actions.ts` | `chrome.tabs.update` + `waitForTabLoad` + content script ping |

### What's Missing

1. **Post-navigation summary trigger.** After `handleNavigateAction()` succeeds in the extension, nothing runs the page summary script.
2. **Bundled payload.** The navigate result has no `summary` field.
3. **Compact output mode.** The existing `pageSummaryScript` returns a full payload (~800-1200 tokens). Navigation needs a compact variant (~300-400 tokens).
4. **CTA extraction.** The existing script counts interactive elements but doesn't extract individual CTAs with selectors.

---

## 3. Design

### 3.1 Architecture

The server owns the page summary script (Go string constant). When a navigate command is queued with `summary: true`, the server embeds the script in the `browser_action` params. The extension executes it after navigation completes and returns the combined result.

```
Agent: interact(what="navigate", url="...", summary=true)
  │
  ├─ Server: embeds compact summary script in browser_action params
  ├─ Server: queues query, enters maybeWaitForCommand (15s)
  │
  ├─ Extension: chrome.tabs.update + waitForTabLoad + 500ms settle
  ├─ Extension: sees summary_script in params
  ├─ Extension: chrome.scripting.executeScript(tabId, ISOLATED, script)
  ├─ Extension: merges script result as `summary` field in BrowserActionResult
  ├─ Extension: returns combined payload via syncClient.queueCommandResult
  │
  └─ Server: receives result, returns to agent
```

### 3.2 One Script, Two Modes

The existing `pageSummaryScript` is refactored to accept a `mode` parameter:

```javascript
(function(mode) {
  // ... shared classification, extraction logic ...

  if (mode === 'compact') {
    return { type, title, headings: h1sOnly, primary_actions: top5CTAs,
             forms: compactForms, content_preview: first300chars,
             interactive_count };
  }
  return { url, title, type, headings: allH1H2H3, nav_links, forms: fullForms,
           main_content_preview: first500chars, interactive_element_count, word_count };
})('compact')  // or 'full'
```

The server substitutes the mode when building the script string:
- `handleBrowserActionNavigate` → embeds script with `mode='compact'`
- `toolAnalyzePageSummary` → sends script with `mode='full'`

### 3.3 Compact Summary Schema

Target: ~300-400 tokens when JSON-serialized.

```typescript
interface CompactPageSummary {
  type: string;            // "article"|"search_results"|"dashboard"|"form"|"login"|"error_page"|"link_list"|"app"|"generic"
  title: string;           // document.title, truncated to 120 chars
  headings: string[];      // H1s only (max 3)
  primary_actions: {       // Top 5 visible CTAs
    label: string;         // text content, max 40 chars
    selector: string;      // CSS/semantic selector
    tag: string;           // "button"|"a"|"input"
  }[];
  forms: {                 // Visible forms (max 3)
    fields: string[];      // field names/labels (max 5 per form)
  }[];
  content_preview: string; // First 300 chars of main content area
  interactive_count: number;
}
```

**Token budget:**

| Field | Est. Tokens |
|-------|------------|
| `type` | 1 |
| `title` | 15-30 |
| `headings` (3 max) | 10-30 |
| `primary_actions` (5 max) | 50-100 |
| `forms` (3 max, 5 fields each) | 20-50 |
| `content_preview` (300 chars) | 75-100 |
| `interactive_count` | 1 |
| JSON overhead | ~30 |
| **Total** | **~200-350** |

### 3.4 New Page Classifications

Add to existing `classifyPage()`:

```javascript
// Before existing checks:
if (forms.length === 1 && document.querySelector('input[type="password"]')) {
  return 'login';
}
if (document.title.match(/404|not found|error|500/i) && wordCount < 100) {
  return 'error_page';
}
```

### 3.5 CTA Detection (Simple v1)

No `getComputedStyle` scoring. Simple heuristic for v1:

1. Collect all visible `button`, `[type="submit"]`, `[role="button"]`, and `a[href]` in `<main>` (or body fallback)
2. Filter: must have non-empty text, width > 0, height > 0, not `display:none`
3. Prefer above-the-fold (`getBoundingClientRect().top < window.innerHeight`)
4. Rank: `[type="submit"]` first, then `button`, then `[role="button"]`, then `a[href]`
5. Within same rank: DOM order
6. Deduplicate by normalized label text
7. Take first 5

Selector generation reuses the pattern from `dom-primitives.ts`: prefer `#id` > `[name]` > `aria-label=` > `text=` > CSS path.

### 3.6 Execution Context

**ISOLATED world.** Consistent with existing `page_summary` (line 331, tools_analyze.go). Read-only DOM access doesn't need MAIN world. CSP-safe.

**Timeout: 3 seconds**, independent of the 15s sync wait. Summary failure never fails navigation.

### 3.7 Post-Load Delay

**Keep the existing 500ms delay.** Pages continue rendering after `load` event. The summary adds <100ms on top. Not worth the risk of capturing incomplete pages.

---

## 4. API Contract

### 4.1 Request

```json
{
  "action": "navigate",
  "url": "https://example.com",
  "summary": true
}
```

| Parameter | Type | Default | Applies To |
|-----------|------|---------|-----------|
| `summary` | boolean | `true` | navigate, refresh, back, forward |

Silently ignored on non-navigation actions (click, type, etc.). Silently ignored when `background: true`.

### 4.2 Responses

**Success with summary:**

```json
{
  "success": true,
  "action": "navigate",
  "url": "https://example.com/docs",
  "content_script_status": "loaded",
  "message": "Content script ready",
  "summary": {
    "type": "article",
    "title": "Introduction to Kaboom",
    "headings": ["h1: Introduction to Kaboom"],
    "primary_actions": [
      {"label": "Get Started", "selector": "text=Get Started", "tag": "a"},
      {"label": "Download", "selector": "text=Download", "tag": "button"}
    ],
    "forms": [],
    "content_preview": "Kaboom is a browser extension and MCP server for real-time browser telemetry...",
    "interactive_count": 24
  }
}
```

**Summary failed (navigation still succeeds):**

```json
{
  "success": true,
  "action": "navigate",
  "url": "chrome://version",
  "content_script_status": "loaded",
  "summary": null,
  "summary_error": "script_injection_failed"
}
```

**Navigation failed (no summary attempted):**

```json
{
  "success": false,
  "error": "restricted_url",
  "message": "Cannot navigate to Chrome internal pages"
}
```

**Auth wall detected:**

```json
{
  "summary": {
    "type": "login",
    "title": "Sign In - Example.com",
    "headings": ["h1: Sign In"],
    "primary_actions": [
      {"label": "Sign In", "selector": "input[type=\"submit\"]", "tag": "input"}
    ],
    "forms": [{"fields": ["email", "password"]}],
    "content_preview": "Sign in to your account",
    "interactive_count": 4
  }
}
```

### 4.3 Relationship to `analyze(what="page_summary")`

| | Navigate-bundled | Standalone analyze |
|---|---|---|
| **Mode** | compact | full |
| **Trigger** | Automatic after navigate/refresh/back/forward | Explicit `analyze(what="page_summary")` call |
| **Script** | Same script, `mode='compact'` | Same script, `mode='full'` |
| **Headings** | H1 only, max 3 | H1+H2+H3, max 30 |
| **Content** | 300 chars | 500 chars |
| **Nav links** | Omitted | Up to 25 |
| **Forms** | Max 3, 5 fields each | Max 10, 25 fields each |
| **CTAs** | Top 5 with selectors | Not present (has `interactive_element_count`) |
| **Word count** | Omitted | Included |
| **Custom world** | No (always ISOLATED) | Yes (`world` param) |

`analyze(what="page_summary")` is **not deprecated**. Use it for re-analysis, full output, custom world/tab targeting, or analysis without navigation.

---

## 5. Implementation

### 5.1 Go Changes

**New file: `cmd/browser-agent/tools_page_summary.go`**

Extract `pageSummaryScript` from `tools_analyze.go` into its own file. Refactor to accept `mode` parameter. Add `login` and `error_page` classification. Add CTA extraction for compact mode.

```go
// tools_page_summary.go — Page summary script and helpers.
// Contains the pageSummaryScript IIFE and mode-specific builders.
package main

// pageSummaryScript is a self-contained IIFE that analyzes the current page.
// Accepts a mode parameter: 'compact' (navigate-bundled) or 'full' (standalone).
const pageSummaryScript = `(function(mode) { ... })`

// compactSummaryScript returns the summary script with mode='compact'.
func compactSummaryScript() string {
    return pageSummaryScript + "('compact')"
}

// fullSummaryScript returns the summary script with mode='full'.
func fullSummaryScript() string {
    return pageSummaryScript + "('full')"
}
```

**Modify: `cmd/browser-agent/tools_interact.go`**

Update `handleBrowserActionNavigate` to embed the compact summary script in the browser_action params:

```go
func (h *ToolHandler) handleBrowserActionNavigate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
    var params struct {
        URL     string `json:"url"`
        TabID   int    `json:"tab_id,omitempty"`
        Summary *bool  `json:"summary,omitempty"`
    }
    // ... existing validation ...

    // Build browser_action params — include summary script if enabled
    navParams := map[string]any{"action": "navigate", "url": params.URL}
    if params.Summary == nil || *params.Summary {
        navParams["summary_script"] = compactSummaryScript()
    }

    navArgs, _ := json.Marshal(navParams)
    query := queries.PendingQuery{
        Type:   "browser_action",
        Params: navArgs,
        // ...
    }
    // ... rest unchanged ...
}
```

Same pattern for `handleBrowserActionRefresh`, `handleBrowserActionBack`, `handleBrowserActionForward`.

**Modify: `cmd/browser-agent/tools_analyze.go`**

Update `toolAnalyzePageSummary` to use `fullSummaryScript()` instead of the raw constant.

**Modify: `cmd/browser-agent/tools_schema.go`**

Add `summary` parameter to the interact tool schema:

```go
"summary": map[string]any{
    "type":        "boolean",
    "description": "Include page summary after navigation (default: true for navigate/refresh/back/forward).",
},
```

**Modify: `cmd/browser-agent/handler.go`**

Update `serverInstructions` to mention auto-summary:

```
- Performance: interact(what="navigate"|"refresh") auto-includes perf_diff and page summary.
  Set summary=false to skip. Use analyze(what="page_summary") for full standalone analysis.
```

### 5.2 Extension Changes

**Modify: `src/background/exec/browser-actions.ts`**

After successful navigation (post-`waitForTabLoad`, post-500ms delay, post-content-script-ping), check for `summary_script` in params:

```typescript
// After navigation succeeds:
if (params.summary_script) {
  try {
    const [result] = await Promise.race([
      chrome.scripting.executeScript({
        target: { tabId },
        world: 'ISOLATED',
        func: new Function('return ' + params.summary_script) // IIFE string
      }),
      timeoutPromise(3000, 'summary_timeout')
    ]);
    navResult.summary = result?.result ?? null;
  } catch (err) {
    navResult.summary = null;
    navResult.summary_error = err.message || 'summary_failed';
  }
}
```

**Note:** The extension does NOT own the summary logic. It receives a script string from the server and executes it. This keeps the extension thin and the script under Go's control.

### 5.3 File Impact Summary

| File | Change | LOC Delta |
|------|--------|-----------|
| `cmd/browser-agent/tools_page_summary.go` | **New** — extracted + refactored script | ~250 |
| `cmd/browser-agent/tools_interact.go` | Modify — embed script in navigate params | +30 |
| `cmd/browser-agent/tools_analyze.go` | Modify — use `fullSummaryScript()` | -165, +3 |
| `cmd/browser-agent/tools_schema.go` | Modify — add `summary` param | +4 |
| `cmd/browser-agent/handler.go` | Modify — update serverInstructions | +2 |
| `src/background/exec/browser-actions.ts` | Modify — run summary_script after nav | +20 |

---

## 6. Error Handling & Edge Cases

### Handled

| Scenario | Behavior |
|----------|----------|
| Summary script times out (>3s) | `summary: null, summary_error: "summary_timeout"` — navigation still succeeds |
| Script injection fails (`chrome://`, `about:blank`) | `summary: null, summary_error: "script_injection_failed"` |
| Navigation fails (DNS, 404) | No summary attempted. Normal error response. |
| Page redirects to login | Summary runs on final page. `type: "login"` detected. Agent sees final URL differs from requested URL. |
| `background: true` | Summary skipped entirely. Agent polls for navigate result only. |
| `summary: false` | Script not embedded in params. No overhead. |
| Huge DOM (100k+ nodes) | 3s timeout is the hard cap. `findMainNode()` scopes to subtree. Fixed element limits (3 headings, 5 CTAs, 3 forms). |

### Out of Scope (v1)

| Scenario | Why | Workaround |
|----------|-----|-----------|
| SPA client-side navigation via click | Detecting `pushState` adds complexity | Agent calls `analyze(what="page_summary")` after click |
| Iframe content | Requires `allFrames: true` + result aggregation | Agent uses `analyze(what="dom", frame="all")` |
| Lazy-loaded / infinite scroll content | No stable "done loading" signal | Summary captures DOM at load time. Agent re-analyzes later if needed. |
| Cookie consent modals obscuring content | Hard to distinguish modal CTAs from page CTAs | Summary may include modal buttons. Agent can dismiss and re-navigate. |
| Pages that never finish loading (streaming) | No reliable stability signal | Summary runs after `waitForTabLoad`. Agent re-analyzes later. |

---

## 7. Performance Budget

| Phase | Target | Notes |
|-------|--------|-------|
| Navigation (`chrome.tabs.update` + load) | Variable (1-10s) | Network/server dependent |
| Post-load settle | 500ms | Fixed. Do not reduce. |
| Summary script execution | <100ms (p95) | DOM read + simple heuristics |
| Summary script timeout | 3000ms | Hard cap. Independent of 15s sync wait. |
| Total summary overhead | <200ms (p95) | Script + serialization |
| Total sync wait ceiling | 15000ms | Existing. Unchanged. |

**Invariants:**
- Summary script MUST NOT modify the DOM
- Summary script MUST NOT make network requests
- Summary script MUST NOT trigger layout reflow beyond `innerText` and `getBoundingClientRect`

---

## 8. Testing Strategy

### 8.1 Go Unit Tests

**New file: `cmd/browser-agent/tools_page_summary_test.go`**

- `compactSummaryScript()` and `fullSummaryScript()` return valid JS (syntax check via `goja` or string validation)
- Script string contains `mode` parameter handling
- Compact mode omits `nav_links`, `word_count`
- Full mode includes all fields

**Modify: `cmd/browser-agent/tools_interact_handler_test.go`**

- Navigate with default params includes `summary_script` in queued query params
- Navigate with `summary: false` omits `summary_script`
- Navigate with `background: true` omits `summary_script`
- Refresh/back/forward include `summary_script` by default
- Click/type do NOT include `summary_script`

**Modify: `cmd/browser-agent/tools_interact_nav_test.go`**

- Response with `summary` field passes through `formatCommandResult` unchanged
- Response with `summary: null, summary_error: "..."` passes through correctly

### 8.2 Extension Tests

- `handleNavigateAction` executes `summary_script` when present in params
- `handleNavigateAction` skips summary when `summary_script` absent
- Summary timeout (3s) doesn't block navigation result
- Summary script error produces `summary: null, summary_error`

### 8.3 Smoke Test

**New file: `scripts/smoke-tests/11-auto-summary.sh`**

1. Navigate to a known URL → verify response contains `summary` with expected fields
2. Navigate with `summary: false` → verify no `summary` in response
3. Navigate with `background: true` → verify queued response, no summary
4. `analyze(what="page_summary")` still works independently
5. Summary field types are correct (string, array, number)

### 8.4 Manual Validation

| Page | Expected `type` | Key Signals |
|------|----------------|-------------|
| `https://github.com` | `app` or `generic` | CTAs: "Sign in", "Sign up" |
| Wikipedia article | `article` | Headings extracted, content preview present |
| Google.com | `form` or `search_results` | Search input detected |
| Login page | `login` | Password field, single form |
| 404 page | `error_page` | "Not Found" in title, low word count |

---

## 9. Migration & Backwards Compatibility

**The change is additive.** The `summary` field is added to navigate responses. No existing fields are removed or renamed.

Agents that don't expect `summary` will ignore it (standard JSON behavior). No breaking change.

Agents using the existing 4-step orientation pattern (navigate → page_info → get_text → list_interactive) continue to work. They can be gradually updated to check for `summary` in the navigate response and skip the follow-up calls.

**`analyze(what="page_summary")` is unchanged.** Same behavior, same response shape. The only internal change is it now calls `fullSummaryScript()` instead of using the raw constant directly.

---

## 10. Resolved Decisions

| Decision | Resolution | Rationale |
|----------|-----------|-----------|
| `summary` default | `true` for navigation actions | Token savings (~3 round trips) far outweigh ~350 added tokens. Server instructions updated so agents know it's there. |
| Script location | Go string constant | Matches existing `pageSummaryScript` pattern. Server controls what runs. Single source of truth for both modes. |
| Script file | Own file (`tools_page_summary.go`) | Self-contained unit with own tests. Prevents `tools_analyze.go` from growing. |
| Compact vs full | One script, `mode` parameter | Avoids duplicating classification logic. One codepath to maintain. |
| CTA scoring | Simple v1 (type rank + DOM order) | No `getComputedStyle` scoring. Agents need "what's clickable," not pixel-perfect prominence. Refine in v2 if needed. |
| Execution world | ISOLATED | CSP-safe, read-only DOM access, no page interference. Consistent with existing `page_summary`. |
| Post-load delay | Keep 500ms | Pages render after `load` event. Not worth saving 300ms at the risk of incomplete pages. |
| Click auto-summary | No (v1) | SPA detection is complex. Agent calls `analyze(what="page_summary")` after click if needed. |
| Nav links in compact | Omitted | Rarely actionable in immediate next step. Available via full mode. |

---

## 11. Open Questions

1. **Should `new_tab` also auto-summarize?** It navigates to a URL but creates a new tab. Likely yes, but needs testing since tab targeting may differ.
2. **Should the compact summary include a `word_count` for agents to decide if they need the full version?** Low cost (~1 token), could be useful.
3. **Should redaction run on `content_preview`?** The existing `RedactionEngine` processes MCP responses. Need to verify it catches PII in nested JSON fields. If not, the summary script itself should strip common patterns (emails, phone numbers) before returning.

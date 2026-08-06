---
doc_type: architecture_decision
status: active
last_reviewed: 2026-08-06
---

# ADR-005: Auto-Summarizing Navigation

- **Status**: Accepted
- **Date**: 2026-02-16
- **Related Issues**: #63, #65

## Context

AI agents spend ~4 tool calls and ~6 seconds orienting after every navigation. The current flow:

1. `interact(what="navigate")` — navigate and wait
2. `get_page_info` — get title/URL
3. `get_text` — get content
4. `list_interactive` — get actionable elements

This creates repetitive approve prompts and wastes ~1,500 tokens per orientation loop.

The synchronous-by-default infrastructure already exists (commit `86d27cc`): `maybeWaitForCommand()` blocks 15s, `background: true` provides an async escape hatch, and `analyze(what="page_summary")` already extracts structured page data. What's missing is the glue to trigger summary automatically after navigation.

## Decision

Bundle a **compact page summary** into navigate/refresh/back/forward responses by default.

### Key Changes

1. **One script, two modes.** The existing `pageSummaryScript` (Go string constant) is refactored to accept a `mode` parameter. `compact` mode returns ~300-400 tokens. `full` mode retains the existing behavior for `analyze(what="page_summary")`.

2. **Server embeds script.** When `summary: true` (default for navigation actions), the Go server embeds the compact summary script in the `browser_action` params sent to the extension.

3. **Extension executes and bundles.** After successful navigation, the extension runs the embedded script via `chrome.scripting.executeScript` (ISOLATED world, 3s timeout) and merges the result into `BrowserActionResult`.

4. **Graceful degradation.** Summary failure (timeout, injection blocked) produces `summary: null, summary_error: "reason"`. Navigation result is always returned.

5. **New classifications.** `login` (password field detected) and `error_page` (404/500 in title, low word count) added to page classifier.

### What This Does NOT Change

- `analyze(what="page_summary")` is unchanged and not deprecated
- Non-navigation actions (click, type, etc.) do not auto-summarize
- SPA client-side navigation is out of scope for v1

## Consequences

- **Positive**: ~75% reduction in orientation turns (4 → 1)
- **Positive**: ~73% token savings per orientation (~1,500 → ~400 tokens)
- **Positive**: Script ownership stays in Go — single source of truth
- **Negative**: Navigate responses are larger (~350 tokens added). Mitigated by `summary: false`.
- **Neutral**: Additive response change — no backwards compatibility break

## Implementation Spec

See [Tech Spec: Auto-Summarizing Navigation](../../specs/contracts/auto-summary-navigation-spec.md) for detailed design, API contracts, error handling, and testing strategy.

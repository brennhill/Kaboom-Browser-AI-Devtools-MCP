---
doc_type: tech-spec
feature_id: feature-auto-fix
status: proposed
owners: []
last_reviewed: 2026-07-27
links:
  index: ./index.md
  product: ./product-spec.md
code_paths:
  - cmd/browser-agent/internal/toolanalyze/pageissues/handler.go
  - cmd/browser-agent/internal/toolanalyze/page_issues_summary.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - cmd/browser-agent/handler_tools_call_postprocess.go
  - cmd/browser-agent/internal/terminal/intent_store.go
  - cmd/browser-agent/internal/terminal/intent_handlers.go
  - src/lib/tabs/request-audit.ts
  - src/background/message-handlers.ts
test_paths:
  - cmd/browser-agent/internal/toolanalyze/pageissues/handler_test.go
  - cmd/browser-agent/internal/toolanalyze/pageissues/summary_test.go
  - cmd/browser-agent/handler_tools_call_postprocess_test.go
  - tests/extension/reliability/request-audit.test.js
  - tests/extension/content/message-handlers.test.js
---

# Auto-Fix Tech Spec

> Plain language. Describes how the Phase 1 audit workflow is wired across the MCP server and the extension. No code.

## TL;DR

- Design: One server-side evidence sweep (`page_issues`) plus one shared extension trigger (`requestAudit`) feeding one repo-owned command/skill that emits one fixed report.
- Key constraints: Local-only, parallel checks with per-check timeouts, terminal-first delivery with a daemon-intent fallback.
- Rollout risk: Low-to-medium — composes existing primitives; the main contracts to preserve are the runtime bridge name and the daemon intent action.

## Architecture Overview

Auto-fix has two halves that meet at the agent:

1. **Evidence sweep (server).** `analyze(what="page_issues")` aggregates every detectable page issue into a single response so an agent does not need to know which individual tools to call.
2. **Trigger and delivery (extension + daemon).** The tracked-site `Audit` entrypoints call one shared helper that opens the terminal panel and requests an audit, with a daemon-intent fallback and an `ACTION REQUIRED` nudge on the next MCP response.

The agent then runs the `/kaboom/audit` command or the bundled `audit` skill, which calls `page_issues` for its baseline and produces the final six-lane report.

## Key Components

### `pageissues.Handle` — the evidence sweep

Located in `cmd/browser-agent/internal/toolanalyze/pageissues/handler.go`. Flow:

1. Parse params (`summary`, `categories`, `limit`); default `limit` to the per-section cap (50).
2. Check tracking via `capture.GetTrackingStatus()`. If no tab is tracked, return `ErrNoData` with a recovery tool call (`configure(what="health")`).
3. Resolve the category set (`defaultCategories`) — all four by default.
4. Run the checks (`runPageIssuesChecks`) and either return the full result or, when `summary` is set, the condensed `BuildPageIssuesSummary` output.

**Shared data prefetch.** `prefetchSharedData` copies the network bodies, waterfall entries, and console/log entries once under a single read lock. This avoids redundant buffer copies and a time-of-check/time-of-use gap between separate lock acquisitions, since multiple parallel checkers read the same underlying data.

**Parallel checks with per-check timeout.** Each enabled category becomes a `pageIssuesChecker`. Checkers run concurrently via `util.SafeGo`; each inner check is wrapped in a buffered `done` channel and a `select` against `pageIssuesCheckTimeout` (5s). A timed-out check is recorded under `checks_skipped`; a completed check (even one returning an error) is recorded under `checks_completed`.

**The four checkers:**
- `console_errors` — filters prefetched log entries for `error`/`warn` levels; `error` maps to `high`, `warn` to `medium`.
- `network_failures` — filters prefetched network bodies for status >= 400; >= 500 maps to `high`, otherwise `medium`.
- `accessibility` — runs an async axe-core audit via `ExecuteA11yQuery` (not prefetchable) and maps axe impact to the unified severity scale (`mapA11yImpact`).
- `security` — runs the security scanner against the prefetched bodies, console entries, page URLs, and waterfall entries.

**Aggregation.** Results are collected from a channel, building `sections`, `total_issues`, and a `by_severity` tally, plus `checks_completed`/`checks_skipped`, the `page_url`, and an RFC3339 `timestamp`.

### Summary mode — `BuildPageIssuesSummary`

In `cmd/browser-agent/internal/toolanalyze/page_issues_summary.go`. When `summary: true`, the full result is mapped into the package's `PageIssuesResult` and condensed to the top findings for a large token reduction, serving as the audit baseline.

### Shared trigger — `requestAudit`

In `src/lib/tabs/request-audit.ts`. Both the popup `Audit` button and the on-page hover launcher call this one helper, which:
1. Opens the terminal side panel first (`open_terminal_panel`), treating open failures as best-effort.
2. Sends the existing `qa_scan_requested` runtime message with the tracked page URL.

`src/background/message-handlers.ts` converts that runtime message into audit-oriented prompt text for terminal injection.

### Fallback and nudge — daemon intent store

The terminal server injects the prompt into the active PTY when possible. When PTY injection is unavailable, it persists a `qa_scan` intent (`cmd/browser-agent/internal/terminal/intent_store.go`, `intent_handlers.go`). On the next MCP tool response, `handler_tools_call_postprocess.go` prepends an `ACTION REQUIRED` warning pointing the operator at `/kaboom/audit` (or `/audit`).

## Data Flow

```
User clicks Audit (popup or hover)
  -> requestAudit (src/lib/tabs/request-audit.ts)
       1. open terminal side panel (best-effort)
       2. send qa_scan_requested runtime message (with tracked URL)
  -> message-handlers.ts builds audit prompt text
  -> terminal server:
       primary  : inject prompt into active PTY
       fallback : store qa_scan intent in the daemon
  -> next MCP tool response: handler_tools_call_postprocess prepends ACTION REQUIRED -> /kaboom/audit
  -> agent runs /kaboom/audit (or audit skill):
       health check
       analyze(what="page_issues", summary=true)  -> baseline evidence
       page exploration + interaction discovery
       six-lane review
  -> one local markdown report
```

## Implementation Strategy

The server side composes existing capture and security subsystems; no new telemetry source is introduced. The extension side reuses the existing `qa_scan_requested` bridge and `qa_scan` intent rather than inventing new message types, keeping the cross-context contract stable. The command and bundled skill are repo-owned assets that share one report structure.

**Trade-offs:**
- Parallel with per-check timeout vs sequential: parallel keeps total latency near the slowest check while a hung check (for example, a stalled a11y audit) cannot block the whole sweep.
- Prefetch-once vs per-checker fetch: prefetching avoids redundant copies and TOCTOU inconsistency at the cost of a single up-front snapshot.
- Summary vs full: summary mode trades completeness for roughly 80% fewer tokens, used as the audit baseline.

## Edge Cases & Assumptions

- **No tracked tab**: returns `ErrNoData` with a recovery tool call; the workflow copy tells the user to track a site first.
- **A check times out**: recorded under `checks_skipped`; the rest of the report still returns.
- **A check errors (non-timeout)**: recorded under `checks_completed` with an empty issue list and an `error` field in its section.
- **Security scanner not configured**: the security checker returns no issues rather than failing.
- **Terminal panel fails to open**: `requestAudit` proceeds to send the audit bridge anyway.
- **PTY injection unavailable**: the daemon persists the intent and nudges the next MCP response.
- **Namespaced slash commands unsupported**: operators fall back from `/kaboom/audit` to `/audit`.
- **Assumption**: per-category `limit` (default 50) is sufficient for triage; agents can request full detail without `summary`.

## Security Considerations

- **Data captured**: console messages, network metadata (URL, method, status, duration, content type), accessibility violations, and security findings — all already captured by Kaboom's telemetry.
- **Locality**: all data flows over localhost; nothing is transmitted externally (Phase 1 is local-only).
- **Attack surface**: the sweep is read-only over already-captured buffers plus one async a11y audit; it does not execute arbitrary page code.

## Edit Guardrails

- Do not rename the `qa_scan_requested` runtime bridge or the `qa_scan` intent action without updating both the extension and daemon contracts together.
- Keep popup and hover entrypoints on the same `requestAudit` helper.
- Preserve terminal-first behavior before requesting the audit bridge.
- Keep the command and bundled skill aligned on one report structure and one six-lane methodology.

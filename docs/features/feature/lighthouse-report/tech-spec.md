---
feature: lighthouse-report
status: proposed
doc_type: tech-spec
feature_id: feature-lighthouse-report
last_reviewed: 2026-06-29
last_verified_version: 0.8.4
last_verified_date: 2026-06-29
---

# Tech Spec: Lighthouse Report

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Lighthouse Report adds a new analyze mode, `analyze({what: "lighthouse_report"})`, that runs
a real Google Lighthouse audit against the tracked tab and returns a trimmed result. The mode
registers in the analyze dispatch registry (`analyzeHandlers` in
`cmd/browser-agent/tools_analyze_dispatch.go`) alongside existing modes such as `audit`,
`performance`, and `accessibility`. Its hint and optional parameters register in
`internal/tools/configure/mode_specs_analyze.go`.

The feature reuses the asynchronous command pattern already used by the interact tool: the Go
daemon enqueues a pending command, the extension executes it against the live browser, and the
daemon waits for the result. A Lighthouse navigation audit takes ten to thirty seconds, so the
mode follows the long-running command path rather than the synchronous query path.

Two implementation paths are possible. The recommended path (Option A) shells out to the
Lighthouse command-line interface (CLI) from the daemon, pointing it at the tab's remote
debugging endpoint. This preserves the extension's zero-dependency constraint, since Lighthouse
never enters the extension bundle. The alternative path (Option B) replicates a subset of
Lighthouse audits directly through CDP domains in the extension; it avoids the external
dependency but duplicates work the existing `audit` mode already approximates.

## Key Components

**Analyze mode handler (Go)**: A new handler registered in `analyzeHandlers` validates the
request parameters (`categories`, `device`, `mode`), then either invokes the Lighthouse CLI as
a subprocess or dispatches an asynchronous CDP command to the extension. The handler follows
the same registration and dispatch conventions as the existing `toolAnalyzeAudit` handler in
`cmd/browser-agent/tools_analyze_audit.go`.

**CDP attach lifecycle (extension)**: The extension already manages the Chrome debugger
attach and detach lifecycle in `src/background/cdp-dispatch.ts`, using
`chrome.debugger.attach({tabId}, CDP_VERSION)` and `chrome.debugger.detach({tabId})`. Lighthouse
requires a debuggable target, so the extension exposes the tracked tab's debugger endpoint and
ensures no conflicting debugger session is already attached.

**Result trimmer (Go)**: A raw Lighthouse JavaScript Object Notation (JSON) report is one
hundred kilobytes or larger. The trimmer extracts only actionable fields: category scores, the
Core Web Vitals metrics (First Contentful Paint, Largest Contentful Paint, Cumulative Layout
Shift, Total Blocking Time, Speed Index, and Time to Interactive), the top opportunities with
estimated savings, and diagnostics. The trimmed response targets under five kilobytes.

**Asynchronous command plumbing**: The daemon enqueues the audit as a pending command and the
agent polls for completion through `observe({what: "command_result"})`. Pending and failed
commands are observable through `observe({what: "pending_commands"})` and
`observe({what: "failed_commands"})`, consistent with the existing command-result enum in
`internal/schema/observe.go`.

## Data Flows

```
AI calls analyze({what: "lighthouse_report", categories: [...], device: "mobile", mode: "navigation"})
  |
  v
Go daemon: lighthouse_report handler validates parameters
  |
  v
Daemon enqueues an asynchronous command (audit is long-running, 10-30s)
  |
  v
Extension ensures the tracked tab is debuggable (chrome.debugger attach, no conflicting session)
  |
  v
Option A: daemon shells out to the Lighthouse CLI against the tab's remote debugging endpoint
Option B: extension drives Lighthouse-equivalent audits through CDP domains
  |
  v
Raw Lighthouse JSON report produced (100KB+)
  |
  v
Daemon trims to scores, Core Web Vitals, top opportunities, and diagnostics (<5KB)
  |
  v
Agent polls observe({what: "command_result"}) and receives the structured report
```

## Implementation Strategy

**New server files**:
- A `lighthouse_report` handler wired into `analyzeHandlers` in
  `cmd/browser-agent/tools_analyze_dispatch.go`.
- A result-trimming helper that maps the raw Lighthouse JSON to the structured response.

**Modified server files**:
- `internal/tools/configure/mode_specs_analyze.go`: add the `lighthouse_report` mode hint and
  its optional parameters (`categories`, `device`, `mode`).

**Extension files**:
- `src/background/cdp-dispatch.ts` and `src/background/commands/analyze.ts`: expose the tracked
  tab's debugger endpoint and guard against conflicting debugger sessions.

**Trade-offs**:
- CLI subprocess (Option A) versus in-extension CDP audits (Option B): the CLI keeps the
  extension dependency-free and produces authoritative scores, but it requires Lighthouse in the
  user's PATH. In-extension audits remove the external dependency but reimplement Lighthouse and
  drift from upstream over time. Option A is recommended for the first ship.
- Trimmed response versus full report: the full report is authoritative but exceeds the context
  budget. Returning a trimmed subset keeps the response actionable while preserving the option to
  save the full report to a file for manual inspection.

## Edge Cases & Assumptions

### Edge Cases

- **Debugger already attached** (for example, DevTools is open): return a clear error naming the
  conflicting session and a recovery action ("close DevTools or other debugging sessions"),
  matching the existing attach-conflict messaging in `src/background/cdp-dispatch.ts`.
- **Lighthouse CLI not in PATH** (Option A): return an actionable error explaining that the
  Lighthouse CLI must be installed, since it is the user's responsibility.
- **Audit exceeds the timeout**: the navigation audit budget is sixty seconds (configurable). On
  timeout, return a timeout error through the failed-commands path.
- **Internal browser page** (`chrome://`, `about:blank`): the debugger cannot attach; return the
  same "cannot attach to this target" error the CDP layer already surfaces.
- **Category filtering**: when `categories` is omitted, run all four categories; when provided,
  run only the requested subset to reduce audit time.
- **Snapshot mode**: `mode: "snapshot"` audits the current page state without reloading; this
  yields no navigation-timing metrics, so the response omits load-only Core Web Vitals.

### Assumptions

- A1: The extension is connected and tracking a tab.
- A2: The tracked tab hosts a real web page, not an internal browser page.
- A3: For Option A, the Lighthouse CLI is available in the user's PATH (common in Node.js
  environments).
- A4: The `debugger` permission is already granted in the manifest, and the CDP attach/detach
  lifecycle in `src/background/cdp-dispatch.ts` is the single point of debugger management.

## Risks & Mitigations

### Risk 1: Lighthouse CLI unavailable or version-skewed
- **Description**: Option A depends on a Lighthouse CLI in PATH, whose version the daemon does
  not control.
- **Mitigation**: Detect absence early and return an actionable install message. Record the
  Lighthouse version in the response so the agent can reason about score comparability.

### Risk 2: Debugger attach conflicts
- **Description**: A second debugger session (open DevTools, another automation tool) blocks the
  audit.
- **Mitigation**: Reuse the existing attach-conflict detection in `src/background/cdp-dispatch.ts`
  and return a recovery action rather than a generic failure.

### Risk 3: Oversized responses
- **Description**: The raw Lighthouse report is one hundred kilobytes or larger and would
  overflow the context window.
- **Mitigation**: Trim aggressively to scores, Core Web Vitals, top opportunities, and
  diagnostics (under five kilobytes typical). Offer an opt-in file save for the full report.

### Risk 4: Long-running audit blocks the agent
- **Description**: A navigation audit takes ten to thirty seconds, longer than synchronous query
  budgets allow.
- **Mitigation**: Use the asynchronous command pattern with polling via
  `observe({what: "command_result"})`, the same mechanism used by long-running interact commands.

## Dependencies

### Depends on:
- The analyze dispatch registry (`cmd/browser-agent/tools_analyze_dispatch.go`).
- The CDP attach/detach lifecycle (`src/background/cdp-dispatch.ts`).
- The asynchronous command pattern and command-result polling
  (`cmd/browser-agent/internal/toolinteract/interact_command_builder.go`,
  `observe({what: "command_result"})`).
- For Option A, a Lighthouse CLI in the user's PATH.

### Depended on by:
- AI agents that need an authoritative pre-ship benchmark rather than the fast heuristic
  `analyze({what: "audit"})` result.

## Performance Considerations

| Metric | Target | Implementation notes |
|--------|--------|---------------------|
| Navigation audit duration | 10-30s | Synthetic benchmark; runs via asynchronous command |
| Timeout budget | 60s (configurable) | Failed-command path on exceed |
| Response size | < 5KB typical | Trimmed to scores, Core Web Vitals, opportunities, diagnostics |
| Daemon parsing of raw report | < 200ms | Single pass over the Lighthouse JSON |

## Security Considerations

**Data captured**: Public performance and quality metrics for the audited page. No form values,
credentials, or storage contents are captured by the Lighthouse run itself.

**Data path**: All audit traffic and results stay on localhost (`127.0.0.1`). The trimmed report
is returned only to the local agent. No external transmission occurs.

**Attack surface**: The mode attaches the debugger (already a granted permission) and, for Option
A, spawns a Lighthouse subprocess. The subprocess is invoked with controlled arguments pointing
at the local debugging endpoint; no user-supplied strings are passed unescaped to the shell.

**Privacy implications**: A Lighthouse navigation audit reloads the page, which may re-trigger
network requests. This is equivalent to the user reloading the tab and does not exfiltrate data.

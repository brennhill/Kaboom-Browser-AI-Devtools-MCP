---
feature: performance-trace
status: proposed
doc_type: tech-spec
feature_id: feature-performance-trace
last_reviewed: 2026-07-05
last_verified_version: 0.8.6
last_verified_date: 2026-06-29
---

# Tech Spec: Performance Trace

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Performance Trace adds a new analyze mode, `analyze({what: "performance_trace"})`, that records
a Chrome DevTools Protocol (CDP) performance trace and returns structured insights. A single
`action` parameter drives the trace lifecycle (`start`, `stop`, `analyze`), keeping the entire
workflow inside one mode rather than three separate tools. The mode registers in the analyze
dispatch registry (`analyzeHandlers` in `cmd/browser-agent/tools_analyze_dispatch.go`), and its
hint plus optional parameters register in `internal/tools/configure/mode_specs_analyze.go`.

The extension drives the CDP `Tracing` domain through the existing Chrome debugger lifecycle in
`src/background/dom/cdp/cdp-dispatch.ts`. The daemon parses raw trace events into actionable insights and
returns a compact summary. This is the on-demand counterpart to the passive Web Vitals path,
which already lives in `internal/tools/observe/analysis.go` and `internal/performance/diff.go`;
Performance Trace reuses the same Web Vitals vocabulary (First Contentful Paint, Largest
Contentful Paint, Cumulative Layout Shift) where the two overlap.

## Key Components

**Trace lifecycle (extension)**: On `action: "start"`, the extension attaches the debugger and
calls `Tracing.start` with the categories `devtools.timeline`, `v8.execute`, and
`blink.user_timing`. If `reload` is true, it reloads the page to capture the full load. It
listens for `Tracing.dataCollected` events and accumulates trace chunks. On `action: "stop"` (or
automatically when `auto_stop` is true and the page load completes), it calls `Tracing.end`,
waits for `Tracing.tracingComplete`, and detaches the debugger.

**Trace ingestion and insight extraction (Go)**: The daemon parses the accumulated trace events
and derives insights: long tasks (main-thread blocks longer than fifty milliseconds), layout
shifts with their Cumulative Layout Shift contribution, forced reflows (style recalculation
triggered after a Document Object Model mutation), script-evaluation time grouped by source
Uniform Resource Locator (URL), and a rendering-pipeline breakdown across paint, composite, and
layout. It also computes a main-thread time breakdown (script, render, idle).

**Insight cache (Go)**: The structured insights from the most recent trace are cached in memory
so `action: "analyze"` can drill into a specific `insight_id` (call stack, affected elements,
timing breakdown) without re-tracing. The cache clears on the next `action: "start"` or at
session end. Raw trace data is optionally saved to a file.

**Asynchronous command plumbing**: `start` and `stop` are dispatched as asynchronous commands
through the existing command builder
(`cmd/browser-agent/internal/toolinteract/interact_command_builder.go`); results are polled via
`observe({what: "command_result"})`. The `analyze` action operates on the cached insights and is
synchronous.

## Data Flows

```
AI calls analyze({what: "performance_trace", action: "start", reload: true})
  |
  v
Daemon enqueues an asynchronous start command
  |
  v
Extension: chrome.debugger attach -> Tracing.start(devtools.timeline, v8.execute, blink.user_timing)
  -> (optional) reload page -> accumulate Tracing.dataCollected chunks
  |
  v
AI calls analyze({what: "performance_trace", action: "stop"})   (or auto_stop fires)
  |
  v
Extension: Tracing.end -> wait Tracing.tracingComplete -> detach -> deliver trace to daemon
  |
  v
Daemon parses events into insights (long tasks, layout shifts, forced reflows,
  script-eval by source, render breakdown) and a main-thread time summary
  |
  v
Daemon returns a compact summary (<5KB) and caches insights for drill-down
  |
  v
AI calls analyze({what: "performance_trace", action: "analyze", insight_id: "..."})
  -> synchronous detail from the cached insight (call stack, affected elements, timing)
```

## Implementation Strategy

**New server files**:
- A `performance_trace` handler wired into `analyzeHandlers` in
  `cmd/browser-agent/tools_analyze_dispatch.go`, with `action` sub-dispatch for start/stop/analyze.
- A trace-event parser that derives insights and the main-thread breakdown.
- An in-memory insight cache keyed for `action: "analyze"` drill-down.

**Modified server files**:
- `internal/tools/configure/mode_specs_analyze.go`: add the `performance_trace` mode hint and its
  optional parameters (`action`, `reload`, `auto_stop`, `insight_id`).

**Extension files**:
- `src/background/dom/cdp/cdp-dispatch.ts` and `src/background/commands/analyze.ts`: add the `Tracing`
  domain start/collect/stop sequence and chunk accumulation.

**Reused components**:
- Web Vitals vocabulary and rating helpers in `internal/performance/diff.go`.
- The passive vitals path in `internal/tools/observe/analysis.go` (the always-on counterpart).

**Trade-offs**:
- Single mode with `action` dispatch versus three separate tools: consolidating start, stop, and
  analyze into one `what` mode keeps the dispatch architecture consistent and reduces
  initialization tokens, at the cost of a stateful lifecycle the agent must sequence correctly.
- Insight extraction versus full-trace return: parsing the ten-to-fifty-megabyte trace in the
  daemon and returning only insights keeps the response within the context budget; the raw trace
  remains available via optional file save for manual inspection.

## Edge Cases & Assumptions

### Edge Cases

- **`stop` without a prior `start`**: return a clear "no active trace" error.
- **Second `start` while a trace is active**: either reject with an "already tracing" error or
  discard the previous trace; the chosen behavior is documented in the response.
- **Debugger already attached**: return the existing attach-conflict error from
  `src/background/dom/cdp/cdp-dispatch.ts` with a recovery action.
- **`auto_stop` with no load event** (single-page application navigation): fall back to a maximum
  trace duration so the recording does not run indefinitely.
- **`analyze` with an unknown `insight_id`**: return a "no such insight" error listing valid ids.
- **Empty or quiet trace**: return a summary with zero long tasks and a note rather than an error.

### Assumptions

- A1: The extension is connected and tracking a tab.
- A2: The `debugger` permission is granted and the CDP lifecycle in
  `src/background/dom/cdp/cdp-dispatch.ts` is the single point of debugger management.
- A3: The tracked tab hosts a real web page, not an internal browser page.
- A4: The agent sequences `start` then `stop` before `analyze`; insights exist only after a
  completed trace.
- A5: A bounded set of trace categories (`devtools.timeline`, `v8.execute`, `blink.user_timing`)
  is sufficient to derive the documented insights.

## Risks & Mitigations

### Risk 1: Runaway trace size or duration
- **Description**: A long recording produces a multi-megabyte trace and a long-running command.
- **Mitigation**: Enforce a maximum trace duration and an `auto_stop` on page-load completion;
  cap the raw trace size and fail fast when exceeded.

### Risk 2: Stateful lifecycle errors
- **Description**: The agent calls `stop` or `analyze` out of order.
- **Mitigation**: Return precise, actionable lifecycle errors ("no active trace", "no such
  insight") so the agent can self-correct.

### Risk 3: Trace parsing cost
- **Description**: Parsing a fifty-megabyte trace could be slow.
- **Mitigation**: Parse off the synchronous request path within the asynchronous command;
  `analyze` drill-down hits the cached insights, not the raw trace.

### Risk 4: Insight attribution accuracy
- **Description**: Script-evaluation time may be misattributed across bundled sources.
- **Mitigation**: Group by source URL from the trace's own stack frames and label aggregates
  clearly; expose the call stack through `action: "analyze"` for verification.

## Dependencies

### Depends on:
- The analyze dispatch registry (`cmd/browser-agent/tools_analyze_dispatch.go`).
- The CDP attach/detach lifecycle and `Tracing` domain (`src/background/dom/cdp/cdp-dispatch.ts`).
- The asynchronous command pattern and command-result polling
  (`cmd/browser-agent/internal/toolinteract/interact_command_builder.go`,
  `observe({what: "command_result"})`).

### Depended on by:
- AI agents debugging main-thread bottlenecks, layout shifts, and long tasks that the passive
  `observe({what: "vitals"})` path cannot attribute.

## Performance Considerations

| Metric | Target | Implementation notes |
|--------|--------|---------------------|
| Trace recording window | Seconds to tens of seconds | Bounded by `auto_stop` and a maximum duration |
| Raw trace size | 10-50MB | Never returned to the agent; parsed in the daemon |
| Summary response size | < 5KB typical | Insights and main-thread breakdown only |
| `analyze` drill-down | < 100ms | Reads cached insights, no re-parse |

## Security Considerations

**Data captured**: Timing and main-thread activity for the tracked page: long tasks, layout
shifts, script-evaluation time, and rendering-pipeline timing. Trace categories are limited to
timeline, V8 execution, and user timing; no form values or storage contents are captured.

**Data path**: All trace data and derived insights stay on localhost (`127.0.0.1`). Nothing is
transmitted externally. Optional raw-trace file save writes only to a user-specified local path.

**Attack surface**: The mode attaches the debugger (already granted) and enables the `Tracing`
domain. It records activity; it does not execute page-supplied code or modify page state. A
`reload` re-triggers the page's own network requests, equivalent to a user reload.

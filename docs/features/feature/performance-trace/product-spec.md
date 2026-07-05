---
feature: performance-trace
status: proposed
version: null
tool: analyze
mode: performance_trace
authors: []
created: 2026-03-06
updated: 2026-03-06
doc_type: product-spec
feature_id: feature-performance-trace
last_reviewed: 2026-07-05
---

# Performance Trace

> Start/stop CDP performance traces and return structured insights for AI consumption.

## Problem

Kaboom captures Web Vitals and navigation timing passively, but cannot produce a detailed performance trace — the kind of data you get from Chrome DevTools Performance panel. A trace reveals exactly what's happening on the main thread: long tasks, layout thrashing, forced reflows, script evaluation bottlenecks, and rendering pipeline stalls.

Chrome DevTools MCP exposes this as `performance_start_trace` / `performance_stop_trace` / `performance_analyze_insight`. This is a gap for Kaboom users who need deep performance debugging without installing a second MCP server.

## Solution

Add `analyze(what="performance_trace")` with an `action` parameter to control the trace lifecycle:

- `action="start"` — Begin recording a performance trace via CDP `Tracing.start`
- `action="stop"` — Stop recording and return structured insights
- `action="analyze"` — Drill into a specific insight from the trace results

This keeps the entire workflow within a single `what` mode, consistent with Kaboom's dispatch architecture.

## MCP Interface

**Tool:** `analyze`
**Mode:** `performance_trace`

### Start Trace

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "performance_trace",
    "action": "start",
    "reload": true
  }
}
```

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `action` | string | yes | -- | `"start"` to begin tracing |
| `reload` | boolean | no | `true` | Reload the page after tracing starts (captures full page load) |
| `auto_stop` | boolean | no | `true` | Automatically stop after page load completes |

### Stop Trace

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "performance_trace",
    "action": "stop"
  }
}
```

Response:

```json
{
  "performance_trace": {
    "duration_ms": 4200,
    "url": "https://example.com",
    "summary": {
      "total_blocking_time_ms": 450,
      "long_tasks": 8,
      "layout_shifts": 3,
      "forced_reflows": 2,
      "script_eval_ms": 1200,
      "render_ms": 380,
      "idle_ms": 2620
    },
    "insights": [
      {
        "id": "insight-1",
        "name": "LongTask",
        "severity": "high",
        "title": "Long task in app.bundle.js (320ms)",
        "description": "Script evaluation blocked the main thread for 320ms",
        "start_ms": 850,
        "duration_ms": 320
      },
      {
        "id": "insight-2",
        "name": "LayoutShift",
        "severity": "moderate",
        "title": "Layout shift at 1.2s (CLS contribution: 0.08)",
        "description": "Image without explicit dimensions caused layout shift",
        "start_ms": 1200,
        "cls_contribution": 0.08
      }
    ],
    "top_bottlenecks": [
      "Script evaluation: 1,200ms (29% of trace)",
      "Rendering: 380ms (9% of trace)",
      "8 long tasks totaling 1,450ms"
    ]
  }
}
```

### Analyze Insight

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "performance_trace",
    "action": "analyze",
    "insight_id": "insight-1"
  }
}
```

Returns detailed breakdown of a specific insight (call stack, affected elements, timing breakdown).

## Implementation Approach

### Extension Side (CDP)

Uses the existing `chrome.debugger` infrastructure from `cdp-dispatch.ts`:

1. **Start:** `chrome.debugger.attach` + `Tracing.start` with categories: `devtools.timeline`, `v8.execute`, `blink.user_timing`
2. **Collect:** Listen for `Tracing.dataCollected` events, accumulate trace chunks
3. **Stop:** `Tracing.end`, wait for `Tracing.tracingComplete`, detach debugger
4. **Analyze:** Parse trace events in the extension (or send raw to daemon for parsing)

### Go Side (Daemon)

- New sub-handler in `tools_analyze.go` for `what="performance_trace"`
- `action="start"` dispatches async command to extension to begin tracing
- `action="stop"` dispatches async command to stop and return trace data
- Daemon processes raw trace events into structured insights:
  - Long tasks (>50ms main thread blocks)
  - Layout shifts with CLS contribution
  - Forced reflows (style recalculation after DOM mutation)
  - Script evaluation time by source URL
  - Rendering pipeline breakdown (paint, composite, layout)
- Returns token-efficient summary (raw traces are 10-50MB; we return <5KB of insights)

### Trace Storage

- Raw trace data optionally saved to file (like Chrome DevTools MCP's `filePath` parameter)
- Structured insights cached in memory for `action="analyze"` drill-down
- Cleared on next `action="start"` or session end

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Start a CDP performance trace on the tracked tab | must |
| R2 | Stop trace and return structured insights | must |
| R3 | Identify long tasks with source attribution | must |
| R4 | Identify layout shifts with CLS contribution | must |
| R5 | Provide main thread time breakdown (script/render/idle) | must |
| R6 | Support page reload on trace start | should |
| R7 | Support auto-stop after page load | should |
| R8 | Support drill-down into specific insights | should |
| R9 | Optionally save raw trace to file | could |
| R10 | Token-efficient response (<5KB typical) | must |

## Non-Goals

- Not a real-time profiler. This is start/stop trace analysis.
- Not replacing `observe(what="vitals")` — that provides passive, always-on Web Vitals. This provides deep, on-demand trace analysis.
- Not parsing the full trace format. We extract the high-value insights an AI agent can act on.

## Dependencies

- `debugger` permission (already in manifest)
- CDP `Tracing` domain
- `cdp-dispatch.ts` attach/detach lifecycle (shipped)
- Async command infrastructure (shipped)

## Competitive Context

Chrome DevTools MCP exposes this as three separate tools (`performance_start_trace`, `performance_stop_trace`, `performance_analyze_insight`). Kaboom consolidates into a single `what` mode with `action` dispatch — consistent architecture, fewer init tokens.

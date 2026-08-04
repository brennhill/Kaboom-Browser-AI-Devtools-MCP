---
feature: performance-trace
status: shipped
version: 0.9.0
tool: analyze
mode: performance_trace
authors: []
created: 2026-03-06
updated: 2026-08-04
doc_type: product-spec
feature_id: feature-performance-trace
last_reviewed: 2026-08-04
---

# Performance Trace

## Problem

Web Vitals identify that a page is slow but do not expose the complete main-thread timeline,
JavaScript samples, rendering work, and task relationships needed for a CPU flamechart. Returning
that raw trace inline would also overwhelm an MCP response.

## Interface

Start recording:

```json
{"what":"performance_trace","action":"start","tab_id":42,"reload":true,"cache":"cold"}
```

Stop and publish the artifact:

```json
{"what":"performance_trace","action":"stop"}
```

The stop response contains:

```json
{
  "trace_id": "random-local-id",
  "artifact_path": "/local/state/performance-traces/cpu-...json",
  "event_count": 12345,
  "chunk_count": 42,
  "bytes": 987654,
  "import_with": "Chrome DevTools Performance panel or https://ui.perfetto.dev"
}
```

`action` is required. `tab_id`, `reload`, `cache` (`warm` or `cold`), and the standard
`background` command option are optional. Reload is never implicit. Both start and stop results
report the resolved tab ID, current URL, Chrome navigation/loader ID, and the application's build
SHA when it exposes one through a standard build meta/global marker; unavailable values are
reported explicitly rather than omitted.

## Requirements

1. Capture the full DevTools CPU profiling and timeline category set required for a flamechart.
2. Preserve every received trace event in an importable local JSON artifact.
3. Stream bounded event batches rather than returning a large trace through MCP.
4. Reject concurrent starts, out-of-order chunks, stale trace identifiers, and wrong-tab stops.
5. Surface debugger detachments, trace data loss, timeouts, and local storage failures.
6. Remove partial artifacts when a start or stop fails.
7. Never transmit trace content outside the local daemon.
8. Target background tabs by stable tab ID and retain that identity across reload/navigation.

## Non-goals

- Deriving an opinionated performance score or summarized insight model.
- Replacing passive Web Vitals.
- Automatically reloading or mutating the page.
- Supporting multiple simultaneous traces in one daemon.

---
doc_type: feature_index
feature_id: feature-performance-trace
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-08
code_paths:
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/performance_trace.go
  - cmd/browser-agent/internal/cli/parser/observe_analyze.go
  - cmd/browser-agent/server.go
  - internal/perftrace/http.go
  - internal/perftrace/manager.go
  - internal/perftrace/wire_trace.go
  - internal/schema/analyze.go
  - internal/state/paths.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
  - src/background/commands/analyze.ts
  - src/background/commands/helpers.ts
  - src/background/dom/cdp/performance-trace.ts
  - src/types/runtime/queries.ts
  - src/types/wire/wire-performance-trace.ts
test_paths:
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/performance_trace_test.go
  - cmd/browser-agent/composition_test.go
  - internal/perftrace/http_test.go
  - internal/perftrace/manager_test.go
  - tests/extension/performance-trace/performance-trace.test.js
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Performance Trace

## TL;DR

- Status: shipped
- Tool: `analyze`
- Mode: `performance_trace`
- Actions: `start`, `stop`

## Overview

Performance Trace records the tracked tab with Chrome's tracing backend and preserves the full
CPU flamechart event stream as a local JSON artifact. The result is importable through Chrome
DevTools' Performance panel or Perfetto. Large traces are streamed to the daemon in ordered,
bounded batches and are never returned inline through MCP.

Use `analyze({"what":"performance_trace","action":"start"})`, reproduce the slow behavior,
then call `analyze({"what":"performance_trace","action":"stop"})`. The stop response includes
the artifact path, byte count, event count, and chunk count. The passive
`observe({"what":"vitals"})` mode remains the lower-cost choice for continuous metrics.

Captured trace data remains local. It is not included in anonymous product-usage telemetry.

Trace artifacts have a five-minute and 512 MiB hard ceiling. A new MV3 worker
atomically recovers any orphaned partial trace before starting, and that
recovery is retained in the local Doctor diagnostic timeline.

Chrome debugger startup failures identify the exact attach, domain-enable,
tracing, navigation, or metadata stage. This keeps target and permission
failures actionable without recording page data in diagnostics.

## Specs

- [Product spec](./product-spec.md)
- [Technical spec](./tech-spec.md)
- [QA plan](./qa-plan.md)

## Requirement IDs

- FEATURE_PERFORMANCE_TRACE_001
- FEATURE_PERFORMANCE_TRACE_002
- FEATURE_PERFORMANCE_TRACE_003

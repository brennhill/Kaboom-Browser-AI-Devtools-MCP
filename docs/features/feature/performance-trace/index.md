---
doc_type: feature_index
feature_id: feature-performance-trace
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - cmd/browser-agent/tools_analyze_dispatch.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
  - internal/performance/diff.go
  - internal/tools/observe/session.go
  - src/background/dom/cdp/cdp-dispatch.ts
  - src/background/commands/analyze.ts
test_paths: []
last_verified_version: 0.8.6
last_verified_date: 2026-06-29
---

# Performance Trace

## TL;DR

- Status: proposed
- Tool: analyze
- Mode/Action: performance_trace (action: start | stop | analyze)
- Location: `docs/features/feature/performance-trace`

## Overview

Performance Trace adds `analyze({what: "performance_trace"})`, a mode that records a Chrome
DevTools Protocol (CDP) performance trace of the tracked tab and returns structured insights:
long tasks, layout shifts, forced reflows, script-evaluation time by source, and a main-thread
time breakdown. An `action` parameter controls the lifecycle: `start` begins recording, `stop`
ends it and returns insights, and `analyze` drills into a specific insight.

This is the on-demand, deep counterpart to the passive `observe({what: "vitals"})` telemetry.
Web Vitals are captured continuously; a performance trace reveals exactly what blocks the main
thread during a recording window. Raw traces are ten to fifty megabytes, so the daemon distills
them into a sub-five-kilobyte summary the agent can act on.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_PERFORMANCE_TRACE_001
- FEATURE_PERFORMANCE_TRACE_002
- FEATURE_PERFORMANCE_TRACE_003

## Related Code

- Analyze dispatch registry: `cmd/browser-agent/tools_analyze_dispatch.go`
- Mode hints and parameter specs: `internal/tools/configure/capabilities/modespecs_analyze.go`
- Performance metrics and Web Vitals: `internal/performance/diff.go`
- Passive vitals counterpart: `internal/tools/observe/session.go` (GetWebVitals)
- CDP attach/detach lifecycle: `src/background/dom/cdp/cdp-dispatch.ts`

## Code and Tests

Add concrete implementation and test links here as this feature evolves.

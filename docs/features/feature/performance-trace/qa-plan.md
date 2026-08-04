---
status: shipped
scope: feature/performance-trace/qa
ai-priority: high
tags: [testing, qa, performance]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-08-04
doc_type: qa-plan
feature_id: feature-performance-trace
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# QA Plan: Performance Trace

## Automated coverage

| Area | Verification |
|---|---|
| Artifact format | Multiple batches produce valid `traceEvents` JSON in original order. |
| Atomic validation | One invalid event rejects its entire batch without a partial append. |
| Lifecycle | Start, chunk, finish, and abort HTTP contracts return explicit statuses. |
| Bounds | Oversized request bodies are rejected with HTTP 413. |
| Concurrency | Concurrent start, stale ID, out-of-order sequence, and wrong-tab stop fail. |
| Chrome lifecycle | Attach, start, data collection, end, completion, and detach are deterministic. |
| Targeting | An explicit background `tab_id` is used for every CDP command without activating the tab. |
| Reload/cache | Cold cache is disabled and cleared after tracing begins and before the target reload. |
| Attribution | Start and stop report tab, URL, navigation ID, and bounded build SHA values. |
| Failure recovery | Debugger detach removes the partial artifact and returns an actionable error. |
| Data integrity | Chrome's `dataLossOccurred` flag fails the capture instead of publishing incomplete evidence. |
| Wire contract | Generated Go/TypeScript fields pass `make check-wire-drift`. |

## Connected UAT

1. Track a normal HTTP or HTTPS tab with the extension connected.
2. Call `analyze({"what":"performance_trace","action":"start","tab_id":ID,"reload":true,"cache":"cold"})`.
3. Exercise page load and the CPU-intensive interaction to investigate.
4. Call `analyze({"what":"performance_trace","action":"stop"})`.
5. Confirm the returned path exists and contains a non-empty `traceEvents` array.
6. Import that JSON into Chrome DevTools Performance or Perfetto and confirm the CPU flamechart,
   main-thread tasks, and JavaScript samples render.
7. Repeat start while active, stop without start, close the tab mid-trace, and open DevTools before
   start; confirm every failure is explicit and no `.partial` artifact remains.

## Privacy checks

- Monitor outbound requests and confirm trace chunks only reach the configured local daemon.
- Confirm product telemetry contains command identifier/outcome only, never trace events, URLs,
  artifact paths, or page content.
- Confirm artifacts are mode 0600 and stored below the local Kaboom state root.

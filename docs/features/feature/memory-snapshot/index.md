---
doc_type: feature_index
feature_id: feature-memory-snapshot
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
  - src/background/dom/cdp/cdp-dispatch.ts
  - src/background/commands/analyze.ts
  - cmd/browser-agent/internal/toolinteract/interact_dom.go
test_paths: []
last_verified_version: 0.8.6
last_verified_date: 2026-06-29
---

# Memory Snapshot

## TL;DR

- Status: proposed
- Tool: analyze
- Mode/Action: memory_snapshot
- Location: `docs/features/feature/memory-snapshot`

## Overview

Memory Snapshot adds `analyze({what: "memory_snapshot"})`, a mode that captures a JavaScript
heap snapshot through the Chrome DevTools Protocol (CDP) `HeapProfiler` domain and returns a
structured, token-efficient analysis instead of a raw binary dump. The daemon parses the
snapshot once, caches it by `snapshot_id`, and serves many targeted analyses against the same
cached graph: a summary, detached Document Object Model (DOM) node analysis, retainer-chain
tracing, two-snapshot leak diffing, string and closure analysis, and full or raw export.

The guiding principle is "capture once, analyze many ways." Graph traversal, aggregation, and
diffing run in Go where compute is cheap, and the daemon returns only conclusions rather than
the megabytes of raw heap data that would otherwise consume the agent's context window.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_MEMORY_SNAPSHOT_001
- FEATURE_MEMORY_SNAPSHOT_002
- FEATURE_MEMORY_SNAPSHOT_003

## Related Code

- Analyze dispatch registry: `cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go`
- Mode hints and parameter specs: `internal/tools/configure/capabilities/modespecs_analyze.go`
- CDP attach/detach lifecycle: `src/background/dom/cdp/cdp-dispatch.ts`
- Asynchronous command builder: `cmd/browser-agent/internal/toolinteract/interact_dom.go`

## Code and Tests

Add concrete implementation and test links here as this feature evolves.

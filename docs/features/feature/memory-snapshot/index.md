---
doc_type: feature_index
feature_id: feature-memory-snapshot
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - cmd/browser-agent/tools_analyze_dispatch.go
  - internal/tools/configure/mode_specs_analyze.go
  - src/background/cdp-dispatch.ts
  - src/background/commands/analyze.ts
  - cmd/browser-agent/internal/interacthandler/interact_command_builder.go
test_paths: []
last_verified_version: 0.8.4
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

- Analyze dispatch registry: `cmd/browser-agent/tools_analyze_dispatch.go`
- Mode hints and parameter specs: `internal/tools/configure/mode_specs_analyze.go`
- CDP attach/detach lifecycle: `src/background/cdp-dispatch.ts`
- Asynchronous command builder: `cmd/browser-agent/internal/interacthandler/interact_command_builder.go`

## Code and Tests

Add concrete implementation and test links here as this feature evolves.

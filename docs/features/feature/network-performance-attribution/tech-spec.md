---
doc_type: tech-spec
feature_id: feature-network-performance-attribution
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Technical Specification

`PerformanceResourceTiming` remains the timing source of truth. The parser computes non-negative
phase deltas and copies browser-provided protocol, delivery type, response status, sizes, and
Server-Timing. A bounded 200-entry attribution queue records the synchronous application stack at
fetch/XHR invocation and joins safe response metadata to the corresponding timing by normalized URL.

Stacks are capped at twelve frames and internal Kaboom frames are removed. Original `src`, TypeScript,
TSX, JSX, or JavaScript source frames are labeled `mapped_or_source`; other frames remain explicitly
`browser_stack`. Function-name heuristics derive optional React, loader, and store hints.

Duplicate grouping is deterministic: equal URLs are grouped only when their start/end windows
overlap. The group ID hashes the URL and earliest start time without exposing additional data.
Go and TypeScript use the generated wire contract in `wire_network.go`.

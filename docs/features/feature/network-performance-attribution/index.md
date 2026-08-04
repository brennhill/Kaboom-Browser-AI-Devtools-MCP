---
doc_type: feature_index
feature_id: feature-network-performance-attribution
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-04
code_paths:
  - internal/types/network.go
  - internal/types/wire_network.go
  - internal/tools/observe/network.go
  - src/lib/net/network.ts
  - src/lib/net/request-attribution.ts
  - src/types/wire/wire-network.ts
test_paths:
  - internal/capture/network_waterfall_test.go
  - internal/tools/observe/network_test.go
  - tests/extension/network-http/network-waterfall.test.js
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Network Performance Attribution

## TL;DR

`observe({"what":"network_waterfall"})` reports detailed browser timing phases, transport/cache
metadata, safe correlation headers, bounded initiator stacks, semantic caller hints, and duplicate
concurrent-request groups. Missing browser evidence is omitted rather than reported as zero.

## Capabilities

- Queueing, DNS, connect, TLS, TTFB, and download durations
- Protocol, cache source, compression ratio, response status, and Server-Timing
- Bounded `x-request-id` and `traceparent` correlation values
- Browser-provided/original-source stack frames with route-loader, React-component, and store-action hints
- Stable grouping for identical requests whose execution windows overlap

Captured URLs, stacks, and correlation values remain local and never enter product telemetry.

## Specs

- [Product specification](./product-spec.md)
- [Technical specification](./tech-spec.md)
- [QA plan](./qa-plan.md)

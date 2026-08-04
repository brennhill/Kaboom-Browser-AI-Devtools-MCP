---
doc_type: qa-plan
feature_id: feature-network-performance-attribution
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# QA Plan

Automated tests verify:

- Every timing phase is derived from controlled Resource Timing fixtures.
- Protocol, status, cache source, compression, and Server-Timing survive the Go/TypeScript boundary.
- Missing phase/status evidence is omitted rather than serialized as zero.
- Bounded stacks produce backed semantic caller hints.
- Identical overlapping requests share a stable duplicate group; sequential requests do not.
- Request IDs and trace context are length-bounded and sensitive headers remain excluded.
- Wire drift, folder/file limits, TypeScript compilation, lint, and duplicate-code gates pass.

Connected UAT should issue two simultaneous identical fetches from different named callers, then
confirm both callers and one duplicate group appear in `observe(network_waterfall)` alongside the
browser timing phases.

---
doc_type: product-spec
feature_id: feature-network-performance-attribution
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Product Specification

## Goal

Make a slow or duplicated request explainable without requiring DevTools: show where time was
spent, how the response was delivered, which application caller initiated it, and whether an
identical request overlapped it.

## Contract

Full waterfall entries add evidence only when it is available. Duration fields use milliseconds.
`source_map_status` distinguishes original/mapped-looking browser frames from generated browser
stacks. Semantic caller fields are best-effort hints backed by the returned stack, not assertions.

Duplicate groups require the same normalized URL and overlapping timing windows. Sequential cache
revalidation is not mislabeled as concurrent duplication.

## Privacy

Stack frames, URLs, request identifiers, and trace context are captured locally. Header capture is
allowlisted to Server-Timing, `x-request-id`, `traceparent`, content encoding, and status; credentials
and general request/response headers are not added to the waterfall.

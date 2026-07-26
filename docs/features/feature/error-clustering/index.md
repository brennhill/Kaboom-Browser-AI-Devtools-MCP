---
doc_type: feature_index
feature_id: feature-error-clustering
status: superseded
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths: []
test_paths: []
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **Removed 2026-07-26 — dead code, never reachable.**
>
> Error clustering was never wired to an MCP surface. The package had zero importers outside its own tests, was absent from `go list -deps ./cmd/browser-agent/...`, and `ClusterErrors` existed nowhere else in the tree — so nothing was clustering anything at runtime. Removed rather than left as `status: shipped`, which it never was.


# Error Clustering

## TL;DR

- Status: shipped
- Tool: observe
- Mode/Action: error_clusters
- Location: `docs/features/feature/error-clustering`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ERROR_CLUSTERING_001
- FEATURE_ERROR_CLUSTERING_002
- FEATURE_ERROR_CLUSTERING_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.

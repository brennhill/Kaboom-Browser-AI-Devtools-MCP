---
doc_type: feature_index
feature_id: feature-error-clustering
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - internal/tools/observe/errorcluster/cluster.go
  - internal/tools/observe/errorcluster/normalize.go
  - internal/tools/observe/logs/logs.go
  - internal/types/wire_log.go
  - internal/schema/analyze.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - src/background/caches/error-groups.ts
  - src/lib/storage/recovery.ts
  - src/lib/storage/validated.ts
test_paths:
  - internal/tools/observe/errorcluster/cluster_test.go
  - internal/tools/observe/errorcluster/normalize_test.go
  - tests/extension/state-recovery/state-recovery-contract.test.js
  - tests/extension/state-recovery/validated-storage.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-07-26
---

> **Correction, 2026-07-26.** An earlier revision of this file marked the feature
> `superseded` and claimed it "was never wired to an MCP surface." That was wrong.
> `analyze({what: "error_clusters"})` is live — advertised in `internal/schema/analyze.go`
> and dispatched at `cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go`.
>
> What was actually deleted was `internal/analysis/clustering`, a *second, unused*
> implementation: a stateful `ClusterManager` whose `AddError` no ingest path ever called.
> Deleting it was correct; describing the whole feature as dead was not.
>
> A second correction: the live path was also described as "0.0% covered." That was a
> per-package measurement artifact — `AnalyzeErrors` measures 100% once
> `cmd/browser-agent/tools_observe_coverage_test.go` is counted with `-coverpkg`. What was
> genuinely missing was not coverage but a *case*: the existing test clustered two
> byte-identical messages, so nothing ever exercised whether siblings differing by an
> embedded id collapse. They did not, and that is the defect fixed here.

# Error Clustering

## TL;DR

- Status: shipped
- Tool: analyze
- Mode/Action: `error_clusters`
- Location: `docs/features/feature/error-clustering`

## How it works

`analyze({what: "error_clusters"})` reads the log buffer and groups `level: "error"`
entries into clusters.

Entries are fingerprinted by **normalized message**: uuids, http(s) urls, ISO-8601
timestamps and numeric ids (3+ digits) are replaced with `{uuid}`, `{url}`,
`{timestamp}` and `{id}` placeholders, then capped at 100 characters. Two errors
differing only by an embedded identifier therefore share a cluster —
`…at /users/12345` and `…at /users/67890` are one bug seen twice, not two findings.

Results are ordered largest cluster first, ties broken by fingerprint. Each cluster's
`urls` list is sorted and deduplicated. The `message` field carries the first raw
message seen, as the human-readable representative of the group.

### Response shape

| Field | Meaning |
| --- | --- |
| `message` | First raw message seen in the cluster |
| `count` | Number of instances |
| `first_seen` / `last_seen` | Timestamps of the first and most recent instance |
| `urls` | Sorted, deduplicated page urls where the error occurred |
| `stack_trace` | Stack from the first instance, when present |

### Normalization performance

Normalization is a single hand-written byte scan rather than the four sequential
regex passes it replaces. The regex form cost ~3.1µs and 13 allocations per message,
which at the 10,000-entry buffer cap turned one `analyze` call into ~28ms of regex
work; the scanner costs ~380ns and returns untouched messages with zero allocations.

The regex version is retained verbatim in `normalize_test.go` as `referenceNormalize`
and pinned to the scanner by a 200,000-case differential test. It is the readable
statement of intent — **edit the reference first**, then make the scanner match it.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ERROR_CLUSTERING_001
- FEATURE_ERROR_CLUSTERING_002
- FEATURE_ERROR_CLUSTERING_003

## Code and Tests

| Path | Role |
| --- | --- |
| `internal/tools/observe/errorcluster/cluster.go` | Grouping rules and deterministic response ordering |
| `internal/tools/observe/errorcluster/normalize.go` | Message fingerprint normalizer (byte scanner) |
| `internal/tools/observe/logs/logs.go` | `AnalyzeErrors` entry point |
| `internal/schema/analyze.go` | Advertises `error_clusters` in the tool schema |
| `cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go` | Routes `error_clusters` to `AnalyzeErrors` |
| `internal/tools/observe/errorcluster/cluster_test.go` | Grouping, ordering, determinism |
| `internal/tools/observe/errorcluster/normalize_test.go` | Table cases + differential test vs regex reference |

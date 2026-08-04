---
doc_type: tech-spec
feature_id: feature-operational-observability
last_reviewed: 2026-08-04
---

# Operational Observability — Technical Specification

## Boundaries

- `internal/incident` owns incident definitions, lifecycle transitions, bounded storage, Doctor projections, fingerprints, and support artifacts.
- `internal/statediag` collects redacted state-loader diagnostics.
- `internal/telemetry` owns the allowlisted external projection and bounded asynchronous delivery.
- `cmd/browser-agent/tools_configure_support.go` exposes support preview/export through `configure`.
- `internal/schema/configure/properties_core.go` defines the public tool schema.

## Invariants

- Registry metadata, not callers, defines telemetry classification.
- Local evidence never enters the telemetry projection.
- State transitions are idempotent and generation-aware.
- Stores and queues have fixed capacity, single-pass eviction, and visible dropped counts.
- The telemetry dispatcher executes outside lifecycle locks and recovers transport panics.
- Support export requires the current content-derived preview token.
- Any incident transition invalidates an outstanding support preview.

## Privacy Boundary

External telemetry is limited to random install/session identifiers, version/platform, command identifiers, outcomes, timing, aggregate counts, and fixed incident classifications. All browser and user data remains local.

## Change Boundary

New operational failures start in the canonical incident registry, then add Doctor and telemetry projections with privacy-contract tests. Parallel reporting paths are removed atomically.

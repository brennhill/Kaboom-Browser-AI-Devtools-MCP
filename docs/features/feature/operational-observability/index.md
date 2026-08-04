---
doc_type: feature_index
feature_id: feature-operational-observability
status: in_progress
feature_type: feature
owners: []
last_reviewed: 2026-08-04
code_paths:
  - internal/incident/registry.go
  - internal/incident/store.go
  - internal/incident/projections.go
  - internal/statediag/collector.go
  - internal/telemetry/beacon.go
test_paths:
  - internal/incident/store_test.go
  - internal/statediag/collector_test.go
  - internal/telemetry/contract_compliance_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Operational Observability

Kaboom models operational failures once as typed local incidents. The canonical
incident owns its stable code, subsystem, lifecycle stage, severity,
retryability, correlation, connection generation, bounded transition history,
and redacted local evidence.

Doctor and analytics are separate projections of that incident:

- Doctor combines registry-owned human guidance with local-only evidence and
  correlation context.
- Analytics contains only allowlisted fixed codes and bounded classifications.
  It cannot contain local evidence, correlation IDs, generations, URLs, paths,
  captured data, or arbitrary caller-provided strings.

Recovery follows an idempotent, generation-aware state machine. Stale
transitions cannot alter current health. Incident storage and history are
bounded, use single-pass eviction, and expose dropped-entry counts rather than
silently losing pressure signals.

Migration is performed one ownership boundary at a time and remains atomic
within that boundary: callers move completely to the canonical incident and its
projections, and their obsolete parallel reporting calls are deleted together.

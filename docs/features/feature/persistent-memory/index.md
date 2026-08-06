---
doc_type: feature_index
feature_id: feature-persistent-memory
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - cmd/browser-agent/server.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolconfigure/session.go
  - internal/persistence/persistence_context.go
  - internal/persistence/persistence_crud.go
  - internal/persistence/persistence_dirty.go
  - internal/persistence/persistence_store.go
  - internal/persistence/persistence_types.go
  - internal/statefile/statefile.go
  - internal/statediag/collector.go
  - internal/tools/configure/capabilities/modespecs_configure.go
test_paths:
  - cmd/browser-agent/tools_interact_helpers_test.go
  - cmd/browser-agent/tools_interact_state_test.go
  - cmd/browser-agent/tools_test_helpers_test.go
  - tests/architecture/user-state-loaders.test.cjs
  - internal/persistence/persistence_branches_test.go
  - internal/statediag/collector_test.go
  - cmd/browser-agent/tools_configure_handler_test.go
  - cmd/browser-agent/tools_configure_persistence_actions_test.go
  - cmd/browser-agent/tools_configure_capabilities_test.go
  - cmd/browser-agent/tools_configure_audit_test.go
  - cmd/browser-agent/tools_stdio_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Persistent Memory

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: store, load, record_event
- Location: `docs/features/feature/persistent-memory`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_PERSISTENT_MEMORY_001
- FEATURE_PERSISTENT_MEMORY_002
- FEATURE_PERSISTENT_MEMORY_003

## Code and Tests

- `cmd/browser-agent/internal/toolconfigure/session.go` owns the canonical store request contract: `store_action`, `namespace`, `key`, and `data`.
- `internal/tools/configure/capabilities/modespecs_configure.go` exposes the same canonical parameters in capability metadata.
- `cmd/browser-agent/tools_configure_persistence_actions_test.go` and `cmd/browser-agent/tools_configure_capabilities_test.go` cover store behavior and its advertised contract.
- `cmd/browser-agent/tools_configure_audit_test.go` and `cmd/browser-agent/tools_stdio_test.go` exercise the canonical store request through action and stdout-purity gates.
- Project metadata and context loaders distinguish normal first-run absence from
  read or parse failures. Failures activate bounded defaults and publish
  redacted, actionable diagnostics through the canonical state-recovery
  collector consumed by System Doctor.
- Immediate session and metadata writes use the canonical durable state-file
  boundary: same-directory temporary write, file sync, atomic rename, and
  directory sync where supported. Pre-rename failures preserve the previous
  durable value, publish a value-free Doctor incident, and a successful retry
  resolves that incident.
- Deferred writes own their queued bytes and retain failed obligations for the
  next bounded flush without overwriting a newer concurrently queued value.
  Write and shutdown failures activate redacted Doctor incidents; a complete
  retry resolves the deferred-write incident.
- One injectable session-filesystem boundary owns metadata, CRUD, context,
  statistics, quota, and restart I/O. Deterministic read, directory, delete,
  quota, corruption, and restart fixtures prove fallback behavior without
  touching real user state. Operational failures return stable value-free
  errors and activate Doctor incidents; expected optional-state absence is
  explicitly classified and resolves earlier incidents.
- CRUD and path validation are one security boundary in
  `persistence_crud.go`. Context loading, quota measurement, and statistics are
  read-only filesystem projections owned by `persistence_context.go`. This
  keeps the package at ten files while preserving separate dirty-flush,
  request-dispatch, and store-lifecycle owners.
- Each server resolves its session project root once, and tool handlers inherit
  that explicit root. Stateful test factories replace it with an isolated
  temporary project before handler construction, preventing parallel tests from
  racing over developer state or each other's quota scans.
- Persisted page-state tests answer capture commands through the canonical
  pending-query notification barrier. Sensitive-value and legacy-shape
  redaction coverage therefore cannot race command creation, and responder
  serialization or missing-command failures are surfaced explicitly.

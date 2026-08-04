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
  - internal/telemetry/beacon_test.go
  - tests/architecture/user-state-loaders.test.cjs
  - tests/cli/contracts/packaged-recovery-uat.test.cjs
  - scripts/tests/release/cat-34-packaged-corruption-recovery.sh
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

The allowed graph is explicit: incidents may resolve directly from `detected`
to `recovered` or `exhausted`; only retryable incidents may enter `retrying`.
This represents immediate success and terminal failures without inventing retry
attempts. Fatal detections are Doctor failures even before their terminal
transition. Internal identity uses the full correlation value's local SHA-256
digest; its separately redacted and bounded display form cannot merge distinct
incidents. Doctor history derives each row's outcome from that transition and
reserves `recovered_at` for actual recovery.

The installation-identity recovery boundary is the first complete production
migration. Its failures enter the canonical store, appear through the Doctor
projection, and emit only the allowlisted initial `app_error` projection. The
obsolete `install_identity_state` calls into `statediag` were removed together.
Telemetry projection uses one fixed-capacity asynchronous dispatcher outside
lifecycle locks, so install-ID loading cannot recursively enter itself and a
failure storm cannot create unbounded goroutines. A five-minute per-code window
collapses repeated incidents without retaining correlation or payload data.
Queue saturation does not consume that window, so a dropped event can retry
after pressure drains. Payload-free diagnostics expose rate limiting,
saturation, recovered delivery panics, and pending work; a panicking transport
cannot kill the worker or strand shutdown accounting. The daemon attaches the
canonical incident store and warms identity before opening its HTTP listener,
preventing first-request initialization from bypassing recovery diagnostics.

All legacy runtime `app_error` producers now use the same closed registry.
Callers cannot provide arbitrary categories, sources, severities, retryability,
or maps; each registered code carries an explicit
`bounded_product_metadata` privacy classification, and unknown codes are
discarded before telemetry envelope construction.

Migration is performed one ownership boundary at a time and remains atomic
within that boundary: callers move completely to the canonical incident and its
projections, and their obsolete parallel reporting calls are deleted together.

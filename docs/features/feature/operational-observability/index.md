---
doc_type: feature_index
feature_id: feature-operational-observability
status: in_progress
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/capture/pressure/stats.go
  - internal/incident/registry.go
  - internal/incident/store.go
  - internal/incident/projections.go
  - internal/incident/support.go
  - internal/statediag/collector.go
  - internal/telemetry/beacon.go
  - cmd/browser-agent/tools_configure_support.go
  - internal/schema/configure/properties_core.go
test_paths:
  - internal/capture/logstore/store_test.go
  - internal/capture/accessor_unit_test.go
  - internal/incident/store_test.go
  - internal/incident/support_test.go
  - cmd/browser-agent/tools_configure_support_test.go
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

Each canonical Doctor projection also carries a 16-character local grouping
fingerprint derived only from registry-owned code, subsystem, stage, severity,
and retryability. Correlation IDs, generations, timestamps, paths, evidence,
and prose do not influence it, so recurring incident classes group stably
without encoding user or project data. The fingerprint is not sent in product
telemetry.

Support bundles are an explicit two-step local workflow on the existing
`configure` tool. `preview_support_bundle` returns the exact redacted JSON
artifact and a content-derived confirmation token. `export_support_bundle`
requires that current token and a user-selected path, then writes the same
bytes with owner-only permissions. Any incident transition invalidates the
preview. Kaboom never uploads the bundle, echoes the local path, or includes
correlation IDs, generations, timestamps, evidence, URLs, content, logs, or
recordings.

Recovery follows an idempotent, generation-aware state machine. Stale
transitions cannot alter current health. Incident storage and history are
bounded, use single-pass eviction, and expose dropped-entry counts rather than
silently losing pressure signals.

Disposable capture owners report resource pressure through the neutral
`internal/capture/pressure` value contract. Extension logs, browser telemetry,
performance samples, and health projections therefore expose consistent size,
capacity, cumulative-drop, and oldest-entry evidence without importing one
another or duplicating operational types.

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
Beacon HTTP work has separate in-flight accounting, so delivery diagnostics
can establish an exact idle boundary without inspecting or mutating the
capacity semaphore.
The analytics projection now preserves every distinct bounded lifecycle
transition: initial failure (`pending:0`), retry attempt buckets, recovery, and
exhaustion, together with a closed latency bucket. This makes recovery
effectiveness queryable by the existing version, platform, client, and
registry-owned subsystem dimensions without exporting local correlation data.

The authenticated release-health dashboard is owned by the adjacent
`kaboom-metrics` analytics boundary. It compares the newest valid stable
semantic version with its predecessor by OS, channel, and MCP client. Alerts
cover crash-free-install regression, sessions without successful commands,
restart loops, readiness/reconnect exhaustion, malformed telemetry, and new or
spiking canonical error codes. Both releases need at least 20 sessions in a
segment, failures must repeat at least three times, and existing rates must
increase by both five percentage points and 2×, which bounds low-volume noise.

All legacy runtime `app_error` producers now use the same closed registry.
Callers cannot provide arbitrary categories, sources, severities, retryability,
or maps; each registered code carries an explicit
`bounded_product_metadata` privacy classification, and unknown codes are
discarded before telemetry envelope construction.

Migration is performed one ownership boundary at a time and remains atomic
within that boundary: callers move completely to the canonical incident and its
projections, and their obsolete parallel reporting calls are deleted together.

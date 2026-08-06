---
doc_type: feature_index
feature_id: feature-app-telemetry
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - internal/telemetry/beacon.go
  - internal/telemetry/install_id.go
  - internal/telemetry/usage_counter.go
  - internal/statefault/fault.go
  - internal/statefile/statefile.go
  - cmd/browser-agent/internal/operationalapi/handler.go
  - internal/statediag/collector.go
  - internal/incident/registry.go
  - internal/incident/store.go
  - internal/incident/projections.go
test_paths:
  - tests/architecture/user-state-loaders.test.cjs
  - internal/telemetry/beacon_test.go
  - internal/telemetry/contract_compliance_test.go
  - internal/telemetry/e2e_reporting_test.go
  - internal/telemetry/e2e_session_test.go
  - internal/telemetry/e2e_usage_summary_test.go
  - internal/telemetry/install_id_test.go
  - internal/telemetry/usage_counter_test.go
  - cmd/browser-agent/internal/operationalapi/debug_test.go
  - internal/statediag/collector_test.go
  - internal/incident/store_test.go
  - tests/cli/contracts/uat-harness-regressions.test.cjs
  - scripts/release/install-upgrade-regression.contract.test.mjs
  - scripts/tests/contracts/app-telemetry-producers.test.mjs
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# App Telemetry

Kaboom emits privacy-bounded product telemetry using the canonical event,
identity, opt-out, and aggregation contract in
[`docs/core/quality/app-metrics.md`](../../../core/quality/app-metrics.md).

The event suite owns individual beacon envelopes and payloads. The session
identity module owns activity boundaries, shutdown, timeout, and opt-out
behavior. The usage aggregation module owns counter aggregation,
snapshot/reset semantics, scheduling, and summary beacons. These
change-coupled owners keep the package within the ten-file architecture limit.
Install identity is installation-scoped at `~/.kaboom/install_id`, independent
of project/UAT runtime-state roots, and concurrent first starts atomically
converge on one persisted value.
Malformed identity is atomically replaced once. Transient read or persistence
failure suppresses telemetry instead of inventing a process-local identity
that would inflate install counts, and System Doctor reports the redacted
recovery state.
Install identity and the first-tool-call marker share one injected filesystem
boundary. Canonical read, write, quota, cancellation, corruption, partial-write,
and restart fixtures never touch the real installation root. A marker is
cached only after its durable write succeeds; otherwise first-call telemetry is
suppressed, preventing duplicate events on the next process restart.
The operational debug endpoint reads non-destructive flat counters through the
canonical `UsageTracker.DebugCounts` API; the former compatibility-named
counter accessor is deleted.

Only the daemon emits product telemetry. Installer, uninstaller, and extension
service-worker lifecycle paths do not send independent envelopes because they
cannot satisfy the canonical install/session identity contract. The generic
lifecycle event surface is deleted; every emitted event is one of the six
documented canonical event types.

Runtime `app_error` producers accept only a canonical `incident.Code`. The
registry owns every bounded analytics dimension and privacy classification;
the former free-form category classifier and normalizer were deleted in the
same migration. Unknown codes emit nothing, and contract tests prove private
or caller-authored fields cannot enter the envelope.
Canonical retry, recovery, and exhaustion transitions use the same event with
closed outcome, attempt, and latency buckets. Rate limiting keys include all
four dimensions, so repeated identical transitions collapse while a recovery
cannot be hidden by its preceding detection. Correlation IDs, generations,
history, and local evidence never cross the analytics boundary.
Session rotation happens before accounting for the command that discovers an
inactivity timeout, so that command and its counters share the same new `sid`.
Session-end rows expose only aggregate tool/error counts and a bounded outcome;
these support failure-free session metrics while per-call rows support
failure-free command rates. Active-install and crash-free rates are derived by
distinct anonymous `iid`/`sid`, version, platform, channel, and client.

This is the sole exception to Kaboom's local-data policy. It reports anonymous
install activity and product-command usage using random install/session
identifiers plus version/platform, command identifiers, outcomes, timing, and
aggregate counts. It must never contain URLs, prompts, page or file content,
captured browser telemetry, credentials, or personal data. Users can disable
delivery with `KABOOM_TELEMETRY=off`.

`/health` and `/diagnostics` expose payload-free `telemetry_delivery` counters
for accepted `202` responses, rejected non-`202` responses, network errors,
drops, suppressions, and the last HTTP status. Its nested reliability snapshot
also reports rate-limited incidents, queue saturation, recovered delivery
panics, and current pending work. No event or identity data is included in
these diagnostics.

The asynchronous beacon transport owns explicit in-flight lifecycle
accounting. Tests and local shutdown diagnostics wait on that boundary instead
of draining semaphore tokens or sleeping; blocked-transport fixtures release
requests through channels. This makes opt-out, rejection, timeout, and no-op
producer checks deterministic without changing fire-and-forget production
delivery.

## Specifications

- [Product specification](product-spec.md)
- [Technical specification](tech-spec.md)
- [QA plan](qa-plan.md)
- [Canonical telemetry contract](../../../core/quality/app-metrics.md)

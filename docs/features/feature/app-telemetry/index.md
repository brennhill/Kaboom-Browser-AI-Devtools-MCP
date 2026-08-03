---
doc_type: feature_index
feature_id: feature-app-telemetry
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - internal/telemetry/beacon.go
  - internal/telemetry/install_id.go
  - internal/telemetry/session.go
  - internal/telemetry/usage_beacon.go
  - internal/telemetry/usage_counter.go
  - internal/statefault/fault.go
  - internal/statefile/statefile.go
  - cmd/browser-agent/internal/operationalapi/handler.go
  - internal/statediag/collector.go
test_paths:
  - tests/architecture/user-state-loaders.test.cjs
  - internal/telemetry/beacon_test.go
  - internal/telemetry/contract_compliance_test.go
  - internal/telemetry/e2e_reporting_test.go
  - internal/telemetry/e2e_session_test.go
  - internal/telemetry/e2e_usage_summary_test.go
  - internal/telemetry/install_id_test.go
  - internal/telemetry/session_test.go
  - internal/telemetry/usage_beacon_test.go
  - internal/telemetry/usage_counter_test.go
  - cmd/browser-agent/internal/operationalapi/debug_test.go
  - internal/statediag/collector_test.go
  - tests/cli/contracts/uat-harness-regressions.test.cjs
  - scripts/release/install-upgrade-regression.contract.test.mjs
  - scripts/tests/contracts/app-telemetry-producers.test.mjs
last_verified_version: 0.8.8
last_verified_date: 2026-07-28
---

# App Telemetry

Kaboom emits privacy-bounded product telemetry using the canonical event,
identity, opt-out, and aggregation contract in
[`docs/core/app-metrics.md`](../../../core/app-metrics.md).

The event suite owns individual beacon envelopes and payloads. The session
suite owns activity boundaries, shutdown, timeout, and opt-out behavior. The
usage-summary suite owns counter aggregation, snapshot/reset semantics, and
summary beacons.
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

This is the sole exception to Kaboom's local-data policy. It reports anonymous
install activity and product-command usage using random install/session
identifiers plus version/platform, command identifiers, outcomes, timing, and
aggregate counts. It must never contain URLs, prompts, page or file content,
captured browser telemetry, credentials, or personal data. Users can disable
delivery with `KABOOM_TELEMETRY=off`.

`/health` and `/diagnostics` expose payload-free `telemetry_delivery` counters
for accepted `202` responses, rejected non-`202` responses, network errors,
drops, suppressions, and the last HTTP status. No event or identity data is
included in these diagnostics.

## Specifications

- [Product specification](product-spec.md)
- [Technical specification](tech-spec.md)
- [QA plan](qa-plan.md)
- [Canonical telemetry contract](../../../core/app-metrics.md)

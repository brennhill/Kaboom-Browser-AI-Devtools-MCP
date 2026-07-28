---
doc_type: feature_index
feature_id: feature-app-telemetry
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - internal/telemetry/beacon.go
  - internal/telemetry/install_id.go
  - internal/telemetry/session.go
  - internal/telemetry/usage_beacon.go
  - internal/telemetry/usage_counter.go
test_paths:
  - internal/telemetry/e2e_reporting_test.go
  - internal/telemetry/e2e_session_test.go
  - internal/telemetry/e2e_usage_summary_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# App Telemetry

Kaboom emits privacy-bounded product telemetry using the canonical event,
identity, opt-out, and aggregation contract in
[`docs/core/app-metrics.md`](../../../core/app-metrics.md).

The event suite owns individual beacon envelopes and payloads. The session
suite owns activity boundaries, shutdown, timeout, and opt-out behavior. The
usage-summary suite owns counter aggregation, snapshot/reset semantics, and
summary beacons.

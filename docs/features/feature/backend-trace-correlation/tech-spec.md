---
doc_type: tech_spec
feature_id: feature-backend-trace-correlation
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Backend Trace Correlation Technical Specification

The extension preserves only the W3C `traceparent` request header needed for
correlation. The daemon accepts a bounded 32 MiB local JSON source in either a
compact `spans` representation or the standard OTLP `resourceSpans` export
shape. It matches exact normalized trace IDs and returns bounded span names,
service names, durations, hierarchy, and categorical totals.

The trace source is opened only for the explicit performance request. No trace
content enters external telemetry.

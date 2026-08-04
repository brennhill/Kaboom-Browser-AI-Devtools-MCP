---
doc_type: qa_plan
feature_id: feature-backend-trace-correlation
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Backend Trace Correlation QA Plan

Tests cover request-header preservation, compact and OTLP JSON sources,
traceparent matching, service/category breakdown, missing configuration,
unmatched context, malformed data, and bounded local file reads.

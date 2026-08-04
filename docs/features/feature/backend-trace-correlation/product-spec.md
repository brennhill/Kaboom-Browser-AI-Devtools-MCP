---
doc_type: product_spec
feature_id: feature-backend-trace-correlation
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Backend Trace Correlation Product Specification

Developers can connect a slow browser request to local backend spans and see
time grouped into edge, auth, application, SQL, Redis, and external-provider
categories. Missing configuration, trace context, matches, and invalid sources
are distinct states rather than empty success.

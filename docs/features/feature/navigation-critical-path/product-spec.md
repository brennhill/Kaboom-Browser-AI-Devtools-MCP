---
doc_type: product_spec
feature_id: feature-navigation-critical-path
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Navigation Critical Path Product Specification

Performance analysis identifies which evidenced segment dominates a page load
and makes missing instrumentation visible. Authentication is distinguished from
application requests; state and React phases use application User Timing marks
when present; paint milestones use browser observer evidence.

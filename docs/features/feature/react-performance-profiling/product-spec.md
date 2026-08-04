---
doc_type: product_spec
feature_id: feature-react-performance-profiling
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# React Performance Profiling Product Specification

Developers can explicitly profile a React interaction without installing a
project dependency. The result identifies costly commits and components,
changed prop names, state-reference changes, and pending Suspense boundaries.
Unsupported pages return a reason instead of an empty successful result.

Zustand subscription invalidations and application-specific data readiness are
not public React browser contracts. Kaboom reports these capabilities as
unavailable rather than guessing; Suspense is the supported readiness signal.

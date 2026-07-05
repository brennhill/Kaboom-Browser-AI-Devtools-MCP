---
status: superseded
scope: feature/dom-diffing
superseded-by: feature/perf-experimentation/tech-spec.md
doc_type: tech-spec
feature_id: feature-dom-diffing
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: DOM Diffing

## This spec has been merged into [Rich Action Results](../perf-experimentation/tech-spec.md).

DOM diffing and performance measurement converged into a single feature: action results that tell the AI everything that happened. The extension is the measurement instrument — MutationObserver for DOM changes, PerformanceObserver for timing, automatic perf diff on navigation.

See the unified spec for the full design.

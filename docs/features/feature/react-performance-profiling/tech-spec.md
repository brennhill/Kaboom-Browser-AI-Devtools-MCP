---
doc_type: tech_spec
feature_id: feature-react-performance-profiling
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# React Performance Profiling Technical Specification

`analyze({what:"react_profile", action:"start"})` targets a tab and installs a
bounded wrapper around `__REACT_DEVTOOLS_GLOBAL_HOOK__.onCommitFiberRoot` in the
page's main world. Stop restores the exact original callback before returning
evidence. Existing callbacks are invoked with their original receiver.

Each commit traverses at most 5,000 fibers. Sessions retain at most 100 commits
and 200 components. Results contain component names, timing, render counts,
changed prop keys, state-reference changes, and Suspense counts. Prop and state
values are never serialized. Dropped commits and traversal truncation are
explicit.

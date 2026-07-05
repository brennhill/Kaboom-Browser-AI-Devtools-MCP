---
doc_type: tech-spec
feature_id: feature-page-structure-detection
status: proposed
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Page Structure Detection Tech Spec

## Detection Model
- Execute in MAIN world first for high-confidence globals.
- Fallback to ISOLATED/DOM-only heuristics when MAIN world is blocked.
- Preserve existing scroll/modal/shadow summaries while expanding framework and routing inference.

## Implementation Constraints
- Bounded DOM/script scanning to keep low execution latency.
- Explicit confidence/evidence fields for heuristic outputs.
- Backward-compatible response shape evolution where possible.

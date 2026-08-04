---
doc_type: tech_spec
feature_id: feature-navigation-critical-path
status: shipped
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Navigation Critical Path Technical Specification

The model consumes the latest performance snapshot and bounded network
waterfall. It emits stable ordered phases with availability, start/end/duration,
and evidence provenance. Unavailable phases have no duration. The dominant
segment is the longest available phase, and explicit gaps name missing phase
transitions.

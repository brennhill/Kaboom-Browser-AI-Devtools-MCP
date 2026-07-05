---
feature: analyze-tool
status: shipped
tool: analyze
version: 0.7.12
doc_type: product-spec
feature_id: feature-analyze-tool
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Analyze Product Spec (TARGET)

## Purpose
Run analysis and command-driven browser inspection workflows that go beyond passive observation.

## Modes (`what`)
`dom`, `performance`, `accessibility`, `error_clusters`, `navigation_patterns`, `security_audit`, `third_party_audit`, `link_health`, `link_validation`, `page_summary`, `annotations`, `annotation_detail`, `api_validation`, `draw_history`, `draw_session`, `computed_styles`, `forms`, `form_state`, `form_validation`, `data_table`, `visual_baseline`, `visual_diff`, `visual_baselines`, `navigation`, `page_structure`, `audit`, `feature_gates`

## Behavior Model
- Sync by default.
- `background:true` (or `sync:false`/`wait:false`) returns queued handles.
- Correlation results are retrievable via `observe({what:"command_result"})`.

## Mode Classes
1. Extension-backed command modes
- `dom`, `accessibility`, `link_health`, `page_summary`, `computed_styles`, `forms`, `form_state`, `form_validation`, `data_table`, `navigation`, `page_structure`, `feature_gates`

2. Server-side analysis modes
- `performance`, `error_clusters`, `navigation_patterns`, `security_audit`, `third_party_audit`, `link_validation`, `api_validation`, `audit`

3. Annotation/session modes
- `annotations`, `annotation_detail`, `draw_history`, `draw_session`

4. Visual regression modes
- `visual_baseline`, `visual_diff`, `visual_baselines`

## Aliases
- `history` → `navigation_patterns` (quiet alias, dispatches correctly but hidden from enum)

## Requirements
- `ANALYZE_PROD_001`: `what` is required and enum-validated.
- `ANALYZE_PROD_002`: async control flags must produce deterministic queued/sync behavior.
- `ANALYZE_PROD_003`: extension-backed command modes must emit correlation IDs.
- `ANALYZE_PROD_004`: `dom` mode must remain the canonical DOM query surface.
- `ANALYZE_PROD_005`: mode-specific parameters must be validated with structured errors.

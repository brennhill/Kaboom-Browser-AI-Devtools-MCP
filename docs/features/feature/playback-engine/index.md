---
doc_type: feature_index
feature_id: feature-playback-engine
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - internal/recording/playback.go
test_paths: []
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Playback Engine

## TL;DR

- Status: proposed
- Tool: configure, observe
- Mode/Action: playback, playback_results
- Location: `docs/features/feature/playback-engine`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: tech-spec.md (planned)
- QA Plan: qa-plan.md (planned)

## Requirement IDs

- FEATURE_PLAYBACK_ENGINE_001 — Action execution (navigate, click, type, select, check, key_press, scroll, screenshot)
- FEATURE_PLAYBACK_ENGINE_002 — Timing model (fast-forward, recorded)
- FEATURE_PLAYBACK_ENGINE_003 — Self-healing selectors (7-level cascade)
- FEATURE_PLAYBACK_ENGINE_004 — Error handling (continue, skip_dependent, stop)
- FEATURE_PLAYBACK_ENGINE_005 — MCP API surface (async with playback_id)
- FEATURE_PLAYBACK_ENGINE_006 — Synthetic flows
- FEATURE_PLAYBACK_ENGINE_007 — Redacted value handling
- FEATURE_PLAYBACK_ENGINE_008 — Recording format (schema version, viewport, source tracking)
- FEATURE_PLAYBACK_ENGINE_009 — Playback prerequisites and environment checks

## Code and Tests

- Stub: `internal/capture/playback.go` (types + placeholder logic, no real browser dispatch)
- Implementation will wire `executeAction` to the existing PendingQuery/interact system

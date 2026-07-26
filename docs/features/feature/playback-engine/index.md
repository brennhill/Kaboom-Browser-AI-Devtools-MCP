---
doc_type: feature_index
feature_id: feature-playback-engine
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - internal/recording/types.go
  - internal/recording/manager.go
  - internal/recording/manager_store.go
  - internal/recording/manager_storage.go
  - internal/recording/playback/types.go
  - internal/recording/playback/session.go
  - internal/recording/playback/actions.go
  - internal/recording/playback/fragile.go
  - internal/recording/logdiff/types.go
  - internal/recording/logdiff/compare.go
  - internal/recording/logdiff/helpers.go
  - internal/recording/logdiff/report.go
  - internal/capture/recording_manager.go
  - cmd/browser-agent/recording_handlers_playback.go
  - cmd/browser-agent/recording_handlers_logdiff.go
test_paths:
  - internal/recording/manager_test.go
  - internal/recording/types_test.go
  - internal/recording/state_path_test.go
  - internal/recording/playback/playback_test.go
  - internal/recording/logdiff/logdiff_test.go
  - internal/capture/recording_delegation_test.go
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

- Recording capture, persistence, and storage quotas: `internal/recording/`
- Replay engine (session lifecycle, action execution, selector fragility): `internal/recording/playback/`
- Recording comparison and regression reporting: `internal/recording/logdiff/`
- Both engines read recordings through a one-method source interface that
  `*recording.Manager` satisfies (`LookupRecording` for replay, `GetRecording`
  for diffing), so neither depends on the manager type and both are tested
  against in-memory fakes.
- Delegation surface: `internal/capture/recording_manager.go`
- MCP handlers: `cmd/browser-agent/recording_handlers_playback.go`, `cmd/browser-agent/recording_handlers_logdiff.go`
- Still a stub: `playback.executeAction` returns synthetic results and is not yet
  wired to the PendingQuery/interact system, so replay does not drive a browser.

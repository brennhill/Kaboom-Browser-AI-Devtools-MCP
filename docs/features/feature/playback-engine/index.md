---
doc_type: feature_index
feature_id: feature-playback-engine
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
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
  - internal/capture/handlers.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolobserve/dispatcher.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolrecording/handler.go
  - cmd/browser-agent/internal/toolrecording/helpers.go
test_paths:
  - cmd/browser-agent/internal/toolrecording/handler_test.go
  - cmd/browser-agent/internal/toolrecording/toolrecording_test.go
  - cmd/browser-agent/recording_playback_result_test.go
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
- Recording persistence reads and writes only the canonical state recordings
  directory; historical storage locations are not migration inputs.
- Replay engine (session lifecycle, action execution, selector fragility): `internal/recording/playback/`
- Recording comparison and regression reporting: `internal/recording/logdiff/`
- Both engines read recordings through a one-method source interface that
  `*recording.RecordingManager` satisfies (`LookupRecording` for replay, `GetRecording`
  for diffing), so neither depends on the manager type and both are tested
  against in-memory fakes.
- Delegation surface: `internal/capture/handlers.go`
- MCP owners: `cmd/browser-agent/tools_configure.go`, `cmd/browser-agent/internal/toolobserve/dispatcher.go`, and the composition root in `tools_core.go`
- Recording and playback MCP behavior/state: `cmd/browser-agent/internal/toolrecording/`
- Still a stub: `playback.executeAction` returns synthetic results and is not yet
  wired to the PendingQuery/interact system, so replay does not drive a browser.

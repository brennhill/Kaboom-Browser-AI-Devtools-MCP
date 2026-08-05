---
doc_type: feature_index
feature_id: feature-playback-engine
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/recording/types.go
  - internal/recording/manager.go
  - internal/recording/manager_store.go
  - internal/statefile/statefile.go
  - internal/statediag/collector.go
  - internal/recording/manager_storage.go
  - internal/recording/playback/types.go
  - internal/recording/playback/session.go
  - internal/recording/playback/actions.go
  - internal/recording/playback/fragile.go
  - internal/recording/logdiff/types.go
  - internal/recording/logdiff/compare.go
  - internal/recording/logdiff/helpers.go
  - internal/recording/logdiff/report.go
  - internal/capture/capture.go
  - internal/capture/httpingest/handlers.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/internal/toolobserve/dispatcher.go
  - cmd/browser-agent/internal/toolobserve/deps.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolrecording/handler.go
  - cmd/browser-agent/internal/toolrecording/helpers.go
test_paths:
  - internal/capture/httpingest/handlers_test.go
  - tests/architecture/user-state-loaders.test.cjs
  - internal/capture/testhelpers_test.go
  - internal/capture/recording_playback_integration_test.go
  - internal/capture/recording_logdiff_integration_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/lint_hardening_test.go
  - cmd/browser-agent/internal/toolrecording/handler_test.go
  - cmd/browser-agent/internal/toolrecording/toolrecording_test.go
  - cmd/browser-agent/recording_playback_result_test.go
  - internal/recording/manager_test.go
  - internal/statediag/collector_test.go
  - internal/recording/types_test.go
  - internal/recording/state_path_test.go
  - internal/recording/playback/playback_test.go
  - internal/recording/logdiff/logdiff_test.go
  - internal/capture/recording_manager_test.go
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

- Capture integration tests install a package-owned temporary state root before
  constructing the production recording manager. Recording setup/load helpers
  fail immediately on persistence errors, preventing secondary nil/index panics.

- Recording capture, persistence, and storage quotas: `internal/recording/`
- Event-recording metadata uses one injectable filesystem boundary and the
  canonical atomic state-file writer. Deterministic read, list, write, sync,
  rename, directory-sync, quota, partial-write, and cancellation failures are
  redacted and reported through Doctor. A failed stop retains the active
  recording for retry; successful retry resolves the incident. User-authored
  recording names are normalized before they become storage identifiers.
- Recording quota scans and deletion use that same boundary. Measurement and
  cleanup failures preserve the previous in-memory quota/accounting state,
  return stable value-free errors, and keep the recording available for retry;
  successful recount or deletion resolves the Doctor incident.
- Health reports the recording count, active recording count, retained bytes,
  and byte capacity from the manager's in-memory accounting without performing
  filesystem I/O on the diagnostic request path.
- Recording persistence reads and writes only the canonical state recordings
  directory; historical storage locations are not migration inputs.
- Malformed or unreadable event-recording metadata is isolated per recording,
  valid siblings remain available, and System Doctor receives a redacted
  recovery warning with remediation.
- Replay engine (session lifecycle, action execution, selector fragility): `internal/recording/playback/`
- Replay execution results are intentionally ephemeral and process-owned. A
  daemon restart yields an explicit no-data response rather than guessing or
  creating a new persisted private-data surface; the durable input recording
  remains available to replay again.
- Recording comparison and regression reporting: `internal/recording/logdiff/`
- Both engines read recordings through a one-method source interface that
  `*recording.RecordingManager` satisfies (`LookupRecording` for replay, `GetRecording`
  for diffing), so neither depends on the manager type and both are tested
  against in-memory fakes.
- Capture exposes the canonical manager through `Capture.Recordings()`; there
  is no recording/playback delegation surface on Capture.
- Recording storage HTTP boundary: `internal/capture/httpingest/handlers.go`
- MCP owners: `cmd/browser-agent/tools_configure.go`, `cmd/browser-agent/internal/toolobserve/dispatcher.go`, and the composition root in `tools_core.go`
- Recording and playback MCP behavior/state: `cmd/browser-agent/internal/toolrecording/`
- Configure playback and log-diff actions route directly to the composed
  `toolrecording.Handler`; no root ToolHandler forwarding surface remains.
- Playback-result tests call `toolrecording.BuildPlaybackResult` directly, and
  composition passes the recording log callback directly; the former root
  result and log forwarding wrappers are deleted and structurally prohibited.
- Still a stub: `playback.executeAction` returns synthetic results and is not yet
  wired to the PendingQuery/interact system, so replay does not drive a browser.

---
doc_type: feature_index
feature_id: feature-flow-recording
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-10
code_paths:
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/internal/toolobserve/dispatcher.go
  - cmd/browser-agent/internal/toolobserve/deps.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolrecording/handler.go
  - cmd/browser-agent/internal/toolrecording/helpers.go
  - internal/capture/capture.go
  - internal/capture/httpingest/handlers.go
  - internal/recording/types.go
  - internal/recording/manager.go
  - internal/recording/actionlog/recorder.go
  - internal/recording/playback/types.go
  - internal/recording/playback/session.go
  - internal/recording/logdiff/types.go
  - internal/recording/logdiff/compare.go
  - src/background/recording/index.ts
  - src/background/recording/capture.ts
  - src/background/recording/listeners.ts
  - src/background/ui/keyboard-shortcuts.ts
  - src/background/ui/context-menus.ts
  - src/background/recording/utils.ts
  - src/background/ui/draw-mode-toggle.ts
  - src/offscreen/recording-worker.ts
  - src/popup/recording/action-recording.ts
  - src/popup/recording/recording.ts
  - src/popup/recording/recording-io.ts
  - src/lib/brand.ts
  - src/lib/daemon-http.ts
  - src/lib/storage/recovery.ts
  - src/lib/storage/validated.ts
test_paths:
  - internal/capture/httpingest/handlers_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - cmd/browser-agent/internal/toolrecording/handler_test.go
  - cmd/browser-agent/internal/toolrecording/toolrecording_test.go
  - cmd/browser-agent/internal/toolrecording/playback_result_test.go
  - internal/capture/recordingtest/recording_manager_test.go
  - internal/recording/manager_active_test.go
  - internal/capture/recordingtest/recording_store_integration_test.go
  - internal/capture/recordingtest/recording_logdiff_integration_test.go
  - internal/capture/recordingtest/recording_extension_lifecycle_test.go
  - internal/capture/no_facade_test.go
  - internal/recording/playback/playback_test.go
  - internal/recording/logdiff/logdiff_test.go
  - internal/recording/types_test.go
  - internal/recording/actionlog/recorder_test.go
  - tests/extension/recording-lifecycle/recording-fixture.js
  - tests/extension/recording-lifecycle/recording-lifecycle.test.js
  - tests/extension/recording-lifecycle/recording-recovery.test.js
  - tests/extension/recording-lifecycle/recording.test.js
  - tests/extension/recording-ui/recording-listeners-target-tab.test.js
  - tests/extension/recording-ui/recording-capture-branding.test.js
  - tests/extension/recording-ui/recording-log-branding.test.js
  - tests/extension/recording-ui/recording-shortcut-command.test.js
  - tests/extension/recording-lifecycle/action-recording-reconcile.test.js
  - tests/extension/contracts/entry-point-parity.test.js
  - tests/extension/ui-controls/tracked-hover-launcher.test.js
  - tests/extension/state-recovery/state-recovery-contract.test.js
  - tests/extension/state-recovery/validated-storage.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Flow Recording

## TL;DR

- Status: proposed
- Tool: interact, observe, configure
- Mode/Action: recording, playback, test-generation
- Location: `docs/features/feature/flow-recording`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_FLOW_RECORDING_001
- FEATURE_FLOW_RECORDING_002
- FEATURE_FLOW_RECORDING_003

## Code and Tests

Recording list and lookup response contracts, including required identifiers
and canonical list metadata, are owned by the recording handler tests rather
than a root observe-contract fixture.

Go callers use the canonical `Recording`, `RecordingAction`,
`RecordingMetadata`, and `RecordingManager` contracts from
`internal/recording`; alias-only recording and capture re-exports are
prohibited.
AI-driven actions from interact, state, upload, sequence, and screen-recording
entry points are normalized by one `internal/recording/actionlog.Recorder`
before entering the capture timeline.
Capture exposes its owned manager only through `Capture.Recordings()`. MCP,
storage, playback, and log-diff callers use the manager and the
`internal/recording/playback` or `internal/recording/logdiff` owners directly;
the former Capture forwarding surface is deleted.
Log-diff summary and report modes share one private parse-and-compare boundary;
only their operation-specific response shaping and failure summary differ.
Configure dispatch routes event-recording start/stop, playback, and log-diff
actions directly to the composed `toolrecording.Handler`; ToolHandler retains
no recording forwarding methods.
Recording log callbacks and playback-result formatting are wired to their
canonical owners directly; root one-line forwarding wrappers are prohibited.

- Core recording lifecycle and listener wiring:
  - `cmd/browser-agent/tools_configure.go`
  - `cmd/browser-agent/internal/toolobserve/dispatcher.go`
  - `cmd/browser-agent/tools_core.go`
  - `cmd/browser-agent/internal/toolrecording/handler.go`
- Only one recording runs at a time, and the running one must stay reachable.
  `event_recording_stop` accepts no `recording_id`, meaning "stop whatever is
  active", and `observe(recordings)` reports `active_recording_id` alongside the
  completed sessions it lists. Without both, an agent that lost the id start
  returned could never record again: stop demanded that id, the listing showed
  only finished recordings, and start refused while one was running.
- Starting while a recording is active returns `already_recording` and a playbook
  naming the stop call. It previously returned `internal_error` with "Check
  storage quota and try again", so a caller acting on the code and the playbook
  went looking for disk space while the actual remedy went unmentioned.
  - `cmd/browser-agent/internal/toolrecording/helpers.go`
  - `internal/capture/httpingest/handlers.go`
  - `src/background/recording/index.ts`
  - `src/background/recording/capture.ts`
  - `src/background/recording/listeners.ts`
  - `src/background/ui/keyboard-shortcuts.ts`
  - `src/background/ui/context-menus.ts`
  - `src/background/recording/utils.ts`
  - `src/offscreen/recording-worker.ts`
  - `src/popup/recording/recording.ts`
  - `src/popup/recording/recording-io.ts`
  - `src/lib/brand.ts`
- Core tests:
  - `cmd/browser-agent/internal/toolrecording/handler_test.go`
  - `cmd/browser-agent/internal/toolrecording/toolrecording_test.go`
  - `cmd/browser-agent/internal/toolrecording/playback_result_test.go`
  - `internal/capture/recordingtest/recording_manager_test.go`
  - `tests/extension/recording-lifecycle/recording.test.js`
  - `tests/extension/recording-ui/recording-listeners-target-tab.test.js`
  - `tests/extension/recording-ui/recording-capture-branding.test.js`
  - `tests/extension/recording-ui/recording-log-branding.test.js`
  - `tests/extension/recording-ui/recording-shortcut-command.test.js`
  - `tests/extension/ui-controls/tracked-hover-launcher.test.js`

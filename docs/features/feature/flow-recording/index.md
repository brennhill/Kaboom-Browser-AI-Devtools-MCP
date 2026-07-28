---
doc_type: feature_index
feature_id: feature-flow-recording
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolobserve/dispatcher.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolrecording/handler.go
  - cmd/browser-agent/internal/toolrecording/helpers.go
  - internal/capture/capture.go
  - internal/capture/handlers.go
  - internal/recording/types.go
  - internal/recording/manager.go
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
test_paths:
  - cmd/browser-agent/internal/toolrecording/handler_test.go
  - cmd/browser-agent/internal/toolrecording/toolrecording_test.go
  - cmd/browser-agent/recording_playback_result_test.go
  - internal/capture/recording_manager_test.go
  - internal/capture/no_facade_test.go
  - internal/recording/playback/playback_test.go
  - internal/recording/logdiff/logdiff_test.go
  - internal/recording/no_facade_test.go
  - tests/extension/recording.test.js
  - tests/extension/recording-listeners-target-tab.test.js
  - tests/extension/recording-capture-branding.test.js
  - tests/extension/recording-log-branding.test.js
  - tests/extension/recording-shortcut-command.test.js
  - tests/extension/action-recording-reconcile.test.js
  - tests/extension/entry-point-parity.test.js
  - tests/extension/tracked-hover-launcher.test.js
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

Go callers use the canonical `Recording`, `RecordingAction`,
`RecordingMetadata`, and `RecordingManager` contracts from
`internal/recording`; alias-only recording and capture re-exports are
prohibited.
Capture exposes its owned manager only through `Capture.Recordings()`. MCP,
storage, playback, and log-diff callers use the manager and the
`internal/recording/playback` or `internal/recording/logdiff` owners directly;
the former Capture forwarding surface is deleted.
Configure dispatch routes event-recording start/stop, playback, and log-diff
actions directly to the composed `toolrecording.Handler`; ToolHandler retains
no recording forwarding methods.

- Core recording lifecycle and listener wiring:
  - `cmd/browser-agent/tools_configure.go`
  - `cmd/browser-agent/internal/toolobserve/dispatcher.go`
  - `cmd/browser-agent/tools_core.go`
  - `cmd/browser-agent/internal/toolrecording/handler.go`
  - `cmd/browser-agent/internal/toolrecording/helpers.go`
  - `internal/capture/handlers.go`
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
  - `cmd/browser-agent/recording_playback_result_test.go`
  - `internal/capture/recording_manager_test.go`
  - `tests/extension/recording.test.js`
  - `tests/extension/recording-listeners-target-tab.test.js`
  - `tests/extension/recording-capture-branding.test.js`
  - `tests/extension/recording-log-branding.test.js`
  - `tests/extension/recording-shortcut-command.test.js`
  - `tests/extension/tracked-hover-launcher.test.js`

---
doc_type: flow_map
flow_id: tab-recording-and-media-ingest
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - cmd/browser-agent/tools_interact_dispatch.go (screen_recording_start|screen_recording_stop + record_start|record_stop aliases)
  - extension/manifest.json (toggle_action_sequence_recording)
  - src/background/event-listeners.ts (installRecordingShortcutCommandListener)
  - src/popup/recording.ts (setupRecordingUI)
  - cmd/browser-agent/server_routes.go (/screenshots|/draw-mode/complete)
  - cmd/browser-agent/tools_observe.go (saved_videos)
code_paths:
  - cmd/browser-agent/tools_interact_dispatch.go
  - internal/schema/interact_actions.go
  - cmd/browser-agent/tools_recording_video.go
  - cmd/browser-agent/tools_recording_interact_handler.go
  - cmd/browser-agent/tools_recording_video_paths.go
  - cmd/browser-agent/tools_recording_video_state.go
  - cmd/browser-agent/tools_recording_video_handlers.go
  - cmd/browser-agent/tools_recording_video_save.go
  - cmd/browser-agent/tools_recording_video_reveal.go
  - cmd/browser-agent/tools_recording_video_observe.go
  - src/background/event-listeners.ts
  - src/background/init.ts
  - src/background/context-menus.ts
  - src/background/recording/badge.ts
  - src/background/recording/capture.ts
  - src/background/recording/index.ts
  - src/background/recording/utils.ts
  - src/popup/recording.ts
  - extension/manifest.json
  - extension/popup.html
  - extension/popup.css
  - cmd/browser-agent/server_routes_media_common.go
  - cmd/browser-agent/server_routes_media_screenshots.go
  - cmd/browser-agent/server_routes_media_draw_mode.go
test_paths:
  - cmd/browser-agent/tools_interact_handler_test.go
  - cmd/browser-agent/tools_recording_video_test.go
  - cmd/browser-agent/server_routes_unit_test.go
  - cmd/browser-agent/tools_draw_mode_http_test.go
  - cmd/browser-agent/annotation_store_test.go
  - tests/extension/recording-shortcut-command.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tab Recording and Media Ingest

## Scope

Covers interact screen recording lifecycle (`screen_recording_start`/`screen_recording_stop` plus legacy `record_start`/`record_stop` aliases), popup/manual recording controls, keyboard-toggle recording controls, screenshot ingest, draw-mode ingest, and saved video listing/reveal behavior.

## Entrypoints

- Interact actions: `screen_recording_start`, `screen_recording_stop`.
- Back-compat interact aliases: `record_start`, `record_stop` (same handlers).
- Extension UI action: popup `Record action sequence` row (`record_start`/`record_stop`).
- Keyboard shortcut: `toggle_action_sequence_recording` (`Alt+Shift+R`) toggles start/stop.
- HTTP media endpoints: `/screenshots`, `/draw-mode/complete`, `/recordings/save`, `/recordings/reveal`, `/recordings/storage`.
- Observe query: `saved_videos`.

## Primary Flow

1. `recordingInteractHandler.handleRecordStart` validates extension readiness, clamps FPS/audio, resolves path, and queues extension command.
2. Recording state transitions are derived from command results in `recordingInteractHandler.resolveInteractRecordingState`.
3. `recordingInteractHandler.handleRecordStop` enforces valid state before queueing stop command.
4. MCP-initiated start writes `kaboom_pending_recording`; popup renders an approval card and sends `RECORDING_GESTURE_GRANTED` / `RECORDING_GESTURE_DENIED`.
5. Popup row and recording shortcut call extension `startRecording(..., fromPopup=true, targetTabId=trackedOrActiveTab)` so manual record follows tracked-tab intent.
6. Shortcut toggle checks current recording state: active -> `stopRecording`; idle -> `startRecording`.
7. Context-menu labels are refreshed on `contextMenus.onShown` to reflect live state (`Control/Release`, `Record/Stop`, `Annotate/Stop`, action recording start/stop).
8. Recording badge timer lifecycle is driven from `startRecording` / `stopRecording` so popup, keyboard, context menu, and MCP entry points stay in sync.
9. `/screenshots` validates rate limits and data URLs, then persists image and optional query result payload.
10. `/draw-mode/complete` stores screenshot, annotations, details, and pushes completion updates.
11. `toolObserveSavedVideos` enumerates persisted video metadata across primary + legacy dirs.

## Error and Recovery Paths

- Invalid audio mode, malformed data URL, invalid path, or missing tab ID return structured errors.
- Shortcut start/stop failures return local action toasts with remediation text.
- MCP recording start can terminate as explicit denial (`RECORDING_GESTURE_DENIED`) or timeout when popup approval does not occur.
- Screenshot limiter returns `429` on per-client burst, `503` on capacity exhaustion.
- Draw-mode parse failures are returned as warnings while valid annotations still persist.

## State and Contracts

- Recording lifecycle constants: `idle`, `awaiting_user_gesture`, `recording`, `stopping`.
- `recordInteract` state is guarded by `recordInteractMu`.
- File path writes must remain under runtime state dirs (`pathWithinDir` / `isWithinDir`).
- Hotkey, popup, and interact paths converge on the same extension recording state machine.

## Code Paths

- `cmd/browser-agent/tools_interact_dispatch.go`
- `internal/schema/interact_actions.go`
- `cmd/browser-agent/tools_recording_video.go`
- `cmd/browser-agent/tools_recording_interact_handler.go`
- `cmd/browser-agent/tools_recording_video_paths.go`
- `cmd/browser-agent/tools_recording_video_state.go`
- `cmd/browser-agent/tools_recording_video_handlers.go`
- `cmd/browser-agent/tools_recording_video_save.go`
- `cmd/browser-agent/tools_recording_video_reveal.go`
- `cmd/browser-agent/tools_recording_video_observe.go`
- `src/background/event-listeners.ts`
- `src/background/init.ts`
- `src/background/context-menus.ts`
- `src/background/recording/badge.ts`
- `src/background/recording/capture.ts`
- `src/background/recording/index.ts`
- `src/background/recording/utils.ts`
- `src/popup/recording.ts`
- `extension/manifest.json`
- `extension/popup.html`
- `extension/popup.css`
- `cmd/browser-agent/server_routes_media_common.go`
- `cmd/browser-agent/server_routes_media_screenshots.go`
- `cmd/browser-agent/server_routes_media_draw_mode.go`

## Test Paths

- `cmd/browser-agent/tools_interact_handler_test.go`
- `cmd/browser-agent/tools_recording_video_test.go`
- `cmd/browser-agent/server_routes_unit_test.go`
- `cmd/browser-agent/tools_draw_mode_http_test.go`
- `cmd/browser-agent/annotation_store_test.go`
- `tests/extension/recording-shortcut-command.test.js`
- `tests/extension/context-menus-labels.test.js`
- `tests/extension/recording-listeners-target-tab.test.js`

## Edit Guardrails

- Keep state-machine transitions deterministic and test-backed.
- Any media endpoint payload change must update endpoint contract tests.
- Preserve directory-bound path validation before writes or reveals.

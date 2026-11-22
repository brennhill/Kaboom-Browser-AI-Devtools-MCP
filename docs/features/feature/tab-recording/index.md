---
doc_type: feature_index
feature_id: feature-tab-recording
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/composition/toolruntime/tools_interact_dispatch.go
  - internal/tools/schema/interact/actions.go
  - internal/composition/toolruntime/tools_core.go
  - cmd/browser-agent/server.go
  - internal/tools/observe/handler/dispatcher.go
  - internal/tools/observe/handler/deps.go
  - internal/screenrec/deps.go
  - internal/tools/guard/guards.go
  - internal/screenrec/types.go
  - internal/screenrec/handlers.go
  - internal/screenrec/state.go
  - internal/screenrec/paths.go
  - internal/screenrec/save.go
  - internal/screenrec/reveal.go
  - internal/screenrec/observe.go
  - internal/lifecycle/doctor/statediag/collector.go
  - internal/platform/state/file/statefile.go
  - internal/recording/actionlog/recorder.go
  - src/background/event-listeners.ts
  - src/background/init.ts
  - src/background/ui/keyboard-shortcuts.ts
  - src/background/ui/content-script-bridge.ts
  - src/background/ui/tracked-tab-state.ts
  - src/background/ui/context-menus.ts
  - src/background/recording/badge.ts
  - src/background/recording/capture.ts
  - src/background/recording/index.ts
  - src/background/recording/rehydration.ts
  - src/background/recording/utils.ts
  - src/background/commands/interact.ts
  - src/background/runtime-state/connection-generation.ts
  - src/offscreen/recording-worker.ts
  - src/types/runtime-messages.ts
  - src/lib/daemon-http.ts
  - src/lib/storage/changes.ts
  - src/lib/storage/io.ts
  - src/lib/storage/local.ts
  - src/lib/storage/session.ts
  - src/lib/storage/recovery.ts
  - src/lib/storage/fault.ts
  - src/lib/storage/validated.ts
  - src/popup/recording/recording.ts
  - extension/manifest.json
  - extension/popup.html
  - extension/popup.css
  - internal/recording/manager.go
  - internal/recording/manager_store.go
  - internal/composition/capture/capture.go
  - internal/tools/recording/handler.go
  - src/popup/recording/action-recording.ts
  - src/popup/recording/recording-io.ts
  - src/background/recording/listeners.ts
test_paths:
  - internal/screenrec/state_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - internal/tools/interact/dispatch/handler_test.go
  - internal/tools/interact/handler/interact_browser_test.go
  - internal/tools/interact/handler/interact_dom_test.go
  - internal/tools/interact/handler/interactstate/state_test.go
  - internal/screenrec/screenrec_test.go
  - internal/lifecycle/doctor/statediag/collector_test.go
  - internal/recording/actionlog/recorder_test.go
  - tests/extension/recording-ui/recording-shortcut-command.test.js
  - tests/extension/ui-controls/context-menus-labels.test.js
  - tests/extension/recording-ui/recording-listeners-target-tab.test.js
  - tests/extension/contracts/storage-modules.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
  - tests/extension/state-recovery/state-recovery-contract.test.js
  - tests/extension/state-recovery/validated-storage.test.js
  - tests/extension/recording-lifecycle/recording-rehydration.test.js
  - tests/extension/recording-lifecycle/recording-fixture.js
  - tests/extension/recording-lifecycle/recording.test.js
  - tests/extension/contracts/background-boundaries.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tab Recording

## TL;DR

- Status: proposed
- Tool: interact, observe
- Mode/Action: screen_recording_start, screen_recording_stop, saved_videos, toggle_action_sequence_recording
- Location: `docs/features/feature/tab-recording`

## Recording Ownership

Kaboom has three recording kinds. Each has exactly one owner, and no owner
reads or writes another kind's state directly. This section is the canonical
ownership reference; the [flow-recording index](../flow-recording/index.md#recording-ownership)
links here.

| Kind | Source of truth | Starts | Restores | Stops | Persists |
| --- | --- | --- | --- | --- | --- |
| Event recording (`configure` `event_recording_start` / `event_recording_stop`) | Go `RecordingManager` (`internal/recording/manager.go`), held by `Capture` and exposed only through `Capture.Recordings()` (`internal/composition/capture/capture.go`) | `toolrecording.Handler.EventRecordingStart` (`internal/tools/recording/handler.go`); the popup action-recording row calls the same daemon API | Not restored. The active recording lives in memory only, so a daemon restart drops it. Saved recordings are listed from disk (`manager_store.go`) | `toolrecording.Handler.EventRecordingStop` | `persistRecordingToDisk` on stop (`manager_store.go`); quota in `manager_storage.go` |
| Live screen recording (`interact` `screen_recording_start` / `screen_recording_stop`, aliases `record_start` / `record_stop`; popup, keyboard, context menu, hover launcher) | Offscreen `MediaRecorder` (`src/offscreen/recording-worker.ts`). The background mirrors it in memory and in `kaboom_recording` | `startRecording` in `src/background/recording/index.ts`. Every entry point goes through it; the MCP path waits for the popup gesture (`capture.ts`) | `rehydrateRecordingStateOnLoad` (`index.ts`) asks the offscreen document via `rehydration.ts`. With no live recorder it clears `kaboom_recording`; it always clears `kaboom_pending_recording` | `stopRecording` in `index.ts`, which stops the offscreen recorder | The offscreen worker POSTs the WebM blob to daemon `/recordings/save` |
| Saved video artifact | Daemon recordings directory (`screenrec.Dir`, `internal/screenrec/paths.go`) | `screenrec.InteractHandler.HandleRecordStart` queues the start command (`handlers.go`) | Read back from disk by `HandleObserveSavedVideos` (`observe.go`) | `HandleRecordStop` (`handlers.go`) | `screenrec.HandleSave` and `writeVideoToDisk` (`save.go`) |

Extension storage keys and their writers (CLAUDE.md rule 18):

| Key | Writer | Readers |
| --- | --- | --- |
| `kaboom_recording` | `src/background/recording/index.ts` | popup `src/popup/recording/recording.ts`, hover launcher |
| `kaboom_pending_recording` (MCP gesture intent) | `src/background/recording/capture.ts` sets it and clears it once the gesture resolves; `index.ts` clears it on service-worker start | popup `recording.ts` shows the approve/deny card and sends only the decision message |
| `kaboom_pending_mic_recording` | popup `recording-io.ts` sets it; popup `recording.ts` consumes it | background `listeners.ts` |
| `kaboom_action_recording` (event-recording mirror) | popup `action-recording.ts` | popup `action-recording.ts`, which reconciles it against the daemon when the popup opens |

`tests/extension/contracts/background-boundaries.test.js` fails if
anything outside `src/background/recording/` writes `kaboom_recording` or
`kaboom_pending_recording`. The popup still trusts the `kaboom_recording`
mirror without asking the live owner; that is tracked as kaboom-jdm5.7.

### Failure modes

- **Event recording.** Start fails with `already_recording` while one is active.
  Stop fails with `recording_not_found` for an unknown ID, and with
  `recording_save_failed` when the disk write fails; the recording then stays
  active so stop can be retried. A daemon restart loses the active recording.
- **Live screen recording.** A service-worker restart during a recording is
  recovered from the offscreen document. A restart while waiting for the popup
  gesture drops the wait, and the next start clears the stale pending intent.
  Gesture denial or the 30 s timeout returns a `RECORD_START` error. If the
  offscreen document dies, the recording ends and the next service-worker
  start clears the stale `kaboom_recording` mirror.
- **Saved video artifact.** An upload with a bad name or path returns a
  `RECORDING_SAVE` error and writes nothing. The save result is also stored as
  the pending query result (`query_id`), so `screen_recording_stop` reports it.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_TAB_RECORDING_001
- FEATURE_TAB_RECORDING_002
- FEATURE_TAB_RECORDING_003

## Code and Tests

The implementation and tests for popup/manual recording and shortcut-toggle recording are listed in frontmatter `code_paths` and `test_paths`.
Saved-video discovery is scoped to the canonical recordings directory; it does
not merge or deduplicate entries from historical storage roots.
Unreadable or malformed saved-video metadata is omitted without failing the
query and is reported through the canonical System Doctor recovery collector.
Video uploads are committed as a durable pair: the WebM is written to a private
temporary file, synced, closed, and durably moved before the sidecar is
atomically written. Any move, metadata, quota, partial-write, sync, or
cancellation failure removes both halves and activates a redacted Doctor
incident; a successful retry resolves it. Incomplete pairs are never returned
as saved recordings.
Extension storage is divided by change lifecycle, shared I/O mechanics, durable
local state, and ephemeral session state. Consumers import those owners
directly; there is no all-purpose storage facade or compatibility barrel.
Screen-recording dependencies receive the query owner callback directly from
the composition boundary through `buildScreenrecDeps`; dependency construction
is not a `ToolHandler` method, and no root `getCommandResult` forwarding method
is retained. The canonical handler constructor rejects every missing host seam
immediately, so incomplete composition cannot survive startup and fail later as
an unclassified nil-function crash.
MCP-initiated start and stop operations carry the daemon connection generation
through the offscreen bridge. Delayed confirmations from a superseded daemon
are rejected with redacted Doctor evidence; a stale start is actively stopped
so it cannot leave an orphaned capture. Popup, keyboard, and context-menu
recordings remain local operations and are intentionally independent of daemon
handoffs.

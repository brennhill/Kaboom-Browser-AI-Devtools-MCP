# Extension Architecture

Last reviewed: 2026-11-21

## Overview

The Kaboom Chrome extension (MV3) captures browser observations (console logs,
network requests, WebSocket events, user actions, performance data) and sends
them to the local Go daemon over HTTP. It also receives **browser commands**
from the daemon over `/sync` and executes them in the tracked tab.

Domain terms in **bold** are defined in [GLOSSARY.md](../../../GLOSSARY.md).
For the daemon half, see [server-architecture.md](server-architecture.md).

## Runtime contexts

These are Chrome execution environments, not bounded contexts. They are sibling
roots under `src/`, and `.architecture-boundaries.json` (`forbidden_imports`)
forbids any of them from importing another. They talk only through
`src/types/runtime-messages.ts` messages and `window.postMessage`.

| Runtime context | Entry | Role |
| --- | --- | --- |
| Background service worker | `src/background.ts`, `src/background/init.ts` | Owns connection, commands, tracking, Pilot and screen-recording lifecycle. No DOM |
| Content script | `src/content.ts`, `src/content/` | Relays between background and inject. Renders overlays. Reports content readiness |
| Inject script (MAIN world) | `src/inject.ts`, `src/inject/` | Instruments page globals and runs page-world queries. Bundled |
| Offscreen document | `src/offscreen/recording-worker.ts` | Encodes screen recordings and uploads saved video |
| Popup and side panel | `src/popup.ts`, `src/popup/`, `src/sidepanel.ts` | User controls |
| Shared library | `src/lib/` | Pure shared code. It cannot import any runtime context |

## Bounded contexts

| Context | Owned state | Code | Public interface | May depend on |
| --- | --- | --- | --- | --- |
| **Connection & Sync** (extension half) | Sync loop state, the mirrored **connection generation**, connection status, batch delivery and circuit breaking | `src/background/sync/`, `src/background/runtime-state/connection-generation.ts`, `src/background/runtime-state/connection-state.ts`, `src/background/orchestration/connection-monitor.ts` | `src/background/sync/sync-client.ts:SyncClient`, `src/background/sync/sync-manager.ts:startSyncClient`, `src/background/sync/sync-manager.ts:resetSyncClientConnection`, wire `src/types/wire/wire-sync.ts` | Browser Command Execution (`handlePendingQuery`) |
| **Browser Command Execution** | In-flight command execution and progress reporting | `src/background/commands/`, `src/background/pending-queries.ts`, `src/background/exec/`, `src/background/dom/` | `src/background/commands/registry.ts:registerCommand`, `src/background/commands/registry.ts:dispatch`, `src/background/commands/helpers.ts` (target resolution, result senders), `src/background/pending-queries.ts:handlePendingQuery` | Tab Tracking & Pilot, Screen Recording (`index.ts`), Connection & Sync (`SyncClient` for results) |
| **Tab Tracking & Pilot** | The **tracked tab** (authoritative in `chrome.storage`), **tracking continuity**, **content readiness**, **Pilot** intent | `src/lib/tabs/`, `src/background/runtime-state/tracking-continuity.ts`, `src/background/runtime-state/content-readiness.ts`, `src/background/runtime-state/pilot-state.ts`, `src/background/event-listeners.ts` (Chrome adapters), `src/types/runtime/tracking.ts` | `src/lib/tabs/tracked-tab-storage.ts:readTrackedTab`, `src/lib/tabs/tracked-tab-storage.ts:setTrackedTab`, `src/lib/tabs/tab-tracking-core.ts:trackTab`, `src/background/runtime-state/tracking-continuity.ts:trackingContinuity`, `src/background/runtime-state/content-readiness.ts:contentReadiness`, `src/background/runtime-state/pilot-state.ts:isAiWebPilotEnabled` | `src/lib/` |
| **Observation Capture** (extension half) | Page hooks and per-type batchers | `src/inject/`, `src/lib/net/`, `src/lib/page/`, `src/content/page-telemetry.ts`, `src/background/sync/batcher-instances.ts` | `src/background/sync/batcher-instances.ts:createBatcherInstances`, the capture runtime messages | Connection & Sync (delivery) |
| **Screen Recording** | Live recording state, its persisted mirror (`StorageKey.RECORDING`), and the offscreen encoder | `src/background/recording/`, `src/offscreen/recording-worker.ts`, `src/popup/recording/recording.ts` | `src/background/recording/index.ts:startRecording`, `src/background/recording/index.ts:stopRecording`, `src/background/recording/index.ts:initRecording`, `src/background/recording/utils.ts`, offscreen messages in `src/types/runtime-messages.ts` | Tab Tracking & Pilot |
| **Event Recording** (UI only) | None. The daemon owns event recordings | `src/popup/recording/action-recording.ts` | Daemon HTTP via `src/lib/daemon-http.ts:postDaemonJSON` | — |
| **Composition** | Wiring only | `src/background/init.ts`, `src/background.ts` | `src/background/init.ts:initializeExtension` | Everything in background |

### Context map

```
           daemon /sync (wire-sync.ts)          daemon POST uploads
                 ^      |                         ^
     heartbeat,  |      | commands (+generation)  | /recordings/save, ingestion endpoints
     results     |      v                         |
   +-------------------------+   handlePendingQuery   +-----------------------------+
   | Connection & Sync       |----------------------->| Browser Command Execution   |
   | SyncClient, generation  |<-----------------------| registry, exec, dom         |
   +-------------------------+   queueCommandResult   +-----------------------------+
          ^ batches                                      |            |
          |                         persistTrackedTab,   |            | startRecording /
   +-------------------------+      requireAiWebPilot,   |            | stopRecording
   | Observation Capture     |      contentReadiness     v            v
   | inject, content, lib    |                 +----------------+  +------------------+
   +-------------------------+                 | Tab Tracking & |  | Screen Recording |
                                               | Pilot          |  | background +     |
            Chrome tabs/storage events  -----> | (owner)        |  | offscreen worker |
                                               +----------------+  +------------------+
```

## Composition root

`src/background/init.ts:initializeExtension` wires the background contexts.
It installs the Chrome lifecycle adapters
(`src/background/event-listeners.ts:handleTrackedTabUrlChange`,
`src/background/event-listeners.ts:handleTrackedTabClosed`), subscribes to
`trackingContinuity`, restores the tracked tab, starts the sync client, and
calls `initRecording` to restore any screen recording after a service-worker
restart.

## Operation owners and handoffs

Rows describe the extension side. The daemon side of each operation is in
[server-architecture.md](server-architecture.md#operation-owners-and-handoffs).

| Operation | Owner | Flow and handoff contracts |
| --- | --- | --- |
| Heartbeat and delivery | Connection & Sync | `SyncClient` sends `SyncRequest` (settings, tracked tab, Pilot, results, in-progress list) and receives commands. Each command carries `connection_generation` (`src/types/wire/wire-sync.ts`) |
| Command execution | Browser Command Execution | `src/background/sync/sync-manager.ts` → `handlePendingQuery` → `dispatch` → the handler registered with `registerCommand`. `rejectSupersededCommand` drops commands from an older generation |
| Progress and results | Browser Command Execution, delivered by Connection & Sync | `SyncClient.updateCommandProgress` and `SyncClient.queueCommandResult`. Results ride the next heartbeat and are stamped with the generation they were issued under |
| Reconnect | Connection & Sync (daemon owns the generation) | A sync response with a new generation → `setConnectionGeneration`. `resetSyncClientConnection` restarts the loop. Work from the old generation is rejected on both sides |
| Tab switch (synchronous) | Tab Tracking & Pilot | `switch_tab` → `src/background/exec/browser-actions.ts:executeSwitchTabAction` → `persistTrackedTab` writes storage before the result returns. The daemon then applies the command-confirmed observation |
| Tab switch (asynchronous) | Tab Tracking & Pilot | User pick (popup), navigation or close → `trackTab` or Chrome adapters → `chrome.storage` and `trackingContinuity` → the next heartbeat carries the new tab |
| Content readiness | Tab Tracking & Pilot | Document-changing actions call `contentReadiness.begin`. The content script answers with `TrackingContentReadyMessage` (`src/types/runtime/tracking.ts`). Commands for which `requiresContentReadiness` is true wait on the barrier in `src/background/commands/registry.ts` |
| Pilot gating | Tab Tracking & Pilot | The popup toggle (`src/popup/ai-web-pilot.ts`) sends `set_ai_web_pilot_enabled` to the background, the single writer (`src/background/message-routing/pilot-handler.ts`), which persists `StorageKey.AI_WEB_PILOT_ENABLED`. `isAiWebPilotEnabled` caches it. `requireAiWebPilot` gates execution. The heartbeat reports it so the daemon can gate too |
| Screen recording start | Screen Recording | Command `screen_recording_start` (`src/background/commands/interact.ts`) → `startRecording` → persists `StorageKey.RECORDING` → `OffscreenStartRecordingMessage` to `src/offscreen/recording-worker.ts` |
| Screen recording restore | Screen Recording | `initRecording` → `src/background/recording/rehydration.ts:resolveRecordingRehydration` reconciles the persisted mirror with the offscreen document (`OffscreenGetRecordingStateMessage`) |
| Screen recording stop and save | Screen Recording | `stopRecording` → `OffscreenStopRecordingMessage` → the worker uploads `POST /recordings/save` (a **saved video**). The daemon's save handler completes the command |
| Event recording (popup) | Event Recording (daemon) | `src/popup/recording/action-recording.ts` calls the daemon over HTTP. The extension keeps no event-recording state beyond UI |
| Observation capture | Observation Capture | inject hooks → `window.postMessage` → content → runtime messages → `createBatcherInstances` batchers → daemon ingestion endpoints. Extension logs ride the heartbeat |

### Current behavior and target

- **Gap (kaboom-jdm5.7):** the popup (`src/popup/recording/recording.ts`) gates
  its recording UI on the `StorageKey.RECORDING` storage mirror. It should
  reconcile with the background owner (CLAUDE.md rule 18), because the mirror
  goes stale if the service worker dies without clearing it.
- **Target:** keep the Chrome lifecycle adapters thin. They report to
  `trackingContinuity` and the tracked-tab storage helpers, and never write
  tracking keys directly.

## Enforcement

`make lint-boundaries` (`scripts/contracts/check-architecture-boundaries.cjs`,
tests in `scripts/contracts/check-architecture-boundaries.test.cjs`) enforces:

- runtime-context isolation (`forbidden_imports`);
- canonical type owners and no re-exports or aggregate files;
- **bounded-context public entry points** (`context_entry_points`): files
  outside `src/background/recording/` may import only `index.ts` and
  `utils.ts` from it, and files outside `src/background/commands/` may import
  only `registry.ts` and `helpers.ts`. Per-family command modules and
  recording internals stay private.

## Constraints

- **DOM action primitives must be self-contained.** Each function under
  `src/background/dom/primitives/` is serialized by
  `chrome.scripting.executeScript({ func })`. It cannot reference closures or
  imports.
- **Content and inject scripts must be bundled.** Run `make compile-ts` after
  any `src/` change.
- **No dynamic imports in the background service worker.**
- **Shared storage keys go through their owner's helpers** (CLAUDE.md rule
  18). Tracked-tab keys go through `src/lib/tabs/tracked-tab-storage.ts`.

## "Change X, edit Y" lookup

| Want to... | Edit |
| --- | --- |
| Add a browser command | `src/background/commands/<family>.ts` (`registerCommand`). The command contract ID regenerates |
| Change server communication | `src/background/sync/sync-client.ts`, `src/background/sync/server.ts`, and the wire pair |
| Add a runtime message | `src/types/runtime-messages.ts` first (CLAUDE.md rule 20) |
| Change tracked-tab behavior | `src/lib/tabs/tracked-tab-storage.ts`, `src/background/runtime-state/tracking-continuity.ts` |
| Add a bounded-context entry rule | `context_entry_points` in `.architecture-boundaries.json`, with a test in `scripts/contracts/check-architecture-boundaries.test.cjs` |

## Testing

```bash
npm run test:ext       # Extension tests
make lint-boundaries   # Runtime-context and bounded-context rules
make test              # Go and extension tests
```

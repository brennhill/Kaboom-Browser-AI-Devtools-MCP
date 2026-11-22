# Kaboom Glossary

The ubiquitous language of Kaboom. Every term names its **owning bounded
context**, the code identifiers that implement it, and the words to avoid.
Use these words in code, docs, beads and reviews. When a term changes, update
this file in the same change.

- Context map, owners and handoffs:
  [server-architecture.md](docs/core/architecture/server-architecture.md) and
  [extension-architecture.md](docs/core/architecture/extension-architecture.md).
- Enforcement: `scripts/contracts/layering/contexts.go` (Go context rules) and
  the `context_entry_points` section of `.architecture-boundaries.json`
  (extension public entry points). `npm run docs:check` verifies that every
  backticked path in this file exists, and that each `path:Identifier`
  reference names an identifier found in that file.

Reference format: `path` is a file or directory. `path:Name` or
`path:Type.Member` is a code identifier in that file.

## Bounded contexts

Each context owns its state. Other contexts reach that state only through the
public interface listed here. Details are in the architecture guides.

| Context | Owns | Daemon code | Extension code |
| --- | --- | --- | --- |
| **Connection & Sync** | Connection identity: extension session, connection generation, liveness, the /sync protocol, and the daemon's projections of tracking and Pilot | `internal/sync/runtime/` | `src/background/sync/`, `src/background/runtime-state/connection-generation.ts` |
| **Browser Command** | Command lifecycle: queue, delivery eligibility, progress, results, expiry, lost commands | `internal/browsercommand/`, `internal/browsercommand/query/` | `src/background/commands/`, `src/background/pending-queries.ts`, `src/background/exec/`, `src/background/dom/` (as **Browser Command Execution**) |
| **Tab Tracking & Pilot** | Tracked-tab selection, tracking continuity, content readiness, Pilot intent | (projection only, in Connection & Sync) | `src/lib/tabs/`, `src/background/runtime-state/tracking-continuity.ts`, `src/background/runtime-state/content-readiness.ts`, `src/background/runtime-state/pilot-state.ts` |
| **Observation Capture** | Retention and queries for captured browser observations | `internal/observation/store/telemetry/`, `internal/observation/store/logs/`, `internal/observation/store/perf/` and their ring stores | `src/inject/`, `src/content/`, `src/background/sync/batcher-instances.ts` (capture and batching) |
| **Extension Ingestion** | HTTP endpoints that accept extension uploads and write them to their owners | `internal/ingestion/http/`, `internal/ingestion/media/` | — |
| **Event Recording** | Recordings of user-flow events, their persistence, playback and diff | `internal/recording/`, `internal/tools/recording/` | — |
| **Screen Recording** | Live tab video capture and saved video files | `internal/screenrec/` (MCP start/stop, saved-video path) | `src/background/recording/`, `src/offscreen/recording-worker.ts`, `src/popup/recording/` |
| **Application Use Cases** | Transport-independent workflows behind the tools, each in its owning context's app folder | `internal/page/app/`, `internal/analysis/app/` | — |
| **Tool Surface** | The five MCP tools: schemas, dispatch, response shaping | `internal/tools/mcp/`, `internal/tools/schema/`, `internal/tools/`, `internal/tools/observe/handler/`, `internal/tools/interact/handler/`, `internal/tools/asynccommand/` | — |
| **Composition** | Construction and wiring of every owner | `cmd/browser-agent/server.go`, `internal/composition/toolruntime/`, `internal/composition/capture/capture.go` | `src/background/init.ts`, `src/background.ts` |
| **Process Lifecycle** | Bridge, daemon spawn/respawn, single-instance governance | `internal/lifecycle/bridge/`, `internal/lifecycle/daemon/`, `internal/lifecycle/instance/gov/` | — |
| **Product Telemetry** | Anonymous usage beacons. Opt out with `KABOOM_TELEMETRY=off` | `internal/telemetry/` | — |
| **Install & Release** | Packaging, version sync, publish | `npm/`, `scripts/release/` | `extension/` (built artifacts) |

## Terms

### Adapter

A module that translates one transport into an owner's interface without
owning domain state. Examples are MCP tool handlers, HTTP handlers and Chrome
event listeners.
- Context: Tool Surface (MCP adapters), Extension Ingestion (HTTP adapters), Tab Tracking & Pilot (Chrome lifecycle adapters).
- Code: `internal/tools/observe/handler/dispatcher.go`, `internal/ingestion/http/handlers.go:Handlers`, `src/background/event-listeners.ts:handleTrackedTabUrlChange`.
- Avoid: "handler" for a module that also owns state. An adapter that holds state is an owner.

### Bounded context

A named area of the system that owns a set of state and exposes a public
interface. Its upstream and downstream dependencies are declared and enforced.
The current list is the table above.
- Code: `scripts/contracts/layering/contexts.go:boundedContexts`, `.architecture-boundaries.json`.
- Avoid: "module", "layer" or "domain" when you mean a context.

### Browser command

A unit of work the daemon asks the extension to perform in the browser, for
example click, navigate, screenshot or switch_tab. The wire form is a
`PendingQuery` with a correlation ID. Its lifecycle is a `CommandResult`.
- Context: Browser Command.
- Code: `internal/browsercommand/query/types.go:PendingQuery`, `internal/browsercommand/query/types.go:CommandResult`, `internal/browsercommand/owner.go:Owner.Enqueue`, `src/background/commands/registry.ts:registerCommand`.
- Avoid: "job", "task", "action" (an action is a parameter of `interact`, not a lifecycle object).

### Browser query

A synchronous browser read that an MCP call waits on by query ID, for example
the a11y audit, page storage or the style probe. It travels on the same queue
as a command but has no LLM-facing correlation ID.
- Context: Browser Command.
- Code: `internal/browsercommand/owner.go:Owner.WaitQueryResult`, `internal/browsercommand/query/types.go:QueryRunner`.
- Avoid: using "query" for a tracked command. The wire struct is named `PendingQuery` for historical reasons and carries both.

### Capture aggregate

The `*capture.Capture` struct built by the composition root. It holds every
store and runtime. It is a wiring object, not a domain owner. Only the
composition root (`cmd/browser-agent` and the toolruntime wiring) holds it.
Features take the owner they read, and health readers take the bundle from
`internal/composition/capture/capture.go:Capture.HealthOwners`. The `capture-root`
layering rule enforces this.
- Context: Composition.
- Code: `internal/composition/capture/capture.go:Capture`, `internal/composition/capture/capture.go:NewCapture`.
- Avoid: passing `*capture.Capture` into a feature. Pass the owner, for example `*syncruntime.Runtime` or `*telemetrystore.Store`.

### Capture store

The owner of one kind of retained observation: its ring buffer, its eviction
and its queries.
- Context: Observation Capture.
- Code: `internal/observation/store/telemetry/`, `internal/observation/store/logs/`, `internal/observation/store/perf/`, `internal/observation/store/actions/`, `internal/observation/store/bodies/`, `internal/observation/store/waterfall/`, `internal/observation/store/ring/`.
- Avoid: "buffer" for the store as a whole. A buffer is the ring inside a store.

### Command contract ID

The hash of every literal `registerCommand` name in the extension. It travels
on /sync so the daemon can detect a daemon and extension that disagree on
supported commands.
- Context: Connection & Sync.
- Code: `internal/browsercommand/contract/generated.go:ID`, `internal/sync/runtime/runtime.go:Runtime.CommandContractID`.

### Command owner

The single owner of the browser-command lifecycle: enqueue, delivery,
progress, results from every source, expiry, disconnect failure and
lost-command reconciliation. No production code reaches the dispatcher except
through it.
- Context: Browser Command.
- Code: `internal/browsercommand/owner.go:Owner`, `internal/browsercommand/internal/dispatch/` (private), `scripts/contracts/layering/command_owner.go`.
- Avoid: "dispatcher", "queue" or "QueryDispatcher" for the owner. The dispatcher is a private implementation detail.

### Composition root

The place where concrete owners are constructed and injected. Kaboom has two
on the daemon side and one in the extension.
- Context: Composition.
- Code: `cmd/browser-agent/server.go:setupHTTPRoutes`, `internal/composition/toolruntime/tools_core.go:NewToolHandler`, `src/background/init.ts:initializeExtension`.
- Avoid: building owners anywhere else. The layering check rejects `capture.NewCapture` outside `cmd/browser-agent`.

### Connection generation

A monotonic number. The daemon bumps it when a new extension session starts.
Every delivered command, sync result and progress report is stamped with it.
Work from a superseded generation is rejected and must not mutate
current-generation commands.
- Context: Connection & Sync. The daemon owns it. The extension mirrors it.
- Code: `internal/sync/runtime/handler.go:bumpSyncGenerationForNewSession`, `internal/sync/runtime/handler.go:applyIfCurrentSyncGeneration`, `internal/browsercommand/exchange.go:GenerationGate`, `src/background/runtime-state/connection-generation.ts:isConnectionGenerationCurrent`, `src/background/commands/registry.ts:rejectSupersededCommand`.
- Avoid: "epoch", "version" or "session counter".

### Content readiness

The barrier that holds commands needing a content script until the tracked
tab's content script reports ready after a navigation or injection.
- Context: Tab Tracking & Pilot (extension).
- Code: `src/background/runtime-state/content-readiness.ts:ContentReadinessBarrier`, `src/background/runtime-state/content-readiness.ts:requiresContentReadiness`, `src/types/runtime/tracking.ts:TrackingContentReadyMessage`.
- Avoid: "page loaded". Readiness means the content script answered, not that the document loaded.

### Correlation ID

The LLM-facing ID of an async browser command. The MCP client uses it to poll
or wait, and every result source uses it to complete the command.
- Context: Browser Command.
- Code: `internal/browsercommand/query/types.go:PendingQuery.CorrelationID`, `internal/browsercommand/owner.go:Owner.WaitCommand`.
- Avoid: confusing it with the query ID (an internal queue ID) or the client ID (the MCP client that owns a query).

### Doctor diagnostic

A persistent, redacted, structured diagnostic that operators see through
`/doctor`. Unexpected failures that are not returned to a caller must emit
one (CLAUDE.md rule 27).
- Context: Process Lifecycle (operations).
- Code: `internal/lifecycle/doctor/statediag/store.go:PersistentCollector.Report`.

### Event recording

A daemon-owned recording of user-flow events. It is started and stopped by
`configure({what: "event_recording_start"|"event_recording_stop"})`, persisted
to disk, and later replayed or diffed. It is not video.
- Context: Event Recording.
- Code: `internal/recording/manager.go:RecordingManager.StartRecording`, `internal/recording/manager.go:RecordingManager.StopRecording`, `internal/tools/recording/handler.go:Handler.EventRecordingStart`, `internal/recording/playback/`, `internal/recording/logdiff/`.
- Avoid: "recording" without a qualifier, "flow recording" in code, and "screen recording".

### Extension session

The identity of one running extension service worker, sent on each /sync. A
new session ID bumps the connection generation.
- Context: Connection & Sync.
- Code: `internal/sync/runtime/runtime.go:ExtensionSnapshot.ExtSessionID`, `src/background/runtime-state/startup-state.ts:EXTENSION_SESSION_ID`.
- Avoid: "client". A client is an MCP client.

### Handoff

The contract one context uses to pass work or data to another: a wire type,
a runtime message, or a Go interface or owner method. Each cross-context
operation is listed with its handoff in the architecture guides.
- Code: `internal/sync/runtime/wire_sync.go`, `src/types/runtime-messages.ts`, `internal/browsercommand/ports/ports.go:ResultSink`.

### Heartbeat

The extension's periodic /sync request. It carries settings, the tracked tab,
Pilot state, command results and in-progress reports. It is also the liveness
signal for the connection.
- Context: Connection & Sync.
- Code: `internal/sync/runtime/wire_sync.go:SyncRequest`, `internal/sync/runtime/wire_sync.go:SyncSettings`, `src/background/sync/sync-client.ts:SyncClient`.
- Avoid: "ping" (the content-script `kaboom_ping` is a different message).

### Lost command

A started command that the extension stopped reporting as in progress past a
grace period. The command owner fails it with a daemon-sourced result.
- Context: Browser Command.
- Code: `internal/browsercommand/exchange.go:LostCommand`, `internal/browsercommand/exchange.go:Owner.ApplySyncExchange`.

### Observation

Any captured browser fact retained by a capture store: console log, network
request or body, WebSocket event, user action, performance snapshot or
extension log. Observe reads return observations.
- Context: Observation Capture.
- Code: `internal/tools/observe/core/deps.go:Deps`, `internal/platform/types/wire_log.go`, `internal/platform/types/wire_network.go`.
- Avoid: "telemetry" for captured page data. Telemetry means anonymous product-usage beacons only.

### Operation owner

The single context responsible for the outcome of a cross-context operation:
it validates, mutates and reports the result. Others only hand off to it.
- See: the operations tables in the architecture guides.

### Pilot

The AI Web Pilot. It is the user-controlled permission that lets agents drive
the browser. **Pilot intent** is the user's enabled or disabled choice. Chrome
owns it in `chrome.storage`. The daemon holds only a projection that arrives
on the heartbeat, and gates actions with it.
- Context: Tab Tracking & Pilot (owner). Connection & Sync holds the projection.
- Code: `src/background/runtime-state/pilot-state.ts:isAiWebPilotEnabled`, `src/lib/constants.ts:StorageKey.AI_WEB_PILOT_ENABLED`, `src/background/commands/helpers.ts:requireAiWebPilot`, `internal/sync/runtime/runtime.go:Runtime.IsPilotActionAllowed`.
- Avoid: "autopilot", or "pilot mode" as a daemon setting.

### Port

A small consumer-owned interface that a use case or adapter declares for what
it needs from an owner. The owner satisfies it implicitly.
- Context: Application Use Cases.
- Code: `internal/browsercommand/ports/ports.go:ResultSink`, `internal/browsercommand/query/types.go:QueryRunner`.
- Avoid: producer-owned interfaces. `.interface-baseline.json` ratchets them down.

### Probe sync

A contract-probe /sync request. The daemon answers it with a well-formed empty
envelope and adopts no session, generation or commands from it.
- Context: Connection & Sync.
- Code: `internal/sync/runtime/handler.go:respondProbeSync`.

### Result source

The path a command result arrived on. Each source has its own validation
policy: `sync` (generation-gated, inside a sync exchange), `query_result`
(out-of-band uploads, validated by identity) and `daemon`
(daemon-originated terminal transitions).
- Context: Browser Command.
- Code: `internal/browsercommand/result.go:ResultSource`, `internal/browsercommand/result.go:Owner.SubmitResult`.

### Saved video

A screen-recording file the extension uploads to the daemon with
`POST /recordings/save`. The daemon writes it to disk and completes the
command that requested it.
- Context: Screen Recording (daemon half).
- Code: `internal/screenrec/save.go:HandleSave`.
- Avoid: "recording" alone, and "event recording".

### Screen recording

Live tab video capture. The background service worker owns its lifecycle
(start, restore after service-worker restart, stop). The offscreen document
runs the encoder.
- Context: Screen Recording.
- Code: `src/background/recording/index.ts:startRecording`, `src/background/recording/index.ts:stopRecording`, `src/background/recording/rehydration.ts:resolveRecordingRehydration`, `src/offscreen/recording-worker.ts`, `src/types/runtime-messages.ts:OffscreenStartRecordingMessage`, `internal/screenrec/handlers.go:InteractHandler.HandleRecordStart`.
- Avoid: "tab recording" in new code (it is the feature folder name only), and "video recording manager".

### Stale work

Sync results or progress stamped with a superseded connection generation. The
command owner reports stale work and never applies it.
- Context: Browser Command, gated by Connection & Sync.
- Code: `internal/browsercommand/exchange.go:StaleWork`, `internal/sync/runtime/handler.go:rejectStaleGeneration`.

### Sync exchange

The command work carried by one heartbeat: results, in-progress reports, and
the request for pending commands. Connection & Sync hands it to the command
owner under a generation gate. The response delivers the pending commands.
- Context: Connection & Sync (transport). Browser Command (applies it).
- Code: `internal/browsercommand/exchange.go:SyncExchange`, `internal/browsercommand/exchange.go:ExchangeReport`, `internal/sync/runtime/handler.go:Handler.HandleSync`, `internal/sync/runtime/wire_sync.go:SyncResponse`, `src/types/wire/wire-sync.ts`.
- Avoid: "poll" for the whole exchange. A long-poll is only how the daemon holds the response open.

### Tracked tab

The single browser tab that Kaboom observes and drives. Chrome owns the
selection and persists it. The daemon keeps a projection that orders
synchronous switch_tab results against heartbeats.
- Context: Tab Tracking & Pilot (owner). Connection & Sync holds the projection.
- Code: `src/lib/tabs/tracked-tab-storage.ts:setTrackedTab`, `src/lib/tabs/tracked-tab-storage.ts:readTrackedTab`, `src/lib/tabs/tab-tracking-core.ts:trackTab`, `internal/sync/runtime/tracking_projection.go:observeTrackingLocked`, `internal/sync/runtime/runtime.go:Runtime.GetTrackingStatus`.
- Avoid: "active tab". The active tab is Chrome's focused tab and may differ from the tracked tab.

### Tracking continuity

The extension's state machine for the tracked tab across navigation,
injection, reconnect and close (establish, confirm, navigationStarted, fail,
close). Lifecycle adapters report to it. They do not write tracking state
directly.
- Context: Tab Tracking & Pilot.
- Code: `src/background/runtime-state/tracking-continuity.ts:trackingContinuity`, `src/types/runtime/tracking.ts:TrackingContinuitySnapshot`.

### Use case

A transport-independent application workflow. It takes narrow ports and never
takes MCP request or response types, HTTP, or the capture aggregate. MCP
adapters map its results and failures to responses.
- Context: Application Use Cases.
- Code: `internal/page/app/screenshot/screenshot.go:Service.Capture`, `internal/page/app/pageinfo/pageinfo.go:Service.Read`, `internal/page/app/storage/storage.go:Service.Storage`, `internal/analysis/app/a11yaudit/a11yaudit.go:Service.Audit`.
- Avoid: putting workflow logic in `internal/tools/` adapters when it does not depend on MCP.

### Wire type

The source-of-truth shape of an HTTP payload between extension and daemon.
The Go `wire_*.go` file and the TypeScript `wire-*.ts` file must change
together (`make check-wire-drift`).
- Code: `internal/sync/runtime/wire_sync.go`, `src/types/wire/wire-sync.ts`, `internal/platform/types/wire_enhanced_action.go`, `src/types/wire/wire-enhanced-action.ts`.

## Words to avoid globally

| Avoid | Say instead |
| --- | --- |
| "recording" (bare) | event recording, screen recording or saved video |
| "telemetry" for page data | observation. Telemetry is anonymous product usage only |
| "query" for a tracked command | browser command (with a correlation ID) |
| "active tab" for Kaboom's tab | tracked tab |
| "session" (bare) | extension session, MCP client, or event recording |
| "Capture" as a dependency | the specific capture store or runtime owner |

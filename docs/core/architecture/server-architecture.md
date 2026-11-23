# Server Architecture

Last reviewed: 2026-11-21

## Overview

The Kaboom server is a zero-dependency Go binary. It receives browser
observations from the Chrome extension over HTTP and exposes them to AI coding
agents through MCP (JSON-RPC 2.0 over stdio or HTTP). It also sends browser
commands back to the extension. Captured browser and user data stays local.
Only anonymous product-usage telemetry may leave the machine. Users can turn it
off with `KABOOM_TELEMETRY=off`.

Domain terms in **bold** are defined in [GLOSSARY.md](../../../GLOSSARY.md).
For the extension half, see
[extension-architecture.md](extension-architecture.md).

## Modes

The binary (`kaboom-agentic-browser`) runs in one of three modes. `main()` in
`cmd/browser-agent/main.go` selects the mode.

- **Bridge mode** (the default). Stdio MCP transport. It spawns or reuses a
  persistent HTTP daemon and forwards JSON-RPC between stdio and the daemon's
  `/mcp` endpoint. If the daemon dies, the bridge respawns it.
  Code: `internal/lifecycle/bridge/bridge.go:Runner`.
- **Daemon mode** (`--daemon`). An HTTP server for the extension endpoints and
  `/mcp`. Code: `cmd/browser-agent/main_connection_mcp.go`, `cmd/browser-agent/server.go`.
- **CLI mode** (`kaboom-agentic-browser observe errors ...`). It sends one tool
  call to a running daemon and formats the result.
  Code: `internal/tools/cli/cli.go:Run`.

## Bounded contexts

Each context owns its state. Other contexts reach that state only through the
public interface listed here. `make check-layering` enforces the arrows (see
[Enforcement](#enforcement)).

| Context | Owned state | Packages | Public interface | May depend on |
| --- | --- | --- | --- | --- |
| **Connection & Sync** | Extension session, **connection generation**, liveness and disconnect, settings and extension-log intake, **command contract ID**, the daemon projections of the **tracked tab** and **Pilot** | `internal/sync/runtime/` | `internal/sync/runtime/handler.go:Handler.HandleSync`, `internal/sync/runtime/runtime.go:Runtime` (read accessors: `Runtime.GetTrackingStatus`, `Runtime.IsPilotActionAllowed`, `Runtime.WaitForExtensionConnected`), wire `internal/sync/runtime/wire_sync.go` | Browser Command (`Owner` only), `internal/observation/store/logs/`, leaf utilities |
| **Browser Command** | Queue, delivery eligibility, acks, progress, results from every **result source**, expiry, **lost commands** | `internal/browsercommand/`, `internal/browsercommand/query/` (types) | `internal/browsercommand/owner.go:Owner`, `internal/browsercommand/exchange.go:SyncExchange`, `internal/browsercommand/result.go:Result`, `internal/browsercommand/query/types.go:QueryRunner` | stdlib only. No HTTP, MCP or capture |
| **Observation Capture** | Ring buffers and queries for logs, network, WebSocket, actions, performance, extension logs | `internal/observation/store/telemetry/`, `internal/observation/store/logs/`, `internal/observation/store/perf/`, `internal/observation/store/actions/`, `internal/observation/store/bodies/`, `internal/observation/store/waterfall/`, `internal/observation/store/ring/` | Each store's `Store` type (for example `internal/observation/store/telemetry/store.go:Store`) | Its own ring stores, `internal/platform/types/` |
| **Extension Ingestion** | None (adapter). Validates extension uploads and writes them to their owners | `internal/ingestion/http/`, `internal/annotation/adapters/http/` | `internal/ingestion/http/handlers.go:Handlers` | Capture stores, Browser Command (`Result`, `SubmitResult`), Event Recording (storage reads) |
| **Event Recording** | Event recordings, their disk persistence and quotas | `internal/recording/` (with `internal/recording/playback/`, `internal/recording/logdiff/`, `internal/recording/actionlog/`) | `internal/recording/manager.go:RecordingManager` | Capture stores (read), leaf state packages |
| **Screen Recording** (daemon half) | Saved video files and their metadata | `internal/screenrec/` | `internal/screenrec/handlers.go:InteractHandler`, `internal/screenrec/save.go:HandleSave` | Browser Command (`Owner`, `ports.ResultSink`) |
| **Application Use Cases** | None (workflows) | `internal/page/app/`, `internal/analysis/app/` (each context's app folder) | `internal/page/app/screenshot/screenshot.go:Service`, `internal/page/app/pageinfo/pageinfo.go:Service`, `internal/page/app/storage/storage.go:Service`, `internal/analysis/app/a11yaudit/a11yaudit.go:Service`, `internal/browsercommand/ports/ports.go:ResultSink` | Browser Command (`QueryRunner`), leaf packages. No HTTP, MCP, capture or `internal/tools` |
| **Tool Surface** | Per-call MCP state only (rate limits, response shaping) | `internal/tools/mcp/`, `internal/tools/schema/`, `internal/tools/`, `internal/tools/observe/handler/`, `internal/tools/interact/handler/`, `internal/tools/analyze/handler/`, `internal/tools/generate/handler/`, `internal/tools/configure/handler/`, `internal/tools/recording/`, `internal/tools/asynccommand/` | The five tools (observe, analyze, generate, configure, interact) | Everything above, through owners and use cases |
| **Composition** | Object graph only | `cmd/browser-agent/server.go`, `internal/composition/toolruntime/`, `internal/composition/capture/capture.go` | `cmd/browser-agent/server.go:setupHTTPRoutes`, `internal/composition/toolruntime/tools_core.go:NewToolHandler` | Everything |
| **Process Lifecycle** | Bridge, daemon spawn and respawn, single instance, Doctor diagnostics | `internal/lifecycle/bridge/`, `internal/lifecycle/daemon/`, `internal/lifecycle/instance/gov/`, `internal/lifecycle/doctor/statediag/` | `internal/lifecycle/bridge/bridge.go:Runner` | Leaf utilities |
| **Product Telemetry** | Anonymous install and usage counters | `internal/telemetry/` | `internal/telemetry/beacon.go` | Leaf utilities |

### Context map

```
                          MCP client (agent)
                                |  JSON-RPC (/mcp, stdio bridge)
                                v
                 +------------------------------+
                 | Tool Surface (adapters)      |-----------------+
                 +------------------------------+                 |
                   |            |             |                   |
         use cases |   enqueue/ | wait        | reads             | configure
                   v            v             v                   v
        +----------------+  +----------------+  +-------------------+  +----------------+
        | Application    |->| Browser Command|  | Observation       |  | Event Recording|
        | Use Cases      |  | (Owner)        |  | Capture (stores)  |  | (Manager)      |
        +----------------+  +----------------+  +-------------------+  +----------------+
                                ^       ^              ^      ^               ^
              SyncExchange under|       | SubmitResult |      | writes        | storage reads
              GenerationGate    |       |              |      |               |
        +---------------------------+  +-----------------------------------------------+
        | Connection & Sync         |  | Extension Ingestion (HTTP adapters)           |
        | (Runtime, Handler)        |  | httpingest, mediaapi, screenrec save          |
        +---------------------------+  +-----------------------------------------------+
                    ^  /sync (wire_sync.go)                ^  POST uploads
                    +------------- Chrome extension -------+
```

Arrows point from the caller to the owner it calls. The Composition context
builds every box and is omitted.

## Composition roots

- `cmd/browser-agent/server.go:setupHTTPRoutes` builds the HTTP surface. It
  wires `/sync` through `cmd/browser-agent/server.go:newSyncHandler` to
  `internal/sync/runtime/handler.go:NewHandler`, binds the ingestion
  routes to `internal/ingestion/http/handlers.go:Handlers`, and binds
  `/recordings/save` to `internal/screenrec/save.go:HandleSave`.
- `internal/composition/toolruntime/tools_core.go:NewToolHandler` builds
  the Tool Surface. It injects owners and use cases
  (`internal/composition/toolruntime/usecase_wiring.go:newScreenshotService`)
  into each tool dispatcher.
- `internal/composition/capture/capture.go:NewCapture` constructs the stores and runtimes.
  Only `cmd/browser-agent` may call it. The **capture aggregate**
  (`internal/composition/capture/capture.go:Capture`) is a wiring object held only by the
  composition root (`cmd/browser-agent` and the `toolruntime` wiring). Every
  feature receives the owner it uses. Health, doctor, dashboard and the
  operational API receive the `internal/observation/healthreader/reader.go:Owners`
  bundle built by `internal/composition/capture/capture.go:Capture.HealthOwners`. The MCP
  guards (`internal/tools/guard/`), `insecureproxy` and
  `mcpresponse` take `*syncruntime.Runtime`. `clientapi` and `terminal` take
  the client registry.

## Operation owners and handoffs

Each row is a cross-context operation. The **owner** decides the outcome. The
**handoff** is the contract crossed, and the direction is caller to callee.

| Operation | Owner | Flow and handoff contracts |
| --- | --- | --- |
| Command enqueue | Browser Command | Tool Surface → `Owner.Enqueue(PendingQuery, timeout, clientID)` (`internal/browsercommand/owner.go:Owner.Enqueue`). MCP-facing async wrapper: `internal/tools/asynccommand/handler.go:Handler.EnqueuePendingQuery`. The returned **correlation ID** goes to the agent |
| Command delivery | Browser Command (eligibility). Connection & Sync (transport) | Extension heartbeat → `Handler.HandleSync` → `Handler.GetPendingQueriesDisconnectAware` → `Owner.Pending`. Commands are stamped with the current **connection generation** in `buildSyncCommands` and returned as `SyncResponse.Commands` (`internal/sync/runtime/wire_sync.go:SyncCommand` ↔ `src/types/wire/wire-sync.ts`) |
| Command ack and progress | Browser Command | Next heartbeat: `SyncRequest` last-ack and in-progress list → `Handler.applySyncExchange` builds `browsercommand.SyncExchange` → `Owner.ApplySyncExchange` under a `GenerationGate`. Reports past the grace period become **lost commands** (`internal/browsercommand/exchange.go:LostCommand`) |
| Command result via /sync | Browser Command | Extension `SyncClient.queueCommandResult` → `SyncRequest` command results → `SyncExchange.Results` → applied as `SourceSync` only while the gate confirms the generation is current. **Stale work** is reported, not applied (`internal/browsercommand/exchange.go:StaleWork`) |
| Command result out of band | Browser Command | `POST /query-result` → `internal/ingestion/http/handlers.go:Handlers.HandleQueryResult` → `Owner.SubmitResult` (`SourceQueryResult`, validated by identity). Upload adapters complete commands the same way through `internal/browsercommand/ports/ports.go:ResultSink` (`internal/annotation/adapters/http/handler.go`, `internal/screenrec/save.go`) |
| MCP wait for result | Browser Command (state). Tool Surface (formatting) | `asynccommand.Handler` waits with `Owner.WaitCommand` (commands) or `Owner.WaitQueryResult` (**browser queries**), then formats with `Handler.FormatCommandResult` |
| Disconnect and expiry | Connection & Sync (detects). Browser Command (fails work) | `Runtime.Disconnected` → `Owner.ExpireAll("extension_disconnected")`. A waiter that sees a disconnect calls `Owner.FailDisconnected` (a `SourceDaemon` result) |
| Reconnect and generation bump | Connection & Sync | A new extension session in `SyncRequest` → `bumpSyncGenerationForNewSession`. Superseded requests → `rejectStaleGeneration`. The new generation is returned in `SyncResponse` and mirrored by the extension (`src/background/runtime-state/connection-generation.ts:setConnectionGeneration`). **Probe syncs** adopt nothing (`respondProbeSync`) |
| Tab switch (synchronous) | Tab Tracking & Pilot (selection). Connection & Sync (projection) | `interact({action: "switch_tab"})` → toolinteract enqueues a command → the extension persists the new tab before it returns (`src/background/exec/browser-actions.ts:executeSwitchTabAction`) → the result reaches `applySwitchTabTracking` (`internal/tools/interact/handler/interact_browser.go`) → `Runtime.UpdateTrackedTab`. This is a command-confirmed observation that opens a hold in `observeTrackingLocked` |
| Tab switch (asynchronous: user pick, close, navigation) | Tab Tracking & Pilot | Chrome adapters update the tracked tab in `chrome.storage` → the next heartbeat's `SyncSettings` → `observeTrackingLocked`. The hold drops up to `staleHeartbeatAllowance` heartbeats that still name the old tab, then the extension wins |
| Content readiness | Tab Tracking & Pilot (extension) | Extension-internal barrier (`src/background/tab-tracking/content-readiness.ts:ContentReadinessBarrier`). The daemon sees only the heartbeat tab status (`Runtime.GetTabStatus`) |
| Pilot gating | Tab Tracking & Pilot (intent). Connection & Sync (projection) | `SyncSettings.PilotEnabled` → `Runtime.IsPilotActionAllowed` gates interact actions in the daemon. The extension re-checks with `requireAiWebPilot` before it executes |
| Event recording start, stop, persist | Event Recording | `configure({what: "event_recording_start"})` → `internal/tools/recording/handler.go:Handler.EventRecordingStart` → `RecordingManager.StartRecording`. Stop → `RecordingManager.StopRecording` persists to disk. Storage reads: `GET /recordings/storage` → `Handlers.HandleRecordingStorage` |
| Screen recording start and stop | Screen Recording (extension owns the live lifecycle) | `interact({action: "screen_recording_start"})` → `InteractHandler.HandleRecordStart` → `Owner.Enqueue` (command `screen_recording_start`) → /sync → extension. See the extension guide for start, restore and stop |
| Saved video | Screen Recording (daemon half) | The offscreen worker sends `POST /recordings/save` → `screenrec.HandleSave` writes the file and calls `SubmitResult` with the upload's query ID, which completes the stop command |
| Screenshot | Application Use Cases | observe(screenshot) → `internal/tools/observe/page/` adapter → `internal/page/app/screenshot/screenshot.go:Service.Capture` → `QueryRunner` (Browser Command) → extension `screenshot` command. The adapter maps `Result` and `Failure` to MCP image blocks and errors |
| Observe reads | Observation Capture | `internal/tools/observe/handler/dispatcher.go` → `internal/tools/observe/` modes → `internal/tools/observe/core/deps.go:Deps` (stores and Runtime). No browser command is involved |
| Observation ingestion | Observation Capture (retention). Extension Ingestion (adapter) | Extension batchers → `/network-bodies`, `/network-waterfall`, `/enhanced-actions`, `/websocket-events`, `/performance-snapshots` → `Handlers` → stores. Extension logs ride `SyncRequest` into `internal/observation/store/logs/` |

### Current behavior and target

- **Done (jdm5.2 to jdm5.6):** one command owner (`internal/browsercommand/`)
  with a private dispatcher, generation-gated sync exchanges, validated result
  sources, the tracked-tab ordering rule, explicit recording ownership, and
  four transport-independent use cases in `internal/page/app/` and `internal/analysis/app/`. Only the
  composition root holds `*capture.Capture` (jdm5.5). Every tool, health,
  guard and API package takes the owners it uses, and the `capture-root`
  layering rule enforces it. Per-client dispatcher helpers were deleted.
- **Gap:** `RecordingManager.AddRecordingAction` has no production caller, so
  event recordings currently record lifecycle and metadata but not actions.
  The target is an ingestion handoff from Observation Capture to Event
  Recording.
- **Gap:** `internal/recording/actionlog/` imports `internal/tools/interact`
  (Tool Surface). The target moves the shared action types to a leaf package.

## Enforcement

`make check-layering` (`scripts/contracts/layering/`) fails the build when an
arrow is broken:

- `scripts/contracts/layering/main.go` enforces the global matrix:
  `internal/**` never imports `cmd/**`; `internal/platform/types` is a leaf;
  `internal/tools/mcp` does not import tools or capture; the command module and
  `internal/*/app/**` have no HTTP, MCP or capture imports; `capture-root` allows
  only `cmd/browser-agent`, its toolruntime wiring, the `internal/composition/capture/`
  tree and test-support packages to import `internal/composition/capture`; `test-support`
  keeps production code from importing test-support packages;
  `capture.NewCapture` is called only by the composition root.
- `scripts/contracts/layering/command_owner.go` keeps the command owner as the
  only dispatcher path.
- `scripts/contracts/layering/contexts.go:boundedContexts` holds the
  per-context rules. Observation Capture, Event Recording, Connection & Sync
  and Extension Ingestion each list the upstream contexts they may not import.
  Violations print the context name from this guide.
- `scripts/contracts/layering/context_roots.go:contextRoots` enforces the
  physical layout: every package under `internal/` sits in a declared context
  root (`internal/<context>/...`). A new top-level folder fails the build until
  it is moved into a context or declared there with its owning context.

## Wire types

Wire types are the source of truth for HTTP payload shapes. Go `wire_*.go`
files (for example `internal/sync/runtime/wire_sync.go` and
`internal/platform/types/wire_network.go`) pair with TypeScript files in
`src/types/wire/`. `make check-wire-drift` enforces that they match.

## Key patterns

- **Explicit dependencies.** Owners accept concrete owners or narrow
  consumer-owned ports. The `.interface-baseline.json` ratchet rejects
  producer-owned and dead interfaces.
- **Zero external dependencies.** stdlib only. Operator diagnostics go through
  `internal/lifecycle/doctor/statediag/`.
- **Append-only I/O on hot paths.** Ring stores evict in a single pass.

## "Change X, edit Y" lookup

| Want to... | Edit |
| --- | --- |
| Add an observe mode | `internal/tools/observe/` (mode) and `internal/tools/observe/handler/registry.go` |
| Add a transport-independent workflow | A new package under the owning context's app folder (for example `internal/page/app/`), wired in `internal/composition/toolruntime/usecase_wiring.go` |
| Add a browser command | Daemon: enqueue through `Owner` from the tool adapter. Extension: `registerCommand` in `src/background/commands/` |
| Change the sync protocol | `internal/sync/runtime/handler.go`, `internal/sync/runtime/wire_sync.go`, `src/types/wire/wire-sync.ts`, `src/background/sync/sync-client.ts` |
| Add an extension HTTP endpoint | `internal/ingestion/http/` (handler) and `cmd/browser-agent/server.go` (route) |
| Add a bounded-context rule | `scripts/contracts/layering/contexts.go`, with a test in `scripts/contracts/layering/contexts_test.go` |

## Testing

```bash
make test                         # Go and extension tests
go test ./internal/...            # Internal packages
make check-layering               # Context and layering rules
./scripts/uat/runners/test-all-tools-comprehensive.sh   # UAT against the npm install
```

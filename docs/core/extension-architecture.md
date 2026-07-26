# Extension Architecture

## Overview

The Kaboom Chrome extension (MV3) captures browser telemetry (console logs, network requests, WebSocket events, user actions, performance data) and relays it to a local Go server over HTTP. It also receives commands from the server (DOM queries, JS execution, navigation) and executes them in the page. The extension operates across three execution contexts that communicate via two message channels: `chrome.runtime` messages and `window.postMessage`.

## Execution Contexts

**Background Service Worker** (`src/background/`) -- The orchestration layer. Manages server connection via a unified `/sync` endpoint, batches captured data before sending, dispatches incoming commands to target tabs, and owns all persistent state (settings, connection status, circuit breakers). Cannot access the DOM.

**Content Script** (`src/content/`) -- The message relay. Bridges background and inject contexts. Also renders UI overlays (toasts, subtitles, draw-mode). Runs in an isolated world per tab -- can access the DOM but not page JavaScript globals.

**Inject Script** (`src/inject/`) -- Page instrumentation. Runs in the MAIN world with full access to page JS globals. Monkey-patches `fetch`, `XMLHttpRequest`, `console.*`, `WebSocket`, and `addEventListener` to capture telemetry. Executes JS commands and DOM queries. Bundled via esbuild into `inject.bundled.js`.

## Message Flow

```
                    CAPTURE (upstream)
  +-----------+   window.postMessage   +---------+   chrome.runtime   +------------+   HTTP POST
  |  Inject   | ---------------------> | Content | -----------------> | Background | -----------> Go Server
  |  (MAIN)   |                        | (isolated)                   |  (SW)      |   /logs
  +-----------+                        +---------+                    +------------+   /ws-events
       |                                    |                              |            /actions
       | captures: console, fetch,          | forwards: log, ws,          | batches    /network-bodies
       | XHR, WebSocket, actions,           | action, network, perf       | & sends    /perf-snapshots
       | performance, exceptions            | messages to background       |
       |                                    |                              |
       |                                    |                              |
                    COMMANDS (downstream)                                   |
  +-----------+   window.postMessage   +---------+   chrome.runtime   +------------+   /sync
  |  Inject   | <--------------------- | Content | <----------------- | Background | <----------- Go Server
  |  (MAIN)   |                        | (isolated)                   |  (SW)      |   (long-poll)
  +-----------+                        +---------+                    +------------+
       |                                    |                              |
       | executes: JS, DOM queries,         | relays: execute_js,         | dispatches via
       | a11y audits, settings,             | dom_query, a11y_query,      | command registry
       | state capture/restore,             | highlight, settings,        | (Map-based)
       | highlight, link health             | state commands              |
```

Security: Content-to-inject messages are authenticated with a per-page-load cryptographic nonce. Background validates sender origin on all incoming `chrome.runtime` messages.

## Directory Map

```
src/
  background/              Service worker modules
    index.ts               State container, exports, batchers, connection management
    message-handlers.ts    chrome.runtime.onMessage routing (DI-based)
    sync-client.ts         Unified /sync endpoint client (replaces individual polling)
    sync-manager.ts        Sync client lifecycle (start/stop/reset)
    batcher-instances.ts   Factory for debounced batchers (logs, ws, actions, network, perf)
    communication.ts       HTTP helpers, badge updates, circuit breaker primitives
    dom-primitives.ts      Self-contained DOM functions for chrome.scripting.executeScript
    dom-dispatch.ts        Dispatches dom-primitives to target tabs
    pending-queries.ts     Legacy command dispatch (being replaced by commands/)
    server.ts              Server URL and request header management
    init.ts                Extension initialization sequence
    commands/              Async command registry (Map-based dispatch)
      registry.ts          registerCommand() / dispatch() entry point
      helpers.ts           Target tab resolution, result wrapping
      interact.ts          Browser interaction commands (click, type, navigate...)
      analyze.ts           DOM/a11y query commands
      observe.ts           Observation commands (screenshot, etc.)

  content/                 Content script (isolated world)
    runtime-message-listener.ts  chrome.runtime message routing from background
    window-message-listener.ts   window.postMessage routing from inject
    message-handlers.ts          Background message delegation and inject forwarding
    script-injection.ts          Injects inject.bundled.js, generates per-page nonce
    message-forwarding.ts        Maps inject message types to background message types
    request-tracking.ts          Pending request/response correlation maps
    tab-tracking.ts              Tab isolation (only forward from tracked tab)
    ui/
      toast.ts                   Action toast overlay
      subtitle.ts                Subtitle and recording watermark overlay

  inject/                  MAIN world page instrumentation (bundled)
    index.ts               Entry point, installs all capture hooks
    message-handlers.ts    Dispatches messages from content script
    settings.ts            Handles capture toggle messages
    execute-js.ts          JS execution sandbox (Function constructor + timeout)
    state.ts               Browser state capture/restore
    observers.ts           MutationObserver and interception deferral
    api.ts                 Public API exposed on window (if needed)

  lib/                     Shared utilities (used by inject via bundling)
    network.ts             fetch/XHR monkey-patching, waterfall capture
    console.ts             console.* monkey-patching
    exceptions.ts          Global error/unhandledrejection capture
    websocket.ts           WebSocket monkey-patching
    actions.ts             User action capture (click, input, scroll, navigation)
    performance.ts         Performance marks/measures capture
    perf-snapshot.ts       Performance snapshot (timing, memory, resources)
    dom-queries.ts         DOM query execution engine
    link-health.ts         Link health checker
    constants.ts           Shared constants (limits, sensitive headers, timeouts)
    serialize.ts           Safe serialization utilities

  types/                   TypeScript type definitions
    index.ts               Barrel re-exports
    messages.ts            Inner barrel over capture/ + runtime/
    runtime-messages.ts    Message type discriminated unions (rule 20 contract)
    utils.ts               Generic type helpers and type guards
    global.d.ts            Ambient globals (KaboomAPI, axe, Performance)
    wire/                  Wire types (generated from Go, CI-enforced)
    capture/               Shapes of page-derived telemetry
      telemetry.ts, network.ts, websocket.ts, performance.ts,
      actions.ts, accessibility.ts, ai-context.ts, dom.ts, sourcemap.ts
    runtime/               The extension's own runtime state
      chrome.ts, debug.ts, queries.ts, state.ts
```

## Key Patterns

- **Dependency injection** -- `MessageHandlerDependencies` interface in `message-handlers.ts` decouples the handler logic from module-level state, enabling isolated unit testing.
- **Command registry** -- `commands/registry.ts` uses a `Map<string, CommandHandler>` for async command dispatch. Each command file calls `registerCommand()` at import time. The `CommandContext` bundles query, sync client, target tab, and wrapped result senders.
- **Batcher pattern** -- `batcher-instances.ts` creates debounced batchers (one per data type) that accumulate entries and flush to the server on a timer. Each batcher is wrapped in a shared circuit breaker to back off on server failures.
- **Sync client** -- A single `/sync` long-poll endpoint replaces individual POST endpoints for extension-to-server communication. The sync loop sends settings + extension logs upstream and receives commands downstream.
- **Wire types** -- `src/types/wire/wire-*.ts` are generated from `internal/types/wire_*.go`. CI runs `make check-wire-drift` to ensure they stay in sync. These define the exact HTTP payload shapes.
- **Nonce validation** -- Content script generates a cryptographic nonce per page load, passes it to inject via a `data-kaboom-nonce` attribute on the script element. All `window.postMessage` calls from content include the nonce; inject validates it before processing.
- **Tab isolation** -- Content script's `window-message-listener.ts` filters captured data so only the tracked tab's telemetry is forwarded to background.

## Constraints

- **dom-primitives.ts must be self-contained** -- Functions in this file are serialized by `chrome.scripting.executeScript({ func })`. They cannot reference closures, imports, or external variables. Types are duplicated intentionally.
- **Content and inject scripts must be bundled** -- MV3 content scripts cannot use ES module imports at runtime. `inject.bundled.js` and `content.bundled.js` are built by esbuild. Source changes in `src/inject/` or `src/lib/` used by inject require `make compile-ts`.
- **No dynamic imports in background service worker** -- MV3 service workers do not support `import()`. All background modules must be statically imported. (Content script can use dynamic import for lazy-loaded modules like `draw-mode.js`.)
- **lib/ code used by inject must go through esbuild** -- Anything in `src/lib/` that inject uses gets bundled into `inject.bundled.js`. Adding a new lib dependency requires `make compile-ts` to take effect.

## "Change X, Edit Y" Lookup

| Want to...                          | Edit these files                                                                                            |
|-------------------------------------|-------------------------------------------------------------------------------------------------------------|
| Add a new capture type              | `src/lib/<capture>.ts`, `src/inject/index.ts` (install hook), `src/types/` (types), `src/background/sync/batcher-instances.ts` (new batcher) |
| Add a new capture toggle            | `src/lib/constants.ts`, `src/types/runtime-messages.ts`, `src/inject/settings.ts`, `src/content/message-handlers.ts` (TOGGLE_MESSAGES set) |
| Add a new async command             | `src/background/commands/<name>.ts` (handler + registerCommand), `src/background/commands/helpers.ts` (if new helper needed) |
| Add a new DOM query type            | `src/background/dom/primitives/dom-primitives.ts` (self-contained function), `src/background/dom/dom-dispatch.ts` (dispatch case) |
| Change server communication         | `src/background/sync/sync-client.ts`, `src/background/sync/server.ts`                                                |
| Add UI overlay                      | `src/content/ui/<name>.ts`, `src/content/runtime-message-listener.ts` (add message handler)                 |
| Add page instrumentation            | `src/inject/` + `src/lib/`, then `make compile-ts`                                                         |
| Add a wire type                     | `internal/types/wire_*.go` (Go source of truth), run `make generate-wire-types`, verify with `make check-wire-drift` |
| Add a background message handler    | `src/background/message-handlers.ts` (add case in handleMessage), update `MessageHandlerDependencies` if new deps needed |
| Add a content-to-inject message     | `src/content/message-handlers.ts` (postToInject), `src/inject/message-handlers.ts` (dispatch case), `src/content/window-message-listener.ts` (response handler) |
| Add extension test                  | `tests/extension/<name>.test.js` (Node.js test runner, factory-based mocks)                                |

## Testing

Tests live in both `tests/extension/` and `extension/background/` and use the Node.js built-in test runner. Chrome APIs are mocked via factory functions (see `tests/extension/helpers.js`). The mocks cover `chrome.runtime`, `chrome.storage`, `chrome.tabs`, and `chrome.scripting`.

```bash
npm run test:ext       # Run all extension tests (both roots, sharded)
npm test               # Run all tests (extension + other)
make test              # Run Go + extension tests
```

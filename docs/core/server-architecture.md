# Server Architecture

## Overview

The Kaboom server is a zero-dependency Go binary that receives browser telemetry from the Chrome extension over HTTP and exposes it to AI coding agents via the MCP protocol (JSON-RPC 2.0 over stdio or HTTP). All data stays local -- nothing leaves the machine.

For the extension half of the system, see [extension-architecture.md](extension-architecture.md).

## Modes

The binary (`kaboom-agentic-browser`) runs in one of three modes:

**Bridge mode** (default) -- Stdio MCP transport. Spawns a persistent HTTP daemon in the background (or connects to an existing one), then forwards JSON-RPC messages between stdin/stdout and the daemon's `/mcp` endpoint. The daemon survives after the bridge exits. If the daemon dies mid-session, the bridge detects it and respawns transparently. Entry: `runBridgeMode()` in `bridge.go`.

**Daemon mode** (`--daemon`) -- HTTP server only. Binds a port (default 7890), serves both extension HTTP endpoints and the `/mcp` JSON-RPC endpoint. Manages PID files and handles SIGTERM/SIGINT for graceful shutdown. Entry: `runMCPMode()` in `main_connection_mcp.go`.

**CLI mode** (`kaboom-agentic-browser observe errors ...`) -- Direct tool access from the terminal. Parses CLI flags into MCP tool arguments, sends a single JSON-RPC request to a running daemon over HTTP, formats the response as human/JSON/CSV, and exits. Entry: `runCLIMode()` in `cli.go`, parsers in `cli_commands.go`.

Mode selection: `main()` checks for CLI mode first (args match a tool name), otherwise `selectRuntimeMode()` picks bridge or daemon based on flags. Bridge is the default because MCP clients expect stdio transport.

## Package Layout

```
cmd/browser-agent/               Main binary
  main.go                      Entry point, flag parsing, mode dispatch
  bridge.go                    Bridge mode: stdio-to-HTTP forwarding, daemon spawn/respawn
  main_connection_mcp.go       Daemon mode: HTTP server startup, PID management, signals
  cli.go, cli_commands.go      CLI mode: arg parsing, HTTP dispatch, output formatting
  server.go                    Server state, subsystem startup, and HTTP route registration
  handler.go                   MCPHandler: JSON-RPC dispatch, initialize/tools/list
  tools_core.go                ToolHandler struct, rate limiter, response helpers
  internal/toolobserve/dispatcher.go             observe tool: dispatches to internal/tools/observe
  tools_analyze.go             analyze tool: DOM, a11y, security, performance
  internal/toolgenerate/dispatcher.go            generate tool: Playwright tests, HAR, CSP, SARIF
  tools_configure.go           configure tool: noise rules, storage, streaming, health
  tools_interact.go            interact tool: click, type, navigate, execute JS
  tools_schema.go              JSON Schema definitions for all 5 tools
  tools_*_schema.go            Per-tool schema files (extracted)

internal/
  mcp/                         MCP protocol types and dependency interfaces
    types.go                   MCPToolResult, MCPInitializeResult, MCPCapabilities, etc.
    deps.go                    Composable interfaces: CaptureProvider, LogBufferReader, etc.
    response.go                Response construction helpers
  capture/                     Extension state and telemetry buffer management
    extension_state.go         Connection tracking, pilot status, tab tracking
    sync.go                    POST /sync endpoint: bidirectional extension communication
    accessor.go                Safe read accessors for buffer counters/timestamps
    settings.go                Capture settings handler
  tools/                       Extracted pure functions (thin packages, no global state)
    observe/                   Buffer queries: errors, logs, network, websocket, actions
    analyze/                   Link validation, DOM analysis
    configure/                 Audit, boundaries, capabilities, rewrite
    generate/                  CSP generation, test script generation
    interact/                  Selector resolution, state management, workflows
  types/                       Wire types: Go structs that define HTTP payload shapes
    wire_*.go                  Source of truth -- TS counterparts generated from these
    log.go, network.go         Core domain types
  ai/                          Noise filtering, session store
  analysis/                    API contract validation, third-party auditing
  annotation/                  Draw-mode annotation storage and retrieval
  audit/                       Session audit trail (append-only)
  bridge/                      Bridge mode helpers (timeouts, request forwarding)
  buffers/                     Generic ring buffer primitives
  export/                      Data export formatters
  pagination/                  Cursor-based pagination logic
  performance/                 Performance snapshot types and wire types
  queries/                     Async command queue (extension command dispatch)
  recording/                   Video recording management
  redaction/                   Sensitive data scrubbing
  reproduction/                Bug reproduction script generation
  schema/                      JSON Schema utilities
  security/                    Security scanning (headers, cookies, credentials)
  session/                     Session verification and management
  server/                      Server configuration helpers
  state/                       Runtime state directory paths (PID files, logs, screenshots)
  streaming/                   Alert buffer and push notification support
  testgen/                     Playwright test generation from captured data
  testing/                     Test utilities
  ttl/                         TTL-based eviction logic
  upload/                      File upload handling (screenshots, recordings)
  util/                        SafeGo, string helpers, misc utilities
```

## Data Flow

```
 TELEMETRY INGESTION (extension -> server)

  Chrome Extension                    Go Server (daemon)                    AI Agent
  +---------------+   POST /sync    +------------------+                  +----------+
  | Background SW | --------------> | Capture          |                  |          |
  | (batches data)|   /network-*    |  - ring buffers  |                  |          |
  |               |   /ws-events    |  - extension     |                  |          |
  |               |   /actions      |    state         |                  |          |
  +---------------+   /perf-*       +--------+---------+                  |          |
                                             |                            |          |
                                             v                            |          |
 MCP TOOL CALLS (agent -> server -> response)                             |          |
                                                                          |          |
                      +------------------+   JSON-RPC    +-----------+    |          |
                      | ToolHandler      | <------------ | Bridge    | <- | stdio    |
                      |  .observe()      |   (or HTTP    | (stdio->  |    |          |
                      |  .analyze()      |    /mcp)      |  HTTP)    |    |          |
                      |  .generate()     |               +-----------+    |          |
                      |  .configure()    |                                |          |
                      |  .interact()     | -----------------------------> | response |
                      +------------------+   JSON-RPC result              +----------+
                        |           |
                        v           v
                  internal/    internal/
                  tools/*      capture/
                  (pure fns)   (buffer reads)

 ASYNC COMMANDS (server -> extension -> result)

  ToolHandler ---queue---> Capture.queries ---/sync response---> Extension
  Extension ---POST /query-result---> Capture ---poll---> ToolHandler
```

## Key Patterns

**Tool composition root** -- `ToolHandler` in `tools_core.go` constructs the concrete feature owners and injects their narrow dependencies. Canonical dispatchers such as `internal/toolobserve/dispatcher.go` own their mode registries; the root does not mirror or forward those APIs.

**Explicit dependencies** -- Feature owners accept concrete owners or narrow
function-field dependency values. Shared host/provider interfaces were deleted
because they forced the composition root to mirror unrelated APIs.
`internal/asynccommand.Handler` owns the full queue-to-result lifecycle and its
functions are injected directly into tool handlers.

**Wire types** -- `internal/types/wire_*.go` are the source of truth for HTTP payload shapes between extension and server. TypeScript counterparts in `src/types/wire/wire-*.ts` are generated from Go. CI enforces drift detection via `make check-wire-drift`.

**Zero external dependencies** -- The entire Go server uses only stdlib. No logging library, HTTP framework, or JSON library. `internal/diag` owns operator diagnostics, while lifecycle logs use append-only file I/O.

**Append-only I/O on hot paths** -- Log writes go through a buffered channel (`logChan`) to an async goroutine. Ring buffers in `Capture` use single-pass eviction (never loop-remove-recheck).

**Bridge respawn** -- The bridge monitors daemon health. If the daemon becomes unresponsive, `daemonState.respawnIfNeeded()` re-launches it transparently. Only one respawn runs at a time (mutex + channel coordination).

## "Change X, Edit Y" Lookup

| Want to...                              | Edit these files                                                                                          |
|-----------------------------------------|-----------------------------------------------------------------------------------------------------------|
| Add a new MCP tool                      | `tools_<name>.go` (handler), `tools_<name>_schema.go` (schema), `tools_core.go` (register in ToolHandler) |
| Add a new observe mode                  | `internal/tools/observe/` (handler), `internal/toolobserve/dispatcher.go` (register in its private mode map)                 |
| Add a new analyze/generate/etc. action  | `internal/tools/<tool>/` (logic), `tools_<tool>.go` (register in dispatch map)                            |
| Add an HTTP endpoint for the extension  | `internal/capture/` (handler), `server.go` (register in setupHTTPRoutes)                           |
| Change sync protocol                    | `internal/capture/syncruntime/handler.go` (server), `src/background/sync/sync-client.ts` (extension)                         |
| Add a wire type                         | `internal/types/wire_*.go` (Go source of truth), run `make check-wire-drift`                              |
| Add a feature dependency                | The owning `internal/<feature>/` package; inject a concrete owner or narrow function field at `tools_core.go` |
| Add a CLI command                       | `cli_commands.go` (parser), `cli.go` (if new output format needed)                                        |
| Change MCP protocol handling            | `handler.go` (MCPHandler), `tools_core.go` (ToolHandler)                                                 |
| Add an internal package                 | `internal/<name>/`, import from `cmd/browser-agent/` -- keep it zero-dep                                    |
| Change bridge behavior                  | `bridge.go` (spawn/respawn), `internal/bridge/` (timeout logic)                                           |
| Add a resource (kaboom://*)             | `handler.go` (resource registration in MCPHandler)                                                        |

## Testing

**Go tests** live alongside source files (`*_test.go`). Run with:

```bash
make test              # All Go + extension tests
go test ./cmd/browser-agent/...   # Server tests only
go test ./internal/...          # Internal package tests only
```

**UAT script** validates the npm-installed binary end-to-end:

```bash
./scripts/test-all-tools-comprehensive.sh
```

Tests cold start, all 5 tool calls, concurrent clients, stdout purity, persistence, and graceful shutdown. Never modify UAT tests during a UAT run.

**Internal tool packages** (`internal/tools/*`) have their own `*_test.go` files with unit tests against the `Deps` interface -- no HTTP server needed.

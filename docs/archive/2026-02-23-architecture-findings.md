# Architecture Findings — 2026-02-23

## Scope

Architecture-focused review of Kaboom control-plane reliability and failure modes across:

- Go MCP bridge/daemon server (`cmd/browser-agent`, `internal/*`)
- Chrome extension runtime (`src/background`, `src/content`, `src/inject`)
- Command lifecycle and sync protocol (`/sync`, async queue, correlation tracking)

## What Is Good

- Strong bridge/daemon resilience patterns including respawn and compatibility checks (`cmd/browser-agent/bridge.go:127`, `cmd/browser-agent/bridge.go:220`, `cmd/browser-agent/bridge.go:477`).
- Correct stdout serialization discipline for MCP responses (single write gate) (`cmd/browser-agent/mcp_stdout.go:17`).
- Clear backpressure behavior in command queue (reject on saturation, no silent queue drops) (`internal/queries/dispatcher_queries.go:60`).
- Command-loss reconciliation exists via in-progress heartbeat tracking (`internal/capture/sync.go:309`).
- Contract-first wire typing with generation + drift checks (`Makefile:43`, `Makefile:243`, `internal/types/wire_network.go:1`, `src/types/wire-network.ts:1`).
- Localhost boundary hardening with Host + Origin checks (`cmd/browser-agent/server_middleware.go:107`).
- High automated test density (substantial Go + extension test coverage footprint).

## What Is Weak

- Message integrity/correlation in content-inject response path is inconsistent across command types.
- Command acknowledgement semantics can get ahead of true execution completion.
- Unknown command statuses default to success in normalization, which can mask protocol drift.
- Extension background state architecture has dual patterns (legacy mutable mirrors + separate state machine), increasing drift risk.
- Some timeout/wakeup mechanisms are safe but computationally noisy under concurrent load.

## Biggest Areas Where Things Can Go Wrong

1. Inject/content response integrity and correlation under concurrent commands.
2. Async command lifecycle during extension restarts/disconnect/reconnect windows.
3. Bridge/daemon startup and recovery race edges under load.
4. Extension runtime state drift from duplicated state-management patterns.

## Detailed Findings (Ordered by Severity)

1. High — Uncorrelated response handling can cross-wire concurrent commands.
- `forwardInjectQuery` resolves by response type, not `requestId`, and waterfall has the same pattern.
- This can return the wrong result when multiple same-type requests overlap.
- Evidence: `src/content/message-handlers.ts:336`, `src/content/message-handlers.ts:382`, `src/content/message-handlers.ts:389`, `src/inject/message-handlers.ts:232`, `src/inject/message-handlers.ts:437`.

2. High — Inject-to-content response authenticity is weak in key response handlers.
- Content-side response listeners accept `window` messages without nonce validation on response paths; inject responses generally omit nonce.
- A page script can attempt spoofed responses.
- Evidence: `src/content/window-message-listener.ts:38`, `src/content/window-message-listener.ts:44`, `src/content/message-handlers.ts:127`, `src/content/message-handlers.ts:335`, `src/content/message-handlers.ts:381`, `src/inject/message-handlers.ts:302`, `src/inject/message-handlers.ts:344`.

3. High — Ack semantics can acknowledge work before execution completion.
- Extension sets `lastCommandAck` on receipt; server removes all pending up to that ID.
- Crash/reload timing windows can convert accepted work into timeout/desync paths.
- Evidence: `src/background/sync-client.ts:348`, `src/background/sync-client.ts:359`, `internal/queries/dispatcher_queries.go:211`, `internal/queries/dispatcher_queries.go:242`, `internal/capture/sync.go:369`.

4. Medium — Unknown command statuses normalize to `complete`.
- This can mask protocol drift and create false-positive success states.
- Evidence: `internal/queries/dispatcher_commands.go:25`.

5. Medium — Wait loop wakeup strategy can create avoidable churn under high concurrency.
- `WaitForResultWithClient` uses a per-waiter ticker broadcasting every 10ms.
- Safe but potentially noisy at scale.
- Evidence: `internal/queries/dispatcher_queries.go:420`, `internal/queries/dispatcher_queries.go:425`.

6. Medium — Pending result queue truncates oldest outcomes when disconnected.
- Limits memory growth, but drops command outcomes under prolonged outage bursts.
- Evidence: `src/background/sync-client.ts:185`, `src/background/sync-client.ts:187`.

7. Medium — Extension state-management architecture has drift risk.
- Mutable state with legacy export mirrors coexists with a formal state-machine module that appears unintegrated.
- Evidence: `src/background/state.ts:84`, `src/background/state.ts:97`, `src/background/connection-state.ts:95`, `src/background/connection-state.ts:421`.

8. Low — Extension endpoint trust is header-based unless API key is configured.
- `X-Kaboom-Client` alone is spoofable by local processes in same-host threat models.
- Evidence: `cmd/browser-agent/server_middleware.go:146`, `cmd/browser-agent/server_middleware.go:149`, `cmd/browser-agent/auth.go:21`.

## Suggested Next Hardening Priorities

1. Enforce `requestId` + nonce validation for all inject/content response flows.
2. Redesign ack semantics to represent completion (or split receipt-ack vs completion-ack explicitly).
3. Treat unknown command statuses as errors (or explicit `unknown`) rather than `complete`.
4. Consolidate extension background state ownership to one model and retire mirror exports over time.

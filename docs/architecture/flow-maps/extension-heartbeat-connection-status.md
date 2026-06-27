---
doc_type: flow_map
flow_id: extension-heartbeat-connection-status
status: active
last_reviewed: 2026-04-13
owners:
  - Brenn
feature_ids:
  - feature-backend-log-streaming
  - feature-browser-extension-enhancement
last_verified_version: 0.8.3
last_verified_date: 2026-04-13
entrypoints:
  - src/background/server.ts
  - src/background/index.ts
  - src/popup/status-display.ts
  - scripts/test-all-tools-comprehensive.sh
code_paths:
  - src/background/server.ts
  - src/background/index.ts
  - src/background/message-handlers.ts
  - src/background/sync-manager.ts
  - src/background/state.ts
  - src/popup.ts
  - src/popup/status-display.ts
  - scripts/test-all-tools-comprehensive.sh
test_paths:
  - tests/extension/server.test.js
  - tests/extension/background-batching.test.js
  - tests/extension/popup-status.test.js
---

# Extension Heartbeat Connection Status

## Scope

Define one meaning for extension "Connected": the daemon has observed a live extension heartbeat through `/sync`, not merely a reachable `/health` endpoint.

## Entrypoints

- `src/background/server.ts` parses daemon `/health` responses into extension connection state.
- `src/background/index.ts` stores the parsed connection state and fans it out to popup/badge consumers.
- `src/popup/status-display.ts` renders the user-visible `Connected` or `Offline` status.
- `scripts/test-all-tools-comprehensive.sh` blocks UAT unless daemon-side `capture.extension_connected` is true.

## Primary Flow

1. Background health checks call `checkServerHealth(serverUrl)` in `src/background/server.ts`.
2. The daemon response is only considered connected when `capture.extension_connected === true`.
3. If heartbeat is missing or stale, `checkServerHealth` returns `connected: false` plus a heartbeat-specific error string.
4. `checkConnectionAndUpdate` in `src/background/index.ts` writes that status into background state and broadcasts `status_update`.
5. Popup `get_status` and `status_update` consumers render `Connected` only from that heartbeat-based `connected` flag.
6. UAT preflight independently validates the same daemon-side field (`/health.capture.extension_connected`) before running the full suite.

## Error and Recovery Paths

- Daemon unreachable:
  `checkServerHealth` returns `connected: false` with the transport error or HTTP status.
- Daemon reachable but heartbeat missing:
  `checkServerHealth` returns `connected: false` with an explicit heartbeat recovery hint.
- Daemon reachable but heartbeat status unavailable:
  treat as disconnected and prompt for matching extension/server versions.
- Heartbeat reconnects:
  sync-manager flips the shared connection state back to connected and popup/badge state recovers on the next status update.

## State and Contracts

- `connected` means heartbeat-confirmed connection.
- Daemon contract:
  `/health.capture.extension_connected` is the source of truth for initial connection semantics.
- Popup contract:
  `get_status` and `status_update` must preserve the heartbeat-based meaning of `connected`.
- UAT contract:
  preflight must fail when daemon heartbeat is absent, even if `/health` returns `200`.

## Code Paths

- `src/background/server.ts`
- `src/background/index.ts`
- `src/background/message-handlers.ts`
- `src/background/sync-manager.ts`
- `src/background/state.ts`
- `src/popup.ts`
- `src/popup/status-display.ts`
- `scripts/test-all-tools-comprehensive.sh`

## Test Paths

- `tests/extension/server.test.js`
- `tests/extension/background-batching.test.js`
- `tests/extension/popup-status.test.js`

## Edit Guardrails

- Do not treat HTTP `200 /health` as sufficient for extension connection.
- Keep popup, badge, and UAT preflight aligned on the same heartbeat-based contract.
- If a future UI wants to expose "daemon reachable" separately, add a new field; keep `connected` reserved for heartbeat-confirmed state.

## Feature Links

- Backend Log Streaming: `docs/features/feature/backend-log-streaming/flow-map.md`
- Browser Extension Enhancement: `docs/features/feature/browser-extension-enhancement/flow-map.md`

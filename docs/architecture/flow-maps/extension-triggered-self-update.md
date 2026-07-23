---
doc_type: flow_map
flow_id: extension-triggered-self-update
status: active
last_reviewed: 2026-04-20
owners:
  - Brenn
entrypoints:
  - src/popup/update-button.ts:renderUpdateAvailableBanner
  - cmd/browser-agent/server_routes_upgrade.go:handleUpgradeInstall
code_paths:
  - src/popup/update-button.ts
  - src/popup.ts
  - extension/popup.html
  - cmd/browser-agent/server_routes_upgrade.go
  - cmd/browser-agent/server_routes.go
  - internal/upgrade/nonce.go
  - internal/upgrade/runner.go
  - internal/upgrade/runner_unix.go
  - internal/upgrade/runner_windows.go
  - scripts/install.sh
test_paths:
  - internal/upgrade/nonce_test.go
  - internal/upgrade/runner_test.go
  - cmd/browser-agent/server_routes_upgrade_test.go
  - tests/extension/popup-update-button.test.js
last_verified_version: 0.8.5
last_verified_date: 2026-04-20
---

# Extension-Triggered Self-Update

## Scope

Covers the "Update now" button in the extension popup, the `/upgrade/*` HTTP routes on the daemon, the detached installer spawn, and the post-respawn "reload extension" prompt.

Related docs:

- `docs/features/feature/self-update/index.md`
- `docs/features/feature/self-update/flow-map.md`
- `docs/core/release.md`
- `docs/architecture/flow-maps/release-asset-contract-and-sarif-distribution.md`

## Entrypoints

1. Popup banner render: `src/popup.ts` does a one-shot `GET /health` on popup open; `src/popup/update-button.ts:renderUpdateAvailableBanner` shows the `#update-available` block when `availableVersion` differs from `version`.
2. User click: `cmd/browser-agent/server_routes_upgrade.go:handleUpgradeInstall`.

## Primary Flow

1. Daemon `/health` includes `version` and (when the GitHub version-check surfaced a newer release) `availableVersion`.
2. Popup sees the delta, renders the banner, wires the click handler.
3. Click: popup `GET /upgrade/nonce` → daemon returns the per-process nonce.
4. Popup `POST /upgrade/install` with `{nonce}` → daemon validates nonce, checks the 60 s rate-limit, spawns `bash -c 'curl -sSL <pinned>/scripts/install.sh | bash'` with `Setsid: true` and detached stdio, returns 202.
5. The installer kills the running daemon (`pkill -f kaboom-agentic-browser`), stages the new binary into `$HOME/.kaboom/bin/`, and respawns via `launchctl bootstrap` (macOS), `systemctl --user restart kaboom.service` (systemd Linux), or `nohup <bin> --daemon --port 7890 &` (XDG-autostart Linux).
6. Popup polls `GET /health` every 2 s for up to 30 s. When `version` matches the previously advertised `availableVersion`, the banner swaps to the "Open extensions page" state wired to `chrome://extensions/?id=<runtime.id>`.
7. User clicks the button → Chrome opens the extensions page → user reloads the extension so the updated extension and daemon versions line up.

## Error and Recovery Paths

1. `GET /upgrade/nonce` with a non-extension client: `extensionOnly` middleware returns 403 before the handler runs.
2. `POST /upgrade/install` with no nonce / wrong nonce: handler returns 401 before any rate-limit mutation, so invalid callers cannot rate-limit legitimate users.
3. `POST /upgrade/install` within the 60 s window of a previous accepted call: 429. Popup surfaces "Update was requested recently. Wait a minute and try again."
4. `POST /upgrade/install` on Windows: runner returns `ErrUnsupportedPlatform`; handler returns 501. Popup surfaces "Self-update is not supported on this platform — re-run the installer manually."
5. Daemon spawn failure (bash not on PATH, `url.Parse` failure): handler returns 500; popup surfaces the generic error.
6. Daemon does not respawn within 30 s: popup surfaces "Daemon did not restart in time." User opens extensions page manually or reruns the installer from a terminal.

## State and Contracts

1. `Nonce` is per-process and rotates on every daemon start. There is no persistent nonce store; a daemon restart invalidates outstanding install requests.
2. The pinned install URL is a compile-time constant in `cmd/browser-agent/server_routes_upgrade.go:upgradeInstallURL`. The endpoint accepts no caller-supplied URL.
3. The install script is responsible for respawning the daemon; the daemon does not supervise its own replacement.
4. Wire: `GET /upgrade/nonce → {nonce: string(64 hex)}`; `POST /upgrade/install {nonce: string} → 202 {status: "installing"} | 400 | 401 | 405 | 429 | 500 | 501`. Documented in `cmd/browser-agent/openapi.json`.

## Code Paths

- Argv + platform spawn: `internal/upgrade/runner.go`, `runner_unix.go`, `runner_windows.go`
- Nonce: `internal/upgrade/nonce.go`
- HTTP handlers: `cmd/browser-agent/server_routes_upgrade.go`
- Route wiring: `cmd/browser-agent/server_routes.go`
- Popup UI: `src/popup/update-button.ts`, `src/popup.ts`, `extension/popup.html`
- Installer respawn: `scripts/install.sh`

## Test Paths

- Nonce behavior: `internal/upgrade/nonce_test.go`
- Argv validation + shell-metachar rejection: `internal/upgrade/runner_test.go`
- HTTP handler matrix: `cmd/browser-agent/server_routes_upgrade_test.go`
- Popup button states + fetch sequence: `tests/extension/popup-update-button.test.js`

## Edit Guardrails

1. Do not introduce a caller-supplied URL on `/upgrade/install`. If the installer location must become configurable, restrict it at build time, not runtime.
2. Keep the nonce check ordered before the rate-limit mutation so invalid callers cannot interfere with legitimate attempts.
3. When adding new allowed shell characters to the URL validator, add a matching negative test in `runner_test.go` covering each way the addition could break single-quote embedding.
4. Preserve `Setsid: true` in the unix spawn path — without it, the installer's pkill of the daemon would also kill the script.
5. Keep extension UI, feature docs, and this flow map aligned whenever the popup click flow changes. Update `test_paths` and `code_paths` in the same change.

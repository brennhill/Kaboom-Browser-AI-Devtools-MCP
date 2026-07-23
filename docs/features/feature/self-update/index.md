---
doc_type: feature_index
feature_id: feature-self-update
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-04-20
code_paths:
  - internal/upgrade/nonce.go
  - internal/upgrade/runner.go
  - internal/upgrade/runner_unix.go
  - internal/upgrade/runner_windows.go
  - cmd/browser-agent/server_routes_upgrade.go
  - cmd/browser-agent/server_routes.go
  - scripts/install.sh
  - src/popup/update-button.ts
  - src/popup.ts
  - extension/popup.html
test_paths:
  - internal/upgrade/nonce_test.go
  - internal/upgrade/runner_test.go
  - cmd/browser-agent/server_routes_upgrade_test.go
  - tests/extension/popup-update-button.test.js
last_verified_version: 0.8.5
last_verified_date: 2026-04-20
---

# Self Update

## TL;DR

- Status: shipped
- Tool: N/A — extension UI surface, not an MCP tool
- Mode/Action: popup "Update now" button; fires detached installer; extension prompts user to reload after daemon respawns
- Location: `docs/features/feature/self-update`

## Specs

- Flow Map: [flow-map.md](./flow-map.md)

## Surface

The daemon exposes two extension-only HTTP routes:

- `GET /upgrade/nonce` — returns the per-process nonce (rotates on daemon start).
- `POST /upgrade/install` — rate-limited to one attempt per minute; validates the nonce, spawns the pinned installer in a detached process, returns 202.

The installer (`scripts/install.sh`) kills the running daemon, stages the new binary, and respawns it via launchd (macOS) or systemd (Linux). Windows returns 501; Windows users must re-run the installer manually for now.

## Security Envelope

1. Localhost-only HTTP (existing daemon constraint).
2. `Origin: chrome-extension://<id>` and `X-Kaboom-Client` gating via `corsMiddleware(extensionOnly(...))`.
3. Per-process nonce gates `/upgrade/install` so unauthenticated callers cannot fire the script even if they reach the port.
4. **Nonce is Origin-bound:** the first fetcher's `Origin` is pinned onto the nonce. Install requests must present the same `Origin`, so a second Chrome extension on the same browser cannot replay a nonce it harvested.
5. `KABOOM_EXTENSION_ID` (optional) — when set by the installer into the supervisor env, `extensionOnly` scopes Origin matching to our specific extension ID, closing the "any valid extension origin passes" gap.
6. No caller-supplied URL — the install URL is hardcoded to the STABLE-branch `scripts/install.sh`; the endpoint accepts only the nonce.
7. Shell-metacharacter rejection in the URL validator so the embedded `'<URL>'` in `curl -sSL '<URL>' | bash` cannot be escaped.
8. Strict checksum verification is forced when `KABOOM_SELF_UPDATE=1` regardless of user env, so a compromised or flaky `checksums.txt` cannot slip a bad binary past the installer.
9. Rate-limit: one attempt per minute, mutated only after nonce validation succeeds.

## Code and Tests

- Runner argv + detached spawn: `internal/upgrade/runner.go`, `runner_unix.go`, `runner_windows.go`.
- Nonce: `internal/upgrade/nonce.go`.
- HTTP routes: `cmd/browser-agent/server_routes_upgrade.go` (tests in `server_routes_upgrade_test.go`).
- Installer respawn (Linux systemd restart + nohup fallback): `scripts/install.sh`.
- Extension popup banner + click flow: `src/popup/update-button.ts`, wired in `src/popup.ts`; DOM in `extension/popup.html` (`#update-available`).

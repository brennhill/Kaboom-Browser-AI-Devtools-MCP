---
status: active
scope: issues/blockers
ai-priority: high
tags: [known-issues, v0.7.x]
last-verified: 2026-03-05
canonical: true
---

# Known Issues

## Release / Distribution Incidents (resolved — guards in place)

These shipped to users and are now prevented fail-closed by the hardened release train
(see [release.md](release.md) → "Hardened Release Train").

| Version | Incident | Root cause | Guard |
|---------|----------|------------|-------|
| 0.8.2 | `npx` could not exec anything | npm silently published an empty (312-byte) platform package — the Go binary was never staged into `bin/` | `scripts/verify-platform-binaries.js` (pre-publish, in `make npm-binaries` + `prepublishOnly` + `make preflight`) **and** `verify-published` execs the published binary post-publish |
| 0.8.4 | Windows `install.ps1` 404'd | npm was published but the GitHub Release was never created (out-of-band/manual publish), so the installer downloaded an HTML 404 | `reconcile` job asserts npm **and** the GitHub Release both serve `VERSION`; `publish.yml` (release-less path) deleted |
| 0.8.4 | Installer showed a stale `STRUM` banner | branding drift in `install.ps1` | `verify-published` runs the real installer end-to-end on every release |

**Principle:** a broken release must be *impossible to publish silently* (pre-publish guards)
and *impossible to leave half-done* (post-publish `verify-published` + `reconcile`).

## v0.7.x — Current Release

### Open Issues

| # | Issue | Severity | Details |
|---|-------|----------|---------|
| 1 | Extension timeout on first interact() | Medium | Content script may not be fully loaded when first `interact()` command is sent after navigation. **Workaround:** Retry after 2-3 seconds. |
| 2 | Tracking loss during cross-origin navigation | Medium | Extension can lose tab tracking state during AI-initiated cross-origin navigation via `interact({action: "navigate"})`. **Workaround:** Re-enable tracking via extension popup. |
| 3 | ~~Pilot test zombies~~ | ~~Low~~ | **Resolved.** Hardcoded `version: '5.2.0'` no longer present in `tests/extension/pilot-*.test.js`. |

### Flaky Tests (Pre-existing)

- `TestAsyncQueueReliability/Slow_polling` — times out at 30s intermittently
- `tests/extension/async-timeout.test.js` — 3 tests flaky

### Fixed in v0.7.x (was v5.8.0)

- Early-patch WebSocket capture — pages creating WS connections before inject script loads now captured
- camelCase to snake_case field mapping for network waterfall entries
- Command results routing through /sync endpoint with proper client ID filtering
- Post-navigation tracking state broadcast for favicon updates
- Empty arrays return `[]` instead of `null` in JSON responses
- Bridge timeouts return proper `extension_timeout` error code

### Fixed in v5.7.x

- Extension health check timeout (5s threshold added)
- Hardcoded version in inject.bundled.js (now reads from VERSION file via esbuild define)
- Stale compiled JS vs TS source

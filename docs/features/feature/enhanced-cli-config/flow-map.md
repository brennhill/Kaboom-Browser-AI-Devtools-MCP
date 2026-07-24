---
doc_type: flow_map_pointer
status: active
last_reviewed: 2026-07-24
canonical_flow_map: ../../../architecture/flow-maps/installer-binary-path-and-manual-extension-handoff.md
---

# Enhanced CLI Config Flow Map Pointer

Canonical flow maps:

- [Installer Binary Path and Manual Extension Handoff](../../../architecture/flow-maps/installer-binary-path-and-manual-extension-handoff.md)
- [Uninstall and Cleanup](../../../architecture/flow-maps/uninstall-and-cleanup.md)

Notable coverage:

- Extension staging integrity checks and source-zip fallback for incomplete release extension artifacts.
- Installer extension refresh now stages + validates + promotes atomically, with rollback to previous extension state on promotion failure.
- npm wrapper install/update/uninstall now converge on `kaboom-browser-devtools` and aggressively remove managed `kaboom-*`, `gasoline-*`, and `strum-*` entries.
- npm wrapper config/doctor helpers now share the same legacy-key list so old `kaboom-*`, `gasoline-*`, and `strum-*` entries are removed during writes and surfaced as non-OK during diagnostics.
- Strict checksum mode (`KABOOM_INSTALL_STRICT=1`) enforces fail-closed binary verification.
- Server postinstall validates `/health` against `kaboom-browser-devtools` before reusing an occupied port.
- Installer defaults unpacked extension output to `~/KaboomAgenticDevtoolExtension` (overridable via `KABOOM_EXTENSION_DIR`) so users can select it in Chrome without enabling hidden files.
- CRX fallback packaging in `scripts/build-crx.js` archives the full `extension/` directory to prevent missing MV3 module imports.
- Startup integrity regression checks assert manifest file paths and service worker import graph resolve before release.
- One-liner uninstallers (`scripts/uninstall.sh`, `scripts/uninstall.ps1`) reverse every install artifact: binaries/state, extension dir, autostart registrations, `# kaboom` PATH lines, MCP client entries (canonical + legacy keys, in-place JSON edits with backups), and marker-managed agent skills. Behavioral coverage in `tests/cli/uninstall-script.test.cjs`.
- Post-install connect confirmation: both channels start the daemon (native via `startDaemonSilently`; npm `--install` via `ensureDaemon`/`startDaemon`, reusing a healthy daemon rather than double-binding) and poll `/health` (`capture.extension_connected`) in a live loop (`waitForExtensionConnected` in Go, `watchExtensionConnect` in `lib/cli.js`), printing a success line or a phase-specific hint on timeout. Skippable and auto-skipped for non-interactive installs.
- Browser-aware extension handoff: `lib/browser.js` detects the first installed Chromium-family browser (Chrome/Brave/Edge/Arc/Chromium) and opens its extensions page directly (`chrome://`/`brave://`/`edge://extensions`), alongside revealing the exact unpacked-extension folder.
- `--connect` re-runs the connect loop on demand; `--doctor` now performs live diagnosis (Node floor, daemon reachability, extension connectivity via `/health`) in addition to config checks — see `lib/health.js`, `lib/daemon.js`, `lib/doctor.js`.
- Install-time opt-outs share one grammar (`isEnvFlagSet`/`envFlagEnabled`): `KABOOM_NO_OPEN`/`KABOOM_INSTALL_NO_OPEN` (auto-open), `KABOOM_NO_WAIT`/`KABOOM_INSTALL_NO_WAIT` (connect-wait), `KABOOM_NO_DAEMON` (daemon-start).

---
doc_type: flow_map
flow_id: installer-binary-path-and-manual-extension-handoff
status: active
last_reviewed: 2026-07-24
owners:
  - Brenn
entrypoints:
  - scripts/install.sh
  - scripts/install.ps1
  - scripts/build-crx.js
  - cmd/browser-agent/native_install.go:runNativeInstall
  - cmd/browser-agent/native_install_connect.go:runExtensionConnectWait
  - npm/kaboom-agentic-browser/lib/install.js:executeInstall
  - npm/kaboom-agentic-browser/lib/cli.js:watchExtensionConnect
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py:run_install
code_paths:
  - Makefile
  - scripts/build-crx.js
  - scripts/install.sh
  - scripts/install.ps1
  - server/scripts/install.js
  - cmd/browser-agent/native_install.go
  - cmd/browser-agent/native_install_connect.go
  - npm/kaboom-agentic-browser/lib/config.js
  - npm/kaboom-agentic-browser/lib/install.js
  - npm/kaboom-agentic-browser/lib/cli.js
  - npm/kaboom-agentic-browser/lib/health.js
  - npm/kaboom-agentic-browser/lib/daemon.js
  - npm/kaboom-agentic-browser/lib/browser.js
  - npm/kaboom-agentic-browser/lib/doctor.js
  - npm/kaboom-agentic-browser/lib/uninstall.js
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py
  - docs/mcp-install-guide.md
test_paths:
  - cmd/browser-agent/native_install_test.go
  - cmd/browser-agent/native_install_connect_test.go
  - npm/kaboom-agentic-browser/lib/config.test.js
  - npm/kaboom-agentic-browser/lib/install.test.js
  - npm/kaboom-agentic-browser/lib/health.test.js
  - npm/kaboom-agentic-browser/lib/daemon.test.js
  - npm/kaboom-agentic-browser/lib/browser.test.js
  - npm/kaboom-agentic-browser/lib/doctor.test.js
  - npm/kaboom-agentic-browser/lib/uninstall.test.js
  - pypi/kaboom-agentic-browser/tests/test_install.py
  - tests/extension/release-extension-zip.test.js
  - tests/extension/release-extension-crx-fallback.test.js
  - tests/extension/manifest-startup-integrity.test.js
---

# Installer Binary Path and Manual Extension Handoff

## Scope

Covers installer behavior for shell, PowerShell, npm wrapper, and PyPI wrapper to ensure:

1. MCP configs use a direct binary path when available.
2. Installer output clearly states that extension loading is a manual browser action.
3. Extension staging always includes required MV3 module files for service-worker registration.
4. Installer output uses a consistent, polished step-and-checklist presentation across entrypoints.
5. CRX fallback packaging must include the full `extension/` tree (no allowlist packaging).
6. Extension refresh is atomic (stage + validate + promote) so failed upgrades do not destroy a previously working extension install.
7. Installers support strict checksum enforcement (`KABOOM_INSTALL_STRICT=1`) for fail-closed install workflows.

## Entrypoints

1. One-liner installers: `scripts/install.sh` and `scripts/install.ps1`.
2. Native CLI install flow: `runNativeInstall`.
3. Wrapper install commands: npm `executeInstall` and PyPI `run_install`.

## Primary Flow

1. Installer resolves platform and downloads/stages binary + extension artifacts.
2. Extension release packaging (`make extension-zip` and `scripts/build-crx.js` fallback zip) archives the entire `extension/` directory.
3. Binary installers verify SHA-256 against release `checksums.txt` (or fail immediately in strict mode).
4. Extension is extracted into a staging directory and validated for required module files (`manifest.json`, `background/init.js`, `content/script-injection.js`, `inject/index.js`, `theme-bootstrap.js`).
5. If the release extension zip is incomplete, installer falls back to source-zip extraction and validates again.
6. Only validated staging directories are promoted atomically to `~/KaboomAgenticDevtoolExtension`; prior extension state is restored on promotion failure.
7. Wrapper/native install writes MCP client configs.
8. Config entries prefer resolved binary paths over transient launchers.
9. Installer prints explicit manual extension checklist:
   - open extensions page
   - enable developer mode
   - load unpacked extension folder
   - pin extension
   - click Track This Tab
10. Installer surfaces a branded panel-style summary at completion with the resolved binary path.
11. Installer reveals the exact extension folder (`openExtensionFolder` / `openExtensionDir`) and opens the detected Chromium-family browser straight to its extensions page (`detectExtensionsTarget` → `openExtensionsPage`, browser-internal schemes `chrome://`/`brave://`/`edge://`).
12. Installer ensures a daemon is running so the extension has something to connect to: the native installer starts it via `startDaemonSilently`; npm `--install` starts it via `ensureDaemon`/`startDaemon` (reusing an already-healthy daemon on the port instead of double-binding).
13. Installer polls `/health` (`capture.extension_connected`) and shows a live connect loop (`waitForExtensionConnected` / `watchExtensionConnect`), printing a success line on connect or a phase-specific hint (`connectHintLine` / `connectHint`) on timeout. The wait is skipped — daemon still started — when opted out (`KABOOM_NO_WAIT`/`KABOOM_NO_DAEMON`) or when the output is non-interactive (piped/CI). `--connect` re-runs this loop on demand; `--doctor` reports the same daemon/extension/Node status non-blocking.

## Error and Recovery Paths

1. If platform binary cannot be resolved, wrappers fall back to command name for compatibility.
2. If release extension zip is missing required module files, installer falls back to source zip and revalidates staged files.
3. If extension promotion fails, installer restores the pre-existing extension directory instead of leaving a partial install.
4. If strict checksum mode is enabled and checksums cannot be verified, installers fail closed.
5. npm postinstall validates existing `/health` identity/version when port is already in use and refuses false-positive success for non-Kaboom services.
6. If extension cannot be side-loaded automatically, installer still succeeds but instructs user on manual steps.
7. Missing client config directories are skipped without aborting install.

## State and Contracts

1. npm wrapper installs register MCP server key `kaboom-browser-devtools` and remove managed `kaboom-*`, `gasoline-*`, and `strum-*` entries during install/update/uninstall.
2. npm wrapper config and doctor helpers share the same legacy-key list so diagnostics flag stale aliases that install/update will purge.
3. File-based clients must receive deterministic command entries (`command` + `args`).
4. Release extension artifacts must include the full extension tree so MV3 module imports resolve at runtime.
5. Installer output must never imply that browser extension installation is fully automatic.
6. In strict mode, checksum verification is mandatory for release binary downloads.
7. Existing-daemon reuse on port checks requires service identity and version parity.
8. Extension unpacked path defaults to `~/KaboomAgenticDevtoolExtension` (overridable with `KABOOM_EXTENSION_DIR`).
9. The connect loop reads only existing `/health` fields (`version`, `capture.extension_connected`); it is a consumer and adds no new wire fields.
10. Install-time opt-outs share one accepted-value grammar (`isEnvFlagSet` in JS, `envFlagEnabled` in Go): unset/empty/`0`/`false`/`no` are off. Auto-open uses `KABOOM_NO_OPEN`/`KABOOM_INSTALL_NO_OPEN`; connect-wait uses `KABOOM_NO_WAIT`/`KABOOM_INSTALL_NO_WAIT`; daemon-start uses `KABOOM_NO_DAEMON`.
11. The connect loop's clock, `/health` fetch, and output sink are injectable so the loop is deterministic under test (no real timers or daemon).

## Code Paths

- `Makefile`
- `scripts/build-crx.js`
- `scripts/install.sh`
- `scripts/install.ps1`
- `server/scripts/install.js`
- `cmd/browser-agent/native_install.go`
- `cmd/browser-agent/native_install_connect.go`
- `npm/kaboom-agentic-browser/lib/config.js`
- `npm/kaboom-agentic-browser/lib/install.js`
- `npm/kaboom-agentic-browser/lib/cli.js`
- `npm/kaboom-agentic-browser/lib/health.js`
- `npm/kaboom-agentic-browser/lib/daemon.js`
- `npm/kaboom-agentic-browser/lib/browser.js`
- `npm/kaboom-agentic-browser/lib/doctor.js`
- `npm/kaboom-agentic-browser/lib/uninstall.js`
- `pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py`
- `pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py`
- `docs/mcp-install-guide.md`

## Test Paths

- `cmd/browser-agent/native_install_test.go`
- `cmd/browser-agent/native_install_connect_test.go`
- `npm/kaboom-agentic-browser/lib/config.test.js`
- `npm/kaboom-agentic-browser/lib/install.test.js`
- `npm/kaboom-agentic-browser/lib/health.test.js`
- `npm/kaboom-agentic-browser/lib/daemon.test.js`
- `npm/kaboom-agentic-browser/lib/browser.test.js`
- `npm/kaboom-agentic-browser/lib/doctor.test.js`
- `npm/kaboom-agentic-browser/lib/uninstall.test.js`
- `pypi/kaboom-agentic-browser/tests/test_install.py`
- `tests/extension/release-extension-zip.test.js`
- `tests/extension/release-extension-crx-fallback.test.js`
- `tests/extension/manifest-startup-integrity.test.js`
- `tests/extension/install-script-extension-source.test.js`
- `tests/cli/server-install-hardening.test.cjs`
- `tests/cli/install.test.cjs`
- `tests/cli/uninstall.test.cjs`

## Edit Guardrails

1. Keep wrapper install outputs aligned across npm and PyPI.
2. Do not regress to `npx`-only config entries for managed installs.
3. Do not reintroduce allowlist-based packaging in extension zip or CRX fallback flows.
4. Preserve manual-extension wording in installer output to avoid user confusion.
5. The connect loop must stay a `/health` consumer — never add wire fields here; changes to the health payload belong to the health flow map and its `wire_*` types.
6. Keep the connect loop's clock/fetch/sink injectable; do not call real timers or `http` directly inside the loop body.
7. The blocking connect-wait must remain skippable and auto-skip for non-interactive installs so CI/piped installs never hang.
8. Keep install-time opt-out grammar centralized (`isEnvFlagSet`/`envFlagEnabled`); do not hand-roll new env parsing.

---
doc_type: feature_index
feature_id: feature-enhanced-cli-config
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-04-20
code_paths:
  - Makefile
  - scripts/build-crx.js
  - cmd/browser-agent/native_install.go
  - scripts/install.sh
  - scripts/install.ps1
  - server/scripts/install.js
  - npm/kaboom-agentic-browser/lib/errors.js
  - npm/kaboom-agentic-browser/bin/kaboom-agentic-browser
  - npm/kaboom-agentic-browser/lib/config.js
  - npm/kaboom-agentic-browser/lib/doctor.js
  - npm/kaboom-agentic-browser/lib/install.js
  - npm/kaboom-agentic-browser/lib/uninstall.js
  - npm/kaboom-agentic-browser/lib/cli.js
  - cmd/browser-agent/main_connection_mcp.go
  - internal/capture/sync.go
  - internal/telemetry/install_id.go
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/__init__.py
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/config.py
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/doctor.py
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/uninstall.py
  - npm/kaboom-agentic-browser/skills/skills.json
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/PKG-INFO
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/entry_points.txt
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/requires.txt
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/top_level.txt
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/SOURCES.txt
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py
  - docs/mcp-install-guide.md
test_paths:
  - scripts/install-upgrade-regression.contract.test.mjs
  - cmd/browser-agent/native_install_test.go
  - internal/telemetry/install_id_test.go
  - npm/kaboom-agentic-browser/lib/config.test.js
  - npm/kaboom-agentic-browser/lib/install.test.js
  - npm/kaboom-agentic-browser/lib/uninstall.test.js
  - pypi/kaboom-agentic-browser/tests/test_branding.py
  - pypi/kaboom-agentic-browser/tests/test_config.py
  - pypi/kaboom-agentic-browser/tests/test_install.py
  - pypi/kaboom-agentic-browser/tests/test_uninstall.py
  - pypi/kaboom-agentic-browser/tests/test_skills.py
  - tests/packaging/kaboom-packaging-branding.test.js
  - tests/extension/install-script-extension-source.test.js
  - tests/extension/release-extension-zip.test.js
  - tests/extension/release-extension-crx-fallback.test.js
  - tests/extension/manifest-startup-integrity.test.js
  - tests/cli/server-install-hardening.test.cjs
  - tests/cli/errors.test.cjs
  - tests/cli/install-analytics-contract.test.cjs
  - tests/cli/cli-integration.test.cjs
  - tests/cli/config.test.cjs
  - tests/cli/doctor.test.cjs
  - tests/cli/install.test.cjs
  - tests/cli/uninstall.test.cjs
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Enhanced Cli Config

## TL;DR

- Status: proposed
- Tool: configure
- Mode/Action: cli
- Location: `docs/features/feature/enhanced-cli-config`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_ENHANCED_CLI_CONFIG_001
- FEATURE_ENHANCED_CLI_CONFIG_002
- FEATURE_ENHANCED_CLI_CONFIG_003

## Code and Tests

- npm wrapper installs now register `kaboom-browser-devtools` and remove legacy `kaboom-*`, `gasoline-*`, and `strum-*` MCP entries during install/update/uninstall.
- npm wrapper config helpers now converge on `mergeKaboomConfig(...)`, and doctor treats legacy MCP keys as non-OK until customers reinstall.
- PyPI wrapper config helpers now converge on `merge_kaboom_config(...)`, and packaged `.egg-info` metadata now exposes only Kaboom package names, entry points, and repo URLs.
- Platform npm packages now ship `kaboom-agentic-browser` and `kaboom-hooks` binaries while preserving legacy cleanup for customer machines.
- Server postinstall now validates `kaboom-browser-devtools` on `/health` reuse checks and points manual extension loading at `KABOOM_EXTENSION_DIR` / `~/KaboomAgenticDevtoolExtension`.
- Shell installer now rejects `sudo` and root execution so user-home install state, ownership, and daemon install identity cannot fork into duplicate installs.
- Native `kaboom-agentic-browser --install` now refuses privileged execution, and npm postinstall skips background daemon auto-start when running with elevated privileges.
- Installer-side analytics beacons were removed; activation and install counting now come only from daemon-owned canonical telemetry such as `first_tool_call`.
- Telemetry state for `install_id` and `first_tool_call` is now claimed atomically so concurrent daemon startups cannot double count one fresh install.
- Daemon startup and extension reconnect lifecycle logs stay local and no longer emit raw install-adjacent analytics rows.

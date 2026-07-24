---
doc_type: feature_index
feature_id: feature-enhanced-cli-config
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-24
code_paths:
  - Makefile
  - scripts/build-crx.js
  - cmd/browser-agent/native_install.go
  - cmd/browser-agent/native_install_connect.go
  - npm/kaboom-agentic-browser/lib/extension.js
  - npm/kaboom-agentic-browser/lib/browser.js
  - npm/kaboom-agentic-browser/lib/health.js
  - npm/kaboom-agentic-browser/lib/daemon.js
  - npm/kaboom-agentic-browser/lib/output.js
  - scripts/install.sh
  - scripts/install.ps1
  - scripts/uninstall.sh
  - scripts/uninstall.ps1
  - server/scripts/install.js
  - npm/kaboom-agentic-browser/bin/kaboom-agentic-browser
  - npm/kaboom-agentic-browser/lib/config.js
  - npm/kaboom-agentic-browser/lib/doctor.js
  - npm/kaboom-agentic-browser/lib/install.js
  - npm/kaboom-agentic-browser/lib/uninstall.js
  - npm/kaboom-agentic-browser/lib/cli.js
  - docs/mcp-install-guide.md
test_paths:
  - cmd/browser-agent/native_install_test.go
  - cmd/browser-agent/native_install_open_test.go
  - cmd/browser-agent/native_install_connect_test.go
  - npm/kaboom-agentic-browser/lib/config.test.js
  - npm/kaboom-agentic-browser/lib/extension.test.js
  - npm/kaboom-agentic-browser/lib/browser.test.js
  - npm/kaboom-agentic-browser/lib/health.test.js
  - npm/kaboom-agentic-browser/lib/daemon.test.js
  - npm/kaboom-agentic-browser/lib/doctor.test.js
  - npm/kaboom-agentic-browser/lib/install.test.js
  - npm/kaboom-agentic-browser/lib/uninstall.test.js
  - tests/packaging/kaboom-packaging-branding.test.js
  - tests/extension/install-script-extension-source.test.js
  - tests/extension/release-extension-zip.test.js
  - tests/extension/release-extension-crx-fallback.test.js
  - tests/extension/manifest-startup-integrity.test.js
  - tests/cli/server-install-hardening.test.cjs
  - tests/cli/cli-integration.test.cjs
  - tests/cli/config.test.cjs
  - tests/cli/doctor.test.cjs
  - tests/cli/install.test.cjs
  - tests/cli/uninstall.test.cjs
  - tests/cli/uninstall-script.test.cjs
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

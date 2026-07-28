---
doc_type: feature_index
feature_id: feature-browser-extension-enhancement
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - src/popup.ts
  - src/popup/shell/status-display.ts
  - src/popup/shell/logo-motion.ts
  - src/options.ts
  - src/background/sync/version-check.ts
  - src/lib/daemon-http.ts
  - extension/popup.html
  - extension/popup.css
  - extension/options.html
test_paths:
  - tests/extension/logo-motion.test.js
  - tests/extension/popup-status.test.js
  - tests/extension/version-check-branding.test.js
  - tests/extension/sync-client-commands.test.js
  - tests/extension/sync-client-fixture.js
  - tests/extension/sync-client-resilience.test.js
  - tests/extension/sync-client.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Browser Extension Enhancement

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/browser-extension-enhancement`
- The popup header now uses the restored Kaboom flame icon consistently and does not swap assets on hover.
- Popup connection status is heartbeat-based: `Connected` only appears after the daemon reports a live extension heartbeat.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_BROWSER_EXTENSION_ENHANCEMENT_001
- FEATURE_BROWSER_EXTENSION_ENHANCEMENT_002
- FEATURE_BROWSER_EXTENSION_ENHANCEMENT_003

## Code and Tests

- `src/popup.ts` initializes popup-side UI wiring, including the shared Kaboom flame icon state.
- `src/popup/shell/status-display.ts` renders `Connected` only for heartbeat-confirmed daemon status and shows offline recovery hints otherwise.
- `src/popup/shell/logo-motion.ts` pins popup logo rendering to the shared flame asset without hover-only swaps.
- `src/options.ts` uses shared daemon request/header helpers for health checks and active-codebase config sync.
- `src/background/sync/version-check.ts` keeps the update badge/title and release download target aligned with Kaboom branding and the canonical Kaboom repo slug.
- `src/lib/daemon-http.ts` defines the canonical extension-client header and JSON request init contract.

---
doc_type: feature_index
feature_id: feature-flow-recording
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-25
code_paths:
  - src/background/recording.ts
  - src/background/recording-capture.ts
  - src/background/recording-listeners.ts
  - src/background/keyboard-shortcuts.ts
  - src/background/context-menus.ts
  - src/background/recording-utils.ts
  - src/background/draw-mode-toggle.ts
  - src/offscreen/recording-worker.ts
  - src/popup/action-recording.ts
  - src/popup/recording.ts
  - src/popup/recording-io.ts
  - src/lib/brand.ts
  - src/lib/daemon-http.ts
test_paths:
  - tests/extension/recording.test.js
  - tests/extension/recording-listeners-target-tab.test.js
  - tests/extension/recording-capture-branding.test.js
  - tests/extension/recording-log-branding.test.js
  - tests/extension/recording-shortcut-command.test.js
  - tests/extension/action-recording-reconcile.test.js
  - tests/extension/entry-point-parity.test.js
  - tests/extension/tracked-hover-launcher.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Flow Recording

## TL;DR

- Status: proposed
- Tool: interact, observe, configure
- Mode/Action: recording, playback, test-generation
- Location: `docs/features/feature/flow-recording`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_FLOW_RECORDING_001
- FEATURE_FLOW_RECORDING_002
- FEATURE_FLOW_RECORDING_003

## Code and Tests

- Core recording lifecycle and listener wiring:
  - `src/background/recording.ts`
  - `src/background/recording-capture.ts`
  - `src/background/recording-listeners.ts`
  - `src/background/keyboard-shortcuts.ts`
  - `src/background/context-menus.ts`
  - `src/background/recording-utils.ts`
  - `src/offscreen/recording-worker.ts`
  - `src/popup/recording.ts`
  - `src/popup/recording-io.ts`
  - `src/lib/brand.ts`
- Core tests:
  - `tests/extension/recording.test.js`
  - `tests/extension/recording-listeners-target-tab.test.js`
  - `tests/extension/recording-capture-branding.test.js`
  - `tests/extension/recording-log-branding.test.js`
  - `tests/extension/recording-shortcut-command.test.js`
  - `tests/extension/tracked-hover-launcher.test.js`

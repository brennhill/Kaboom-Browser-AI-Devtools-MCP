---
doc_type: feature_index
feature_id: feature-tab-recording
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/tools_interact_dispatch.go
  - internal/schema/interact/actions.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolobserve/dispatcher.go
  - cmd/browser-agent/internal/toolobserve/deps.go
  - cmd/browser-agent/internal/screenrec/deps.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/internal/screenrec/types.go
  - cmd/browser-agent/internal/screenrec/handlers.go
  - cmd/browser-agent/internal/screenrec/state.go
  - cmd/browser-agent/internal/screenrec/paths.go
  - cmd/browser-agent/internal/screenrec/save.go
  - cmd/browser-agent/internal/screenrec/reveal.go
  - cmd/browser-agent/internal/screenrec/observe.go
  - src/background/event-listeners.ts
  - src/background/init.ts
  - src/background/ui/keyboard-shortcuts.ts
  - src/background/ui/tab-state.ts
  - src/background/ui/context-menus.ts
  - src/background/recording/badge.ts
  - src/background/recording/capture.ts
  - src/background/recording/index.ts
  - src/background/recording/utils.ts
  - src/offscreen/recording-worker.ts
  - src/lib/daemon-http.ts
  - src/lib/storage/changes.ts
  - src/lib/storage/io.ts
  - src/lib/storage/local.ts
  - src/lib/storage/session.ts
  - src/popup/recording/recording.ts
  - extension/manifest.json
  - extension/popup.html
  - extension/popup.css
test_paths:
  - cmd/browser-agent/lint_hardening_test.go
  - cmd/browser-agent/screenrec_wiring_test.go
  - cmd/browser-agent/tools_interact_handler_test.go
  - cmd/browser-agent/internal/screenrec/screenrec_test.go
  - tests/extension/recording-shortcut-command.test.js
  - tests/extension/context-menus-labels.test.js
  - tests/extension/recording-listeners-target-tab.test.js
  - tests/extension/storage-modules.test.js
  - tests/extension/no-compatibility-facades.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tab Recording

## TL;DR

- Status: proposed
- Tool: interact, observe
- Mode/Action: screen_recording_start, screen_recording_stop, saved_videos, toggle_action_sequence_recording
- Location: `docs/features/feature/tab-recording`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_TAB_RECORDING_001
- FEATURE_TAB_RECORDING_002
- FEATURE_TAB_RECORDING_003

## Code and Tests

The implementation and tests for popup/manual recording and shortcut-toggle recording are listed in frontmatter `code_paths` and `test_paths`.
Saved-video discovery is scoped to the canonical recordings directory; it does
not merge or deduplicate entries from historical storage roots.
Extension storage is divided by change lifecycle, shared I/O mechanics, durable
local state, and ephemeral session state. Consumers import those owners
directly; there is no all-purpose storage facade or compatibility barrel.
Screen-recording dependencies receive the query owner callback directly from
composition; no root `getCommandResult` forwarding method is retained.

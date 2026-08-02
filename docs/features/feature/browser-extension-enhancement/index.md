---
doc_type: feature_index
feature_id: feature-browser-extension-enhancement
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-02
code_paths:
  - src/popup/system-doctor.ts
  - src/popup.ts
  - src/popup/feature-toggles.ts
  - src/popup/settings.ts
  - src/popup/shell/status-display.ts
  - src/popup/shell/ui-utils.ts
  - src/popup/shell/logo-motion.ts
  - src/popup/tabs/tab-tracking.ts
  - src/options.ts
  - src/background/sync/version-check.ts
  - src/background/sync/server.ts
  - src/background/sync/sync-manager.ts
  - src/background/orchestration/connection-monitor.ts
  - src/types/runtime/state.ts
  - src/background/message-handlers.ts
  - src/background/message-routing/
  - src/background/runtime-state/
  - src/background/runtime-state/state-recovery.ts
  - src/background/event-listeners.ts
  - src/background/commands/helpers.ts
  - src/background/exec/browser-actions.ts
  - src/background/message-routing/capture-handler.ts
  - src/background/message-routing/pilot-handler.ts
  - src/background/recording/listeners.ts
  - src/background/state-snapshots.ts
  - src/background/ui/content-script-bridge.ts
  - src/background/ui/settings-storage.ts
  - src/background/ui/tracked-tab-state.ts
  - src/background/ui/terminal-workspace.ts
  - src/background/ui/context-menus.ts
  - src/background/ui/side-panel-availability.ts
  - src/background/ui/terminal-panel.ts
  - src/content/draw-mode/persistence-submission.js
  - src/content/script-injection.ts
  - src/content/runtime-message-listener.ts
  - src/content/ui/terminal-panel-bridge.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/lib/daemon-http.ts
  - src/lib/storage/io.ts
  - src/lib/storage/recovery.ts
  - src/lib/storage/validated.ts
  - src/lib/tabs/tracked-tab-storage.ts
  - src/popup/recording/recording.ts
  - src/offscreen/recording-worker.ts
  - cmd/browser-agent/internal/health/doctor_live_checks.go
  - cmd/browser-agent/internal/daemonlife/
  - cmd/browser-agent/internal/screenrec/
  - cmd/browser-agent/internal/sequencehandler/
  - cmd/browser-agent/internal/summarypref/
  - cmd/browser-agent/internal/toolinteract/interactstate/
  - internal/noise/
  - internal/persistence/
  - internal/recording/
  - internal/statediag/collector.go
  - internal/telemetry/install_id.go
  - extension/popup.html
  - extension/popup.css
  - extension/options.html
test_paths:
  - tests/architecture/async-failure-evidence.test.cjs
  - tests/architecture/user-state-loaders.test.cjs
  - internal/recording/manager_test.go
  - scripts/contracts/check-architecture-boundaries.test.cjs
  - tests/extension/system-doctor/system-doctor-ui.test.js
  - cmd/browser-agent/internal/health/health_coverage_test.go
  - cmd/browser-agent/internal/health/health_test.go
  - internal/statediag/collector_test.go
  - tests/extension/popup-shell/popup-features.test.js
  - tests/extension/popup-shell/popup-toggles.test.js
  - tests/extension/ui-controls/toggle-feature.test.js
  - tests/extension/branding/logo-motion.test.js
  - tests/extension/popup-shell/popup-status.test.js
  - tests/extension/popup-shell/popup-tab-tracking-sync.test.js
  - tests/extension/pilot/pilot-toggle.test.js
  - tests/extension/branding/version-check-branding.test.js
  - tests/extension/sync/sync-client-commands.test.js
  - tests/extension/sync/sync-client-fixture.js
  - tests/extension/sync/sync-client-resilience.test.js
  - tests/extension/sync/sync-client.test.js
  - tests/extension/sync/sync-manager.test.js
  - tests/extension/sync/background-batching.test.js
  - tests/extension/reliability/server.test.js
  - tests/extension/contracts/background-boundaries.test.js
  - tests/extension/content/message-handlers.test.js
  - tests/extension/content/message-handlers-edge.test.js
  - tests/extension/state-recovery/state-recovery-contract.test.js
  - tests/extension/state-recovery/validated-storage.test.js
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
- Popup server status is based on daemon HTTP reachability. Extension heartbeat
  health is tracked independently, so a transient heartbeat gap cannot present
  a live daemon as offline; Doctor retains the actionable heartbeat diagnosis.
- The popup System Doctor renders the daemon's canonical readiness checks,
  treating the absence of a tracked page as a healthy idle state while reserving
  its attention treatment for actionable faults. It sits after the routine
  controls in a compact footer treatment with a health-plus icon, keeping
  diagnostics available without competing with primary workflows.
  including redacted local Claude/Codex authentication classification,
  subscription-versus-API provider status, keychain failures, version state,
  extension connectivity, and tracked-tab readiness.
- Corrupt or unreadable extension-local state now falls back deterministically
  and emits a redacted `state_recovery` diagnostic without exposing the
  persisted value. Diagnostics have an explicit active/recovered lifecycle,
  retain bounded transition history and occurrence counts, and clear their
  warning state only after the owning loader verifies fresh valid state.
- Async failures are never discarded with empty promise catches. Unexpected
  failures leave redacted evidence; intentionally unlogged absence/cancellation
  carries an adjacent `EXPECTED_ABSENCE:` rationale explaining both why the
  condition is normal and why logging would be misleading. Architecture tests
  enforce the same contract for comment-only promise catches and empty
  synchronous catches.

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
- Popup feature toggles and WebSocket mode consume the orchestrator's single
  batched storage read through `applyFeatureToggles` and `applyWebSocketMode`;
  self-loading compatibility initializers are not exported.
- `src/popup/shell/status-display.ts` renders daemon reachability without
  conflating it with extension heartbeat or tracked-tab readiness.
- `src/popup/shell/logo-motion.ts` pins popup logo rendering to the shared flame asset without hover-only swaps.
- `src/options.ts` uses shared daemon request/header helpers for health checks and active-codebase config sync.
- `src/background/sync/version-check.ts` keeps the update badge/title and release download target aligned with Kaboom branding and the canonical Kaboom repo slug.
- `src/lib/daemon-http.ts` defines the canonical extension-client header and JSON request init contract.
- `src/background/message-handlers.ts` validates sender trust and delegates to typed,
  feature-owned handlers under `message-routing/`; each handler receives only its
  change-coupled dependencies.
- `src/background/runtime-state/` separates connection, settings, pilot, startup,
  and diagnostic queue lifecycles. Queue consumers receive defensive snapshots.

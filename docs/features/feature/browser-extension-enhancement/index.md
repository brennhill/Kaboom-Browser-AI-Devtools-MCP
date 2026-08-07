---
doc_type: feature_index
feature_id: feature-browser-extension-enhancement
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - src/lib/diagnostics/page-capture.ts
  - src/types/runtime/telemetry-messages.ts
  - src/content/message-forwarding.ts
  - src/background/message-routing/telemetry-handler.ts
  - cmd/browser-agent/tools_configure.go
  - scripts/contracts/check-silent-catches.cjs
  - src/background/
  - src/content/
  - src/lib/
  - src/sidepanel.ts
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
  - src/background/sync/sync-client.ts
  - src/background/ui/ui-usage-tracker.ts
  - src/types/wire/wire-sync.ts
  - src/background/orchestration/connection-monitor.ts
  - src/types/runtime/state.ts
  - src/background/message-handlers.ts
  - src/background/message-routing/
  - src/background/runtime-state/
  - src/background/runtime-state/state-recovery.ts
  - src/background/event-listeners.ts
  - src/background/commands/helpers.ts
  - src/background/commands/registry.ts
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
  - src/lib/storage/fault.ts
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
  - internal/statediag/store.go
  - internal/telemetry/install_id.go
  - extension/popup.html
  - extension/popup.css
  - extension/options.html
test_paths:
  - tests/extension/network-http/network-waterfall.test.js
  - tests/extension/content/message-handlers.test.js
  - cmd/browser-agent/noise_doctor_test.go
  - scripts/contracts/check-silent-catches.test.cjs
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
  - tests/extension/popup-shell/options.test.js
  - tests/extension/pilot/pilot-toggle.test.js
  - tests/extension/pilot/command-lifecycle.test.js
  - tests/extension/branding/version-check-branding.test.js
  - tests/extension/sync/sync-client-commands.test.js
  - tests/extension/sync/sync-client-fixture.js
  - tests/extension/sync/sync-client-resilience.test.js
  - tests/extension/sync/sync-client.test.js
  - tests/extension/sync/sync-manager.test.js
  - tests/extension/ui-controls/ui-usage-tracker.test.js
  - tests/extension/sync/background-batching.test.js
  - tests/extension/reliability/server.test.js
  - tests/extension/reliability/diagnostic-log-queue.test.js
  - tests/extension/contracts/background-boundaries.test.js
  - tests/extension/content/message-handlers.test.js
  - tests/extension/content/message-handlers-edge.test.js
  - tests/extension/state-recovery/state-recovery-contract.test.js
  - tests/extension/state-recovery/storage-fault-contract.test.js
  - tests/extension/state-recovery/storage-fault-fixture.js
  - tests/extension/state-recovery/storage-owner-faults.test.js
  - tests/extension/state-recovery/validated-storage.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Browser Extension Enhancement

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
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
  warning state only after the owning loader verifies fresh valid state. Active
  incidents are never evicted; Doctor retains at most 100 recovered incidents,
  deterministically evicts the oldest in one pass, and exposes its dropped count
  through a content-free retention check.
- Extension diagnostics persist as a bounded, redacted session ring and flush
  through the existing local sync transport after daemon recovery. System
  Doctor summarizes the latest worker/reconnect sequence and reports dropped
  entries without exposing private diagnostic values or sending them to usage
  telemetry.
- System Doctor incidents form a bounded correlated timeline with failure and
  recurrence transitions, expected next state, deadline, recovery attempt and
  outcome, and the last successful transition. The daemon persists the redacted
  timeline under its canonical state root, restores it after restart, retains
  active obligations, and evicts only the oldest recovered history. The popup
  renders a compact recent sequence; recovered incidents remain historical and
  do not degrade current readiness.
- Authored extension failures are never discarded through empty or bare-fallback
  synchronous and Promise catches. Unexpected
  failures leave redacted evidence; intentionally unlogged absence/cancellation
  carries an adjacent `EXPECTED_ABSENCE:` rationale explaining both why the
  condition is normal and why logging would be misleading. The structural gate
  scans source modules and canonical DOM generator templates (rather than their
  generated output), including nested catches and concise Promise fallbacks.
- Page-world capture distinguishes unavailable browser APIs from unexpected
  implementation failures. Network-waterfall failures emit a bounded redacted
  diagnostic over the authenticated page channel, content validates it,
  background persists it in the local diagnostic queue, and Doctor receives it
  through `/sync`; private exception messages never cross that boundary.
- Remote command cancellation is part of the command-handler contract. The sync
  timeout aborts the same signal passed through the manager and registry to the
  handler; handlers checkpoint that signal before post-await mutation. Once a
  command is cancelled, the registry rejects its completion and suppresses any
  late terminal result, preserving the original correlation and connection
  generation for the daemon's timeout evidence.
- UI-originated feature telemetry keys are generated from the canonical Go sync
  schema and shared by runtime messages, the tracker, and the daemon. Failed-sync
  restoration rejects unknown keys without recording their values and leaves a
  bounded local Doctor diagnostic until a clean restoration proves recovery.
- Options persist locally even when the daemon is absent. A reachable daemon
  that rejects an active-codebase update is a distinct failure: the redacted
  HTTP status is reported to Doctor, while the private filesystem path is not.

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
- Command dispatch validates the daemon-owned connection generation both before
  invoking a handler and when it emits its terminal result. A daemon handoff
  therefore converts late in-flight completion into a retryable stale-generation
  error instead of allowing obsolete work to report success.

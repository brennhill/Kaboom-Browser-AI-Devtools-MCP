---
doc_type: feature_index
feature_id: feature-tab-tracking-ux
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - src/lib/brand.ts
  - src/lib/constants.ts
  - src/lib/tabs/request-audit.ts
  - src/lib/tabs/tab-tracking-core.ts
  - src/lib/tabs/tracked-tab-storage.ts
  - src/lib/storage/recovery.ts
  - src/lib/storage/fault.ts
  - src/lib/storage/validated.ts
  - src/lib/tabs/internal-url.ts
  - src/types/runtime-messages.ts
  - src/types/runtime/tracking.ts
  - src/content.ts
  - src/content/tab-tracking.ts
  - src/content/script-injection.ts
  - src/content/ui/terminal-panel-bridge.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/popup.ts
  - src/popup/shell/logo-motion.ts
  - src/popup/tabs/tab-tracking.ts
  - src/popup/tabs/tab-tracking-api.ts
  - extension/popup.html
  - extension/popup.css
  - src/background/message-handlers.ts
  - src/background/message-routing/pilot-handler.ts
  - src/background/runtime-state/pilot-state.ts
  - src/background/runtime-state/connection-generation.ts
  - src/background/runtime-state/tracking-continuity.ts
  - src/background/runtime-state/content-readiness.ts
  - src/background/runtime-state/state-recovery.ts
  - src/background/commands/registry.ts
  - src/background/exec/browser-actions.ts
  - src/content/runtime-message-listener.ts
  - src/background/event-listeners.ts
  - src/background/init.ts
  - src/background/ui/content-script-bridge.ts
  - src/background/ui/settings-storage.ts
  - src/background/ui/tracked-tab-state.ts
  - src/background/ui/terminal-workspace.ts
  - src/background/ui/keyboard-shortcuts.ts
  - src/background/ui/context-menus.ts
  - src/background/recording/listeners.ts
test_paths:
  - tests/extension/contracts/background-boundaries.test.js
  - tests/extension/tab-state/tab-state.test.js
  - tests/extension/branding/brand-metadata.test.js
  - tests/extension/popup-shell/popup-audit-button.test.js
  - tests/extension/popup-shell/popup-tab-tracking-branding.test.js
  - tests/extension/popup-shell/popup-tab-tracking-sync.test.js
  - tests/extension/popup-shell/popup-untrack-storage.test.js
  - tests/extension/reliability/request-audit.test.js
  - tests/extension/recording-ui/recording-listeners-target-tab.test.js
  - tests/extension/ui-controls/tracked-hover-launcher.test.js
  - tests/extension/branding/logo-motion.test.js
  - tests/extension/content/content.test.js
  - tests/extension/content/content-tab-filtering.test.js
  - tests/extension/content/content-tab-tracking.test.js
  - tests/extension/branding/runtime-log-branding.test.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal-fixture.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal-io.test.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal-ui.test.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal.test.js
  - tests/extension/tab-state/tab-tracking-core.test.js
  - tests/extension/tab-state/tracking-continuity.test.js
  - tests/extension/tab-state/content-readiness.test.js
  - tests/extension/ui-controls/toggle-overlay.test.js
  - tests/extension/pilot/command-lifecycle.test.js
  - tests/architecture/async-failure-evidence.test.cjs
  - tests/extension/injection/script-injection-ready.test.js
  - tests/extension/shared/background-message-router.js
  - extension/background/event-listeners.test.js
  - tests/extension/contracts/entry-point-parity.test.js
  - tests/extension/misc/integration.test.cjs
  - tests/extension/contracts/no-compatibility-facades.test.js
  - tests/extension/state-recovery/state-recovery-contract.test.js
  - tests/extension/state-recovery/validated-storage.test.js
  - tests/extension/state-recovery/storage-fault-fixture.js
  - tests/extension/state-recovery/storage-owner-faults.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-04-03
---

# Tab Tracking Ux

## TL;DR

- Status: shipped
- Tool: null
- Mode/Action: null
- Location: `docs/features/feature/tab-tracking-ux`
- When a site is tracked, the popup now exposes an `Audit` CTA that shares the same trigger path as the tracked hover launcher.
- The hover launcher is shown on tracked workspace tabs and hides only while the Kaboom side panel is open.
- Terminal workspace ownership now targets one Chrome tab group, even though broader tracking flows still use `TRACKED_TAB_ID` during the rollout.
- The hover launcher now includes an `Audit` action that opens the side panel and then triggers the shared audit bridge.
- Cloaked-domain disable messaging and popup-driven recording guidance now use Kaboom copy consistently.
- The hover launcher settings gear now points at `gokaboom.dev/docs` and the Kaboom repo, and tracked-tab-loss guidance tells users to reopen the Kaboom popup.
- Invalid or unreadable tracked-tab state is treated as an untracked workspace,
  with a redacted recovery entry available in System Doctor.
- Post-navigation readiness probes and command dispatch retain the daemon
  connection generation that originated them. A reconnect supersedes delayed
  acknowledgements and commands before they can mutate the current page, with
  correlated recovery evidence retained for System Doctor.
- Draw-mode recovery warnings from the hover launcher now use Kaboom copy when the extension was reloaded or the draw bundle is unavailable.
- Popup tab-tracking logs now use the shared Kaboom runtime prefix instead of hardcoded Kaboom labels.
- The popup validates the stored tab ID before presenting it as healthy. A closed
  tab now shows its last title and URL with a one-click **Track Current Tab**
  recovery action.
- Healthy tracking identity consistently presents both the tracked page title
  and URL. Reads and writes use the canonical tracked-tab storage module.
- Tracking continuity is now an explicit state machine. Navigation start,
  provisional URL, content reinjection, extension reconnect, confirmation, and
  failure retain one stable tab ID until that tab is explicitly closed or
  untracked. The popup renders transitional progress instead of briefly
  reporting that capture is disabled.
- Document-changing browser actions create a correlation-scoped readiness
  transition. The first subsequent content command uses bounded deterministic
  backoff and proceeds only after the newly injected content script echoes that
  exact correlation ID. Stale acknowledgements cannot release newer
  navigations; final failures remain visible in System Doctor without exposing
  page data.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_TAB_TRACKING_UX_001
- FEATURE_TAB_TRACKING_UX_002
- FEATURE_TAB_TRACKING_UX_003

## Code and Tests

Concrete implementation and test paths are listed in frontmatter `code_paths` and `test_paths`.

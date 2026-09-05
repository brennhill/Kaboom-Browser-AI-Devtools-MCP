---
doc_type: feature_index
feature_id: feature-tab-tracking-ux
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - docs/architecture/diagrams/ui/flame-flicker-visual.md
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
  - src/content/favicon-replacer.ts
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
  - src/background/commands/helpers.ts
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
  - src/background/dom/cdp/cdp-session.ts
  - src/lib/tabs/tab-focus.ts
  - src/background/message-routing/capture-handler.ts
  - src/background/push-handler.ts
  - src/background/recording/capture.ts
  - src/background/commands/observe.ts
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
  - tests/extension/branding/favicon-replacer.test.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal-fixture.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal-io.test.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal-ui.test.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal.test.js
  - tests/extension/tab-state/tab-tracking-core.test.js
  - tests/extension/tab-state/internal-url.test.js
  - extension/background/registry_csp_navigation.test.js
  - tests/extension/tab-state/tracking-continuity.test.js
  - tests/extension/tab-state/content-readiness.test.js
  - tests/extension/ui-controls/toggle-overlay.test.js
  - tests/extension/pilot/command-lifecycle.test.js
  - tests/extension/sync/pending-query-targeting.test.js
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
  - tests/extension/capture/background/background-tab-capture.test.js
  - tests/extension/capture/background/visible-tab-capture-fallback.test.js
  - tests/extension/capture/observe-screenshot.test.js
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
- Favicon tracking indicators consume runtime-validated canonical tracking
  messages. Malformed extension messages and incomplete initial responses are
  ignored rather than mutating page UI from untrusted shapes.
- The hover launcher is shown on tracked workspace tabs and hides only while the Kaboom side panel is open.
- Terminal workspace ownership now targets one Chrome tab group, even though broader tracking flows still use `TRACKED_TAB_ID` during the rollout.
- The hover launcher now includes an `Audit` action that opens the side panel and then triggers the shared audit bridge.
- Cloaked-domain disable messaging and popup-driven recording guidance now use Kaboom copy consistently.
- The hover launcher settings gear now points at `gokaboom.dev/docs` and the Kaboom repo, and tracked-tab-loss guidance tells users to reopen the Kaboom popup.
- Invalid or unreadable tracked-tab state is treated as an untracked workspace,
  with a redacted recovery entry available in System Doctor.
- A stored tab ID is revalidated against its live URL before every targeted
  command. If that tab navigated to a browser-internal or another extension's
  page, Kaboom clears the stale target and recovers to a trackable web tab.
  The same validation covers daemon-resolved `query.tab_id` context; a tab ID
  explicitly requested by the user fails closed instead of being retargeted.
- Browser escape actions (`navigate`, `refresh`, `back`, `forward`, `new_tab`,
  `switch_tab`, `close_tab`) are the exception: they keep the restricted tab as
  their target, because they are how the user gets off it. Recovering to a
  different tab would navigate that tab while leaving the stuck one in place.
  The registry's restricted-page gate exempts the same action set.
- Trackability is decided from both URLs Chrome reports for a tab. While a
  navigation is in flight, `url` still names the outgoing document and only
  `pendingUrl` names the destination, so a tab racing toward a browser-internal
  or another extension's page used to read as scriptable and strand commands
  (a performance trace attached mid-navigation failed with "Cannot access a
  chrome-extension:// URL of different extension"). The canonical
  `isInternalTab` predicate fails closed on either URL, and target resolution
  carries `pendingUrl` through to the restricted-page gate.
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

## Foreground is an explicit request

Capture no longer activates the tab it photographs. `captureTabImage` in
`src/background/ui/tracked-tab-state.ts` takes a CDP lease and calls `Page.captureScreenshot`;
`chrome.tabs.captureVisibleTab` — the API that forced the activate/capture/restore dance — is
reached only when `chrome.debugger` is unavailable or the CDP capture fails.

The remaining foreground grabs are deliberate and each says why at the call site: `activate_tab`
and the popup URL click (`src/lib/tabs/tab-focus.ts`), a screen recording waiting on a user
gesture (`src/background/recording/capture.ts`), draw mode's backdrop capture, and the
`push_screenshot` keyboard shortcut — the last two act on the tab already in front of the user.

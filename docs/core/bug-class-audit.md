---
status: active
scope: process/quality
ai-priority: high
tags: [bugs, guardrails, audit, prevention]
last-verified: 2026-07-25
backlog-status: F2-F10 fixed; both class-killing guardrails built
---

# Bug-Class Audit & Guardrails

A batch of user-reported bugs (missing flame launcher, terminal not starting after
a folder pick, silent terminal failures, daemon "connection dies") all reduced to
**five recurring shapes**. This doc records the classes, the guardrails added to
prevent them, and the remaining instances found by a codebase-wide audit.

## The five bug classes

1. **Uncentralized reset/rebuild invariant** — a function with a `forceFresh`/reset
   flag leaves a required teardown to callers, so some forget it. *(CLAUDE.md rule 19.)*
2. **Stale mirror read as source of truth** — a decision gates on a `chrome.storage`
   mirror that goes stale when its writer's context dies without flushing, instead
   of the authoritative live signal. *(CLAUDE.md rule 18.)*
3. **Silent failure on a mutating path** — an error is swallowed or a real failure
   is masked as an expected/recoverable state. *(CLAUDE.md rule 25.)*
4. **Multi-entry-point action with divergent implementations** — the same action
   from keyboard/context-menu/popup/MCP doesn't route through one shared helper.
   *(CLAUDE.md rule 19.)*
5. **Lifecycle event with no reliable teardown hook** — state set on open/start is
   cleared only on a best-effort unload/close that can be skipped.

## Guardrails added

- **CLAUDE.md rules 18/19/25** extended with the authoritative-source, invariant-in-
  the-shared-function, and fail-loud conventions.
- **ESLint** `no-empty: { allowEmptyCatch: false }` made explicit (bans bare `catch {}`).
  Note: ESLint is JS-only in this repo (no typescript-eslint), so it does not reach
  `src/**/*.ts` — the two AST guardrails below cover the TypeScript source instead.
- **Fail-loud contract test (Class 3):** `tests/extension/fail-loud-contract.test.js`
  AST-scans `src` (via the `typescript` dev dep) for any `start|save|write|persist|
  upload|stop|spawn|connect` function that masks a failure as `catch { return
  false|null }`. Attribution walks up to the nearest *named* enclosing function, so
  a swallow nested in a `.then()`/chrome-callback arrow (the dominant MV3 shape)
  still counts. Self-verifying (synthetic violations, incl. the nested-arrow case,
  must trip the detector). **Known limitation:** an object return that masks failure
  as success (the 409-with-token shape) is not machine-detectable without semantic
  analysis, so it is out of scope — code review remains the backstop for that form.
- **Entry-point parity test (Class 4):** `tests/extension/entry-point-parity.test.js`
  asserts each multi-entry action (tab tracking, action-recording, terminal toggle,
  annotation tracking) routes through its one shared helper. Uses TS-AST
  call-verification (`functionContainsCall`/`fileContainsCall`), not substring
  checks, so it is not satisfied by a mere import or same-file definition — it fails
  if the entry point stops *calling* the helper. Catches F2, F6, F7, F10. Backed by
  `npx jscpd src/background src/popup` (rule 22) — clone density 3.49% after the dedup.
- **Terminal lifecycle UAT checklist** — `docs/features/feature/terminal/qa-plan.md`
  (open → folder → start → Chrome-X close → flame returns), backed by regression
  tests: `sidepanel-terminal` (forceFresh + `applyRootFolder` re-attach),
  `terminal-panel-presence` (port connect/disconnect mirrors `TERMINAL_UI_STATE`).
- **`--doctor` daemon restart-churn surface** + `crash.log` → `exit-diagnostics.log`
  rename (the file records normal exits, not just crashes).

## Audit findings (other live instances)

Reference bugs already fixed: Class 1 `bootTerminalPanel`, Class 2 flame gating,
Class 3 `HandleTerminalStart` 409, Class 5 sidepanel port teardown. New instances
(F2–F10 fixed in the bug-class backlog change; see test paths per row):

| ID | Sev | Class | Where | Issue | Status |
|----|-----|-------|-------|-------|--------|
| F1 | HIGH | 3+5 | `internal/terminal/relay.go` `WriteToFirst` | Discarded `writeBuf.Write` error → reports success on a dead shell; in-page **Audit prompt silently lost**. | **Fixed** (propagate the write error). Follow-up: prune dead relays from the `Map` on `readLoop` exit. |
| F2 | MED | 4 | `background/context-menus.ts` vs `popup/tab-tracking-api.ts` | Context-menu "Control Tab" skipped the cloaked/internal-page guard (**privacy leak**, rule 7), content-script inject, and stop-recording-on-release. | **Fixed** — shared `src/lib/tabs/tab-tracking-core.ts` `trackTab`/`untrackTab`; both entry points route through it. Tests: `tab-tracking-core`, `entry-point-parity`. |
| F3 | MED | 3 | `src/lib/storage-utils.ts` `writeStorage` | Never checked `chrome.runtime.lastError` → reported success on a failed/over-quota write. | **Fixed** — write/remove/setAccessLevel reject on `lastError`; 19 fire-and-forget callers moved to a logged `persist()` helper. Tests: `storage-utils` (fail-loud writes). |
| F4 | MED | 1 | `src/sidepanel.ts` (exit/close/minimize) | Three close paths re-listed the same teardown; drift could leak `resetWriteGuardState`. | **Fixed** — one `closePanelWithIntent(intent)`; the teardown invariant lives in it. Tests: `sidepanel-terminal` (disconnect/close/minimize). |
| F5 | MED | 2+5 | `src/popup/action-recording.ts` | Restored "recording" from the storage mirror with no daemon revalidation → phantom recording after a daemon restart. | **Fixed** — capture the daemon **PID** at record-start, reconcile on popup open by comparing it to the daemon's current PID (restart-stable, no clock math; no new MCP surface). **Destructive-safe / fail-open:** only deletes the mirror when CONFIDENT the daemon restarted; any uncertainty (unreachable, non-2xx, no baseline) keeps it. A first draft used uptime-vs-wall-clock and was caught in review deleting live recordings across laptop sleep / NTP jumps. Tests: `action-recording-reconcile`. |
| F6 | MED | 4 | `keyboard-shortcuts.ts` vs `context-menus.ts` | Action-sequence recording copy-inlined the toggle (context menu was silent — no toast, no tracking). | **Fixed** — shared `toggleActionSequenceRecording`; added `action_recording` UI-feature key (wire-synced Go+TS). Tests: `recording-shortcut-command`, `entry-point-parity`, Go `sync_analytics`. |
| F7 | MED | 4 | popup/launcher/MCP draw-mode | `trackUIFeature('annotations')` fired only on keyboard+context-menu → analytics undercounted. | **Fixed** — new `track_ui_feature` runtime message; popup + in-page launcher report in; MCP still excluded. Tests: `message-handlers`, `entry-point-parity`. |
| F8 | LOW | 3 | `internal/pty/upload.go` | Deferred `f.Close()` error dropped → truncated upload reported as success. | **Fixed** — explicit close checked; partial removed on failure. Tests: `upload_test` (write-error removes partial). |
| F9 | LOW | 3 | `background/commands/interact.ts` (subtitle) | `sendMessage(...).catch(()=>{})` then unconditional `success:true`. | **Fixed** — await the send; report the real result. Tests: `interact-subtitle`. |
| F10 | LOW | 4 | terminal keyboard vs context menu | Keyboard was open-only while `toggleTerminalSidePanel`'s doc claimed it unified both. | **Fixed** — keyboard shortcut now toggles via the shared helper. Tests: `terminal-panel-gesture-entrypoints`, `entry-point-parity`. |

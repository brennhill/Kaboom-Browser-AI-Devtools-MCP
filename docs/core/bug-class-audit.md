---
status: active
scope: process/quality
ai-priority: high
tags: [bugs, guardrails, audit, prevention]
last-verified: 2026-07-25
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
- **Terminal lifecycle UAT checklist** — `docs/features/feature/terminal/qa-plan.md`
  (open → folder → start → Chrome-X close → flame returns), backed by regression
  tests: `sidepanel-terminal` (forceFresh + `applyRootFolder` re-attach),
  `terminal-panel-presence` (port connect/disconnect mirrors `TERMINAL_UI_STATE`).
- **`--doctor` daemon restart-churn surface** + `crash.log` → `exit-diagnostics.log`
  rename (the file records normal exits, not just crashes).

### Recommended, not yet built (each kills a class)
- **Entry-point registry test (Class 4):** assert every keyboard/context-menu/popup/MCP
  terminal/recording/draw/tracking action calls the same shared `toggle*`/`track*`
  helper, plus a `jscpd` clone check over `context-menus.ts` + `keyboard-shortcuts.ts`
  + `src/popup/*` handlers. Would catch F2, F6, F7.
- **Fail-loud mutation lint (Class 3):** `no-restricted-syntax` flagging
  `catch { return false|null }` and result-discarded `sendMessage`/storage writes in
  `start|save|write|persist|upload|stop|spawn|connect` functions; audit Go `errcheck`
  deferred-`Close` blind spot. Would catch F8, F9, and the F3/F1 shapes.

## Audit findings (other live instances)

Reference bugs already fixed: Class 1 `bootTerminalPanel`, Class 2 flame gating,
Class 3 `HandleTerminalStart` 409, Class 5 sidepanel port teardown. New instances:

| ID | Sev | Class | Where | Issue | Status |
|----|-----|-------|-------|-------|--------|
| F1 | HIGH | 3+5 | `internal/terminal/relay.go` `WriteToFirst` | Discarded `writeBuf.Write` error → reports success on a dead shell; in-page **Audit prompt silently lost**. | **Fixed** (propagate the write error). Follow-up: prune dead relays from the `Map` on `readLoop` exit. |
| F2 | MED | 4 | `background/context-menus.ts` vs `popup/tab-tracking-api.ts` | Context-menu "Control Tab" skips the cloaked/internal-page guard (**privacy leak**, rule 7), content-script inject, and stop-recording-on-release. | Open — extract shared `trackTab`/`untrackTab`. |
| F3 | MED | 3 | `src/lib/storage-utils.ts` `writeStorage` | Never checks `chrome.runtime.lastError` → reports success on a failed/over-quota write (state save, recording, tracked-tab). | Open — reject on `lastError` (audit all `void set*` callers first). |
| F4 | MED | 1 | `src/sidepanel.ts` (exit/close/minimize) | Three close paths re-list the same teardown; one more drifts and leaks `resetWriteGuardState`. | Open — one `closePanelWithIntent(intent, {clearSession})`. |
| F5 | MED | 2+5 | `src/popup/action-recording.ts` | Restores "recording" from the storage mirror with no daemon revalidation → phantom recording after a daemon restart. | Open — add `event_recording_status`, reconcile on popup open. |
| F6 | MED | 4 | `keyboard-shortcuts.ts` vs `context-menus.ts` | Action-sequence recording copy-inlines the toggle (context menu is silent, no toast, no `trackUIFeature`). | Open — `toggleActionSequenceRecording` helper. |
| F7 | MED | 4 | popup/launcher/MCP draw-mode | `trackUIFeature('annotations')` fires only on keyboard+context-menu → analytics undercounted. | Open — route through shared helper. |
| F8 | LOW | 3 | `internal/pty/upload.go` | Deferred `f.Close()` error dropped → truncated upload reported as success. | Open — capture close err, remove partial. |
| F9 | LOW | 3 | `background/commands/interact.ts` (subtitle) | `sendMessage(...).catch(()=>{})` then unconditional `success:true`. | Open — await + report real result. |
| F10 | LOW | 4 | terminal keyboard vs context menu | Keyboard opens-only; `toggleTerminalSidePanel`'s doc claims it unifies both. | Open — align code or fix the comment. |

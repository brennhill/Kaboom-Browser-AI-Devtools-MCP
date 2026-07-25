// @ts-nocheck
/**
 * @fileoverview entry-point-parity.test.js — Guardrail for bug Class 4
 * ("multi-entry-point action with divergent implementations", CLAUDE.md rule 19).
 *
 * Every action a user can trigger from more than one place (keyboard, context
 * menu, popup, in-page launcher) must route through ONE shared helper, so the
 * guards/tracking/toasts cannot drift between entry points. This whole batch of
 * bugs was drift: the context menu skipped the cloaked-domain privacy guard (F2)
 * and the action-recording toasts/tracking (F6); the keyboard terminal shortcut
 * was open-only while its shared helper claimed to unify it (F10); the popup and
 * launcher never counted annotations (F7).
 *
 * If this fails: route the entry point through the shared helper named in the
 * assertion instead of re-implementing it. See docs/core/bug-class-audit.md.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import { readSrc, parseSource, fileContainsCall, functionContainsCall, SRC_ROOT } from './source-contract-utils.js'
import { join } from 'node:path'

const contextMenus = readSrc('background/context-menus.ts')
const keyboard = readSrc('background/keyboard-shortcuts.ts')
const popupDraw = readSrc('popup/draw-mode.ts')
const launcher = readSrc('content/ui/tracked-hover-launcher.ts')

const ast = (rel) => parseSource(join(SRC_ROOT, rel))
const contextMenusAst = ast('background/context-menus.ts')
const keyboardAst = ast('background/keyboard-shortcuts.ts')
const trackingApiAst = ast('popup/tab-tracking-api.ts')

// `callsHelper` verifies an actual CallExpression exists — unlike a substring
// check, it is NOT satisfied by an import or a same-named definition. That
// matters for keyboard-shortcuts.ts, which DEFINES toggleActionSequenceRecording
// and would pass a naive `includes()` even if the listener stopped calling it.
function callsHelper(sourceAst, callee, why) {
  assert.ok(fileContainsCall(sourceAst, callee), `Entry point must CALL the shared helper: ${why} (no call to ${callee}())`)
}
function fnCallsHelper(sourceAst, fnName, callee, why) {
  assert.ok(functionContainsCall(sourceAst, fnName, callee), `${fnName} must CALL ${callee}(): ${why}`)
}
function mustNotContain(src, needle, why) {
  assert.ok(!src.includes(needle), `Entry point re-inlined a primitive instead of the shared helper: ${why} (found "${needle}")`)
}

describe('entry-point parity (Class 4, rule 19)', () => {
  test('tab tracking goes through the shared trackTab/untrackTab core (F2 privacy guard)', () => {
    // The context menu must not persist tracking directly — that skipped the
    // internal-page and cloaked-domain guards the popup enforced.
    callsHelper(contextMenusAst, 'trackTab', 'context-menu Control Tab')
    callsHelper(contextMenusAst, 'untrackTab', 'context-menu Release Control')
    mustNotContain(contextMenus, 'setTrackedTab(', 'context menu must not persist tracking directly')
    mustNotContain(contextMenus, 'clearTrackedTab(', 'context menu must not clear tracking directly')
    callsHelper(trackingApiAst, 'trackTab', 'popup track')
    callsHelper(trackingApiAst, 'untrackTab', 'popup untrack')
  })

  test('action-sequence recording goes through toggleActionSequenceRecording (F6)', () => {
    // AST call-check (not substring): keyboard-shortcuts.ts defines this helper,
    // so a substring check would pass even if the listener re-inlined start/stop.
    fnCallsHelper(keyboardAst, 'installRecordingShortcutCommandListener', 'toggleActionSequenceRecording',
      'keyboard shortcut must route through the shared toggle')
    callsHelper(contextMenusAst, 'toggleActionSequenceRecording', 'context menu action-record item')
    mustNotContain(contextMenus, 'actionRecordingHandlers.startRecording', 'must not inline action-recording start')
    mustNotContain(contextMenus, 'actionRecordingHandlers.stopRecording', 'must not inline action-recording stop')
  })

  test('the terminal keyboard shortcut toggles via the shared helper, not open-only (F10)', () => {
    fnCallsHelper(keyboardAst, 'installTerminalPanelCommandListener', 'toggleTerminalSidePanel',
      'keyboard terminal shortcut must toggle via the shared helper')
    mustNotContain(keyboard, 'openTerminalSidePanel', 'keyboard shortcut must not call the open-only path directly')
    callsHelper(contextMenusAst, 'toggleTerminalSidePanel', 'context menu terminal item')
  })

  test('popup and in-page launcher report annotation usage (F7 undercount)', () => {
    // These send a runtime-message string literal, so a substring check is the
    // right contract here (there is no helper function to call).
    assert.ok(popupDraw.includes('track_ui_feature'), 'popup draw-mode must report annotation usage to the background')
    assert.ok(launcher.includes('track_ui_feature'), 'in-page launcher must report annotation usage to the background')
  })
})

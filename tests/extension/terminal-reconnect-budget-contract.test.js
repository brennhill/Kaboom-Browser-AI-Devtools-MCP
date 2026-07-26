// @ts-nocheck
/**
 * terminal-reconnect-budget-contract.test.js — the write-guard's give-up budget
 * must outlive the iframe's reconnect schedule.
 *
 * Finding S1: TERMINAL_GUARD_MAX_WAIT_MS was a hand-picked 30s while the iframe
 * (terminal.html) does not emit `reconnect_exhausted` until ~45s (delays
 * 1,2,4,8,10,10,10 → attempt 7). The guard therefore discarded the queued agent
 * writes 15s BEFORE the parent's recovery even started, so the queue could never
 * survive the outage it exists for.
 *
 * The two budgets live in different worlds — terminal.html is a hand-authored,
 * Go-embedded asset; the guard is compiled TS — so they cannot import from each
 * other. This test is the joint: the TS side declares the schedule and DERIVES
 * the budget from it, and these assertions pin that declaration to the literals
 * terminal.html actually uses.
 */
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'
import assert from 'node:assert'

import {
  TERMINAL_GUARD_MAX_WAIT_MS,
  TERMINAL_RECONNECT_BASE_DELAY_MS,
  TERMINAL_RECONNECT_MAX_DELAY_MS,
  TERMINAL_MAX_RECONNECT_ATTEMPTS,
  terminalReconnectExhaustionMs
} from '../../extension/content/ui/terminal-widget-types.js'

const TERMINAL_HTML = 'cmd/browser-agent/internal/terminal/terminal_assets/terminal.html'
const html = readFileSync(TERMINAL_HTML, 'utf8')

/** Pull an integer out of terminal.html, failing loudly if the shape changed. */
function htmlInt(re, label) {
  const m = html.match(re)
  assert.ok(m, `could not read ${label} from terminal.html — the reconnect schedule moved`)
  return Number(m[1])
}

const htmlBaseDelay = htmlInt(/var\s+reconnectDelay\s*=\s*(\d+)/, 'the initial reconnect delay')
const htmlMaxDelay = htmlInt(
  /reconnectDelay\s*=\s*Math\.min\(\s*reconnectDelay\s*\*\s*2\s*,\s*(\d+)\s*\)/,
  'the reconnect delay ceiling'
)
const htmlMaxAttempts = htmlInt(/var\s+MAX_RECONNECT_ATTEMPTS\s*=\s*(\d+)/, 'MAX_RECONNECT_ATTEMPTS')

/**
 * Wall-clock time terminal.html spends before it gives up, computed from its own
 * literals. It waits before EVERY attempt including the one that trips the cap
 * (`reconnectAttempts > MAX_RECONNECT_ATTEMPTS` is checked after the increment
 * inside the timer), so there are MAX+1 waits.
 */
function htmlExhaustionMs() {
  let delay = htmlBaseDelay
  let total = 0
  for (let i = 0; i <= htmlMaxAttempts; i++) {
    total += delay
    delay = Math.min(delay * 2, htmlMaxDelay)
  }
  return total
}

describe('terminal reconnect budget contract', () => {
  test('the guard outlives the iframe reconnect schedule', () => {
    const exhaustion = htmlExhaustionMs()
    assert.ok(
      TERMINAL_GUARD_MAX_WAIT_MS >= exhaustion,
      `TERMINAL_GUARD_MAX_WAIT_MS (${TERMINAL_GUARD_MAX_WAIT_MS}ms) gives up before the iframe ` +
        `emits reconnect_exhausted (${exhaustion}ms): queued writes are dropped before recovery starts`
    )
  })

  test('the TS-side schedule matches the literals terminal.html uses', () => {
    assert.strictEqual(
      TERMINAL_RECONNECT_BASE_DELAY_MS,
      htmlBaseDelay,
      'the declared base reconnect delay drifted from terminal.html'
    )
    assert.strictEqual(
      TERMINAL_RECONNECT_MAX_DELAY_MS,
      htmlMaxDelay,
      'the declared reconnect delay ceiling drifted from terminal.html'
    )
    assert.strictEqual(
      TERMINAL_MAX_RECONNECT_ATTEMPTS,
      htmlMaxAttempts,
      'the declared reconnect attempt cap drifted from terminal.html'
    )
  })

  test('the guard budget is derived from the schedule, not hand-picked', () => {
    const derived = terminalReconnectExhaustionMs()
    assert.strictEqual(
      derived,
      htmlExhaustionMs(),
      'terminalReconnectExhaustionMs() must reproduce terminal.html own schedule'
    )
    assert.ok(
      TERMINAL_GUARD_MAX_WAIT_MS > derived,
      'the guard must allow the parent some time to act on reconnect_exhausted, not give up at the same instant'
    )
  })
})

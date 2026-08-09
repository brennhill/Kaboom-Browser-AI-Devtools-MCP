// connected-fixture-determinism.test.cjs — Fixture-dependent actions must establish the fixture.
'use strict'

const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const { describe, test } = require('node:test')

const COVERAGE = 'scripts/tests/browser/cat-33-connected-action-coverage.sh'

function section(source, opener) {
  const start = source.indexOf(opener)
  assert.notEqual(start, -1, `${opener} must exist`)
  const end = source.indexOf('\n}', start)
  return source.slice(start, end)
}

/** Every `mode) echo '<args>'` pair declared by action_args. */
function declaredArgs(source) {
  const body = section(source, 'action_args() {')
  return [...body.matchAll(/^\s+([a-z]+\/[a-z_]+)\)\s+echo\s+(.+?);;\s*$/gm)].map((m) => ({
    mode: m[1],
    args: m[2]
  }))
}

describe('connected action coverage fixture determinism', () => {
  const source = readFileSync(COVERAGE, 'utf8')

  // interact/highlight looked for #sf-btn on whatever page a preceding navigate
  // had left up, and observe/indexeddb seeded its database against that page's
  // origin. Nineteen actions were in that state, so the category failed on a
  // different pair every run depending on execution order.
  test('an action whose arguments name fixture DOM establishes the fixture first', () => {
    const gate = section(source, 'action_targets_fixture_dom() {')
    const patternMatch = gate.match(/grep -qE '([^']+)'/)
    assert.ok(patternMatch, 'the fixture gate must declare a marker pattern')
    const markers = new RegExp(patternMatch[1])

    // The gate must actually run before the action is prepared and invoked.
    assert.match(
      source,
      /action_targets_fixture_dom "\$\(action_args[^\n]*\n\s*ensure_fixture_page \|\| continue/,
      'the fixture gate must run before prepare_action and the invocation'
    )

    // Any fixture selector used in an argument must be one the gate recognises,
    // otherwise a newly added action silently escapes the guarantee.
    const selectors = new Set()
    for (const { args } of declaredArgs(source)) {
      for (const m of args.matchAll(/"(?:selector|wait_for|submit_selector|target|database)":"([^"]+)"/g)) {
        selectors.add(m[1])
      }
    }

    const escaped = [...selectors].filter((selector) => {
      // Generic selectors (body, a, html) do not depend on the fixture.
      if (!/^[#.]|kaboom_uat/.test(selector)) return false
      return !markers.test(selector)
    })

    assert.deepEqual(
      escaped,
      [],
      `these fixture selectors are not recognised by action_targets_fixture_dom, so their actions run on whatever page precedes them:\n  ${escaped.join('\n  ')}`
    )
  })

  test('the fixture gate is reachable from the action loop', () => {
    // A gate defined but never called would restore the original flakiness.
    const calls = [...source.matchAll(/action_targets_fixture_dom\b/g)]
    assert.ok(calls.length >= 2, 'the fixture gate must be defined and called')
  })
})

// affected-tests.test.mjs — Proves the selector cannot repeat the failure it
// exists to prevent: a green branch gate that never ran the test the change broke.

import { test, describe } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'

const {
  selectTests,
  untraceable,
  isTestFile,
  loadAlwaysRun,
  listGraphFiles,
  buildGraph,
  invert,
  testsReaching,
  REPO_ROOT
} = await import('./affected-tests.mjs')

describe('the selector reaches the tests a scoped glob missed', () => {
  test('a change to the CDP dispatch module selects the iframe test that pins it', () => {
    // This is the exact regression in kaboom-2n0x: the branch ran
    // tests/extension/dom/* and extension/background/**tests**/*, but
    // dom-primitives-iframe.test.js sits in extension/background/ and pins the
    // executeScript call count that the change altered.
    const selection = selectTests(['src/background/dom/cdp/cdp-dispatch.ts'])
    assert.ok(
      selection.js_tests.includes('extension/background/dom-primitives-iframe.test.js'),
      `the test the branch broke was not selected:\n${selection.js_tests.join('\n')}`
    )
  })

  test('a TypeScript change reaches tests that only know the compiled file', () => {
    // Nothing under tests/ mentions src/. Without the src -> extension mapping a
    // TypeScript edit would select nothing at all and every branch would be green.
    const selection = selectTests(['src/background/dom/cdp/cdp-dispatch.ts'])
    assert.ok(selection.js_tests.length > 3, `selected ${selection.js_tests.length} tests`)
    assert.ok(selection.js_tests.some((file) => file.startsWith('tests/')))
  })

  test('the selection is a real subset, not everything', () => {
    // Control: selecting every test would satisfy the two assertions above while
    // being useless — the whole point is to run fewer than all of them.
    const selection = selectTests(['src/background/dom/cdp/cdp-dispatch.ts'])
    const all = listGraphFiles().filter(isTestFile)
    assert.ok(all.length > 100, `only ${all.length} test files were found; the walk is broken`)
    assert.ok(
      selection.js_tests.length < all.length,
      `selected ${selection.js_tests.length} of ${all.length} — that is not a selection`
    )
  })

  test('an unrelated change does not drag in the whole suite', () => {
    const selection = selectTests(['src/popup/popup-connection.ts'])
    assert.strictEqual(selection.full_suite, false)
    assert.ok(
      !selection.js_tests.includes('extension/background/dom-primitives-iframe.test.js'),
      'a popup change selected a DOM primitives test, so the graph is not discriminating'
    )
  })
})

describe('a change nothing can bound runs everything', () => {
  test('a Makefile change forces the full suite and says why', () => {
    const selection = selectTests(['Makefile'])
    assert.strictEqual(selection.full_suite, true)
    assert.match(selection.full_suite_reason, /Makefile/)
  })

  test('goldens, configs and generators are all untraceable', () => {
    const blocked = untraceable([
      'cmd/browser-agent/testdata/mcp-tools-list.golden.json',
      'tsconfig.json',
      'scripts/build/generate-wire-types.js',
      'src/background/init.ts'
    ])
    // The generator is a .js file and IS in the graph, so it is traced rather
    // than blocking — its output is what a golden change would catch.
    assert.deepStrictEqual(blocked, [
      'cmd/browser-agent/testdata/mcp-tools-list.golden.json',
      'tsconfig.json'
    ])
  })

  test('a Go change is traced, not treated as unboundable', () => {
    assert.deepStrictEqual(untraceable(['internal/hook/session.go']), [])
  })
})

describe('the always-run list stays honest', () => {
  test('every entry names a file that exists and says why it cannot be traced', () => {
    const entries = loadAlwaysRun()
    assert.ok(entries.length > 0, 'the list is empty; if that is right, delete the mechanism')
    for (const entry of entries) {
      assert.ok(
        fs.existsSync(path.join(REPO_ROOT, entry.file)),
        `${entry.file} is listed but does not exist, so it silently runs nothing`
      )
      assert.ok(
        (entry.reason ?? '').length > 30,
        `${entry.file} has no stated reason; without one this list becomes a dumping ground`
      )
    }
  })

  test('always-run entries appear in every selection', () => {
    const selection = selectTests(['src/popup/popup-connection.ts'])
    for (const entry of loadAlwaysRun()) {
      assert.ok(
        selection.js_tests.includes(entry.file),
        `${entry.file} is on the always-run list but was not selected`
      )
    }
  })
})

describe('the walk follows importers transitively', () => {
  test('a module two hops from a test still selects it', () => {
    const importers = invert(
      new Map([
        ['a.test.js', ['middle.js']],
        ['middle.js', ['deep.js']],
        ['deep.js', []]
      ])
    )
    assert.deepStrictEqual(testsReaching(['deep.js'], importers), ['a.test.js'])

    // Control: a module nothing imports selects nothing, or the walk would be
    // returning every test regardless of the change.
    assert.deepStrictEqual(testsReaching(['unrelated.js'], importers), [])
  })

  test('a cycle terminates', () => {
    const importers = invert(
      new Map([
        ['a.test.js', ['one.js']],
        ['one.js', ['two.js']],
        ['two.js', ['one.js']]
      ])
    )
    assert.deepStrictEqual(testsReaching(['two.js'], importers), ['a.test.js'])
  })

  test('imports written through a variable are still followed', () => {
    // Tests here do `const CDP = '../../../extension/...'` then `await
    // import(CDP)`. Matching only literal import statements would miss every
    // one of them, which is most of the DOM suite.
    const graph = buildGraph(['tests/extension/dom/cdp-gestures.test.js'])
    const reached = graph.get('tests/extension/dom/cdp-gestures.test.js') ?? []
    assert.ok(
      reached.some((file) => file.includes('cdp')),
      `the variable import was not followed: ${JSON.stringify(reached)}`
    )
  })
})

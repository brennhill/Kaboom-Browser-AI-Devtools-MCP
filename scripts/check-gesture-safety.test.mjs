/**
 * check-gesture-safety.test.mjs — the gesture rule has to be right about the
 * shapes that actually occur, or it becomes noise people disable.
 *
 * A lint rule that cries wolf gets suppressed, and a rule with holes is worse
 * than none because it implies coverage. Both halves are pinned here: what it
 * must catch, and what it must leave alone.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import { readdirSync, statSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { analyzeSource, findGestureViolations, ALLOWED_CALLER, ALLOW_MARKER } from './check-gesture-safety.mjs'

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

/** Analyze a snippet as if it were the shared opener. */
function inOpener(source) {
  return analyzeSource(ALLOWED_CALLER, source)
}

describe('gesture safety: what it must catch', () => {
  test('an await on the way to open() is a violation', () => {
    // The exact regression: a storage read to decide open-vs-close burned the
    // gesture, and "Open Kaboom Terminal" silently did nothing.
    const violations = inOpener(`
      export async function openPanel(tabId: number) {
        const isOpen = await chrome.storage.session.get('state')
        if (isOpen) return
        await chrome.sidePanel.open({ tabId })
      }
    `)
    assert.strictEqual(violations.length, 1)
    assert.strictEqual(violations[0].rule, 'no-await-before-open')
    assert.strictEqual(violations[0].line, 3)
  })

  test('an await earlier in the same statement counts', () => {
    const violations = inOpener(`
      export async function openPanel() {
        await chrome.sidePanel.open({ tabId: await resolveTab() })
      }
    `)
    assert.strictEqual(violations.length, 1, 'the argument is evaluated first, so the gesture is gone')
  })

  test('opening from anywhere but the shared opener is a violation', () => {
    // Every entry point routes through one opener (repo rule 19). A second
    // call site is a second place for these rules to be relearned the hard way.
    const violations = analyzeSource('src/background/context-menus.ts', `
      chrome.contextMenus.onClicked.addListener((info, tab) => {
        chrome.sidePanel.open({ tabId: tab.id })
      })
    `)
    assert.strictEqual(violations.length, 1)
    assert.strictEqual(violations[0].rule, 'single-opener')
  })

  test('several awaits before one open are each reported', () => {
    const violations = inOpener(`
      export async function openPanel(tabId: number) {
        const a = await one()
        const b = await two()
        await chrome.sidePanel.open({ tabId })
      }
    `)
    assert.strictEqual(violations.length, 2, 'each await is separately fixable')
  })
})

describe('gesture safety: what it must leave alone', () => {
  test('awaiting open() itself is not "before" it', () => {
    assert.deepStrictEqual(
      inOpener(`
        export async function openPanel(tabId: number) {
          await chrome.sidePanel.open({ tabId })
        }
      `),
      []
    )
  })

  test('awaits after open() are fine — the gesture has done its job', () => {
    assert.deepStrictEqual(
      inOpener(`
        export async function openPanel(tabId: number) {
          await chrome.sidePanel.open({ tabId })
          await refineWorkspace(tabId)
        }
      `),
      []
    )
  })

  test('a guard clause that returns does not reach the later open', () => {
    // The real shape of openTerminalSidePanel: a fast path that opens and
    // returns, then a slow path. The fast path's await never precedes the slow
    // path's open, because taking it means never getting there.
    assert.deepStrictEqual(
      inOpener(`
        export async function openPanel(tabId?: number) {
          if (typeof tabId === 'number') {
            await chrome.sidePanel.open({ tabId })
            return { success: true }
          }
          const target = resolveSync()
          await chrome.sidePanel.open({ tabId: target })
          return { success: true }
        }
      `),
      []
    )
  })

  test('an await inside a callback does not run before the open', () => {
    assert.deepStrictEqual(
      inOpener(`
        export async function openPanel(tabId: number) {
          void (async () => { await later() })()
          await chrome.sidePanel.open({ tabId })
        }
      `),
      []
    )
  })

  test('a dispatched promise that is never awaited is fine', () => {
    // This is the pattern the opener relies on: setOptions is fired so Chrome
    // orders it before the open, but never awaited, so the gesture survives.
    assert.deepStrictEqual(
      inOpener(`
        export async function openPanel(tabId: number) {
          void chrome.sidePanel.setOptions({ tabId, enabled: true })
          await chrome.sidePanel.open({ tabId })
        }
      `),
      []
    )
  })

  test('files that never open the panel are not scanned', () => {
    assert.deepStrictEqual(
      analyzeSource('src/background/anything.ts', 'export async function f() { await g() }'),
      []
    )
  })
})

describe('gesture safety: the escape hatch', () => {
  test('the marker suppresses a violation', () => {
    assert.deepStrictEqual(
      inOpener(`
        export async function openPanel() {
          // ${ALLOW_MARKER} — no tab id available; best-effort slow path
          const target = await resolveTab()
          await chrome.sidePanel.open({ tabId: target })
        }
      `),
      []
    )
  })

  test('the marker on the awaiting line itself works', () => {
    assert.deepStrictEqual(
      inOpener(`
        export async function openPanel() {
          const target = await resolveTab() // ${ALLOW_MARKER} — best-effort slow path
          await chrome.sidePanel.open({ tabId: target })
        }
      `),
      []
    )
  })

  test('an unrelated comment does not suppress anything', () => {
    const violations = inOpener(`
      export async function openPanel() {
        // this is fine, honestly
        const target = await resolveTab()
        await chrome.sidePanel.open({ tabId: target })
      }
    `)
    assert.strictEqual(violations.length, 1)
  })
})

describe('the repository itself', () => {
  test('src/ has no unannotated gesture violations', () => {
    const files = []
    const walk = (dir) => {
      for (const entry of readdirSync(dir)) {
        const full = path.join(dir, entry)
        if (statSync(full).isDirectory()) walk(full)
        else if (entry.endsWith('.ts') && !entry.endsWith('.d.ts')) files.push(full)
      }
    }
    walk(path.join(REPO_ROOT, 'src'))

    const violations = findGestureViolations(files)
    assert.deepStrictEqual(
      violations.map((v) => `${v.file}:${v.line} [${v.rule}]`),
      [],
      'run `node scripts/check-gesture-safety.mjs` for the details'
    )
  })
})

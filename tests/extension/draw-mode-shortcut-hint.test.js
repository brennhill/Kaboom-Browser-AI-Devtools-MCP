/**
 * Contract: the draw-mode overlay must remind the user of the keyboard shortcut
 * that toggles the mode, and that shortcut must match the one Chrome actually
 * binds (manifest.json → commands.toggle_draw_mode).
 *
 * Why: the shortcut is hardcoded in extension/content/draw-mode.js (a plain-JS
 * web-accessible resource injected into the page, so it can't read the manifest
 * at runtime). If someone remaps the command in the manifest, the on-screen hint
 * would silently lie. This test fails the build when the two drift.
 */
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

const MANIFEST = 'extension/manifest.json'
const DRAW_MODE = 'extension/content/draw-mode.js'

describe('draw-mode overlay shortcut hint', () => {
  const manifest = JSON.parse(readFileSync(MANIFEST, 'utf8'))
  const drawMode = readFileSync(DRAW_MODE, 'utf8')

  const manifestShortcut = manifest?.commands?.toggle_draw_mode?.suggested_key?.default
  const hintShortcut = drawMode.match(/TOGGLE_DRAW_MODE_SHORTCUT\s*=\s*['"]([^'"]+)['"]/)?.[1]

  test('manifest defines a toggle_draw_mode default shortcut', () => {
    assert.ok(manifestShortcut, 'manifest.json commands.toggle_draw_mode.suggested_key.default must be set')
  })

  test('the overlay hint shortcut matches the manifest binding', () => {
    assert.ok(hintShortcut, 'draw-mode.js must define TOGGLE_DRAW_MODE_SHORTCUT')
    assert.strictEqual(
      hintShortcut,
      manifestShortcut,
      `draw-mode.js hints '${hintShortcut}' but manifest binds '${manifestShortcut}' — the on-screen reminder would be wrong.`
    )
  })

  test('both overlay hints actually surface the toggle shortcut to the user', () => {
    // The persistent bottom bar (always visible) and the center toast (first-run
    // reminder) must each mention the toggle shortcut, or the reminder is missing.
    const escHint = drawMode.match(/escHint\.textContent\s*=\s*`([^`]+)`/)?.[1] ?? ''
    const instruction = drawMode.match(/instruction\.innerHTML\s*=([\s\S]*?)overlay\.appendChild\(instruction\)/)?.[1] ?? ''
    assert.match(escHint, /TOGGLE_DRAW_MODE_SHORTCUT/, 'the persistent ESC-hint bar must include the toggle shortcut')
    assert.match(instruction, /TOGGLE_DRAW_MODE_SHORTCUT/, 'the center instruction toast must include the toggle shortcut')
  })
})

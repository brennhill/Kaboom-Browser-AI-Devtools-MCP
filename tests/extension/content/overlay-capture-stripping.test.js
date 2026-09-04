// @ts-nocheck
/**
 * @fileoverview overlay-capture-stripping.test.js — Every Kaboom overlay must be strippable
 * before a screenshot, by construction rather than by remembering to list it.
 *
 * Why this exists: setKaboomOverlayVisibility hid a HARDCODED list of two element ids,
 * ['kaboom-tracked-hover-launcher', 'kaboom-draw-toolbar']. The second id is created
 * nowhere in the codebase — the draw overlay's real roots are kaboom-draw-overlay,
 * kaboom-draw-badge and kaboom-draw-instruction. So every screenshot taken while draw mode
 * was active captured Kaboom's own overlay, and the agent then reasoned about its own UI as
 * if it were page content. A list you must remember to update is the defect; the marker
 * attribute is the fix, and this test is what keeps it true.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const SRC = fileURLToPath(new URL('../../../src/', import.meta.url))
const read = (rel) => readFileSync(SRC + rel, 'utf8')

/**
 * Strip comments before asserting on code. The bridge deliberately DOCUMENTS the old broken
 * id in a comment so the next reader knows why the marker exists; a naive substring check
 * would read that explanation as the defect it warns about.
 */
const readCode = (rel) =>
  read(rel)
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|[^:])\/\/.*$/gm, '$1')

/** The single marker every overlay root carries and the stripper selects on. */
const MARKER = 'data-kaboom-overlay'

describe('overlay capture stripping', () => {
  test('the stripper selects by marker attribute, not by a hardcoded id list', () => {
    const bridge = readCode('background/ui/content-script-bridge.ts')
    assert.ok(
      bridge.includes(`[${MARKER}]`),
      `setKaboomOverlayVisibility must query [${MARKER}] so a new overlay cannot be forgotten`
    )
    assert.ok(
      !bridge.includes('kaboom-draw-toolbar'),
      'kaboom-draw-toolbar is created nowhere — a stripper listing it hides nothing (rule 26: delete the obsolete surface)'
    )
  })

  test('every overlay root sets the marker attribute', () => {
    // Each entry is a module that creates a visible, page-level overlay root, and the
    // identifier of the element variable it must mark.
    const roots = [
      ['content/ui/tracked-hover-launcher.ts', 'tracked hover launcher'],
      ['content/draw-mode/lifecycle-overlay.js', 'draw mode overlay'],
      ['content/ui/supervision/agent-indicator.ts', 'agent supervision indicator']
    ]
    for (const [rel, label] of roots) {
      const source = read(rel)
      assert.ok(
        source.includes(MARKER),
        `${label} (${rel}) must set ${MARKER} on its root, or it will appear in every screenshot`
      )
    }
  })

  test('the draw overlay ids the stripper used to name still exist', () => {
    // Guards the reverse mistake: renaming a root without updating anything that reasons
    // about it. These are the real ids, verified against the source.
    const drawSource = read('content/draw-mode/lifecycle-overlay.js')
    assert.ok(drawSource.includes('kaboom-draw-overlay'), 'draw overlay root id changed')
  })
})

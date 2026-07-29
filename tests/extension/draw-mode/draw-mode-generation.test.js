// draw-mode-generation.test.js — Canonical partial and generated-artifact contracts.

import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { readFileSync, readdirSync } from 'node:fs'
import { describe, test } from 'node:test'

const PARTIAL_DIR = 'src/content/draw-mode'
const GENERATED_FILE = 'extension/content/draw-mode.js'
const PARTIALS = [
  'lifecycle-overlay.js',
  'input-rendering.js',
  'element-capture.js',
  'element-analysis.js',
  'persistence-submission.js',
  'geometry-context.js'
]

describe('draw-mode generation', () => {
  test('canonical partials are change-coupled, bounded, and complete', () => {
    assert.deepEqual(
      readdirSync(PARTIAL_DIR).filter((name) => name.endsWith('.js')).sort(),
      [...PARTIALS].sort()
    )
    assert.ok(PARTIALS.length <= 10)
    for (const partial of PARTIALS) {
      const lines = readFileSync(`${PARTIAL_DIR}/${partial}`, 'utf8').split('\n').length
      assert.ok(lines <= 800, `${partial} has ${lines} lines`)
    }
  })

  test('generated extension artifact is byte-current with canonical partials', () => {
    execFileSync('node', ['scripts/build/generate-draw-mode.js', '--check'], {
      cwd: process.cwd(),
      stdio: 'pipe'
    })
    assert.match(readFileSync(GENERATED_FILE, 'utf8'), /GENERATED FILE[\s\S]*generate-draw-mode\.js/)
  })
})

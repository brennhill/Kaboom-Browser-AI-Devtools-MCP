// dom-primitives-generation.test.js — Contracts for canonical self-contained DOM primitive generation.
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'

describe('DOM primitive generation', () => {
  test('intent and overlay primitives share the canonical injected helper partial', () => {
    const intent = readFileSync('scripts/templates/dom-primitives-intent.ts.tpl', 'utf8')
    const overlay = readFileSync('scripts/templates/dom-primitives-overlay.ts.tpl', 'utf8')

    assert.match(intent, /@include shared\/_dom-self-contained-core\.tpl/)
    assert.match(overlay, /@include shared\/_dom-self-contained-core\.tpl/)
  })

  test('all generated primitives are current and mark required emitted duplication', () => {
    execFileSync(process.execPath, ['scripts/build/generate-dom-primitives.js', '--check'], {
      cwd: process.cwd(),
      stdio: 'pipe'
    })

    for (const family of ['pointer', 'form', 'read', 'intent', 'overlay']) {
      const generated = readFileSync(`src/background/dom/primitives/dom-primitives-${family}.ts`, 'utf8')
      assert.match(generated, /AUTO-GENERATED FILE/)
      assert.match(generated, /jscpd:ignore-start/)
      assert.match(generated, /jscpd:ignore-end/)
    }
  })
})

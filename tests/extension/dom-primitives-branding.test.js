// @ts-nocheck
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

const DOM_PRIMITIVE_FILES = [
  'src/background/dom/primitives/dom-primitives-pointer.ts',
  'src/background/dom/primitives/dom-primitives-form.ts',
  'src/background/dom/primitives/dom-primitives-read.ts',
  'src/background/dom/primitives/dom-primitives-list-interactive.ts',
  'src/background/dom/primitives/dom-primitives-intent.ts',
  'src/background/dom/primitives/dom-primitives-overlay.ts'
]

describe('DOM primitive branding contracts', () => {
  test('element handle stores use Kaboom-scoped globals', () => {
    for (const relativePath of DOM_PRIMITIVE_FILES) {
      const contents = readFileSync(relativePath, 'utf8')

      assert.doesNotMatch(
        contents,
        /__gasolineElementHandles/,
        `${relativePath} still references legacy __gasolineElementHandles storage`
      )
      assert.match(contents, /__kaboomElementHandles/, `${relativePath} should use __kaboomElementHandles storage`)
    }
  })

  test('DOM ownership uses an explicit marker instead of page naming conventions', () => {
    const selectorTemplate = readFileSync('scripts/templates/partials/_dom-selectors.tpl', 'utf8')
    const standalonePrimitives = [
      'src/background/dom/primitives/dom-primitives-intent.ts',
      'src/background/dom/primitives/dom-primitives-overlay.ts'
    ]

    for (const [relativePath, contents] of [
      ['scripts/templates/partials/_dom-selectors.tpl', selectorTemplate],
      ...standalonePrimitives.map(relativePath => [relativePath, readFileSync(relativePath, 'utf8')])
    ]) {
      assert.match(
        contents,
        /getAttribute\('data-kaboom-owned'\) === 'true'/,
        `${relativePath} should recognize the explicit Kaboom ownership marker`
      )
      assert.doesNotMatch(
        contents,
        /id\.startsWith\('kaboom-'\)|className\.includes\('kaboom-'\)/,
        `${relativePath} must not claim ordinary page elements by ID or class prefix`
      )
    }
  })
})

// @ts-nocheck
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

const DOM_PRIMITIVE_FILES = [
  'src/background/dom/dom-primitives.ts',
  'src/background/dom/dom-primitives-list-interactive.ts',
  'src/background/dom/dom-primitives-intent.ts',
  'src/background/dom/dom-primitives-overlay.ts'
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
})

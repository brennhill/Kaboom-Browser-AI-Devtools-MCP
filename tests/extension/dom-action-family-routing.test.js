// @ts-nocheck
/**
 * @fileoverview Regression coverage for routing DOM actions to self-contained
 * injected primitive families.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

const expectedPrimitiveByAction = {
  click: 'domPrimitivePointer',
  hover: 'domPrimitivePointer',
  focus: 'domPrimitivePointer',
  scroll_to: 'domPrimitivePointer',
  type: 'domPrimitiveForm',
  paste: 'domPrimitiveForm',
  select: 'domPrimitiveForm',
  check: 'domPrimitiveForm',
  key_press: 'domPrimitiveForm',
  set_attribute: 'domPrimitiveForm',
  get_text: 'domPrimitiveRead',
  get_value: 'domPrimitiveRead',
  get_attribute: 'domPrimitiveRead'
}

function pendingQuery(action) {
  return {
    id: `query-${action}`,
    correlation_id: `correlation-${action}`,
    type: 'dom_action',
    params: JSON.stringify({ action, selector: '#target', text: 'value', value: 'value' }),
    created_at: Date.now()
  }
}

describe('DOM action-family routing', () => {
  let injectedFunctions

  beforeEach(() => {
    injectedFunctions = []
    globalThis.chrome = {
      scripting: {
        executeScript: mock.fn((options) => {
          injectedFunctions.push(options.func.name)
          return Promise.resolve([
            {
              frameId: 0,
              result: {
                success: true,
                action: options.args?.[0] || 'unknown',
                selector: '#target'
              }
            }
          ])
        })
      },
      storage: {
        local: {
          get: mock.fn(() => Promise.resolve({})),
          set: mock.fn(() => Promise.resolve()),
          remove: mock.fn(() => Promise.resolve())
        }
      }
    }
  })

  for (const [action, expectedPrimitive] of Object.entries(expectedPrimitiveByAction)) {
    test(`${action} uses ${expectedPrimitive}`, async () => {
      const { executeDOMAction } = await import('../../extension/background/dom/dom-dispatch.js')
      await executeDOMAction(
        pendingQuery(action),
        1,
        { id: 'test-client' },
        mock.fn(),
        mock.fn()
      )

      assert.deepStrictEqual(injectedFunctions, [expectedPrimitive])
    })
  }
})

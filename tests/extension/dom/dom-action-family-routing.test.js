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
      const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')
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

/**
 * A coordinate that is not on the screen must come back as an error naming the viewport.
 *
 * Chrome does not reject an out-of-range Input.dispatchMouseEvent: it clamps the point to the
 * nearest edge and reports success, so the agent is told it clicked the thing it pointed at
 * while the click landed on whatever sits at the border.
 */
describe('coordinate actions are held inside the viewport', () => {
  let injectedFunctions
  let asyncResults

  const VIEWPORT = { viewport_width: 1280, viewport_height: 800, scroll_x: 0, scroll_y: 0 }

  function coordinateQuery(action, params) {
    return {
      id: `query-${action}`,
      correlation_id: `correlation-${action}`,
      type: 'dom_action',
      params: JSON.stringify({ action, ...params }),
      created_at: Date.now()
    }
  }

  beforeEach(() => {
    injectedFunctions = []
    asyncResults = []
    globalThis.chrome = {
      scripting: {
        executeScript: mock.fn((options) => {
          injectedFunctions.push(options.func.name)
          if (options.func.name === 'readPageViewportMetrics') {
            return Promise.resolve([{ frameId: 0, result: { ...VIEWPORT, device_pixel_ratio: 1 } }])
          }
          // The injected gesture reports an outcome, not a DOMResult — the service worker
          // builds the result from it. See buildGestureDOMResult.
          return Promise.resolve([{ frameId: 0, result: { dispatched: true, button: 'none' } }])
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

  async function run(action, params) {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')
    await executeDOMAction(
      coordinateQuery(action, params),
      1,
      { id: 'test-client' },
      mock.fn((_client, _id, _correlation, status, _result, error) => asyncResults.push({ status, error })),
      mock.fn()
    )
  }

  test('a hover_at past the right edge is refused instead of dispatched', async () => {
    await run('hover_at', { x: 1900, y: 400 })
    assert.deepStrictEqual(
      asyncResults.map((entry) => entry.status),
      ['error']
    )
    assert.match(asyncResults[0].error, /outside the viewport/)
    assert.equal(
      injectedFunctions.includes('domGestureScroll'),
      false,
      'the gesture must not be dispatched after the point was refused'
    )
  })

  test('a hover_at inside the viewport still dispatches', async () => {
    await run('hover_at', { x: 640, y: 400 })
    assert.equal(injectedFunctions.includes('domGestureScroll'), true)
    assert.deepStrictEqual(
      asyncResults.map((entry) => entry.status),
      ['complete']
    )
  })
})

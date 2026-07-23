// @ts-nocheck
/**
 * @fileoverview dom-primitives-iframe.test.js — Tests for allFrames iframe support.
 *
 * Verifies that executeDOMAction dispatches to all frames via allFrames: true,
 * picks the correct frame result (main > iframe > error), and merges
 * list_interactive results across frames.
 *
 * Run: node --test extension/background/dom-primitives-iframe.test.js
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// Minimal DOM + Chrome mocks
// ---------------------------------------------------------------------------
class MockHTMLElement {
  constructor(tag, props = {}) {
    this.tagName = tag
    this.id = props.id || ''
    this.textContent = props.textContent || ''
    this.offsetParent = {}
    this.style = { position: '' }
  }
  click() {}
  focus() {}
  getAttribute() {
    return null
  }
  closest() {
    return null
  }
  querySelector() {
    return null
  }
  querySelectorAll() {
    return []
  }
  scrollIntoView() {}
  setAttribute() {}
  dispatchEvent() {}
}

globalThis.HTMLElement = MockHTMLElement
globalThis.HTMLInputElement = class extends MockHTMLElement {}
globalThis.HTMLTextAreaElement = class extends MockHTMLElement {}
globalThis.HTMLSelectElement = class extends MockHTMLElement {}
globalThis.CSS = { escape: (s) => s }
globalThis.NodeFilter = { SHOW_TEXT: 4 }
globalThis.InputEvent = class extends Event {}
globalThis.KeyboardEvent = class extends Event {}
globalThis.getComputedStyle = () => ({ visibility: 'visible', display: 'block' })
globalThis.MutationObserver = class {
  observe() {}
  disconnect() {}
}
globalThis.performance = { now: () => 0 }
globalThis.requestAnimationFrame = (cb) => cb()
globalThis.document = {
  querySelector: () => null,
  querySelectorAll: () => [],
  body: { querySelectorAll: () => [] },
  documentElement: {},
  createTreeWalker: () => ({ nextNode: () => null }),
  getSelection: () => null,
  execCommand: () => {}
}

// Track executeScript calls
let executeScriptCalls = []
let executeScriptReturn = []

globalThis.chrome = {
  scripting: {
    executeScript: async (opts) => {
      executeScriptCalls.push(opts)
      return executeScriptReturn.shift() || []
    }
  }
}

const { executeDOMAction } = await import('./dom-dispatch.js')

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function makeQuery(params) {
  return { id: 'q1', type: 'dom_action', params, correlation_id: 'c1' }
}

function makeSyncClient() {
  return {}
}

function captureAsyncResult() {
  const calls = []
  const fn = (_sc, _qid, _cid, status, result, error) => {
    calls.push({ status, result, error })
  }
  return { fn, calls }
}

function noopToast() {}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('iframe support: allFrames flag', () => {
  beforeEach(() => {
    executeScriptCalls = []
    executeScriptReturn = []
  })

  test('executeStandardAction passes allFrames: true to chrome.scripting.executeScript', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: true, action: 'click', selector: '#btn', matched: { tag: 'button', selector: '#btn' } } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(makeQuery({ action: 'click', selector: '#btn' }), 1, makeSyncClient(), res.fn, noopToast)

    assert.strictEqual(executeScriptCalls.length, 1)
    assert.strictEqual(
      executeScriptCalls[0].target.allFrames,
      true,
      'executeScript target must include allFrames: true'
    )
    assert.strictEqual(executeScriptCalls[0].target.tabId, 1)
  })

  test('executeStandardAction forwards element_id into injected options', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: true, action: 'click', selector: '', matched: { tag: 'button', selector: '', element_id: 'el_1' } } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(makeQuery({ action: 'click', element_id: 'el_1' }), 1, makeSyncClient(), res.fn, noopToast)

    assert.strictEqual(executeScriptCalls.length, 1)
    assert.strictEqual(executeScriptCalls[0].args[2].element_id, 'el_1')
    assert.strictEqual(res.calls[0].status, 'complete')
  })

  test('executeStandardAction forwards scope_rect into injected options', async () => {
    executeScriptReturn.push([
      {
        frameId: 0,
        result: {
          success: true,
          action: 'click',
          selector: '.btn',
          matched: { tag: 'button', selector: '.btn' }
        }
      }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({
        action: 'click',
        selector: '.btn',
        scope_rect: { x: 120, y: 240, width: 300, height: 200 }
      }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(executeScriptCalls.length, 1)
    assert.deepStrictEqual(executeScriptCalls[0].args[2].scope_rect, { x: 120, y: 240, width: 300, height: 200 })
    assert.strictEqual(res.calls[0].status, 'complete')
  })

  test('wait_for passes allFrames: true to both quick-check and polling calls', async () => {
    // Quick-check returns not found (main frame)
    executeScriptReturn.push([
      { frameId: 0, result: { success: false, action: 'wait_for', selector: '#x', error: 'element_not_found' } }
    ])
    // Polling fallback
    executeScriptReturn.push([
      { frameId: 0, result: { success: true, action: 'wait_for', selector: '#x', value: 'div' } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'wait_for', selector: '#x', timeout_ms: 100 }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(executeScriptCalls.length, 2, 'should make 2 executeScript calls')
    assert.strictEqual(executeScriptCalls[0].target.allFrames, true, 'quick-check must use allFrames: true')
    assert.strictEqual(executeScriptCalls[1].target.allFrames, true, 'polling fallback must use allFrames: true')
  })

  test('wait_for returns timeout when polling never finds a match', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: false, action: 'wait_for', selector: '#missing', error: 'element_not_found' } }
    ])
    executeScriptReturn.push([
      { frameId: 0, result: { success: false, action: 'wait_for', selector: '#missing', error: 'element_not_found' } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'wait_for', selector: '#missing', timeout_ms: 60 }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.ok(executeScriptCalls.length >= 2, 'wait_for should make at least quick-check + one poll call')
    assert.strictEqual(res.calls[0].status, 'error')
    assert.strictEqual(res.calls[0].result.success, false)
    assert.strictEqual(res.calls[0].result.error, 'timeout')
  })
})

describe('iframe support: pickFrameResult', () => {
  beforeEach(() => {
    executeScriptCalls = []
    executeScriptReturn = []
  })

  test('prefers main frame success when both frames succeed', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: true, action: 'get_text', selector: '#el', value: 'main-text' } },
      { frameId: 1, result: { success: true, action: 'get_text', selector: '#el', value: 'iframe-text' } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(makeQuery({ action: 'get_text', selector: '#el' }), 1, makeSyncClient(), res.fn, noopToast)

    assert.strictEqual(res.calls.length, 1)
    assert.strictEqual(res.calls[0].status, 'complete')
    assert.strictEqual(res.calls[0].result.value, 'main-text', 'should prefer main frame result when both succeed')
  })

  test('falls back to iframe success when main frame fails', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: false, action: 'click', selector: '#btn', error: 'element_not_found' } },
      { frameId: 2, result: { success: true, action: 'click', selector: '#btn', matched: { tag: 'button', selector: '#btn' } } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(makeQuery({ action: 'click', selector: '#btn' }), 1, makeSyncClient(), res.fn, noopToast)

    assert.strictEqual(res.calls[0].status, 'complete')
    assert.strictEqual(res.calls[0].result.success, true, 'should use successful iframe result when main frame fails')
    assert.strictEqual(res.calls[0].result.frame_id, 2, 'auto frame selection should include frame_id evidence')
    assert.strictEqual(res.calls[0].result.matched.frame_id, 2, 'matched evidence should include selected frame_id')
  })

  test('returns main frame error when all frames fail', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: false, action: 'click', selector: '#btn', error: 'element_not_found' } },
      { frameId: 1, result: { success: false, action: 'click', selector: '#btn', error: 'element_not_found' } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(makeQuery({ action: 'click', selector: '#btn' }), 1, makeSyncClient(), res.fn, noopToast)

    assert.strictEqual(res.calls[0].status, 'error')
    assert.strictEqual(res.calls[0].result.success, false)
    assert.strictEqual(
      res.calls[0].result.error,
      'element_not_found',
      'should return main frame error when all frames fail'
    )
  })

  test('handles single frame result (no iframes on page)', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: true, action: 'focus', selector: '#input', matched: { tag: 'input', selector: '#input' } } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(makeQuery({ action: 'focus', selector: '#input' }), 1, makeSyncClient(), res.fn, noopToast)

    assert.strictEqual(res.calls[0].status, 'complete')
    assert.strictEqual(res.calls[0].result.success, true, 'single frame result should work unchanged')
  })

  test('wait_for quick-check picks iframe success over main frame failure', async () => {
    // Quick-check: main frame fails, iframe succeeds → should return immediately
    executeScriptReturn.push([
      { frameId: 0, result: { success: false, action: 'wait_for', selector: '#el', error: 'element_not_found' } },
      { frameId: 3, result: { success: true, action: 'wait_for', selector: '#el', value: 'span' } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'wait_for', selector: '#el', timeout_ms: 100 }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    // Should have only 1 executeScript call (quick-check succeeded via iframe)
    assert.strictEqual(
      executeScriptCalls.length,
      1,
      'should not fall through to polling when quick-check found element in iframe'
    )
    assert.strictEqual(res.calls[0].result.success, true)
  })
})

describe('iframe support: no-silent-success invariants', () => {
  beforeEach(() => {
    executeScriptCalls = []
    executeScriptReturn = []
  })

  test('coerces mutating success without matched evidence into error', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: true, action: 'click', selector: '#btn' } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(makeQuery({ action: 'click', selector: '#btn' }), 1, makeSyncClient(), res.fn, noopToast)

    assert.strictEqual(res.calls[0].status, 'error')
    assert.strictEqual(res.calls[0].error, 'missing_match_evidence')
    assert.strictEqual(res.calls[0].result.success, false)
    assert.strictEqual(res.calls[0].result.error, 'missing_match_evidence')
  })

  test('coerces success payload with embedded error into status_mismatch', async () => {
    executeScriptReturn.push([
      {
        frameId: 0,
        result: {
          success: true,
          action: 'click',
          selector: '#btn',
          matched: { tag: 'button', selector: '#btn' },
          error: 'element_not_found'
        }
      }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(makeQuery({ action: 'click', selector: '#btn' }), 1, makeSyncClient(), res.fn, noopToast)

    assert.strictEqual(res.calls[0].status, 'error')
    assert.strictEqual(res.calls[0].error, 'status_mismatch')
    assert.strictEqual(res.calls[0].result.success, false)
    assert.strictEqual(res.calls[0].result.error, 'status_mismatch')
  })
})

describe('iframe support: mergeListInteractive', () => {
  beforeEach(() => {
    executeScriptCalls = []
    executeScriptReturn = []
  })

  test('merges elements from main frame and iframes', async () => {
    executeScriptReturn.push([
      {
        frameId: 0,
        result: {
          success: true,
          elements: [
            { tag: 'button', selector: '#btn1', label: 'Main Button', visible: true },
            { tag: 'a', selector: '#link1', label: 'Main Link', visible: true }
          ]
        }
      },
      {
        frameId: 1,
        result: {
          success: true,
          elements: [{ tag: 'input', selector: '#input1', label: 'Iframe Input', visible: true }]
        }
      },
      {
        frameId: 2,
        result: {
          success: true,
          elements: [{ tag: 'button', selector: '#btn2', label: 'Iframe2 Button', visible: true }]
        }
      }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'list_interactive', selector: '' }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(res.calls[0].status, 'complete')
    const result = res.calls[0].result
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.elements.length, 4, 'should merge elements from all 3 frames')
    assert.strictEqual(result.elements[0].label, 'Main Button')
    assert.strictEqual(result.elements[2].label, 'Iframe Input')
    assert.strictEqual(result.elements[3].label, 'Iframe2 Button')
  })

  test('preserves bbox metadata when merging list_interactive across frames', async () => {
    executeScriptReturn.push([
      {
        frameId: 0,
        result: {
          success: true,
          elements: [
            {
              tag: 'button',
              selector: '#main-btn',
              label: 'Main Button',
              visible: true,
              bbox: { x: 10, y: 20, width: 100, height: 30 }
            }
          ]
        }
      },
      {
        frameId: 1,
        result: {
          success: true,
          elements: [
            {
              tag: 'input',
              selector: '#frame-input',
              label: 'Iframe Input',
              visible: true,
              bbox: { x: 320, y: 180, width: 220, height: 36 }
            }
          ]
        }
      }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'list_interactive', selector: '' }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(res.calls[0].status, 'complete')
    const result = res.calls[0].result
    assert.deepStrictEqual(result.elements[0].bbox, { x: 10, y: 20, width: 100, height: 30 })
    assert.deepStrictEqual(result.elements[1].bbox, { x: 320, y: 180, width: 220, height: 36 })
  })

  test('caps merged elements at 100', async () => {
    const manyElements = Array.from({ length: 60 }, (_, i) => ({
      tag: 'button',
      selector: `#btn${i}`,
      label: `Button ${i}`,
      visible: true
    }))
    executeScriptReturn.push([
      { frameId: 0, result: { success: true, elements: manyElements } },
      { frameId: 1, result: { success: true, elements: manyElements } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'list_interactive', selector: '' }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(res.calls[0].result.elements.length, 100, 'should cap at 100 elements across all frames')
  })

  test('handles frames with no elements gracefully', async () => {
    executeScriptReturn.push([
      {
        frameId: 0,
        result: { success: true, elements: [{ tag: 'button', selector: '#btn', label: 'Button', visible: true }] }
      },
      { frameId: 1, result: { success: true, elements: [] } },
      { frameId: 2, result: null }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'list_interactive', selector: '' }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(res.calls[0].result.elements.length, 1, 'should handle empty/null frame results without error')
  })

  test('forwards selector as scope argument for list_interactive execution', async () => {
    executeScriptReturn.push([{ frameId: 0, result: { success: true, elements: [] } }])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'list_interactive', selector: '[role="dialog"]' }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(executeScriptCalls.length, 1)
    assert.deepStrictEqual(
      executeScriptCalls[0].args,
      ['[role="dialog"]'],
      'list_interactive should pass selector as scope argument to injected primitive'
    )
  })

  test('forwards scope_rect argument for list_interactive execution', async () => {
    executeScriptReturn.push([{ frameId: 0, result: { success: true, elements: [] } }])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({
        action: 'list_interactive',
        selector: '[role="dialog"]',
        scope_rect: { x: 300, y: 200, width: 400, height: 280 }
      }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(executeScriptCalls.length, 1)
    assert.deepStrictEqual(
      executeScriptCalls[0].args,
      ['[role="dialog"]', { scope_rect: { x: 300, y: 200, width: 400, height: 280 } }],
      'list_interactive should pass selector + scope_rect to injected primitive'
    )
  })

  test('returns error status when scoped list_interactive fails in all frames', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: false, error: 'scope_not_found', message: 'missing scope', elements: [] } },
      { frameId: 1, result: { success: false, error: 'scope_not_found', message: 'missing scope', elements: [] } }
    ])
    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'list_interactive', selector: '[role="dialog"]' }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(res.calls[0].status, 'error')
    assert.strictEqual(res.calls[0].error, 'scope_not_found')
    assert.strictEqual(res.calls[0].result.success, false)
  })
})

describe('iframe support: explicit frame targeting', () => {
  beforeEach(() => {
    executeScriptCalls = []
    executeScriptReturn = []
  })

  test('frame index resolves to frameIds target', async () => {
    // Probe call: only frameId 2 matches requested index
    executeScriptReturn.push([
      { frameId: 0, result: { matches: false } },
      { frameId: 2, result: { matches: true } }
    ])
    // Action call: executes only in matched frame
    executeScriptReturn.push([{
      frameId: 2,
      result: { success: true, action: 'click', selector: '#btn', matched: { tag: 'button', selector: '#btn' } }
    }])

    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'click', selector: '#btn', frame: 0 }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(executeScriptCalls.length, 2, 'should probe frames then execute in matched frame')
    assert.strictEqual(executeScriptCalls[0].target.allFrames, true, 'probe should run in all frames')
    assert.deepStrictEqual(executeScriptCalls[1].target.frameIds, [2], 'action should target matched frameId only')
    assert.strictEqual(res.calls[0].status, 'complete')
    assert.strictEqual(res.calls[0].result.frame_id, 2, 'result should include selected frame_id')
    assert.strictEqual(res.calls[0].result.matched.frame_id, 2, 'matched evidence should include selected frame_id')
  })

  test('frame selector returns frame_not_found when no frame matches', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { matches: false } },
      { frameId: 1, result: { matches: false } }
    ])

    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'click', selector: '#btn', frame: 'iframe[name="missing"]' }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(executeScriptCalls.length, 1, 'should stop after probe when no frame matches')
    assert.strictEqual(res.calls[0].status, 'error')
    assert.ok(res.calls[0].error.startsWith('frame_not_found'), `expected error to start with 'frame_not_found', got: ${res.calls[0].error}`)
  })

  test('frame="all" keeps allFrames execution without probe', async () => {
    executeScriptReturn.push([
      { frameId: 0, result: { success: false, action: 'click', selector: '#btn', error: 'element_not_found' } },
      { frameId: 1, result: { success: true, action: 'click', selector: '#btn', matched: { tag: 'button', selector: '#btn' } } }
    ])

    const res = captureAsyncResult()

    await executeDOMAction(
      makeQuery({ action: 'click', selector: '#btn', frame: 'all' }),
      1,
      makeSyncClient(),
      res.fn,
      noopToast
    )

    assert.strictEqual(executeScriptCalls.length, 1, 'should skip probe for frame=all')
    assert.strictEqual(executeScriptCalls[0].target.allFrames, true)
    assert.strictEqual(res.calls[0].status, 'complete')
  })
})

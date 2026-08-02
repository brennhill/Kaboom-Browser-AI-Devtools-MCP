// @ts-nocheck
/**
 * Purpose: Verifies waterfall bridge failures remain failures through background dispatch.
 */
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

const registered = new Map()
const debugLog = mock.fn()

mock.module('../../../extension/background/commands/registry.js', {
  namedExports: { registerCommand: mock.fn((name, handler) => registered.set(name, handler)) }
})
mock.module('../../../extension/background/caches/cache-limits.js', {
  namedExports: { canTakeScreenshot: mock.fn(), recordScreenshot: mock.fn() }
})
mock.module('../../../extension/background/runtime-state/settings-state.js', {
  namedExports: { getServerUrl: () => 'http://localhost:7890' }
})
mock.module('../../../extension/background/debug.js', {
  namedExports: { DebugCategory: { CAPTURE: 'capture' }, debugLog }
})

globalThis.chrome = {
  tabs: {
    get: mock.fn(async () => ({ id: 12, windowId: 2, url: 'https://example.test/' })),
    sendMessage: mock.fn()
  },
  scripting: { executeScript: mock.fn(async () => []) },
  windows: { update: mock.fn(async () => ({})) }
}

await import('../../../extension/background/commands/observe.js')

describe('observe waterfall bridge result', () => {
  beforeEach(() => mock.reset())

  test('preserves a structured content bridge failure', async () => {
    chrome.tabs.sendMessage.mock.mockImplementationOnce(async () => ({
      entries: [],
      error: 'waterfall_bridge_timeout',
      message: 'Injected waterfall bridge did not respond before the deadline.'
    }))
    const sendResult = mock.fn()

    await registered.get('waterfall')({ tabId: 12, query: { id: 'q-waterfall' }, sendResult })

    assert.deepStrictEqual(sendResult.mock.calls[0].arguments[0], {
      entries: [],
      error: 'waterfall_bridge_timeout',
      message: 'Injected waterfall bridge did not respond before the deadline.',
      page_url: 'https://example.test/',
      count: 0
    })
  })

  test('keeps a confirmed empty waterfall successful', async () => {
    chrome.tabs.sendMessage.mock.mockImplementationOnce(async () => ({ entries: [] }))
    const sendResult = mock.fn()

    await registered.get('waterfall')({ tabId: 12, query: { id: 'q-empty' }, sendResult })

    assert.deepStrictEqual(sendResult.mock.calls[0].arguments[0], {
      entries: [],
      page_url: 'https://example.test/',
      count: 0
    })
  })
})

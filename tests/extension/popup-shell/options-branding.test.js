// @ts-nocheck
/**
 * @fileoverview options-branding.test.js — Focused checks for operator-facing options diagnostics copy.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

function createMockDocument() {
  const elements = {}

  return {
    body: {
      classList: {
        add() {},
        remove() {},
        toggle() {},
        contains() {
          return false
        }
      }
    },
    getElementById(id) {
      if (!elements[id]) {
        elements[id] = {
          id,
          value: '',
          textContent: '',
          disabled: false,
          classList: {
            add() {},
            remove() {},
            toggle() {},
            contains() {
              return false
            }
          },
          style: {},
          addEventListener: mock.fn()
        }
      }
      return elements[id]
    },
    addEventListener: mock.fn()
  }
}

const mockChrome = {
  runtime: {
    sendMessage: mock.fn(),
    onMessage: { addListener: mock.fn() }
  },
  storage: {
    local: {
      get: mock.fn((keys, cb) => {
        if (typeof cb === 'function') {
          cb({})
          return undefined
        }
        return Promise.resolve({})
      }),
      set: mock.fn((data, cb) => cb && cb())
    }
  }
}

globalThis.chrome = mockChrome
globalThis.document = createMockDocument()

const { testConnection } = await import('../../../extension/options.js')

describe('options branding copy', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    globalThis.fetch = mock.fn()
  })

  test('timeout guidance points to kaboom-mcp', async () => {
    document.getElementById('server-url-input').value = 'http://127.0.0.1:7890'
    globalThis.fetch.mock.mockImplementation(() => Promise.reject(new Error('timeout')))

    await testConnection()

    const resultEl = document.getElementById('test-result')
    assert.match(resultEl.textContent, /Run: npx kaboom-mcp/)
  })

  test('404 guidance refers to Kaboom MCP', async () => {
    document.getElementById('server-url-input').value = 'http://127.0.0.1:7890'
    globalThis.fetch.mock.mockImplementation(() => Promise.resolve({
      ok: false,
      status: 404,
      statusText: 'Not Found'
    }))

    await testConnection()

    const resultEl = document.getElementById('test-result')
    assert.match(resultEl.textContent, /Is this KaBOOM! MCP v5\.8\.0\+\?/)
  })
})

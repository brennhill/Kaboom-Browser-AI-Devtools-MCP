// @ts-nocheck
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

function createMockElement(id) {
  return {
    id,
    textContent: '',
    style: {},
    disabled: false,
    title: '',
    onclick: null,
    addEventListener: mock.fn()
  }
}

function createMockDocument() {
  const elements = {}
  return {
    getElementById: mock.fn((id) => {
      if (!elements[id]) elements[id] = createMockElement(id)
      return elements[id]
    })
  }
}

describe('popup tab-tracking branding', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.document = createMockDocument()
    globalThis.chrome = {
      storage: {
        local: {
          get: mock.fn((keys, callback) => {
            callback?.({})
            return Promise.resolve({})
          })
        },
        onChanged: {
          addListener: mock.fn()
        }
      },
      tabs: {
        query: mock.fn((_query, callback) => callback([{ id: 7, url: 'https://dash.cloudflare.com', title: 'Cloudflare' }]))
      }
    }
  })

  test('cloaked-domain button title uses Kaboom copy', async () => {
    const { initTrackPageButton } = await import('../../extension/popup/tabs/tab-tracking.js')

    initTrackPageButton()
    await new Promise((resolve) => setTimeout(resolve, 0))

    const button = document.getElementById('track-page-btn')
    assert.strictEqual(button.disabled, true)
    assert.match(button.title, /KaBOOM! is disabled here/)
  })
})

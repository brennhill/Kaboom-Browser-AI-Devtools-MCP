// @ts-nocheck
/**
 * @fileoverview popup-update-wire-shape.test.js — Regression guard pinning the
 * exact wire key the popup reads from the daemon's /health endpoint. Per repo
 * policy (CLAUDE.md: "ALL JSON fields use snake_case"), the daemon emits
 * `available_version`. A prior regression saw popup code reading
 * `availableVersion` (camelCase); tests passed because fixtures matched the
 * popup, not the daemon, so the banner silently never rendered in production.
 *
 * These tests lock the contract: snake_case payload must SHOW the banner;
 * the camelCase shape (the regression shape) must NOT.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let importCounter = 0

function createMockElement(id) {
  const style = { display: id === 'update-available' ? 'none' : '' }
  const dataset = {}
  const listeners = new Map()
  return {
    id,
    textContent: '',
    innerHTML: '',
    style,
    dataset,
    disabled: false,
    addEventListener: mock.fn((event, handler) => {
      listeners.set(event, handler)
    })
  }
}

function createMockDocument() {
  const elements = {}
  return {
    getElementById: mock.fn((id) => {
      if (!elements[id]) elements[id] = createMockElement(id)
      return elements[id]
    }),
    _elements: elements
  }
}

async function importUpdateButton() {
  const mod = await import(`../../extension/popup/update-button.js?v=${++importCounter}`)
  return mod
}

describe('popup update-button /health wire shape', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.fetch = mock.fn()
    globalThis.document = createMockDocument()
    globalThis.chrome = {
      runtime: { id: 'test-ext-id', getManifest: () => ({ version: '0.8.5' }) },
      storage: {
        local: {
          get: mock.fn((_keys, cb) => {
            cb?.({})
            return Promise.resolve({})
          })
        }
      },
      tabs: {
        create: mock.fn(() => Promise.resolve({}))
      }
    }
  })

  test('snake_case available_version (what the daemon actually emits) SHOWS the banner', async () => {
    const { renderUpdateAvailableBanner } = await importUpdateButton()
    await renderUpdateAvailableBanner({ version: '0.8.5', available_version: '0.9.0' })

    const container = document.getElementById('update-available')
    assert.strictEqual(
      container.style.display,
      '',
      'banner must be visible when daemon reports available_version (snake_case, matches daemon wire shape)'
    )
  })

  test('camelCase availableVersion (the regression shape) does NOT show the banner', async () => {
    const { renderUpdateAvailableBanner } = await importUpdateButton()
    // Intentionally send the wrong (camelCase) shape — what the popup used to
    // read. The daemon does not emit this key, so the banner must stay hidden.
    await renderUpdateAvailableBanner({ version: '0.8.5', availableVersion: '0.9.0' })

    const container = document.getElementById('update-available')
    assert.strictEqual(
      container.style.display,
      'none',
      'banner must stay hidden on camelCase payload — this catches future drift away from snake_case wire shape'
    )
  })
})

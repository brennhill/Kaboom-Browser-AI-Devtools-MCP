// @ts-nocheck
/**
 * @fileoverview popup-audit-button.test.js — Tests for the tracked-state popup Audit CTA.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let storageState = {}
let importCounter = 0
let runtimeSendMessage

function createMockElement(id) {
  const hiddenByDefault = id === 'tracking-bar' || id === 'tracking-bar-audit' || id === 'no-tracking-warning'
  return {
    id,
    textContent: '',
    innerHTML: '',
    className: '',
    classList: {
      add: mock.fn(),
      remove: mock.fn(),
      toggle: mock.fn()
    },
    style: { display: hiddenByDefault ? 'none' : '' },
    addEventListener: mock.fn(),
    setAttribute: mock.fn(),
    getAttribute: mock.fn(),
    onclick: null,
    value: '',
    checked: false,
    disabled: false
  }
}

function createMockDocument() {
  const elements = {}
  return {
    getElementById: mock.fn((id) => {
      if (!elements[id]) elements[id] = createMockElement(id)
      return elements[id]
    }),
    addEventListener: mock.fn(),
    querySelector: mock.fn(),
    querySelectorAll: mock.fn(() => []),
    readyState: 'complete'
  }
}

describe('popup audit button', () => {
  beforeEach(() => {
    mock.reset()
    storageState = {
      trackedTabId: 7,
      trackedTabUrl: 'https://tracked.example/',
      trackedTabTitle: 'Tracked Example'
    }
    runtimeSendMessage = mock.fn((message) => Promise.resolve({ success: true, type: message?.type }))

    globalThis.document = createMockDocument()
    globalThis.chrome = {
      runtime: {
        id: 'test-extension-id',
        sendMessage: runtimeSendMessage,
        onMessage: { addListener: mock.fn() }
      },
      storage: {
        local: {
          get: mock.fn((_keys, callback) => {
            callback?.({ ...storageState })
            return Promise.resolve({ ...storageState })
          }),
          set: mock.fn((_data, callback) => {
            callback?.()
            return Promise.resolve()
          }),
          remove: mock.fn((_keys, callback) => {
            callback?.()
            return Promise.resolve()
          })
        },
        onChanged: {
          addListener: mock.fn()
        }
      },
      tabs: {
        query: mock.fn((_queryInfo, callback) => callback([{ id: 7, url: 'https://tracked.example/', title: 'Tracked Example' }])),
        sendMessage: mock.fn(() => Promise.resolve({ status: 'alive' })),
        update: mock.fn(() => Promise.resolve({ id: 7 })),
        get: mock.fn(() => Promise.resolve({ id: 7, windowId: 1 })),
        reload: mock.fn(() => Promise.resolve())
      },
      windows: {
        update: mock.fn(() => Promise.resolve())
      }
    }
  })

  // AUDIT_BUTTON_ENABLED in src/popup/tabs/tab-tracking.ts is currently false: the CTA
  // is hidden until the terminal side-panel path is fully verified. The helper it
  // would call is still covered by tests/extension/request-audit.test.js.
  // When the flag flips back to true, restore the shown-state assertions kept in
  // the sibling test below.
  test('keeps the Audit CTA hidden and inert while the feature flag is off', async () => {
    const auditButton = document.getElementById('tracking-bar-audit')
    assert.strictEqual(auditButton.style.display, 'none')

    const { initTrackPageButton } = await import(`../../extension/popup/tabs/tab-tracking.js?v=${++importCounter}`)
    initTrackPageButton()
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(auditButton.style.display, 'none', 'Audit CTA must stay hidden while disabled')
    assert.strictEqual(auditButton.onclick, null, 'disabled Audit CTA must not be clickable')

    // Nothing should have been dispatched on behalf of the audit workflow.
    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.ok(
      !sentTypes.includes('qa_scan_requested'),
      `disabled Audit CTA must not start the audit workflow (saw: ${sentTypes.join(', ')})`
    )
  })

  // Restore-guard: documents the exact contract to re-assert when the flag flips.
  // Skipped rather than deleted so the intended behavior isn't lost.
  test.skip('shows an Audit CTA while tracked and routes through the shared audit helper (re-enable with AUDIT_BUTTON_ENABLED)', async () => {
    const auditButton = document.getElementById('tracking-bar-audit')
    const { initTrackPageButton } = await import(`../../extension/popup/tabs/tab-tracking.js?v=${++importCounter}`)
    initTrackPageButton()
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(auditButton.textContent, 'Audit')
    assert.strictEqual(auditButton.style.display, 'inline-flex')

    await auditButton.onclick?.()

    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.deepStrictEqual(sentTypes, ['open_terminal_panel', 'qa_scan_requested'])
    assert.strictEqual(runtimeSendMessage.mock.calls[1].arguments[0].page_url, 'https://tracked.example/')
  })
})

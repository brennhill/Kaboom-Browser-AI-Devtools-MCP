// @ts-nocheck
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let storageState = {}
let storageChangeListener = null
let trackedTabExists = true
let continuityState = { phase: 'confirmed', is_tracked: true, tab_id: 7 }

const mockChrome = {
  runtime: {
    sendMessage: mock.fn((message) =>
      Promise.resolve(message?.type === 'get_tracking_state'
        ? { state: { continuity: continuityState } }
        : undefined)),
    onMessage: { addListener: mock.fn() }
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => {
        if (callback) {
          callback({ ...storageState })
          return
        }
        return Promise.resolve({ ...storageState })
      }),
      set: mock.fn((data, callback) => {
        storageState = { ...storageState, ...data }
        if (callback) callback()
        return Promise.resolve()
      }),
      remove: mock.fn((_keys, callback) => {
        if (callback) callback()
        return Promise.resolve()
      })
    },
    onChanged: {
      addListener: mock.fn((listener) => {
        storageChangeListener = listener
      })
    }
  },
  tabs: {
    query: mock.fn((_queryInfo, callback) => callback([{ id: 7, url: 'https://active/7', title: 'Active Tab' }])),
    sendMessage: mock.fn(() => Promise.resolve({ status: 'alive' })),
    update: mock.fn(() => Promise.resolve({ id: 7 })),
    get: mock.fn((tabId) => trackedTabExists
      ? Promise.resolve({ id: tabId, windowId: 1, title: 'Active Tab', url: 'https://active/7' })
      : Promise.reject(new Error('No tab with id'))),
    reload: mock.fn(() => Promise.resolve())
  },
  windows: {
    update: mock.fn(() => Promise.resolve())
  }
}

globalThis.chrome = mockChrome

function createMockElement(id) {
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
    style: {},
    addEventListener: mock.fn(),
    setAttribute: mock.fn(),
    getAttribute: mock.fn(),
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

describe('popup tab tracking sync', () => {
  beforeEach(() => {
    mock.reset()
    storageState = {}
    storageChangeListener = null
    trackedTabExists = true
    continuityState = { phase: 'confirmed', is_tracked: true, tab_id: 7 }
    globalThis.document = createMockDocument()
  })

  test('tracks storage trackedTabId changes while popup is open', async () => {
    const { initTrackPageButton } = await import('../../../extension/popup/tabs/tab-tracking.js')
    await initTrackPageButton()
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.ok(storageChangeListener, 'expected tab tracking module to subscribe to storage changes')

    storageState = {
      trackedTabId: 7,
      trackedTabUrl: 'https://active/7',
      trackedTabTitle: 'Active Tab'
    }
    storageChangeListener(
      {
        trackedTabId: { oldValue: null, newValue: 7 },
        trackedTabUrl: { oldValue: '', newValue: 'https://active/7' }
      },
      'local'
    )
    await new Promise((resolve) => setTimeout(resolve, 0))

    const trackingBar = document.getElementById('tracking-bar')
    const auditButton = document.getElementById('tracking-bar-audit')
    const warning = document.getElementById('no-tracking-warning')
    assert.strictEqual(trackingBar.style.display, 'flex')
    // Audit stays hidden while AUDIT_BUTTON_ENABLED is false (see
    // src/popup/tabs/tab-tracking.ts). The tracking bar itself must still appear.
    assert.strictEqual(auditButton.style.display, 'none')
    assert.strictEqual(warning.style.display, 'none')
  })

  test('shows stale tracked identity with one-click current-tab recovery', async () => {
    storageState = {
      trackedTabId: 91,
      trackedTabUrl: 'https://closed.example/work',
      trackedTabTitle: 'Closed workspace'
    }
    trackedTabExists = false

    const { initTrackPageButton } = await import('../../../extension/popup/tabs/tab-tracking.js')
    initTrackPageButton()
    await new Promise((resolve) => setTimeout(resolve, 0))

    const button = document.getElementById('track-page-btn')
    const warning = document.getElementById('no-tracking-warning')
    const staleIdentity = document.getElementById('stale-tracking-identity')
    assert.strictEqual(button.textContent, 'Track Current Tab')
    assert.strictEqual(button.disabled, false)
    assert.strictEqual(warning.style.display, 'block')
    assert.match(warning.textContent, /no longer available/i)
    assert.match(staleIdentity.textContent, /Closed workspace/)
    assert.match(staleIdentity.textContent, /closed\.example/)
  })

  test('renders the same tracked title and URL in the healthy identity bar', async () => {
    storageState = {
      trackedTabId: 7,
      trackedTabUrl: 'https://active/7',
      trackedTabTitle: 'Active Tab'
    }

    const { initTrackPageButton } = await import('../../../extension/popup/tabs/tab-tracking.js')
    initTrackPageButton()
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(document.getElementById('tracking-bar-title').textContent, 'Active Tab')
    assert.strictEqual(document.getElementById('tracking-bar-url').textContent, 'https://active/7')
  })

  test('shows continuity progress instead of transient no-tracking state', async () => {
    storageState = {
      trackedTabId: 7,
      trackedTabUrl: 'https://next.example/',
      trackedTabTitle: 'Active Tab'
    }
    continuityState = {
      tab_id: 7,
      phase: 'extension_reconnecting',
      is_tracked: true,
      provisional_url: 'https://next.example/'
    }

    const { initTrackPageButton } = await import('../../../extension/popup/tabs/tab-tracking.js')
    initTrackPageButton()
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.match(document.getElementById('tracking-bar-title').textContent, /Reconnecting/)
    assert.strictEqual(document.getElementById('no-tracking-warning').style.display, 'none')
  })
})

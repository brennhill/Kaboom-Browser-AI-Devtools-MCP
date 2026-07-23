// @ts-nocheck
/**
 * @fileoverview tracked-hover-launcher.test.js — Unit tests for tracked-tab quick actions launcher UI.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let elementsById
let storageData
let storageChangeListeners
let runtimeSendMessage
let runtimeOnMessageListeners
let setTrackedHoverLauncherEnabled
let sharedStorageKey
let sharedReshowMessageType
let terminalUiStateKey
let sessionStorageData
let importCounter = 0

function registerElement(el) {
  if (el && el.id) {
    elementsById[el.id] = el
  }
}

function createMockElement(tag) {
  const listeners = {}
  const el = {
    tag,
    id: '',
    type: '',
    title: '',
    textContent: '',
    className: '',
    disabled: false,
    href: '',
    target: '',
    rel: '',
    dataset: {},
    style: {},
    children: [],
    appendChild: mock.fn((child) => {
      el.children.push(child)
      registerElement(child)
      return child
    }),
    remove: mock.fn(() => {
      if (el.id) delete elementsById[el.id]
    }),
    addEventListener: mock.fn((type, handler) => {
      listeners[type] = handler
    }),
    setAttribute: mock.fn((name, value) => {
      el[name] = value
    }),
    dispatch(type) {
      const handler = listeners[type]
      if (handler) {
        handler({
          preventDefault() {},
          stopPropagation() {}
        })
      }
    }
  }
  return el
}

function findElementByTitle(element, title) {
  if (!element) return null
  if (element.title === title) return element
  for (const child of element.children || []) {
    const found = findElementByTitle(child, title)
    if (found) return found
  }
  return null
}

function findLinkByText(element, text) {
  if (!element) return null
  if (element.tag === 'a' && hasChildWithText(element, text)) return element
  for (const child of element.children || []) {
    const found = findLinkByText(child, text)
    if (found) return found
  }
  return null
}

function hasChildWithText(element, text) {
  if (element.textContent === text) return true
  for (const child of element.children || []) {
    if (child.textContent === text) return true
  }
  return false
}

function findElementByTitlePrefix(element, prefix) {
  if (!element) return null
  if (element.title && element.title.startsWith(prefix)) return element
  for (const child of element.children || []) {
    const found = findElementByTitlePrefix(child, prefix)
    if (found) return found
  }
  return null
}

function findElementWithChildText(element, text) {
  if (!element) return null
  if (hasChildWithText(element, text)) return element
  for (const child of element.children || []) {
    const found = findElementWithChildText(child, text)
    if (found) return found
  }
  return null
}

function dispatchRuntimeMessage(message) {
  for (const listener of runtimeOnMessageListeners) {
    listener(message, { id: 'test-extension-id' }, () => {})
  }
}

function emitStorageChange(areaName, key, oldValue, newValue) {
  for (const listener of storageChangeListeners) {
    listener({ [key]: { oldValue, newValue } }, areaName)
  }
}

function resetGlobals() {
  elementsById = {}
  storageData = { kaboom_recording: { active: false } }
  sessionStorageData = {}
  storageChangeListeners = []
  runtimeOnMessageListeners = []

  runtimeSendMessage = mock.fn((message, callback) => {
    if (message?.type === 'capture_screenshot') {
      callback?.({ success: true })
      return Promise.resolve({ success: true })
    }
    if (message?.type === 'open_terminal_panel') {
      callback?.({ success: true })
      return Promise.resolve({ success: true })
    }
    if (message?.type === 'record_start') {
      storageData.kaboom_recording = { active: true }
      callback?.({ status: 'recording' })
      return Promise.resolve({ status: 'recording' })
    }
    if (message?.type === 'record_stop') {
      storageData.kaboom_recording = { active: false }
      callback?.({ status: 'saved' })
      return Promise.resolve({ status: 'saved' })
    }
    callback?.({})
    return Promise.resolve({})
  })

  globalThis.chrome = {
    runtime: {
      id: 'test-extension-id',
      getURL: mock.fn((path) => `chrome-extension://test/${path}`),
      sendMessage: runtimeSendMessage,
      onMessage: {
        addListener: mock.fn((listener) => {
          runtimeOnMessageListeners.push(listener)
        }),
        removeListener: mock.fn((listener) => {
          runtimeOnMessageListeners = runtimeOnMessageListeners.filter((item) => item !== listener)
        })
      }
    },
    storage: {
      local: {
        get: mock.fn((_keys, callback) => {
          callback?.(storageData)
          return Promise.resolve(storageData)
        }),
        set: mock.fn((value, callback) => {
          storageData = { ...storageData, ...(value || {}) }
          callback?.()
          return Promise.resolve()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          for (const key of keyList) {
            delete storageData[key]
          }
          callback?.()
          return Promise.resolve()
        })
      },
      session: {
        get: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const result = {}
          for (const key of keyList) {
            result[key] = sessionStorageData[key]
          }
          callback?.(result)
          return Promise.resolve(result)
        }),
        set: mock.fn((value, callback) => {
          const prev = { ...sessionStorageData }
          sessionStorageData = { ...sessionStorageData, ...(value || {}) }
          for (const [key, nextValue] of Object.entries(value || {})) {
            emitStorageChange('session', key, prev[key], nextValue)
          }
          callback?.()
          return Promise.resolve()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const prev = { ...sessionStorageData }
          for (const key of keyList) {
            delete sessionStorageData[key]
            emitStorageChange('session', key, prev[key], undefined)
          }
          callback?.()
          return Promise.resolve()
        })
      },
      onChanged: {
        addListener: mock.fn((listener) => {
          storageChangeListeners.push(listener)
        }),
        removeListener: mock.fn((listener) => {
          storageChangeListeners = storageChangeListeners.filter((item) => item !== listener)
        })
      }
    }
  }

  globalThis.document = {
    getElementById: mock.fn((id) => elementsById[id] || null),
    createElement: mock.fn((tag) => createMockElement(tag)),
    createElementNS: mock.fn((_ns, tag) => createMockElement(tag)),
    readyState: 'complete',
    body: {
      appendChild: mock.fn((el) => {
        registerElement(el)
        return el
      })
    },
    documentElement: {
      appendChild: mock.fn((el) => {
        registerElement(el)
        return el
      })
    }
  }

  globalThis.window = {
    addEventListener: mock.fn(),
    removeEventListener: mock.fn()
  }

  globalThis.location = {
    href: 'https://example.com/',
    hostname: 'example.com'
  }
}

describe('tracked hover launcher', () => {
  beforeEach(async () => {
    mock.reset()
    resetGlobals()
    const constants = await import(`../../extension/lib/constants.js?v=${++importCounter}`)
    sharedStorageKey = constants.StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN
    terminalUiStateKey = constants.StorageKey.TERMINAL_UI_STATE
    sharedReshowMessageType = constants.RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER
    const bridgeModule = await import('../../extension/content/ui/terminal-panel-bridge.js')
    bridgeModule._terminalPanelBridgeForTests?.reset?.()
    ;({ setTrackedHoverLauncherEnabled } = await import(
      `../../extension/content/ui/tracked-hover-launcher.js?v=${++importCounter}`
    ))
    await setTrackedHoverLauncherEnabled(false)
  })

  test('mounts only when tracked is enabled', async () => {
    await setTrackedHoverLauncherEnabled(true)

    assert.ok(elementsById['kaboom-tracked-hover-launcher'], 'launcher root should be mounted')
    assert.ok(elementsById['kaboom-tracked-hover-toggle'], 'launcher toggle should exist')
    assert.ok(elementsById['kaboom-tracked-hover-panel'], 'launcher panel should exist')
  })

  test('hover island keeps the flame icon on hover', async () => {
    await setTrackedHoverLauncherEnabled(true)

    const toggle = elementsById['kaboom-tracked-hover-toggle']
    assert.ok(toggle, 'expected hover island toggle')
    const logo = toggle.children[0]
    assert.ok(logo, 'expected logo image inside hover island toggle')

    toggle.dispatch('mouseenter')
    assert.ok(String(logo.src || '').includes('icons/icon.svg'))

    toggle.dispatch('mouseleave')
    assert.ok(String(logo.src || '').includes('icons/icon.svg'))
  })

  test('untracked localhost pages do not mount the launcher', async () => {
    globalThis.location = {
      href: 'http://localhost:3000/',
      hostname: 'localhost'
    }

    await setTrackedHoverLauncherEnabled(false)

    assert.strictEqual(elementsById['kaboom-tracked-hover-launcher'], undefined)
    assert.strictEqual(elementsById['kaboom-tracked-hover-toggle'], undefined)
  })

  test('screenshot action sends captureScreenshot runtime message', async () => {
    await setTrackedHoverLauncherEnabled(true)

    const root = elementsById['kaboom-tracked-hover-launcher']
    const screenshotButton = findElementByTitlePrefix(root, 'Screenshot')
    assert.ok(screenshotButton, 'expected screenshot button')

    screenshotButton.dispatch('click')

    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.ok(sentTypes.includes('capture_screenshot'))
  })

  test('audit action uses Audit wording and opens the shared audit workflow', async () => {
    await setTrackedHoverLauncherEnabled(true)

    const root = elementsById['kaboom-tracked-hover-launcher']
    const auditButton = findElementByTitlePrefix(root, 'Audit')
    assert.ok(auditButton, 'expected audit button')
    assert.strictEqual(findElementByTitlePrefix(root, 'Find Problems'), null)

    auditButton.dispatch('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.deepStrictEqual(sentTypes.slice(-2), ['open_terminal_panel', 'qa_scan_requested'])
    assert.strictEqual(runtimeSendMessage.mock.calls.at(-1).arguments[0].page_url, 'https://example.com/')
  })

  test('stop recording button sends screen_recording_stop and is hidden by default', async () => {
    await setTrackedHoverLauncherEnabled(true)

    const root = elementsById['kaboom-tracked-hover-launcher']
    const stopButton = findElementByTitle(root, 'Stop recording')
    assert.ok(stopButton, 'expected stop recording button')
    assert.strictEqual(stopButton.style.display, 'none', 'stop button should be hidden when not recording')

    stopButton.dispatch('click')

    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.ok(sentTypes.includes('screen_recording_stop'))
  })

  test('annotate action warns with Kaboom copy when extension context is invalidated', async () => {
    await setTrackedHoverLauncherEnabled(true)
    const warn = mock.method(console, 'warn', () => {})
    globalThis.chrome.runtime.getURL = undefined

    const root = elementsById['kaboom-tracked-hover-launcher']
    const drawButton = findElementByTitlePrefix(root, 'Annotate the page')
    assert.ok(drawButton, 'expected annotate button')

    drawButton.dispatch('click')

    assert.strictEqual(warn.mock.calls.length, 1)
    const message = warn.mock.calls[0].arguments[0]
    assert.match(message, /KaBOOM!/)
    assert.doesNotMatch(message, /Gasoline|STRUM/)
  })

  test('annotate action warns with Kaboom copy when draw-mode module load fails', async () => {
    await setTrackedHoverLauncherEnabled(true)
    const warn = mock.method(console, 'warn', () => {})

    const root = elementsById['kaboom-tracked-hover-launcher']
    const drawButton = findElementByTitlePrefix(root, 'Annotate the page')
    assert.ok(drawButton, 'expected annotate button')

    drawButton.dispatch('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(warn.mock.calls.length, 1)
    const message = warn.mock.calls[0].arguments[0]
    assert.match(message, /KaBOOM!/)
    assert.match(message, /chrome:\/\/extensions/)
    assert.doesNotMatch(message, /Gasoline|STRUM/)
  })

  test('settings menu exposes docs and github links', async () => {
    await setTrackedHoverLauncherEnabled(true)

    const root = elementsById['kaboom-tracked-hover-launcher']
    const settingsButton = findElementByTitlePrefix(root, 'Settings')
    assert.ok(settingsButton, 'expected settings button')
    settingsButton.dispatch('click')

    const docsLink = findLinkByText(root, 'Docs')
    const repoLink = findLinkByText(root, 'GitHub Repository')

    assert.ok(docsLink, 'expected docs link')
    assert.ok(repoLink, 'expected repo link')
    assert.strictEqual(docsLink.href, 'https://gokaboom.dev/docs')
    assert.strictEqual(repoLink.href, 'https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP')
  })

  test('hide action removes launcher until popup show message arrives', async () => {
    await setTrackedHoverLauncherEnabled(true)

    const root = elementsById['kaboom-tracked-hover-launcher']
    const toggle = elementsById['kaboom-tracked-hover-toggle']
    const settingsButton = findElementByTitlePrefix(root, 'Settings')
    assert.ok(settingsButton)
    assert.strictEqual(toggle?.title, 'KaBOOM! quick actions')
    settingsButton.dispatch('click')

    const hideButton = findElementWithChildText(root, 'Hide KaBOOM! Devtool')
    assert.ok(hideButton, 'expected hide button')
    hideButton.dispatch('click')

    assert.strictEqual(elementsById['kaboom-tracked-hover-launcher'], undefined)
    assert.strictEqual(storageData[sharedStorageKey], true, 'hide state should persist in storage')

    dispatchRuntimeMessage({ type: sharedReshowMessageType })
    assert.ok(elementsById['kaboom-tracked-hover-launcher'], 'launcher should remount after popup signal')
    assert.strictEqual(storageData[sharedStorageKey], undefined, 'reshow should clear persisted hidden state')
  })

  test('terminal action opens the side panel', async () => {
    await setTrackedHoverLauncherEnabled(true)

    const root = elementsById['kaboom-tracked-hover-launcher']
    const terminalButton = findElementByTitlePrefix(root, 'Terminal')
    assert.ok(terminalButton, 'expected terminal button')

    terminalButton.dispatch('click')

    const sentTypes = runtimeSendMessage.mock.calls.map((call) => call.arguments[0]?.type)
    assert.ok(sentTypes.includes('open_terminal_panel'))
  })

  test('launcher returns when terminal session is minimized and hides only while panel is open', async () => {
    await setTrackedHoverLauncherEnabled(true)
    assert.ok(elementsById['kaboom-tracked-hover-launcher'], 'launcher should start mounted')

    await chrome.storage.session.set({ [terminalUiStateKey]: 'open' })
    await new Promise((resolve) => setTimeout(resolve, 10))
    assert.strictEqual(elementsById['kaboom-tracked-hover-launcher'], undefined, 'launcher should hide while the side panel is open')

    await chrome.storage.session.set({ [terminalUiStateKey]: 'minimized' })
    await new Promise((resolve) => setTimeout(resolve, 10))
    assert.ok(elementsById['kaboom-tracked-hover-launcher'], 'launcher should return when the side panel is minimized')
  })

  test('persisted hidden state suppresses launcher after module reload until popup signal', async () => {
    storageData[sharedStorageKey] = true
    ;({ setTrackedHoverLauncherEnabled } = await import(
      `../../extension/content/ui/tracked-hover-launcher.js?v=${++importCounter}`
    ))

    await setTrackedHoverLauncherEnabled(true)
    assert.strictEqual(elementsById['kaboom-tracked-hover-launcher'], undefined)

    dispatchRuntimeMessage({ type: sharedReshowMessageType })
    assert.ok(elementsById['kaboom-tracked-hover-launcher'], 'launcher should remount after popup signal')
  })

  test('unmount removes launcher and storage listener', async () => {
    await setTrackedHoverLauncherEnabled(true)
    assert.ok(elementsById['kaboom-tracked-hover-launcher'])
    const mountedListenerCount = storageChangeListeners.length
    assert.ok(mountedListenerCount > 0, 'storage listener should be installed while mounted')

    await setTrackedHoverLauncherEnabled(false)

    assert.strictEqual(elementsById['kaboom-tracked-hover-launcher'], undefined)
    assert.ok(storageChangeListeners.length < mountedListenerCount, 'launcher-specific storage listeners should be removed on unmount')
  })
})

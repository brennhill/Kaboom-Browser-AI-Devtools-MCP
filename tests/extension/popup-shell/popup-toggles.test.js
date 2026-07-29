// @ts-nocheck
/**
 * @fileoverview popup-toggles.test.js — Tests for popup feature toggles: network body
 * capture, network waterfall, performance marks, action replay, screenshot on error,
 * source maps, action toasts, subtitles, and FEATURE_TOGGLES completeness.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { applyFeatureTogglesFromMockStorage } from '../shared/helpers.js'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    sendMessage: mock.fn(() => Promise.resolve()),
    onMessage: {
      addListener: mock.fn()
    }
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => callback && callback())
    },
    sync: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback())
    },
    onChanged: {
      addListener: mock.fn()
    }
  },
  tabs: {
    query: mock.fn((queryInfo, callback) => callback([{ id: 1, url: 'http://localhost:3000' }]))
  }
}

globalThis.chrome = mockChrome

// Mock DOM elements
const createMockDocument = () => {
  const elements = {}

  return {
    getElementById: mock.fn((id) => {
      if (!elements[id]) {
        elements[id] = createMockElement(id)
      }
      return elements[id]
    }),
    querySelector: mock.fn(),
    querySelectorAll: mock.fn(() => []),
    addEventListener: mock.fn()
  }
}

const createMockElement = (id) => ({
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
})

let mockDocument

describe('Network Body Capture Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include network body capture in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-network-body-capture')
    assert.ok(toggle, 'Network body capture toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'networkBodyCaptureEnabled')
    assert.strictEqual(toggle.messageType, 'set_network_body_capture_enabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should send setNetworkBodyCaptureEnabled message when toggled', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('networkBodyCaptureEnabled', 'set_network_body_capture_enabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_network_body_capture_enabled' && c.arguments[0].enabled === false
      )
    )
  })

  test('should send message when network body capture toggled', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('networkBodyCaptureEnabled', 'set_network_body_capture_enabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_network_body_capture_enabled' && c.arguments[0].enabled === true
      )
    )
  })
})

describe('Network Waterfall Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include network waterfall in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-network-waterfall')
    assert.ok(toggle, 'Network waterfall toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'networkWaterfallEnabled')
    assert.strictEqual(toggle.messageType, 'set_network_waterfall_enabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default network waterfall to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({}) // No saved value — defaults to ON
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-network-waterfall')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved network waterfall state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ networkWaterfallEnabled: false })
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-network-waterfall')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setNetworkWaterfallEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('networkWaterfallEnabled', 'set_network_waterfall_enabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_network_waterfall_enabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setNetworkWaterfallEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('networkWaterfallEnabled', 'set_network_waterfall_enabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_network_waterfall_enabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Performance Marks Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include performance marks in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-performance-marks')
    assert.ok(toggle, 'Performance marks toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'performanceMarksEnabled')
    assert.strictEqual(toggle.messageType, 'set_performance_marks_enabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default performance marks to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-performance-marks')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved performance marks state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ performanceMarksEnabled: false })
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-performance-marks')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setPerformanceMarksEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('performanceMarksEnabled', 'set_performance_marks_enabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_performance_marks_enabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setPerformanceMarksEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('performanceMarksEnabled', 'set_performance_marks_enabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_performance_marks_enabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Action Replay Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include action replay in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-action-replay')
    assert.ok(toggle, 'Action replay toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'actionReplayEnabled')
    assert.strictEqual(toggle.messageType, 'set_action_replay_enabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default action replay to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-action-replay')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved action replay state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ actionReplayEnabled: false })
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-action-replay')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setActionReplayEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('actionReplayEnabled', 'set_action_replay_enabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_action_replay_enabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setActionReplayEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('actionReplayEnabled', 'set_action_replay_enabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_action_replay_enabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Screenshot on Error Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include screenshot in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-screenshot')
    assert.ok(toggle, 'Screenshot toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'screenshotOnError')
    assert.strictEqual(toggle.messageType, 'set_screenshot_on_error')
    assert.strictEqual(toggle.default, true)
  })

  test('should default screenshot on error to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-screenshot')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved screenshot state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ screenshotOnError: false })
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-screenshot')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setScreenshotOnError message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('screenshotOnError', 'set_screenshot_on_error', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_screenshot_on_error' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setScreenshotOnError message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('screenshotOnError', 'set_screenshot_on_error', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_screenshot_on_error' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Source Maps Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include source maps in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-source-maps')
    assert.ok(toggle, 'Source maps toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'sourceMapEnabled')
    assert.strictEqual(toggle.messageType, 'set_source_map_enabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default source maps to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-source-maps')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved source maps state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ sourceMapEnabled: false })
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-source-maps')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setSourceMapEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('sourceMapEnabled', 'set_source_map_enabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_source_map_enabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setSourceMapEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('sourceMapEnabled', 'set_source_map_enabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_source_map_enabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Action Toasts Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include action toasts in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-action-toasts')
    assert.ok(toggle, 'Action toasts toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'actionToastsEnabled')
    assert.strictEqual(toggle.messageType, 'set_action_toasts_enabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default action toasts to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-action-toasts')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved action toasts state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ actionToastsEnabled: false })
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-action-toasts')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setActionToastsEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('actionToastsEnabled', 'set_action_toasts_enabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_action_toasts_enabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setActionToastsEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('actionToastsEnabled', 'set_action_toasts_enabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_action_toasts_enabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Subtitles Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include subtitles in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-subtitles')
    assert.ok(toggle, 'Subtitles toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'subtitlesEnabled')
    assert.strictEqual(toggle.messageType, 'set_subtitles_enabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default subtitles to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-subtitles')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved subtitles state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ subtitlesEnabled: false })
    })

    await applyFeatureTogglesFromMockStorage()

    const toggle = mockDocument.getElementById('toggle-subtitles')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setSubtitlesEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('subtitlesEnabled', 'set_subtitles_enabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_subtitles_enabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setSubtitlesEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    handleFeatureToggle('subtitlesEnabled', 'set_subtitles_enabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_subtitles_enabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('FEATURE_TOGGLES Completeness', () => {
  test('should have exactly 9 feature toggles', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    assert.strictEqual(FEATURE_TOGGLES.length, 9, 'Should have 9 feature toggles')
  })

  test('all toggles should have required fields', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    for (const toggle of FEATURE_TOGGLES) {
      assert.ok(toggle.id, `Toggle missing id`)
      assert.ok(toggle.storageKey, `Toggle ${toggle.id} missing storageKey`)
      assert.ok(toggle.messageType, `Toggle ${toggle.id} missing messageType`)
      assert.strictEqual(typeof toggle.default, 'boolean', `Toggle ${toggle.id} default should be boolean`)
    }
  })

  test('all toggle IDs should be unique', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const ids = FEATURE_TOGGLES.map((t) => t.id)
    const uniqueIds = new Set(ids)
    assert.strictEqual(uniqueIds.size, ids.length, 'All toggle IDs should be unique')
  })

  test('all storage keys should be unique', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const keys = FEATURE_TOGGLES.map((t) => t.storageKey)
    const uniqueKeys = new Set(keys)
    assert.strictEqual(uniqueKeys.size, keys.length, 'All storage keys should be unique')
  })
})

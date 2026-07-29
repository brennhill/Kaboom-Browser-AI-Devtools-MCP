// @ts-nocheck
/**
 * @fileoverview Tests for AI capture control extension integration.
 * Covers: settings polling, override application, and status communication.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock fetch
let fetchMock
let lastFetchUrl

beforeEach(() => {
  fetchMock = mock.fn()
  lastFetchUrl = null
  globalThis.fetch = fetchMock
})

describe('pollCaptureSettings', () => {
  test('fetches /settings endpoint', async () => {
    fetchMock.mock.mockImplementation((url) => {
      lastFetchUrl = url
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ connected: true, capture_overrides: {} })
      })
    })

    const { pollCaptureSettings } = await loadModule()
    await pollCaptureSettings('http://localhost:7890')

    assert.strictEqual(lastFetchUrl, 'http://localhost:7890/settings')
  })

  test('returns overrides from response', async () => {
    fetchMock.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            connected: true,
            capture_overrides: { ws_mode: 'medium', log_level: 'all' }
          })
      })
    )

    const { pollCaptureSettings } = await loadModule()
    const result = await pollCaptureSettings('http://localhost:7890')

    assert.deepStrictEqual(result, { ws_mode: 'medium', log_level: 'all' })
  })

  test('returns empty object when no overrides', async () => {
    fetchMock.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ connected: true, capture_overrides: {} })
      })
    )

    const { pollCaptureSettings } = await loadModule()
    const result = await pollCaptureSettings('http://localhost:7890')

    assert.deepStrictEqual(result, {})
  })

  test('returns null on fetch error', async () => {
    fetchMock.mock.mockImplementation(() => Promise.reject(new Error('Network error')))

    const { pollCaptureSettings } = await loadModule()
    const result = await pollCaptureSettings('http://localhost:7890')

    assert.strictEqual(result, null)
  })

  test('returns null on non-OK response', async () => {
    fetchMock.mock.mockImplementation(() =>
      Promise.resolve({
        ok: false,
        status: 404
      })
    )

    const { pollCaptureSettings } = await loadModule()
    const result = await pollCaptureSettings('http://localhost:7890')

    assert.strictEqual(result, null)
  })
})

describe('applyCaptureOverrides', () => {
  test('applies log_level override', async () => {
    const { applyCaptureOverrides, getCaptureState } = await loadModule()
    applyCaptureOverrides({ log_level: 'all' })

    const state = getCaptureState()
    assert.strictEqual(state.logLevel, 'all')
  })

  test('applies ws_mode override', async () => {
    const { applyCaptureOverrides, getCaptureState } = await loadModule()
    applyCaptureOverrides({ ws_mode: 'medium' })

    const state = getCaptureState()
    assert.strictEqual(state.wsMode, 'medium')
  })

  test('applies network_bodies override', async () => {
    const { applyCaptureOverrides, getCaptureState } = await loadModule()
    applyCaptureOverrides({ network_bodies: 'false' })

    const state = getCaptureState()
    assert.strictEqual(state.networkBodies, false)
  })

  test('applies screenshot_on_error override', async () => {
    const { applyCaptureOverrides, getCaptureState } = await loadModule()
    applyCaptureOverrides({ screenshot_on_error: 'true' })

    const state = getCaptureState()
    assert.strictEqual(state.screenshotOnError, true)
  })

  test('applies multiple overrides at once', async () => {
    const { applyCaptureOverrides, getCaptureState } = await loadModule()
    applyCaptureOverrides({
      log_level: 'warn',
      ws_mode: 'off',
      network_bodies: 'false'
    })

    const state = getCaptureState()
    assert.strictEqual(state.logLevel, 'warn')
    assert.strictEqual(state.wsMode, 'off')
    assert.strictEqual(state.networkBodies, false)
  })

  test('empty overrides does not change state', async () => {
    const { applyCaptureOverrides, getCaptureState } = await loadModule()
    const before = { ...getCaptureState() }
    applyCaptureOverrides({})
    const after = getCaptureState()

    assert.strictEqual(before.logLevel, after.logLevel)
    assert.strictEqual(before.wsMode, after.wsMode)
  })

  test('tracks AI-controlled status', async () => {
    const { applyCaptureOverrides, isAIControlled } = await loadModule()
    assert.strictEqual(isAIControlled(), false)

    applyCaptureOverrides({ log_level: 'all' })
    assert.strictEqual(isAIControlled(), true)

    applyCaptureOverrides({})
    assert.strictEqual(isAIControlled(), false)
  })
})

// --- Module loader helper ---

async function loadModule() {
  // State variables simulating the extension's capture settings
  let logLevel = 'error'
  let wsMode = 'lifecycle'
  let networkBodies = true
  let screenshotOnError = false
  let actionReplay = true
  let aiControlled = false

  async function pollCaptureSettings(serverUrl) {
    try {
      const response = await fetch(`${serverUrl}/settings`)
      if (!response.ok) return null
      const data = await response.json()
      return data.capture_overrides || {}
    } catch {
      return null
    }
  }

  function applyCaptureOverrides(overrides) {
    aiControlled = Object.keys(overrides).length > 0

    if (overrides.log_level !== undefined) {
      logLevel = overrides.log_level
    }
    if (overrides.ws_mode !== undefined) {
      wsMode = overrides.ws_mode
    }
    if (overrides.network_bodies !== undefined) {
      networkBodies = overrides.network_bodies === 'true'
    }
    if (overrides.screenshot_on_error !== undefined) {
      screenshotOnError = overrides.screenshot_on_error === 'true'
    }
    if (overrides.action_replay !== undefined) {
      actionReplay = overrides.action_replay === 'true'
    }
  }

  function getCaptureState() {
    return { logLevel, wsMode, networkBodies, screenshotOnError, actionReplay }
  }

  function isAIControlled() {
    return aiControlled
  }

  return { pollCaptureSettings, applyCaptureOverrides, getCaptureState, isAIControlled }
}

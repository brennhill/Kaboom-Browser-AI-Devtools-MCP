// @ts-nocheck
/**
 * @fileoverview performance-trace.test.js — Deterministic CDP CPU trace lifecycle tests.
 */

import { describe, test, mock } from 'node:test'
import assert from 'node:assert/strict'
import {
  createPerformanceTraceController,
  isTargetNotDebuggableError
} from '../../../extension/background/dom/cdp/performance-trace.js'

function fixture(completionParams = {}, failMethod = '') {
  let eventListener
  let detachListener
  const requests = []
  const debuggerApi = {
    attach: mock.fn(async () => {
      if (failMethod === 'Debugger.attach') throw new Error('target rejected')
    }),
    detach: mock.fn(async () => undefined),
    sendCommand: mock.fn(async (_target, method) => {
      if (method === failMethod) throw new Error('target rejected')
      if (method === 'Tracing.end') {
        queueMicrotask(() => eventListener({ tabId: 41 }, 'Tracing.tracingComplete', completionParams))
      }
      if (method === 'Page.getFrameTree') {
        return { frameTree: { frame: { url: 'https://app.test/design', loaderId: 'nav-123' } } }
      }
      if (method === 'Runtime.evaluate') {
        return { result: { value: 'build-abc123' } }
      }
      if (method === 'Page.reload') {
        queueMicrotask(() =>
          eventListener(
            { tabId: 41 },
            'Page.frameNavigated',
            { frame: { url: 'https://app.test/design', loaderId: 'nav-reloaded' } }
          )
        )
      }
      return {}
    }),
    onEvent: { addListener: (listener) => (eventListener = listener) },
    onDetach: { addListener: (listener) => (detachListener = listener) }
  }
  const postJSON = mock.fn(async (path, payload) => {
    requests.push({ path, payload })
    if (path === '/performance-trace/start') return { trace_id: 'trace-1', recovered: true }
    if (path === '/performance-trace/finish') {
      return {
        trace_id: 'trace-1',
        artifact_path: '/tmp/cpu-trace.json',
        event_count: 2,
        chunk_count: 1,
        bytes: 128,
        tab_id: payload.tab_id,
        url: payload.url,
        navigation_id: payload.navigation_id,
        build_sha: payload.build_sha
      }
    }
    return { accepted: true }
  })
  const controller = createPerformanceTraceController({ debuggerApi, postJSON, completionTimeoutMs: 100 })
  return { controller, debuggerApi, postJSON, requests, emit: (...args) => eventListener(...args), detach: (...args) => detachListener(...args) }
}

describe('Chrome performance trace controller', () => {
  test('captures full trace events and returns a local importable artifact', async () => {
    const f = fixture()
    const started = await f.controller.start(41)
    assert.deepEqual(started, {
      status: 'recording',
      trace_id: 'trace-1',
      tab_id: 41,
      url: 'https://app.test/design',
      navigation_id: 'nav-123',
      build_sha: 'build-abc123',
      cache: 'warm',
      reloaded: false
      ,recovered: true
    })

    f.emit({ tabId: 41 }, 'Tracing.dataCollected', {
      value: [
        { name: 'RunTask', ph: 'X', ts: 1 },
        { name: 'FunctionCall', ph: 'X', ts: 2 }
      ]
    })
    const finished = await f.controller.stop(41)

    assert.equal(f.debuggerApi.attach.mock.calls.length, 1)
    assert.equal(f.debuggerApi.detach.mock.calls.length, 1)
    assert.equal(finished.artifact_path, '/tmp/cpu-trace.json')
    assert.equal(finished.tab_id, 41)
    assert.equal(finished.url, 'https://app.test/design')
    assert.equal(finished.navigation_id, 'nav-123')
    assert.equal(finished.build_sha, 'build-abc123')
    assert.equal(finished.import_with, 'Chrome DevTools Performance panel or https://ui.perfetto.dev')
    const chunk = f.requests.find((request) => request.path === '/performance-trace/chunk')
    assert.equal(chunk.payload.sequence, 0)
    assert.equal(chunk.payload.events.length, 2)
  })

  test('targets a background tab and applies cold-cache reload after tracing starts', async () => {
    const f = fixture()
    const started = await f.controller.start(41, { reload: true, cache: 'cold' })
    assert.equal(started.cache, 'cold')
    assert.equal(started.reloaded, true)

    const methods = f.debuggerApi.sendCommand.mock.calls.map((call) => call.arguments[1])
    assert.ok(methods.indexOf('Tracing.start') < methods.indexOf('Network.clearBrowserCache'))
    assert.ok(methods.indexOf('Network.clearBrowserCache') < methods.indexOf('Page.reload'))
    assert.ok(methods.includes('Network.setCacheDisabled'))
    assert.equal(f.debuggerApi.attach.mock.calls[0].arguments[0].tabId, 41)
  })

  test('rejects concurrent starts and wrong-tab stops', async () => {
    const f = fixture()
    await f.controller.start(41)
    await assert.rejects(() => f.controller.start(41), /already active/)
    await assert.rejects(() => f.controller.stop(42), /tracked tab changed/)
  })

  test('aborts local artifact and reports debugger detachment', async () => {
    const f = fixture()
    await f.controller.start(41)
    f.detach({ tabId: 41 }, 'target_closed')
    await assert.rejects(() => f.controller.stop(41), /detached/)
    assert.ok(f.requests.some((request) => request.path === '/performance-trace/abort'))
    assert.equal(f.debuggerApi.detach.mock.calls.length, 0, 'must not detach an already-detached debugger target')
  })

  test('surfaces trace data loss reported by Chrome', async () => {
    const f = fixture({ dataLossOccurred: true })
    await f.controller.start(41)
    await assert.rejects(() => f.controller.stop(41), /lost trace data/)
  })

  test('identifies the exact startup stage when Chrome rejects a debugger command', async () => {
    for (const stage of ['Debugger.attach', 'Page.enable', 'Network.enable', 'Network.setCacheDisabled', 'Tracing.start', 'Page.getFrameTree', 'Runtime.evaluate']) {
      const f = fixture({}, stage)
      await assert.rejects(() => f.controller.start(41), new RegExp(`${stage} failed: target rejected`))
      assert.ok(f.requests.some((request) => request.path === '/performance-trace/abort'))
    }
  })
})

// Chrome refuses to attach the debugger to a target the extension may not access.
// The refusal is a property of the target, not of tracing, so it must be
// distinguishable from a genuine tracing fault before the caller retries.
describe('debugger target access classification', () => {
  test('recognizes every Chrome refusal that names an inaccessible target', () => {
    for (const message of [
      'Debugger.attach failed: Cannot access a chrome-extension:// URL of different extension',
      'Debugger.attach failed: Cannot access a chrome:// URL',
      'Debugger.attach failed: Cannot attach to this target.',
      'Debugger.attach failed: Cannot access contents of the page'
    ]) {
      assert.equal(isTargetNotDebuggableError(new Error(message)), true, message)
    }
  })

  test('does not misclassify genuine tracing faults as target refusals', () => {
    for (const message of [
      'Tracing.start failed: target rejected',
      'Debugger.attach failed: Another debugger is already attached to the tab with id: 41',
      'Chrome lost trace data before the CPU flamechart completed',
      'performance trace daemon returned an invalid start response'
    ]) {
      assert.equal(isTargetNotDebuggableError(new Error(message)), false, message)
    }
  })
})

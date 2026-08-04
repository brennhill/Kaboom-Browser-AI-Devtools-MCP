// @ts-nocheck
/**
 * @fileoverview performance-trace.test.js — Deterministic CDP CPU trace lifecycle tests.
 */

import { describe, test, mock } from 'node:test'
import assert from 'node:assert/strict'
import { createPerformanceTraceController } from '../../../extension/background/dom/cdp/performance-trace.js'

function fixture(completionParams = {}) {
  let eventListener
  let detachListener
  const requests = []
  const debuggerApi = {
    attach: mock.fn(async () => undefined),
    detach: mock.fn(async () => undefined),
    sendCommand: mock.fn(async (_target, method) => {
      if (method === 'Tracing.end') {
        queueMicrotask(() => eventListener({ tabId: 41 }, 'Tracing.tracingComplete', completionParams))
      }
      return {}
    }),
    onEvent: { addListener: (listener) => (eventListener = listener) },
    onDetach: { addListener: (listener) => (detachListener = listener) }
  }
  const postJSON = mock.fn(async (path, payload) => {
    requests.push({ path, payload })
    if (path === '/performance-trace/start') return { trace_id: 'trace-1' }
    if (path === '/performance-trace/finish') {
      return {
        trace_id: 'trace-1',
        artifact_path: '/tmp/cpu-trace.json',
        event_count: 2,
        chunk_count: 1,
        bytes: 128
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
    assert.deepEqual(started, { status: 'recording', trace_id: 'trace-1', tab_id: 41 })

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
    assert.equal(finished.import_with, 'Chrome DevTools Performance panel or https://ui.perfetto.dev')
    const chunk = f.requests.find((request) => request.path === '/performance-trace/chunk')
    assert.equal(chunk.payload.sequence, 0)
    assert.equal(chunk.payload.events.length, 2)
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
})

// content-readiness.test.js — Verifies correlated bounded post-navigation readiness.
import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  ContentReadinessBarrier,
  requiresContentReadiness
} from '../../../extension/background/runtime-state/content-readiness.js'

test('only content-script commands are gated', () => {
  for (const command of ['dom', 'a11y', 'dom_action', 'upload', 'get_markdown']) {
    assert.equal(requiresContentReadiness(command), true, command)
  }
  for (const command of ['browser_action', 'tabs', 'screenshot', 'screen_recording_start']) {
    assert.equal(requiresContentReadiness(command), false, command)
  }
})

test('matching acknowledgement releases the first command without delay', async () => {
  const calls = []
  const barrier = new ContentReadinessBarrier({
    probe: async (tabId, correlationId) => {
      calls.push([tabId, correlationId])
      return { ready: true, correlation_id: correlationId, connection_generation: 0 }
    },
    wait: async () => {
      throw new Error('matching first acknowledgement must not back off')
    }
  })
  barrier.begin(7, 'nav-1')

  assert.deepEqual(await barrier.waitUntilReady(7), {
    ready: true,
    correlation_id: 'nav-1',
    attempts: 1
  })
  assert.deepEqual(calls, [[7, 'nav-1']])
})

test('stale acknowledgement cannot release a newer navigation', async () => {
  const waits = []
  let attempts = 0
  const barrier = new ContentReadinessBarrier({
    probe: async (_tabId, correlationId) => {
      attempts += 1
      return attempts === 1
        ? { ready: true, correlation_id: 'nav-old', connection_generation: 0 }
        : { ready: true, correlation_id: correlationId, connection_generation: 0 }
    },
    wait: async (delayMs) => waits.push(delayMs),
    delays_ms: [10, 20]
  })
  barrier.begin(7, 'nav-new')

  const result = await barrier.waitUntilReady(7)
  assert.equal(result.ready, true)
  assert.equal(result.correlation_id, 'nav-new')
  assert.equal(result.attempts, 2)
  assert.deepEqual(waits, [10])
})

test('retry schedule is bounded and deterministic', async () => {
  const waits = []
  const timeouts = []
  const barrier = new ContentReadinessBarrier({
    probe: async () => undefined,
    wait: async (delayMs) => waits.push(delayMs),
    delays_ms: [5, 15, 45],
    onTimeout: (tabId, correlationId, attempts) => timeouts.push([tabId, correlationId, attempts])
  })
  barrier.begin(7, 'nav-timeout')

  assert.deepEqual(await barrier.waitUntilReady(7), {
    ready: false,
    correlation_id: 'nav-timeout',
    attempts: 4,
    error: 'content_readiness_timeout'
  })
  assert.deepEqual(waits, [5, 15, 45])
  assert.deepEqual(timeouts, [[7, 'nav-timeout', 4]])
})

test('superseded waiter cannot affect the newer navigation', async () => {
  let releaseFirst
  const barrier = new ContentReadinessBarrier({
    probe: async (_tabId, correlationId) => {
      if (correlationId === 'nav-old') {
        await new Promise((resolve) => {
          releaseFirst = resolve
        })
      }
      return { ready: true, correlation_id: correlationId, connection_generation: 0 }
    },
    wait: async () => undefined
  })
  barrier.begin(7, 'nav-old')
  const oldWait = barrier.waitUntilReady(7)
  await Promise.resolve()
  barrier.begin(7, 'nav-new')
  releaseFirst()

  assert.equal((await oldWait).error, 'readiness_superseded')
  assert.equal((await barrier.waitUntilReady(7)).correlation_id, 'nav-new')
})

test('daemon generation handoff rejects an in-flight readiness acknowledgement', async () => {
  let generation = 1
  let releaseProbe
  const superseded = []
  const barrier = new ContentReadinessBarrier({
    get_generation: () => generation,
    probe: async (_tabId, correlationId, connectionGeneration) => {
      await new Promise((resolve) => {
        releaseProbe = resolve
      })
      return { ready: true, correlation_id: correlationId, connection_generation: connectionGeneration }
    },
    wait: async () => undefined,
    onSuperseded: (tabId, correlationId, expectedGeneration, currentGeneration) =>
      superseded.push([tabId, correlationId, expectedGeneration, currentGeneration])
  })
  barrier.begin(7, 'nav-generation-1')
  const readiness = barrier.waitUntilReady(7)
  await Promise.resolve()

  generation = 2
  releaseProbe()

  assert.deepEqual(await readiness, {
    ready: false,
    correlation_id: 'nav-generation-1',
    attempts: 1,
    error: 'readiness_superseded'
  })
  assert.deepEqual(superseded, [[7, 'nav-generation-1', 1, 2]])
})

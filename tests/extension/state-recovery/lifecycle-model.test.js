// lifecycle-model.test.js — Seeded daemon-generation and Doctor obligation model.
import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  isConnectionGenerationCurrent,
  setConnectionGeneration
} from '../../../extension/background/runtime-state/connection-generation.js'
import {
  reportStateRecovery,
  resolveStateRecovery
} from '../../../extension/background/runtime-state/state-recovery.js'
import {
  clearExtensionLogsForTesting,
  getExtensionLogQueueSnapshot,
  initializeExtensionLogQueue
} from '../../../extension/background/runtime-state/log-queue.js'

function nextRandom(state) {
  return (Math.imul(state, 1664525) + 1013904223) >>> 0
}

test('seeded lifecycle sequences preserve state or a visible recovery obligation', async () => {
  for (let seed = 1; seed <= 100; seed++) {
    clearExtensionLogsForTesting()
    await initializeExtensionLogQueue({ read: async () => undefined, write: async () => undefined })
    let random = seed
    let generation = 1
    const obligations = new Map()
    setConnectionGeneration(generation)

    for (let step = 0; step < 200; step++) {
      random = nextRandom(random)
      const id = `model_state_${random % 12}`
      const action = random % 5
      if (action === 0) {
        obligations.set(id, { generation, restoring: false })
        reportStateRecovery({ name: id, detail: 'Model state requires recovery.', fix: 'Replay the emitted seed.' })
      } else if (action === 1) {
        generation++
        setConnectionGeneration(generation)
      } else if (action === 2) {
        const obligation = obligations.get(id)
        if (obligation && isConnectionGenerationCurrent(obligation.generation)) obligation.restoring = true
      } else if (action === 3) {
        const obligation = obligations.get(id)
        if (obligation?.restoring && isConnectionGenerationCurrent(obligation.generation)) {
          resolveStateRecovery(id)
          obligations.delete(id)
        }
      } else if (obligations.has(id)) {
        reportStateRecovery({ name: id, detail: 'Recovery was canceled.', fix: 'Retry recovery.' })
      }

      const logs = getExtensionLogQueueSnapshot()
      for (const [name] of obligations) {
        const visible = logs.some(
          (entry) => entry.category === 'state_recovery' && entry.data?.name === name && entry.data?.lifecycle === 'active'
        )
        assert.ok(visible, `seed=${seed} step=${step}: ${name} lost Doctor evidence`)
      }
    }
  }
})

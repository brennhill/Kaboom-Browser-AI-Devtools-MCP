// validated-storage.test.js — Verifies safe extension storage fallbacks.
import assert from 'node:assert/strict'
import { afterEach, test } from 'node:test'

const originalChrome = globalThis.chrome
afterEach(() => {
  globalThis.chrome = originalChrome
})

async function loadValidated() {
  return import(`../../../extension/lib/storage/validated.js?test=${Date.now()}-${Math.random()}`)
}

function installLocalRead(value) {
  globalThis.chrome = {
    runtime: { sendMessage: async () => undefined },
    storage: {
      local: {
        get(_key, callback) {
          callback({ state: value })
        }
      }
    }
  }
}

test('valid persisted state is returned unchanged', async () => {
  installLocalRead(true)
  const { readLocalState } = await loadValidated()
  const recovered = []
  const resolved = []
  const value = await readLocalState({
    key: 'state',
    fallback: false,
    validate: (candidate) => typeof candidate === 'boolean',
    diagnostic: { name: 'state', detail: 'fallback', fix: 'reset' },
    report: (diagnostic) => recovered.push(diagnostic),
    resolve: (name) => resolved.push(name)
  })
  assert.equal(value, true)
  assert.deepEqual(recovered, [])
  assert.deepEqual(resolved, ['state'])
})

test('malformed persisted state uses fallback and reports redacted diagnostic', async () => {
  installLocalRead({ token: 'secret' })
  const { readLocalState } = await loadValidated()
  const recovered = []
  const value = await readLocalState({
    key: 'state',
    fallback: false,
    validate: (candidate) => typeof candidate === 'boolean',
    diagnostic: { name: 'state', detail: 'fallback used', fix: 'reset' },
    report: (diagnostic) => recovered.push(diagnostic)
  })
  assert.equal(value, false)
  assert.deepEqual(recovered, [
    { name: 'state', detail: 'Extension state corruption failure; fallback used', fix: 'reset' }
  ])
  assert.doesNotMatch(JSON.stringify(recovered), /secret/)
})

test('storage API failure uses the same fallback and diagnostic path', async () => {
  globalThis.chrome = {
    runtime: { sendMessage: async () => undefined },
    storage: {
      local: {
        get() {
          throw new Error('private filesystem path')
        }
      }
    }
  }
  const { readLocalState } = await loadValidated()
  const recovered = []
  const value = await readLocalState({
    key: 'state',
    fallback: 'default',
    validate: (candidate) => typeof candidate === 'string',
    diagnostic: { name: 'state', detail: 'fallback used', fix: 'reset' },
    report: (diagnostic) => recovered.push(diagnostic)
  })
  assert.equal(value, 'default')
  assert.equal(recovered.length, 1)
  assert.equal(recovered[0].detail, 'Extension state read failure; fallback used')
  assert.doesNotMatch(JSON.stringify(recovered), /private filesystem path/)
})

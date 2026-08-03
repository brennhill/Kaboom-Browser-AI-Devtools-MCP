// install-identity-faults.test.js — Canonical extension daemon-identity recovery tests.
import assert from 'node:assert/strict'
import { mock, test } from 'node:test'

import { createStorageFaultScenario } from '../state-recovery/storage-fault-fixture.js'

function installChrome(readValue, writeError) {
  const transitions = []
  globalThis.chrome = {
    runtime: {
      sendMessage: mock.fn((message) => {
        transitions.push(message)
        return Promise.resolve()
      })
    },
    storage: {
      local: {
        get: mock.fn(() =>
          readValue instanceof Error
            ? Promise.reject(readValue)
            : Promise.resolve({ kaboom_server_install_id: readValue })
        ),
        set: mock.fn(() => (writeError ? Promise.reject(writeError) : Promise.resolve())),
        remove: mock.fn(() => Promise.resolve())
      },
      session: { set: mock.fn(() => Promise.resolve()) }
    }
  }
  return transitions
}

async function loadIdentity() {
  return import(`../../../extension/background/sync/install-identity.js?fault=${Date.now()}-${Math.random()}`)
}

test('daemon identity read faults suppress an unverified cached identity', async () => {
  for (const kind of ['read', 'quota', 'cancellation']) {
    const scenario = createStorageFaultScenario(kind, 'private-install-identity')
    installChrome(scenario.error)
    const identity = await loadIdentity()

    await identity.loadServerInstallId()
    assert.equal(identity.getServerInstallId(), undefined)
  }
})

test('corrupt and partial daemon identities are rejected', async () => {
  for (const kind of ['corruption', 'partial_write']) {
    const scenario = createStorageFaultScenario(kind, 'private-install-identity')
    installChrome(scenario.storedValue({ id: 'aabbccddeeff', extra: true }))
    const identity = await loadIdentity()

    await identity.loadServerInstallId()
    assert.equal(identity.getServerInstallId(), undefined)
  }
})

test('write faults retain the live identity and publish a classified redacted transition', async () => {
  for (const kind of ['write', 'quota', 'cancellation']) {
    const scenario = createStorageFaultScenario(kind, 'private-install-identity')
    const transitions = installChrome(undefined, scenario.error)
    const identity = await loadIdentity()

    identity.updateServerInstallId('aabbccddeeff')
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.equal(identity.getServerInstallId(), 'aabbccddeeff')
    assert.ok(transitions.some((entry) => entry.diagnostic?.detail.includes(`${kind} failure`)))
    assert.doesNotMatch(JSON.stringify(transitions), /private-install-identity/)
  }
})

test('invalid live daemon identity is rejected before cache or persistence', async () => {
  installChrome(undefined)
  const queue = await import('../../../extension/background/runtime-state/log-queue.js')
  queue.clearExtensionLogsForTesting()
  const identity = await loadIdentity()

  identity.updateServerInstallId('attacker-controlled-id')

  assert.equal(identity.getServerInstallId(), undefined)
  assert.equal(globalThis.chrome.storage.local.set.mock.calls.length, 0)
  const incident = queue
    .getExtensionLogQueueSnapshot()
    .find((entry) => entry.data?.name === 'extension_install_identity_state')
  assert.match(incident.data.detail, /corruption failure/)
})

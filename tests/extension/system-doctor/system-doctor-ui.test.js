// system-doctor-ui.test.js — Popup System Doctor rendering and transport contracts.
import assert from 'node:assert/strict'
import { beforeEach, describe, test } from 'node:test'

const elements = new Map()

function element(id = '') {
  return {
    id,
    textContent: '',
    className: '',
    style: {},
    children: [],
    appendChild(child) {
      this.children.push(child)
      return child
    },
    replaceChildren(...children) {
      this.children = children
    }
  }
}

function renderedText(node) {
  return [node.textContent, ...node.children.map(renderedText)].join(' ')
}

describe('popup System Doctor', () => {
  beforeEach(() => {
    elements.clear()
    globalThis.document = {
      createElement: () => element(),
      getElementById: (id) => elements.get(id) ?? null
    }
    for (const id of ['system-doctor', 'system-doctor-overall', 'system-doctor-checks']) {
      elements.set(id, element(id))
    }
  })

  test('fetches the canonical daemon doctor report and renders actionable checks', async () => {
    const { refreshSystemDoctor } = await import('../../../extension/popup/system-doctor.js')
    const requests = []
    const fetchImpl = async (url) => {
      requests.push(url)
      return {
        ok: true,
        json: async () => ({
          status: 'degraded',
          ready_for_interaction: false,
          version: '0.9.0',
          checks: [
            { name: 'extension_connected', status: 'pass', detail: 'Extension connected' },
            {
              name: 'tracked_tab',
              status: 'warn',
              detail: 'No tab is being tracked',
              fix: 'Click Track This Tab'
            }
          ]
        })
      }
    }

    await refreshSystemDoctor(
      { connected: true, serverUrl: 'http://127.0.0.1:7890' },
      fetchImpl
    )

    assert.deepEqual(requests, ['http://127.0.0.1:7890/doctor'])
    assert.equal(document.getElementById('system-doctor-overall').textContent, 'Needs attention')
    const checks = document.getElementById('system-doctor-checks')
    assert.ok(renderedText(checks).includes('Extension connected'))
    assert.ok(renderedText(checks).includes('Click Track This Tab'))
  })

  test('shows daemon unavailability without throwing or inventing check results', async () => {
    const { refreshSystemDoctor } = await import('../../../extension/popup/system-doctor.js')
    await refreshSystemDoctor(
      { connected: false, serverUrl: 'http://127.0.0.1:7890' },
      async () => {
        throw new Error('must not fetch while offline')
      }
    )

    assert.equal(document.getElementById('system-doctor-overall').textContent, 'Daemon offline')
    assert.match(renderedText(document.getElementById('system-doctor-checks')), /Start the daemon/)
  })

  test('reports failed doctor transport as a retryable diagnostic error', async () => {
    const { refreshSystemDoctor } = await import('../../../extension/popup/system-doctor.js')
    await refreshSystemDoctor(
      { connected: true, serverUrl: 'http://127.0.0.1:7890' },
      async () => ({ ok: false, status: 503 })
    )

    assert.equal(document.getElementById('system-doctor-overall').textContent, 'Check failed')
    assert.match(renderedText(document.getElementById('system-doctor-checks')), /HTTP 503/)
  })
})

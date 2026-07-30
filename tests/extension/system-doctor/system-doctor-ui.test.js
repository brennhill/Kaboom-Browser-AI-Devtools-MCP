// system-doctor-ui.test.js — Popup System Doctor rendering and transport contracts.
import assert from 'node:assert/strict'
import { beforeEach, describe, test } from 'node:test'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const popupHtml = readFileSync(fileURLToPath(new URL('../../../extension/popup.html', import.meta.url)), 'utf8')

const elements = new Map()

function element(id = '') {
  return {
    id,
    textContent: '',
    className: '',
    dataset: {},
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
  test('keeps diagnostics at the bottom with a health plus icon', () => {
    const doctorIndex = popupHtml.indexOf('id="system-doctor"')
    const linksIndex = popupHtml.indexOf('class="links"')

    assert.ok(doctorIndex > linksIndex, 'System Doctor should follow all routine popup controls and links')
    assert.match(popupHtml, /class="system-doctor-icon"[^>]*>\+<\/span>/)
    assert.match(popupHtml, /<span>System Doctor<\/span>/)
    assert.doesNotMatch(popupHtml, /🩺/)
  })

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

  test('renders an unattached browser as idle rather than needing attention', async () => {
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
    assert.equal(document.getElementById('system-doctor-overall').textContent, 'Ready when attached')
    assert.equal(document.getElementById('system-doctor-overall').className, 'doctor-overall doctor-pass')
    const checks = document.getElementById('system-doctor-checks')
    assert.ok(renderedText(checks).includes('Extension connected'))
    assert.ok(renderedText(checks).includes('Click Track This Tab'))
  })

  test('reserves attention state for actionable doctor warnings', async () => {
    const { refreshSystemDoctor } = await import('../../../extension/popup/system-doctor.js')
    await refreshSystemDoctor(
      { connected: true, serverUrl: 'http://127.0.0.1:7890' },
      async () => ({
        ok: true,
        json: async () => ({
          status: 'degraded',
          ready_for_interaction: false,
          version: '0.9.0',
          checks: [
            { name: 'extension_connected', status: 'pass', detail: 'Extension connected' },
            {
              name: 'pilot_enabled',
              status: 'warn',
              detail: 'Browser control is disabled',
              fix: 'Enable browser control'
            }
          ]
        })
      })
    )

    assert.equal(document.getElementById('system-doctor-overall').textContent, 'Needs attention')
    assert.equal(document.getElementById('system-doctor-overall').className, 'doctor-overall doctor-warn')
  })

  test('renders recovered diagnostics as historical health information', async () => {
    const { refreshSystemDoctor } = await import('../../../extension/popup/system-doctor.js')
    await refreshSystemDoctor(
      { connected: true, serverUrl: 'http://127.0.0.1:7890' },
      async () => ({
        ok: true,
        json: async () => ({
          status: 'healthy',
          ready_for_interaction: true,
          version: '0.9.0',
          checks: [{
            name: 'tracked_tab_state',
            status: 'pass',
            detail: 'Saved tracking state was malformed; automatic tracking was used.',
            lifecycle: 'recovered',
            recovered_at: '2026-07-30T08:01:00Z',
            occurrences: 2
          }]
        })
      })
    )

    const checks = document.getElementById('system-doctor-checks')
    assert.match(renderedText(checks), /Recovered/)
    assert.match(renderedText(checks), /2 occurrences/)
    assert.equal(checks.children[0].dataset.lifecycle, 'recovered')
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

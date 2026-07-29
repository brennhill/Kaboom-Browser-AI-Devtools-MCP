// @ts-nocheck
/**
 * terminal-port-discovery.test.js — the terminal server port must be DISCOVERED,
 * not assumed.
 *
 * Finding S2: getTerminalServerUrl derived the port as `base + 1` and nothing ever
 * read the value the daemon actually publishes. The daemon binds the terminal
 * server on base+1 by default, but when that port is taken it logs
 * `terminal_server_bind_failed` — and it reports whatever port it really bound as
 * `terminal_port` in /health. The browser ignored it, so every terminal request
 * went to a port nothing was listening on and the terminal looked simply broken.
 */
import { beforeEach, afterEach, describe, test } from 'node:test'
import assert from 'node:assert'

import {
  resetTerminalPortDiscovery,
  getTerminalServerUrl,
  resolveTerminalServerUrl
} from '../../../extension/lib/terminal-server.js'

const BASE = 'http://127.0.0.1:7890'
const realFetch = globalThis.fetch

/** Stub fetch with a /health responder; records every requested URL. */
function stubHealth(responder) {
  const calls = []
  globalThis.fetch = async (url) => {
    calls.push(String(url))
    return responder(String(url))
  }
  return calls
}

function healthResponse(body) {
  return { ok: true, json: async () => body }
}

describe('terminal port discovery', () => {
  beforeEach(() => resetTerminalPortDiscovery())
  afterEach(() => {
    globalThis.fetch = realFetch
    resetTerminalPortDiscovery()
  })

  test('uses the terminal_port the daemon publishes', async () => {
    const calls = stubHealth(() => healthResponse({ status: 'ok', terminal_port: 7999 }))

    assert.strictEqual(await resolveTerminalServerUrl(BASE), 'http://127.0.0.1:7999')
    assert.ok(
      calls.some((u) => u === `${BASE}/health`),
      `discovery must read /health, saw ${JSON.stringify(calls)}`
    )
  })

  test('the discovered port is reused by the synchronous accessor', async () => {
    stubHealth(() => healthResponse({ status: 'ok', terminal_port: 7999 }))
    await resolveTerminalServerUrl(BASE)

    // notifyIframe's postMessage origin cannot await; it must still target the
    // real port once discovery has run in this context.
    assert.strictEqual(getTerminalServerUrl(BASE), 'http://127.0.0.1:7999')
  })

  test('discovery is cached — one /health per base URL', async () => {
    const calls = stubHealth(() => healthResponse({ status: 'ok', terminal_port: 7999 }))

    await resolveTerminalServerUrl(BASE)
    await resolveTerminalServerUrl(BASE)
    await resolveTerminalServerUrl(BASE)

    assert.strictEqual(calls.length, 1, 'the port must not be re-fetched on every terminal request')
  })

  test('falls back to base+1 when the daemon does not publish a port', async () => {
    // Windows / terminal server failed to bind: the field is omitted or zero.
    stubHealth(() => healthResponse({ status: 'ok' }))
    assert.strictEqual(await resolveTerminalServerUrl(BASE), 'http://127.0.0.1:7891')

    resetTerminalPortDiscovery()
    stubHealth(() => healthResponse({ status: 'ok', terminal_port: 0 }))
    assert.strictEqual(await resolveTerminalServerUrl(BASE), 'http://127.0.0.1:7891')
  })

  test('falls back to base+1 when /health is unreachable', async () => {
    stubHealth(() => {
      throw new Error('connection refused')
    })
    assert.strictEqual(await resolveTerminalServerUrl(BASE), 'http://127.0.0.1:7891')
  })

  test('falls back to base+1 when /health answers non-OK', async () => {
    stubHealth(() => ({ ok: false, status: 503, json: async () => ({}) }))
    assert.strictEqual(await resolveTerminalServerUrl(BASE), 'http://127.0.0.1:7891')
  })

  test('the synchronous accessor still derives base+1 before any discovery', () => {
    assert.strictEqual(getTerminalServerUrl(BASE), 'http://127.0.0.1:7891')
    assert.strictEqual(getTerminalServerUrl('http://127.0.0.1:9000'), 'http://127.0.0.1:9001')
  })
})

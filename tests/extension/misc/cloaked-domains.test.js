// @ts-nocheck
/**
 * @fileoverview cloaked-domains.test.js -- Tests for lib/cloaked-domains module.
 * Verifies built-in domain blocklist, user-configurable domains, and subdomain matching.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// =============================================================================
// MOCK FACTORY
// =============================================================================

function createChromeMock() {
  const store = {}
  return {
    runtime: { lastError: null },
    storage: {
      local: {
        get: mock.fn((keys) => {
          const result = {}
          const keyArr = Array.isArray(keys) ? keys : [keys]
          for (const k of keyArr) {
            if (store[k] !== undefined) result[k] = store[k]
          }
          return Promise.resolve(result)
        }),
        set: mock.fn((data) => {
          Object.assign(store, data)
          return Promise.resolve()
        }),
        remove: mock.fn((keys) => {
          const keyArr = Array.isArray(keys) ? keys : [keys]
          for (const k of keyArr) delete store[k]
          return Promise.resolve()
        })
      },
      sync: {
        get: mock.fn((k, cb) => cb && cb({})),
        set: mock.fn((d, cb) => cb && cb()),
        remove: mock.fn(() => Promise.resolve())
      },
      session: {
        get: mock.fn(() => Promise.resolve({})),
        set: mock.fn(() => Promise.resolve()),
        remove: mock.fn(() => Promise.resolve()),
        setAccessLevel: mock.fn(() => Promise.resolve())
      },
      onChanged: {
        addListener: mock.fn(),
        removeListener: mock.fn()
      }
    },
    _store: store
  }
}

let chromeMock = createChromeMock()
globalThis.chrome = chromeMock

let mod
async function loadModule() {
  if (!mod) {
    mod = await import('../../../extension/lib/tabs/cloaked-domains.js')
  }
  return mod
}

// =============================================================================
// BUILT-IN CLOAKED DOMAINS
// =============================================================================

describe('built-in cloaked domains', () => {
  beforeEach(() => {
    chromeMock = createChromeMock()
    globalThis.chrome = chromeMock
  })

  test('cloudflare.com is cloaked', async () => {
    const { isDomainCloaked } = await loadModule()
    assert.strictEqual(await isDomainCloaked('cloudflare.com'), true)
  })

  test('dash.cloudflare.com is cloaked (subdomain match)', async () => {
    const { isDomainCloaked } = await loadModule()
    assert.strictEqual(await isDomainCloaked('dash.cloudflare.com'), true)
  })

  test('api.cloudflare.com is cloaked (subdomain match)', async () => {
    const { isDomainCloaked } = await loadModule()
    assert.strictEqual(await isDomainCloaked('api.cloudflare.com'), true)
  })

  test('notcloudflare.com is NOT cloaked', async () => {
    const { isDomainCloaked } = await loadModule()
    assert.strictEqual(await isDomainCloaked('notcloudflare.com'), false)
  })

  test('example.com is NOT cloaked', async () => {
    const { isDomainCloaked } = await loadModule()
    assert.strictEqual(await isDomainCloaked('example.com'), false)
  })

  test('empty hostname returns false', async () => {
    const { isDomainCloaked } = await loadModule()
    assert.strictEqual(await isDomainCloaked(''), false)
  })
})

// =============================================================================
// USER-CONFIGURED CLOAKED DOMAINS
// =============================================================================

describe('user-configured cloaked domains', () => {
  beforeEach(() => {
    chromeMock = createChromeMock()
    globalThis.chrome = chromeMock
  })

  test('user-added domain is cloaked', async () => {
    const { isDomainCloaked } = await loadModule()
    chromeMock._store['kaboom_cloaked_domains'] = ['internal.corp.com']
    assert.strictEqual(await isDomainCloaked('internal.corp.com'), true)
  })

  test('subdomain of user-added domain is cloaked', async () => {
    const { isDomainCloaked } = await loadModule()
    chromeMock._store['kaboom_cloaked_domains'] = ['corp.com']
    assert.strictEqual(await isDomainCloaked('app.corp.com'), true)
  })

  test('unrelated domain is NOT cloaked', async () => {
    const { isDomainCloaked } = await loadModule()
    chromeMock._store['kaboom_cloaked_domains'] = ['corp.com']
    assert.strictEqual(await isDomainCloaked('other.com'), false)
  })

  test('works with empty user list', async () => {
    const { isDomainCloaked } = await loadModule()
    chromeMock._store['kaboom_cloaked_domains'] = []
    assert.strictEqual(await isDomainCloaked('example.com'), false)
  })

  test('works when user list is undefined', async () => {
    const { isDomainCloaked } = await loadModule()
    // No kaboom_cloaked_domains in storage
    assert.strictEqual(await isDomainCloaked('example.com'), false)
  })
})

// =============================================================================
// getCloakedDomains
// =============================================================================

describe('getCloakedDomains', () => {
  beforeEach(() => {
    chromeMock = createChromeMock()
    globalThis.chrome = chromeMock
  })

  test('returns built-in domains when no user domains configured', async () => {
    const { getCloakedDomains } = await loadModule()
    const domains = await getCloakedDomains()
    assert.ok(domains.includes('cloudflare.com'))
    assert.ok(domains.includes('dash.cloudflare.com'))
  })

  test('includes user-configured domains', async () => {
    const { getCloakedDomains } = await loadModule()
    chromeMock._store['kaboom_cloaked_domains'] = ['mysite.com']
    const domains = await getCloakedDomains()
    assert.ok(domains.includes('cloudflare.com'))
    assert.ok(domains.includes('mysite.com'))
  })
})

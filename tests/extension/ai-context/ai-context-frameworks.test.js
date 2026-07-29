// @ts-nocheck
/**
 * @fileoverview AI-context framework ancestry and application-state snapshot tests.
 */

import { test, describe, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow } from './ai-context-fixture.js'

let originalWindow

describe('Component Ancestry - React', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should detect React from __reactFiber$ key', async () => {
    const { detectFramework } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const element = { __reactFiber$abc123: {} }
    const result = detectFramework(element)

    assert.strictEqual(result.framework, 'react')
    assert.strictEqual(result.key, '__reactFiber$abc123')
  })

  test('should detect React from __reactInternalInstance$ key', async () => {
    const { detectFramework } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const element = { __reactInternalInstance$xyz: {} }
    const result = detectFramework(element)

    assert.strictEqual(result.framework, 'react')
  })

  test('should extract component names from fiber tree', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'LoginForm' },
      memoizedProps: { initialEmail: '' },
      memoizedState: { email: '', loading: false },
      return: {
        type: { name: 'AuthProvider' },
        memoizedProps: { children: null },
        return: {
          type: { name: 'App' },
          memoizedProps: { theme: 'dark' },
          return: null
        }
      }
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry.length, 3)
    // Root first order
    assert.strictEqual(ancestry[0].name, 'App')
    assert.strictEqual(ancestry[1].name, 'AuthProvider')
    assert.strictEqual(ancestry[2].name, 'LoginForm')
  })

  test('should prefer displayName over name', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'Comp', displayName: 'MyDisplayName' },
      memoizedProps: {},
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry[0].name, 'MyDisplayName')
  })

  test('should use Anonymous for unnamed components', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: '', displayName: null },
      memoizedProps: {},
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry[0].name, 'Anonymous')
  })

  test('should extract prop keys excluding children', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'Button' },
      memoizedProps: { onClick: () => {}, className: 'btn', children: 'text', disabled: false },
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.ok(ancestry[0].propKeys.includes('onClick'))
    assert.ok(ancestry[0].propKeys.includes('className'))
    assert.ok(!ancestry[0].propKeys.includes('children'))
  })

  test('should extract state keys', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'Form' },
      memoizedProps: {},
      memoizedState: { email: '', loading: false, error: null },
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry[0].hasState, true)
    assert.ok(ancestry[0].stateKeys.includes('email'))
    assert.ok(ancestry[0].stateKeys.includes('loading'))
    assert.ok(ancestry[0].stateKeys.includes('error'))
  })

  test('should limit ancestry depth to 10', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    let current = null
    for (let i = 0; i < 15; i++) {
      current = {
        type: { name: `C${i}` },
        memoizedProps: {},
        return: current
      }
    }

    const ancestry = getReactComponentAncestry(current)

    assert.ok(ancestry.length <= 10)
  })

  test('should limit prop keys to 20', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const props = {}
    for (let i = 0; i < 30; i++) props[`prop${i}`] = i

    const fiber = {
      type: { name: 'Big' },
      memoizedProps: props,
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.ok(ancestry[0].propKeys.length <= 20)
  })

  test('should limit state keys to 10', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const state = {}
    for (let i = 0; i < 15; i++) state[`state${i}`] = i

    const fiber = {
      type: { name: 'Big' },
      memoizedProps: {},
      memoizedState: state,
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.ok(ancestry[0].stateKeys.length <= 10)
  })

  test('should skip host elements (div, span, etc.)', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'Child' },
      memoizedProps: {},
      return: {
        type: 'div', // Host element
        memoizedProps: {},
        return: {
          type: { name: 'Parent' },
          memoizedProps: {},
          return: null
        }
      }
    }

    const ancestry = getReactComponentAncestry(fiber)
    const names = ancestry.map((c) => c.name)

    assert.ok(!names.includes('div'))
    assert.ok(names.includes('Child'))
    assert.ok(names.includes('Parent'))
  })

  test('should handle null fiber gracefully', async () => {
    const { getReactComponentAncestry } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const ancestry = getReactComponentAncestry(null)

    assert.strictEqual(ancestry, null)
  })
})

// --- Component Ancestry: Vue ---

describe('Component Ancestry - Vue', () => {
  test('should detect Vue 3 from __vueParentComponent', async () => {
    const { detectFramework } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const result = detectFramework({ __vueParentComponent: {} })

    assert.strictEqual(result.framework, 'vue')
  })

  test('should detect Vue app root from __vue_app__', async () => {
    const { detectFramework } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const result = detectFramework({ __vue_app__: {} })

    assert.strictEqual(result.framework, 'vue')
  })
})

// --- Component Ancestry: Svelte ---

describe('Component Ancestry - Svelte', () => {
  test('should detect Svelte from __svelte_meta', async () => {
    const { detectFramework } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const result = detectFramework({ __svelte_meta: { loc: { file: 'App.svelte' } } })

    assert.strictEqual(result.framework, 'svelte')
  })
})

// --- No Framework ---

describe('Framework Detection - None', () => {
  test('should return null for plain DOM elements', async () => {
    const { detectFramework } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const result = detectFramework({ tagName: 'DIV', className: 'container' })

    assert.strictEqual(result, null)
  })

  test('should return null for empty object', async () => {
    const { detectFramework } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    assert.strictEqual(detectFramework({}), null)
  })

  test('should return null for null', async () => {
    const { detectFramework } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    assert.strictEqual(detectFramework(null), null)
  })
})

// --- State Snapshot ---

describe('Application State Snapshot', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should detect Redux store and extract keys', async () => {
    const { captureStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        auth: { user: null, loading: false, error: 'Unauthorized' },
        cart: { items: [], total: 0 }
      })
    }

    const snapshot = captureStateSnapshot('Unauthorized')

    assert.strictEqual(snapshot.source, 'redux')
    assert.ok(snapshot.keys.auth)
    assert.ok(snapshot.keys.cart)

    delete globalThis.window.__REDUX_STORE__
  })

  test('should extract correct types for state values', async () => {
    const { captureStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        obj: { nested: true },
        arr: [1, 2, 3],
        num: 42,
        str: 'hello',
        bool: true,
        nil: null
      })
    }

    const snapshot = captureStateSnapshot('')

    assert.strictEqual(snapshot.keys.obj.type, 'object')
    assert.strictEqual(snapshot.keys.arr.type, 'array')
    assert.strictEqual(snapshot.keys.num.type, 'number')
    assert.strictEqual(snapshot.keys.str.type, 'string')
    assert.strictEqual(snapshot.keys.bool.type, 'boolean')

    delete globalThis.window.__REDUX_STORE__
  })

  test('should extract relevant slice based on error keywords', async () => {
    const { captureStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        auth: { user: null, error: 'Token expired' },
        cart: { items: ['a'], total: 50 },
        ui: { theme: 'dark' }
      })
    }

    const snapshot = captureStateSnapshot('auth failed: Token expired')

    assert.ok(snapshot.relevantSlice)
    // Should include auth state because error mentions "auth"
    const keys = Object.keys(snapshot.relevantSlice)
    assert.ok(keys.some((k) => k.startsWith('auth')))

    delete globalThis.window.__REDUX_STORE__
  })

  test('should include error/loading/status keys in relevant slice', async () => {
    const { captureStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        data: { items: [], loading: true, error: null, status: 'pending' },
        ui: { modal: false }
      })
    }

    const snapshot = captureStateSnapshot('')

    const keys = Object.keys(snapshot.relevantSlice)
    assert.ok(keys.some((k) => k.includes('loading')))
    assert.ok(keys.some((k) => k.includes('status')))

    delete globalThis.window.__REDUX_STORE__
  })

  test('should limit relevant slice to 10 entries', async () => {
    const { captureStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const state = {}
    for (let i = 0; i < 20; i++) {
      state[`mod${i}`] = { error: `err${i}`, loading: false }
    }

    globalThis.window.__REDUX_STORE__ = { getState: () => state }

    const snapshot = captureStateSnapshot('')

    assert.ok(Object.keys(snapshot.relevantSlice).length <= 10)

    delete globalThis.window.__REDUX_STORE__
  })

  test('should truncate values at 200 chars', async () => {
    const { captureStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        data: { error: 'x'.repeat(500) }
      })
    }

    const snapshot = captureStateSnapshot('')

    const errorValue = snapshot.relevantSlice['data.error']
    assert.ok(String(errorValue).length <= 200)

    delete globalThis.window.__REDUX_STORE__
  })

  test('should return null when no store is found', async () => {
    const { captureStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const snapshot = captureStateSnapshot('some error')

    assert.strictEqual(snapshot, null)
  })

  test('should handle store.getState() throwing', async () => {
    const { captureStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => {
        throw new Error('store error')
      }
    }

    const snapshot = captureStateSnapshot('')

    assert.strictEqual(snapshot, null)

    delete globalThis.window.__REDUX_STORE__
  })
})

// --- Summary Generation ---

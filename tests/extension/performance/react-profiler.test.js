// react-profiler.test.js — Deterministic opt-in React profiling contracts.
// Docs: docs/features/feature/react-performance-profiling/index.md

import { beforeEach, test } from 'node:test'
import assert from 'node:assert/strict'

let originalWindow

beforeEach(() => {
  originalWindow = globalThis.window
})

test.afterEach(() => {
  globalThis.window = originalWindow
})

test('reports unsupported without mutating pages that lack the React hook', async () => {
  globalThis.window = {}
  const profiler = await import('../../../extension/lib/analysis/react-profiler.js')
  profiler.resetReactProfilerForTesting()
  assert.deepStrictEqual(profiler.startReactProfile(), {
    status: 'unsupported',
    reason: 'react_devtools_hook_unavailable'
  })
})

test('captures bounded commit evidence and restores the original hook', async () => {
  let originalCalls = 0
  const original = () => {
    originalCalls++
  }
  const hook = {
    renderers: new Map([[1, { version: '19.1.0', rendererPackageName: 'react-dom' }]]),
    onCommitFiberRoot: original
  }
  globalThis.window = { __REACT_DEVTOOLS_GLOBAL_HOOK__: hook }
  const profiler = await import('../../../extension/lib/analysis/react-profiler.js')
  profiler.resetReactProfilerForTesting()

  assert.equal(profiler.startReactProfile().status, 'recording')
  const alternate = { memoizedProps: { title: 'before' }, memoizedState: { ready: false } }
  const child = {
    type: { displayName: 'DesignShell' },
    actualDuration: 12,
    memoizedProps: { title: 'after' },
    memoizedState: { ready: true },
    alternate,
    child: null,
    sibling: null,
    tag: 0
  }
  hook.onCommitFiberRoot(1, { current: { actualDuration: 18, child, sibling: null } })
  assert.equal(originalCalls, 1)

  const result = profiler.stopReactProfile()
  assert.equal(hook.onCommitFiberRoot, original)
  assert.equal(result.status, 'complete')
  assert.equal(result.commits[0].duration_ms, 18)
  assert.deepStrictEqual(result.components[0], {
    name: 'DesignShell',
    render_count: 1,
    total_duration_ms: 12,
    changed_props: ['title'],
    changed_state: true
  })
  assert.equal(JSON.stringify(result).includes('before'), false)
  assert.equal(JSON.stringify(result).includes('after'), false)
})

test('reports Suspense evidence and explicit Zustand capability limits', async () => {
  const hook = { renderers: new Map(), onCommitFiberRoot: undefined }
  globalThis.window = { __REACT_DEVTOOLS_GLOBAL_HOOK__: hook }
  const profiler = await import('../../../extension/lib/analysis/react-profiler.js')
  profiler.resetReactProfilerForTesting()
  profiler.startReactProfile()
  hook.onCommitFiberRoot(1, {
    current: {
      actualDuration: 3,
      child: { tag: 13, memoizedState: { dehydrated: null }, child: null, sibling: null }
    }
  })
  const result = profiler.stopReactProfile()
  assert.equal(result.suspense.pending_boundary_commits, 1)
  assert.deepStrictEqual(result.zustand, {
    status: 'unavailable',
    reason: 'zustand_does_not_expose_subscription_invalidations'
  })
})

test('page-owned fiber failures are logged and never break React commits', async () => {
  let logged = 0
  const originalError = console.error
  console.error = () => {
    logged++
  }
  try {
    const hook = { renderers: new Map(), onCommitFiberRoot: undefined }
    globalThis.window = { __REACT_DEVTOOLS_GLOBAL_HOOK__: hook }
    const profiler = await import('../../../extension/lib/analysis/react-profiler.js')
    profiler.resetReactProfilerForTesting()
    profiler.startReactProfile()
    const root = {}
    Object.defineProperty(root, 'current', { get: () => { throw new Error('hostile fiber') } })
    assert.doesNotThrow(() => hook.onCommitFiberRoot(1, root))
    assert.equal(logged, 1)
  } finally {
    console.error = originalError
  }
})

test('stop preserves a hook installed by another profiler during capture', async () => {
	const original = () => undefined
	const replacement = () => undefined
	const hook = { renderers: new Map(), onCommitFiberRoot: original }
	globalThis.window = { __REACT_DEVTOOLS_GLOBAL_HOOK__: hook }
	const profiler = await import('../../../extension/lib/analysis/react-profiler.js')
	profiler.resetReactProfilerForTesting()
	profiler.startReactProfile()
	hook.onCommitFiberRoot = replacement
	const result = profiler.stopReactProfile()
	assert.equal(hook.onCommitFiberRoot, replacement)
	assert.equal(result.timing_semantics, 'subtree_inclusive_actual_duration')
})

test('stop restores only the page hook owned when profiling started', async () => {
	const original = () => undefined
	const nextPageHook = () => undefined
	const firstHook = { renderers: new Map(), onCommitFiberRoot: original }
	const nextHook = { renderers: new Map(), onCommitFiberRoot: nextPageHook }
	globalThis.window = { __REACT_DEVTOOLS_GLOBAL_HOOK__: firstHook }
	const profiler = await import('../../../extension/lib/analysis/react-profiler.js')
	profiler.resetReactProfilerForTesting()
	profiler.startReactProfile()

	globalThis.window.__REACT_DEVTOOLS_GLOBAL_HOOK__ = nextHook
	assert.equal(profiler.stopReactProfile().status, 'complete')
	assert.equal(firstHook.onCommitFiberRoot, original)
	assert.equal(nextHook.onCommitFiberRoot, nextPageHook)
})

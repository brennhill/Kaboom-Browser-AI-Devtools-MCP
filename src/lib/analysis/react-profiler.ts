/**
 * Purpose: Capture bounded opt-in React commit evidence through the public DevTools hook boundary.
 * Docs: docs/features/feature/react-performance-profiling/index.md
 */

const MAX_COMMITS = 100
const MAX_FIBERS_PER_COMMIT = 5_000
const MAX_COMPONENTS = 200
const MAX_CHANGED_KEYS = 20

interface ReactRenderer {
  version?: string
  rendererPackageName?: string
}

interface Fiber {
  tag?: number
  type?: string | { displayName?: string; name?: string }
  elementType?: string | { displayName?: string; name?: string }
  actualDuration?: number
  memoizedProps?: unknown
  memoizedState?: unknown
  alternate?: Fiber | null
  child?: Fiber | null
  sibling?: Fiber | null
}

interface FiberRoot {
  current?: Fiber
}

type CommitHook = (rendererID: number, root: FiberRoot, priority?: unknown) => void

interface ReactDevtoolsHook {
  renderers?: Map<number, ReactRenderer>
  onCommitFiberRoot?: CommitHook
}

interface ComponentEvidence {
  name: string
  render_count: number
  total_duration_ms: number
  changed_props: string[]
  changed_state: boolean
}

interface CommitEvidence {
  renderer_id: number
  duration_ms: number
  rendered_components: number
  pending_suspense_boundaries: number
  truncated: boolean
}

export interface ReactProfileResult {
  status: 'complete'
  renderers: Array<{ id: number; package: string; version: string }>
  commits: CommitEvidence[]
  components: ComponentEvidence[]
  suspense: { pending_boundary_commits: number }
  zustand: { status: 'unavailable'; reason: 'zustand_does_not_expose_subscription_invalidations' }
  data_readiness: { status: 'suspense_only'; reason: 'application_data_contract_not_exposed' }
  dropped_commits: number
  timing_semantics: 'subtree_inclusive_actual_duration'
}

declare global {
  interface Window {
    __REACT_DEVTOOLS_GLOBAL_HOOK__?: ReactDevtoolsHook
  }
}

let activeHook: ReactDevtoolsHook | null = null
let originalCommitHook: CommitHook | undefined
let installedCommitHook: CommitHook | undefined
let commits: CommitEvidence[] = []
let components = new Map<string, ComponentEvidence>()
let droppedCommits = 0
let pendingBoundaryCommits = 0

export function startReactProfile(): { status: 'recording' } | { status: 'unsupported'; reason: string } {
  if (activeHook) return { status: 'recording' }
  const hook = typeof window !== 'undefined' ? window.__REACT_DEVTOOLS_GLOBAL_HOOK__ : undefined
  if (!hook) return { status: 'unsupported', reason: 'react_devtools_hook_unavailable' }
  resetEvidence()
  activeHook = hook
  originalCommitHook = hook.onCommitFiberRoot
  installedCommitHook = function (rendererID, root, priority): void {
    originalCommitHook?.call(hook, rendererID, root, priority)
    if (activeHook !== hook) return
    try {
      captureCommit(rendererID, root)
    } catch (error) {
      console.error('[KaBOOM!][react_profile] Commit attribution failed', {
        error: error instanceof Error ? error.message.slice(0, 256) : 'unknown_error'
      })
    }
  }
  hook.onCommitFiberRoot = installedCommitHook
  return { status: 'recording' }
}

export function stopReactProfile(): ReactProfileResult | { status: 'not_recording' } {
  if (!activeHook) return { status: 'not_recording' }
  if (activeHook.onCommitFiberRoot === installedCommitHook) {
    activeHook.onCommitFiberRoot = originalCommitHook
  }
  const result: ReactProfileResult = {
    status: 'complete',
    renderers: rendererEvidence(activeHook),
    commits: commits.map((commit) => ({ ...commit })),
    components: [...components.values()]
      .map((component) => ({ ...component, changed_props: [...component.changed_props] }))
      .sort((left, right) => right.total_duration_ms - left.total_duration_ms),
    suspense: { pending_boundary_commits: pendingBoundaryCommits },
    zustand: { status: 'unavailable', reason: 'zustand_does_not_expose_subscription_invalidations' },
    data_readiness: { status: 'suspense_only', reason: 'application_data_contract_not_exposed' },
    dropped_commits: droppedCommits,
    timing_semantics: 'subtree_inclusive_actual_duration'
  }
  activeHook = null
  originalCommitHook = undefined
  installedCommitHook = undefined
  return result
}

export function resetReactProfilerForTesting(): void {
  const hook = activeHook
  if (hook && hook.onCommitFiberRoot === installedCommitHook) hook.onCommitFiberRoot = originalCommitHook
  activeHook = null
  originalCommitHook = undefined
  installedCommitHook = undefined
  resetEvidence()
}

function captureCommit(rendererID: number, root: FiberRoot): void {
  if (commits.length >= MAX_COMMITS) {
    droppedCommits++
    return
  }
  const current = root.current
  const traversal = traverseFibers(current?.child ?? null)
  if (traversal.pendingSuspense > 0) pendingBoundaryCommits++
  commits.push({
    renderer_id: rendererID,
    duration_ms: finite(current?.actualDuration),
    rendered_components: traversal.rendered,
    pending_suspense_boundaries: traversal.pendingSuspense,
    truncated: traversal.truncated
  })
}

function traverseFibers(first: Fiber | null): { rendered: number; pendingSuspense: number; truncated: boolean } {
  const stack: Fiber[] = first ? [first] : []
  let visited = 0
  let rendered = 0
  let pendingSuspense = 0
  while (stack.length > 0 && visited < MAX_FIBERS_PER_COMMIT) {
    const fiber = stack.pop()
    if (!fiber) continue
    visited++
    if (fiber.sibling) stack.push(fiber.sibling)
    if (fiber.child) stack.push(fiber.child)
    if (fiber.tag === 13 && fiber.memoizedState != null) pendingSuspense++
    const duration = finite(fiber.actualDuration)
    const name = componentName(fiber)
    if (duration <= 0 || !name) continue
    rendered++
    recordComponent(name, duration, fiber)
  }
  return { rendered, pendingSuspense, truncated: stack.length > 0 }
}

function recordComponent(name: string, duration: number, fiber: Fiber): void {
  const existing = components.get(name)
  const changedProps = changedKeys(fiber.alternate?.memoizedProps, fiber.memoizedProps)
  const changedState = fiber.alternate != null && fiber.alternate.memoizedState !== fiber.memoizedState
  if (existing) {
    existing.render_count++
    existing.total_duration_ms = round(existing.total_duration_ms + duration)
    existing.changed_props = [...new Set([...existing.changed_props, ...changedProps])].slice(0, MAX_CHANGED_KEYS)
    existing.changed_state ||= changedState
    return
  }
  if (components.size >= MAX_COMPONENTS) return
  components.set(name, {
    name,
    render_count: 1,
    total_duration_ms: round(duration),
    changed_props: changedProps,
    changed_state: changedState
  })
}

function changedKeys(before: unknown, after: unknown): string[] {
  if (!isRecord(before) || !isRecord(after)) return []
  const keys = new Set([...Object.keys(before), ...Object.keys(after)])
  return [...keys].filter((key) => before[key] !== after[key]).slice(0, MAX_CHANGED_KEYS)
}

function componentName(fiber: Fiber): string {
  const type = fiber.type ?? fiber.elementType
  if (typeof type === 'string') return bounded(type, 128)
  return bounded(type?.displayName ?? type?.name ?? '', 128)
}

function rendererEvidence(hook: ReactDevtoolsHook): ReactProfileResult['renderers'] {
  return [...(hook.renderers ?? new Map()).entries()].slice(0, 20).map(([id, renderer]) => ({
    id,
    package: bounded(renderer.rendererPackageName ?? 'unknown', 128),
    version: bounded(renderer.version ?? 'unknown', 64)
  }))
}

function resetEvidence(): void {
  commits = []
  components = new Map()
  droppedCommits = 0
  pendingBoundaryCommits = 0
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function finite(value: number | undefined): number {
  return Number.isFinite(value) ? round(Number(value)) : 0
}

function round(value: number): number {
  return Math.round(value * 1000) / 1000
}

function bounded(value: string, max: number): string {
  return value.slice(0, max)
}

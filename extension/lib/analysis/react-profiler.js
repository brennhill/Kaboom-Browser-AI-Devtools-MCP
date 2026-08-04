/**
 * Purpose: Capture bounded opt-in React commit evidence through the public DevTools hook boundary.
 * Docs: docs/features/feature/react-performance-profiling/index.md
 */
const MAX_COMMITS = 100;
const MAX_FIBERS_PER_COMMIT = 5_000;
const MAX_COMPONENTS = 200;
const MAX_CHANGED_KEYS = 20;
let activeHook = null;
let originalCommitHook;
let commits = [];
let components = new Map();
let droppedCommits = 0;
let pendingBoundaryCommits = 0;
export function startReactProfile() {
    if (activeHook)
        return { status: 'recording' };
    const hook = typeof window !== 'undefined' ? window.__REACT_DEVTOOLS_GLOBAL_HOOK__ : undefined;
    if (!hook)
        return { status: 'unsupported', reason: 'react_devtools_hook_unavailable' };
    resetEvidence();
    activeHook = hook;
    originalCommitHook = hook.onCommitFiberRoot;
    hook.onCommitFiberRoot = function (rendererID, root, priority) {
        originalCommitHook?.call(hook, rendererID, root, priority);
        try {
            captureCommit(rendererID, root);
        }
        catch (error) {
            console.error('[KaBOOM!][react_profile] Commit attribution failed', {
                error: error instanceof Error ? error.message.slice(0, 256) : 'unknown_error'
            });
        }
    };
    return { status: 'recording' };
}
export function stopReactProfile() {
    if (!activeHook)
        return { status: 'not_recording' };
    activeHook.onCommitFiberRoot = originalCommitHook;
    const result = {
        status: 'complete',
        renderers: rendererEvidence(activeHook),
        commits: commits.map((commit) => ({ ...commit })),
        components: [...components.values()]
            .map((component) => ({ ...component, changed_props: [...component.changed_props] }))
            .sort((left, right) => right.total_duration_ms - left.total_duration_ms),
        suspense: { pending_boundary_commits: pendingBoundaryCommits },
        zustand: { status: 'unavailable', reason: 'zustand_does_not_expose_subscription_invalidations' },
        data_readiness: { status: 'suspense_only', reason: 'application_data_contract_not_exposed' },
        dropped_commits: droppedCommits
    };
    activeHook = null;
    originalCommitHook = undefined;
    return result;
}
export function resetReactProfilerForTesting() {
    if (activeHook)
        activeHook.onCommitFiberRoot = originalCommitHook;
    activeHook = null;
    originalCommitHook = undefined;
    resetEvidence();
}
function captureCommit(rendererID, root) {
    if (commits.length >= MAX_COMMITS) {
        droppedCommits++;
        return;
    }
    const current = root.current;
    const traversal = traverseFibers(current?.child ?? null);
    if (traversal.pendingSuspense > 0)
        pendingBoundaryCommits++;
    commits.push({
        renderer_id: rendererID,
        duration_ms: finite(current?.actualDuration),
        rendered_components: traversal.rendered,
        pending_suspense_boundaries: traversal.pendingSuspense,
        truncated: traversal.truncated
    });
}
function traverseFibers(first) {
    const stack = first ? [first] : [];
    let visited = 0;
    let rendered = 0;
    let pendingSuspense = 0;
    while (stack.length > 0 && visited < MAX_FIBERS_PER_COMMIT) {
        const fiber = stack.pop();
        if (!fiber)
            continue;
        visited++;
        if (fiber.sibling)
            stack.push(fiber.sibling);
        if (fiber.child)
            stack.push(fiber.child);
        if (fiber.tag === 13 && fiber.memoizedState != null)
            pendingSuspense++;
        const duration = finite(fiber.actualDuration);
        const name = componentName(fiber);
        if (duration <= 0 || !name)
            continue;
        rendered++;
        recordComponent(name, duration, fiber);
    }
    return { rendered, pendingSuspense, truncated: stack.length > 0 };
}
function recordComponent(name, duration, fiber) {
    const existing = components.get(name);
    const changedProps = changedKeys(fiber.alternate?.memoizedProps, fiber.memoizedProps);
    const changedState = fiber.alternate != null && fiber.alternate.memoizedState !== fiber.memoizedState;
    if (existing) {
        existing.render_count++;
        existing.total_duration_ms = round(existing.total_duration_ms + duration);
        existing.changed_props = [...new Set([...existing.changed_props, ...changedProps])].slice(0, MAX_CHANGED_KEYS);
        existing.changed_state ||= changedState;
        return;
    }
    if (components.size >= MAX_COMPONENTS)
        return;
    components.set(name, {
        name,
        render_count: 1,
        total_duration_ms: round(duration),
        changed_props: changedProps,
        changed_state: changedState
    });
}
function changedKeys(before, after) {
    if (!isRecord(before) || !isRecord(after))
        return [];
    const keys = new Set([...Object.keys(before), ...Object.keys(after)]);
    return [...keys].filter((key) => before[key] !== after[key]).slice(0, MAX_CHANGED_KEYS);
}
function componentName(fiber) {
    const type = fiber.type ?? fiber.elementType;
    if (typeof type === 'string')
        return bounded(type, 128);
    return bounded(type?.displayName ?? type?.name ?? '', 128);
}
function rendererEvidence(hook) {
    return [...(hook.renderers ?? new Map()).entries()].slice(0, 20).map(([id, renderer]) => ({
        id,
        package: bounded(renderer.rendererPackageName ?? 'unknown', 128),
        version: bounded(renderer.version ?? 'unknown', 64)
    }));
}
function resetEvidence() {
    commits = [];
    components = new Map();
    droppedCommits = 0;
    pendingBoundaryCommits = 0;
}
function isRecord(value) {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
}
function finite(value) {
    return Number.isFinite(value) ? round(Number(value)) : 0;
}
function round(value) {
    return Math.round(value * 1000) / 1000;
}
function bounded(value, max) {
    return value.slice(0, max);
}
//# sourceMappingURL=react-profiler.js.map
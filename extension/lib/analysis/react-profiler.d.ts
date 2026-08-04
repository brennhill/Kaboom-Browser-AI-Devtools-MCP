/**
 * Purpose: Capture bounded opt-in React commit evidence through the public DevTools hook boundary.
 * Docs: docs/features/feature/react-performance-profiling/index.md
 */
interface ReactRenderer {
    version?: string;
    rendererPackageName?: string;
}
interface Fiber {
    tag?: number;
    type?: string | {
        displayName?: string;
        name?: string;
    };
    elementType?: string | {
        displayName?: string;
        name?: string;
    };
    actualDuration?: number;
    memoizedProps?: unknown;
    memoizedState?: unknown;
    alternate?: Fiber | null;
    child?: Fiber | null;
    sibling?: Fiber | null;
}
interface FiberRoot {
    current?: Fiber;
}
type CommitHook = (rendererID: number, root: FiberRoot, priority?: unknown) => void;
interface ReactDevtoolsHook {
    renderers?: Map<number, ReactRenderer>;
    onCommitFiberRoot?: CommitHook;
}
interface ComponentEvidence {
    name: string;
    render_count: number;
    total_duration_ms: number;
    changed_props: string[];
    changed_state: boolean;
}
interface CommitEvidence {
    renderer_id: number;
    duration_ms: number;
    rendered_components: number;
    pending_suspense_boundaries: number;
    truncated: boolean;
}
export interface ReactProfileResult {
    status: 'complete';
    renderers: Array<{
        id: number;
        package: string;
        version: string;
    }>;
    commits: CommitEvidence[];
    components: ComponentEvidence[];
    suspense: {
        pending_boundary_commits: number;
    };
    zustand: {
        status: 'unavailable';
        reason: 'zustand_does_not_expose_subscription_invalidations';
    };
    data_readiness: {
        status: 'suspense_only';
        reason: 'application_data_contract_not_exposed';
    };
    dropped_commits: number;
}
declare global {
    interface Window {
        __REACT_DEVTOOLS_GLOBAL_HOOK__?: ReactDevtoolsHook;
    }
}
export declare function startReactProfile(): {
    status: 'recording';
} | {
    status: 'unsupported';
    reason: string;
};
export declare function stopReactProfile(): ReactProfileResult | {
    status: 'not_recording';
};
export declare function resetReactProfilerForTesting(): void;
export {};
//# sourceMappingURL=react-profiler.d.ts.map
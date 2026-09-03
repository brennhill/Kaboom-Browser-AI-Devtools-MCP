/**
 * Purpose: Capture a Chrome DevTools-compatible CPU flamechart trace and stream it to local daemon storage.
 * Docs: docs/features/feature/performance-trace/index.md
 */
import type { WirePerformanceTraceResult } from '../../../types/wire/wire-performance-trace.js';
import { type CDPSessionManager } from './cdp-session.js';
interface Debuggee {
    tabId?: number;
}
interface DebuggerAPI {
    sendCommand(target: Debuggee, method: string, commandParams?: object): Promise<object | undefined>;
    onEvent: {
        addListener(listener: (source: Debuggee, method: string, params?: object) => void): void;
    };
    onDetach: {
        addListener(listener: (source: Debuggee, reason: string) => void): void;
    };
}
interface ControllerDeps {
    debuggerApi: DebuggerAPI;
    /**
     * Owns debugger attachment. A trace takes an EXCLUSIVE lease: concurrent input dispatch
     * on the same tab would appear in the trace as user activity that never happened.
     */
    sessions: Pick<CDPSessionManager, 'acquire'>;
    postJSON: (path: string, payload: unknown) => Promise<unknown>;
    completionTimeoutMs?: number;
}
export interface PerformanceTraceStartOptions {
    reload?: boolean;
    cache?: 'warm' | 'cold';
}
export interface PerformanceTraceStarted {
    status: 'recording';
    trace_id: string;
    tab_id: number;
    url: string;
    navigation_id: string;
    build_sha: string;
    cache: 'warm' | 'cold';
    reloaded: boolean;
    recovered: boolean;
}
export interface PerformanceTraceFinished extends WirePerformanceTraceResult {
    import_with: string;
}
export declare class PerformanceTraceController {
    private readonly deps;
    private active;
    private readonly completionTimeoutMs;
    constructor(deps: ControllerDeps);
    start(tabId: number, options?: PerformanceTraceStartOptions): Promise<PerformanceTraceStarted>;
    stop(tabId: number): Promise<PerformanceTraceFinished>;
    private requireActive;
    private handleFrameNavigated;
    private handleTracingComplete;
    private uploadTraceEvents;
    private onEvent;
    private onDetach;
    private abortActive;
    private readTargetMetadata;
}
export declare function createPerformanceTraceController(deps: ControllerDeps): PerformanceTraceController;
/**
 * Chrome refused the debugger because the extension may not access this target.
 *
 * This is a property of the target, not of tracing: a tab whose DevTools target
 * URL belongs to a browser-internal page or another extension can look perfectly
 * scriptable through the tabs API and still reject every attach. Retrying against
 * the same tab can never succeed, so the caller must release an auto-resolved
 * target instead of reporting a generic tracing fault forever.
 *
 * "Another debugger is already attached" is deliberately excluded — that target is
 * accessible and the condition clears on its own.
 */
export declare function isTargetNotDebuggableError(error: unknown): boolean;
export declare function createDefaultPerformanceTraceController(): PerformanceTraceController;
export {};
//# sourceMappingURL=performance-trace.d.ts.map
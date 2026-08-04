/**
 * Purpose: Capture a Chrome DevTools-compatible CPU flamechart trace and stream it to local daemon storage.
 * Docs: docs/features/feature/performance-trace/index.md
 */
import type { WirePerformanceTraceResult } from '../../../types/wire/wire-performance-trace.js';
interface Debuggee {
    tabId?: number;
}
interface DebuggerAPI {
    attach(target: Debuggee, requiredVersion: string): Promise<void>;
    detach(target: Debuggee): Promise<void>;
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
    private onEvent;
    private onDetach;
    private abortActive;
    private readTargetMetadata;
}
export declare function createPerformanceTraceController(deps: ControllerDeps): PerformanceTraceController;
export declare function createDefaultPerformanceTraceController(): PerformanceTraceController;
export {};
//# sourceMappingURL=performance-trace.d.ts.map
/**
 * Purpose: Owns every chrome.debugger attachment so input dispatch, performance tracing and
 *          full-page capture share one session per tab instead of competing for it.
 * Why: Chrome permits exactly ONE debugger attachment per (extension, tab). Kaboom had four
 *      independent attach owners. Per-action attach/detach also races a click's own navigation
 *      — Input.dispatchMouseEvent resolves on delivery, not on default action, so detaching in
 *      a finally tears the session down mid-navigation (kaboom-knms) — and makes Chrome's
 *      debugging banner flicker once per action.
 * Docs: docs/features/feature/interact-explore/index.md
 */
export interface Debuggee {
    tabId: number;
}
/** The chrome.debugger surface this manager owns. Injected so tests use no real browser. */
export interface DebuggerApi {
    attach(target: Debuggee, requiredVersion: string): Promise<void>;
    detach(target: Debuggee): Promise<void>;
    sendCommand(target: Debuggee, method: string, params?: Record<string, unknown>): Promise<unknown>;
    onDetach: {
        addListener(listener: (source: Debuggee, reason: string) => void): void;
    };
}
type TimerHandle = ReturnType<typeof setTimeout>;
export interface CDPSessionDeps {
    debuggerApi: DebuggerApi;
    setTimeout?: (fn: () => void, ms: number) => TimerHandle;
    clearTimeout?: (handle: TimerHandle) => void;
    /** How long a session stays warm at zero refs. Long enough that an action burst reuses it. */
    idleGraceMs?: number;
    cdpVersion?: string;
    log?: (event: string, fields: Record<string, unknown>) => void;
}
export interface AcquireOptions {
    /** Take the session alone. Tracing needs this: concurrent input would pollute the trace. */
    exclusive?: boolean;
    /** How long an exclusive acquire waits for shared leases to drain before failing. */
    drainTimeoutMs?: number;
}
/** Named failures. Callers match on these instead of parsing free-text Chrome errors. */
export declare const CDP_SESSION_ERRORS: {
    readonly INVALIDATED: "cdp_session_invalidated";
    readonly EXCLUSIVE_HELD: "cdp_session_exclusive_held";
    readonly DRAIN_TIMEOUT: "cdp_session_drain_timeout";
    readonly ATTACH_FAILED: "cdp_session_attach_failed";
};
export declare const DEFAULT_IDLE_GRACE_MS = 30000;
export declare const DEFAULT_DRAIN_TIMEOUT_MS = 10000;
/** A held reference to a tab's CDP session. Release exactly once; repeat calls are no-ops. */
export interface Lease {
    readonly tabId: number;
    readonly valid: boolean;
    send(method: string, params?: Record<string, unknown>): Promise<unknown>;
    /** Send `<domain>.enable` once for the lifetime of this session. */
    ensureDomain(domain: string): Promise<void>;
    release(): void;
}
export declare class CDPSessionManager {
    private readonly deps;
    private readonly sessions;
    constructor(deps: CDPSessionDeps);
    /** Acquire a reference to the tab's session, attaching (or adopting) if it is cold. */
    acquire(tabId: number, options?: AcquireOptions): Promise<Lease>;
    /**
     * Tear the tab's session down NOW, invalidating every outstanding lease.
     *
     * This is what makes the supervision Stop button real: the interrupted action's next
     * lease.send fails loud with cdp_session_invalidated instead of running to completion
     * behind an overlay the user has already dismissed. Unlike a release, this does not wait
     * out the idle grace — the point is to interrupt.
     *
     * Inert for a tab with no live session, so a stop that races an action finishing cannot
     * tear down a session a later action already owns.
     */
    abort(tabId: number, reason: string): void;
    /** True when a live attachment exists for the tab. Diagnostics only. */
    isAttached(tabId: number): boolean;
    private sessionFor;
    private acquireExclusive;
    private grant;
    /**
     * Attach, or adopt an attachment that outlived this service worker.
     *
     * MV3 terminates the worker without detaching, so in-memory state can say "cold" while
     * Chrome still has us attached — and a second attach then throws. A read-only probe is
     * the only way to tell "we are already attached" from "someone else is": chrome.debugger
     * reports both as the same error string, and getTargets() reports that a debugger is
     * attached without saying whose it is (rule 18: reconcile against the live signal).
     */
    private ensureAttached;
    private probeAdoptable;
    private makeLease;
    private releaseRef;
    /** Hand a drained session to the longest-waiting exclusive acquirer, if any. */
    private promoteWaiter;
    private scheduleIdleDetach;
    private cancelIdleTimer;
    private teardown;
    /**
     * Chrome detached us: DevTools opened, the tab closed, or the user clicked Cancel on the
     * debugging banner. The detach is authoritative — every outstanding lease is dead and must
     * fail loud on its next send rather than silently no-op against a session that is gone.
     */
    private onExternalDetach;
}
/**
 * The one manager every CDP consumer must go through.
 *
 * Returns null when chrome.debugger is unavailable (tests, constrained extension contexts)
 * so callers fall back to their non-CDP path instead of throwing. This is the single
 * authoritative owner of debugger attachment — do not call chrome.debugger.attach directly
 * anywhere else (repo rule 18).
 */
export declare function cdpSessions(): CDPSessionManager | null;
export {};
//# sourceMappingURL=cdp-session.d.ts.map
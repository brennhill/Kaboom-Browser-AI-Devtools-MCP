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
// cdp-session.ts — Single-owner, ref-counted CDP session manager with detach recovery.
import { CDP_VERSION } from '../../../lib/constants.js';
import { errorMessage } from '../../../lib/error-utils.js';
/** Named failures. Callers match on these instead of parsing free-text Chrome errors. */
export const CDP_SESSION_ERRORS = {
    INVALIDATED: 'cdp_session_invalidated',
    EXCLUSIVE_HELD: 'cdp_session_exclusive_held',
    DRAIN_TIMEOUT: 'cdp_session_drain_timeout',
    ATTACH_FAILED: 'cdp_session_attach_failed'
};
export const DEFAULT_IDLE_GRACE_MS = 30_000;
export const DEFAULT_DRAIN_TIMEOUT_MS = 10_000;
/** Read-only probe used to decide whether Chrome already has us attached. */
const ADOPTION_PROBE = 'Page.getLayoutMetrics';
/**
 * Makes a tab the user is not looking at behave as if it were focused.
 *
 * Chrome delivers focus only to the visible tab, so a background tab reports
 * `document.hasFocus() === false`: `:focus` never paints, focus/blur handlers never run,
 * `autofocus` is ignored, and every widget that gates on focus — command palettes,
 * rich-text editors, type-to-search inputs — silently swallows the keystrokes the agent
 * dispatches. Without this override, driving a background tab reports success while the
 * field stays empty. With it, kaboom no longer has to steal the user's foreground to type.
 */
const FOCUS_EMULATION = 'Emulation.setFocusEmulationEnabled';
export class CDPSessionManager {
    deps;
    sessions = new Map();
    constructor(deps) {
        this.deps = {
            debuggerApi: deps.debuggerApi,
            setTimeout: deps.setTimeout ?? ((fn, ms) => setTimeout(fn, ms)),
            clearTimeout: deps.clearTimeout ?? ((handle) => clearTimeout(handle)),
            idleGraceMs: deps.idleGraceMs ?? DEFAULT_IDLE_GRACE_MS,
            cdpVersion: deps.cdpVersion ?? CDP_VERSION,
            log: deps.log
        };
        this.deps.debuggerApi.onDetach.addListener((source, reason) => this.onExternalDetach(source, reason));
    }
    /** Acquire a reference to the tab's session, attaching (or adopting) if it is cold. */
    async acquire(tabId, options = {}) {
        const session = this.sessionFor(tabId);
        if (options.exclusive)
            return this.acquireExclusive(session, options);
        if (session.exclusive) {
            throw new Error(`${CDP_SESSION_ERRORS.EXCLUSIVE_HELD}: tab ${tabId} is held exclusively (performance trace or equivalent)`);
        }
        return this.grant(session, false);
    }
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
    abort(tabId, reason) {
        const session = this.sessions.get(tabId);
        if (!session || !session.attached)
            return;
        this.cancelIdleTimer(session);
        session.refs = 0;
        session.exclusive = false;
        for (const waiter of session.waiters.splice(0)) {
            if (waiter.timer !== null)
                this.deps.clearTimeout(waiter.timer);
            waiter.reject(new Error(`${CDP_SESSION_ERRORS.INVALIDATED}: tab ${tabId} aborted (${reason})`));
        }
        void this.teardown(session, reason);
    }
    /** True when a live attachment exists for the tab. Diagnostics only. */
    isAttached(tabId) {
        return this.sessions.get(tabId)?.attached === true;
    }
    sessionFor(tabId) {
        let session = this.sessions.get(tabId);
        if (!session) {
            session = {
                tabId,
                generation: 0,
                refs: 0,
                attached: false,
                exclusive: false,
                enabledDomains: new Set(),
                idleTimer: null,
                waiters: []
            };
            this.sessions.set(tabId, session);
        }
        return session;
    }
    async acquireExclusive(session, options) {
        if (session.refs === 0 && !session.exclusive)
            return this.grant(session, true);
        const timeoutMs = options.drainTimeoutMs ?? DEFAULT_DRAIN_TIMEOUT_MS;
        return new Promise((resolve, reject) => {
            const waiter = { resolve, reject, timer: null };
            waiter.timer = this.deps.setTimeout(() => {
                session.waiters = session.waiters.filter((w) => w !== waiter);
                reject(new Error(`${CDP_SESSION_ERRORS.DRAIN_TIMEOUT}: tab ${session.tabId} still had ${session.refs} open lease(s) after ${timeoutMs}ms`));
            }, timeoutMs);
            session.waiters.push(waiter);
        });
    }
    async grant(session, exclusive) {
        this.cancelIdleTimer(session);
        session.refs += 1;
        session.exclusive = exclusive;
        try {
            await this.ensureAttached(session);
        }
        catch (err) {
            session.refs -= 1;
            session.exclusive = false;
            this.scheduleIdleDetach(session);
            throw err;
        }
        return this.makeLease(session);
    }
    /**
     * Attach, or adopt an attachment that outlived this service worker.
     *
     * MV3 terminates the worker without detaching, so in-memory state can say "cold" while
     * Chrome still has us attached — and a second attach then throws. A read-only probe is
     * the only way to tell "we are already attached" from "someone else is": chrome.debugger
     * reports both as the same error string, and getTargets() reports that a debugger is
     * attached without saying whose it is (rule 18: reconcile against the live signal).
     */
    async ensureAttached(session) {
        if (session.attached)
            return;
        const target = { tabId: session.tabId };
        if (await this.probeAdoptable(target)) {
            session.attached = true;
            this.deps.log?.('cdp_session_adopted', { tab_id: session.tabId });
        }
        else {
            try {
                await this.deps.debuggerApi.attach(target, this.deps.cdpVersion);
            }
            catch (err) {
                throw new Error(`${CDP_SESSION_ERRORS.ATTACH_FAILED}: ${errorMessage(err, 'unknown_error')}`);
            }
            session.attached = true;
            session.enabledDomains.clear();
            this.deps.log?.('cdp_session_attached', { tab_id: session.tabId });
        }
        // Once per session, including an adopted one: an attachment that outlived the service
        // worker kept Chrome's side alive but nothing re-asserts the override for us.
        await this.setFocusEmulation(target, true);
    }
    /**
     * Turn background-tab focus emulation on or off.
     *
     * A target that refuses the override (extension pages, pre-render placeholders) is still
     * perfectly drivable for clicks, reads and captures, so the refusal is reported and the
     * lease is granted anyway — failing the acquire would take away work that does succeed.
     */
    async setFocusEmulation(target, enabled) {
        try {
            await this.deps.debuggerApi.sendCommand(target, FOCUS_EMULATION, { enabled });
        }
        catch (err) {
            this.deps.log?.('cdp_focus_emulation_failed', {
                tab_id: target.tabId,
                enabled,
                error: errorMessage(err, 'unknown_error')
            });
        }
    }
    async probeAdoptable(target) {
        try {
            await this.deps.debuggerApi.sendCommand(target, ADOPTION_PROBE, {});
            return true;
        }
        catch {
            // EXPECTED_ABSENCE: a cold tab rejecting the probe is the normal path, and is exactly
            // the signal that we must attach. Logging it would report every first action as a failure.
            return false;
        }
    }
    makeLease(session) {
        const generation = session.generation;
        let released = false;
        const manager = this;
        const assertLive = () => {
            if (released || session.generation !== generation || !session.attached) {
                throw new Error(`${CDP_SESSION_ERRORS.INVALIDATED}: the CDP session for tab ${session.tabId} was detached`);
            }
        };
        return {
            tabId: session.tabId,
            get valid() {
                return !released && session.generation === generation && session.attached;
            },
            async send(method, params = {}) {
                assertLive();
                return manager.deps.debuggerApi.sendCommand({ tabId: session.tabId }, method, params);
            },
            async ensureDomain(domain) {
                assertLive();
                if (session.enabledDomains.has(domain))
                    return;
                await manager.deps.debuggerApi.sendCommand({ tabId: session.tabId }, `${domain}.enable`, {});
                session.enabledDomains.add(domain);
            },
            release() {
                if (released)
                    return;
                released = true;
                if (session.generation !== generation)
                    return;
                manager.releaseRef(session);
            }
        };
    }
    releaseRef(session) {
        session.refs = Math.max(0, session.refs - 1);
        if (session.refs > 0)
            return;
        session.exclusive = false;
        if (this.promoteWaiter(session))
            return;
        this.scheduleIdleDetach(session);
    }
    /** Hand a drained session to the longest-waiting exclusive acquirer, if any. */
    promoteWaiter(session) {
        const waiter = session.waiters.shift();
        if (!waiter)
            return false;
        if (waiter.timer !== null)
            this.deps.clearTimeout(waiter.timer);
        this.grant(session, true).then(waiter.resolve, waiter.reject);
        return true;
    }
    scheduleIdleDetach(session) {
        if (!session.attached || session.idleTimer !== null)
            return;
        session.idleTimer = this.deps.setTimeout(() => {
            session.idleTimer = null;
            if (session.refs > 0)
                return;
            void this.teardown(session, 'idle');
        }, this.deps.idleGraceMs);
    }
    cancelIdleTimer(session) {
        if (session.idleTimer === null)
            return;
        this.deps.clearTimeout(session.idleTimer);
        session.idleTimer = null;
    }
    async teardown(session, reason) {
        if (!session.attached)
            return;
        session.attached = false;
        session.generation += 1;
        session.enabledDomains.clear();
        // Hand the tab back without a forced-focus override. Issued but not awaited: Chrome
        // processes the two chrome.debugger calls in the order they were made, so the clear still
        // lands before the detach, and not awaiting keeps `detach` in the same turn as the
        // synchronous lease invalidation above — the Stop path and its tests depend on the
        // attachment being gone by the time abort() returns.
        void this.setFocusEmulation({ tabId: session.tabId }, false);
        try {
            await this.deps.debuggerApi.detach({ tabId: session.tabId });
            this.deps.log?.('cdp_session_detached', { tab_id: session.tabId, reason });
        }
        catch (err) {
            // A detach that fails leaves nothing to clean up — Chrome drops the session when the
            // tab or worker dies — but it is still unexpected, so it is reported (rule 27).
            this.deps.log?.('cdp_session_detach_failed', {
                tab_id: session.tabId,
                reason,
                error: errorMessage(err, 'unknown_error')
            });
        }
    }
    /**
     * Chrome detached us: DevTools opened, the tab closed, or the user clicked Cancel on the
     * debugging banner. The detach is authoritative — every outstanding lease is dead and must
     * fail loud on its next send rather than silently no-op against a session that is gone.
     */
    onExternalDetach(source, reason) {
        const session = this.sessions.get(source.tabId);
        if (!session || !session.attached)
            return;
        this.cancelIdleTimer(session);
        session.attached = false;
        session.generation += 1;
        session.refs = 0;
        session.exclusive = false;
        session.enabledDomains.clear();
        this.deps.log?.('cdp_session_external_detach', { tab_id: source.tabId, reason });
        for (const waiter of session.waiters.splice(0)) {
            if (waiter.timer !== null)
                this.deps.clearTimeout(waiter.timer);
            waiter.reject(new Error(`${CDP_SESSION_ERRORS.INVALIDATED}: tab ${source.tabId} detached (${reason})`));
        }
    }
}
// =============================================================================
// PROCESS-WIDE INSTANCE
// =============================================================================
let sharedManager = null;
/**
 * The one manager every CDP consumer must go through.
 *
 * Returns null when chrome.debugger is unavailable (tests, constrained extension contexts)
 * so callers fall back to their non-CDP path instead of throwing. This is the single
 * authoritative owner of debugger attachment — do not call chrome.debugger.attach directly
 * anywhere else (repo rule 18).
 */
export function cdpSessions() {
    if (sharedManager)
        return sharedManager;
    const api = globalThis.chrome?.debugger;
    if (!api?.attach || !api?.detach || !api?.sendCommand || !api?.onDetach)
        return null;
    sharedManager = new CDPSessionManager({ debuggerApi: api });
    return sharedManager;
}
//# sourceMappingURL=cdp-session.js.map
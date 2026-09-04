/**
 * Purpose: Owns "kaboom is driving this tab" as a SESSION rather than a per-action blip, and
 *          makes the user's Stop control actually interrupt the agent.
 * Why: The supervision overlay shipped with two halves missing. Nothing ever sent a
 *      heartbeat, so the overlay's self-teardown timer could only ever fire — the mechanism
 *      meant to prevent a stranded badge was the only thing that would remove a live one.
 *      And the Stop button's message had no listener at all: pressing it removed the overlay
 *      while the agent kept driving. A safety control that appears to work and does not is
 *      worse than none.
 * Docs: docs/features/feature/agent-supervision/index.md
 */
import { sendAgentIndicator } from '../ui/content-script-bridge.js';
import { cdpSessions } from '../dom/cdp/cdp-session.js';
/** How often the background reassures the overlay it is still alive. */
export const HEARTBEAT_INTERVAL_MS = 5_000;
/** Tracks which tabs are being driven, one entry per tab. */
export class DrivingSessions {
    sessions = new Map();
    deps;
    constructor(deps) {
        this.deps = {
            notify: deps.notify,
            abortSession: deps.abortSession,
            setInterval: deps.setInterval ?? ((fn, ms) => setInterval(fn, ms)),
            clearInterval: deps.clearInterval ?? ((handle) => clearInterval(handle))
        };
    }
    isDriving(tabId) {
        return this.sessions.has(tabId);
    }
    /**
     * Begin driving, or relabel a session already running.
     *
     * Relabelling must not stack a second heartbeat timer: a burst of actions on one tab is
     * one driving session to the person watching, and two timers would double the traffic and
     * leak one of them on stop.
     */
    start(tabId, action) {
        const existing = this.sessions.get(tabId);
        if (existing) {
            existing.action = action;
            this.deps.notify(tabId, 'driving', { action });
            return;
        }
        const heartbeat = this.deps.setInterval(() => {
            this.deps.notify(tabId, 'heartbeat');
        }, HEARTBEAT_INTERVAL_MS);
        this.sessions.set(tabId, { action, heartbeat, stopRequested: false });
        this.deps.notify(tabId, 'driving', { action });
    }
    /** Move the phantom cursor. Ignored when the tab is not being driven. */
    cursor(tabId, x, y) {
        if (!this.sessions.has(tabId))
            return;
        this.deps.notify(tabId, 'cursor', { x, y });
    }
    /** End driving. Idempotent: a second call emits nothing and leaks no timer. */
    stop(tabId) {
        const session = this.sessions.get(tabId);
        if (!session)
            return;
        this.deps.clearInterval(session.heartbeat);
        this.sessions.delete(tabId);
        this.deps.notify(tabId, 'idle');
    }
    /**
     * The USER pressed Stop. Only reachable from a trusted event in the content script.
     *
     * A stop for a tab nobody is driving is a race — the action finished as the user clicked —
     * not an error, and must NOT abort a CDP session that a later action may already own.
     */
    requestStop(tabId) {
        const session = this.sessions.get(tabId);
        if (!session)
            return;
        session.stopRequested = true;
        this.deps.abortSession(tabId, 'stopped_by_user');
        this.deps.clearInterval(session.heartbeat);
        this.sessions.delete(tabId);
        this.stopRequests.add(tabId);
        this.deps.notify(tabId, 'idle');
    }
    /** Tabs whose stop has not yet been reported to the action that was interrupted. */
    stopRequests = new Set();
    /**
     * Report and clear a pending stop.
     *
     * Consumed once so the interrupted action is reported as stopped_by_user and the NEXT
     * action on that tab is not also labelled stopped.
     */
    consumeStopRequest(tabId) {
        return this.stopRequests.delete(tabId);
    }
}
// =============================================================================
// PROCESS-WIDE INSTANCE + MESSAGE HANDLER
// =============================================================================
let shared = null;
/** The one driving-session registry. All CDP input paths must route through it. */
export function drivingSessions() {
    if (shared)
        return shared;
    shared = new DrivingSessions({
        notify: (tabId, phase, detail) => sendAgentIndicator(tabId, phase, detail),
        abortSession: (tabId, reason) => cdpSessions()?.abort(tabId, reason)
    });
    return shared;
}
/**
 * Handles the user pressing Stop on the supervision overlay.
 *
 * The tab is taken from the SENDER, never from the message body. A content script can only
 * speak for its own tab, so a page cannot craft a message that stops the agent's work in a
 * different tab.
 */
export function createSupervisionMessageHandler(sessions = drivingSessions()) {
    return {
        feature: 'agent-supervision',
        handle(message, sender, sendResponse) {
            if (message.type !== 'kaboom_agent_stop_requested')
                return undefined;
            const tabId = sender.tab?.id;
            if (typeof tabId !== 'number') {
                // A stop with no owning tab cannot be attributed, and guessing which tab to
                // interrupt would be worse than declining.
                sendResponse({ success: false, error: 'stop_without_tab' });
                return false;
            }
            sessions.requestStop(tabId);
            sendResponse({ success: true, stopped_tab_id: tabId });
            return false;
        }
    };
}
//# sourceMappingURL=driving-session.js.map
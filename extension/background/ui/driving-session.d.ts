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
import type { MessageHandlerOwner } from '../message-routing/types.js';
/** How often the background reassures the overlay it is still alive. */
export declare const HEARTBEAT_INTERVAL_MS = 5000;
type IntervalHandle = ReturnType<typeof setInterval>;
export type IndicatorPhase = 'driving' | 'idle' | 'cursor' | 'heartbeat';
export interface DrivingSessionDeps {
    /** Send a phase to the tab's supervision overlay. */
    notify: (tabId: number, phase: IndicatorPhase, detail?: {
        action?: string;
        x?: number;
        y?: number;
    }) => void;
    /**
     * Tear down the tab's CDP session so an in-flight action cannot continue. This is what
     * makes Stop real: invalidating the session makes the next lease.send fail loud instead
     * of the action running to completion behind a dismissed overlay.
     */
    abortSession: (tabId: number, reason: string) => void;
    setInterval?: (fn: () => void, ms: number) => IntervalHandle;
    clearInterval?: (handle: IntervalHandle) => void;
}
/** Tracks which tabs are being driven, one entry per tab. */
export declare class DrivingSessions {
    private readonly sessions;
    private readonly deps;
    constructor(deps: DrivingSessionDeps);
    isDriving(tabId: number): boolean;
    /**
     * Begin driving, or relabel a session already running.
     *
     * Relabelling must not stack a second heartbeat timer: a burst of actions on one tab is
     * one driving session to the person watching, and two timers would double the traffic and
     * leak one of them on stop.
     */
    start(tabId: number, action: string): void;
    /** Move the phantom cursor. Ignored when the tab is not being driven. */
    cursor(tabId: number, x: number, y: number): void;
    /** End driving. Idempotent: a second call emits nothing and leaks no timer. */
    stop(tabId: number): void;
    /**
     * The USER pressed Stop. Only reachable from a trusted event in the content script.
     *
     * A stop for a tab nobody is driving is a race — the action finished as the user clicked —
     * not an error, and must NOT abort a CDP session that a later action may already own.
     */
    requestStop(tabId: number): void;
    /** Tabs whose stop has not yet been reported to the action that was interrupted. */
    private readonly stopRequests;
    /**
     * Report and clear a pending stop.
     *
     * Consumed once so the interrupted action is reported as stopped_by_user and the NEXT
     * action on that tab is not also labelled stopped.
     */
    consumeStopRequest(tabId: number): boolean;
}
/** The one driving-session registry. All CDP input paths must route through it. */
export declare function drivingSessions(): DrivingSessions;
/**
 * Handles the user pressing Stop on the supervision overlay.
 *
 * The tab is taken from the SENDER, never from the message body. A content script can only
 * speak for its own tab, so a page cannot craft a message that stops the agent's work in a
 * different tab.
 */
export declare function createSupervisionMessageHandler(sessions?: DrivingSessions): MessageHandlerOwner;
export {};
//# sourceMappingURL=driving-session.d.ts.map
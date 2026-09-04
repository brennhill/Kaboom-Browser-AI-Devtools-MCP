/**
 * Purpose: Shows the human whose tab it is what the agent is about to do, and gives them a
 *          way to stop it — phantom cursor, driving indicator, stop control, heartbeat.
 * Why: Kaboom drives with trusted CDP input over a session that outlives a single action.
 *      Per-action toasts narrate what already happened; nothing said "an agent is driving
 *      this tab right now", and there was no stop control anywhere in the product.
 * Docs: docs/features/feature/agent-supervision/index.md
 */
/** Element ids. Exported so tests and the capture stripper can reason about them. */
export declare const AGENT_INDICATOR_IDS: {
    readonly root: "kaboom-agent-indicator";
    readonly cursor: "kaboom-phantom-cursor";
    readonly glow: "kaboom-driving-glow";
    readonly pill: "kaboom-driving-pill";
    readonly stop: "kaboom-driving-stop";
};
/**
 * How long the overlay survives without a heartbeat.
 *
 * MV3 terminates the service worker without warning. If the worker dies mid-action the
 * overlay must remove ITSELF, or the user is left with a permanent "an agent is driving
 * this tab" badge on a tab nothing is driving — the same staleness failure that left
 * TERMINAL_UI_STATE='open' forever (CLAUDE.md rule 18).
 */
export declare const HEARTBEAT_TTL_MS = 15000;
/** Above every plausible page z-index, below nothing we own. */
export declare const OVERLAY_Z_INDEX = 2147483646;
/** Why the overlay stopped being shown. Distinguishes a clean stop from a dead worker. */
export type TeardownReason = 'released' | 'heartbeat_expired' | 'stopped_by_user';
export interface AgentIndicatorState {
    driving: boolean;
    action: string | null;
    cursor: {
        x: number;
        y: number;
    } | null;
    lastHeartbeatAt: number;
}
/**
 * The supervision state machine, with no DOM dependency.
 *
 * Split from rendering so the parts that decide behaviour — heartbeat expiry, whether a stop
 * is honoured, whether the overlay should be visible — are testable as pure functions of the
 * clock and the inputs. This repo has no jsdom; logic that only exists inside DOM callbacks
 * cannot be tested here at all.
 */
export declare class AgentIndicatorCore {
    private readonly now;
    private state;
    constructor(now: () => number);
    snapshot(): AgentIndicatorState;
    get driving(): boolean;
    /** Begin (or relabel) driving. Idempotent: a second call updates the label only. */
    startDriving(action: string): void;
    /** The CDP lease was released, or the action finished. */
    stopDriving(): void;
    /** Move the phantom cursor. Ignored when not driving: a stray cursor implies activity. */
    moveCursor(x: number, y: number): boolean;
    heartbeat(): void;
    /**
     * Clock check. Returns a teardown reason when the overlay must come down, else null.
     * Pure with respect to the injected clock, so expiry is testable without waiting.
     */
    tick(): TeardownReason | null;
}
/**
 * Decide whether a stop interaction is honoured.
 *
 * Gated on `event.isTrusted`. A page can dispatch a synthetic click on any element in its
 * own document, so without this a hostile page could abort the agent at will — or, worse,
 * silently suppress the stop control's meaning by firing it constantly. Only a real user
 * gesture carries isTrusted:true.
 */
export declare function isHonouredStop(event: Pick<Event, 'isTrusted'> | null | undefined): boolean;
/** Label shown in the pill. Kept here so wording and truncation stay consistent (rule 21). */
export declare function drivingLabel(action: string | null): string;
/**
 * The supervision overlay as mounted in a page.
 *
 * The root carries `data-kaboom-overlay` so screenshot capture strips it — see
 * setKaboomOverlayVisibility. Do not rely on the id: an id list is exactly the mechanism
 * that silently stopped stripping the draw overlay.
 */
export declare class AgentIndicator {
    private readonly deps;
    private rendered;
    private readonly core;
    constructor(deps: {
        now: () => number;
        onStop: () => void;
    });
    get mounted(): boolean;
    get driving(): boolean;
    /** Create the overlay if absent. Idempotent, so any entry point may call it. */
    mount(): void;
    unmount(): void;
    startDriving(action: string): void;
    stopDriving(): void;
    moveCursor(x: number, y: number): void;
    heartbeat(): void;
    /** Drive from a timer. Removes the overlay when its worker has stopped heartbeating. */
    tick(): TeardownReason | null;
    private paint;
}
//# sourceMappingURL=agent-indicator.d.ts.map
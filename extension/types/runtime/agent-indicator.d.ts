/**
 * Purpose: Message contracts for the agent supervision overlay (phantom cursor, driving
 *          indicator, stop control, heartbeat).
 * Why: Cross-context message types must be declared before use (CLAUDE.md rule 20), and
 *      runtime-messages.ts is at its 800-line module limit.
 * Docs: docs/features/feature/agent-supervision/index.md
 */
/**
 * Drives the supervision overlay in a page.
 *
 * `driving` spans the lifetime of a CDP lease, not one action, so the user sees a steady
 * "an agent is driving this tab" state rather than a flicker of per-action toasts.
 * `cursor` moves the phantom pointer to the target BEFORE input dispatches, so what the
 * user sees is intent rather than history. `heartbeat` is what lets the overlay remove
 * itself when the service worker dies mid-action.
 */
export interface AgentIndicatorMessage {
    readonly type: 'kaboom_agent_indicator';
    readonly phase: 'driving' | 'idle' | 'cursor' | 'heartbeat';
    /** Action name shown in the pill. Present for phase 'driving'. */
    readonly action?: string;
    /** Viewport coordinates for phase 'cursor'. */
    readonly x?: number;
    readonly y?: number;
}
/**
 * Sent from the content script when the USER clicks Stop on the supervision overlay.
 * Only dispatched for a trusted event — a page cannot forge an abort.
 */
export interface AgentStopRequestMessage {
    readonly type: 'kaboom_agent_stop_requested';
    readonly at: number;
}
//# sourceMappingURL=agent-indicator.d.ts.map
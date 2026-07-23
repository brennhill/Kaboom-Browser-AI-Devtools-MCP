/**
 * Purpose: Formal state machine managing extension-server connection lifecycle with invariant enforcement.
 * Why: Prevents illegal state combinations (e.g., polling while disconnected) that cause silent failures.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Connection State Machine - Manages extension-server connection lifecycle
 * Implements formal state machine with invariant enforcement for reliable connection handling.
 */
export type ServerState = 'down' | 'booting' | 'up';
export type ExtensionState = 'disconnected' | 'connected' | 'active';
export type CircuitState = 'closed' | 'open' | 'half-open';
export type PollingState = 'stopped' | 'running';
export type PilotState = 'disabled' | 'enabled';
export type TrackingState = 'none' | 'tab_tracked';
export type CommandState = 'none' | 'queued' | 'processing';
export interface ConnectionState {
    server: ServerState;
    extension: ExtensionState;
    circuit: CircuitState;
    polling: PollingState;
    pilot: PilotState;
    tracking: TrackingState;
    commands: CommandState;
    lastHealthCheck: number;
    lastSuccessfulPoll: number;
    lastStateChange: number;
}
export type ConnectionEvent = {
    type: 'SERVER_UP';
} | {
    type: 'SERVER_DOWN';
} | {
    type: 'SERVER_BOOTING';
} | {
    type: 'HEALTH_OK';
} | {
    type: 'HEALTH_FAIL';
    error?: string;
} | {
    type: 'POLLING_STARTED';
} | {
    type: 'POLLING_STOPPED';
} | {
    type: 'POLL_SUCCESS';
} | {
    type: 'POLL_FAIL';
    error?: string;
} | {
    type: 'POLL_STALE';
} | {
    type: 'CB_OPENED';
} | {
    type: 'CB_HALF_OPEN';
} | {
    type: 'CB_CLOSED';
} | {
    type: 'CB_PROBE_SUCCESS';
} | {
    type: 'CB_PROBE_FAIL';
} | {
    type: 'USER_RESET';
} | {
    type: 'PILOT_ENABLED';
} | {
    type: 'PILOT_DISABLED';
} | {
    type: 'TRACKING_ENABLED';
    tabId: number;
} | {
    type: 'TRACKING_DISABLED';
} | {
    type: 'COMMAND_QUEUED';
} | {
    type: 'COMMAND_PROCESSING';
} | {
    type: 'COMMAND_COMPLETED';
} | {
    type: 'COMMAND_TIMEOUT';
};
export type StateChangeCallback = (oldState: ConnectionState, newState: ConnectionState, event: ConnectionEvent) => void;
export interface InvariantViolation {
    invariant: string;
    expected: string;
    actual: string;
    timestamp: number;
}
export declare class ConnectionStateMachine {
    private state;
    private listeners;
    private violations;
    private transitionHistory;
    private readonly maxHistorySize;
    constructor();
    private getInitialState;
    /**
     * Get current state (immutable copy)
     */
    getState(): ConnectionState;
    /**
     * Register a state change listener
     */
    onStateChange(callback: StateChangeCallback): () => void;
    /**
     * Get recent transition history for debugging
     */
    getTransitionHistory(): Array<{
        event: ConnectionEvent;
        timestamp: number;
    }>;
    /**
     * Get invariant violations for debugging
     */
    getViolations(): InvariantViolation[];
    /**
     * Process an event and transition to new state
     */
    transition(event: ConnectionEvent): ConnectionState;
    /** Transition table: each handler mutates the state draft */
    private static readonly transitions;
    /**
     * Compute the next state based on current state and event
     */
    private computeNextState;
    /**
     * Enforce invariants, fixing any violations
     */
    private enforceInvariants;
    /**
     * Record an invariant violation
     */
    private recordViolation;
    /**
     * Check if two states are equal
     */
    private statesEqual;
    /**
     * Check if polling is stale (no successful poll in threshold time)
     */
    isPollingStale(thresholdMs?: number): boolean;
    /**
     * Check if health check is stale
     */
    isHealthStale(thresholdMs?: number): boolean;
    /**
     * Get a human-readable summary of current state
     */
    getSummary(): string;
    /**
     * Reset to initial state (for testing or catastrophic recovery)
     */
    reset(): void;
}
export declare function getConnectionStateMachine(): ConnectionStateMachine;
export declare function resetConnectionStateMachine(): void;
//# sourceMappingURL=connection-state.d.ts.map
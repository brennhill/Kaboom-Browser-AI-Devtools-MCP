/**
 * Purpose: Defines canonical network telemetry type aliases and pending-request tracking contracts.
 * Why: Aligns extension network payload types with server wire contracts to prevent shape drift.
 * Docs: docs/features/feature/normalized-event-schema/index.md
 */
/**
 * @fileoverview Network Types
 * Network waterfall, request tracking, and body capture
 */
/**
 * Pending network request tracking (internal to inject script, not a wire type)
 */
export interface PendingRequest {
    readonly id: string;
    readonly url: string;
    readonly method: string;
    readonly startTime: number;
}
//# sourceMappingURL=network.d.ts.map
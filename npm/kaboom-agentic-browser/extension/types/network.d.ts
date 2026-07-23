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
/**
 * Network body payload — re-exported from wire type (canonical HTTP payload shape).
 * The stale interface previously used camelCase fields (contentType, requestBody, etc.)
 * that didn't match the Go server expectations.
 */
export type { WireNetworkBody as NetworkBodyPayload } from './wire-network.js';
/**
 * Network waterfall entry — re-exported from wire type.
 * The stale interface previously used camelCase fields and a WaterfallPhases sub-object
 * that didn't match the actual runtime data.
 */
export type { WireNetworkWaterfallEntry as WaterfallEntry } from './wire-network.js';
//# sourceMappingURL=network.d.ts.map
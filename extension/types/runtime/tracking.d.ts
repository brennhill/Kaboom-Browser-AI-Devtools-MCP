/**
 * Purpose: Defines tracked-tab continuity phases and runtime message contracts.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export type TrackingContinuityPhase = 'idle' | 'confirmed' | 'navigation_started' | 'provisional_url' | 'content_injecting' | 'extension_reconnecting' | 'recovery_failed';
export interface TrackingContinuitySnapshot {
    readonly tab_id?: number;
    readonly phase: TrackingContinuityPhase;
    readonly is_tracked: boolean;
    readonly confirmed_url?: string;
    readonly provisional_url?: string;
    readonly failure?: string;
}
export interface GetTrackingStateMessage {
    readonly type: 'get_tracking_state';
}
export interface GetTrackingStateResponse {
    readonly state: {
        isTracked: boolean;
        aiPilotEnabled: boolean;
        continuity: TrackingContinuitySnapshot;
    };
}
export interface TrackingContentReadyMessage {
    readonly type: 'tracking_content_ready';
    readonly url: string;
}
export interface TrackingReadinessProbeMessage {
    readonly type: 'tracking_readiness_probe';
    readonly correlation_id: string;
    readonly connection_generation: number;
}
export interface TrackingReadinessProbeResponse {
    readonly ready: true;
    readonly correlation_id: string;
    readonly connection_generation: number;
}
export interface TrackingContinuityChangedMessage {
    readonly type: 'tracking_continuity_changed';
    readonly snapshot: TrackingContinuitySnapshot;
}
//# sourceMappingURL=tracking.d.ts.map
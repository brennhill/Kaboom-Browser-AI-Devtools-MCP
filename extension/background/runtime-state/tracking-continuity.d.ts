/**
 * Purpose: Own the tracked-tab continuity state machine across navigation and reinjection.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import type { TrackingContinuitySnapshot } from '../../types/runtime/tracking.js';
type Listener = (snapshot: TrackingContinuitySnapshot) => void;
export declare class TrackingContinuity {
    private state;
    private readonly listeners;
    snapshot(): TrackingContinuitySnapshot;
    subscribe(listener: Listener): () => void;
    establish(tabId: number, url?: string): void;
    confirm(tabId: number, url?: string): void;
    private setConfirmed;
    navigationStarted(tabId: number): void;
    observeProvisionalURL(tabId: number, url: string): void;
    injectionStarted(tabId: number): void;
    extensionReconnectStarted(tabId: number): void;
    fail(tabId: number, failure: string): void;
    close(tabId: number): void;
    private canOwn;
    private forTrackedTab;
    private transition;
}
export declare const trackingContinuity: TrackingContinuity;
export {};
//# sourceMappingURL=tracking-continuity.d.ts.map
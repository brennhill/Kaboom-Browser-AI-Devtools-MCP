import type { TrackingContinuitySnapshot } from '../../types/runtime/tracking.js';
import type { MessageHandlerOwner } from './types.js';
export interface PilotHandlerDependencies {
    isEnabled: () => boolean;
    setEnabled: (enabled: boolean, callback?: () => void) => void;
    getTrackingContinuity: () => TrackingContinuitySnapshot;
    confirmTracking: (tabId: number, url?: string) => void;
}
export declare function broadcastTrackingState(untrackedTabId?: number | null): Promise<void>;
export declare function createPilotMessageHandler(deps: PilotHandlerDependencies): MessageHandlerOwner;
//# sourceMappingURL=pilot-handler.d.ts.map
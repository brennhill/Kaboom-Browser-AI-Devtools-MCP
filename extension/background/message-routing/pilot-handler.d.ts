import type { MessageHandlerOwner } from './types.js';
export interface PilotHandlerDependencies {
    isEnabled: () => boolean;
    setEnabled: (enabled: boolean, callback?: () => void) => void;
}
export declare function broadcastTrackingState(untrackedTabId?: number | null): Promise<void>;
export declare function createPilotMessageHandler(deps: PilotHandlerDependencies): MessageHandlerOwner;
//# sourceMappingURL=pilot-handler.d.ts.map
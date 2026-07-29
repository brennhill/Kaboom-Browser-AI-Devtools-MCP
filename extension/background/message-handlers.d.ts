import type { MessageHandlerOwner } from './message-routing/types.js';
export type { MessageHandlerOwner } from './message-routing/types.js';
export { createTelemetryMessageHandler } from './message-routing/telemetry-handler.js';
export { createStatusMessageHandler } from './message-routing/status-handler.js';
export { createSettingsMessageHandler } from './message-routing/settings-handler.js';
export { createPilotMessageHandler, broadcastTrackingState } from './message-routing/pilot-handler.js';
export { createCaptureMessageHandler } from './message-routing/capture-handler.js';
export { createUtilityMessageHandler } from './message-routing/utility-handler.js';
export interface MessageRouterDependencies {
    handlers: readonly MessageHandlerOwner[];
    debugLog: (category: string, message: string, data?: unknown) => void;
}
export declare function installMessageListener(deps: MessageRouterDependencies): void;
//# sourceMappingURL=message-handlers.d.ts.map
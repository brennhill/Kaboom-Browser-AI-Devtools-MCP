/**
 * Purpose: Own routing for telemetry events emitted by content scripts.
 */
import type { ChromeMessageSender, EnhancedAction, LogEntry, NetworkBodyPayload, PerformanceSnapshot, WebSocketEvent } from '../../types/index.js';
import type { MessageHandlerOwner } from './types.js';
export interface TelemetryHandlerDependencies {
    addLog: (entry: LogEntry) => void;
    addWebSocket: (event: WebSocketEvent) => void;
    addEnhancedAction: (action: EnhancedAction) => void;
    addNetworkBody: (body: NetworkBodyPayload) => void;
    addPerformance: (snapshot: PerformanceSnapshot) => void;
    handleLog: (payload: LogEntry, sender: ChromeMessageSender, tabId?: number) => Promise<void>;
    isNetworkBodyCaptureDisabled: () => boolean;
    debugLog: (category: string, message: string, data?: unknown) => void;
}
export declare function createTelemetryMessageHandler(deps: TelemetryHandlerDependencies): MessageHandlerOwner;
//# sourceMappingURL=telemetry-handler.d.ts.map
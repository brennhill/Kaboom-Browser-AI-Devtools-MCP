/**
 * Purpose: Own routing for telemetry events emitted by content scripts.
 */
import type { ChromeMessageSender } from '../../types/runtime/chrome.js';
import type { WireEnhancedAction as EnhancedAction } from '../../types/wire/wire-enhanced-action.js';
import type { LogEntry } from '../../types/capture/telemetry.js';
import type { WireNetworkBody as NetworkBodyPayload } from '../../types/wire/wire-network.js';
import type { WirePerformanceSnapshot as PerformanceSnapshot } from '../../types/wire/wire-performance-snapshot.js';
import type { WireWebSocketEvent as WebSocketEvent } from '../../types/wire/wire-websocket-event.js';
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
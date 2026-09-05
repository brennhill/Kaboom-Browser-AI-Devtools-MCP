/**
 * Purpose: Own routing for telemetry events emitted by content scripts.
 */
import type { ChromeMessageSender } from '../../types/runtime/chrome.js';
import type { WireEnhancedAction as EnhancedAction, WireEnvironmentPin } from '../../types/wire/wire-enhanced-action.js';
import type { LogEntry } from '../../types/capture/telemetry.js';
import type { WireNetworkBody as NetworkBodyPayload } from '../../types/wire/wire-network.js';
import type { WirePerformanceSnapshot as PerformanceSnapshot } from '../../types/wire/wire-performance-snapshot.js';
import type { WireWebSocketEvent as WebSocketEvent } from '../../types/wire/wire-websocket-event.js';
import type { MessageHandlerOwner } from './types.js';
import type { CaptureDiagnosticMessage } from '../../types/runtime/telemetry-messages.js';
export interface TelemetryHandlerDependencies {
    addLog: (entry: LogEntry) => void;
    addWebSocket: (event: WebSocketEvent) => void;
    addEnhancedAction: (action: EnhancedAction) => void;
    /**
     * The environment pin in force for the tab that produced an action, or undefined when the
     * tab is not pinned. Read per action rather than per session because a navigation clears
     * CDP overrides: a session-level stamp would claim a pin that lapsed halfway through.
     */
    environmentPinFor: (tabId: number | undefined) => WireEnvironmentPin | undefined;
    addNetworkBody: (body: NetworkBodyPayload) => void;
    addPerformance: (snapshot: PerformanceSnapshot) => void;
    handleLog: (payload: LogEntry, sender: ChromeMessageSender, tabId?: number) => Promise<void>;
    isNetworkBodyCaptureDisabled: () => boolean;
    debugLog: (category: string, message: string, data?: unknown) => void;
    addDiagnostic: (payload: CaptureDiagnosticMessage['payload']) => void;
}
export declare function createTelemetryMessageHandler(deps: TelemetryHandlerDependencies): MessageHandlerOwner;
//# sourceMappingURL=telemetry-handler.d.ts.map
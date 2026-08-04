/**
 * Purpose: Internal type definitions for content script message types, page-to-background message mapping, and pending request interfaces.
 */
/**
 * @fileoverview Content Script Internal Types
 * Type definitions for internal content script use
 */
import type { WebSocketCaptureMode } from '../types/capture/websocket.js';
import type { StateAction, BrowserStateSnapshot } from '../types/runtime/state.js';
import type { LogEntry } from '../types/capture/telemetry.js';
import type { WireWebSocketEvent as WebSocketEvent } from '../types/wire/wire-websocket-event.js';
import type { WireNetworkBody as NetworkBodyPayload } from '../types/wire/wire-network.js';
import type { WireEnhancedAction as EnhancedAction } from '../types/wire/wire-enhanced-action.js';
import type { WirePerformanceSnapshot as PerformanceSnapshot } from '../types/wire/wire-performance-snapshot.js';
import type { CaptureDiagnosticMessage } from '../types/runtime/telemetry-messages.js';
/**
 * Pending request statistics
 */
export interface PendingRequestStats {
    readonly highlight: number;
    readonly execute: number;
    readonly a11y: number;
    readonly dom: number;
}
/**
 * Setting message to be posted to page context
 */
export interface SettingMessage {
    type: 'kaboom_setting';
    setting: string;
    enabled?: boolean;
    mode?: WebSocketCaptureMode;
    url?: string;
}
/**
 * Highlight request message to page context
 */
export interface HighlightRequestMessage {
    type: 'kaboom_highlight_request';
    requestId: number;
    params: {
        selector: string;
        duration_ms?: number;
    };
}
/**
 * Execute JS request message to page context
 */
export interface ExecuteJsRequestMessage {
    type: 'kaboom_execute_js';
    requestId: number;
    script: string;
    timeoutMs: number;
}
/**
 * A11y query request message to page context
 */
export interface A11yQueryRequestMessage {
    type: 'kaboom_a11y_query';
    requestId: number;
    params: Record<string, unknown>;
}
/**
 * DOM query request message to page context
 */
export interface DomQueryRequestMessage {
    type: 'kaboom_dom_query';
    requestId: number;
    params: Record<string, unknown>;
}
/**
 * Get waterfall request message to page context
 */
export interface GetWaterfallRequestMessage {
    type: 'kaboom_get_waterfall';
    requestId: number;
}
/**
 * State command message to page context
 */
export interface StateCommandMessage {
    type: 'kaboom_state_command';
    messageId: string;
    action?: StateAction;
    name?: string;
    state?: BrowserStateSnapshot;
    include_url?: boolean;
}
/**
 * Union of all messages posted to page context
 */
export type PagePostMessage = SettingMessage | HighlightRequestMessage | ExecuteJsRequestMessage | A11yQueryRequestMessage | DomQueryRequestMessage | GetWaterfallRequestMessage | StateCommandMessage;
/**
 * Background message types sent from content script
 */
export interface LogMessageToBackground {
    type: 'log';
    payload: LogEntry;
    tabId: number | null;
}
export interface WsEventMessageToBackground {
    type: 'ws_event';
    payload: WebSocketEvent;
    tabId: number | null;
}
export interface NetworkBodyMessageToBackground {
    type: 'network_body';
    payload: NetworkBodyPayload;
    tabId: number | null;
}
export interface EnhancedActionMessageToBackground {
    type: 'enhanced_action';
    payload: EnhancedAction;
    tabId: number | null;
}
export interface PerformanceSnapshotMessageToBackground {
    type: 'performance_snapshot';
    payload: PerformanceSnapshot;
    tabId: number | null;
}
export type BackgroundMessageFromContent = LogMessageToBackground | WsEventMessageToBackground | NetworkBodyMessageToBackground | EnhancedActionMessageToBackground | PerformanceSnapshotMessageToBackground | CaptureDiagnosticMessage;
//# sourceMappingURL=types.d.ts.map
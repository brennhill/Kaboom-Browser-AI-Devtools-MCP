/**
 * Purpose: Internal type definitions for content script message types, page-to-background message mapping, and pending request interfaces.
 */
/**
 * @fileoverview Content Script Internal Types
 * Type definitions for internal content script use
 */
import type { WebSocketCaptureMode, StateAction, BrowserStateSnapshot, PageMessageType, LogEntry, WebSocketEvent, NetworkBodyPayload, EnhancedAction, PerformanceSnapshot } from '../types/index.js';
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
 * Page message event data from inject.js
 */
export interface PageMessageEventData {
    type?: PageMessageType;
    requestId?: number | string;
    result?: unknown;
    payload?: unknown;
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
export type BackgroundMessageFromContent = LogMessageToBackground | WsEventMessageToBackground | NetworkBodyMessageToBackground | EnhancedActionMessageToBackground | PerformanceSnapshotMessageToBackground;
//# sourceMappingURL=types.d.ts.map
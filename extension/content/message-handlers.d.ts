/**
 * Purpose: Handles incoming chrome.runtime messages from the background script -- pings, setting toggles, highlights, JS execution, state management, and draw mode.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * @fileoverview Message Handlers Module
 * Handles messages from background script
 */
import type { ContentMessage, ContentPingResponse, HighlightResponse } from '../types/runtime-messages.js';
import type { WebSocketCaptureMode } from '../types/capture/websocket.js';
import type { WireNetworkWaterfallEntry as WaterfallEntry } from '../types/wire/wire-network.js';
import type { StateAction, BrowserStateSnapshot } from '../types/runtime/state.js';
import type { A11yAuditResult } from '../types/capture/accessibility.js';
export declare const TOGGLE_MESSAGES: ReadonlySet<string>;
/**
 * Security: Validate sender is from the extension background script
 * Prevents content script from trusting messages from compromised page context
 */
export declare function isValidBackgroundSender(sender: chrome.runtime.MessageSender): boolean;
/**
 * Forward a highlight message from background to inject.js
 */
export declare function forwardHighlightMessage(message: {
    params: {
        selector: string;
        duration_ms?: number;
    };
}): Promise<HighlightResponse>;
/**
 * Handle state capture/restore commands
 */
export declare function handleStateCommand(params: {
    action?: StateAction;
    name?: string;
    state?: BrowserStateSnapshot;
    include_url?: boolean;
} | undefined): Promise<{
    error?: string;
    [key: string]: unknown;
}>;
/**
 * Handle KABOOM_PING message
 */
export declare function handlePing(sendResponse: (response: ContentPingResponse) => void): boolean;
/**
 * Handle toggle messages
 */
export declare function handleToggleMessage(message: ContentMessage & {
    enabled?: boolean;
    mode?: WebSocketCaptureMode;
    url?: string;
}): void;
type ExecuteJsResponse = {
    success: boolean;
    error?: string;
    message?: string;
    result?: unknown;
    stack?: string;
};
/**
 * Handle kaboom_execute_js message.
 * Always executes in MAIN world via inject script.
 * Returns inject_not_loaded error if inject script isn't available,
 * so background can fallback to chrome.scripting API.
 */
export declare function handleExecuteJs(params: {
    script?: string;
    timeout_ms?: number;
}, sendResponse: (result: ExecuteJsResponse) => void): boolean;
/**
 * Handle KABOOM_EXECUTE_QUERY message (async command path)
 */
export declare function handleExecuteQuery(params: string | Record<string, unknown>, sendResponse: (result: ExecuteJsResponse) => void): boolean;
/**
 * Handle A11Y_QUERY message
 */
export declare function handleA11yQuery(params: string | Record<string, unknown>, sendResponse: (result: A11yAuditResult | {
    error: string;
}) => void): boolean;
/**
 * Handle DOM_QUERY message
 */
export declare function handleDomQuery(params: string | Record<string, unknown>, sendResponse: (result: {
    error?: string;
    matches?: unknown[];
}) => void): boolean;
/**
 * Handle GET_NETWORK_WATERFALL message
 */
export declare function handleGetNetworkWaterfall(sendResponse: (result: {
    entries: WaterfallEntry[];
}) => void): boolean;
export declare function handleComputedStylesQuery(params: string | Record<string, unknown>, sendResponse: (result: unknown) => void): boolean;
export declare function handleFormDiscoveryQuery(params: string | Record<string, unknown>, sendResponse: (result: unknown) => void): boolean;
export declare function handleFormStateQuery(params: string | Record<string, unknown>, sendResponse: (result: unknown) => void): boolean;
export declare function handleDataTableQuery(params: string | Record<string, unknown>, sendResponse: (result: unknown) => void): boolean;
export declare function handleLinkHealthQuery(params: string | Record<string, unknown>, sendResponse: (result: unknown) => void): boolean;
/**
 * Handle GET_READABLE message — extract readable content directly in ISOLATED world.
 */
export declare function handleGetReadable(sendResponse: (result: unknown) => void): boolean;
/**
 * Handle GET_MARKDOWN message — extract markdown content directly in ISOLATED world.
 */
export declare function handleGetMarkdown(sendResponse: (result: unknown) => void): boolean;
/**
 * Handle PAGE_SUMMARY message — extract page summary directly in ISOLATED world.
 */
export declare function handlePageSummary(sendResponse: (result: unknown) => void): boolean;
export {};
//# sourceMappingURL=message-handlers.d.ts.map
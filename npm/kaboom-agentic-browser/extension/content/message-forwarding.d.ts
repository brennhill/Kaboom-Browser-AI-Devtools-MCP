/**
 * Purpose: Forwards window.postMessage events from the inject context to the background script via chrome.runtime.sendMessage.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Message Forwarding Module
 * Forwards messages between page context and background script
 */
import type { BackgroundMessageFromContent } from './types.js';
export declare const MESSAGE_MAP: Record<string, string>;
/**
 * Safely send a message to the background script
 * Handles extension context invalidation gracefully
 */
export declare function safeSendMessage(msg: BackgroundMessageFromContent): void;
//# sourceMappingURL=message-forwarding.d.ts.map
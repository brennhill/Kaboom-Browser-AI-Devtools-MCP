/**
 * Purpose: Wraps the WebSocket constructor to intercept lifecycle events and messages, delegating tracking and sampling to websocket-tracking.ts.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview WebSocket capture.
 * Wraps the WebSocket constructor to intercept lifecycle events and messages.
 * Delegates tracking, sampling, and schema detection to websocket-tracking.ts.
 *
 * Re-exports all tracking primitives so existing importers are unaffected.
 */
import type { WebSocketCaptureMode } from '../types/index.js';
export { getSize, formatPayload, truncateWsMessage, createConnectionTracker } from './websocket-tracking.js';
export type { ConnectionTracker } from './websocket-tracking.js';
/**
 * Install WebSocket capture by wrapping the WebSocket constructor.
 * If the early-patch script ran first (world: "MAIN", document_start),
 * uses the saved original constructor and adopts buffered connections.
 */
export declare function installWebSocketCapture(): void;
/**
 * Set the WebSocket capture mode
 */
export declare function setWebSocketCaptureMode(mode: WebSocketCaptureMode): void;
/**
 * Set WebSocket capture enabled state
 */
export declare function setWebSocketCaptureEnabled(enabled: boolean): void;
/**
 * Get the current WebSocket capture mode
 */
export declare function getWebSocketCaptureMode(): WebSocketCaptureMode;
/**
 * Uninstall WebSocket capture, restoring the original constructor
 */
export declare function uninstallWebSocketCapture(): void;
/**
 * Reset all module state for testing purposes
 * Restores original WebSocket if installed, resets capture settings to defaults.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export declare function resetForTesting(): void;
//# sourceMappingURL=websocket.d.ts.map
/**
 * Purpose: Defines websocket capture mode/event contracts and canonical websocket wire-type aliases.
 * Why: Prevents websocket payload divergence between extension capture and server ingestion paths.
 * Docs: docs/features/feature/normalized-event-schema/index.md
 */
/**
 * @fileoverview WebSocket Types
 * WebSocket capture modes, events, and connection tracking
 */
/**
 * WebSocket capture modes
 */
export type WebSocketCaptureMode = 'low' | 'medium' | 'high' | 'all';
/**
 * WebSocket event types
 */
export type WebSocketEventType = 'open' | 'close' | 'error' | 'message';
//# sourceMappingURL=websocket.d.ts.map
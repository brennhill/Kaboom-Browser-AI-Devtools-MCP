/**
 * Purpose: WebSocket connection statistics, adaptive message sampling, schema inference, and message formatting primitives.
 * Docs: docs/features/feature/observe/index.md
 */
import type { WebSocketCaptureMode } from '../types/index.js';
/** WebSocket message data variants */
export type WebSocketMessageData = string | ArrayBuffer | Blob;
/** Object with a size property (like Blob) */
export interface SizedObject {
    size: number;
}
/** Per-direction message statistics */
interface DirectionStats {
    count: number;
    bytes: number;
    lastPreview: string | null;
    lastAt: number | null;
}
/** Connection statistics */
export interface ConnectionStats {
    incoming: DirectionStats;
    outgoing: DirectionStats;
}
/** Message direction */
export type MessageDirection = 'incoming' | 'outgoing';
/** Truncation result */
export interface TruncationResult {
    data: string;
    truncated: boolean;
}
/** Sampling info */
export interface SamplingInfo {
    rate: string;
    logged: string;
    window: string;
}
/** Schema info */
export interface SchemaInfo {
    detectedKeys: string[] | null;
    consistent: boolean;
    variants?: string[];
}
/** Connection tracker interface */
export interface ConnectionTracker {
    id: string;
    url: string;
    messageCount: number;
    _sampleCounter: number;
    _messageRate: number;
    _messageTimestamps: number[];
    _schemaKeys: string[];
    _schemaVariants: Map<string, number>;
    _schemaConsistent: boolean;
    _schemaDetected: boolean;
    stats: ConnectionStats;
    recordMessage(direction: MessageDirection, data: WebSocketMessageData | null): void;
    shouldSample(direction: MessageDirection): boolean;
    shouldLogLifecycle(): boolean;
    getSamplingInfo(): SamplingInfo;
    getMessageRate(): number;
    setMessageRate(rate: number): void;
    getSchema(): SchemaInfo;
    isSchemaChange(data: string | null): boolean;
}
/** Set the WebSocket capture mode */
export declare function setWebSocketCaptureModeInternal(mode: WebSocketCaptureMode): void;
/** Get the current WebSocket capture mode */
export declare function getWebSocketCaptureModeInternal(): WebSocketCaptureMode;
/** Reset capture mode to default (for testing) */
export declare function resetCaptureModeForTesting(): void;
/**
 * Get the byte size of a WebSocket message
 */
export declare function getSize(data: WebSocketMessageData | SizedObject | null): number;
/**
 * Format a WebSocket payload for logging
 */
export declare function formatPayload(data: WebSocketMessageData | null): string;
/**
 * Truncate a WebSocket message to the size limit
 */
export declare function truncateWsMessage(message: string): TruncationResult;
/**
 * Create a connection tracker for adaptive sampling and schema detection
 */
export declare function createConnectionTracker(id: string, url: string): ConnectionTracker;
export {};
//# sourceMappingURL=websocket-tracking.d.ts.map
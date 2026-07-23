/**
 * @fileoverview Wire type for WebSocket events — matches internal/types/wire_websocket_event.go
 *
 * Canonical TypeScript definition for the WebSocketEvent HTTP payload.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */
/**
 * WireWebSocketEvent is the JSON shape sent over HTTP for captured WebSocket events.
 */
export interface WireWebSocketEvent {
    readonly ts?: string;
    readonly type?: string;
    readonly event: string;
    readonly id: string;
    readonly url?: string;
    readonly direction?: 'incoming' | 'outgoing';
    readonly data?: string;
    readonly size?: number;
    readonly code?: number;
    readonly reason?: string;
}
//# sourceMappingURL=wire-websocket-event.d.ts.map
/**
 * Purpose: Exposes the canonical extension type barrel that aggregates runtime, telemetry, and utility contracts.
 * Why: Provides a stable import surface so cross-module typing remains consistent during refactors.
 * Docs: docs/features/feature/query-service/index.md
 */
/**
 * @fileoverview Type Index - Barrel export for all Kaboom Extension types
 *
 * This is the single entry point for importing types in the extension.
 * Usage: import type { LogEntry, BackgroundMessage } from './types.js';
 */
export type * from './capture/accessibility.js';
export type * from './capture/actions.js';
export type * from './capture/ai-context.js';
export type * from './capture/dom.js';
export type * from './capture/network.js';
export type * from './capture/performance.js';
export type * from './capture/sourcemap.js';
export type * from './capture/telemetry.js';
export type * from './capture/websocket.js';
export type * from './runtime/chrome.js';
export type * from './runtime/debug.js';
export type * from './runtime/queries.js';
export type * from './runtime/state.js';
export type * from './runtime-messages.js';
export type { WireEnhancedAction } from './wire/wire-enhanced-action.js';
export type { WireNetworkBody, WireNetworkWaterfallEntry, WireNetworkWaterfallPayload } from './wire/wire-network.js';
export type { WireWebSocketEvent } from './wire/wire-websocket-event.js';
export type { WirePerformanceTiming, WireTypeSummary, WireSlowRequest, WireNetworkSummary, WireLongTaskMetrics, WireUserTimingEntry, WireUserTimingData, WirePerformanceSnapshot } from './wire/wire-performance-snapshot.js';
export type { DeepReadonly, PartialBy, RequiredBy, ArrayElement, JsonPrimitive, JsonArray, JsonObject, JsonValue, Serializable, NonNullableFields, KeysOfType, OmitByType, PickByType, AsyncFunction, Callback, ErrorCallback, EventHandler, DebouncedFunction, Result, AsyncResult, OperationResult, Brand, TabId, QueryId, SessionId, CorrelationId, Timestamp, ValidatedString, ValidatedUrl, ExtractByType, TypesOf, MessageHandlerMap, SerializedElementInfo, ElementSelector, ExtensionSettings, PartialSettings, RateLimitConfig, BatcherConfig, TimeoutId, IntervalId, TimerCleanup, BufferState, MemoryEstimate } from './utils.js';
export { isObject, isNonEmptyString, hasType, isJsonValue, createTypeGuard } from './utils.js';
/**
 * Tracking state for favicon replacer
 */
export interface TrackingState {
    isTracked: boolean;
    aiPilotEnabled: boolean;
}
//# sourceMappingURL=index.d.ts.map
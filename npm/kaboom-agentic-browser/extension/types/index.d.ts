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
export type { LogLevel, LogLevelFilter, LogType, BaseLogEntry, ConsoleLogEntry, NetworkLogEntry, ExceptionLogEntry, ScreenshotLogEntry, LogEntry, ProcessedLogEntry, WebSocketCaptureMode, WebSocketEventType, WebSocketEvent, WaterfallEntry, PendingRequest, NetworkBodyPayload, PerformanceMark, PerformanceMeasure, PerformanceSnapshot, ActionType, ActionEntry, SelectorStrategies, EnhancedAction, StackFrame, SourceSnippet, ReactComponentAncestry, AiContextData, A11yViolationNode, A11yViolation, A11yAuditResult, DomElementInfo, DomQueryResult, PageInfo, BrowserStateSnapshot, SavedStateSnapshot, StateAction, GetTabIdMessage, GetTabIdResponse, WsEventMessage, EnhancedActionMessage, NetworkBodyMessage, PerformanceSnapshotMessage, LogMessage, GetStatusMessage, ClearLogsMessage, SetLogLevelMessage, SetBooleanSettingMessage, SetWebSocketCaptureModeMessage, GetAiWebPilotEnabledMessage, GetAiWebPilotEnabledResponse, GetDiagnosticStateMessage, GetDiagnosticStateResponse, CaptureScreenshotMessage, GetDebugLogMessage, ClearDebugLogMessage, SetServerUrlMessage, StatusUpdateMessage, DrawModeCompletedMessage, BackgroundMessage, ContentPingMessage, ContentPingResponse, HighlightMessage, HighlightResponse, ExecuteJsMessage, ExecuteQueryMessage, DomQueryMessage, A11yQueryMessage, GetNetworkWaterfallMessage, ManageStateMessage, ContentMessage, PageMessageType, ContentToPageMessageType, ExecuteJsResult, CircuitBreakerState, CircuitBreakerStats, MemoryPressureLevel, MemoryPressureState, ConnectionStatus, ContextWarning, DebugCategory, DebugLogEntry, ErrorGroup, RateLimitResult, CaptureScreenshotResult, QueryType, PendingQuery, BrowserActionParams, BrowserActionResult, TabInfo, ParsedSourceMap, OriginalLocation, ChromeMessageSender, ChromeTabInfo, StorageChange, StorageAreaName, ChromeSessionStorage, ChromeStorageWithSession } from './messages.js';
export type { WireEnhancedAction } from './wire-enhanced-action.js';
export type { WireNetworkBody, WireNetworkWaterfallEntry, WireNetworkWaterfallPayload } from './wire-network.js';
export type { WireWebSocketEvent } from './wire-websocket-event.js';
export type { WirePerformanceTiming, WireTypeSummary, WireSlowRequest, WireNetworkSummary, WireLongTaskMetrics, WireUserTimingEntry, WireUserTimingData, WirePerformanceSnapshot } from './wire-performance-snapshot.js';
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
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

// Re-export all message types
export type {
  // Log types
  LogLevel,
  LogLevelFilter,
  LogType,
  BaseLogEntry,
  ConsoleLogEntry,
  NetworkLogEntry,
  ExceptionLogEntry,
  ScreenshotLogEntry,
  LogEntry,
  ProcessedLogEntry,

  // WebSocket types
  WebSocketCaptureMode,
  WebSocketEventType,
  WebSocketEvent,

  // Network types
  WaterfallEntry,
  PendingRequest,
  NetworkBodyPayload,

  // Performance types
  PerformanceMark,
  PerformanceMeasure,
  PerformanceSnapshot,

  // User action types
  ActionType,
  ActionEntry,
  SelectorStrategies,
  EnhancedAction,

  // AI context types
  StackFrame,
  SourceSnippet,
  ReactComponentAncestry,
  AiContextData,

  // Accessibility types
  A11yViolationNode,
  A11yViolation,
  A11yAuditResult,

  // DOM query types
  DomElementInfo,
  DomQueryResult,
  PageInfo,

  // State management types
  BrowserStateSnapshot,
  SavedStateSnapshot,
  StateAction,

  // Background message types
  GetTabIdMessage,
  GetTabIdResponse,
  WsEventMessage,
  EnhancedActionMessage,
  NetworkBodyMessage,
  PerformanceSnapshotMessage,
  LogMessage,
  GetStatusMessage,
  ClearLogsMessage,
  SetLogLevelMessage,
  SetBooleanSettingMessage,
  SetWebSocketCaptureModeMessage,
  GetAiWebPilotEnabledMessage,
  GetAiWebPilotEnabledResponse,
  GetDiagnosticStateMessage,
  GetDiagnosticStateResponse,
  CaptureScreenshotMessage,
  GetDebugLogMessage,
  ClearDebugLogMessage,
  SetServerUrlMessage,
  StatusUpdateMessage,
  DrawModeCompletedMessage,
  BackgroundMessage,

  // Content script message types
  ContentPingMessage,
  ContentPingResponse,
  HighlightMessage,
  HighlightResponse,
  ExecuteJsMessage,
  ExecuteQueryMessage,
  DomQueryMessage,
  A11yQueryMessage,
  GetNetworkWaterfallMessage,
  ManageStateMessage,
  ContentMessage,

  // Inject script message types
  PageMessageType,
  ContentToPageMessageType,
  ExecuteJsResult,

  // State types
  CircuitBreakerState,
  CircuitBreakerStats,
  MemoryPressureLevel,
  MemoryPressureState,
  ConnectionStatus,
  ContextWarning,
  DebugCategory,
  DebugLogEntry,
  ErrorGroup,
  RateLimitResult,
  CaptureScreenshotResult,

  // Pending query types
  QueryType,
  PendingQuery,
  BrowserActionParams,
  BrowserActionResult,
  TabInfo,

  // Source map types
  ParsedSourceMap,
  OriginalLocation,

  // Chrome API wrapper types
  ChromeMessageSender,
  ChromeTabInfo,
  StorageChange,
  StorageAreaName,
  ChromeSessionStorage,
  ChromeStorageWithSession
} from './messages.js'

// Re-export wire types (canonical HTTP payload shapes)
export type { WireEnhancedAction } from './wire/wire-enhanced-action.js'

export type { WireNetworkBody, WireNetworkWaterfallEntry, WireNetworkWaterfallPayload } from './wire/wire-network.js'

export type { WireWebSocketEvent } from './wire/wire-websocket-event.js'

export type {
  WirePerformanceTiming,
  WireTypeSummary,
  WireSlowRequest,
  WireNetworkSummary,
  WireLongTaskMetrics,
  WireUserTimingEntry,
  WireUserTimingData,
  WirePerformanceSnapshot
} from './wire/wire-performance-snapshot.js'

// Re-export all utility types
export type {
  // Generic utility types
  DeepReadonly,
  PartialBy,
  RequiredBy,
  ArrayElement,
  JsonPrimitive,
  JsonArray,
  JsonObject,
  JsonValue,
  Serializable,
  NonNullableFields,
  KeysOfType,
  OmitByType,
  PickByType,

  // Function types
  AsyncFunction,
  Callback,
  ErrorCallback,
  EventHandler,
  DebouncedFunction,

  // Result types
  Result,
  AsyncResult,
  OperationResult,

  // Branded types
  Brand,
  TabId,
  QueryId,
  SessionId,
  CorrelationId,
  Timestamp,

  // Validation types
  ValidatedString,
  ValidatedUrl,

  // Discriminated union helpers
  ExtractByType,
  TypesOf,
  MessageHandlerMap,

  // Element types
  SerializedElementInfo,
  ElementSelector,

  // Configuration types
  ExtensionSettings,
  PartialSettings,
  RateLimitConfig,
  BatcherConfig,

  // Timer types
  TimeoutId,
  IntervalId,
  TimerCleanup,

  // Buffer types
  BufferState,
  MemoryEstimate
} from './utils.js'

// Re-export type guards from utils
export { isObject, isNonEmptyString, hasType, isJsonValue, createTypeGuard } from './utils.js'

// ============================================
// Favicon Replacer Types
// ============================================

/**
 * Tracking state for favicon replacer
 */
export interface TrackingState {
  isTracked: boolean
  aiPilotEnabled: boolean
}

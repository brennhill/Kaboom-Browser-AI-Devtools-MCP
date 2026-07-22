/**
 * Purpose: Defines canonical runtime message envelopes across background, content, inject, and popup contexts.
 * Why: Keeps inter-context communication explicit and compatible as message surfaces evolve.
 * Docs: docs/features/feature/query-service/index.md
 */

/**
 * @fileoverview Runtime Message Types
 * Chrome runtime messages for background, content, and inject script communication
 */

import type { LogEntry, ScreenshotLogEntry } from './telemetry.js'
import type { WebSocketEvent, WebSocketCaptureMode } from './websocket.js'
import type { NetworkBodyPayload, WaterfallEntry } from './network.js'
import type { EnhancedAction } from './actions.js'
import type { PerformanceSnapshot } from './performance.js'
import type { LogLevelFilter } from './telemetry.js'
import type { ConnectionStatus } from './state.js'
import type { BrowserStateSnapshot, StateAction } from './state.js'
import type { DomQueryResult } from './dom.js'
import type { A11yAuditResult } from './accessibility.js'
import type { RuntimeMessageName } from '../lib/constants.js'

// =============================================================================
// BACKGROUND MESSAGE TYPES (chrome.runtime messages)
// =============================================================================

/**
 * Message to get current tab ID
 */
export interface GetTabIdMessage {
  readonly type: 'get_tab_id'
}

export interface GetTabIdResponse {
  readonly tabId?: number
}

/**
 * WebSocket event message from content script
 */
export interface WsEventMessage {
  readonly type: 'ws_event'
  readonly payload: WebSocketEvent
  readonly tabId?: number
}

/**
 * Enhanced action message from content script
 */
export interface EnhancedActionMessage {
  readonly type: 'enhanced_action'
  readonly payload: EnhancedAction
  readonly tabId?: number
}

/**
 * Network body message from content script
 */
export interface NetworkBodyMessage {
  readonly type: 'network_body'
  readonly payload: NetworkBodyPayload
  readonly tabId?: number
}

/**
 * Performance snapshot message from content script
 */
export interface PerformanceSnapshotMessage {
  readonly type: 'performance_snapshot'
  readonly payload: PerformanceSnapshot
  readonly tabId?: number
}

/**
 * Log message from content script
 */
export interface LogMessage {
  readonly type: 'log'
  readonly payload: LogEntry
  readonly tabId?: number
}

/**
 * Get extension status message
 */
export interface GetStatusMessage {
  readonly type: 'get_status'
}

/**
 * Clear logs message
 */
export interface ClearLogsMessage {
  readonly type: 'clear_logs'
}

/**
 * Set log level message
 */
export interface SetLogLevelMessage {
  readonly type: 'set_log_level'
  readonly level: LogLevelFilter
}

/**
 * Toggle boolean setting messages
 */
export interface SetBooleanSettingMessage {
  readonly type:
    | 'set_screenshot_on_error'
    | 'set_ai_web_pilot_enabled'
    | 'set_source_map_enabled'
    | 'set_network_waterfall_enabled'
    | 'set_performance_marks_enabled'
    | 'set_action_replay_enabled'
    | 'set_web_socket_capture_enabled'
    | 'set_performance_snapshot_enabled'
    | 'set_deferral_enabled'
    | 'set_network_body_capture_enabled'
    | 'set_action_toasts_enabled'
    | 'set_subtitles_enabled'
    | 'set_debug_mode'
  readonly enabled: boolean
}

/**
 * Set WebSocket capture mode message
 */
export interface SetWebSocketCaptureModeMessage {
  readonly type: 'set_web_socket_capture_mode'
  readonly mode: WebSocketCaptureMode
}

/**
 * Get AI Web Pilot enabled message
 */
export interface GetAiWebPilotEnabledMessage {
  readonly type: 'get_ai_web_pilot_enabled'
}

export interface GetAiWebPilotEnabledResponse {
  readonly enabled: boolean
}

/**
 * Get tracking state message (for favicon replacer)
 */
interface GetTrackingStateMessage {
  readonly type: 'get_tracking_state'
}

interface GetTrackingStateResponse {
  readonly state: {
    isTracked: boolean
    aiPilotEnabled: boolean
  }
}

/**
 * Get diagnostic state message
 */
export interface GetDiagnosticStateMessage {
  readonly type: 'get_diagnostic_state'
}

export interface GetDiagnosticStateResponse {
  readonly cache: boolean
  readonly storage: boolean | undefined
  readonly timestamp: string
}

/**
 * Capture screenshot message
 */
export interface CaptureScreenshotMessage {
  readonly type: 'capture_screenshot'
}

/**
 * Debug log messages
 */
export interface GetDebugLogMessage {
  readonly type: 'get_debug_log'
}

export interface ClearDebugLogMessage {
  readonly type: 'clear_debug_log'
}

/**
 * Set server URL message
 */
export interface SetServerUrlMessage {
  readonly type: 'set_server_url'
  readonly url: string
}

/**
 * Status update notification (background to popup)
 */
export interface StatusUpdateMessage {
  readonly type: 'status_update'
  readonly status: ConnectionStatus & { aiControlled: boolean }
}

/**
 * Version mismatch notification (background to popup).
 * Fired when extension and server major versions differ.
 */
export interface VersionMismatchMessage {
  readonly type: 'version_mismatch'
  readonly extensionVersion: string
  readonly serverVersion: string
}

/**
 * Union of all background-bound messages
 */
export type BackgroundMessage =
  | GetTabIdMessage
  | WsEventMessage
  | EnhancedActionMessage
  | NetworkBodyMessage
  | PerformanceSnapshotMessage
  | LogMessage
  | GetStatusMessage
  | ClearLogsMessage
  | SetLogLevelMessage
  | SetBooleanSettingMessage
  | SetWebSocketCaptureModeMessage
  | GetAiWebPilotEnabledMessage
  | GetTrackingStateMessage
  | GetDiagnosticStateMessage
  | CaptureScreenshotMessage
  | GetDebugLogMessage
  | ClearDebugLogMessage
  | SetServerUrlMessage
  | DrawModeCaptureScreenshotMessage
  | DrawModeCompletedMessage
  | PushChatMessage
  | ScreenRecordingStartMessage
  | ScreenRecordingStopMessage
  | RecordingGestureGrantedMessage
  | RecordingGestureDeniedMessage
  | OpenPopupForRecordingMessage
  | OpenTerminalPanelMessage
  | QaScanRequestedMessage

/**
 * Draw mode: content script requests screenshot capture
 */
interface DrawModeCaptureScreenshotMessage {
  readonly type: 'kaboom_capture_screenshot'
}

/**
 * Draw mode: content script sends completed annotation results.
 * Fields match the wire format sent by extension/content/draw-mode.js.
 */
export interface DrawModeCompletedMessage {
  readonly type: 'draw_mode_completed'
  readonly annotations?: readonly unknown[]
  readonly screenshot_data_url?: string
  readonly elementDetails?: Readonly<Record<string, unknown>>
  readonly page_url?: string
  readonly correlation_id?: string
  readonly annot_session_name?: string
}

/**
 * Push chat: content script sends a chat message to push to AI.
 */
interface PushChatMessage {
  readonly type: 'kaboom_push_chat'
  readonly message: string
  readonly page_url: string
}

/**
 * Screen recording start (from popup or hover launcher).
 */
interface ScreenRecordingStartMessage {
  readonly type: 'screen_recording_start'
  readonly audio?: string
}

/**
 * Screen recording stop (from popup or hover launcher).
 */
interface ScreenRecordingStopMessage {
  readonly type: 'screen_recording_stop'
}

/**
 * Popup approval for MCP-initiated screen recording request.
 */
interface RecordingGestureGrantedMessage {
  readonly type: 'recording_gesture_granted'
}

/**
 * Popup denial for MCP-initiated screen recording request.
 */
interface RecordingGestureDeniedMessage {
  readonly type: 'recording_gesture_denied'
}

/**
 * Content script requests popup open to activate activeTab for tabCapture.
 */
interface OpenPopupForRecordingMessage {
  readonly type: 'kaboom_open_popup_for_recording'
}

/**
 * Content script requests the side panel terminal to open.
 */
interface OpenTerminalPanelMessage {
  readonly type: 'open_terminal_panel'
}

/**
 * Runtime message forwarded to the side panel terminal host to write text.
 */
export interface TerminalPanelWriteMessage {
  readonly type: 'terminal_panel_write'
  readonly text: string
}

/**
 * User clicked "Audit" in the tracked-site UI.
 * Background handler tries PTY injection, falls back to intent store.
 */
export interface QaScanRequestedMessage {
  readonly type: 'qa_scan_requested'
  readonly page_url?: string
}

/**
 * Toggle chat widget message (background to content).
 */
interface ToggleChatMessage {
  readonly type: 'kaboom_toggle_chat'
  readonly client_name?: string
}

// =============================================================================
// CONTENT SCRIPT MESSAGE TYPES (background to content)
// =============================================================================

/**
 * Ping message to check if content script is loaded
 */
export interface ContentPingMessage {
  readonly type: 'kaboom_ping'
}

export interface ContentPingResponse {
  readonly status: 'alive'
  readonly timestamp: number
}

/**
 * Highlight element message
 */
export interface HighlightMessage {
  readonly type: 'kaboom_highlight'
  readonly params: {
    readonly selector: string
    readonly duration_ms?: number
  }
}

export interface HighlightResponse {
  readonly success: boolean
  readonly selector?: string
  readonly bounds?: {
    readonly x: number
    readonly y: number
    readonly width: number
    readonly height: number
  }
  readonly error?: string
}

/**
 * Execute JavaScript message
 */
export interface ExecuteJsMessage {
  readonly type: 'kaboom_execute_js'
  readonly params: {
    readonly script: string
    readonly timeout_ms?: number
  }
}

/**
 * Execute query message (polling system)
 */
export interface ExecuteQueryMessage {
  readonly type: 'kaboom_execute_query'
  readonly queryId: string
  readonly params: string | Record<string, unknown>
}

/**
 * DOM query message
 */
export interface DomQueryMessage {
  readonly type: 'dom_query'
  readonly params:
    | string
    | {
        readonly selector?: string
        readonly limit?: number
        readonly includeHtml?: boolean
      }
}

/**
 * Accessibility query message
 */
export interface A11yQueryMessage {
  readonly type: 'a11y_query'
  readonly params:
    | string
    | {
        readonly selector?: string
        readonly runOnly?: string[]
      }
}

/**
 * Get network waterfall message
 */
export interface GetNetworkWaterfallMessage {
  readonly type: 'get_network_waterfall'
}

/**
 * Link health check message
 */
interface LinkHealthMessage {
  readonly type: 'link_health_query'
  readonly params?: string | Record<string, unknown>
}

/**
 * Computed styles query message
 */
interface ComputedStylesQueryMessage {
  readonly type: 'computed_styles_query'
  readonly params?: string | Record<string, unknown>
}

/**
 * Form discovery query message
 */
interface FormDiscoveryQueryMessage {
  readonly type: 'form_discovery_query'
  readonly params?: string | Record<string, unknown>
}

/**
 * Form state query message
 */
interface FormStateQueryMessage {
  readonly type: 'form_state_query'
  readonly params?: string | Record<string, unknown>
}

/**
 * Data table query message
 */
interface DataTableQueryMessage {
  readonly type: 'data_table_query'
  readonly params?: string | Record<string, unknown>
}

/**
 * Draw mode control messages (background to content)
 */
interface DrawModeStartMessage {
  readonly type: 'kaboom_draw_mode_start'
  readonly started_by?: string
  readonly annot_session_name?: string
  readonly correlation_id?: string
}

interface DrawModeStopMessage {
  readonly type: 'kaboom_draw_mode_stop'
}

interface GetAnnotationsMessage {
  readonly type: 'kaboom_get_annotations'
}

/**
 * Tracking state change notification (background to content)
 */
export interface TrackingStateChangedMessage {
  readonly type: 'tracking_state_changed'
  readonly state: {
    readonly isTracked: boolean
    readonly aiPilotEnabled: boolean
  }
}

/**
 * State management message
 */
export interface ManageStateMessage {
  readonly type: 'kaboom_manage_state'
  readonly params: {
    readonly action: StateAction
    readonly name?: string
    readonly state?: BrowserStateSnapshot
    readonly include_url?: boolean
  }
}

/**
 * Action toast message — visual indicator for AI actions.
 * Supports color-coded states: trying (blue), success (green), warning (amber), error (red), audio (orange with animation).
 */
interface ActionToastMessage {
  readonly type: 'kaboom_action_toast'
  readonly text: string
  readonly detail?: string
  readonly state?: 'trying' | 'success' | 'warning' | 'error' | 'audio'
  readonly duration_ms?: number
}

/**
 * Subtitle overlay message (persistent narration text)
 */
interface SubtitleMessage {
  readonly type: 'kaboom_subtitle'
  readonly text: string
}

/**
 * Recording watermark overlay message
 */
interface RecordingWatermarkMessage {
  readonly type: 'kaboom_recording_watermark'
  readonly visible: boolean
}

/**
 * Request content launcher re-show after user reopens popup.
 */
export interface ShowTrackedHoverLauncherMessage {
  readonly type: typeof RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER
}

/**
 * Union of all content-script-bound messages
 */
export type ContentMessage =
  | ContentPingMessage
  | HighlightMessage
  | ExecuteJsMessage
  | ExecuteQueryMessage
  | DomQueryMessage
  | A11yQueryMessage
  | GetNetworkWaterfallMessage
  | LinkHealthMessage
  | ComputedStylesQueryMessage
  | FormDiscoveryQueryMessage
  | FormStateQueryMessage
  | DataTableQueryMessage
  | ManageStateMessage
  | ActionToastMessage
  | SubtitleMessage
  | RecordingWatermarkMessage
  | ShowTrackedHoverLauncherMessage
  | DrawModeStartMessage
  | DrawModeStopMessage
  | GetAnnotationsMessage
  | TrackingStateChangedMessage
  | ToggleChatMessage
  | SetBooleanSettingMessage
  | SetWebSocketCaptureModeMessage
  | SetServerUrlMessage

// =============================================================================
// INJECT SCRIPT MESSAGE TYPES (postMessage between content and inject)
// =============================================================================

/**
 * Page to content script messages (postMessage types)
 */
export type PageMessageType =
  | 'kaboom_log'
  | 'kaboom_ws'
  | 'kaboom_network_body'
  | 'kaboom_enhanced_action'
  | 'kaboom_performance_snapshot'
  | 'kaboom_inject_bridge_pong'
  | 'kaboom_highlight_response'
  | 'kaboom_execute_js_result'
  | 'kaboom_a11y_query_response'
  | 'kaboom_dom_query_response'
  | 'kaboom_state_response'
  | 'kaboom_waterfall_response'
  | 'kaboom_link_health_response'
  | 'kaboom_form_state_response'
  | 'kaboom_data_table_response'

/**
 * Content to page messages (postMessage types)
 */
export type ContentToPageMessageType =
  | 'kaboom_setting'
  | 'kaboom_inject_bridge_ping'
  | 'kaboom_highlight_request'
  | 'kaboom_execute_js'
  | 'kaboom_a11y_query'
  | 'kaboom_dom_query'
  | 'kaboom_state_command'
  | 'kaboom_get_waterfall'
  | 'kaboom_link_health_query'
  | 'kaboom_form_state_query'
  | 'kaboom_data_table_query'

// =============================================================================
// OFFSCREEN DOCUMENT MESSAGE TYPES (service worker ↔ offscreen)
// =============================================================================

/**
 * Start recording message (SW → offscreen)
 */
export interface OffscreenStartRecordingMessage {
  readonly target: 'offscreen'
  readonly type: 'offscreen_start_recording'
  readonly streamId: string
  readonly serverUrl: string
  readonly name: string
  readonly fps: number
  readonly audioMode: string
  readonly tabId: number
  readonly url: string
}

/**
 * Stop recording message (SW → offscreen)
 */
export interface OffscreenStopRecordingMessage {
  readonly target: 'offscreen'
  readonly type: 'offscreen_stop_recording'
}

/**
 * Recording started confirmation (offscreen → SW)
 */
export interface OffscreenRecordingStartedMessage {
  readonly target: 'background'
  readonly type: 'offscreen_recording_started'
  readonly success: boolean
  readonly error?: string
}

/**
 * Recording stopped result (offscreen → SW)
 */
export interface OffscreenRecordingStoppedMessage {
  readonly target: 'background'
  readonly type: 'offscreen_recording_stopped'
  readonly status: string
  readonly name: string
  readonly duration_seconds?: number
  readonly size_bytes?: number
  readonly truncated?: boolean
  readonly path?: string
  readonly error?: string
}

/**
 * Union of offscreen messages
 */
export type OffscreenMessage =
  | OffscreenStartRecordingMessage
  | OffscreenStopRecordingMessage
  | OffscreenRecordingStartedMessage
  | OffscreenRecordingStoppedMessage

/**
 * Execute JS result
 */
export interface ExecuteJsResult {
  readonly success: boolean
  readonly result?: unknown
  readonly error?: string
  readonly message?: string
  readonly stack?: string
}

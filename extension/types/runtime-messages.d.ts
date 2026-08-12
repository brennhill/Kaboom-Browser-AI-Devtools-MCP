/**
 * Purpose: Defines canonical runtime message envelopes across background, content, inject, and popup contexts.
 * Why: Keeps inter-context communication explicit and compatible as message surfaces evolve.
 * Docs: docs/features/feature/query-service/index.md
 */
import type { WebSocketCaptureMode } from './capture/websocket.js';
import type { WsEventMessage, EnhancedActionMessage, NetworkBodyMessage, PerformanceSnapshotMessage, CaptureDiagnosticMessage, LogMessage } from './runtime/telemetry-messages.js';
import type { LogLevelFilter } from './capture/telemetry.js';
import type { ConnectionStatus } from './runtime/state.js';
import type { BrowserStateSnapshot, StateAction } from './runtime/state.js';
import type { GetTrackingStateMessage, TrackingContentReadyMessage, TrackingReadinessProbeMessage, TrackingContinuityChangedMessage, TrackingStateChangedMessage } from './runtime/tracking.js';
import type { RuntimeMessageName } from '../lib/constants.js';
/**
 * Message to get current tab ID
 */
export interface GetTabIdMessage {
    readonly type: 'get_tab_id';
}
export interface GetTabIdResponse {
    readonly tabId?: number;
}
/**
 * WebSocket event message from content script
 */
/**
 * Enhanced action message from content script
 */
/**
 * Network body message from content script
 */
/**
 * Performance snapshot message from content script
 */
/**
 * Log message from content script
 */
/**
 * Get extension status message
 */
export interface GetStatusMessage {
    readonly type: 'get_status';
}
/** Clear logs message. */
export interface ClearLogsMessage {
    readonly type: 'clear_logs';
}
export interface StateRecoveryDiagnostic {
    readonly name: string;
    readonly detail: string;
    readonly fix: string;
    readonly correlation_id?: string;
    readonly expected_next_transition?: string;
    readonly deadline?: string;
    readonly recovery_attempt?: number;
    readonly recovery_outcome?: string;
}
export type StateRecoveryLifecycle = 'active' | 'recovered';
export interface ReportStateRecoveryMessage {
    readonly type: 'report_state_recovery';
    readonly lifecycle: StateRecoveryLifecycle;
    readonly diagnostic: StateRecoveryDiagnostic;
}
/**
 * Set log level message
 */
export interface SetLogLevelMessage {
    readonly type: 'set_log_level';
    readonly level: LogLevelFilter;
}
/**
 * Toggle boolean setting messages
 */
export interface SetBooleanSettingMessage {
    readonly type: 'set_screenshot_on_error' | 'set_ai_web_pilot_enabled' | 'set_source_map_enabled' | 'set_network_waterfall_enabled' | 'set_performance_marks_enabled' | 'set_action_replay_enabled' | 'set_web_socket_capture_enabled' | 'set_performance_snapshot_enabled' | 'set_deferral_enabled' | 'set_network_body_capture_enabled' | 'set_action_toasts_enabled' | 'set_subtitles_enabled' | 'set_debug_mode';
    readonly enabled: boolean;
}
/**
 * Set WebSocket capture mode message
 */
export interface SetWebSocketCaptureModeMessage {
    readonly type: 'set_web_socket_capture_mode';
    readonly mode: WebSocketCaptureMode;
}
/**
 * Get AI Web Pilot enabled message
 */
export interface GetAiWebPilotEnabledMessage {
    readonly type: 'get_ai_web_pilot_enabled';
}
export interface GetAiWebPilotEnabledResponse {
    readonly enabled: boolean;
}
/**
 * Get diagnostic state message
 */
export interface GetDiagnosticStateMessage {
    readonly type: 'get_diagnostic_state';
}
export interface GetDiagnosticStateResponse {
    readonly cache: boolean;
    readonly storage: boolean | undefined;
    readonly timestamp: string;
}
/**
 * Capture screenshot message
 */
export interface CaptureScreenshotMessage {
    readonly type: 'capture_screenshot';
}
/**
 * Track a UI-originated feature use (background usage counter).
 *
 * Non-background entry points (popup buttons, the in-page hover launcher) that
 * trigger a feature directly against the content script — bypassing the
 * background — send this so `trackUIFeature` still counts them. Background entry
 * points (keyboard, context menu) call `trackUIFeature` directly and do not need
 * it. AI/MCP paths deliberately never send it.
 *
 * The canonical feature key union is generated from the Go /sync wire schema.
 */
export interface TrackUiFeatureMessage {
    readonly type: 'track_ui_feature';
    readonly feature: import('./wire/wire-sync.js').SyncUIFeature;
}
/**
 * Debug log messages
 */
export interface GetDebugLogMessage {
    readonly type: 'get_debug_log';
}
export interface ClearDebugLogMessage {
    readonly type: 'clear_debug_log';
}
/**
 * Set server URL message
 */
export interface SetServerUrlMessage {
    readonly type: 'set_server_url';
    readonly url: string;
}
/**
 * Status update notification (background to popup)
 */
export interface StatusUpdateMessage {
    readonly type: 'status_update';
    readonly status: ConnectionStatus & {
        aiControlled: boolean;
    };
}
/**
 * Version mismatch notification (background to popup).
 * Fired when extension and server major versions differ.
 */
export interface VersionMismatchMessage {
    readonly type: 'version_mismatch';
    readonly extensionVersion: string;
    readonly serverVersion: string;
}
/**
 * Union of all background-bound messages
 */
export type BackgroundMessage = GetTabIdMessage | WsEventMessage | EnhancedActionMessage | NetworkBodyMessage | PerformanceSnapshotMessage | CaptureDiagnosticMessage | LogMessage | GetStatusMessage | ClearLogsMessage | ReportStateRecoveryMessage | SetLogLevelMessage | SetBooleanSettingMessage | SetWebSocketCaptureModeMessage | GetAiWebPilotEnabledMessage | GetTrackingStateMessage | TrackingContentReadyMessage | TrackingContinuityChangedMessage | GetDiagnosticStateMessage | CaptureScreenshotMessage | TrackUiFeatureMessage | GetDebugLogMessage | ClearDebugLogMessage | SetServerUrlMessage | DrawModeCaptureScreenshotMessage | DrawModeCompletedMessage | PushChatMessage | ScreenRecordingStartMessage | ScreenRecordingStopMessage | RecordingGestureGrantedMessage | RecordingGestureDeniedMessage | OpenPopupForRecordingMessage | OpenTerminalPanelMessage | CloseTerminalPanelMessage | QaScanRequestedMessage;
/**
 * Draw mode: content script requests screenshot capture
 */
interface DrawModeCaptureScreenshotMessage {
    readonly type: 'kaboom_capture_screenshot';
}
/**
 * Draw mode: content script sends completed annotation results.
 * Fields match the wire format sent by extension/content/draw-mode.js.
 */
export interface DrawModeCompletedMessage {
    readonly type: 'draw_mode_completed';
    readonly annotations?: readonly unknown[];
    readonly screenshot_data_url?: string;
    readonly elementDetails?: Readonly<Record<string, unknown>>;
    readonly page_url?: string;
    readonly correlation_id?: string;
    readonly annot_session_name?: string;
}
/**
 * Push chat: content script sends a chat message to push to AI.
 */
interface PushChatMessage {
    readonly type: 'kaboom_push_chat';
    readonly message: string;
    readonly page_url: string;
}
/**
 * Screen recording start (from popup or hover launcher).
 */
interface ScreenRecordingStartMessage {
    readonly type: 'screen_recording_start';
    readonly audio?: string;
}
/**
 * Screen recording stop (from popup or hover launcher).
 */
interface ScreenRecordingStopMessage {
    readonly type: 'screen_recording_stop';
}
/**
 * Popup approval for MCP-initiated screen recording request.
 */
interface RecordingGestureGrantedMessage {
    readonly type: 'recording_gesture_granted';
}
/**
 * Popup denial for MCP-initiated screen recording request.
 */
interface RecordingGestureDeniedMessage {
    readonly type: 'recording_gesture_denied';
}
/**
 * Content script requests popup open to activate activeTab for tabCapture.
 */
interface OpenPopupForRecordingMessage {
    readonly type: 'kaboom_open_popup_for_recording';
}
/**
 * Content script requests the side panel terminal to open.
 */
interface OpenTerminalPanelMessage {
    readonly type: 'open_terminal_panel';
    /** Tab that should host the panel. Required from the popup (no sender.tab). */
    readonly tab_id?: number;
}
/**
 * Background asks the side panel document to close itself.
 *
 * The background cannot close a side panel on every Chrome version
 * (`chrome.sidePanel.close` is very new), but the panel document can call
 * `window.close()`. Closing this way leaves the shell running.
 */
interface CloseTerminalPanelMessage {
    readonly type: 'close_terminal_panel';
}
/**
 * Background asks an existing side panel document to show the terminal again.
 *
 * Sent over the presence port (TERMINAL_PANEL_PORT), not runtime.sendMessage.
 * `chrome.sidePanel.open()` on a panel that already exists merely focuses it and
 * runs no code inside it, so a minimized or unmounted panel would stay blank and
 * "open" would appear to do nothing.
 */
export interface RestoreTerminalPanelMessage {
    readonly type: 'restore_terminal_panel';
}
/**
 * Runtime message forwarded to the side panel terminal host to write text.
 */
export interface TerminalPanelWriteMessage {
    readonly type: 'terminal_panel_write';
    readonly text: string;
}
/**
 * Acknowledgement the side-panel terminal host returns for a
 * `terminal_panel_write`. The background never responds to this type, so this ack
 * is the sole reliable signal that a panel DOCUMENT actually received the write —
 * letting the sender tell a delivered write from one that vanished because the
 * panel was closed (e.g. via Chrome's own X) while the visibility mirror it gated
 * on stayed stale. Its absence means "no panel received it" (fail-loud, rule 25).
 */
export interface TerminalPanelWriteResponse {
    readonly received: boolean;
}
/**
 * User clicked "Audit" in the tracked-site UI.
 * Background handler tries PTY injection, falls back to intent store.
 */
export interface QaScanRequestedMessage {
    readonly type: 'qa_scan_requested';
    readonly page_url?: string;
}
/**
 * Toggle chat widget message (background to content).
 */
interface ToggleChatMessage {
    readonly type: 'kaboom_toggle_chat';
    readonly client_name?: string;
}
/**
 * Ping message to check if content script is loaded
 */
export interface ContentPingMessage {
    readonly type: 'kaboom_ping';
}
export interface ContentPingResponse {
    readonly status: 'alive';
    readonly timestamp: number;
}
/**
 * Highlight element message
 */
export interface HighlightMessage {
    readonly type: 'kaboom_highlight';
    readonly params: {
        readonly selector: string;
        readonly duration_ms?: number;
    };
}
export interface HighlightResponse {
    readonly success: boolean;
    readonly selector?: string;
    readonly bounds?: {
        readonly x: number;
        readonly y: number;
        readonly width: number;
        readonly height: number;
    };
    readonly error?: string;
}
/**
 * Execute JavaScript message
 */
export interface ExecuteJsMessage {
    readonly type: 'kaboom_execute_js';
    readonly params: {
        readonly script: string;
        readonly timeout_ms?: number;
    };
}
/**
 * Execute query message (polling system)
 */
export interface ExecuteQueryMessage {
    readonly type: 'kaboom_execute_query';
    readonly queryId: string;
    readonly params: string | Record<string, unknown>;
}
/**
 * DOM query message
 */
export interface DomQueryMessage {
    readonly type: 'dom_query';
    readonly params: string | {
        readonly selector?: string;
        readonly limit?: number;
        readonly includeHtml?: boolean;
    };
}
/**
 * Accessibility query message
 */
export interface A11yQueryMessage {
    readonly type: 'a11y_query';
    readonly params: string | {
        readonly selector?: string;
        readonly runOnly?: string[];
    };
}
/**
 * Get network waterfall message
 */
export interface GetNetworkWaterfallMessage {
    readonly type: 'get_network_waterfall';
}
/**
 * Link health check message
 */
interface LinkHealthMessage {
    readonly type: 'link_health_query';
    readonly params?: string | Record<string, unknown>;
}
/**
 * Computed styles query message
 *
 * params: { selector, properties?, max_elements?, include_custom_properties? }
 * The response is a WireStyleProbeResult (src/types/wire/wire-style-probe.ts),
 * which carries the matched elements plus the truncation facts and, when
 * requested, the CSS custom-property tables the design-drift analyzers match
 * against. This message is the single probe surface — design_audit extends it
 * rather than adding a parallel query.
 */
interface ComputedStylesQueryMessage {
    readonly type: 'computed_styles_query';
    readonly params?: string | Record<string, unknown>;
}
/**
 * Form discovery query message
 */
interface FormDiscoveryQueryMessage {
    readonly type: 'form_discovery_query';
    readonly params?: string | Record<string, unknown>;
}
/**
 * Form state query message
 */
interface FormStateQueryMessage {
    readonly type: 'form_state_query';
    readonly params?: string | Record<string, unknown>;
}
/**
 * Data table query message
 */
interface DataTableQueryMessage {
    readonly type: 'data_table_query';
    readonly params?: string | Record<string, unknown>;
}
/**
 * Draw mode control messages (background to content)
 */
interface DrawModeStartMessage {
    readonly type: 'kaboom_draw_mode_start';
    readonly started_by?: string;
    readonly annot_session_name?: string;
    readonly correlation_id?: string;
}
interface DrawModeStopMessage {
    readonly type: 'kaboom_draw_mode_stop';
}
interface GetAnnotationsMessage {
    readonly type: 'kaboom_get_annotations';
}
/**
 * State management message
 */
export interface ManageStateMessage {
    readonly type: 'kaboom_manage_state';
    readonly params: {
        readonly action: StateAction;
        readonly name?: string;
        readonly state?: BrowserStateSnapshot;
        readonly include_url?: boolean;
    };
}
/**
 * Action toast message — visual indicator for AI actions.
 * Supports color-coded states: trying (blue), success (green), warning (amber), error (red), audio (orange with animation).
 */
interface ActionToastMessage {
    readonly type: 'kaboom_action_toast';
    readonly text: string;
    readonly detail?: string;
    readonly state?: 'trying' | 'success' | 'warning' | 'error' | 'audio';
    readonly duration_ms?: number;
}
/**
 * Subtitle overlay message (persistent narration text)
 */
interface SubtitleMessage {
    readonly type: 'kaboom_subtitle';
    readonly text: string;
}
/**
 * Recording watermark overlay message
 */
interface RecordingWatermarkMessage {
    readonly type: 'kaboom_recording_watermark';
    readonly visible: boolean;
}
/**
 * Request content launcher re-show after user reopens popup.
 */
export interface ShowTrackedHoverLauncherMessage {
    readonly type: typeof RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER;
}
/**
 * Union of all content-script-bound messages
 */
export type ContentMessage = ContentPingMessage | TrackingReadinessProbeMessage | HighlightMessage | ExecuteJsMessage | ExecuteQueryMessage | DomQueryMessage | A11yQueryMessage | GetNetworkWaterfallMessage | LinkHealthMessage | ComputedStylesQueryMessage | FormDiscoveryQueryMessage | FormStateQueryMessage | DataTableQueryMessage | ManageStateMessage | ActionToastMessage | SubtitleMessage | RecordingWatermarkMessage | ShowTrackedHoverLauncherMessage | DrawModeStartMessage | DrawModeStopMessage | GetAnnotationsMessage | TrackingStateChangedMessage | ToggleChatMessage | SetBooleanSettingMessage | SetWebSocketCaptureModeMessage | SetServerUrlMessage;
/** Page-to-content postMessage types. */
export type PageMessageType = 'kaboom_log' | 'kaboom_ws' | 'kaboom_network_body' | 'kaboom_enhanced_action' | 'kaboom_performance_snapshot' | 'kaboom_capture_diagnostic' | 'kaboom_inject_bridge_pong' | 'kaboom_highlight_response' | 'kaboom_execute_js_result' | 'kaboom_a11y_query_response' | 'kaboom_dom_query_response' | 'kaboom_state_response' | 'kaboom_waterfall_response' | 'kaboom_link_health_response' | 'kaboom_computed_styles_response' | 'kaboom_form_discovery_response' | 'kaboom_form_state_response' | 'kaboom_data_table_response';
/** Content-to-page postMessage types. */
export type ContentToPageMessageType = 'kaboom_setting' | 'kaboom_inject_bridge_ping' | 'kaboom_highlight_request' | 'kaboom_execute_js' | 'kaboom_a11y_query' | 'kaboom_dom_query' | 'kaboom_state_command' | 'kaboom_get_waterfall' | 'kaboom_link_health_query' | 'kaboom_computed_styles_query' | 'kaboom_form_discovery_query' | 'kaboom_form_state_query' | 'kaboom_data_table_query';
export interface PageMessageEventData {
    readonly _nonce: string;
    readonly type?: PageMessageType;
    readonly requestId?: number | string;
    readonly result?: unknown;
    readonly payload?: unknown;
}
/**
 * Start recording message (SW → offscreen)
 */
export interface OffscreenStartRecordingMessage {
    readonly target: 'offscreen';
    readonly type: 'offscreen_start_recording';
    readonly streamId: string;
    readonly serverUrl: string;
    readonly name: string;
    readonly fps: number;
    readonly audioMode: string;
    readonly tabId: number;
    readonly url: string;
    readonly connection_generation?: number;
}
/**
 * Stop recording message (SW → offscreen)
 */
export interface OffscreenStopRecordingMessage {
    readonly target: 'offscreen';
    readonly type: 'offscreen_stop_recording';
    readonly connection_generation?: number;
}
/**
 * Query live recording state (SW → offscreen).
 * Sent on service-worker startup so a restarted SW can rehydrate in-memory
 * recording state while the offscreen MediaRecorder is still running.
 */
export interface OffscreenGetRecordingStateMessage {
    readonly target: 'offscreen';
    readonly type: 'offscreen_get_recording_state';
}
/**
 * Live recording state (offscreen → SW, sendResponse payload for
 * OffscreenGetRecordingStateMessage).
 */
export interface OffscreenRecordingStateResponse {
    readonly active: boolean;
    readonly name: string;
    readonly startTime: number;
    readonly fps: number;
    readonly audioMode: string;
    readonly tabId: number;
    readonly url: string;
}
/**
 * Recording started confirmation (offscreen → SW)
 */
export interface OffscreenRecordingStartedMessage {
    readonly target: 'background';
    readonly type: 'offscreen_recording_started';
    readonly success: boolean;
    readonly error?: string;
    readonly connection_generation?: number;
}
/**
 * Recording stopped result (offscreen → SW)
 */
export interface OffscreenRecordingStoppedMessage {
    readonly target: 'background';
    readonly type: 'offscreen_recording_stopped';
    readonly status: string;
    readonly name: string;
    readonly duration_seconds?: number;
    readonly size_bytes?: number;
    readonly truncated?: boolean;
    readonly path?: string;
    readonly error?: string;
    readonly connection_generation?: number;
}
/**
 * Union of offscreen messages
 */
export type OffscreenMessage = OffscreenStartRecordingMessage | OffscreenStopRecordingMessage | OffscreenGetRecordingStateMessage | OffscreenRecordingStartedMessage | OffscreenRecordingStoppedMessage;
/**
 * Execute JS result
 */
export interface ExecuteJsResult {
    readonly success: boolean;
    readonly result?: unknown;
    readonly error?: string;
    readonly message?: string;
    readonly stack?: string;
}
export {};
//# sourceMappingURL=runtime-messages.d.ts.map
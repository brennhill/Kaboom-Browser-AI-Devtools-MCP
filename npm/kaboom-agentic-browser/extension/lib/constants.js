/**
 * Purpose: Shared constants (server defaults, serialization limits, buffer sizes, storage keys, feature toggles) used across all extension modules.
 */
/**
 * @fileoverview Shared constants for the Kaboom extension capture modules.
 */
import { scaleTimeout } from './timeouts.js';
// Chrome DevTools Protocol version used for debugger.attach calls
export const CDP_VERSION = '1.3';
// Server defaults
export const DEFAULT_SERVER_URL = 'http://localhost:7890';
// Terminal server runs on a dedicated port (main port + offset) for isolation.
export const TERMINAL_PORT_OFFSET = 1;
// Serialization limits
export const MAX_STRING_LENGTH = 10240; // 10KB
export const MAX_RESPONSE_LENGTH = 5120; // 5KB
export const MAX_DEPTH = 10;
export const MAX_CONTEXT_SIZE = 50; // Max number of context keys
export const MAX_CONTEXT_VALUE_SIZE = 4096; // Max size of serialized context value
export const SENSITIVE_HEADERS = [
    'authorization',
    'cookie',
    'set-cookie',
    'x-auth-token',
    'x-api-key',
    'x-csrf-token',
    'proxy-authorization'
];
// User action replay settings
export const MAX_ACTION_BUFFER_SIZE = 20; // Max number of recent actions to keep
export const SCROLL_THROTTLE_MS = 250; // Throttle scroll events
export const SENSITIVE_INPUT_TYPES = ['password', 'credit-card', 'cc-number', 'cc-exp', 'cc-csc'];
// Network Waterfall settings
export const MAX_WATERFALL_ENTRIES = 50; // Max network entries to capture
export const WATERFALL_TIME_WINDOW_MS = 30000; // Only capture last 30 seconds
// Performance Marks settings
export const MAX_PERFORMANCE_ENTRIES = 50; // Max performance entries to capture
export const PERFORMANCE_TIME_WINDOW_MS = 60000; // Only capture last 60 seconds
// WebSocket capture settings
export const WS_MAX_BODY_SIZE = 4096; // 4KB truncation limit
export const WS_PREVIEW_LIMIT = 200; // Preview character limit
// Network body capture settings
export const REQUEST_BODY_MAX = 8192; // 8KB
export const RESPONSE_BODY_MAX = 16384; // 16KB
// Intentionally aggressive (5ms) to avoid blocking the main thread during fetch body reads.
// Network body capture uses this as a race timeout - if the body isn't available nearly
// instantly, we skip it rather than degrade page performance.
export const BODY_READ_TIMEOUT_MS = 5;
export const SENSITIVE_HEADER_PATTERNS = /^(authorization|cookie|set-cookie|x-api-key|x-auth-token|x-secret|x-password|.*token.*|.*secret.*|.*key.*|.*password.*)$/i;
export const BINARY_CONTENT_TYPES = /^(image|video|audio|font)\/|^application\/(wasm|octet-stream|zip|gzip|pdf)/;
// DOM query settings
export const DOM_QUERY_MAX_ELEMENTS = 50;
export const DOM_QUERY_MAX_TEXT = 500;
export const DOM_QUERY_MAX_DEPTH = 5;
export const DOM_QUERY_MAX_HTML = 200;
export const A11Y_MAX_NODES_PER_VIOLATION = 10;
export const ASYNC_COMMAND_TIMEOUT_MS = scaleTimeout(60000);
export const A11Y_AUDIT_TIMEOUT_MS = ASYNC_COMMAND_TIMEOUT_MS;
// Memory pressure settings
export const MEMORY_SOFT_LIMIT_MB = 20;
export const MEMORY_HARD_LIMIT_MB = 50;
// AI Context settings
export const AI_CONTEXT_SNIPPET_LINES = 5; // Lines before and after error
export const AI_CONTEXT_MAX_LINE_LENGTH = 200; // Truncate lines
export const AI_CONTEXT_MAX_SNIPPETS_SIZE = 10240; // 10KB total snippets
export const AI_CONTEXT_MAX_ANCESTRY_DEPTH = 10;
export const AI_CONTEXT_MAX_PROP_KEYS = 20;
export const AI_CONTEXT_MAX_STATE_KEYS = 10;
export const AI_CONTEXT_MAX_RELEVANT_SLICE = 10;
export const AI_CONTEXT_MAX_VALUE_LENGTH = 200;
export const AI_CONTEXT_SOURCE_MAP_CACHE_SIZE = 20;
export const AI_CONTEXT_PIPELINE_TIMEOUT_MS = scaleTimeout(3000);
// Reproduction script settings
export const ENHANCED_ACTION_BUFFER_SIZE = 50;
export const CSS_PATH_MAX_DEPTH = 5;
export const SELECTOR_TEXT_MAX_LENGTH = 50;
export const SCRIPT_MAX_SIZE = 51200; // 50KB
export const CLICKABLE_TAGS = new Set(['BUTTON', 'A', 'SUMMARY']);
// Actionable keys for recording
export const ACTIONABLE_KEYS = new Set([
    'Enter',
    'Escape',
    'Tab',
    'ArrowUp',
    'ArrowDown',
    'ArrowLeft',
    'ArrowRight',
    'Backspace',
    'Delete'
]);
// Performance snapshot settings
export const MAX_LONG_TASKS = 50;
export const MAX_SLOWEST_REQUESTS = 3;
export const MAX_URL_LENGTH = 80;
// =============================================================================
// SETTING NAMES — Single source of truth for all toggle/setting message types.
// Background, content, inject, and popup layers all import from here.
// =============================================================================
/**
 * Setting names used across background, content, and inject contexts.
 * Single source of truth — all layers import from here.
 *
 * Note: These are the RUNTIME string values sent as message `type` fields.
 * The TYPE-LEVEL literals in runtime-messages.ts (SetBooleanSettingMessage etc.)
 * are deliberately kept as literal strings for TypeScript discriminated union narrowing.
 */
export const SettingName = {
    NETWORK_WATERFALL: 'set_network_waterfall_enabled',
    PERFORMANCE_MARKS: 'set_performance_marks_enabled',
    ACTION_REPLAY: 'set_action_replay_enabled',
    WEBSOCKET_CAPTURE: 'set_web_socket_capture_enabled',
    WEBSOCKET_CAPTURE_MODE: 'set_web_socket_capture_mode',
    PERFORMANCE_SNAPSHOT: 'set_performance_snapshot_enabled',
    DEFERRAL: 'set_deferral_enabled',
    NETWORK_BODY_CAPTURE: 'set_network_body_capture_enabled',
    ACTION_TOASTS: 'set_action_toasts_enabled',
    SUBTITLES: 'set_subtitles_enabled',
    SERVER_URL: 'set_server_url'
};
/** All valid setting names as a Set (for runtime validation) */
const VALID_SETTING_NAMES = new Set(Object.values(SettingName));
// =============================================================================
// RUNTIME MESSAGE NAMES — Shared one-off message names used across contexts.
// =============================================================================
export const RuntimeMessageName = {
    SHOW_TRACKED_HOVER_LAUNCHER: 'kaboom_show_tracked_hover_launcher'
};
/**
 * Settings forwarded from background -> content -> inject (MAIN world).
 * These are the settings that the inject script knows how to handle.
 * Content-only settings (ACTION_TOASTS, SUBTITLES) are NOT in this set —
 * they are handled directly by the content script runtime-message-listener.
 */
export const INJECT_FORWARDED_SETTINGS = new Set([
    SettingName.NETWORK_WATERFALL,
    SettingName.PERFORMANCE_MARKS,
    SettingName.ACTION_REPLAY,
    SettingName.WEBSOCKET_CAPTURE,
    SettingName.WEBSOCKET_CAPTURE_MODE,
    SettingName.PERFORMANCE_SNAPSHOT,
    SettingName.DEFERRAL,
    SettingName.NETWORK_BODY_CAPTURE,
    SettingName.SERVER_URL
]);
// =============================================================================
// STORAGE KEYS — Single source of truth for chrome.storage key strings.
// =============================================================================
/**
 * Chrome storage key strings used in chrome.storage.local.get/set/remove calls.
 * Single source of truth — all layers import from here.
 */
export const StorageKey = {
    TRACKED_TAB_ID: 'trackedTabId',
    TRACKED_TAB_URL: 'trackedTabUrl',
    TRACKED_TAB_TITLE: 'trackedTabTitle',
    AI_WEB_PILOT_ENABLED: 'aiWebPilotEnabled',
    DEBUG_MODE: 'debugMode',
    SERVER_URL: 'serverUrl',
    SCREENSHOT_ON_ERROR: 'screenshotOnError',
    SOURCE_MAP_ENABLED: 'sourceMapEnabled',
    LOG_LEVEL: 'logLevel',
    THEME: 'theme',
    DEFERRAL_ENABLED: 'deferralEnabled',
    WEBSOCKET_CAPTURE_ENABLED: 'webSocketCaptureEnabled',
    WEBSOCKET_CAPTURE_MODE: 'webSocketCaptureMode',
    NETWORK_WATERFALL_ENABLED: 'networkWaterfallEnabled',
    PERFORMANCE_MARKS_ENABLED: 'performanceMarksEnabled',
    ACTION_REPLAY_ENABLED: 'actionReplayEnabled',
    NETWORK_BODY_CAPTURE_ENABLED: 'networkBodyCaptureEnabled',
    ACTION_TOASTS_ENABLED: 'actionToastsEnabled',
    SUBTITLES_ENABLED: 'subtitlesEnabled',
    ACTION_RECORDING: 'kaboom_action_recording',
    RECORDING: 'kaboom_recording',
    TRACKED_HOVER_LAUNCHER_HIDDEN: 'kaboom_tracked_hover_launcher_hidden',
    PENDING_RECORDING: 'kaboom_pending_recording',
    PENDING_MIC_RECORDING: 'kaboom_pending_mic_recording',
    MIC_GRANTED: 'kaboom_mic_granted',
    RECORD_AUDIO_PREF: 'kaboom_record_audio_pref',
    TERMINAL_CONFIG: 'kaboom_terminal_config',
    TERMINAL_AI_COMMAND: 'kaboom_terminal_ai_command',
    TERMINAL_DEV_ROOT: 'kaboom_terminal_dev_root',
    POPUP_LAST_STATUS: 'kaboom_popup_last_status',
    TERMINAL_SESSION: 'kaboom_terminal_session',
    TERMINAL_UI_STATE: 'kaboom_terminal_ui_state',
    TERMINAL_WORKSPACE_GROUP_ID: 'kaboom_terminal_workspace_group_id',
    TERMINAL_WORKSPACE_MAIN_TAB_ID: 'kaboom_terminal_workspace_main_tab_id',
    CLOAKED_DOMAINS: 'kaboom_cloaked_domains',
    ERROR_GROUPS: 'kaboom_error_groups'
};
//# sourceMappingURL=constants.js.map
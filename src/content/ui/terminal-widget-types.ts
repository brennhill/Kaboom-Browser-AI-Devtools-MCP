/**
 * Purpose: Shared constants, types, and mutable state for the terminal widget.
 * Why: Centralises state and constants so split modules reference the same values
 *      without circular dependencies.
 * Docs: docs/features/feature/terminal/index.md
 */

import { DEFAULT_SERVER_URL, TERMINAL_PORT_OFFSET } from '../../lib/constants.js'

// ---------------------------------------------------------------------------
// DOM element IDs
// ---------------------------------------------------------------------------
export const WIDGET_ID = 'kaboom-terminal-widget'
export const IFRAME_ID = 'kaboom-terminal-iframe'
export const HEADER_ID = 'kaboom-terminal-header'
export const TERMINAL_BODY_ID = 'kaboom-terminal-body'
export const DISCONNECT_TERMINAL_BUTTON_ID = 'kaboom-terminal-disconnect-button'
export const ANNOTATE_TERMINAL_BUTTON_ID = 'kaboom-terminal-annotate-button'
export const REDRAW_TERMINAL_BUTTON_ID = 'kaboom-terminal-redraw-button'
export const MINIMIZE_TERMINAL_BUTTON_ID = 'kaboom-terminal-minimize-button'
export const CLOSE_TERMINAL_BUTTON_ID = 'kaboom-terminal-close-button'
export const START_TERMINAL_BUTTON_ID = 'kaboom-terminal-start-button'
export const ROOT_FOLDER_INPUT_ID = 'kaboom-terminal-root-folder-input'
export const ROOT_FOLDER_SAVE_BUTTON_ID = 'kaboom-terminal-root-folder-save'
export const ROOT_FOLDER_BAR_ID = 'kaboom-terminal-root-folder-bar'
export const ROOT_FOLDER_BROWSE_BUTTON_ID = 'kaboom-terminal-root-folder-browse'
export const ROOT_FOLDER_PICKER_ID = 'kaboom-terminal-root-folder-picker'
export const ROOT_FOLDER_PICKER_UP_ID = 'kaboom-terminal-root-folder-up'
export const ROOT_FOLDER_PICKER_USE_ID = 'kaboom-terminal-root-folder-use'

// ---------------------------------------------------------------------------
// Timing constants
// ---------------------------------------------------------------------------
export const TERMINAL_WRITE_SUBMIT_DELAY_MS = 600
export const TERMINAL_TYPING_IDLE_MS = 1500
export const TERMINAL_GUARD_POLL_MS = 200
export const TERMINAL_GUARD_TOAST_INTERVAL_MS = 3000

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
export interface TerminalConfig {
  cmd?: string
  args?: string[]
  dir?: string
  serverUrl?: string
}

export interface TerminalSessionState {
  token: string
  sessionId: string
}

export type TerminalUIState = 'open' | 'closed' | 'minimized'

// ---------------------------------------------------------------------------
// Shared mutable state — single object so every module sees the same values.
// ---------------------------------------------------------------------------
export interface TerminalWidgetState {
  widgetEl: HTMLDivElement | null
  iframeEl: HTMLIFrameElement | null
  sessionState: TerminalSessionState | null
  visible: boolean
  serverUrl: string
  terminalFocused: boolean
  lastTypingAt: number
  queuedWrites: string[]
  queuedWriteFlushTimer: ReturnType<typeof setTimeout> | null
  queuedSubmitTimer: ReturnType<typeof setTimeout> | null
  queuedWriteInFlight: boolean
  lastGuardToastAt: number
  terminalConnected: boolean
}

export const state: TerminalWidgetState = {
  widgetEl: null,
  iframeEl: null,
  sessionState: null,
  visible: false,
  serverUrl: DEFAULT_SERVER_URL,
  terminalFocused: false,
  lastTypingAt: 0,
  queuedWrites: [],
  queuedWriteFlushTimer: null,
  queuedSubmitTimer: null,
  queuedWriteInFlight: false,
  lastGuardToastAt: 0,
  terminalConnected: false
}

/** Reset all mutable state to initial values. Used by tests to isolate module-cached state. */
export function resetAllState(): void {
  state.widgetEl = null
  state.iframeEl = null
  state.sessionState = null
  state.visible = false
  state.serverUrl = DEFAULT_SERVER_URL
  state.terminalFocused = false
  state.lastTypingAt = 0
  state.queuedWrites = []
  if (state.queuedWriteFlushTimer !== null) clearTimeout(state.queuedWriteFlushTimer)
  state.queuedWriteFlushTimer = null
  if (state.queuedSubmitTimer !== null) clearTimeout(state.queuedSubmitTimer)
  state.queuedSubmitTimer = null
  state.queuedWriteInFlight = false
  state.lastGuardToastAt = 0
  state.terminalConnected = false
}

// ---------------------------------------------------------------------------
// Utility: compute terminal server URL from a base daemon URL.
// ---------------------------------------------------------------------------
/** Compute the terminal server URL from a base daemon URL (port + TERMINAL_PORT_OFFSET). */
export function getTerminalServerUrl(baseUrl: string): string {
  const url = new URL(baseUrl)
  url.port = String(parseInt(url.port || '7890', 10) + TERMINAL_PORT_OFFSET)
  return url.origin
}

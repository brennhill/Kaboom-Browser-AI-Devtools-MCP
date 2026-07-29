/**
 * Purpose: Shared constants, types, and mutable state for the terminal widget.
 * Why: Centralises state and constants so split modules reference the same values
 *      without circular dependencies.
 * Docs: docs/features/feature/terminal/index.md
 */

import { DEFAULT_SERVER_URL } from '../../lib/constants.js'
import { resetTerminalPortDiscovery } from '../../lib/terminal-server.js'

// ---------------------------------------------------------------------------
// DOM element IDs
// ---------------------------------------------------------------------------
export const WIDGET_ID = 'kaboom-terminal-widget'
export const IFRAME_ID = 'kaboom-terminal-iframe'
export const HEADER_ID = 'kaboom-terminal-header'
export const TERMINAL_PROVIDER_BADGE_ID = 'kaboom-terminal-provider-badge'
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
// Write-queue bounds
// ---------------------------------------------------------------------------
/** Maximum number of agent writes held while the terminal is unreachable. */
export const MAX_QUEUED_WRITES = 200
/**
 * Maximum total SIZE of that backlog, in UTF-8 bytes.
 *
 * The entry count alone is not a bound on anything that matters: 200 one-megabyte
 * writes is a legal state under it, i.e. ~200 MB pinned in the side panel with
 * nothing to stop it (finding S14). Writes are only queued while the socket is
 * down, so this also mirrors the daemon's own 1 MB PTY write-buffer cap — more
 * than this could never be delivered in one go anyway.
 */
export const MAX_QUEUED_WRITE_BYTES = 1 << 20

// ---------------------------------------------------------------------------
// Reconnect schedule — mirrored from the terminal iframe (terminal.html).
// ---------------------------------------------------------------------------
// terminal.html is a hand-authored, Go-embedded asset, so it cannot import these
// and they cannot import it. They are declared here because the write-guard's
// give-up budget below is DERIVED from them; tests/extension/
// terminal-reconnect-budget-contract.test.js pins these values to the literals
// terminal.html actually uses, so the two cannot drift apart.
export const TERMINAL_RECONNECT_BASE_DELAY_MS = 1000
export const TERMINAL_RECONNECT_MAX_DELAY_MS = 10000
export const TERMINAL_MAX_RECONNECT_ATTEMPTS = 6
// The iframe jitters each backoff by up to this fraction (additive only) so that
// panels dropped together by a daemon restart do not reconnect in lockstep.
export const TERMINAL_RECONNECT_JITTER_RATIO = 0.25

/**
 * WORST-CASE wall-clock time from the first disconnect until the iframe gives up
 * and posts `reconnect_exhausted` (which is what triggers the parent's
 * validate-and-rebuild recovery). The iframe waits before EVERY attempt, including
 * the one that trips the cap — the `reconnectAttempts > MAX_RECONNECT_ATTEMPTS`
 * check runs after the increment, inside the timer — so there are MAX+1 waits:
 * 1+2+4+8+10+10+10 = 45s, and up to 25% more once jitter is applied.
 *
 * Worst case, not average, is the right number here: the write-guard budget must
 * cover the slowest run, or it goes back to dropping the queue early.
 */
export function terminalReconnectExhaustionMs(): number {
  let delay = TERMINAL_RECONNECT_BASE_DELAY_MS
  let total = 0
  for (let attempt = 0; attempt <= TERMINAL_MAX_RECONNECT_ATTEMPTS; attempt++) {
    total += delay
    delay = Math.min(delay * 2, TERMINAL_RECONNECT_MAX_DELAY_MS)
  }
  return Math.ceil(total * (1 + TERMINAL_RECONNECT_JITTER_RATIO))
}

// Headroom on top of the iframe's schedule so the parent has time to actually run
// its recovery (validate token → rebuild session → reconnect) after
// `reconnect_exhausted` lands, instead of the guard expiring at the same instant.
export const TERMINAL_GUARD_RECOVERY_GRACE_MS = 10000

// Escape hatch: the maximum total time the write-guard will keep an agent write
// "in flight" or a queued write "deferred" (waiting for the socket to reconnect
// or the user to stop typing) before giving up LOUDLY. Without this bound a
// permanently-down socket or a stuck `queuedWriteInFlight`/`terminalFocused`
// flag would wedge the terminal forever — writes queue but never flush and the
// poller spins silently.
//
// DERIVED, not hand-picked (finding S1): the old fixed 30s expired 15s before the
// iframe even reported `reconnect_exhausted` at ~45s, so the queue was thrown away
// before the recovery it was waiting for could begin — the queue could never
// survive the outage it exists for.
export const TERMINAL_GUARD_MAX_WAIT_MS = terminalReconnectExhaustionMs() + TERMINAL_GUARD_RECOVERY_GRACE_MS

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
  // Wall-clock time (ms) the guard first got stuck unable to deliver a write
  // (0 = making progress). Drives the TERMINAL_GUARD_MAX_WAIT_MS escape hatch.
  guardBlockedSince: number
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
  terminalConnected: false,
  guardBlockedSince: 0
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
  state.guardBlockedSince = 0
  resetTerminalPortDiscovery()
}

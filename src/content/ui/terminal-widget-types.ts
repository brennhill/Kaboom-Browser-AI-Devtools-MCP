/**
 * Purpose: Shared constants, types, and mutable state for the terminal widget.
 * Why: Centralises state and constants so split modules reference the same values
 *      without circular dependencies.
 * Docs: docs/features/feature/terminal/index.md
 */

import { DEFAULT_SERVER_URL, TERMINAL_PORT_OFFSET } from '../../lib/constants.js'
import { buildDaemonHeaders } from '../../lib/daemon-http.js'
import type { components } from '../../generated/openapi-types.js'

type HealthResponse = components['schemas']['HealthResponse']

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

/**
 * Wall-clock time from the first disconnect until the iframe gives up and posts
 * `reconnect_exhausted` (which is what triggers the parent's validate-and-rebuild
 * recovery). The iframe waits before EVERY attempt, including the one that trips
 * the cap — the `reconnectAttempts > MAX_RECONNECT_ATTEMPTS` check runs after the
 * increment, inside the timer — so there are MAX+1 waits: 1+2+4+8+10+10+10 = 45s.
 */
export function terminalReconnectExhaustionMs(): number {
  let delay = TERMINAL_RECONNECT_BASE_DELAY_MS
  let total = 0
  for (let attempt = 0; attempt <= TERMINAL_MAX_RECONNECT_ATTEMPTS; attempt++) {
    total += delay
    delay = Math.min(delay * 2, TERMINAL_RECONNECT_MAX_DELAY_MS)
  }
  return total
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
export const TERMINAL_GUARD_MAX_WAIT_MS =
  terminalReconnectExhaustionMs() + TERMINAL_GUARD_RECOVERY_GRACE_MS

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

// ---------------------------------------------------------------------------
// Terminal server URL — discovered, with a derived fallback.
// ---------------------------------------------------------------------------
// The daemon *prefers* base + TERMINAL_PORT_OFFSET but does not guarantee it: if
// that port is taken it logs terminal_server_bind_failed, and it always reports
// the port it actually bound as `terminal_port` in /health. Assuming base+1 (and
// never reading the published value) sent every terminal request to a port
// nothing was listening on, which looks to the user like a broken terminal
// (finding S2).
//
// Cached per base URL and per extension context (content script, service worker,
// side panel each hold their own module instance), with a TTL so a daemon that
// restarts onto a different port is picked up.
const TERMINAL_PORT_CACHE_TTL_MS = 60000

interface DiscoveredTerminalPort {
  baseUrl: string
  port: number
  discoveredAt: number
}

let discoveredTerminalPort: DiscoveredTerminalPort | null = null
/** In-flight discovery, so concurrent callers share one /health request. */
let terminalPortDiscovery: Promise<void> | null = null

/** Clear the discovered-port cache (context teardown and tests). */
export function resetTerminalPortDiscovery(): void {
  discoveredTerminalPort = null
  terminalPortDiscovery = null
}

/** The cached port for baseUrl, or null when absent or stale. */
function cachedTerminalPort(baseUrl: string, nowMs: number): number | null {
  if (!discoveredTerminalPort) return null
  if (discoveredTerminalPort.baseUrl !== baseUrl) return null
  if (nowMs - discoveredTerminalPort.discoveredAt > TERMINAL_PORT_CACHE_TTL_MS) return null
  return discoveredTerminalPort.port
}

/**
 * Compute the terminal server URL for a base daemon URL.
 *
 * Synchronous, so it can serve call sites that cannot await (notifyIframe's
 * postMessage target origin). It uses the discovered port when this context has
 * one and otherwise derives base + TERMINAL_PORT_OFFSET — the daemon's own
 * default, so this is never worse than the old behaviour. Prefer
 * resolveTerminalServerUrl wherever awaiting is possible.
 */
export function getTerminalServerUrl(baseUrl: string): string {
  const url = new URL(baseUrl)
  const basePort = parseInt(url.port || '7890', 10)
  const port = cachedTerminalPort(baseUrl, Date.now()) ?? basePort + TERMINAL_PORT_OFFSET
  url.port = String(port)
  return url.origin
}

/**
 * Resolve the terminal server URL, discovering the daemon's real terminal port
 * first (once per TTL). Discovery lives INSIDE this helper rather than at the call
 * sites so no caller can forget it (rule 19).
 *
 * Every failure mode — daemon down, non-OK response, unparseable body, no
 * `terminal_port` field (Windows, or a terminal server that failed to bind) —
 * falls through to the derived port, so discovery can only ever improve on the
 * old assumption, never break a working setup.
 */
export async function resolveTerminalServerUrl(baseUrl: string): Promise<string> {
  if (cachedTerminalPort(baseUrl, Date.now()) === null) {
    if (!terminalPortDiscovery) {
      terminalPortDiscovery = discoverTerminalPort(baseUrl).finally(() => {
        terminalPortDiscovery = null
      })
    }
    await terminalPortDiscovery
  }
  return getTerminalServerUrl(baseUrl)
}

/** Read `terminal_port` from /health and cache it. Never throws. */
async function discoverTerminalPort(baseUrl: string): Promise<void> {
  try {
    const resp = await fetch(`${baseUrl}/health`, {
      headers: buildDaemonHeaders({ contentType: null }),
      signal: AbortSignal.timeout(2000)
    })
    if (!resp.ok) return
    const data = (await resp.json()) as HealthResponse
    const port = data.terminal_port
    // 0 / absent means the terminal server is not running (Windows, or a bind
    // failure). Keep the derived fallback rather than caching a dead port.
    if (typeof port !== 'number' || port <= 0) return
    discoveredTerminalPort = { baseUrl, port, discoveredAt: Date.now() }
  } catch {
    // Daemon unreachable or the response was not JSON — the caller falls back to
    // the derived port, which is the daemon's own default.
  }
}

/**
 * Purpose: Side panel host for the Kaboom terminal.
 * Why: Removes the terminal from page context so CSP on arbitrary sites cannot
 * interfere with the xterm host, while keeping the session and reconnect model intact.
 * Docs: docs/features/feature/terminal/index.md
 */

import { StorageKey, TERMINAL_PANEL_PORT } from './lib/constants.js'
import { onStorageChanged } from './lib/storage-utils.js'
import {
  state,
  resetAllState,
  getTerminalServerUrl,
  resolveTerminalServerUrl,
  WIDGET_ID,
  IFRAME_ID,
  HEADER_ID,
  TERMINAL_BODY_ID,
  DISCONNECT_TERMINAL_BUTTON_ID,
  ANNOTATE_TERMINAL_BUTTON_ID,
  CLOSE_TERMINAL_BUTTON_ID,
  REDRAW_TERMINAL_BUTTON_ID,
  MINIMIZE_TERMINAL_BUTTON_ID,
  TERMINAL_WRITE_SUBMIT_DELAY_MS,
  TERMINAL_TYPING_IDLE_MS,
  TERMINAL_GUARD_POLL_MS,
  TERMINAL_GUARD_TOAST_INTERVAL_MS,
  type TerminalUIState
} from './content/ui/terminal-widget-types.js'
import {
  getServerUrl,
  getTerminalConfig,
  persistUIState,
  loadPersistedSession,
  clearPersistedSession,
  validateSession,
  startSession,
  getTerminalDevRoot,
  setTerminalDevRoot,
  stopActiveSession
} from './content/ui/terminal-widget-session.js'
import { showActionToast } from './content/ui/toast.js'
import { getHostTabId, startPageAnnotation, closeBrowserSidePanel } from './content/ui/panel/host-tab.js'
import { createRootFolderBar } from './content/ui/terminal-root-folder.js'
import { renderNoSessionState, renderStartFailure, renderStartPending } from './content/ui/terminal-panel-states.js'
import { createPanelShell as buildPanelShell } from './content/ui/panel/shell.js'
import {
  notifyIframe,
  resetWriteGuardState,
  shouldDeferQueuedWrite,
  maybeShowQueuedWriteToast,
  scheduleQueuedWriteFlush,
  scheduleQueuedSubmit,
  flushQueuedWrites,
  enqueueBoundedWrite
} from './content/ui/terminal-write-guard.js'

// =============================================================================
// TERMINAL PANEL STATE
// =============================================================================

/**
 * All mutable panel-UI state in one object (was 14 scattered module `let`s). The
 * side panel is a singleton in the real document, but grouping the DOM refs +
 * lifecycle flags makes the panel's state explicit and lets a fresh panel be
 * created for isolation. `panel` is const — fields are mutated in place
 * (unmountPanel clears them); bootGeneration bumps per boot for supersede-abandon.
 */
interface PanelUi {
  rootEl: HTMLDivElement | null
  terminalShellEl: HTMLDivElement | null
  terminalBodyEl: HTMLDivElement | null
  statusDotEl: HTMLSpanElement | null
  minimizeButtonEl: HTMLButtonElement | null
  runtimeListenerInstalled: boolean
  storageListenerInstalled: boolean
  unloadListenerInstalled: boolean
  panelReady: boolean
  pendingSandboxError: { message: string; instruction: string; command: string } | null
  panelCloseIntent: TerminalUIState | 'clear' | null
  presencePort: chrome.runtime.Port | null
  rootFolderBar: { element: HTMLDivElement; setRoot: (root: string) => void } | null
  bootGeneration: number
  // Timestamps of exhaustion-driven recoveries (iframe reconnect_exhausted →
  // redrawTerminal) within the sliding window. A flapping daemon — up enough for
  // the 2s validate but not for onopen — would otherwise thrash
  // redraw→reconnect→exhaust forever; past the ceiling we drop to the no-session
  // state instead of re-looping (finding E-i). Reset on a real 'connected'.
  reconnectRecoveryAt: number[]
}

function freshPanelUi(): PanelUi {
  return {
    rootEl: null,
    terminalShellEl: null,
    terminalBodyEl: null,
    statusDotEl: null,
    minimizeButtonEl: null,
    runtimeListenerInstalled: false,
    storageListenerInstalled: false,
    unloadListenerInstalled: false,
    panelReady: false,
    pendingSandboxError: null,
    panelCloseIntent: null,
    presencePort: null,
    rootFolderBar: null,
    bootGeneration: 0,
    reconnectRecoveryAt: []
  }
}

/** Sliding window and cap for parent-side exhaustion-driven recoveries (E-i). */
const RECONNECT_RECOVERY_WINDOW_MS = 30_000
const MAX_RECONNECT_RECOVERIES = 3

/**
 * Record one exhaustion-driven recovery attempt and report whether the ceiling is
 * now exceeded. Prunes attempts older than the window so a slow, occasional
 * daemon-restart recovery never trips it — only a fast flap does.
 */
function exhaustionRecoveryCeilingReached(): boolean {
  const now = Date.now()
  panel.reconnectRecoveryAt = panel.reconnectRecoveryAt.filter((t) => now - t < RECONNECT_RECOVERY_WINDOW_MS)
  panel.reconnectRecoveryAt.push(now)
  return panel.reconnectRecoveryAt.length > MAX_RECONNECT_RECOVERIES
}

function resetExhaustionRecovery(): void {
  panel.reconnectRecoveryAt = []
}

const panel = freshPanelUi()

/**
 * Reset all panel-UI state to a clean slate. `panel` is const, so mutate its
 * fields in place (Object.assign) rather than rebind. Lets a test start from a
 * known-empty panel without reloading the module.
 */
export function resetPanelUi(): void {
  Object.assign(panel, freshPanelUi())
}

/** Backoff before re-announcing presence after a service-worker restart. */
const PRESENCE_RECONNECT_DELAY_MS = 500

function setPanelVisible(visible: boolean): void {
  state.visible = visible
  if (!panel.rootEl) return
  panel.rootEl.style.opacity = visible ? '1' : '0'
  panel.rootEl.style.pointerEvents = visible ? 'auto' : 'none'
}

// The terminal is a full-height side panel; there is no collapse-in-place state
// (the old MINIMIZED_WIDGET_HEIGHT was leftover from the in-page-widget era and
// was never actually applied \u2014 every caller passed `visible: true`). The body is
// simply shown; "minimize" and "close" both dismiss the whole panel and keep the
// session (see closePanelWithIntent).
function showTerminalBody(): void {
  if (!panel.terminalBodyEl || !panel.terminalShellEl) return
  panel.terminalBodyEl.style.display = 'block'
  panel.terminalShellEl.style.height = '100%'
  panel.terminalShellEl.style.minHeight = '0'
  panel.terminalShellEl.style.flex = '1 1 auto'
}

/**
 * Show the recoverable no-session state in the terminal body.
 */
function showNoSessionState(): void {
  if (!panel.terminalBodyEl) return
  renderNoSessionState(panel.terminalBodyEl, () => { void bootTerminalPanel(true) })
}

const BOOT_PENDING_ID = 'kaboom-terminal-boot-pending'

/**
 * Show a standalone "starting…" overlay for the window where no panel exists.
 *
 * A boot resolves the session BEFORE it builds and mounts the shell, and a
 * forceFresh boot unmounts the old panel first — so for the whole (network-bound,
 * now retry-extended) `ensureTerminalSession()` call there is no terminal body to
 * render into and the side panel sits empty. An empty panel is indistinguishable
 * from a broken one, which is exactly the ambiguity the honest-error work is
 * trying to remove.
 *
 * Deliberately NOT part of the panel shell: it owns its own element and lifecycle,
 * so it cannot interact with the generation-based mount races that `panel.*`
 * carefully guards.
 */
function showBootPending(): void {
  if (document.getElementById(BOOT_PENDING_ID)) return
  const host = document.body || document.documentElement
  if (!host) return
  const el = document.createElement('div')
  el.id = BOOT_PENDING_ID
  renderStartPending(el)
  host.appendChild(el)
}

/** Remove the boot overlay. Safe to call when it was never shown. */
function clearBootPending(): void {
  document.getElementById(BOOT_PENDING_ID)?.remove()
}

/**
 * Mount the root-folder bar and keep it showing the current root.
 *
 * The bar is built synchronously so the shell lays out immediately; the stored
 * root arrives a tick later and is filled in.
 */
function createRootFolderBarElement(): HTMLDivElement {
  const bar = createRootFolderBar({
    initialRoot: '',
    onApply: (root: string) => { void applyRootFolder(root) }
  })
  panel.rootFolderBar = bar
  void getTerminalDevRoot().then((root) => bar.setRoot(root))
  return bar.element
}

/**
 * Persist the root folder and restart the shell there. A running PTY cannot be
 * moved — its cwd is fixed at spawn — so the old session is stopped first.
 */
async function applyRootFolder(root: string): Promise<void> {
  await setTerminalDevRoot(root)
  await stopActiveSession()
  showActionToast('Terminal root folder set', root || '(auto-detect)', 'success', 2500)
  await bootTerminalPanel(true)
}

/**
 * Surface a terminal start failure — logged (in startSession) AND shown to the
 * user; no start failure may vanish into the console (repo rule 25, fail-loud).
 *
 * The `kind` decides the surface:
 * - `unavailable` (daemon answered with an error status, e.g. 500): recoverable.
 *   Do NOT remember it as a stuck error — fall through to the no-session state
 *   (Start + root folder). Remembering it here would replace the recoverable UI
 *   with a dead-end panel (regresses the daemon-unavailable fallback).
 * - `unreachable` (daemon did not answer) / `sandbox` (spawn refused): a real,
 *   actionable failure. Remember it so a later remount re-shows it, and surface
 *   it now — inline when the body is mounted, else via a toast so a
 *   daemon-down-at-open failure is never swallowed silently.
 */
function showSandboxError(
  message: string,
  instruction: string,
  command: string,
  kind?: 'unreachable' | 'unavailable' | 'sandbox'
): void {
  if (kind === 'unavailable') {
    // Reachable but not ready — the no-session fallback IS the surface. Already
    // logged by startSession; do not remember or render a dead-end error.
    return
  }
  panel.pendingSandboxError = { message, instruction, command }
  if (panel.terminalBodyEl) {
    renderStartFailure(panel.terminalBodyEl, message, instruction, command)
  } else {
    // No body to render into yet (daemon-down-at-open): surface via toast so the
    // failure is visible instead of only reaching the console. A subsequent
    // remount re-renders it inline from pendingSandboxError.
    const detail = [instruction, command].filter(Boolean).join(' ')
    showActionToast(message, detail || 'Terminal', 'error', 6000)
  }
}

function updateStatusDot(dotState: 'connected' | 'disconnected' | 'exited'): void {
  if (!panel.statusDotEl) return
  switch (dotState) {
    case 'connected':
      panel.statusDotEl.style.background = '#9ece6a'
      break
    case 'disconnected':
      panel.statusDotEl.style.background = '#e0af68'
      break
    case 'exited':
      panel.statusDotEl.style.background = '#f7768e'
      break
  }
}

function handleIframeMessage(event: MessageEvent): void {
  if (!event.data || event.data.source !== 'kaboom-terminal') return
  try {
    const termOrigin = getTerminalServerUrl(state.serverUrl)
    if (event.origin !== termOrigin) return
  } catch {
    return
  }
  switch (event.data.event as string) {
    case 'connected':
      // Trail for diagnosing "can't type": these WS transitions are where the
      // terminal loses input when the daemon terminal-server (port+1) blinks.
      console.log('[KaBOOM! terminal] ws connected')
      updateStatusDot('connected')
      state.terminalConnected = true
      // A real connection clears the flap budget so an unrelated future outage
      // gets its own full recovery allowance (E-i).
      resetExhaustionRecovery()
      if (state.queuedWrites.length > 0 && !state.queuedWriteInFlight) {
        scheduleQueuedWriteFlush(0)
      }
      break
    case 'disconnected':
      console.log('[KaBOOM! terminal] ws disconnected (input paused; writes will queue)')
      updateStatusDot('disconnected')
      state.terminalConnected = false
      state.terminalFocused = false
      break
    case 'reconnect_exhausted':
      // The iframe gave up reconnecting on a token that almost certainly died with
      // a full daemon restart. Recover instead of sitting on a permanent silent
      // disconnect: revalidate and rebuild into a fresh session (or the recoverable
      // no-session state). redrawTerminal owns that validate-then-rebuild logic.
      updateStatusDot('disconnected')
      state.terminalConnected = false
      state.terminalFocused = false
      if (exhaustionRecoveryCeilingReached()) {
        // A flapping daemon (up for the 2s validate, not for onopen) would thrash
        // redraw→reconnect→exhaust indefinitely. Stop auto-recovering: detach the
        // iframe and drop to the recoverable no-session state so the user restarts
        // on their terms rather than watching a silent, endless reconnect (E-i).
        console.warn('[KaBOOM! terminal] reconnect recovery ceiling reached — showing no-session state')
        resetExhaustionRecovery()
        state.iframeEl = null
        showNoSessionState()
        break
      }
      console.log('[KaBOOM! terminal] reconnect exhausted — revalidating and rebuilding')
      void redrawTerminal()
      break
    case 'exited':
      console.log('[KaBOOM! terminal] session exited (write-guard reset)')
      updateStatusDot('exited')
      state.terminalConnected = false
      state.terminalFocused = false
      resetWriteGuardState()
      break
    case 'focus':
      state.terminalFocused = Boolean((event.data.data as { focused?: boolean } | undefined)?.focused)
      if (state.terminalFocused) {
        state.lastTypingAt = Date.now()
      } else if (state.queuedWrites.length > 0 && !state.queuedWriteInFlight) {
        scheduleQueuedWriteFlush(0)
      }
      break
    case 'typing': {
      const rawAt = (event.data.data as { at?: number } | undefined)?.at
      const parsedAt = typeof rawAt === 'number' && Number.isFinite(rawAt) ? rawAt : Date.now()
      state.terminalFocused = true
      state.lastTypingAt = parsedAt
      break
    }
  }
}

/**
 * Wire the extracted shell builder to this module's panel/state fields.
 *
 * The builder owns no state, so every element it creates is routed back here. One
 * adapter keeps that wiring in a single place and leaves call sites unchanged.
 */
function createPanelShell(token: string): HTMLDivElement {
  return buildPanelShell(token, {
    serverUrl: state.serverUrl,
    onExit: () => { void exitTerminalSession() },
    onAnnotate: () => { void startPageAnnotation() },
    onRedraw: () => { void redrawTerminal() },
    onMinimize: () => { void minimizePanel() },
    onClose: () => { void closePanelKeepingSession() },
    createRootFolderBar: () => createRootFolderBarElement(),
    setStatusDot: (el) => { panel.statusDotEl = el },
    setMinimizeButton: (el) => { panel.minimizeButtonEl = el },
    setTerminalShell: (el) => { panel.terminalShellEl = el },
    setTerminalBody: (el) => { panel.terminalBodyEl = el },
    setWidget: (el) => { state.widgetEl = el },
    setIframe: (el) => { state.iframeEl = el }
  })
}

function mountPanel(root: HTMLDivElement): void {
  if (panel.rootEl) return
  panel.rootEl = root
  const target = document.body || document.documentElement
  if (!target) return
  target.appendChild(panel.rootEl)
  setPanelVisible(true)
  state.visible = true
  window.addEventListener('message', handleIframeMessage)
}

function unmountPanel(): void {
  if (panel.rootEl) {
    panel.rootEl.remove()
    panel.rootEl = null
  }
  panel.terminalShellEl = null
  panel.terminalBodyEl = null
  panel.statusDotEl = null
  panel.minimizeButtonEl = null
  panel.rootFolderBar = null
  state.widgetEl = null
  state.iframeEl = null
  panel.panelReady = false
  setPanelVisible(false)
  window.removeEventListener('message', handleIframeMessage)
}

async function redrawTerminal(): Promise<void> {
  if (!state.widgetEl || !state.iframeEl) return
  const currentToken = state.sessionState?.token
  if (!currentToken) return
  // After a daemon restart the token is dead; a plain reload would reconnect
  // forever to a session that no longer exists and the panel would wedge on
  // "disconnected". Confirm the token still maps to a live shell first; if not,
  // rebuild so the panel recovers into a fresh session (or the recoverable
  // no-session state) rather than a permanent disconnect.
  if (!(await validateSession(currentToken))) {
    await bootTerminalPanel(true)
    return
  }
  const iframe = state.iframeEl
  // Reloading the iframe tears down the old WebSocket and reconnects from
  // scratch; until the fresh document posts 'connected', the socket is not OPEN.
  // Mark disconnected so a write in that reconnect gap queues instead of being
  // sent-and-dropped (the redraw sub-case of the write-connection race).
  state.terminalConnected = false
  iframe.src = `${await resolveTerminalServerUrl(state.serverUrl)}/terminal?token=${encodeURIComponent(currentToken)}`
  showTerminalBody()
  persistUIState('open')
}

/**
 * The single teardown path for dismissing the panel, whatever the reason.
 *
 * Three dismissals (exit / close / minimize) previously copy-pasted the same
 * teardown tail, and the invariant — persist-or-clear intent, then
 * resetWriteGuardState() + unmountPanel() + closeBrowserSidePanel() — lived in
 * each caller, so one drifting would leak the write guard. Now the invariant
 * lives here and no caller can forget a step (repo rule 19).
 *
 * 'clear' fully ends the session (the shell is already being stopped by the
 * caller); 'closed'/'minimized' keep the session and persist the UI state so
 * reopening reconnects.
 */
async function closePanelWithIntent(intent: TerminalUIState | 'clear'): Promise<void> {
  panel.panelCloseIntent = intent
  if (intent === 'clear') {
    clearPersistedSession()
    resetAllState()
  } else {
    persistUIState(intent)
  }
  resetWriteGuardState()
  // Hide immediately, then ask the browser to close the panel BEFORE tearing down
  // the DOM. On mainstream Chrome window.close() destroys this document (its
  // presence port drops so the background can reopen); doing the authoritative
  // browser close first means we never leave an unmounted, blank document holding
  // a stale-open presence port on the latent path where the close is refused.
  setPanelVisible(false)
  await closeBrowserSidePanel()
  unmountPanel()
}

async function exitTerminalSession(): Promise<void> {
  // Set the intent before the network call in case a disconnect races the stop.
  panel.panelCloseIntent = 'clear'
  if (state.sessionState) {
    try {
      const termUrl = await resolveTerminalServerUrl(state.serverUrl)
      await fetch(`${termUrl}/terminal/stop`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: state.sessionState.sessionId }),
        signal: AbortSignal.timeout(3000)
      })
    } catch {
      // daemon unreachable or timeout — tear down locally
    }
  }
  await closePanelWithIntent('clear')
}

/**
 * Close the drawer and leave the shell running.
 *
 * This is what "close" has to mean for a terminal: exitTerminalSession() kills
 * the PTY, so a user who just wanted the panel out of the way lost their shell
 * and had no way back. Reopening the panel reconnects to this session.
 */
async function closePanelKeepingSession(): Promise<void> {
  await closePanelWithIntent('closed')
}

async function minimizePanel(): Promise<void> {
  await closePanelWithIntent('minimized')
}

function writeToTerminal(text: string): void {
  if (!state.visible || !state.iframeEl) return
  // Queue (do NOT send now) when the user is mid-keystroke OR the socket is not
  // yet OPEN. Sending while disconnected is silently dropped by the iframe host
  // (ws.readyState !== OPEN), and the deferred submit would then fire a bare Enter
  // with the text gone — the AI's write / annotation vanishes with no error. The
  // flush poller replays queued writes once `terminalConnected` (the same gate the
  // drainers use). Fail loud, not silent (rule 25).
  const typing = shouldDeferQueuedWrite()
  if (typing || !state.terminalConnected) {
    enqueueBoundedWrite(text)
    if (typing) maybeShowQueuedWriteToast()
    scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS)
    return
  }
  if (state.queuedWriteInFlight) {
    enqueueBoundedWrite(text)
    return
  }
  state.queuedWriteInFlight = true
  notifyIframe('redraw')
  notifyIframe('write', { text })
  scheduleQueuedSubmit(TERMINAL_WRITE_SUBMIT_DELAY_MS)
}

function installRuntimeListener(): void {
  if (panel.runtimeListenerInstalled) return
  panel.runtimeListenerInstalled = true
  chrome.runtime.onMessage.addListener((
    message: { type?: string; text?: string },
    sender: chrome.runtime.MessageSender,
    sendResponse: (response?: unknown) => void
  ) => {
    if (sender.id !== chrome.runtime.id) return false
    // The background cannot close a side panel document on every Chrome version,
    // but this document can, so it asks us to.
    if (message.type === 'close_terminal_panel') {
      void closePanelKeepingSession()
      return false
    }
    if (message.type !== 'terminal_panel_write') return false
    // Acknowledge synchronously: this document existing IS the proof the sender
    // needs that the write reached a live panel (the background never replies to
    // this type). Ack first so a later fault in writeToTerminal can't swallow it.
    sendResponse({ received: true })
    if (typeof message.text === 'string') writeToTerminal(message.text)
    return false
  })
}

/**
 * Announce that a panel document exists, for as long as it exists.
 *
 * This is how the background answers "is there a panel?" before deciding whether
 * a toggle should open or close. It has to be a port: closing the panel with
 * Chrome's own X destroys this document without running any of our teardown, so
 * a stored flag would stay "open" forever and the toggle would never open again.
 * Chrome disconnects the port either way.
 *
 * The reconnect covers service-worker restarts, which drop every port while this
 * document happily stays open.
 */
function connectPresencePort(): void {
  if (panel.presencePort || typeof chrome === 'undefined' || !chrome.runtime?.connect) return
  try {
    const port = chrome.runtime.connect({ name: TERMINAL_PANEL_PORT })
    panel.presencePort = port
    port.onMessage.addListener((message: { type?: string }) => {
      if (message?.type === 'close_terminal_panel') void closePanelKeepingSession()
      if (message?.type === 'restore_terminal_panel') void restoreTerminalPanel()
    })
    port.onDisconnect.addListener(() => {
      panel.presencePort = null
      // Only worth retrying while this document is still around and the runtime
      // is still valid; both stop being true on teardown, and connect() throws.
      if (!panel.rootEl) return
      setTimeout(connectPresencePort, PRESENCE_RECONNECT_DELAY_MS)
    })
  } catch {
    panel.presencePort = null // Extension context invalidated — nothing to announce to.
  }
}

function installStorageListener(): void {
  if (panel.storageListenerInstalled) return
  panel.storageListenerInstalled = true
  onStorageChanged((changes, areaName) => {
    if (areaName !== 'session') return
    const change = changes[StorageKey.TERMINAL_UI_STATE]
    if (!change) return
    const uiState = change.newValue as TerminalUIState | undefined
    if (uiState === 'closed') {
      state.visible = false
      if (panel.rootEl) panel.rootEl.style.opacity = '0'
      return
    }
    state.visible = true
    if (panel.rootEl) panel.rootEl.style.opacity = '1'
  })
}

function installUnloadListener(): void {
  if (panel.unloadListenerInstalled) return
  panel.unloadListenerInstalled = true
  window.addEventListener('pagehide', () => {
    if (panel.panelCloseIntent !== null) return
    persistUIState('closed')
  })
}

/**
 * Bring the terminal back after an open request that landed on a panel document
 * that was already here.
 *
 * Chrome answers `sidePanel.open()` on an existing panel by focusing it, which
 * runs no code in this document. A panel left minimized — or unmounted because
 * `window.close()` was refused — would just sit there blank. Opening must always
 * mean "there is a working terminal in front of me".
 */
async function restoreTerminalPanel(): Promise<void> {
  panel.panelCloseIntent = null
  if (panel.rootEl && state.iframeEl) {
    // A live terminal is already mounted — it was only minimized or hidden.
    setPanelVisible(true)
    showTerminalBody()
    persistUIState('open')
    // Nothing here was rebuilt, so a root changed elsewhere (the options page,
    // another panel) would otherwise still read as the old one.
    void getTerminalDevRoot().then((root) => panel.rootFolderBar?.setRoot(root))
    return
  }
  // No terminal in here: either unmounted, or mounted with no session (the
  // daemon was down when it booted). Rebuild, revalidating the token so the
  // xterm reconnects to a shell that is actually alive rather than rendering a
  // dead one.
  if (panel.rootEl) unmountPanel()
  panel.panelReady = false
  await bootTerminalPanel()
}

async function ensureTerminalSession(): Promise<void> {
  const persisted = await loadPersistedSession()
  if (persisted.session) {
    const alive = await validateSession(persisted.session.token)
    if (alive) {
      state.sessionState = persisted.session
      return
    }
    clearPersistedSession()
  }
  const config = await getTerminalConfig()
  const ss = await startSession(config, showSandboxError)
  if (!ss) return
  state.sessionState = ss
}


/**
 * Boot (or rebuild) the terminal panel — GENERATION-based, not serialized.
 *
 * Each boot claims a generation (`panel.bootGeneration`); a newer boot supersedes
 * older ones. Any boot whose generation is stale by the time it reaches DOM
 * mutation aborts before touching the panel. This gives BOTH guarantees at once:
 *
 *  - No iframe-orphan race: two concurrent forceFresh boots can't both mount —
 *    the older aborts at the guard before createPanelShell()/mountPanel(), so the
 *    visible iframe and `state.iframeEl` never diverge (the "writes disappear" bug).
 *  - "Start terminal" ALWAYS works: we never await a previous boot, so a boot that
 *    STALLED on the network (a daemon that isn't answering) can neither block nor
 *    corrupt a fresh Start — the panic button always re-attempts and resets state.
 *    (Serializing on a bootChain regressed exactly this: a hung boot froze Start.)
 */
function bootTerminalPanel(forceFresh = false): Promise<void> {
  if (panel.panelReady && !forceFresh) return Promise.resolve()
  const myGen = ++panel.bootGeneration
  return bootTerminalPanelInner(forceFresh, myGen)
}

async function bootTerminalPanelInner(forceFresh: boolean, gen: number): Promise<void> {
  // Drop the existing panel DOM before (re)building. mountPanel() early-returns
  // while `panel.rootEl` is set, so without this the freshly-built shell — bound to the
  // NEW session in the just-selected folder — is never attached, and the user
  // keeps staring at the old panel wired to the session we just stopped (the
  // "terminal won't start after picking a folder" bug, and the retry button).
  // Done before `panel.panelReady = true` because unmountPanel() clears that flag;
  // restoreTerminalPanel already unmounts, applyRootFolder did not.
  if (forceFresh) unmountPanel()
  panel.panelReady = true
  panel.panelCloseIntent = null
  panel.pendingSandboxError = null
  // Nothing is mounted from here until mountPanel() below, and the session call in
  // between is network-bound (and may now retry a transient EPERM). Show a live
  // pending state for that gap. Cleared in `finally` so every exit — success, throw,
  // and all three superseded-generation early returns — takes it down.
  showBootPending()
  try {
    state.serverUrl = await getServerUrl()
    if (gen !== panel.bootGeneration) return // superseded by a newer boot
    installRuntimeListener()
    installStorageListener()
    installUnloadListener()
    connectPresencePort()
    if (forceFresh) {
      resetAllState()
      state.serverUrl = await getServerUrl()
      if (gen !== panel.bootGeneration) return
    }
    await ensureTerminalSession()
    // A newer boot superseded this one while we awaited the (possibly slow, possibly
    // hung) session call — abandon BEFORE mutating the DOM so we can't orphan its iframe.
    if (gen !== panel.bootGeneration) return
    const token = state.sessionState?.token
    const root = createPanelShell(token ?? '')
    mountPanel(root)
    showTerminalBody()
    persistUIState('open')
    if (!token) {
      const error = panel.pendingSandboxError as { message: string; instruction: string; command: string } | null
      if (error) {
        showSandboxError(error.message, error.instruction, error.command)
      } else {
        showNoSessionState()
      }
    }
  } catch (err) {
    // A boot that threw (network error, extension context invalidated) must not
    // wedge the button: clear panelReady so the next "Start terminal" boots fresh,
    // and — if this is still the current boot and a body exists — show the
    // retriable no-session state instead of a blank/half-built panel.
    if (gen === panel.bootGeneration) {
      panel.panelReady = false
      console.log('[KaBOOM! terminal] boot failed:', String(err))
      if (panel.terminalBodyEl) showNoSessionState()
    }
  } finally {
    // Must run on EVERY exit path, including the superseded-generation returns —
    // a leaked overlay would sit on top of the panel the newer boot just mounted.
    clearBootPending()
  }
}

/**
 * Entry point: boot the panel once the document is ready. Auto-invoked at module
 * scope in the real side-panel document; the `process === undefined` guard keeps
 * it from firing under Node test imports. Named + exported so it is an explicit,
 * callable entry rather than an anonymous top-level side effect.
 */
export function main(): void {
  if (typeof document === 'undefined' || typeof (globalThis as Record<string, unknown>).process !== 'undefined') {
    return
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
      void bootTerminalPanel()
    })
  } else {
    void bootTerminalPanel()
  }
}

main()

export const _terminalPanelForTests = {
  bootTerminalPanel,
  applyRootFolder,
  writeToTerminal,
  exitTerminalSession,
  redrawTerminal,
  resetPanelUi
}

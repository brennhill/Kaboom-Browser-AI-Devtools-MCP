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
  WIDGET_ID,
  IFRAME_ID,
  HEADER_ID,
  TERMINAL_BODY_ID,
  DISCONNECT_TERMINAL_BUTTON_ID,
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
import { createRootFolderBar } from './content/ui/terminal-root-folder.js'
import { renderNoSessionState, renderStartFailure } from './content/ui/terminal-panel-states.js'
import {
  notifyIframe,
  resetWriteGuardState,
  shouldDeferQueuedWrite,
  maybeShowQueuedWriteToast,
  scheduleQueuedWriteFlush,
  scheduleQueuedSubmit,
  flushQueuedWrites
} from './content/ui/terminal-write-guard.js'

// =============================================================================
// TERMINAL PANEL STATE
// =============================================================================

let rootEl: HTMLDivElement | null = null
let terminalShellEl: HTMLDivElement | null = null
let terminalBodyEl: HTMLDivElement | null = null
let statusDotEl: HTMLSpanElement | null = null
let minimizeButtonEl: HTMLButtonElement | null = null
let runtimeListenerInstalled = false
let storageListenerInstalled = false
let unloadListenerInstalled = false
let panelReady = false
let pendingSandboxError: { message: string; instruction: string; command: string } | null = null
let panelCloseIntent: TerminalUIState | 'clear' | null = null
let presencePort: chrome.runtime.Port | null = null
let rootFolderBar: { element: HTMLDivElement; setRoot: (root: string) => void } | null = null

/** Backoff before re-announcing presence after a service-worker restart. */
const PRESENCE_RECONNECT_DELAY_MS = 500

function getHostTabIdFromLocation(): number | undefined {
  try {
    const raw = new URLSearchParams(globalThis.location?.search ?? '').get('tabId')
    if (!raw) return undefined
    const parsed = Number(raw)
    return Number.isFinite(parsed) ? parsed : undefined
  } catch {
    return undefined
  }
}

async function getHostTabId(): Promise<number | undefined> {
  const fromLocation = getHostTabIdFromLocation()
  if (fromLocation !== undefined) return fromLocation
  if (!chrome.tabs?.query) return undefined
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true })
    return tab?.id
  } catch {
    return undefined
  }
}

/**
 * Close the browser side panel.
 *
 * `chrome.sidePanel.close()` only exists in very recent Chrome. The old code
 * bailed out silently when it was missing, so the close button did nothing at
 * all — combined with unmountPanel() that left a blank panel the user could not
 * close *or* recover. `window.close()` works from the panel document itself on
 * every version that has side panels, so it is the fallback and the last word.
 */
async function closeBrowserSidePanel(): Promise<void> {
  if (chrome.sidePanel?.close) {
    const tabId = await getHostTabId()
    if (tabId !== undefined) {
      try {
        await chrome.sidePanel.close({ tabId })
        return
      } catch {
        // Fall through to window.close().
      }
    }
  }
  try {
    window.close()
  } catch {
    // Nothing else to try; the panel stays open but remains usable.
  }
}

function setPanelVisible(visible: boolean): void {
  state.visible = visible
  if (!rootEl) return
  rootEl.style.opacity = visible ? '1' : '0'
  rootEl.style.pointerEvents = visible ? 'auto' : 'none'
}

// The terminal is a full-height side panel; there is no collapse-in-place state
// (the old MINIMIZED_WIDGET_HEIGHT was leftover from the in-page-widget era and
// was never actually applied \u2014 every caller passed `visible: true`). The body is
// simply shown; "minimize" and "close" both dismiss the whole panel and keep the
// session (see closePanelWithIntent).
function showTerminalBody(): void {
  if (!terminalBodyEl || !terminalShellEl) return
  terminalBodyEl.style.display = 'block'
  terminalShellEl.style.height = '100%'
  terminalShellEl.style.minHeight = '0'
  terminalShellEl.style.flex = '1 1 auto'
}

/**
 * Show the recoverable no-session state in the terminal body.
 */
function showNoSessionState(): void {
  if (!terminalBodyEl) return
  renderNoSessionState(terminalBodyEl, () => { void bootTerminalPanel(true) })
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
  rootFolderBar = bar
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
 * Show a start failure, remembering it so a later remount can show it again.
 */
function showSandboxError(message: string, instruction: string, command: string): void {
  if (!terminalBodyEl) return
  pendingSandboxError = { message, instruction, command }
  renderStartFailure(terminalBodyEl, message, instruction, command)
}

function updateStatusDot(dotState: 'connected' | 'disconnected' | 'exited'): void {
  if (!statusDotEl) return
  switch (dotState) {
    case 'connected':
      statusDotEl.style.background = '#9ece6a'
      break
    case 'disconnected':
      statusDotEl.style.background = '#e0af68'
      break
    case 'exited':
      statusDotEl.style.background = '#f7768e'
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
      updateStatusDot('connected')
      state.terminalConnected = true
      if (state.queuedWrites.length > 0 && !state.queuedWriteInFlight) {
        scheduleQueuedWriteFlush(0)
      }
      break
    case 'disconnected':
      updateStatusDot('disconnected')
      state.terminalConnected = false
      state.terminalFocused = false
      break
    case 'exited':
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
 * Build one 24×24 icon button for the terminal header. All four header controls
 * (disconnect / redraw / minimize / close) share the same box, hover affordance,
 * and click-swallowing wrapper — only the id, glyph, tooltip, accent colour, and
 * action differ. One factory keeps them from drifting apart (repo rule 19/DRY).
 */
function createTerminalHeaderButton(opts: {
  id: string
  glyph: string
  title: string
  color: string
  fontSize?: string
  onClick: () => void
}): HTMLButtonElement {
  const button = document.createElement('button')
  button.id = opts.id
  button.textContent = opts.glyph
  button.title = opts.title
  button.type = 'button'
  Object.assign(button.style, {
    width: '24px',
    height: '24px',
    border: 'none',
    background: 'transparent',
    color: opts.color,
    fontSize: opts.fontSize ?? '14px',
    cursor: 'pointer',
    borderRadius: '4px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: '0'
  })
  button.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    opts.onClick()
  })
  return button
}

function createTerminalHeader(): HTMLDivElement {
  const header = document.createElement('div')
  header.id = HEADER_ID
  Object.assign(header.style, {
    height: '38px',
    background: '#16161e',
    display: 'flex',
    alignItems: 'center',
    padding: '0 10px 0 12px',
    gap: '8px',
    borderBottom: '1px solid #292e42',
    flexShrink: '0'
  })

  statusDotEl = document.createElement('span')
  statusDotEl.className = 'kaboom-terminal-status-dot'
  Object.assign(statusDotEl.style, {
    width: '8px',
    height: '8px',
    borderRadius: '50%',
    background: '#565f89',
    flexShrink: '0',
    transition: 'background 200ms ease'
  })

  const titleSpan = document.createElement('span')
  titleSpan.textContent = 'KaBOOM! Terminal'
  Object.assign(titleSpan.style, {
    color: '#d8dee9',
    fontSize: '12px',
    fontWeight: '600',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    userSelect: 'none'
  })

  const spacer = document.createElement('div')
  spacer.style.flex = '1'

  const disconnectButton = createTerminalHeaderButton({
    id: DISCONNECT_TERMINAL_BUTTON_ID,
    glyph: '\u23FB',
    title: 'End session — stops the shell and closes the panel',
    color: '#f7768e',
    fontSize: '12px',
    onClick: () => void exitTerminalSession()
  })

  const redrawButton = createTerminalHeaderButton({
    id: REDRAW_TERMINAL_BUTTON_ID,
    glyph: '\u21BB',
    title: 'Redraw terminal graphics',
    color: '#565f89',
    onClick: () => void redrawTerminal()
  })

  minimizeButtonEl = createTerminalHeaderButton({
    id: MINIMIZE_TERMINAL_BUTTON_ID,
    glyph: '\u2581',
    title: 'Minimize terminal',
    color: '#565f89',
    onClick: () => void minimizePanel()
  })

  const closeButton = createTerminalHeaderButton({
    id: CLOSE_TERMINAL_BUTTON_ID,
    glyph: '\u2715',
    title: 'Close panel — the shell keeps running, reopen to come back',
    color: '#c0caf5',
    onClick: () => void closePanelKeepingSession()
  })

  header.appendChild(statusDotEl)
  header.appendChild(titleSpan)
  header.appendChild(disconnectButton)
  header.appendChild(spacer)
  header.appendChild(redrawButton)
  header.appendChild(minimizeButtonEl)
  // Rightmost, where every other close control on the platform lives.
  header.appendChild(closeButton)

  return header
}

function createPanelShell(token: string): HTMLDivElement {
  const root = document.createElement('div')
  root.id = WIDGET_ID
  Object.assign(root.style, {
    position: 'fixed',
    inset: '0',
    zIndex: '2147483644',
    display: 'flex',
    flexDirection: 'column',
    background: '#0f1117',
    color: '#e5e7eb',
    opacity: '1',
    pointerEvents: 'auto',
    transition: 'opacity 180ms ease'
  })

  const terminalShell = document.createElement('div')
  terminalShell.style.cssText = [
    'flex:1 1 auto',
    'height:100%',
    'min-height:0',
    'display:flex',
    'flex-direction:column',
    'background:#11131a'
  ].join(';')

  const header = createTerminalHeader()

  const terminalBody = document.createElement('div')
  terminalBody.id = TERMINAL_BODY_ID
  terminalBody.style.cssText = [
    'flex:1',
    'min-height:0',
    'display:block',
    'background:#1a1b26'
  ].join(';')

  if (token) {
    const iframe = document.createElement('iframe')
    iframe.id = IFRAME_ID
    iframe.src = `${getTerminalServerUrl(state.serverUrl)}/terminal?token=${encodeURIComponent(token)}`
    iframe.setAttribute('allow', 'clipboard-write')
    iframe.style.cssText = 'width:100%;height:100%;border:none;background:#1a1b26;display:block;'
    terminalBody.appendChild(iframe)
    state.iframeEl = iframe
  } else {
    state.iframeEl = null
  }

  terminalShell.appendChild(header)
  // Above the terminal, always visible: the working directory is the single
  // most consequential thing about a shell, and it used to be invisible unless
  // the session had failed to start.
  terminalShell.appendChild(createRootFolderBarElement())
  terminalShell.appendChild(terminalBody)

  root.appendChild(terminalShell)

  terminalShellEl = terminalShell
  terminalBodyEl = terminalBody
  state.widgetEl = root

  return root
}

function mountPanel(root: HTMLDivElement): void {
  if (rootEl) return
  rootEl = root
  const target = document.body || document.documentElement
  if (!target) return
  target.appendChild(rootEl)
  setPanelVisible(true)
  state.visible = true
  window.addEventListener('message', handleIframeMessage)
}

function unmountPanel(): void {
  if (rootEl) {
    rootEl.remove()
    rootEl = null
  }
  terminalShellEl = null
  terminalBodyEl = null
  statusDotEl = null
  minimizeButtonEl = null
  rootFolderBar = null
  state.widgetEl = null
  state.iframeEl = null
  panelReady = false
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
  iframe.src = `${getTerminalServerUrl(state.serverUrl)}/terminal?token=${encodeURIComponent(currentToken)}`
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
  panelCloseIntent = intent
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
  panelCloseIntent = 'clear'
  if (state.sessionState) {
    try {
      const termUrl = getTerminalServerUrl(state.serverUrl)
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
    if (state.queuedWrites.length >= 200) {
      state.queuedWrites.shift()
    }
    state.queuedWrites.push(text)
    if (typing) maybeShowQueuedWriteToast()
    scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS)
    return
  }
  if (state.queuedWriteInFlight) {
    if (state.queuedWrites.length >= 200) {
      state.queuedWrites.shift()
    }
    state.queuedWrites.push(text)
    return
  }
  state.queuedWriteInFlight = true
  notifyIframe('redraw')
  notifyIframe('write', { text })
  scheduleQueuedSubmit(TERMINAL_WRITE_SUBMIT_DELAY_MS)
}

function installRuntimeListener(): void {
  if (runtimeListenerInstalled) return
  runtimeListenerInstalled = true
  chrome.runtime.onMessage.addListener((message: { type?: string; text?: string }, sender: chrome.runtime.MessageSender) => {
    if (sender.id !== chrome.runtime.id) return false
    // The background cannot close a side panel document on every Chrome version,
    // but this document can, so it asks us to.
    if (message.type === 'close_terminal_panel') {
      void closePanelKeepingSession()
      return false
    }
    if (message.type !== 'terminal_panel_write') return false
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
  if (presencePort || typeof chrome === 'undefined' || !chrome.runtime?.connect) return
  try {
    const port = chrome.runtime.connect({ name: TERMINAL_PANEL_PORT })
    presencePort = port
    port.onMessage.addListener((message: { type?: string }) => {
      if (message?.type === 'close_terminal_panel') void closePanelKeepingSession()
      if (message?.type === 'restore_terminal_panel') void restoreTerminalPanel()
    })
    port.onDisconnect.addListener(() => {
      presencePort = null
      // Only worth retrying while this document is still around and the runtime
      // is still valid; both stop being true on teardown, and connect() throws.
      if (!rootEl) return
      setTimeout(connectPresencePort, PRESENCE_RECONNECT_DELAY_MS)
    })
  } catch {
    presencePort = null // Extension context invalidated — nothing to announce to.
  }
}

function installStorageListener(): void {
  if (storageListenerInstalled) return
  storageListenerInstalled = true
  onStorageChanged((changes, areaName) => {
    if (areaName !== 'session') return
    const change = changes[StorageKey.TERMINAL_UI_STATE]
    if (!change) return
    const uiState = change.newValue as TerminalUIState | undefined
    if (uiState === 'closed') {
      state.visible = false
      if (rootEl) rootEl.style.opacity = '0'
      return
    }
    state.visible = true
    if (rootEl) rootEl.style.opacity = '1'
  })
}

function installUnloadListener(): void {
  if (unloadListenerInstalled) return
  unloadListenerInstalled = true
  window.addEventListener('pagehide', () => {
    if (panelCloseIntent !== null) return
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
  panelCloseIntent = null
  if (rootEl && state.iframeEl) {
    // A live terminal is already mounted — it was only minimized or hidden.
    setPanelVisible(true)
    showTerminalBody()
    state.minimized = false
    persistUIState('open')
    // Nothing here was rebuilt, so a root changed elsewhere (the options page,
    // another panel) would otherwise still read as the old one.
    void getTerminalDevRoot().then((root) => rootFolderBar?.setRoot(root))
    return
  }
  // No terminal in here: either unmounted, or mounted with no session (the
  // daemon was down when it booted). Rebuild, revalidating the token so the
  // xterm reconnects to a shell that is actually alive rather than rendering a
  // dead one.
  if (rootEl) unmountPanel()
  panelReady = false
  await bootTerminalPanel()
}

async function ensureTerminalSession(): Promise<void> {
  const persisted = await loadPersistedSession()
  if (persisted.session) {
    const alive = await validateSession(persisted.session.token)
    if (alive) {
      state.sessionState = persisted.session
      state.minimized = false
      return
    }
    clearPersistedSession()
  }
  const config = await getTerminalConfig()
  const ss = await startSession(config, showSandboxError)
  if (!ss) return
  state.sessionState = ss
  state.minimized = false
}

async function bootTerminalPanel(forceFresh = false): Promise<void> {
  if (panelReady && !forceFresh) return
  // Drop the existing panel DOM before (re)building. mountPanel() early-returns
  // while `rootEl` is set, so without this the freshly-built shell — bound to the
  // NEW session in the just-selected folder — is never attached, and the user
  // keeps staring at the old panel wired to the session we just stopped (the
  // "terminal won't start after picking a folder" bug, and the retry button).
  // Done before `panelReady = true` because unmountPanel() clears that flag;
  // restoreTerminalPanel already unmounts, applyRootFolder did not.
  if (forceFresh) unmountPanel()
  panelReady = true
  panelCloseIntent = null
  pendingSandboxError = null
  state.serverUrl = await getServerUrl()
  installRuntimeListener()
  installStorageListener()
  installUnloadListener()
  connectPresencePort()
  if (forceFresh) {
    resetAllState()
    state.serverUrl = await getServerUrl()
  }
  await ensureTerminalSession()
  const token = state.sessionState?.token
  const root = createPanelShell(token ?? '')
  mountPanel(root)
  showTerminalBody()
  persistUIState('open')
  if (!token) {
    const error = pendingSandboxError as { message: string; instruction: string; command: string } | null
    if (error) {
      showSandboxError(error.message, error.instruction, error.command)
    } else {
      showNoSessionState()
    }
  }
}

if (typeof document !== 'undefined' && typeof (globalThis as Record<string, unknown>).process === 'undefined') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
      void bootTerminalPanel()
    })
  } else {
    void bootTerminalPanel()
  }
}

export const _terminalPanelForTests = {
  bootTerminalPanel,
  applyRootFolder,
  writeToTerminal,
  exitTerminalSession,
  redrawTerminal
}

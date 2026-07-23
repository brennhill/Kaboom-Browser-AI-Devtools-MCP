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
  DISCONNECT_TERMINAL_BUTTON_ID,
  CLOSE_TERMINAL_BUTTON_ID,
  REDRAW_TERMINAL_BUTTON_ID,
  MINIMIZE_TERMINAL_BUTTON_ID,
  START_TERMINAL_BUTTON_ID,
  ROOT_FOLDER_INPUT_ID,
  ROOT_FOLDER_SAVE_BUTTON_ID,
  MINIMIZED_WIDGET_HEIGHT,
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

// =============================================================================
// WRITE GUARD — defer queued writes while user is typing in the terminal
// =============================================================================

function resetWriteGuardState(): void {
  state.queuedWrites = []
  state.terminalFocused = false
  state.lastTypingAt = 0
  state.queuedWriteInFlight = false
  state.lastGuardToastAt = 0
  if (state.queuedWriteFlushTimer !== null) {
    clearTimeout(state.queuedWriteFlushTimer)
    state.queuedWriteFlushTimer = null
  }
  if (state.queuedSubmitTimer !== null) {
    clearTimeout(state.queuedSubmitTimer)
    state.queuedSubmitTimer = null
  }
}

function shouldDeferQueuedWrite(nowMs = Date.now()): boolean {
  if (!state.terminalFocused) return false
  return nowMs - state.lastTypingAt < TERMINAL_TYPING_IDLE_MS
}

function maybeShowQueuedWriteToast(nowMs = Date.now()): void {
  if (nowMs - state.lastGuardToastAt < TERMINAL_GUARD_TOAST_INTERVAL_MS) return
  state.lastGuardToastAt = nowMs
  showActionToast('waiting for user to stop typing', 'Queued terminal action', 'warning', 1800)
}

function scheduleQueuedWriteFlush(delayMs = 0): void {
  if (state.queuedWriteFlushTimer !== null) clearTimeout(state.queuedWriteFlushTimer)
  state.queuedWriteFlushTimer = setTimeout(() => {
    state.queuedWriteFlushTimer = null
    flushQueuedWrites()
  }, delayMs)
}

function scheduleQueuedSubmit(delayMs: number): void {
  if (state.queuedSubmitTimer !== null) clearTimeout(state.queuedSubmitTimer)
  state.queuedSubmitTimer = setTimeout(() => {
    state.queuedSubmitTimer = null
    if (!state.visible || !state.iframeEl) {
      resetWriteGuardState()
      return
    }
    if (!state.terminalConnected) {
      scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS)
      return
    }
    if (shouldDeferQueuedWrite()) {
      maybeShowQueuedWriteToast()
      scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS)
      return
    }
    notifyIframe('write', { text: '\r' })
    notifyIframe('focus')
    state.queuedWriteInFlight = false
    if (state.queuedWrites.length > 0) {
      scheduleQueuedWriteFlush(0)
    }
  }, delayMs)
}

function flushQueuedWrites(): void {
  if (!state.visible || !state.iframeEl) {
    resetWriteGuardState()
    return
  }
  if (!state.terminalConnected) {
    scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS)
    return
  }
  if (state.queuedWriteInFlight) return
  if (state.queuedWrites.length === 0) {
    state.lastGuardToastAt = 0
    return
  }
  if (shouldDeferQueuedWrite()) {
    maybeShowQueuedWriteToast()
    scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS)
    return
  }

  const nextWrite = state.queuedWrites.shift()
  if (!nextWrite) return

  state.lastGuardToastAt = 0
  state.queuedWriteInFlight = true
  notifyIframe('redraw')
  notifyIframe('write', { text: nextWrite })
  scheduleQueuedSubmit(TERMINAL_WRITE_SUBMIT_DELAY_MS)
}

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

function setTerminalBodyVisible(visible: boolean): void {
  if (!terminalBodyEl || !terminalShellEl || !minimizeButtonEl) return
  terminalBodyEl.style.display = visible ? 'block' : 'none'
  terminalShellEl.style.height = visible ? '100%' : `${MINIMIZED_WIDGET_HEIGHT}px`
  terminalShellEl.style.minHeight = visible ? '0' : `${MINIMIZED_WIDGET_HEIGHT}px`
  terminalShellEl.style.flex = visible ? '1 1 auto' : `0 0 ${MINIMIZED_WIDGET_HEIGHT}px`
  minimizeButtonEl.textContent = visible ? '\u2581' : '\u25A1'
  minimizeButtonEl.title = visible ? 'Minimize terminal' : 'Restore terminal'
}

/**
 * Render a recoverable "no shell" state.
 *
 * The panel used to print a dead sentence and stop, so an ended or failed
 * session left a panel with nothing to click — the user could neither retry nor
 * change anything without digging through the options page. Every path out of
 * here is in the panel itself.
 */
function showNoSessionState(): void {
  if (!terminalBodyEl) return
  terminalBodyEl.replaceChildren()

  const wrap = document.createElement('div')
  Object.assign(wrap.style, {
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
    padding: '16px',
    color: '#a9b1d6',
    fontSize: '12px'
  })

  const msg = document.createElement('div')
  msg.textContent = 'No terminal session. Start one below, or check that the KaBOOM! daemon is running.'
  msg.style.color = '#c0caf5'

  const startButton = document.createElement('button')
  startButton.id = START_TERMINAL_BUTTON_ID
  startButton.textContent = 'Start terminal'
  startButton.type = 'button'
  Object.assign(startButton.style, {
    padding: '8px 12px',
    borderRadius: '8px',
    border: '1px solid #7aa2f7',
    background: '#1a1b26',
    color: '#7aa2f7',
    cursor: 'pointer',
    fontSize: '12px',
    alignSelf: 'flex-start'
  })
  startButton.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    void bootTerminalPanel(true)
  })

  wrap.appendChild(msg)
  wrap.appendChild(startButton)
  wrap.appendChild(createRootFolderControl())
  terminalBodyEl.appendChild(wrap)
}

/**
 * Root-folder editor. The working directory used to be reachable only from the
 * options page, which is a long way to go to point a shell at a different repo.
 * Changing it restarts the session, because a PTY's cwd is fixed at spawn.
 */
function createRootFolderControl(): HTMLDivElement {
  const box = document.createElement('div')
  Object.assign(box.style, { display: 'flex', flexDirection: 'column', gap: '6px' })

  const label = document.createElement('label')
  label.textContent = 'Root folder'
  label.htmlFor = ROOT_FOLDER_INPUT_ID
  Object.assign(label.style, { color: '#787c99', fontSize: '11px' })

  const row = document.createElement('div')
  Object.assign(row.style, { display: 'flex', gap: '6px' })

  const input = document.createElement('input')
  input.id = ROOT_FOLDER_INPUT_ID
  input.type = 'text'
  input.placeholder = '/path/to/your/project'
  Object.assign(input.style, {
    flex: '1',
    padding: '6px 8px',
    borderRadius: '6px',
    border: '1px solid #292e42',
    background: '#16161e',
    color: '#c0caf5',
    fontSize: '12px',
    minWidth: '0'
  })
  void getTerminalDevRoot().then((root) => {
    input.value = root
  })

  const save = document.createElement('button')
  save.id = ROOT_FOLDER_SAVE_BUTTON_ID
  save.textContent = 'Use'
  save.type = 'button'
  Object.assign(save.style, {
    padding: '6px 10px',
    borderRadius: '6px',
    border: '1px solid #9ece6a',
    background: '#1a1b26',
    color: '#9ece6a',
    cursor: 'pointer',
    fontSize: '12px',
    flexShrink: '0'
  })
  save.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    void applyRootFolder(input.value.trim())
  })

  row.appendChild(input)
  row.appendChild(save)
  box.appendChild(label)
  box.appendChild(row)
  return box
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

function showSandboxError(message: string, instruction: string, command: string): void {
  if (!terminalBodyEl) return
  pendingSandboxError = { message, instruction, command }
  terminalBodyEl.replaceChildren()

  const overlay = document.createElement('div')
  Object.assign(overlay.style, {
    display: 'flex',
    flexDirection: 'column',
    gap: '10px',
    padding: '16px',
    borderRadius: '12px',
    background: '#1a1b26',
    border: '1px solid #f7768e',
    color: '#a9b1d6',
    margin: '16px'
  })

  const title = document.createElement('div')
  title.textContent = 'Terminal unavailable'
  Object.assign(title.style, {
    color: '#f7768e',
    fontWeight: '600',
    fontSize: '14px'
  })

  const msg = document.createElement('div')
  msg.textContent = message
  Object.assign(msg.style, {
    fontSize: '12px',
    color: '#787c99'
  })

  const inst = document.createElement('div')
  inst.textContent = instruction
  inst.style.fontSize = '12px'

  const cmdBox = document.createElement('div')
  Object.assign(cmdBox.style, {
    background: '#16161e',
    border: '1px solid #292e42',
    borderRadius: '8px',
    padding: '10px 12px',
    fontFamily: '"SF Mono", "Fira Code", Menlo, Monaco, monospace',
    fontSize: '12px',
    color: '#9ece6a'
  })
  cmdBox.textContent = command

  overlay.appendChild(title)
  overlay.appendChild(msg)
  // Not every start failure has a remedy to offer; empty boxes would just be noise.
  if (instruction) overlay.appendChild(inst)
  if (command) overlay.appendChild(cmdBox)
  terminalBodyEl.appendChild(overlay)
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

function notifyIframe(command: string, data: Record<string, unknown> = {}): void {
  if (!state.iframeEl?.contentWindow) return
  state.iframeEl.contentWindow.postMessage({ command, ...data }, '*')
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

  const disconnectButton = document.createElement('button')
  disconnectButton.id = DISCONNECT_TERMINAL_BUTTON_ID
  disconnectButton.textContent = '\u23FB'
  disconnectButton.title = 'End session — stops the shell and closes the panel'
  disconnectButton.type = 'button'
  Object.assign(disconnectButton.style, {
    width: '24px',
    height: '24px',
    border: 'none',
    background: 'transparent',
    color: '#f7768e',
    fontSize: '12px',
    cursor: 'pointer',
    borderRadius: '4px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: '0'
  })
  disconnectButton.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    void exitTerminalSession()
  })

  const redrawButton = document.createElement('button')
  redrawButton.id = REDRAW_TERMINAL_BUTTON_ID
  redrawButton.textContent = '\u21BB'
  redrawButton.title = 'Redraw terminal graphics'
  redrawButton.type = 'button'
  Object.assign(redrawButton.style, {
    width: '24px',
    height: '24px',
    border: 'none',
    background: 'transparent',
    color: '#565f89',
    fontSize: '14px',
    cursor: 'pointer',
    borderRadius: '4px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: '0'
  })
  redrawButton.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    redrawTerminal()
  })

  minimizeButtonEl = document.createElement('button')
  minimizeButtonEl.id = MINIMIZE_TERMINAL_BUTTON_ID
  minimizeButtonEl.textContent = '\u2581'
  minimizeButtonEl.title = 'Minimize terminal'
  minimizeButtonEl.type = 'button'
  Object.assign(minimizeButtonEl.style, {
    width: '24px',
    height: '24px',
    border: 'none',
    background: 'transparent',
    color: '#565f89',
    fontSize: '14px',
    cursor: 'pointer',
    borderRadius: '4px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: '0'
  })
  minimizeButtonEl.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    void minimizePanel()
  })

  const closeButton = document.createElement('button')
  closeButton.id = CLOSE_TERMINAL_BUTTON_ID
  closeButton.textContent = '\u2715'
  closeButton.title = 'Close panel — the shell keeps running, reopen to come back'
  closeButton.type = 'button'
  Object.assign(closeButton.style, {
    width: '24px',
    height: '24px',
    border: 'none',
    background: 'transparent',
    color: '#c0caf5',
    fontSize: '14px',
    cursor: 'pointer',
    borderRadius: '4px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: '0'
  })
  closeButton.addEventListener('click', (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    void closePanelKeepingSession()
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
  state.widgetEl = null
  state.iframeEl = null
  panelReady = false
  setPanelVisible(false)
  window.removeEventListener('message', handleIframeMessage)
}

function redrawTerminal(): void {
  if (!state.widgetEl || !state.iframeEl) return
  const currentToken = state.sessionState?.token
  if (!currentToken) return
  const iframe = state.iframeEl
  iframe.src = `${getTerminalServerUrl(state.serverUrl)}/terminal?token=${encodeURIComponent(currentToken)}`
  setTerminalBodyVisible(true)
  state.minimized = false
  persistUIState('open')
}

async function exitTerminalSession(): Promise<void> {
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
  clearPersistedSession()
  resetAllState()
  resetWriteGuardState()
  unmountPanel()
  await closeBrowserSidePanel()
}

/**
 * Close the drawer and leave the shell running.
 *
 * This is what "close" has to mean for a terminal: exitTerminalSession() kills
 * the PTY, so a user who just wanted the panel out of the way lost their shell
 * and had no way back. Reopening the panel reconnects to this session.
 */
async function closePanelKeepingSession(): Promise<void> {
  panelCloseIntent = 'closed'
  persistUIState('closed')
  resetWriteGuardState()
  unmountPanel()
  await closeBrowserSidePanel()
}

async function minimizePanel(): Promise<void> {
  panelCloseIntent = 'minimized'
  persistUIState('minimized')
  resetWriteGuardState()
  unmountPanel()
  await closeBrowserSidePanel()
}

function writeToTerminal(text: string): void {
  if (!state.visible || !state.iframeEl) return
  if (shouldDeferQueuedWrite()) {
    if (state.queuedWrites.length >= 200) {
      state.queuedWrites.shift()
    }
    state.queuedWrites.push(text)
    maybeShowQueuedWriteToast()
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
    setTerminalBodyVisible(true)
    state.minimized = false
    persistUIState('open')
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
  setTerminalBodyVisible(true)
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
  writeToTerminal,
  exitTerminalSession,
  redrawTerminal
}

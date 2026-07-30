/**
 * Purpose: Bridges the page hover launcher to the terminal side panel.
 * Why: Keeps the page overlay focused on quick actions while terminal visibility
 * and writes are coordinated through session state and runtime messages.
 * Docs: docs/features/feature/terminal/index.md
 */

import { StorageKey, TERMINAL_PANEL_FALLBACK_HINT, TERMINAL_PANEL_STALE_CONTEXT_HINT } from '../../lib/constants.js'
import { onStorageChanged } from '../../lib/storage/changes.js'
import { readSessionState } from '../../lib/storage/validated.js'
import { reportStateRecovery, resolveStateRecovery } from '../../lib/storage/recovery.js'
import { showActionToast } from './toast.js'

type VisibilityListener = (visible: boolean) => void

let panelVisible = false
let bridgeInitialized = false
let storageListenerInstalled = false
const visibilityListeners = new Set<VisibilityListener>()

// A single unacked write can be the side panel's brief BOOT WINDOW — the document
// exists (so the mirror is 'open') but its onMessage listener is not installed yet,
// so only the background sees the write and it goes unacked. Retrying once after a
// short delay distinguishes that (retry acks) from a genuinely gone panel (retry
// also misses) — which the TERMINAL_UI_STATE mirror alone cannot, since it reads
// 'open' in both cases. Only after the retry also misses do we conclude "gone" and
// reconcile the stale mirror.
const TERMINAL_PANEL_WRITE_RETRY_MS = 250
let writeRetryDelayMs = TERMINAL_PANEL_WRITE_RETRY_MS

// The single outstanding retry timer, and a generation stamped onto it. An
// untracked retry timer (finding J) fires after teardown or after a new session
// opens — re-sending a stale nudge and spuriously reconciling the visibility
// mirror. We keep exactly one retry, cancel it on teardown/supersede, and the
// retry no-ops if its generation is stale or the panel is no longer visible.
let writeRetryTimer: ReturnType<typeof setTimeout> | null = null
let writeGeneration = 0

function clearWriteRetry(): void {
  if (writeRetryTimer !== null) {
    clearTimeout(writeRetryTimer)
    writeRetryTimer = null
  }
}

function notifyVisibilityListeners(visible: boolean): void {
  for (const listener of visibilityListeners) {
    listener(visible)
  }
}

function setPanelVisible(nextVisible: boolean): void {
  if (panelVisible === nextVisible) return
  panelVisible = nextVisible
  notifyVisibilityListeners(panelVisible)
}

async function syncPanelVisibilityFromStorage(): Promise<void> {
  const uiState = await readSessionState<'open' | 'closed' | 'minimized'>({
    key: StorageKey.TERMINAL_UI_STATE,
    fallback: 'closed',
    validate: (value): value is 'open' | 'closed' | 'minimized' =>
      value === 'open' || value === 'closed' || value === 'minimized',
    diagnostic: {
      name: 'terminal_session_state',
      detail: 'Saved terminal panel visibility was invalid or unreadable; closed state is active.',
      fix: 'Reopen the terminal panel.'
    }
  })
  setPanelVisible(uiState === 'open')
}

function installStorageListener(): void {
  if (storageListenerInstalled) return
  storageListenerInstalled = true
  onStorageChanged((changes, areaName) => {
    if (areaName !== 'session') return
    const change = changes[StorageKey.TERMINAL_UI_STATE]
    if (!change) return
    const nextValue = change.newValue
    if (nextValue !== undefined && nextValue !== 'open' && nextValue !== 'closed' && nextValue !== 'minimized') {
      reportStateRecovery({
        name: 'terminal_session_state',
        detail: 'Terminal visibility changed to an invalid value; closed state is active.',
        fix: 'Reopen the terminal panel.'
      })
      setPanelVisible(false)
      return
    }
    resolveStateRecovery('terminal_session_state')
    setPanelVisible(nextValue === 'open')
  })
}

export async function initTerminalPanelBridge(): Promise<void> {
  if (bridgeInitialized) return
  bridgeInitialized = true
  installStorageListener()
  await syncPanelVisibilityFromStorage()
}

export function isTerminalVisible(): boolean {
  return panelVisible
}

export function onTerminalPanelVisibilityChanged(listener: VisibilityListener): () => void {
  visibilityListeners.add(listener)
  return () => {
    visibilityListeners.delete(listener)
  }
}

/**
 * Report why the side panel refused to open.
 *
 * This used to be a bare `catch { return false }`, so a rejected
 * `chrome.sidePanel.open()` left no trace anywhere — the Terminal button looked
 * simply dead. console.error is deliberate: the daemon captures page errors, so
 * the Chrome message becomes retrievable via observe(what:"errors").
 */
function reportPanelOpenFailure(reason: string): void {
  // Chrome grants message listeners only a restricted user gesture, which
  // sidePanel.open() rejects on some builds (crbug 355266358). Both fallbacks
  // are gesture-native, so point at them rather than leaving a dead end — unless
  // the page itself is the problem, in which case only a reload fixes it.
  const stale = isStaleContextError(reason)
  const hint = stale ? TERMINAL_PANEL_STALE_CONTEXT_HINT : TERMINAL_PANEL_FALLBACK_HINT
  // The raw Chrome error always reaches the console — it is the only diagnostic
  // signal, and the daemon captures it. The toast drops it when it would only
  // add noise to advice the user can act on directly.
  console.error(`[KaBOOM!] Terminal side panel did not open: ${reason} ${hint}`)
  try {
    showActionToast('Terminal side panel did not open', stale ? hint : `${reason} ${hint}`, 'error', 8000)
  } catch {
    // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
    // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
    // Toast is best-effort; the console error above is the durable signal.
  }
}

/**
 * Whether this page has been cut off from the extension for good.
 *
 * Reloading the extension orphans the content script in every tab that was
 * already open; from then on every runtime call throws this. It is not a
 * terminal fault and no terminal advice applies.
 */
function isStaleContextError(reason: string): boolean {
  return reason.includes('Extension context invalidated')
}

export async function openTerminalPanel(): Promise<boolean> {
  try {
    const result = (await chrome.runtime.sendMessage({ type: 'open_terminal_panel' })) as
      | { success?: boolean; error?: string }
      | undefined
    if (result?.success === true) return true
    reportPanelOpenFailure(result?.error ?? 'the background service worker sent no response')
    return false
  } catch (err) {
    reportPanelOpenFailure(err instanceof Error ? err.message : String(err))
    return false
  }
}

/**
 * Surface a terminal write that did not land, and reconcile the visibility mirror.
 *
 * The write is a runtime message to the side-panel DOCUMENT; a missing ack means
 * no panel received it — typically the panel was closed with Chrome's own X while
 * the TERMINAL_UI_STATE mirror we gate on stayed 'open' (rule 18). Fail loud
 * (rule 25): log + best-effort toast so a vanished annotation nudge is not
 * silent. `reconcile` corrects the stale mirror so the gate stops firing into the
 * void — done only when the miss proves the panel is gone (no ack), not on a
 * transient/ambiguous transport error.
 */
function reportTerminalWriteFailure(text: string, reason: string, reconcile: boolean): void {
  console.warn(`[KaBOOM!] Terminal write did not land ("${text.slice(0, 40)}"): ${reason}`)
  if (reconcile) setPanelVisible(false)
  try {
    showActionToast('Terminal did not receive the message', 'Open the terminal panel and try again', 'warning', 5000)
  } catch {
    // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
    // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
    // Toast is best-effort; the console warning above is the durable signal.
  }
}

export function writeToTerminal(text: string): void {
  if (!panelVisible) return
  // A new write supersedes any pending retry: bump the generation and cancel the
  // outstanding timer so an older session's retry cannot fire alongside this one.
  writeGeneration += 1
  clearWriteRetry()
  sendTerminalWrite(text, true)
}

/**
 * Send one terminal_panel_write attempt. `allowRetry` gives the panel's boot
 * window a second chance before we treat a missing ack as a gone panel: on the
 * first miss we retry once after writeRetryDelayMs; only when the retry also
 * misses do we fail loud and reconcile the (possibly stale) visibility mirror.
 */
function sendTerminalWrite(text: string, allowRetry: boolean): void {
  const attemptGeneration = writeGeneration
  let pending: Promise<{ received?: boolean } | undefined> | undefined
  try {
    pending = chrome.runtime.sendMessage({ type: 'terminal_panel_write', text }) as
      | Promise<{ received?: boolean } | undefined>
      | undefined
  } catch (err) {
    // Synchronous throw = extension context invalidated; the content script is
    // orphaned and only a page reload restores it. The panel state is unknown, so
    // do not reconcile the mirror — just fail loud instead of dropping silently.
    reportTerminalWriteFailure(text, err instanceof Error ? err.message : String(err), false)
    return
  }
  if (!pending || typeof pending.then !== 'function') return
  pending.then(
    (resp) => {
      // Only the panel document acks this type (the background never replies).
      if (resp && resp.received === true) return
      if (allowRetry) {
        if (attemptGeneration !== writeGeneration) {
          // EXPECTED_ABSENCE: discarding a teardown-superseded response is normal;
          // logging it would misleadingly attribute an old miss to the fresh session.
          return
        }
        // Possibly the panel's boot window — retry once before concluding it is gone.
        scheduleWriteRetry(text)
        return
      }
      // Retry also missed: no panel received it. Fail loud + reconcile the mirror.
      reportTerminalWriteFailure(text, 'no terminal panel received the message', true)
    },
    (err: unknown) => {
      // Transport error. Retry once (transient); if it recurs, fail loud but do NOT
      // reconcile — an ambiguous transport error is not proof the panel is gone.
      if (allowRetry) {
        if (attemptGeneration !== writeGeneration) {
          // EXPECTED_ABSENCE: discarding a teardown-superseded result is normal;
          // logging it would misleadingly diagnose an intentionally stale retry.
          return
        }
        scheduleWriteRetry(text)
        return
      }
      reportTerminalWriteFailure(text, err instanceof Error ? err.message : String(err), false)
    }
  )
}

/**
 * Schedule the single retry for `text`, tracked so teardown/supersede can cancel
 * it, and guarded so it no-ops if the bridge state moved on since it was scheduled
 * (finding J): a stale retry must never re-send into a new session or reconcile a
 * panel it no longer describes.
 */
function scheduleWriteRetry(text: string): void {
  clearWriteRetry()
  const generation = writeGeneration
  writeRetryTimer = setTimeout(() => {
    writeRetryTimer = null
    // EXPECTED_ABSENCE: stale timer cancellation is normal after reset or a newer
    // send; logging it would misleadingly diagnose intentional session isolation.
    if (generation !== writeGeneration || !panelVisible) return
    sendTerminalWrite(text, false)
  }, writeRetryDelayMs)
}

export const _terminalPanelBridgeForTests = {
  reset(): void {
    panelVisible = false
    bridgeInitialized = false
    storageListenerInstalled = false
    writeRetryDelayMs = TERMINAL_PANEL_WRITE_RETRY_MS
    visibilityListeners.clear()
    // Cancel any pending retry and bump the generation so an in-flight one no-ops:
    // teardown must not leave a timer that re-sends or reconciles a new session (J).
    clearWriteRetry()
    writeGeneration += 1
  },
  setWriteRetryDelay(ms: number): void {
    writeRetryDelayMs = ms
  }
}

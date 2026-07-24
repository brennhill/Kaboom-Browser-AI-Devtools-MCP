/**
 * Purpose: Orchestrates popup initialization and binds UI modules for tracking, recording, draw mode, and pilot controls.
 * Why: Keeps popup behavior consistent by coordinating status/state hydration in one lifecycle entrypoint.
 * Docs: docs/features/feature/ai-web-pilot/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

/**
 * @fileoverview popup.ts - Extension popup UI showing connection status and controls.
 * Displays server connection state, entry count, error count, log level selector,
 * and log file path. Polls the background worker for status updates and provides
 * a clear-logs button. Shows troubleshooting hints when disconnected.
 * Design: Pure DOM manipulation, no framework. Communicates with background.js
 * via chrome.runtime.sendMessage for status queries and log-level changes.
 */

import type { WebSocketCaptureMode } from './types/index.js'
import type { components } from './generated/openapi-types.js'
import type { PopupConnectionStatus, ToggleWarningConfig } from './popup/types.js'
import type { ShowTrackedHoverLauncherMessage } from './types/runtime-messages.js'
import { RuntimeMessageName, StorageKey } from './lib/constants.js'
import { getLocal, getLocals, setSession, getSession, onStorageChanged } from './lib/storage-utils.js'
import { updateConnectionStatus } from './popup/status-display.js'
import { renderUpdateAvailableBanner } from './popup/update-button.js'
import { DEFAULT_SERVER_URL } from './lib/constants.js'
import { buildDaemonHeaders } from './lib/daemon-http.js'
import { setupRecordingUI } from './popup/recording.js'
import { setupDrawModeButton } from './popup/draw-mode.js'
import { setupActionRecordingUI } from './popup/action-recording.js'
import { FEATURE_TOGGLES as TOGGLE_DEFS, initFeatureToggles, applyFeatureToggles } from './popup/feature-toggles.js'
import { initTrackPageButton } from './popup/tab-tracking.js'
import { initAiWebPilotToggle, applyAiWebPilotToggle } from './popup/ai-web-pilot.js'
import { initPopupLogoMotion } from './popup/logo-motion.js'
import {
  initWebSocketModeSelector,
  applyWebSocketMode,
  handleWebSocketModeChange,
  handleClearLogs,
  resetClearConfirm
} from './popup/settings.js'

// Re-export for testing
export { resetClearConfirm, handleClearLogs }
export { updateConnectionStatus }
export { FEATURE_TOGGLES, initFeatureToggles, applyFeatureToggles } from './popup/feature-toggles.js'
export { handleFeatureToggle } from './popup/feature-toggles.js'
export { initAiWebPilotToggle, handleAiWebPilotToggle, applyAiWebPilotToggle } from './popup/ai-web-pilot.js'
export { initTrackPageButton, handleTrackPageClick } from './popup/tab-tracking.js'
export { handleWebSocketModeChange } from './popup/settings.js'
export { initWebSocketModeSelector, applyWebSocketMode } from './popup/settings.js'
export { isInternalUrl } from './popup/ui-utils.js'

// Apply theme early to prevent flash of unstyled content (moved from inline script for CSP compliance).
void getLocal('theme').then((value) => {
  if (value === 'light') document.body.classList.add('light-theme')
})

const DEFAULT_MAX_ENTRIES = 1000
const RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE: ShowTrackedHoverLauncherMessage = {
  type: RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER
}

/**
 * Bind a toggle element to show/hide a target element based on a condition.
 * Sets initial display state and adds a change listener.
 */
function bindToggleVisibility(
  toggle: HTMLInputElement | HTMLSelectElement,
  target: HTMLElement,
  isVisible: () => boolean
): void {
  target.style.display = isVisible() ? 'block' : 'none'
  toggle.addEventListener('change', () => {
    target.style.display = isVisible() ? 'block' : 'none'
  })
}

// #lizard forgives
function setupWebSocketUI(): void {
  const wsToggle = document.getElementById('toggle-websocket') as HTMLInputElement | null
  const wsModeContainer = document.getElementById('ws-mode-container')
  if (wsToggle && wsModeContainer) {
    bindToggleVisibility(wsToggle, wsModeContainer, () => wsToggle.checked)
  }

  const wsModeSelect = document.getElementById('ws-mode') as HTMLSelectElement | null
  if (wsModeSelect) {
    wsModeSelect.addEventListener('change', (e: Event) => {
      handleWebSocketModeChange((e.target as HTMLSelectElement).value as WebSocketCaptureMode)
    })
  }

  const wsMessagesWarning = document.getElementById('ws-messages-warning')
  if (wsModeSelect && wsMessagesWarning) {
    bindToggleVisibility(wsModeSelect, wsMessagesWarning, () => wsModeSelect.value === 'all')
  }
}

function setupToggleWarnings(): void {
  const toggleWarnings: ToggleWarningConfig[] = [
    { toggleId: 'toggle-screenshot', warningId: 'screenshot-warning' },
    { toggleId: 'toggle-network-waterfall', warningId: 'waterfall-warning' },
    { toggleId: 'toggle-performance-marks', warningId: 'perfmarks-warning' }
  ]
  for (const { toggleId, warningId } of toggleWarnings) {
    const toggle = document.getElementById(toggleId) as HTMLInputElement | null
    const warning = document.getElementById(warningId)
    if (toggle && warning) {
      warning.style.display = toggle.checked ? 'block' : 'none'
      toggle.addEventListener('change', () => {
        warning.style.display = toggle.checked ? 'block' : 'none'
      })
    }
  }
}

function requestTrackedHoverLauncherReshow(): void {
  if (!chrome.tabs?.query || !chrome.tabs?.sendMessage) return
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    const tabId = tabs[0]?.id
    if (!tabId) return
    chrome.tabs.sendMessage(tabId, RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE, () => {
      void chrome.runtime.lastError
    })
  })
}

/** Cache status to session storage so the popup renders instantly on next open. */
function cacheStatus(status: PopupConnectionStatus): void {
  void setSession(StorageKey.POPUP_LAST_STATUS, status)
}

/**
 * Initialize the popup.
 *
 * Optimized for instant first paint:
 * 1. HTML renders with default states (idle buttons, checked toggles from markup).
 * 2. One batched storage read fetches ALL keys in parallel.
 * 3. Results are applied synchronously in a single pass (no await chains).
 * 4. Non-critical init (logo, draw mode) deferred via requestAnimationFrame.
 */
export function initPopup(): void {
  // ── Immediate: wire up event listeners & sync UI (no async) ──────────

  // Recording rows are visible from HTML with idle defaults — no visibility:hidden.
  setupRecordingUI()
  setupActionRecordingUI()
  initTrackPageButton()
  setupWebSocketUI()
  setupToggleWarnings()

  const clearBtn = document.getElementById('clear-btn')
  if (clearBtn) clearBtn.addEventListener('click', handleClearLogs)

  // Listen for status updates
  chrome.runtime.onMessage.addListener(
    (message: { type: string; status?: PopupConnectionStatus; enabled?: boolean }) => {
      if (message.type === 'status_update' && message.status) {
        updateConnectionStatus(message.status)
        cacheStatus(message.status)
      }
    }
  )

  // Listen for storage changes (e.g., tracked tab URL updates)
  onStorageChanged((changes, areaName) => {
    if (areaName === 'local' && changes[StorageKey.TRACKED_TAB_URL]) {
      const urlEl = document.getElementById('tracking-bar-url')
      if (urlEl && changes[StorageKey.TRACKED_TAB_URL]!.newValue) {
        urlEl.textContent = changes[StorageKey.TRACKED_TAB_URL]!.newValue as string
        console.log('[KaBOOM!] Tracked tab URL updated in popup:', changes[StorageKey.TRACKED_TAB_URL]!.newValue)
      }
    }
  })

  // ── Cached status: hydrate from sessionStorage (sync read) ───────────
  void getSession(StorageKey.POPUP_LAST_STATUS).then((value) => {
    const cached = value as PopupConnectionStatus | undefined
    if (cached) updateConnectionStatus(cached)
  })

  // ── Fresh status: request from background (async IPC) ────────────────
  try {
    chrome.runtime.sendMessage({ type: 'get_status' }, (status: PopupConnectionStatus | undefined) => {
      if (chrome.runtime.lastError) {
        updateConnectionStatus({
          connected: false,
          entries: 0,
          maxEntries: DEFAULT_MAX_ENTRIES,
          errorCount: 0,
          logFile: '',
          error: 'Extension restarting — please wait a moment and reopen popup'
        })
        return
      }
      if (status) {
        updateConnectionStatus(status)
        cacheStatus(status)
      }
    })
  } catch {
    updateConnectionStatus({
      connected: false,
      entries: 0,
      maxEntries: DEFAULT_MAX_ENTRIES,
      errorCount: 0,
      logFile: '',
      error: 'Extension error — try reloading the extension'
    })
  }

  // ── One-shot health poll for the "Update available" banner ────────────
  void (async () => {
    try {
      const stored = (await getLocal(StorageKey.SERVER_URL)) as string | undefined
      const serverUrl = stored && stored.length > 0 ? stored : DEFAULT_SERVER_URL
      const resp = await fetch(`${serverUrl}/health`, { headers: buildDaemonHeaders() })
      if (!resp.ok) return
      const health = (await resp.json()) as components['schemas']['HealthResponse']
      await renderUpdateAvailableBanner(health)
    } catch {
      // Daemon unreachable — banner stays hidden; the connection-status
      // surface already communicates the offline state.
    }
  })()

  // ── Batched storage read: one call for ALL toggle/setting keys ────────
  const toggleKeys = TOGGLE_DEFS.map((t) => t.storageKey)
  const allKeys = [
    ...toggleKeys,
    StorageKey.WEBSOCKET_CAPTURE_MODE,
    StorageKey.AI_WEB_PILOT_ENABLED
  ]

  void getLocals(allKeys).then((result) => {
    // Apply feature toggles (9 checkboxes)
    applyFeatureToggles(result)

    // Apply WS mode selector
    applyWebSocketMode(result[StorageKey.WEBSOCKET_CAPTURE_MODE])

    // Apply AI Web Pilot toggle
    applyAiWebPilotToggle(result[StorageKey.AI_WEB_PILOT_ENABLED])
  })

  // ── Deferred: non-critical cosmetic init after first paint ───────────
  const deferredInit = (): void => {
    initPopupLogoMotion()
    setupDrawModeButton()
    requestTrackedHoverLauncherReshow()
  }
  if (typeof requestAnimationFrame === 'function') {
    requestAnimationFrame(deferredInit)
  } else {
    // Node.js test environment — run synchronously
    deferredInit()
  }
}

// Initialize when DOM is ready
if (typeof document !== 'undefined' && typeof (globalThis as Record<string, unknown>).process === 'undefined') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initPopup)
  } else {
    initPopup()
  }
}

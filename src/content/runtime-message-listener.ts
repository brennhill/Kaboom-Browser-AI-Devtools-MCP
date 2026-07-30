// runtime-message-listener.ts — Message routing between background and content contexts.

/**
 * Purpose: Installs the chrome.runtime.onMessage listener that routes background messages to content-script handlers with sender validation.
 * Docs: docs/features/feature/csp-safe-execution/index.md
 */

/**
 * @fileoverview Runtime Message Listener Module
 * Handles chrome.runtime messages from background script
 */

import type { ContentMessage } from '../types/runtime-messages.js'
import type { WebSocketCaptureMode } from '../types/capture/websocket.js'
import { KABOOM_LOG_PREFIX } from '../lib/brand.js'
import { SettingName } from '../lib/constants.js'
import { getLocals } from '../lib/storage/local.js'
import { reportStateRecovery, resolveStateRecovery } from '../lib/storage/recovery.js'
import {
  isValidBackgroundSender,
  handlePing,
  handleToggleMessage,
  forwardHighlightMessage,
  handleStateCommand,
  handleExecuteJs,
  handleExecuteQuery,
  handleA11yQuery,
  handleDomQuery,
  handleGetNetworkWaterfall,
  handleLinkHealthQuery,
  handleComputedStylesQuery,
  handleFormDiscoveryQuery,
  handleFormStateQuery,
  handleDataTableQuery,
  handleGetReadable,
  handleGetMarkdown,
  handlePageSummary
} from './message-handlers.js'
import { showActionToast } from './ui/toast.js'
import { showSubtitle, toggleRecordingWatermark } from './ui/subtitle.js'
import { toggleChatWidget } from './ui/chat-widget.js'

// Toggle state caches — updated by forwarded setting messages from background
let actionToastsEnabled = true
let subtitlesEnabled = true

function applyOverlayToggleState(result: Record<string, unknown>): void {
  const actionToasts = result.actionToastsEnabled
  const subtitles = result.subtitlesEnabled
  if (
    (actionToasts !== undefined && typeof actionToasts !== 'boolean') ||
    (subtitles !== undefined && typeof subtitles !== 'boolean')
  ) {
    reportStateRecovery({
      name: 'overlay_settings_state',
      detail: 'Saved overlay settings were malformed; enabled defaults are active.',
      fix: 'Open extension settings and save overlay preferences again.'
    })
    return
  }
  if (typeof actionToasts === 'boolean') actionToastsEnabled = actionToasts
  if (typeof subtitles === 'boolean') subtitlesEnabled = subtitles
  resolveStateRecovery('overlay_settings_state')
}

function hydrateOverlayToggleState(): void {
  void getLocals(['actionToastsEnabled', 'subtitlesEnabled'])
    .then(applyOverlayToggleState)
    .catch(() => {
      reportStateRecovery({
        name: 'overlay_settings_state',
        detail: 'Saved overlay settings could not be read; enabled defaults are active.',
        fix: 'Reload the extension, then save overlay preferences again.'
      })
    })
}

/**
 * Initialize runtime message listener
 * Listens for messages from background (feature toggles and pilot commands)
 */
export function initRuntimeMessageListener(): void {
  actionToastsEnabled = true
  subtitlesEnabled = true
  hydrateOverlayToggleState()
  /** Sync message handlers — return false (no async response needed) */
  type SyncMsg = ContentMessage & { enabled?: boolean; mode?: WebSocketCaptureMode; url?: string; params?: unknown }
  const syncHandlers: Record<string, (msg: SyncMsg) => false | void> = {
    kaboom_ping: () => {
      /* handled below via sendResponse */
    },
    kaboom_action_toast: (msg) => {
      if (!actionToastsEnabled) return false
      const m = msg as {
        text?: string
        detail?: string
        state?: 'trying' | 'success' | 'warning' | 'error'
        duration_ms?: number
      }
      if (m.text) showActionToast(m.text, m.detail, m.state || 'trying', m.duration_ms)
      return false
    },
    kaboom_toggle_chat: (msg) => {
      toggleChatWidget((msg as { client_name?: string }).client_name)
      return false
    },
    kaboom_recording_watermark: (msg) => {
      toggleRecordingWatermark((msg as { visible?: boolean }).visible ?? false)
      return false
    },
    kaboom_subtitle: (msg) => {
      if (!subtitlesEnabled) return false
      showSubtitle((msg as { text?: string }).text ?? '')
      return false
    },
    [SettingName.ACTION_TOASTS]: (msg) => {
      actionToastsEnabled = (msg as { enabled: boolean }).enabled
      return false
    },
    [SettingName.SUBTITLES]: (msg) => {
      subtitlesEnabled = (msg as { enabled: boolean }).enabled
      return false
    }
  }

  /** Delegated handlers — return boolean | undefined (some are async, returning true) */
  type DelegatedHandler = (msg: SyncMsg, sendResponse: (r?: unknown) => void) => boolean | undefined
  const delegatedHandlers: Record<string, DelegatedHandler> = {
    kaboom_draw_mode_start: (msg, sr) => {
      const m = msg as { started_by?: string; annot_session_name?: string; correlation_id?: string }
      import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
        .then((mod) => {
          const result = mod.activateDrawMode(
            m.started_by || 'user',
            m.annot_session_name || '',
            m.correlation_id || ''
          )
          sr(result)
        })
        .catch((e: Error) => sr({ error: 'draw_mode_load_failed', message: e.message }))
      return true
    },
    kaboom_draw_mode_stop: (_msg, sr) => {
      import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
        .then((mod) => {
          const result = mod.deactivateAndSendResults?.() || mod.deactivateDrawMode?.()
          sr(result || { status: 'stopped' })
        })
        .catch((e: Error) => sr({ error: 'draw_mode_load_failed', message: e.message }))
      return true
    },
    kaboom_get_annotations: (_msg, sr) => {
      import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
        .then((mod) => {
          sr({ draw_mode_active: mod.isDrawModeActive?.() ?? false })
        })
        .catch(() => sr({ draw_mode_active: false }))
      return true
    },
    kaboom_highlight: (msg, sr) => {
      forwardHighlightMessage({ params: msg.params as { selector: string; duration_ms?: number } })
        .then((r) => sr(r))
        .catch((e: Error) => sr({ success: false, error: e.message }))
      return true
    },
    kaboom_manage_state: (msg, sr) => {
      handleStateCommand(msg.params as Record<string, unknown>)
        .then((r) => sr(r))
        .catch((e: Error) => sr({ error: e.message }))
      return true
    },
    kaboom_execute_js: (msg, sr) => handleExecuteJs((msg.params as { script?: string; timeout_ms?: number }) || {}, sr),
    kaboom_execute_query: (msg, sr) => handleExecuteQuery((msg.params || {}) as Record<string, unknown>, sr),
    a11y_query: (msg, sr) => handleA11yQuery((msg.params || {}) as Record<string, unknown>, sr),
    dom_query: (msg, sr) => handleDomQuery((msg.params || {}) as Record<string, unknown>, sr),
    get_network_waterfall: (_msg, sr) => handleGetNetworkWaterfall(sr),
    link_health_query: (msg, sr) => handleLinkHealthQuery((msg.params ?? {}) as Record<string, unknown>, sr),
    computed_styles_query: (msg, sr) => handleComputedStylesQuery((msg.params ?? {}) as Record<string, unknown>, sr),
    form_discovery_query: (msg, sr) => handleFormDiscoveryQuery((msg.params ?? {}) as Record<string, unknown>, sr),
    form_state_query: (msg, sr) => handleFormStateQuery((msg.params ?? {}) as Record<string, unknown>, sr),
    data_table_query: (msg, sr) => handleDataTableQuery((msg.params ?? {}) as Record<string, unknown>, sr),
    kaboom_get_readable: (_msg, sr) => handleGetReadable(sr),
    kaboom_get_markdown: (_msg, sr) => handleGetMarkdown(sr),
    kaboom_page_summary: (_msg, sr) => handlePageSummary(sr)
  }

  chrome.runtime.onMessage.addListener(
    (
      message: SyncMsg,
      sender: chrome.runtime.MessageSender,
      sendResponse: (response?: unknown) => void
    ): boolean | undefined => {
      if (!isValidBackgroundSender(sender)) {
        console.warn(KABOOM_LOG_PREFIX, 'Rejected message from untrusted sender:', sender.id)
        return false
      }

      // Ping is special: sync handler that needs sendResponse
      if (message.type === 'kaboom_ping') return handlePing(sendResponse)

      // Try sync handlers first
      const syncHandler = syncHandlers[message.type] // nosemgrep: unsafe-dynamic-method
      if (syncHandler) {
        syncHandler(message)
        return false
      }

      // Handle toggle messages (no dispatch needed, always runs)
      handleToggleMessage(message)

      // Try delegated handlers
      const delegated = delegatedHandlers[message.type] // nosemgrep: unsafe-dynamic-method
      if (delegated) return delegated(message, sendResponse)

      return undefined
    }
  )
}

/**
 * Purpose: Installs Chrome runtime message listeners for recording start/stop, auto-stop from offscreen memory guard, and mic permission flow.
 * Docs: docs/features/feature/flow-recording/index.md
 */

// recording-listeners.ts — Chrome runtime message listeners for recording.
// Handles popup-initiated record start/stop, auto-stop from offscreen memory guard,
// mic permission grant flow, and file reveal requests.
// Deps are injected to avoid circular imports with recording.ts.

import { scaleTimeout } from '../lib/timeouts.js'
import { StorageKey } from '../lib/constants.js'
import { getLocal } from '../lib/storage-utils.js'
import type { OffscreenRecordingStoppedMessage } from '../types/runtime-messages.js'
import { errorMessage } from '../lib/error-utils.js'
import { trackUIFeature } from './ui-usage-tracker.js'
import { postDaemonJSON } from '../lib/daemon-http.js'
import { buildScreenRecordingSlug } from './recording-utils.js'
import type { ScreenRecordingHandlers } from './keyboard-shortcuts.js'
import { stopRecordingBadgeTimer } from './recording-badge.js'
import { KABOOM_RECORDING_LOG_PREFIX } from '../lib/brand.js'

const LOG = KABOOM_RECORDING_LOG_PREFIX

async function resolvePopupRecordingTargetTab(): Promise<chrome.tabs.Tab | undefined> {
  const trackedTabId = (await getLocal(StorageKey.TRACKED_TAB_ID)) as number | undefined
  if (trackedTabId) {
    try {
      return await chrome.tabs.get(trackedTabId)
    } catch (err) {
      console.warn(LOG, 'Tracked tab unavailable for popup recording start, falling back to active tab', {
        trackedTabId,
        error: errorMessage(err)
      })
    }
  }
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
  return tabs[0]
}

/** Dependencies injected by recording.ts to avoid circular imports. */
export interface RecordingListenerDeps extends Omit<ScreenRecordingHandlers, 'isRecording'> {
  isActive: () => boolean
  getTabId: () => number
  setInactive: () => void
  clearRecordingState: () => Promise<void>
  getServerUrl: () => string
}

/**
 * Install all chrome.runtime.onMessage listeners for recording.
 * Must be called once at module load time, guarded by chrome runtime availability.
 */
export function installRecordingListeners(deps: RecordingListenerDeps): void {
  /**
   * Listen for unsolicited messages from offscreen (auto-stop from memory guard or tab close).
   */
  chrome.runtime.onMessage.addListener(
    (message: OffscreenRecordingStoppedMessage, sender: chrome.runtime.MessageSender) => {
      // Only accept messages from the extension itself
      if (sender.id !== chrome.runtime.id) return
      if (message.target !== 'background' || message.type !== 'offscreen_recording_stopped') return
      // Only handle if we think we're still recording (auto-stop case)
      if (!deps.isActive()) return

      console.log(LOG, 'Auto-stop from offscreen (memory guard or tab close)', {
        status: message.status,
        name: message.name
      })
      stopRecordingBadgeTimer()
      deps.setInactive()
      deps.clearRecordingState().catch(() => {})
    }
  )

  /**
   * Handle popup-initiated screen_recording_start / screen_recording_stop messages.
   * These are direct chrome.runtime messages from the popup, not MCP pending queries.
   */
  chrome.runtime.onMessage.addListener(
    (
      message: { type?: string; audio?: string },
      sender: chrome.runtime.MessageSender,
      sendResponse: (response?: unknown) => void
    ) => {
      // Only accept messages from the extension itself (popup)
      if (sender.id !== chrome.runtime.id) return false
      if (message.type === 'screen_recording_start') {
        trackUIFeature('video')
        console.log(LOG, 'Popup screen_recording_start received', { audio: message.audio })
        resolvePopupRecordingTargetTab()
          .then((targetTab) => {
            const slug = buildScreenRecordingSlug(targetTab?.url)
            const audio = message.audio ?? ''
            console.log(LOG, 'Popup screen_recording_start \u2192 startRecording', {
              slug,
              audio,
              targetTabId: targetTab?.id,
              tabUrl: targetTab?.url?.substring(0, 60)
            })
            deps
              .startRecording(slug, 15, '', audio, true, targetTab?.id)
              .then((result) => {
                console.log(LOG, 'Popup screen_recording_start result:', result)
                sendResponse(result)
              })
              .catch((err) => {
                console.error(LOG, 'Popup screen_recording_start EXCEPTION:', err)
                sendResponse({ status: 'error' })
              })
          })
          .catch((err) => {
            // Without this, a tabs.query/get rejection would leave the popup hanging forever.
            console.error(LOG, 'Popup screen_recording_start target tab resolution FAILED:', err)
            sendResponse({ status: 'error' })
          })
        return true // async response
      }
      if (message.type === 'screen_recording_stop') {
        console.log(LOG, 'Popup screen_recording_stop received')
        deps
          .stopRecording()
          .then((result) => {
            console.log(LOG, 'Popup screen_recording_stop result:', result)
            sendResponse(result)
          })
          .catch((err) => {
            console.error(LOG, 'Popup screen_recording_stop EXCEPTION:', err)
            sendResponse({ status: 'error' })
          })
        return true // async response
      }
      return false
    }
  )

  /**
   * Handle mic_granted_close_tab from the mic-permission page.
   * Closes the permission tab, activates the original tab, and shows a guidance toast.
   */
  // #lizard forgives
  chrome.runtime.onMessage.addListener((message: { type?: string }, sender: chrome.runtime.MessageSender) => {
    // Only accept messages from the extension itself
    if (sender.id !== chrome.runtime.id) return false
    if (message.type !== 'mic_granted_close_tab') return false
    console.log(LOG, 'mic_granted_close_tab received from tab', sender.tab?.id)

    // Read the stored return tab before closing the permission tab
    void (async () => {
      const value = await getLocal(StorageKey.PENDING_MIC_RECORDING)
      const pending = value as { returnTabId?: number } | undefined
      const returnTabId = pending?.returnTabId
      console.log(
        LOG,
        'Pending mic recording intent:',
        pending,
        'returnTabId:',
        returnTabId
      )

      // Close the permission tab
      if (sender.tab?.id) {
        console.log(LOG, 'Closing permission tab', sender.tab.id)
        chrome.tabs.remove(sender.tab.id).catch(() => {})
      }

      // Activate the original tab and show guidance toast
      if (returnTabId) {
        console.log(LOG, 'Activating return tab', returnTabId)
        chrome.tabs
          .update(returnTabId, { active: true })
          .then(() => {
            console.log(LOG, 'Return tab activated, sending toast in 300ms')
            // Short delay to let the tab activation settle before sending message
            setTimeout(() => {
              console.log(LOG, 'Sending guidance toast to tab', returnTabId)
              chrome.tabs
                .sendMessage(returnTabId, {
                  type: 'kaboom_action_toast',
                  text: 'Mic permission granted',
                  detail: 'Open KaBOOM! and click Record',
                  state: 'success' as const,
                  duration_ms: scaleTimeout(8000)
                })
                .catch((err) => {
                  console.error(LOG, 'Toast send FAILED to tab', returnTabId, ':', errorMessage(err))
                })
            }, scaleTimeout(300))
          })
          .catch((err) => {
            console.error(LOG, 'Tab activation FAILED for tab', returnTabId, ':', errorMessage(err))
          })
      } else {
        console.warn(LOG, 'No returnTabId found — cannot activate tab or show toast')
      }
    })()

    return false
  })

  /**
   * Handle reveal_file — opens the file in the OS file manager via the Go server.
   */
  chrome.runtime.onMessage.addListener(
    (
      message: { type?: string; path?: string },
      sender: chrome.runtime.MessageSender,
      sendResponse: (response?: unknown) => void
    ) => {
      // Only accept messages from the extension itself
      if (sender.id !== chrome.runtime.id) return false
      if (message.type !== 'reveal_file' || !message.path) return false

      postDaemonJSON(`${deps.getServerUrl()}/recordings/reveal`, { path: message.path })
        .then((r) => {
          if (!r.ok) throw new Error(`HTTP ${r.status}`)
          return r.json()
        })
        .then((result) => sendResponse(result))
        .catch((err) => sendResponse({ error: errorMessage(err) }))

      return true // async response
    }
  )
}

/**
 * @fileoverview Draw Mode Background Handler
 * Routes draw_mode queries from the server to content script messages.
 * Handles DRAW_MODE_COMPLETED from content script to deliver results to server.
 * Manages keyboard shortcut toggle via chrome.commands.
 */
import * as index from './index.js'
import * as eventListeners from './event-listeners.js'

const { debugLog } = index

/**
 * Handle a draw_mode query from the Go server (via pending queries).
 * @param {Object} query - PendingQuery with type='draw_mode'
 * @param {number} tabId - Target tab ID
 * @param {Function} sendResult - Callback to send sync result
 * @param {Function} sendAsyncResult - Callback to send async result
 * @param {Object} syncClient - Sync client for queueing results
 */
export async function handleDrawModeQuery(query, tabId, sendResult, sendAsyncResult, syncClient) {
  let params
  try {
    params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
  } catch {
    params = {}
  }

  const action = params.action || 'start'

  if (action === 'start') {
    try {
      const sessionName = params.annot_session || ''
      const result = await chrome.tabs.sendMessage(tabId, {
        type: 'kaboom_draw_mode_start',
        started_by: 'llm',
        annot_session_name: sessionName,
        correlation_id: query.correlation_id || query.id || ''
      })
      sendResult(syncClient, query.id, {
        status: result?.status || 'active',
        message:
          'Draw mode activated. User can now draw annotations on the page. Results will be delivered when user finishes (presses ESC).',
        annotation_count: result?.annotation_count || 0
      })
    } catch (err) {
      sendResult(syncClient, query.id, {
        error: 'draw_mode_failed',
        message:
          err.message || 'Failed to activate draw mode. Ensure content script is loaded (try refreshing the page).'
      })
    }
    return
  }

  sendResult(syncClient, query.id, {
    error: 'unknown_draw_mode_action',
    message: `Unknown draw mode action: ${action}. Use 'start'.`
  })
}

/**
 * Handle KABOOM_CAPTURE_SCREENSHOT message from content script.
 * Captures visible tab while the draw mode overlay is still visible.
 * @param {Object} sender - Chrome message sender
 * @returns {Promise<{dataUrl: string}>}
 */
export async function handleCaptureScreenshot(sender) {
  const tabId = sender.tab?.id
  if (!tabId) return { dataUrl: '' }

  try {
    const tab = await chrome.tabs.get(tabId)
    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
      format: 'png'
    })
    return { dataUrl }
  } catch (err) {
    debugLog('error', 'Screenshot capture failed', { error: err.message })
    return { dataUrl: '' }
  }
}

/**
 * Handle DRAW_MODE_COMPLETED message from content script.
 * Uses the screenshot already captured (before overlay removal) and sends to server.
 * @param {Object} message - Message with annotations, elementDetails, and screenshot_data_url
 * @param {Object} sender - Chrome message sender
 * @param {Object} syncClient - Sync client for queueing results
 */
export async function handleDrawModeCompleted(message, sender, syncClient) {
  const tabId = sender.tab?.id
  if (!tabId) return

  const annotations = message.annotations || []
  const elementDetails = message.elementDetails || {}
  const pageUrl = message.page_url || ''
  const sessionName = message.annot_session_name || ''
  // Screenshot is now captured by content script before overlay removal
  const screenshotDataUrl = message.screenshot_data_url || ''

  try {
    // POST annotated data to server
    const serverUrl = index.serverUrl
    const body = {
      screenshot_data_url: screenshotDataUrl,
      annotations,
      element_details: elementDetails,
      page_url: pageUrl,
      tab_id: tabId,
      correlation_id: message.correlation_id || ''
    }
    if (sessionName) {
      body.annot_session_name = sessionName
    }
    const response = await fetch(`${serverUrl}/draw-mode/complete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })

    if (!response.ok) {
      const body = await response.text().catch(() => '')
      debugLog('error', 'Draw mode completion POST failed', { status: response.status, body })
    }
  } catch (err) {
    debugLog('error', 'Draw mode completion error', { error: err.message })
  }
}

/**
 * Install keyboard shortcut listener for draw mode toggle.
 */
export function installDrawModeCommandListener() {
  if (typeof chrome === 'undefined' || !chrome.commands) return

  chrome.commands.onCommand.addListener(async (command) => {
    if (command !== 'toggle_draw_mode') return

    try {
      // Get active tab
      const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
      const tab = tabs[0]
      if (!tab?.id) return

      // Check if draw mode is active by querying content script
      try {
        const result = await chrome.tabs.sendMessage(tab.id, {
          type: 'kaboom_get_annotations'
        })

        if (result?.draw_mode_active) {
          // Deactivate
          await chrome.tabs.sendMessage(tab.id, {
            type: 'kaboom_draw_mode_stop'
          })
        } else {
          // Activate
          await chrome.tabs.sendMessage(tab.id, {
            type: 'kaboom_draw_mode_start',
            started_by: 'user'
          })
        }
      } catch {
        // Content script not loaded — try activating anyway
        try {
          await chrome.tabs.sendMessage(tab.id, {
            type: 'kaboom_draw_mode_start',
            started_by: 'user'
          })
        } catch {
          // Can't reach content script — show toast so user knows why it failed
          debugLog('warn', 'Cannot reach content script for draw mode toggle')
          try {
            await chrome.tabs.sendMessage(tab.id, {
              type: 'kaboom_action_toast',
              text: 'Draw mode unavailable',
              detail: 'Refresh the page and try again',
              state: 'error',
              duration_ms: 3000
            })
          } catch {
            // Tab truly unreachable — nothing more we can do
          }
        }
      }
    } catch (err) {
      debugLog('error', 'Draw mode keyboard shortcut error', { error: err.message })
    }
  })
}

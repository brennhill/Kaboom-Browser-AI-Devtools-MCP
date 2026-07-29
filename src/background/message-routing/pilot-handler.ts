/**
 * Purpose: Own AI Pilot and tracked-tab runtime messages.
 */
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import { StorageKey } from '../../lib/constants.js'
import { getLocal, getLocals } from '../../lib/storage/local.js'
import type { MessageHandlerOwner } from './types.js'

export interface PilotHandlerDependencies {
  isEnabled: () => boolean
  setEnabled: (enabled: boolean, callback?: () => void) => void
}

export async function broadcastTrackingState(untrackedTabId?: number | null): Promise<void> {
  try {
    const result = await getLocals([StorageKey.TRACKED_TAB_ID, StorageKey.AI_WEB_PILOT_ENABLED])
    const trackedTabId = result[StorageKey.TRACKED_TAB_ID] as number | undefined
    const aiPilotEnabled = result[StorageKey.AI_WEB_PILOT_ENABLED] === true
    if (trackedTabId) {
      chrome.tabs
        .sendMessage(trackedTabId, {
          type: 'tracking_state_changed',
          state: { isTracked: true, aiPilotEnabled }
        })
        .catch(() => {})
    }
    if (untrackedTabId && untrackedTabId !== trackedTabId) {
      chrome.tabs
        .sendMessage(untrackedTabId, {
          type: 'tracking_state_changed',
          state: { isTracked: false, aiPilotEnabled: false }
        })
        .catch(() => {})
    }
  } catch (error) {
    console.error(`${KABOOM_LOG_PREFIX} Failed to broadcast tracking state:`, error)
  }
}

export function createPilotMessageHandler(deps: PilotHandlerDependencies): MessageHandlerOwner {
  return {
    feature: 'pilot',
    handle(message, sender, sendResponse) {
      switch (message.type) {
        case 'set_ai_web_pilot_enabled': {
          const enabled = message.enabled === true
          deps.setEnabled(enabled, () => {
            void broadcastTrackingState()
          })
          sendResponse({ success: true })
          return false
        }
        case 'get_ai_web_pilot_enabled':
          sendResponse({ enabled: deps.isEnabled() })
          return false
        case 'get_tracking_state':
          getLocal(StorageKey.TRACKED_TAB_ID)
            .then((tracked) => {
              sendResponse({
                state: {
                  isTracked: sender.tab?.id !== undefined && sender.tab.id === tracked,
                  aiPilotEnabled: deps.isEnabled()
                }
              })
            })
            .catch(() => sendResponse({ state: { isTracked: false, aiPilotEnabled: false } }))
          return true
        case 'get_diagnostic_state':
          getLocal(StorageKey.AI_WEB_PILOT_ENABLED).then((storage) => {
            sendResponse({ cache: deps.isEnabled(), storage, timestamp: new Date().toISOString() })
          })
          return true
        default:
          return undefined
      }
    }
  }
}

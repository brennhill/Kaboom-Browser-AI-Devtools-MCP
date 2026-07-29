/**
 * Purpose: Own background settings mutations and content-script propagation.
 */
import { DEFAULT_SERVER_URL, SettingName, StorageKey } from '../../lib/constants.js'
import type { MessageHandlerOwner } from './types.js'

export interface SettingsHandlerDependencies {
  getServerUrl: () => string
  setServerUrl: (url: string) => void
  setLogLevel: (level: string) => void
  setScreenshotOnError: (enabled: boolean) => void
  setSourceMapEnabled: (enabled: boolean) => void
  setDebugMode: (enabled: boolean) => void
  clearSourceMapCache: () => void
  saveSetting: (key: string, value: unknown) => void
  forwardToContentScripts: (message: { type: string; [key: string]: unknown }) => void
  checkConnection: () => Promise<void>
  debugLog: (category: string, message: string, data?: unknown) => void
}

const forwardedSettings = new Set([
  'set_network_waterfall_enabled',
  'set_performance_marks_enabled',
  'set_action_replay_enabled',
  'set_web_socket_capture_enabled',
  'set_web_socket_capture_mode',
  'set_performance_snapshot_enabled',
  'set_deferral_enabled',
  'set_network_body_capture_enabled',
  'set_action_toasts_enabled',
  'set_subtitles_enabled'
])

export function createSettingsMessageHandler(deps: SettingsHandlerDependencies): MessageHandlerOwner {
  return {
    feature: 'settings',
    handle(message, _sender, sendResponse) {
      if (forwardedSettings.has(message.type)) {
        const forwarded = message as { type: string; enabled?: boolean; mode?: string }
        deps.debugLog('settings', `Setting ${forwarded.type}: ${forwarded.enabled ?? forwarded.mode}`)
        deps.forwardToContentScripts(forwarded)
        sendResponse({ success: true })
        return false
      }
      switch (message.type) {
        case 'set_log_level':
          deps.setLogLevel(message.level)
          deps.saveSetting(StorageKey.LOG_LEVEL, message.level)
          return false
        case 'set_screenshot_on_error':
          deps.setScreenshotOnError(message.enabled)
          deps.saveSetting(StorageKey.SCREENSHOT_ON_ERROR, message.enabled)
          sendResponse({ success: true })
          return false
        case 'set_source_map_enabled':
          deps.setSourceMapEnabled(message.enabled)
          deps.saveSetting(StorageKey.SOURCE_MAP_ENABLED, message.enabled)
          if (!message.enabled) deps.clearSourceMapCache()
          sendResponse({ success: true })
          return false
        case 'set_debug_mode':
          deps.setDebugMode(message.enabled)
          deps.saveSetting(StorageKey.DEBUG_MODE, message.enabled)
          sendResponse({ success: true })
          return false
        case 'set_server_url':
          deps.setServerUrl(message.url || DEFAULT_SERVER_URL)
          deps.saveSetting(StorageKey.SERVER_URL, deps.getServerUrl())
          deps.debugLog('settings', `Server URL changed to: ${deps.getServerUrl()}`)
          deps.forwardToContentScripts({ type: SettingName.SERVER_URL, url: deps.getServerUrl() })
          void deps.checkConnection()
          sendResponse({ success: true })
          return false
        default:
          return undefined
      }
    }
  }
}

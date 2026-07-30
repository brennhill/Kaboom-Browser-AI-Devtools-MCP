/**
 * Purpose: Owns persisted extension setting reads and writes used during background startup.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import { StorageKey } from '../../lib/constants.js'
import { persist } from '../../lib/storage/io.js'
import { getLocal, getLocals, setLocal } from '../../lib/storage/local.js'

export interface SavedSettings {
  serverUrl?: string
  logLevel?: string
  screenshotOnError?: boolean
  sourceMapEnabled?: boolean
  debugMode?: boolean
}

export async function loadSavedSettings(): Promise<SavedSettings> {
  try {
    return (await getLocals([
      StorageKey.SERVER_URL,
      StorageKey.LOG_LEVEL,
      StorageKey.SCREENSHOT_ON_ERROR,
      StorageKey.SOURCE_MAP_ENABLED,
      StorageKey.DEBUG_MODE
    ])) as SavedSettings
  } catch {
    console.warn(`${KABOOM_LOG_PREFIX} Could not load saved settings - using defaults`)
    return {}
  }
}

export async function loadAiWebPilotState(logFn?: (message: string) => void): Promise<boolean> {
  const startTime = performance.now()
  if (typeof chrome === 'undefined' || !chrome.storage) return false
  const aiEnabled = await getLocal(StorageKey.AI_WEB_PILOT_ENABLED)
  const wasLoaded = aiEnabled !== false
  const loadTime = performance.now() - startTime
  logFn?.(`${KABOOM_LOG_PREFIX} AI Web Pilot loaded on startup: ${wasLoaded} (took ${loadTime.toFixed(1)}ms)`)
  return wasLoaded
}

export async function loadDebugModeState(): Promise<boolean> {
  return (await getLocal(StorageKey.DEBUG_MODE)) === true
}

export function saveSetting(key: string, value: unknown): void {
  persist(setLocal(key, value), `setting:${key}`)
}

export async function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>> {
  return (await getLocals([
    StorageKey.AI_WEB_PILOT_ENABLED,
    StorageKey.WEBSOCKET_CAPTURE_ENABLED,
    StorageKey.NETWORK_WATERFALL_ENABLED,
    StorageKey.PERFORMANCE_MARKS_ENABLED,
    StorageKey.ACTION_REPLAY_ENABLED,
    StorageKey.SCREENSHOT_ON_ERROR,
    StorageKey.SOURCE_MAP_ENABLED,
    StorageKey.NETWORK_BODY_CAPTURE_ENABLED
  ])) as Record<string, boolean | string | undefined>
}

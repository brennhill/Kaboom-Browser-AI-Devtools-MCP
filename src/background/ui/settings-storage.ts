/**
 * Purpose: Owns persisted extension setting reads and writes used during background startup.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import { StorageKey } from '../../lib/constants.js'
import { persist } from '../../lib/storage/io.js'
import { classifyStorageFailure, storageFaultDetail, type StorageFaultKind } from '../../lib/storage/fault.js'
import { getLocals, setLocal } from '../../lib/storage/local.js'
import { readLocalState } from '../../lib/storage/validated.js'
import { reportStateRecovery, resolveStateRecovery } from '../runtime-state/state-recovery.js'

export interface SavedSettings {
  serverUrl?: string
  logLevel?: string
  screenshotOnError?: boolean
  sourceMapEnabled?: boolean
  debugMode?: boolean
}

export async function loadSavedSettings(): Promise<SavedSettings> {
  try {
    const stored = await getLocals([
      StorageKey.SERVER_URL,
      StorageKey.LOG_LEVEL,
      StorageKey.SCREENSHOT_ON_ERROR,
      StorageKey.SOURCE_MAP_ENABLED,
      StorageKey.DEBUG_MODE
    ])
    const valid =
      typeof stored === 'object' &&
      stored !== null &&
      !Array.isArray(stored) &&
      (stored[StorageKey.SERVER_URL] === undefined || typeof stored[StorageKey.SERVER_URL] === 'string') &&
      (stored[StorageKey.LOG_LEVEL] === undefined || typeof stored[StorageKey.LOG_LEVEL] === 'string') &&
      [StorageKey.SCREENSHOT_ON_ERROR, StorageKey.SOURCE_MAP_ENABLED, StorageKey.DEBUG_MODE].every(
        (key) => stored[key] === undefined || typeof stored[key] === 'boolean'
      )
    if (valid) {
      resolveStateRecovery('extension_settings_state')
      return stored as SavedSettings
    }
    reportSettingsRecovery('corruption', 'Saved extension settings were malformed; defaults are active.')
    return {}
  } catch (error) {
    reportSettingsRecovery(
      classifyStorageFailure(error, 'read'),
      'Saved extension settings could not be read; defaults are active.'
    )
    return {}
  }
}

export async function loadAiWebPilotState(logFn?: (message: string) => void): Promise<boolean> {
  const startTime = performance.now()
  if (typeof chrome === 'undefined' || !chrome.storage) return false
  const aiEnabled = await readLocalState<boolean>({
    key: StorageKey.AI_WEB_PILOT_ENABLED,
    fallback: true,
    validate: (value): value is boolean => typeof value === 'boolean',
    diagnostic: settingsDiagnostic('Saved AI Web Pilot preference was invalid or unreadable; enabled is active.'),
    report: reportStateRecovery,
    resolve: resolveStateRecovery
  })
  const wasLoaded = aiEnabled !== false
  const loadTime = performance.now() - startTime
  logFn?.(`${KABOOM_LOG_PREFIX} AI Web Pilot loaded on startup: ${wasLoaded} (took ${loadTime.toFixed(1)}ms)`)
  return wasLoaded
}

export async function loadDebugModeState(): Promise<boolean> {
  return readLocalState<boolean>({
    key: StorageKey.DEBUG_MODE,
    fallback: false,
    validate: (value): value is boolean => typeof value === 'boolean',
    diagnostic: settingsDiagnostic('Saved debug-mode preference was invalid or unreadable; disabled is active.'),
    report: reportStateRecovery,
    resolve: resolveStateRecovery
  })
}

export function saveSetting(key: string, value: unknown): void {
  persist(setLocal(key, value), `setting:${key}`)
}

function settingsDiagnostic(detail: string) {
  return {
    name: 'extension_settings_state',
    detail,
    fix: 'Open extension settings and save your preferences again.'
  } as const
}

function reportSettingsRecovery(kind: StorageFaultKind, detail: string): void {
  const classifiedDetail = storageFaultDetail(kind, detail)
  reportStateRecovery(settingsDiagnostic(classifiedDetail))
  console.warn(`${KABOOM_LOG_PREFIX} ${classifiedDetail}`)
}

export async function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>> {
  try {
    const stored = await getLocals([
      StorageKey.AI_WEB_PILOT_ENABLED,
      StorageKey.WEBSOCKET_CAPTURE_ENABLED,
      StorageKey.NETWORK_WATERFALL_ENABLED,
      StorageKey.PERFORMANCE_MARKS_ENABLED,
      StorageKey.ACTION_REPLAY_ENABLED,
      StorageKey.SCREENSHOT_ON_ERROR,
      StorageKey.SOURCE_MAP_ENABLED,
      StorageKey.NETWORK_BODY_CAPTURE_ENABLED
    ])
    if (
      typeof stored === 'object' &&
      stored !== null &&
      !Array.isArray(stored) &&
      Object.values(stored).every(
        (value) => value === undefined || typeof value === 'boolean' || typeof value === 'string'
      )
    ) {
      resolveStateRecovery('extension_settings_state')
      return stored as Record<string, boolean | string | undefined>
    }
    reportSettingsRecovery('corruption', 'Saved capture settings were malformed; defaults are active.')
  } catch (error) {
    reportSettingsRecovery(
      classifyStorageFailure(error, 'read'),
      'Saved capture settings could not be read; defaults are active.'
    )
  }
  return {}
}

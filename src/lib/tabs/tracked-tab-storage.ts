/**
 * Purpose: Shared storage helpers for tracked-tab state (TRACKED_TAB_ID/URL/TITLE).
 * Why: Tracked-tab storage keys must be accessed through one helper module (CLAUDE.md rule 18)
 *      so background and popup never drift (e.g., leaving a stale TRACKED_TAB_TITLE behind).
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import { StorageKey } from '../constants.js'
import { getLocals, setLocals, removeLocals } from '../storage/local.js'
import { reportStateRecovery } from '../storage/recovery.js'

export interface TrackedTabState {
  id?: number
  url?: string
  title?: string
}

/** All storage keys that make up tracked-tab state. Always read/cleared together. */
export const TRACKED_TAB_STORAGE_KEYS = [
  StorageKey.TRACKED_TAB_ID,
  StorageKey.TRACKED_TAB_URL,
  StorageKey.TRACKED_TAB_TITLE
]

/** Read the complete tracked-tab identity as one consistent snapshot. */
export async function readTrackedTab(): Promise<TrackedTabState> {
  let stored: Record<string, unknown>
  try {
    stored = await getLocals(TRACKED_TAB_STORAGE_KEYS)
  } catch {
    reportTrackedTabRecovery('Saved tracked-tab state could not be read; automatic tab selection is active.')
    return {}
  }
  const id = stored[StorageKey.TRACKED_TAB_ID]
  const url = stored[StorageKey.TRACKED_TAB_URL]
  const title = stored[StorageKey.TRACKED_TAB_TITLE]
  const valid =
    (id === undefined || (typeof id === 'number' && Number.isInteger(id) && id > 0)) &&
    (url === undefined || typeof url === 'string') &&
    (title === undefined || typeof title === 'string')
  if (!valid) {
    reportTrackedTabRecovery('Saved tracked-tab state was malformed; automatic tab selection is active.')
    return {}
  }
  return {
    id,
    url,
    title
  }
}

function reportTrackedTabRecovery(detail: string): void {
  reportStateRecovery({
    name: 'tracked_tab_state',
    detail,
    fix: 'Choose a tab from the extension popup to save a fresh tracking state.'
  })
}

/**
 * Persist tracked tab state.
 */
export async function setTrackedTab(tab: Pick<chrome.tabs.Tab, 'id' | 'url' | 'title'>): Promise<void> {
  if (!tab.id) return
  await setLocals({
    [StorageKey.TRACKED_TAB_ID]: tab.id,
    [StorageKey.TRACKED_TAB_URL]: tab.url ?? '',
    [StorageKey.TRACKED_TAB_TITLE]: tab.title ?? ''
  })
}

/**
 * Clear tracked tab state (all keys, including title).
 */
export async function clearTrackedTab(): Promise<void> {
  await removeLocals(TRACKED_TAB_STORAGE_KEYS)
}

/**
 * Purpose: Shared storage helpers for tracked-tab state (TRACKED_TAB_ID/URL/TITLE).
 * Why: Tracked-tab storage keys must be accessed through one helper module (CLAUDE.md rule 18)
 *      so background and popup never drift (e.g., leaving a stale TRACKED_TAB_TITLE behind).
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import { StorageKey } from './constants.js'
import { setLocals, removeLocals } from './storage-utils.js'

/** All storage keys that make up tracked-tab state. Always read/cleared together. */
export const TRACKED_TAB_STORAGE_KEYS = [
  StorageKey.TRACKED_TAB_ID,
  StorageKey.TRACKED_TAB_URL,
  StorageKey.TRACKED_TAB_TITLE
]

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

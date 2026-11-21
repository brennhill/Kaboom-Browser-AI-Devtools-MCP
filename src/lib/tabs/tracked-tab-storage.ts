/**
 * Purpose: Shared storage helpers for tracked-tab state (TRACKED_TAB_ID/URL/TITLE).
 * Why: Tracked-tab storage keys must be accessed through one helper module (CLAUDE.md rule 18)
 *      so background and popup never drift (e.g., leaving a stale TRACKED_TAB_TITLE behind).
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import { StorageKey } from '../constants.js'
import { classifyStorageFailure, storageFaultDetail, type StorageFaultKind } from '../storage/fault.js'
import { getLocals, setLocals, removeLocals } from '../storage/local.js'
import { reportStateRecovery, resolveStateRecovery } from '../storage/recovery.js'

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
  } catch (error) {
    reportTrackedTabRecovery(
      classifyStorageFailure(error, 'read'),
      'Saved tracked-tab state could not be read; automatic tab selection is active.'
    )
    return {}
  }
  const id = stored[StorageKey.TRACKED_TAB_ID]
  const url = stored[StorageKey.TRACKED_TAB_URL]
  const title = stored[StorageKey.TRACKED_TAB_TITLE]
  const valid =
    typeof stored === 'object' &&
    stored !== null &&
    (id === undefined || (typeof id === 'number' && Number.isInteger(id) && id > 0)) &&
    (url === undefined || typeof url === 'string') &&
    (title === undefined || typeof title === 'string')
  if (!valid) {
    reportTrackedTabRecovery('corruption', 'Saved tracked-tab state was malformed; automatic tab selection is active.')
    return {}
  }
  resolveStateRecovery('tracked_tab_state')
  return {
    id,
    url,
    title
  }
}

function reportTrackedTabRecovery(kind: StorageFaultKind, detail: string): void {
  reportStateRecovery({
    name: 'tracked_tab_state',
    detail: storageFaultDetail(kind, detail),
    fix: 'Choose a tab from the extension popup to save a fresh tracking state.'
  })
}

// Ordering rule (rule 18, single writer): every tracked-tab write in this context runs
// through one serialized queue, and writes scoped to a tab (navigation details, close)
// re-read the tracked id inside that queue. A late update for tab A therefore can never
// land on tab B after a switch: either it runs before the switch (and is overwritten by
// it) or after (and is dropped because the tracked id no longer matches).
let writeQueue: Promise<unknown> = Promise.resolve()

function serialized<T>(write: () => Promise<T>): Promise<T> {
  const next = writeQueue.then(write, write)
  writeQueue = next.catch(() => {
    // EXPECTED_ABSENCE: the failure is surfaced to this write's own caller via `next`, so
    // swallowing it here is expected; logging it again would double-report one failure.
    // The queue only needs to keep running for later writes.
  })
  return next
}

/**
 * Persist tracked tab state (selection: popup, context menu, switch_tab, auto-track).
 */
export function setTrackedTab(tab: Pick<chrome.tabs.Tab, 'id' | 'url' | 'title'>): Promise<void> {
  const id = tab.id
  if (!id) return Promise.resolve()
  return serialized(() =>
    setLocals({
      [StorageKey.TRACKED_TAB_ID]: id,
      [StorageKey.TRACKED_TAB_URL]: tab.url ?? '',
      [StorageKey.TRACKED_TAB_TITLE]: tab.title ?? ''
    })
  )
}

/**
 * Record navigation details for `tabId` only if it is still the tracked tab when the
 * write runs. Returns whether the details were written.
 */
export function updateTrackedTabDetails(tabId: number, details: { url: string; title?: string }): Promise<boolean> {
  return serialized(async () => {
    if ((await readTrackedTab()).id !== tabId) return false
    const updates: Record<string, unknown> = { [StorageKey.TRACKED_TAB_URL]: details.url }
    if (details.title) updates[StorageKey.TRACKED_TAB_TITLE] = details.title
    await setLocals(updates)
    return true
  })
}

/**
 * Clear tracked tab state only if `tabId` is still the tracked tab (Chrome close
 * lifecycle). Returns whether tracking was cleared.
 */
export function clearTrackedTabIfCurrent(tabId: number): Promise<boolean> {
  return serialized(async () => {
    if ((await readTrackedTab()).id !== tabId) return false
    await removeLocals(TRACKED_TAB_STORAGE_KEYS)
    return true
  })
}

/**
 * Clear tracked tab state (all keys, including title).
 */
export function clearTrackedTab(): Promise<void> {
  return serialized(() => removeLocals(TRACKED_TAB_STORAGE_KEYS))
}

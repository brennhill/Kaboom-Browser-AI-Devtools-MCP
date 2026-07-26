/**
 * Purpose: Chrome context menu installation and click handlers for Kaboom actions.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */

import { StorageKey } from '../../lib/constants.js'
import { getLocal } from '../../lib/storage-utils.js'
import type { ScreenRecordingHandlers, RecordingShortcutHandlers } from './keyboard-shortcuts.js'
import { toggleScreenRecording, toggleActionSequenceRecording } from './keyboard-shortcuts.js'
import { errorMessage } from '../../lib/error-utils.js'
import { toggleDrawModeForTab } from './draw-mode-toggle.js'
import { trackTab, untrackTab } from '../../lib/tabs/tab-tracking-core.js'
import { trackUIFeature } from './ui-usage-tracker.js'
import { toggleTerminalSidePanel, isTerminalPanelOpenSync } from './terminal-panel.js'

// =============================================================================
// CONTEXT MENU IDS
// =============================================================================

const MENU_ID_CONTROL = 'kaboom-control-page'
const MENU_ID_TERMINAL = 'kaboom-open-terminal'
const TERMINAL_OPEN_TITLE = 'Open Kaboom Terminal'
const TERMINAL_CLOSE_TITLE = 'Close Kaboom Terminal'
const MENU_ID_SCREENSHOT = 'kaboom-screenshot'
const MENU_ID_ANNOTATE = 'kaboom-annotate-page'
const MENU_ID_RECORD = 'kaboom-record-screen'
const MENU_ID_ACTION_RECORD = 'kaboom-action-record'

const CONTROL_TAB_TITLE = 'Control Tab'
const RELEASE_CONTROL_TITLE = 'Release Control'
const ANNOTATE_START_TITLE = 'Annotate Page'
const ANNOTATE_STOP_TITLE = 'Stop Annotation'
const RECORD_START_TITLE = 'Record Screen'
const RECORD_STOP_TITLE = 'Stop Screen Recording'
const ACTION_RECORD_START_TITLE = 'Record User Actions'
const ACTION_RECORD_STOP_TITLE = 'Stop User Action Recording'

function updateContextMenuTitle(menuId: string, title: string): Promise<void> {
  return new Promise((resolve) => {
    chrome.contextMenus.update(menuId, { title }, () => resolve())
  })
}

async function isDrawModeActive(tabId: number | undefined): Promise<boolean> {
  if (!tabId) return false
  try {
    const result = (await chrome.tabs.sendMessage(tabId, {
      type: 'kaboom_get_annotations'
    })) as { draw_mode_active?: boolean }
    return result?.draw_mode_active === true
  } catch {
    return false
  }
}

async function refreshDynamicContextMenuTitles(
  tabId: number | undefined,
  recordingHandlers: ScreenRecordingHandlers,
  actionRecordingHandlers: RecordingShortcutHandlers
): Promise<void> {
  const trackedTabId = (await getLocal(StorageKey.TRACKED_TAB_ID)) as number | undefined
  const drawModeActive = await isDrawModeActive(tabId)

  await Promise.all([
    updateContextMenuTitle(MENU_ID_CONTROL, trackedTabId && tabId === trackedTabId ? RELEASE_CONTROL_TITLE : CONTROL_TAB_TITLE),
    updateContextMenuTitle(MENU_ID_ANNOTATE, drawModeActive ? ANNOTATE_STOP_TITLE : ANNOTATE_START_TITLE),
    updateContextMenuTitle(MENU_ID_RECORD, recordingHandlers.isRecording() ? RECORD_STOP_TITLE : RECORD_START_TITLE),
    updateContextMenuTitle(
      MENU_ID_ACTION_RECORD,
      actionRecordingHandlers.isRecording() ? ACTION_RECORD_STOP_TITLE : ACTION_RECORD_START_TITLE
    ),
    // Reads "Close" while the panel is up, so the menu is a toggle rather than
    // an Open that does nothing when it is already open.
    updateContextMenuTitle(MENU_ID_TERMINAL, isTerminalPanelOpenSync() ? TERMINAL_CLOSE_TITLE : TERMINAL_OPEN_TITLE)
  ])
  const contextMenusWithRefresh = chrome.contextMenus as unknown as { refresh?: () => void }
  contextMenusWithRefresh.refresh?.()
}

// =============================================================================
// CONTEXT MENU INSTALLATION
// =============================================================================

/**
 * Create context menu items for Kaboom actions.
 * Chrome auto-groups multiple items under a parent with the extension icon.
 */
export function installContextMenus(
  recordingHandlers: ScreenRecordingHandlers,
  actionRecordingHandlers: RecordingShortcutHandlers,
  logFn?: (message: string) => void
): void {
  if (typeof chrome === 'undefined' || !chrome.contextMenus) return

  chrome.contextMenus.removeAll(() => {
    const ctx: ['page'] = ['page']
    chrome.contextMenus.create({ id: MENU_ID_CONTROL, title: CONTROL_TAB_TITLE, contexts: ctx })
    chrome.contextMenus.create({ id: MENU_ID_SCREENSHOT, title: 'Take Screenshot', contexts: ctx })
    chrome.contextMenus.create({ id: MENU_ID_ANNOTATE, title: ANNOTATE_START_TITLE, contexts: ctx })
    chrome.contextMenus.create({ id: MENU_ID_RECORD, title: RECORD_START_TITLE, contexts: ctx })
    chrome.contextMenus.create({ id: MENU_ID_ACTION_RECORD, title: ACTION_RECORD_START_TITLE, contexts: ctx })
    chrome.contextMenus.create({ id: MENU_ID_TERMINAL, title: TERMINAL_OPEN_TITLE, contexts: ctx })
  })

  const contextMenusWithShown = chrome.contextMenus as unknown as {
    onShown?: {
      addListener: (listener: (info: chrome.contextMenus.OnClickData, tab?: chrome.tabs.Tab) => void) => void
    }
  }
  contextMenusWithShown.onShown?.addListener((_info: chrome.contextMenus.OnClickData, tab?: chrome.tabs.Tab) => {
    refreshDynamicContextMenuTitles(tab?.id, recordingHandlers, actionRecordingHandlers).catch((err) => {
      if (logFn) logFn(`Context menu title refresh error: ${errorMessage(err)}`)
    })
  })

  chrome.contextMenus.onClicked.addListener(async (info, tab) => {
    if (!tab?.id) return

    // FIRST, before any await: chrome.sidePanel.open() needs a live user gesture,
    // and contextMenus.onClicked is one of the few entry points Chrome grants a
    // full (unrestricted) gesture. Awaiting anything first would expire it.
    if (info.menuItemId === MENU_ID_TERMINAL) {
      // Call synchronously — awaiting ANYTHING first (even a storage read) burns
      // the user gesture and Chrome then refuses sidePanel.open(). This is what
      // made "Open Kaboom Terminal" do nothing.
      toggleTerminalSidePanel(tab.id)
        .then((result) => {
          if (!result.success && logFn) {
            logFn(`Toggle terminal via context menu failed: ${result.error ?? 'unknown error'}`)
          }
        })
        .catch((err) => {
          if (logFn) logFn(`Toggle terminal via context menu error: ${errorMessage(err)}`)
        })
      return
    }

    if (info.menuItemId === MENU_ID_CONTROL) {
      try {
        const trackedTabId = (await getLocal(StorageKey.TRACKED_TAB_ID)) as number | undefined
        if (trackedTabId === tab.id) {
          // Release: shared core clears state, stops recording, and notifies the
          // content script. The menu can reach the recording handler directly.
          await untrackTab(tab.id, async () => {
            if (recordingHandlers.isRecording()) await recordingHandlers.stopRecording()
          })
          if (logFn) logFn(`Released control for tab ${tab.id}`)
        } else {
          // Track through the shared core so the same internal-page and
          // cloaked-domain guards the popup enforces apply here too — controlling
          // a cloaked tab from the menu was a privacy leak (F2, rule 7).
          const outcome = await trackTab(tab)
          if (outcome === 'tracked') {
            if (logFn) logFn(`Now controlling tab ${tab.id}: ${tab.url}`)
          } else if (logFn) {
            logFn(`Not controlling tab ${tab.id}: ${outcome === 'cloaked' ? 'cloaked domain' : 'internal page'}`)
          }
        }
      } catch (err) {
        if (logFn) logFn(`Control page error: ${errorMessage(err)}`)
      }
    } else if (info.menuItemId === MENU_ID_SCREENSHOT) {
      try {
        trackUIFeature('screenshot')
        chrome.tabs.sendMessage(tab.id, { type: 'capture_screenshot' })
      } catch {
        if (logFn) logFn('Cannot reach content script for screenshot via context menu')
      }
    } else if (info.menuItemId === MENU_ID_RECORD) {
      try {
        trackUIFeature('video')
        await toggleScreenRecording(recordingHandlers, tab, logFn)
      } catch (err) {
        if (logFn) logFn(`Context menu recording error: ${errorMessage(err)}`)
      }
    } else if (info.menuItemId === MENU_ID_ACTION_RECORD) {
      try {
        await toggleActionSequenceRecording(actionRecordingHandlers, tab, logFn)
      } catch (err) {
        if (logFn) logFn(`Context menu action recording error: ${errorMessage(err)}`)
      }
    } else if (info.menuItemId === MENU_ID_ANNOTATE) {
      try {
        trackUIFeature('annotations')
        await toggleDrawModeForTab(tab.id)
      } catch {
        if (logFn) logFn('Cannot reach content script for annotation via context menu')
      }
    }

    refreshDynamicContextMenuTitles(tab.id, recordingHandlers, actionRecordingHandlers).catch(() => {})
  })
}

/**
 * Purpose: Manages recording lifecycle (start/stop) and recording state, delegating capture plumbing and listener registration to sub-modules.
 * Docs: docs/features/feature/flow-recording/index.md
 */

// recording.ts — Recording lifecycle management (start/stop) and state.
// Delegates tab capture / offscreen plumbing to recording-capture.ts and
// chrome runtime listener registration to recording-listeners.ts.

import { getServerUrl } from '../runtime-state/settings-state.js'
import { pingContentScript, sendTabToast } from '../ui/content-script-bridge.js'
import { waitForTabLoad, getActiveTab } from '../ui/tracked-tab-state.js'
import { scaleTimeout } from '../../lib/timeouts.js'
import { StorageKey } from '../../lib/constants.js'
import type {
  OffscreenRecordingStartedMessage,
  OffscreenRecordingStoppedMessage,
  OffscreenRecordingStateResponse
} from '../../types/runtime-messages.js'
import { ensureOffscreenDocument, getStreamIdWithRecovery, requestRecordingGesture } from './capture.js'
import { installRecordingListeners } from './listeners.js'
import { isPersistedRecordingState, resolveRecordingRehydration, type PersistedRecordingState } from './rehydration.js'
import { errorMessage } from '../../lib/error-utils.js'
import { persist } from '../../lib/storage/io.js'
import { getLocal, removeLocal, setLocals } from '../../lib/storage/local.js'
import { delay } from '../../lib/timeout-utils.js'
import { buildRecordingToastLabel } from './utils.js'
import { startRecordingBadgeTimer, stopRecordingBadgeTimer } from './badge.js'
import { readTrackedTab, setTrackedTab } from '../../lib/tabs/tracked-tab-storage.js'
import { KABOOM_RECORDING_LOG_PREFIX } from '../../lib/brand.js'
import { readLocalState } from '../../lib/storage/validated.js'
import { reportStateRecovery } from '../runtime-state/state-recovery.js'

// =============================================================================
// STATE
// =============================================================================

interface RecordingState {
  active: boolean
  name: string
  startTime: number
  fps: number
  audioMode: string
  tabId: number
  url: string
  queryId: string
}

const defaultState: RecordingState = {
  active: false,
  name: '',
  startTime: 0,
  fps: 15,
  audioMode: '',
  tabId: 0,
  url: '',
  queryId: ''
}

let recordingState: RecordingState = { ...defaultState }

const LOG = KABOOM_RECORDING_LOG_PREFIX

/** Listener to re-send watermark when recording tab navigates or content script re-injects. */
let tabUpdateListener: ((tabId: number, changeInfo: { status?: string }) => void) | null = null

/**
 * Kick off recording rehydration. Call once from background init.
 *
 * MV3 service workers restart routinely while the offscreen MediaRecorder keeps
 * recording, so on startup we ask the offscreen document whether a recording is
 * still active: rehydrate state + badge timer if so, otherwise clear stale
 * storage (e.g. a browser crash during a previous recording).
 *
 * This is an explicit call, NOT a module-load side effect: importing this module
 * merely to reach isRecording()/startRecording() must not fire chrome messaging
 * and a storage read (which forced every test to stub chrome before import, and
 * re-ran rehydration for every importer). Only initializeExtension() calls it.
 */
export function initRecording(): Promise<void> {
  console.log(LOG, 'Checking for surviving offscreen recording')
  return rehydrateRecordingStateOnLoad()
}

/** Ask the offscreen document for its live recording state. Null when unreachable/inactive context. */
async function queryOffscreenRecordingState(): Promise<OffscreenRecordingStateResponse | null> {
  if (typeof chrome === 'undefined' || !chrome.runtime?.sendMessage) return null
  try {
    const response = (await chrome.runtime.sendMessage({
      target: 'offscreen',
      type: 'offscreen_get_recording_state'
    })) as OffscreenRecordingStateResponse | undefined
    if (response && typeof response.active === 'boolean') return response
    return null
  } catch {
    // EXPECTED_ABSENCE: no offscreen listener after restart is normal; logging would mislabel an inactive recording as failure.
    return null
  }
}

/** Rehydrate recording state after a SW restart, or clear stale persisted state. */
async function rehydrateRecordingStateOnLoad(): Promise<void> {
  try {
    const restored = await resolveRecordingRehydration({
      queryOffscreenRecordingState,
      getPersistedRecording: async () =>
        await readLocalState<PersistedRecordingState | null>({
          key: StorageKey.RECORDING,
          fallback: null,
          validate: isPersistedRecordingState,
          diagnostic: {
            name: 'screen_recording_state',
            detail: 'Saved screen-recording state was invalid or unreadable; idle state is active.',
            fix: 'Start the screen recording again.'
          },
          report: reportStateRecovery
        })
    })
    if (restored) {
      recordingState = { ...restored }
      startRecordingBadgeTimer(restored.startTime)
      console.log(LOG, 'Rehydrated active recording after service worker restart', {
        name: restored.name,
        startTime: restored.startTime,
        tabId: restored.tabId
      })
      return
    }
    // Guard: a recording may have started while we were querying — don't wipe fresh state.
    if (!recordingState.active) {
      console.log(LOG, 'No surviving offscreen recording — clearing stale recording state from storage')
      await removeLocal(StorageKey.RECORDING)
    }
  } catch {
    persist(removeLocal(StorageKey.RECORDING), 'recording-clear')
  }
}

// =============================================================================
// STATE QUERIES
// =============================================================================

/** Returns whether a recording is currently active. */
export function isRecording(): boolean {
  return recordingState.active
}

/** Returns current recording info for popup sync. */
export function getRecordingInfo(): { active: boolean; name: string; startTime: number } {
  return {
    active: recordingState.active,
    name: recordingState.name,
    startTime: recordingState.startTime
  }
}

// =============================================================================
// INTERNAL HELPERS
// =============================================================================

async function clearRecordingState(): Promise<void> {
  recordingState = { ...defaultState }
  stopRecordingBadgeTimer()
  if (tabUpdateListener) {
    chrome.tabs.onUpdated.removeListener(tabUpdateListener)
    tabUpdateListener = null
  }
  await removeLocal(StorageKey.RECORDING)
}

type RecordingStartResult = { status: string; name: string; startTime?: number; error?: string }

/**
 * Resolve the tab to record: an explicit targetTabId (MCP), else the active tab.
 * Returns the tab, or an error message when the tab is missing.
 */
async function resolveRecordingTab(
  targetTabId: number | undefined
): Promise<(chrome.tabs.Tab & { id: number }) | { error: string }> {
  let tab: chrome.tabs.Tab | undefined
  if (targetTabId && targetTabId > 0) {
    try {
      tab = await chrome.tabs.get(targetTabId)
    } catch {
      console.error(LOG, 'START FAILED: target tab not found', { targetTabId })
      return { error: `RECORD_START: Target tab ${targetTabId} not found.` }
    }
  } else {
    tab = (await getActiveTab()) ?? undefined
  }
  console.log(LOG, 'Resolved recording tab:', {
    requestedTabId: targetTabId ?? null,
    resolvedTabId: tab?.id,
    url: tab?.url?.substring(0, 80),
    title: tab?.title?.substring(0, 40)
  })
  if (!tab?.id) {
    console.error(LOG, 'START FAILED: No active tab found')
    return { error: 'RECORD_START: No active tab found.' }
  }
  return tab as chrome.tabs.Tab & { id: number }
}

/**
 * Make sure the content script is responsive (needed for toasts). Reloads the
 * tab if it isn't, then waits for the extension to initialize. Skipped from the
 * popup path, where a tab reload would close the popup.
 */
async function ensureContentScriptReady(tabId: number, fromPopup: boolean): Promise<void> {
  if (fromPopup) {
    console.log(LOG, 'Skipping content script ping (fromPopup=true)')
    return
  }
  console.log(LOG, 'Pinging content script on tab', tabId)
  const alive = await pingContentScript(tabId)
  console.log(LOG, 'Content script alive:', alive)
  if (!alive) {
    console.log(LOG, 'Reloading tab for content script injection')
    chrome.tabs.reload(tabId)
    await waitForTabLoad(tabId, scaleTimeout(5000))
  }
  // Add extra delay to ensure extension is fully initialized for tabCapture.
  console.log(LOG, 'Waiting for extension to fully initialize...')
  await delay(scaleTimeout(1000))
}

/**
 * Acquire the tabCapture stream id. The popup path already holds an activeTab
 * grant; the MCP path must request the recording gesture first. Returns the
 * stream id or a fully-formed error result (the gesture path returns its own).
 */
async function acquireStreamId(
  tab: chrome.tabs.Tab,
  name: string,
  fps: number,
  audio: string,
  fromPopup: boolean
): Promise<{ streamId: string } | { errorResult: RecordingStartResult }> {
  let streamId: string
  if (fromPopup) {
    console.log(LOG, 'Getting stream ID via fromPopup path (targetTabId:', tab.id, ')')
    streamId = await getStreamIdWithRecovery(tab.id!)
  } else {
    const mediaType = audio ? 'Audio' : 'Video'
    const gestureResult = await requestRecordingGesture(tab, name, fps, audio, mediaType)
    if (gestureResult.error) {
      return { errorResult: gestureResult as RecordingStartResult }
    }
    streamId = await new Promise<string>((resolve, reject) => {
      chrome.tabCapture.getMediaStreamId({ targetTabId: tab.id! }, (id) => {
        if (chrome.runtime.lastError) {
          reject(new Error(chrome.runtime.lastError.message ?? 'getMediaStreamId failed'))
        } else {
          resolve(id)
        }
      })
    })
  }
  if (!streamId) {
    console.error(LOG, 'START FAILED: streamId is empty')
    return {
      errorResult: {
        status: 'error',
        name: '',
        error: 'RECORD_START: getMediaStreamId returned empty. Check tabCapture permission.'
      }
    }
  }
  return { streamId }
}

/**
 * Ensure the offscreen document exists, send it the START command, and wait
 * (bounded) for its confirmation. Returns the offscreen reply (success or a
 * failure/timeout message).
 */
async function startOffscreenRecording(
  streamId: string,
  tab: chrome.tabs.Tab,
  name: string,
  fps: number,
  audio: string
): Promise<OffscreenRecordingStartedMessage> {
  console.log(LOG, 'Ensuring offscreen document exists')
  await ensureOffscreenDocument()
  console.log(LOG, 'Offscreen document ready, sending START command')

  return await new Promise<OffscreenRecordingStartedMessage>((resolve) => {
    const listener = (message: OffscreenRecordingStartedMessage) => {
      if (message.target === 'background' && message.type === 'offscreen_recording_started') {
        clearTimeout(timeout)
        chrome.runtime.onMessage.removeListener(listener)
        resolve(message)
      }
    }

    const timeout = setTimeout(() => {
      chrome.runtime.onMessage.removeListener(listener)
      resolve({
        target: 'background',
        type: 'offscreen_recording_started',
        success: false,
        error: 'RECORD_START: Offscreen document timed out.'
      })
    }, scaleTimeout(10000))

    chrome.runtime.onMessage.addListener(listener)

    chrome.runtime
      .sendMessage({
        target: 'offscreen',
        type: 'offscreen_start_recording',
        streamId,
        serverUrl: getServerUrl(),
        name,
        fps,
        audioMode: audio,
        tabId: tab.id,
        url: tab.url ?? ''
      })
      .catch(() => {
        // EXPECTED_ABSENCE: a closed request port is normal because offscreen
        // replies use a separate broadcast; logging it would misleadingly report failure.
      })
  })
}

// =============================================================================
// LIFECYCLE — START
// =============================================================================

/**
 * Start recording a target tab (or the active tab when no target is provided).
 * @param name — Pre-generated filename from the Go server (e.g., "checkout-bug--2026-02-07-1423")
 * @param fps — Framerate (5–60, default 15)
 * @param queryId — PendingQuery ID for result resolution
 * @param audio — Audio mode: 'tab', 'mic', 'both', or '' (no audio)
 * @param fromPopup — true when initiated from popup (activeTab already granted, skip reload)
 */
// #lizard forgives
export async function startRecording(
  name: string,
  fps: number = 15,
  queryId: string = '',
  audio: string = '',
  fromPopup: boolean = false,
  targetTabId?: number
): Promise<{ status: string; name: string; startTime?: number; error?: string }> {
  console.log(LOG, 'startRecording called', {
    name,
    fps,
    queryId,
    audio,
    fromPopup,
    targetTabId: targetTabId ?? null,
    currentlyActive: recordingState.active
  })

  if (recordingState.active) {
    console.warn(LOG, 'START BLOCKED: already recording', { currentState: { ...recordingState } })
    return { status: 'error', name: '', error: 'RECORD_START: Already recording. Stop current recording first.' }
  }

  // Mark active immediately to prevent TOCTOU race across awaits
  recordingState.active = true // eslint-disable-line require-atomic-updates
  console.log(LOG, 'Marked active=true (TOCTOU guard)')

  // Clamp fps
  fps = Math.max(5, Math.min(60, fps))

  try {
    // Resolve the target tab (explicit MCP tab_id, else the active tab).
    const tabResult = await resolveRecordingTab(targetTabId)
    if ('error' in tabResult) {
      recordingState.active = false // eslint-disable-line require-atomic-updates
      return { status: 'error', name: '', error: tabResult.error }
    }
    const tab = tabResult

    // Auto-enable tab tracking if not already tracked
    const trackedTabId = (await readTrackedTab()).id
    console.log(LOG, 'Tracked tab:', {
      trackedTabId,
      willAutoTrack: !trackedTabId
    })
    if (!trackedTabId) {
      await setTrackedTab(tab)
    }

    // Ensure the content script is responsive (toasts); may reload the tab.
    await ensureContentScriptReady(tab.id, fromPopup)

    // Acquire the tabCapture stream id (popup activeTab grant vs MCP gesture).
    const streamResult = await acquireStreamId(tab, name, fps, audio, fromPopup)
    if ('errorResult' in streamResult) {
      recordingState.active = false // eslint-disable-line require-atomic-updates
      return streamResult.errorResult
    }
    const streamId = streamResult.streamId

    // Start the offscreen recorder and wait (bounded) for its confirmation.
    const startResult = await startOffscreenRecording(streamId, tab, name, fps, audio)

    console.log(LOG, 'Offscreen START result:', { success: startResult.success, error: startResult.error })

    if (!startResult.success) {
      recordingState.active = false // eslint-disable-line require-atomic-updates
      console.error(LOG, 'START FAILED: offscreen rejected:', startResult.error)
      return {
        status: 'error',
        name: '',
        error: startResult.error ?? 'RECORD_START: Offscreen document failed to start recording.'
      }
    }

    /* eslint-disable require-atomic-updates */
    recordingState = {
      active: true,
      name,
      startTime: Date.now(),
      fps,
      audioMode: audio,
      tabId: tab.id,
      url: tab.url ?? '',
      queryId
    }
    /* eslint-enable require-atomic-updates */

    // Persist state for popup sync + rehydration after service-worker restart
    const persisted: PersistedRecordingState = {
      active: true,
      name,
      startTime: recordingState.startTime,
      fps,
      audioMode: audio,
      tabId: tab.id,
      url: tab.url ?? '',
      queryId
    }
    await setLocals({ [StorageKey.RECORDING]: persisted })
    startRecordingBadgeTimer(recordingState.startTime)

    // Show "Recording started" toast (fades after 2s)
    sendTabToast(tab.id, buildRecordingToastLabel(tab.url), '', 'success', scaleTimeout(2000))

    // Watermark removed — badge timer on extension icon replaces it (not captured by tabCapture)
    if (tabUpdateListener) chrome.tabs.onUpdated.removeListener(tabUpdateListener)
    tabUpdateListener = null

    console.log(LOG, 'Recording STARTED successfully', { name, tabId: tab.id, audioMode: audio, fps })
    return { status: 'recording', name, startTime: recordingState.startTime }
  } catch (err) {
    recordingState.active = false // eslint-disable-line require-atomic-updates
    console.error(LOG, 'START EXCEPTION:', errorMessage(err), (err as Error).stack)
    return {
      status: 'error',
      name: '',
      error: `RECORD_START: ${errorMessage(err, 'Failed to start recording.')}`
    }
  }
}

// =============================================================================
// LIFECYCLE — STOP
// =============================================================================

/**
 * Stop recording and save the video.
 * @param truncated — true if auto-stopped due to memory guard or tab close
 */
// #lizard forgives
export async function stopRecording(truncated: boolean = false): Promise<{
  status: string
  name: string
  duration_seconds?: number
  size_bytes?: number
  truncated?: boolean
  path?: string
  error?: string
}> {
  console.log(LOG, 'stopRecording called', {
    currentlyActive: recordingState.active,
    name: recordingState.name,
    tabId: recordingState.tabId,
    truncated
  })

  if (!recordingState.active) {
    // Clean up stale storage in case of zombie recording state (e.g., service worker restarted)
    console.warn(LOG, 'STOP: No active recording in memory — cleaning up zombie storage')
    stopRecordingBadgeTimer()
    persist(removeLocal(StorageKey.RECORDING), 'recording-clear')
    return { status: 'error', name: '', error: 'RECORD_STOP: No active recording.' }
  }

  const { tabId } = recordingState

  // Mark as no longer active immediately to prevent double-stop
  recordingState.active = false
  stopRecordingBadgeTimer()
  console.log(LOG, 'Marked active=false, sending STOP to offscreen')

  try {
    // Send stop command to offscreen document and wait for result (30s timeout for upload)
    const stopResult = await new Promise<OffscreenRecordingStoppedMessage>((resolve) => {
      const listener = (message: OffscreenRecordingStoppedMessage) => {
        if (message.target === 'background' && message.type === 'offscreen_recording_stopped') {
          clearTimeout(timeout)
          chrome.runtime.onMessage.removeListener(listener)
          resolve(message)
        }
      }

      const timeout = setTimeout(() => {
        chrome.runtime.onMessage.removeListener(listener)
        resolve({
          target: 'background',
          type: 'offscreen_recording_stopped',
          status: 'error',
          name: recordingState.name || '',
          error: 'RECORD_STOP: Offscreen document timed out during save.'
        })
      }, scaleTimeout(30000))

      chrome.runtime.onMessage.addListener(listener)

      chrome.runtime
        .sendMessage({
          target: 'offscreen',
          type: 'offscreen_stop_recording'
        })
        .catch(() => {
          // EXPECTED_ABSENCE: a closed request port is normal because offscreen
          // replies use a separate broadcast; logging it would misleadingly report failure.
        })
    })

    console.log(LOG, 'Offscreen STOP result:', {
      status: stopResult.status,
      name: stopResult.name,
      error: stopResult.error,
      size: stopResult.size_bytes,
      path: stopResult.path
    })

    await clearRecordingState()

    // Show save toast on the recorded tab
    if (tabId && stopResult.status === 'saved') {
      const sizeMB = stopResult.size_bytes ? (stopResult.size_bytes / (1024 * 1024)).toFixed(1) : '?'
      sendTabToast(
        tabId,
        'Recording saved',
        `${stopResult.path ?? stopResult.name} (${sizeMB} MB)`,
        'success',
        scaleTimeout(5000)
      )
    }

    return {
      status: stopResult.status,
      name: stopResult.name,
      duration_seconds: stopResult.duration_seconds,
      size_bytes: stopResult.size_bytes,
      truncated: stopResult.truncated,
      path: stopResult.path,
      error: stopResult.error
    }
  } catch (err) {
    console.error(LOG, 'STOP EXCEPTION:', errorMessage(err), (err as Error).stack)
    // Capture the name BEFORE clearing — clearRecordingState resets it to ''.
    const failedName = recordingState.name || ''
    await clearRecordingState()
    return {
      status: 'error',
      name: failedName,
      error: `RECORD_STOP: ${errorMessage(err, 'Failed to stop recording.')}`
    }
  }
}

// =============================================================================
// CHROME RUNTIME LISTENERS (delegated to recording-listeners.ts)
// =============================================================================

// Guard: all top-level event listeners require chrome runtime (not available in test contexts)
if (typeof chrome !== 'undefined' && chrome.runtime?.onMessage) {
  installRecordingListeners({
    startRecording,
    stopRecording,
    isActive: () => recordingState.active,
    getTabId: () => recordingState.tabId,
    setInactive: () => {
      recordingState.active = false
    },
    clearRecordingState,
    getServerUrl: () => getServerUrl()
  })
}

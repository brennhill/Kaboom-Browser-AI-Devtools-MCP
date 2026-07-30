/**
 * Purpose: Implements popup recording controls, mic-permission flow, and saved-recording reveal behavior.
 * Why: Gives users reliable start/stop control with explicit permission/error handling for tab capture sessions.
 * Docs: docs/features/feature/playback-engine/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */

/**
 * @fileoverview Recording UI Module for Popup
 * Manages recording controls, timer display, and mic permission flow.
 */

import { StorageKey } from '../../lib/constants.js'
import { KABOOM_RECORDING_LOG_PREFIX } from '../../lib/brand.js'
import { onStorageChanged } from '../../lib/storage/changes.js'
import { persist } from '../../lib/storage/io.js'
import { removeLocal } from '../../lib/storage/local.js'
import { readLocalState } from '../../lib/storage/validated.js'
import { reportStateRecovery } from '../../lib/storage/recovery.js'
import {
  sendRecordingGestureDecision,
  handleStartClick,
  handleStopClick,
  type RecordingElements,
  type RecordingState
} from './recording-io.js'

interface PendingRecordingIntent {
  highlight?: boolean
  name?: string
  fps?: number
  audio?: string
  tabId?: number
  url?: string
}

function isAudioMode(value: unknown): value is string {
  return value === '' || value === 'tab' || value === 'mic' || value === 'both'
}

function isPendingRecordingIntent(value: unknown): value is PendingRecordingIntent {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false
  const intent = value as PendingRecordingIntent
  return (
    (intent.highlight === undefined || typeof intent.highlight === 'boolean') &&
    (intent.name === undefined || typeof intent.name === 'string') &&
    (intent.fps === undefined || typeof intent.fps === 'number') &&
    (intent.audio === undefined || isAudioMode(intent.audio)) &&
    (intent.tabId === undefined || typeof intent.tabId === 'number') &&
    (intent.url === undefined || typeof intent.url === 'string')
  )
}

function isPopupRecordingState(value: unknown): value is { active?: boolean; name?: string; startTime?: number } {
  return (
    typeof value === 'object' &&
    value !== null &&
    (typeof (value as { active?: unknown }).active === 'boolean' ||
      (value as { active?: unknown }).active === undefined) &&
    (typeof (value as { name?: unknown }).name === 'string' || (value as { name?: unknown }).name === undefined) &&
    (typeof (value as { startTime?: unknown }).startTime === 'number' ||
      (value as { startTime?: unknown }).startTime === undefined)
  )
}

interface ApprovalElements {
  card: HTMLElement | null
  detail: HTMLElement | null
  approveBtn: HTMLButtonElement | null
  denyBtn: HTMLButtonElement | null
}

function recordingDiagnostic(detail: string) {
  return {
    name: 'screen_recording_state',
    detail,
    fix: 'Start the screen recording flow again.'
  } as const
}

const START_LABEL = 'Record screen'
const STOP_LABEL = 'Stop recording'
const HIGHLIGHT_LABEL = '\u25CF \u00AB Click here to record'
const RECENT_RECORDING_START_MS = 8000
const TOP_NOTICE_DURATION_MS = 4000
const LOG = `${KABOOM_RECORDING_LOG_PREFIX} Popup:`
const AUDIO_LABELS: Record<string, string> = {
  '': 'Video only',
  tab: 'Video + tab audio',
  mic: 'Video + microphone',
  both: 'Video + tab + mic'
}

let topNoticeTimer: ReturnType<typeof setTimeout> | null = null

function getRecordSection(els: RecordingElements): Element | null {
  const closest = (els.row as { closest?: unknown }).closest
  if (typeof closest !== 'function') return null
  return closest.call(els.row, '.section') as Element | null
}

function applyRecordHighlight(els: RecordingElements): void {
  const section = getRecordSection(els)
  if (section) section.classList.add('record-highlight')
  els.label.textContent = HIGHLIGHT_LABEL
}

function removeRecordHighlight(els: RecordingElements): void {
  const section = getRecordSection(els)
  if (section) section.classList.remove('record-highlight')
  if (els.label.textContent === HIGHLIGHT_LABEL) {
    els.label.textContent = START_LABEL
  }
}

// #lizard forgives
function showRecording(els: RecordingElements, state: RecordingState, name: string, startTime: number): void {
  const wasRecording = state.isRecording
  removeRecordHighlight(els)
  state.isRecording = true
  els.row.classList.add('is-recording')
  els.label.textContent = STOP_LABEL
  els.statusEl.textContent = ''
  if (els.optionsEl) els.optionsEl.style.display = 'none'

  if (state.timerInterval) clearInterval(state.timerInterval)
  state.timerInterval = setInterval(() => {
    const elapsed = Math.round((Date.now() - startTime) / 1000)
    const mins = Math.floor(elapsed / 60)
    const secs = elapsed % 60
    els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`
  }, 1000)

  if (!wasRecording && Date.now() - startTime <= RECENT_RECORDING_START_MS) {
    showTopNotice(els, 'Recording started')
  }
}

function showIdle(els: RecordingElements, state: RecordingState): void {
  state.isRecording = false
  removeRecordHighlight(els)
  els.row.classList.remove('is-recording')
  els.label.textContent = START_LABEL
  els.statusEl.textContent = ''
  if (els.optionsEl) els.optionsEl.style.display = 'block'
  if (state.timerInterval) {
    clearInterval(state.timerInterval)
    state.timerInterval = null
  }
}

function describePendingRecording(pending: PendingRecordingIntent): string {
  const parts: string[] = []
  if (pending.name) parts.push(`Name: ${pending.name}`)
  if (typeof pending.fps === 'number') parts.push(`FPS: ${pending.fps}`)
  const audioLabel = AUDIO_LABELS[pending.audio ?? ''] ?? AUDIO_LABELS['']
  parts.push(`Mode: ${audioLabel}`)
  return parts.join(' \u00b7 ')
}

function setApprovalPendingState(
  els: RecordingElements,
  approvalEls: ApprovalElements,
  state: RecordingState,
  pending: PendingRecordingIntent | null
): void {
  const setRowAriaDisabled = (value: string | null): void => {
    const setAttr = (els.row as { setAttribute?: unknown }).setAttribute
    const removeAttr = (els.row as { removeAttribute?: unknown }).removeAttribute
    if (value !== null) {
      if (typeof setAttr === 'function') setAttr.call(els.row, 'aria-disabled', value)
      return
    }
    if (typeof removeAttr === 'function') removeAttr.call(els.row, 'aria-disabled')
  }

  const approvalPending = Boolean(pending && !pending.highlight && !state.isRecording)
  if (approvalPending) {
    if (approvalEls.detail && pending) approvalEls.detail.textContent = describePendingRecording(pending)
    if (approvalEls.card) approvalEls.card.style.display = 'block'
    els.row.classList.add('is-disabled')
    setRowAriaDisabled('true')
    if (els.optionsEl) els.optionsEl.style.display = 'none'
    return
  }
  if (approvalEls.detail) approvalEls.detail.textContent = ''
  if (approvalEls.card) approvalEls.card.style.display = 'none'
  els.row.classList.remove('is-disabled')
  setRowAriaDisabled(null)
  if (!state.isRecording && els.optionsEl) els.optionsEl.style.display = 'block'
}

function showTopNotice(els: RecordingElements, text: string): void {
  const notice = els.topNoticeEl
  if (!notice) return
  notice.textContent = text
  notice.style.display = 'block'
  if (topNoticeTimer) clearTimeout(topNoticeTimer)
  topNoticeTimer = setTimeout(() => {
    notice.style.display = 'none'
  }, TOP_NOTICE_DURATION_MS)
}

function showSavedLink(saveInfoEl: HTMLElement, displayName: string, filePath: string): void {
  saveInfoEl.textContent = 'Saved: '
  const link = document.createElement('a')
  link.href = '#'
  link.id = 'reveal-recording'
  link.textContent = displayName
  link.style.color = '#58a6ff'
  link.style.textDecoration = 'underline'
  link.style.cursor = 'pointer'
  saveInfoEl.appendChild(link)
  const linkEl = document.getElementById('reveal-recording')
  if (linkEl) {
    linkEl.addEventListener('click', (e) => {
      e.preventDefault()
      chrome.runtime.sendMessage({ type: 'reveal_file', path: filePath }, (result: { error?: string } | undefined) => {
        if (result?.error) {
          saveInfoEl.textContent = `Could not open folder: ${result.error}`
          saveInfoEl.style.color = '#f85149'
          setTimeout(() => {
            saveInfoEl.style.display = 'none'
          }, 5000)
        }
      })
    })
  }
}

function showSaveResult(
  saveInfoEl: HTMLElement | null,
  resp: { status?: string; name?: string; path?: string; error?: string } | undefined
): void {
  if (resp?.status !== 'saved' || !resp.name || !saveInfoEl) return
  const displayName = resp.name.replace(/--\d{4}-\d{2}-\d{2}-\d{4}(-\d+)?$/, '')
  if (resp.path) {
    showSavedLink(saveInfoEl, displayName, resp.path)
  } else {
    saveInfoEl.textContent = `Saved: ${displayName}`
  }
  saveInfoEl.style.display = 'block'
  setTimeout(() => {
    saveInfoEl.style.display = 'none'
  }, 12000)
}

function showStartError(saveInfoEl: HTMLElement | null, errorText: string): void {
  if (!saveInfoEl) return
  saveInfoEl.textContent = errorText
  saveInfoEl.style.display = 'block'
  saveInfoEl.style.background = 'rgba(248, 81, 73, 0.1)'
  saveInfoEl.style.color = '#f85149'
  setTimeout(() => {
    saveInfoEl.style.display = 'none'
    saveInfoEl.style.background = 'rgba(63, 185, 80, 0.1)'
    saveInfoEl.style.color = '#3fb950'
  }, 5000)
}

export function setupRecordingUI(): void {
  const row = document.getElementById('record-row')
  const label = document.getElementById('record-label')
  const statusEl = document.getElementById('recording-status')
  if (!row || !label || !statusEl) return

  const els: RecordingElements = {
    row,
    label,
    statusEl,
    optionsEl: document.getElementById('record-options'),
    saveInfoEl: document.getElementById('record-save-info'),
    topNoticeEl: document.getElementById('record-top-notice')
  }
  const approvalEls: ApprovalElements = {
    card: document.getElementById('record-approval-card'),
    detail: document.getElementById('record-approval-detail'),
    approveBtn: document.getElementById('record-approve-btn') as HTMLButtonElement | null,
    denyBtn: document.getElementById('record-deny-btn') as HTMLButtonElement | null
  }

  const state: RecordingState = { isRecording: false, timerInterval: null }
  let pendingRecordingIntent: PendingRecordingIntent | null = null

  const updatePendingRecording = (pendingValue: unknown): void => {
    const pending = pendingValue as PendingRecordingIntent | undefined
    if (pending?.highlight && !state.isRecording) {
      applyRecordHighlight(els)
      pendingRecordingIntent = null
      setApprovalPendingState(els, approvalEls, state, null)
      persist(removeLocal(StorageKey.PENDING_RECORDING), 'pending-recording-clear')
      return
    }
    pendingRecordingIntent = pending && !pending.highlight ? pending : null
    if (!pendingRecordingIntent && !state.isRecording) removeRecordHighlight(els)
    setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent)
  }

  const clearPendingRecordingIntent = (): void => {
    pendingRecordingIntent = null
    setApprovalPendingState(els, approvalEls, state, null)
    persist(removeLocal(StorageKey.PENDING_RECORDING), 'pending-recording-clear')
  }

  // Row is visible immediately with default "not recording" state.
  // Storage read updates it async — visual change is minimal (button label toggle).
  void readLocalState<{ active?: boolean; name?: string; startTime?: number } | null>({
    key: StorageKey.RECORDING,
    fallback: null,
    validate: isPopupRecordingState,
    diagnostic: recordingDiagnostic('Saved screen recording was invalid or unreadable; idle state is active.')
  }).then(async (rec) => {
    console.log(LOG, 'recording state from storage:', rec)
    if (rec?.active && rec.name && rec.startTime) {
      console.log(LOG, 'resuming recording UI for', rec.name)
      showRecording(els, state, rec.name, rec.startTime)
    }

    // Check for highlight request from hover launcher
    const pendingValue = await readLocalState<PendingRecordingIntent | null>({
      key: StorageKey.PENDING_RECORDING,
      fallback: null,
      validate: isPendingRecordingIntent,
      diagnostic: recordingDiagnostic('Saved pending recording request was invalid or unreadable; it was cancelled.')
    })
    updatePendingRecording(pendingValue)
  })

  onStorageChanged((changes, areaName) => {
    if (areaName === 'local' && changes[StorageKey.RECORDING]) {
      const rec = changes[StorageKey.RECORDING]!.newValue
      if (rec !== undefined && !isPopupRecordingState(rec)) {
        reportStateRecovery(recordingDiagnostic('Screen recording changed to an invalid value; idle state is active.'))
        showIdle(els, state)
        return
      }
      console.log(LOG, 'recording state changed:', rec)
      if (rec?.active && rec.name && rec.startTime) {
        showRecording(els, state, rec.name, rec.startTime)
      } else {
        showIdle(els, state)
      }
      setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent)
      return
    }
    if (areaName === 'local' && changes[StorageKey.PENDING_RECORDING]) {
      const pending = changes[StorageKey.PENDING_RECORDING]!.newValue
      if (pending !== undefined && !isPendingRecordingIntent(pending)) {
        reportStateRecovery(recordingDiagnostic('Pending recording changed to an invalid value; it was cancelled.'))
        updatePendingRecording(null)
        return
      }
      updatePendingRecording(pending)
    }
  })

  approvalEls.approveBtn?.addEventListener('click', (event) => {
    event.preventDefault()
    sendRecordingGestureDecision('recording_gesture_granted')
    clearPendingRecordingIntent()
  })

  approvalEls.denyBtn?.addEventListener('click', (event) => {
    event.preventDefault()
    sendRecordingGestureDecision('recording_gesture_denied')
    clearPendingRecordingIntent()
  })

  void readLocalState<{ audioMode?: string } | null>({
    key: StorageKey.PENDING_MIC_RECORDING,
    fallback: null,
    validate: (value): value is { audioMode?: string } =>
      typeof value === 'object' &&
      value !== null &&
      ((value as { audioMode?: unknown }).audioMode === undefined ||
        isAudioMode((value as { audioMode?: unknown }).audioMode)),
    diagnostic: recordingDiagnostic('Saved microphone recording request was invalid or unreadable; it was cancelled.')
  }).then(async (intent) => {
    console.log(LOG, 'pending mic recording intent:', intent)
    if (!intent?.audioMode) return

    console.log(LOG, 'consuming mic intent, pre-selecting audio mode:', intent.audioMode)
    await removeLocal(StorageKey.PENDING_MIC_RECORDING)

    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (tabs[0]?.id) {
        chrome.tabs
          .sendMessage(tabs[0].id, {
            type: 'kaboom_action_toast',
            text: '',
            detail: '',
            state: 'success' as const,
            duration_ms: 1
          })
          .catch(() => {})
      }
    })

    const audioSelect = document.getElementById('record-audio-mode') as HTMLSelectElement | null
    if (audioSelect) audioSelect.value = intent.audioMode
  })

  void readLocalState<string>({
    key: StorageKey.RECORD_AUDIO_PREF,
    fallback: '',
    validate: (value): value is string => isAudioMode(value),
    diagnostic: recordingDiagnostic('Saved recording audio preference was invalid or unreadable; video-only is active.')
  }).then((saved) => {
    if (saved) {
      const audioSelect = document.getElementById('record-audio-mode') as HTMLSelectElement | null
      if (audioSelect) audioSelect.value = saved
    }
  })

  row.addEventListener('click', () => {
    console.log(LOG, 'record row clicked, isRecording:', state.isRecording)
    if (pendingRecordingIntent && !state.isRecording) {
      console.log(LOG, 'record row click ignored while approval is pending')
      return
    }
    removeRecordHighlight(els)
    if (state.isRecording) {
      handleStopClick(els, state, showIdle, showSaveResult)
    } else {
      handleStartClick(els, state, showRecording, showIdle, showStartError)
    }
  })
}

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
import { getLocal, removeLocal } from '../../lib/storage/local.js'
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

interface ApprovalElements {
  card: HTMLElement | null
  detail: HTMLElement | null
  approveBtn: HTMLButtonElement | null
  denyBtn: HTMLButtonElement | null
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
  void getLocal(StorageKey.RECORDING).then(async (value: unknown) => {
    const rec = value as { active?: boolean; name?: string; startTime?: number } | undefined
    console.log(LOG, 'recording state from storage:', rec)
    if (rec?.active && rec.name && rec.startTime) {
      console.log(LOG, 'resuming recording UI for', rec.name)
      showRecording(els, state, rec.name, rec.startTime)
    }

    // Check for highlight request from hover launcher
    const pendingValue = await getLocal(StorageKey.PENDING_RECORDING)
    updatePendingRecording(pendingValue)
  })

  onStorageChanged((changes, areaName) => {
    if (areaName === 'local' && changes[StorageKey.RECORDING]) {
      const rec = changes[StorageKey.RECORDING]!.newValue as
        | { active?: boolean; name?: string; startTime?: number }
        | undefined
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
      updatePendingRecording(changes[StorageKey.PENDING_RECORDING]!.newValue)
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

  void getLocal(StorageKey.PENDING_MIC_RECORDING).then(async (value: unknown) => {
    const intent = value as { audioMode?: string } | undefined
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

  void getLocal(StorageKey.RECORD_AUDIO_PREF).then((value: unknown) => {
    const saved = value as string | undefined
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

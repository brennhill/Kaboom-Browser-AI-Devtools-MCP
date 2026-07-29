/**
 * Purpose: Chrome runtime messaging, storage, and mic permission logic for recording controls.
 * Why: Separates browser API side-effects from recording UI rendering.
 * Docs: docs/features/feature/playback-engine/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */

import { StorageKey } from '../../lib/constants.js'
import { KABOOM_RECORDING_LOG_PREFIX } from '../../lib/brand.js'
import { errorMessage } from '../../lib/error-utils.js'
import { persist } from '../../lib/storage/io.js'
import { removeLocal, setLocal } from '../../lib/storage/local.js'

export interface RecordingElements {
  row: HTMLElement
  label: HTMLElement
  statusEl: HTMLElement
  optionsEl: HTMLElement | null
  saveInfoEl: HTMLElement | null
  topNoticeEl: HTMLElement | null
}

export interface RecordingState {
  isRecording: boolean
  timerInterval: ReturnType<typeof setInterval> | null
}

export type ShowRecordingFn = (els: RecordingElements, state: RecordingState, name: string, startTime: number) => void
export type ShowIdleFn = (els: RecordingElements, state: RecordingState) => void
export type ShowStartErrorFn = (saveInfoEl: HTMLElement | null, errorText: string) => void

const LOG = `${KABOOM_RECORDING_LOG_PREFIX} Popup:`

export function sendRecordingGestureDecision(type: 'recording_gesture_granted' | 'recording_gesture_denied'): void {
  chrome.runtime.sendMessage({ type }, () => {
    void chrome.runtime.lastError
  })
}

function showMicPermissionPrompt(saveInfoEl: HTMLElement, audioMode: string): void {
  chrome.tabs.query({ active: true, currentWindow: true }, (activeTabs) => {
    persist(
      setLocal(StorageKey.PENDING_MIC_RECORDING, { audioMode, returnTabId: activeTabs[0]?.id }),
      'pending-mic-recording'
    )
  })
  saveInfoEl.innerHTML =
    'Microphone access needed. <a href="#" id="grant-mic-link" style="color: #58a6ff; text-decoration: underline; cursor: pointer">Grant access</a>'
  saveInfoEl.style.display = 'block'
  saveInfoEl.style.background = 'rgba(248, 81, 73, 0.1)'
  saveInfoEl.style.color = '#f85149'
  const link = document.getElementById('grant-mic-link')
  if (link) {
    link.addEventListener('click', (e) => {
      e.preventDefault()
      chrome.tabs.create({ url: chrome.runtime.getURL('mic-permission.html') })
    })
  }
}

function sendRecordStart(
  els: RecordingElements,
  state: RecordingState,
  audioMode: string,
  showRecording: ShowRecordingFn,
  showIdle: ShowIdleFn,
  showStartError: ShowStartErrorFn
): void {
  console.log(LOG, 'sendStart() called, sending screen_recording_start with audio:', audioMode)
  chrome.runtime.sendMessage(
    { type: 'screen_recording_start', audio: audioMode },
    (resp: { status?: string; name?: string; startTime?: number; error?: string } | undefined) => {
      console.log(LOG, 'screen_recording_start response:', resp)
      if (chrome.runtime.lastError) {
        console.error(LOG, 'screen_recording_start lastError:', chrome.runtime.lastError.message)
      }
      if (resp?.status === 'recording' && resp.name) {
        showRecording(els, state, resp.name, resp.startTime ?? Date.now())
      } else {
        showIdle(els, state)
        if (resp?.error) showStartError(els.saveInfoEl, resp.error)
      }
    }
  )
}

// #lizard forgives
function tryMicPermissionThenStart(
  els: RecordingElements,
  state: RecordingState,
  audioMode: string,
  showRecording: ShowRecordingFn,
  showIdle: ShowIdleFn,
  showStartError: ShowStartErrorFn
): void {
  console.log(LOG, 'trying getUserMedia from popup...')
  navigator.mediaDevices
    .getUserMedia({ audio: true })
    .then((micStream) => {
      console.log(LOG, 'getUserMedia succeeded from popup')
      micStream.getTracks().forEach((t) => t.stop())
      persist(setLocal(StorageKey.MIC_GRANTED, true), 'mic-granted')
      sendRecordStart(els, state, audioMode, showRecording, showIdle, showStartError)
    })
    .catch((err) => {
      console.log(LOG, 'getUserMedia FAILED:', (err as Error).name, errorMessage(err))
      persist(removeLocal(StorageKey.MIC_GRANTED), 'mic-granted-clear')
      showIdle(els, state)
      if (els.saveInfoEl) showMicPermissionPrompt(els.saveInfoEl, audioMode)
    })
}

export function handleStartClick(
  els: RecordingElements,
  state: RecordingState,
  showRecording: ShowRecordingFn,
  showIdle: ShowIdleFn,
  showStartError: ShowStartErrorFn
): void {
  const audioSelect = document.getElementById('record-audio-mode') as HTMLSelectElement | null
  const audioMode = audioSelect?.value ?? ''
  persist(setLocal(StorageKey.RECORD_AUDIO_PREF, audioMode), 'record-audio-pref')
  if (els.optionsEl) els.optionsEl.style.display = 'none'
  if (els.saveInfoEl) els.saveInfoEl.style.display = 'none'
  els.label.textContent = 'Starting...'

  if (audioMode === 'mic' || audioMode === 'both') {
    console.log(LOG, 'mic/both mode — checking stored mic approval')
    tryMicPermissionThenStart(els, state, audioMode, showRecording, showIdle, showStartError)
  } else {
    sendRecordStart(els, state, audioMode, showRecording, showIdle, showStartError)
  }
}

export function handleStopClick(
  els: RecordingElements,
  state: RecordingState,
  showIdle: ShowIdleFn,
  showSaveResult: (
    saveInfoEl: HTMLElement | null,
    resp: { status?: string; name?: string; path?: string; error?: string } | undefined
  ) => void
): void {
  els.row.classList.remove('is-recording')
  els.label.textContent = 'Saving...'
  console.log(LOG, 'sending screen_recording_stop')
  chrome.runtime.sendMessage(
    { type: 'screen_recording_stop' },
    (resp: { status?: string; name?: string; path?: string; error?: string } | undefined) => {
      console.log(LOG, 'screen_recording_stop response:', resp)
      if (chrome.runtime.lastError) {
        console.error(LOG, 'screen_recording_stop lastError:', chrome.runtime.lastError.message)
      }
      showIdle(els, state)
      showSaveResult(els.saveInfoEl, resp)
    }
  )
}

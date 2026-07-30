/**
 * Purpose: Decides whether in-memory recording state should be rehydrated after a service-worker restart.
 * Why: MV3 service workers restart routinely while the offscreen MediaRecorder keeps recording;
 *      the offscreen document is the source of truth for "is a recording still active".
 * Docs: docs/features/feature/tab-recording/index.md
 */

// recording-rehydration.ts — Pure rehydration decision logic for recording.ts.
// No chrome API access here: dependencies are injected so the decision is unit-testable.

import type { OffscreenRecordingStateResponse } from '../../types/runtime-messages.js'

/** Recording metadata persisted under StorageKey.RECORDING at recording start. */
export interface PersistedRecordingState {
  active?: boolean
  name?: string
  startTime?: number
  fps?: number
  audioMode?: string
  tabId?: number
  url?: string
  queryId?: string
}

export function isPersistedRecordingState(value: unknown): value is PersistedRecordingState {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false
  const state = value as PersistedRecordingState
  return (
    (state.active === undefined || typeof state.active === 'boolean') &&
    (state.name === undefined || typeof state.name === 'string') &&
    (state.startTime === undefined || typeof state.startTime === 'number') &&
    (state.fps === undefined || typeof state.fps === 'number') &&
    (state.audioMode === undefined || typeof state.audioMode === 'string') &&
    (state.tabId === undefined || typeof state.tabId === 'number') &&
    (state.url === undefined || typeof state.url === 'string') &&
    (state.queryId === undefined || typeof state.queryId === 'string')
  )
}

/** Fully-populated recording state restored after a service-worker restart. */
export interface RehydratedRecordingState {
  active: true
  name: string
  startTime: number
  fps: number
  audioMode: string
  tabId: number
  url: string
  queryId: string
}

export interface RecordingRehydrationDeps {
  /** Ask the offscreen document for its live recording state. Resolves null when unreachable. */
  queryOffscreenRecordingState: () => Promise<OffscreenRecordingStateResponse | null>
  /** Read persisted recording metadata (StorageKey.RECORDING). Resolves null when absent. */
  getPersistedRecording: () => Promise<PersistedRecordingState | null>
}

/**
 * Decide whether an active recording survived a service-worker restart.
 * Returns the rehydrated state when the offscreen document reports an active
 * recording (preferring live offscreen values, falling back to persisted
 * metadata), otherwise null — in which case the caller should clear stale
 * persisted recording state.
 */
export async function resolveRecordingRehydration(
  deps: RecordingRehydrationDeps
): Promise<RehydratedRecordingState | null> {
  const offscreen = await deps.queryOffscreenRecordingState()
  if (!offscreen?.active) return null

  let persisted: PersistedRecordingState | null = null
  try {
    persisted = await deps.getPersistedRecording()
  } catch {
    persisted = null
  }

  return {
    active: true,
    name: offscreen.name || persisted?.name || '',
    startTime: offscreen.startTime || persisted?.startTime || Date.now(),
    fps: offscreen.fps || persisted?.fps || 15,
    audioMode: offscreen.audioMode || persisted?.audioMode || '',
    tabId: offscreen.tabId || persisted?.tabId || 0,
    url: offscreen.url || persisted?.url || '',
    queryId: persisted?.queryId ?? ''
  }
}

/**
 * Purpose: Defines tracked-tab continuity phases and runtime message contracts.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

export type TrackingContinuityPhase =
  | 'idle'
  | 'confirmed'
  | 'navigation_started'
  | 'provisional_url'
  | 'content_injecting'
  | 'extension_reconnecting'
  | 'recovery_failed'

export interface TrackingContinuitySnapshot {
  readonly tab_id?: number
  readonly phase: TrackingContinuityPhase
  readonly is_tracked: boolean
  readonly confirmed_url?: string
  readonly provisional_url?: string
  readonly failure?: string
}

export interface GetTrackingStateMessage {
  readonly type: 'get_tracking_state'
}

export interface TrackingState {
  readonly isTracked: boolean
  readonly aiPilotEnabled: boolean
}

export interface GetTrackingStateResponse {
  readonly state: TrackingState & {
    continuity: TrackingContinuitySnapshot
  }
}

export interface TrackingStateChangedMessage {
  readonly type: 'tracking_state_changed'
  readonly state: TrackingState
}

export interface TrackingContentReadyMessage {
  readonly type: 'tracking_content_ready'
  readonly url: string
}

export interface TrackingReadinessProbeMessage {
  readonly type: 'tracking_readiness_probe'
  readonly correlation_id: string
  readonly connection_generation: number
}

export interface TrackingReadinessProbeResponse {
  readonly ready: true
  readonly correlation_id: string
  readonly connection_generation: number
}

export interface TrackingContinuityChangedMessage {
  readonly type: 'tracking_continuity_changed'
  readonly snapshot: TrackingContinuitySnapshot
}

const TRACKING_PHASES: ReadonlySet<TrackingContinuityPhase> = new Set([
  'idle',
  'confirmed',
  'navigation_started',
  'provisional_url',
  'content_injecting',
  'extension_reconnecting',
  'recovery_failed'
])

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isTrackingState(value: unknown): value is TrackingState {
  return isRecord(value) && typeof value.isTracked === 'boolean' && typeof value.aiPilotEnabled === 'boolean'
}

export function isTrackingStateChangedMessage(value: unknown): value is TrackingStateChangedMessage {
  return isRecord(value) && value.type === 'tracking_state_changed' && isTrackingState(value.state)
}

export function isGetTrackingStateResponse(value: unknown): value is GetTrackingStateResponse {
  if (!isRecord(value) || !isRecord(value.state) || !isTrackingState(value.state)) return false
  const continuity = value.state.continuity
  return (
    isRecord(continuity) &&
    typeof continuity.phase === 'string' &&
    TRACKING_PHASES.has(continuity.phase as TrackingContinuityPhase) &&
    typeof continuity.is_tracked === 'boolean'
  )
}

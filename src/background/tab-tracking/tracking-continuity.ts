/**
 * Purpose: Own the tracked-tab continuity state machine across navigation and reinjection.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import type { TrackingContinuitySnapshot } from '../../types/runtime/tracking.js'

type Listener = (snapshot: TrackingContinuitySnapshot) => void

export class TrackingContinuity {
  private state: TrackingContinuitySnapshot = { phase: 'idle', is_tracked: false }
  private readonly listeners = new Set<Listener>()

  snapshot(): TrackingContinuitySnapshot {
    return { ...this.state }
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  establish(tabId: number, url?: string): void {
    if (!this.canOwn(tabId)) return
    this.setConfirmed(tabId, url)
  }

  confirm(tabId: number, url?: string): void {
    if (this.state.tab_id !== tabId) return
    this.setConfirmed(tabId, url)
  }

  private setConfirmed(tabId: number, url?: string): void {
    this.transition({
      tab_id: tabId,
      phase: 'confirmed',
      is_tracked: true,
      ...(url ? { confirmed_url: url } : {})
    })
  }

  navigationStarted(tabId: number): void {
    this.forTrackedTab(tabId, (current) => ({
      ...current,
      phase: 'navigation_started',
      failure: undefined
    }))
  }

  observeProvisionalURL(tabId: number, url: string): void {
    this.forTrackedTab(tabId, (current) => ({
      ...current,
      phase: 'provisional_url',
      provisional_url: url,
      failure: undefined
    }))
  }

  injectionStarted(tabId: number): void {
    this.forTrackedTab(tabId, (current) => {
      if (
        current.phase !== 'navigation_started' &&
        current.phase !== 'provisional_url' &&
        current.phase !== 'extension_reconnecting'
      ) {
        return current
      }
      return { ...current, phase: 'content_injecting', failure: undefined }
    })
  }

  extensionReconnectStarted(tabId: number): void {
    this.forTrackedTab(tabId, (current) => ({
      ...current,
      phase: 'extension_reconnecting',
      failure: undefined
    }))
  }

  fail(tabId: number, failure: string): void {
    this.forTrackedTab(tabId, (current) => ({
      ...current,
      phase: 'recovery_failed',
      failure
    }))
  }

  close(tabId: number): void {
    if (this.state.tab_id !== tabId) return
    this.transition({ phase: 'idle', is_tracked: false })
  }

  private canOwn(tabId: number): boolean {
    return Number.isInteger(tabId) && tabId > 0 && (this.state.tab_id === undefined || this.state.tab_id === tabId)
  }

  private forTrackedTab(
    tabId: number,
    update: (current: TrackingContinuitySnapshot) => TrackingContinuitySnapshot
  ): void {
    if (this.state.tab_id !== tabId) return
    const next = update(this.state)
    if (next === this.state) return
    this.transition(next)
  }

  private transition(next: TrackingContinuitySnapshot): void {
    this.state = withoutUndefined(next)
    const snapshot = this.snapshot()
    for (const listener of this.listeners) listener(snapshot)
  }
}

function withoutUndefined(snapshot: TrackingContinuitySnapshot): TrackingContinuitySnapshot {
  return Object.fromEntries(
    Object.entries(snapshot).filter(([, value]) => value !== undefined)
  ) as unknown as TrackingContinuitySnapshot
}

export const trackingContinuity = new TrackingContinuity()

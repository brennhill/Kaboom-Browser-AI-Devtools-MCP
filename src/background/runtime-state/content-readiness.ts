/**
 * Purpose: Gates post-navigation content commands on a correlation-matched content-script acknowledgement.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import { debugLog, DebugCategory } from '../debug.js'
import { reportStateRecovery, resolveStateRecovery } from './state-recovery.js'
import { trackingContinuity } from './tracking-continuity.js'
import { getConnectionGeneration } from './connection-generation.js'

export interface ContentReadinessAcknowledgement {
  readonly ready: true
  readonly correlation_id: string
  readonly connection_generation: number
}

export type ContentReadinessResult =
  | {
      readonly ready: true
      readonly correlation_id: string
      readonly attempts: number
    }
  | {
      readonly ready: false
      readonly correlation_id: string
      readonly attempts: number
      readonly error: 'content_readiness_timeout' | 'readiness_superseded'
    }

interface ContentReadinessOptions {
  readonly probe: (
    tabId: number,
    correlationId: string,
    connectionGeneration: number
  ) => Promise<ContentReadinessAcknowledgement | undefined>
  readonly wait: (delayMs: number) => Promise<void>
  readonly get_generation?: () => number
  readonly delays_ms?: readonly number[]
  readonly onReady?: (tabId: number, correlationId: string, attempts: number) => void
  readonly onTimeout?: (tabId: number, correlationId: string, attempts: number) => void
  readonly onSuperseded?: (
    tabId: number,
    correlationId: string,
    expectedGeneration: number,
    currentGeneration: number
  ) => void
}

const DEFAULT_DELAYS_MS = [25, 75, 150] as const
const CONTENT_COMMANDS = new Set([
  'a11y',
  'dom',
  'dom_action',
  'draw_mode',
  'execute',
  'explore_page',
  'get_markdown',
  'get_readable',
  'highlight',
  'link_health',
  'page_info',
  'page_inventory',
  'page_structure',
  'page_summary',
  'state_capture',
  'state_save',
  'state_load',
  'upload'
])

export function requiresContentReadiness(queryType: string): boolean {
  return CONTENT_COMMANDS.has(queryType)
}

export class ContentReadinessBarrier {
  private readonly pending = new Map<number, { correlationId: string; connectionGeneration: number }>()
  private readonly probe: ContentReadinessOptions['probe']
  private readonly wait: ContentReadinessOptions['wait']
  private readonly delaysMs: readonly number[]
  private readonly onReady?: ContentReadinessOptions['onReady']
  private readonly onTimeout?: ContentReadinessOptions['onTimeout']
  private readonly onSuperseded?: ContentReadinessOptions['onSuperseded']
  private readonly getGeneration: () => number

  constructor(options: ContentReadinessOptions) {
    this.probe = options.probe
    this.wait = options.wait
    this.delaysMs = options.delays_ms ?? DEFAULT_DELAYS_MS
    this.onReady = options.onReady
    this.onTimeout = options.onTimeout
    this.onSuperseded = options.onSuperseded
    this.getGeneration = options.get_generation ?? getConnectionGeneration
  }

  begin(tabId: number, correlationId: string): void {
    this.pending.set(tabId, { correlationId, connectionGeneration: this.getGeneration() })
  }

  hasPending(tabId: number): boolean {
    return this.pending.has(tabId)
  }

  cancel(tabId: number, correlationId: string): void {
    if (this.pending.get(tabId)?.correlationId === correlationId) this.pending.delete(tabId)
  }

  async waitUntilReady(tabId: number): Promise<ContentReadinessResult> {
    const pending = this.pending.get(tabId)
    if (!pending) {
      return {
        ready: false,
        correlation_id: '',
        attempts: 0,
        error: 'readiness_superseded'
      }
    }
    const { correlationId, connectionGeneration } = pending

    for (let attempt = 1; attempt <= this.delaysMs.length + 1; attempt += 1) {
      const acknowledgement = await this.probe(tabId, correlationId, connectionGeneration)
      const currentGeneration = this.getGeneration()
      if (
        this.pending.get(tabId) !== pending ||
        currentGeneration !== connectionGeneration ||
        (acknowledgement !== undefined && acknowledgement.connection_generation !== connectionGeneration)
      ) {
        if (this.pending.get(tabId) === pending) this.pending.delete(tabId)
        this.onSuperseded?.(tabId, correlationId, connectionGeneration, currentGeneration)
        return {
          ready: false,
          correlation_id: correlationId,
          attempts: attempt,
          error: 'readiness_superseded'
        }
      }
      if (acknowledgement?.ready && acknowledgement.correlation_id === correlationId) {
        this.pending.delete(tabId)
        this.onReady?.(tabId, correlationId, attempt)
        return { ready: true, correlation_id: correlationId, attempts: attempt }
      }
      const delayMs = this.delaysMs[attempt - 1]
      if (delayMs !== undefined) await this.wait(delayMs)
    }

    const result = {
      ready: false,
      correlation_id: correlationId,
      attempts: this.delaysMs.length + 1,
      error: 'content_readiness_timeout'
    } as const
    this.onTimeout?.(tabId, correlationId, result.attempts)
    return result
  }
}

const failedReadinessTabs = new Set<number>()

export const contentReadiness = new ContentReadinessBarrier({
  probe: async (tabId, correlationId, connectionGeneration) => {
    try {
      return (await chrome.tabs.sendMessage(tabId, {
        type: 'tracking_readiness_probe',
        correlation_id: correlationId,
        connection_generation: connectionGeneration
      })) as ContentReadinessAcknowledgement | undefined
    } catch {
      // EXPECTED_ABSENCE: the content script is normally unavailable while Chrome
      // reinjects it after navigation; logging each bounded probe miss would
      // misrepresent expected recovery progress as an independent failure.
      return undefined
    }
  },
  wait: (delayMs) => new Promise((resolve) => setTimeout(resolve, delayMs)),
  onReady: (tabId, correlationId, attempts) => {
    trackingContinuity.confirm(tabId)
    if (failedReadinessTabs.delete(tabId)) resolveStateRecovery('content_readiness_state')
    debugLog(DebugCategory.CONNECTION, 'Content readiness acknowledged', {
      tab_id: tabId,
      correlation_id: correlationId,
      attempts
    })
  },
  onTimeout: (tabId, correlationId, attempts) => {
    failedReadinessTabs.add(tabId)
    trackingContinuity.fail(tabId, 'content_readiness_timeout')
    reportStateRecovery({
      name: 'content_readiness_state',
      detail: `Content readiness failed after ${attempts} correlated attempts.`,
      fix: 'Reload the tracked tab or reconnect the extension, then retry the command.'
    })
    debugLog(DebugCategory.CONNECTION, 'Content readiness transition failed', {
      tab_id: tabId,
      correlation_id: correlationId,
      attempts,
      error: 'content_readiness_timeout'
    })
  },
  onSuperseded: (tabId, correlationId, expectedGeneration, currentGeneration) => {
    reportStateRecovery({
      name: 'content_readiness_generation',
      detail: 'A content readiness acknowledgement arrived after its daemon connection was superseded.',
      fix: 'Retry the command after the extension reconnects.'
    })
    debugLog(DebugCategory.CONNECTION, 'Rejected stale connection generation', {
      tab_id: tabId,
      correlation_id: correlationId,
      bridge: 'content_readiness',
      received_generation: expectedGeneration,
      current_generation: currentGeneration
    })
  }
})

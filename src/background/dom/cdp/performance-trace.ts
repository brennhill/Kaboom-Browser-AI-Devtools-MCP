/**
 * Purpose: Capture a Chrome DevTools-compatible CPU flamechart trace and stream it to local daemon storage.
 * Docs: docs/features/feature/performance-trace/index.md
 */

import type {
  WirePerformanceTraceChunkRequest,
  WirePerformanceTraceResult,
  WirePerformanceTraceStartResponse
} from '../../../types/wire/wire-performance-trace.js'
import { buildDaemonJSONRequestInit } from '../../../lib/daemon-http.js'
import { errorMessage } from '../../../lib/error-utils.js'
import { getServerUrl } from '../../runtime-state/settings-state.js'

const CDP_VERSION = '1.3'
const MAX_CHUNK_JSON_BYTES = 3 * 1024 * 1024
const DEFAULT_COMPLETION_TIMEOUT_MS = 15_000
const TRACE_CATEGORIES = [
  '-*',
  'blink.console',
  'blink.user_timing',
  'loading',
  'devtools.timeline',
  'disabled-by-default-devtools.timeline',
  'disabled-by-default-devtools.timeline.frame',
  'disabled-by-default-devtools.timeline.stack',
  'disabled-by-default-v8.cpu_profiler',
  'disabled-by-default-v8.cpu_profiler.hires',
  'v8',
  'v8.execute',
  'navigation,rail'
].join(',')

interface Debuggee {
  tabId?: number
}

interface DebuggerAPI {
  attach(target: Debuggee, requiredVersion: string): Promise<void>
  detach(target: Debuggee): Promise<void>
  sendCommand(target: Debuggee, method: string, commandParams?: object): Promise<object | undefined>
  onEvent: { addListener(listener: (source: Debuggee, method: string, params?: object) => void): void }
  onDetach: { addListener(listener: (source: Debuggee, reason: string) => void): void }
}

interface ControllerDeps {
  debuggerApi: DebuggerAPI
  postJSON: (path: string, payload: unknown) => Promise<unknown>
  completionTimeoutMs?: number
}

interface ActiveTrace {
  traceId: string
  tabId: number
  sequence: number
  uploads: Promise<void>
  completion?: { resolve: () => void; reject: (error: Error) => void }
  failure?: Error
  abortRequested?: boolean
  abortPromise?: Promise<unknown>
  debuggerDetached?: boolean
  detachExpected?: boolean
}

export interface PerformanceTraceStarted {
  status: 'recording'
  trace_id: string
  tab_id: number
}

export interface PerformanceTraceFinished extends WirePerformanceTraceResult {
  import_with: string
}

export class PerformanceTraceController {
  private active: ActiveTrace | undefined
  private readonly completionTimeoutMs: number

  constructor(private readonly deps: ControllerDeps) {
    this.completionTimeoutMs = deps.completionTimeoutMs ?? DEFAULT_COMPLETION_TIMEOUT_MS
    deps.debuggerApi.onEvent.addListener((source, method, params) => this.onEvent(source, method, params))
    deps.debuggerApi.onDetach.addListener((source, reason) => this.onDetach(source, reason))
  }

  async start(tabId: number): Promise<PerformanceTraceStarted> {
    if (this.active) throw new Error(`performance trace already active for tab ${this.active.tabId}`)
    const opened = requireStartResponse(await this.deps.postJSON('/performance-trace/start', { tab_id: tabId }))
    const active: ActiveTrace = { traceId: opened.trace_id, tabId, sequence: 0, uploads: Promise.resolve() }
    this.active = active
    try {
      await this.deps.debuggerApi.attach({ tabId }, CDP_VERSION)
      await this.deps.debuggerApi.sendCommand({ tabId }, 'Tracing.start', {
        categories: TRACE_CATEGORIES,
        options: 'record-as-much-as-possible',
        transferMode: 'ReportEvents'
      })
      return { status: 'recording', trace_id: active.traceId, tab_id: tabId }
    } catch (error) {
      await this.abortActive(errorMessage(error, 'Chrome tracing failed to start'))
      throw error
    }
  }

  async stop(tabId: number): Promise<PerformanceTraceFinished> {
    const active = this.requireActive(tabId)
    if (active.failure) {
      const failure = active.failure
      await this.abortActive(failure.message)
      throw failure
    }

    const completion = new Promise<void>((resolve, reject) => {
      active.completion = { resolve, reject }
    })
    const timeout = globalThis.setTimeout(() => {
      active.completion?.reject(new Error('Chrome tracing did not complete before the bounded timeout'))
    }, this.completionTimeoutMs)

    try {
      await this.deps.debuggerApi.sendCommand({ tabId }, 'Tracing.end')
      await completion
      await active.uploads
      if (active.failure) throw active.failure
      active.detachExpected = true
      await this.deps.debuggerApi.detach({ tabId })
      active.debuggerDetached = true
      const result = requireTraceResult(
        await this.deps.postJSON('/performance-trace/finish', { trace_id: active.traceId })
      )
      this.active = undefined
      return {
        ...result,
        import_with: 'Chrome DevTools Performance panel or https://ui.perfetto.dev'
      }
    } catch (error) {
      await this.abortActive(errorMessage(error, 'Chrome tracing failed to stop'))
      throw error
    } finally {
      globalThis.clearTimeout(timeout)
    }
  }

  private requireActive(tabId: number): ActiveTrace {
    if (!this.active) throw new Error('no performance trace is active')
    if (this.active.tabId !== tabId) {
      throw new Error(`tracked tab changed during performance trace (started ${this.active.tabId}, got ${tabId})`)
    }
    return this.active
  }

  private onEvent(source: Debuggee, method: string, params?: object): void {
    const active = this.active
    if (!active || source.tabId !== active.tabId) return
    if (method === 'Tracing.tracingComplete') {
      if ((params as { dataLossOccurred?: unknown } | undefined)?.dataLossOccurred === true) {
        active.failure = new Error('Chrome lost trace data before the CPU flamechart completed')
      }
      active.completion?.resolve()
      return
    }
    if (method !== 'Tracing.dataCollected') return
    const events = (params as { value?: unknown[] } | undefined)?.value
    if (!Array.isArray(events) || events.length === 0) return

    try {
      for (const batch of boundedEventBatches(events)) {
        const request: WirePerformanceTraceChunkRequest = {
          trace_id: active.traceId,
          sequence: active.sequence++,
          events: batch
        }
        active.uploads = active.uploads.then(async () => {
          await this.deps.postJSON('/performance-trace/chunk', request)
        })
      }
      active.uploads = active.uploads.catch((error: unknown) => {
        active.failure = new Error(errorMessage(error, 'performance trace chunk upload failed'))
      })
    } catch (error) {
      active.failure = new Error(errorMessage(error, 'performance trace event could not be serialized'))
    }
  }

  private onDetach(source: Debuggee, reason: string): void {
    const active = this.active
    if (!active || source.tabId !== active.tabId) return
    if (active.detachExpected) {
      // EXPECTED_ABSENCE: Tracing.stop deliberately detaches after Chrome confirms
      // all events were delivered. Logging this normal onDetach callback would
      // falsely report a trace failure.
      active.debuggerDetached = true
      return
    }
    const failure = new Error(`Chrome debugger detached during performance trace: ${reason}`)
    active.failure = failure
    active.debuggerDetached = true
    active.completion?.reject(failure)
    active.abortRequested = true
    active.abortPromise = this.deps
      .postJSON('/performance-trace/abort', { trace_id: active.traceId })
      .catch((error: unknown) => {
        console.error('[Kaboom][performance_trace] Failed to abort detached trace artifact', {
          trace_id: active.traceId,
          error: errorMessage(error)
        })
      })
  }

  private async abortActive(reason: string): Promise<void> {
    const active = this.active
    if (!active) return
    this.active = undefined
    try {
      if (active.abortPromise) {
        await active.abortPromise
      } else if (!active.abortRequested) {
        await this.deps.postJSON('/performance-trace/abort', { trace_id: active.traceId })
      }
    } catch (error) {
      console.error('[Kaboom][performance_trace] Failed to remove partial trace artifact', {
        trace_id: active.traceId,
        reason,
        error: errorMessage(error)
      })
    }
    if (active.debuggerDetached) return
    try {
      await this.deps.debuggerApi.detach({ tabId: active.tabId })
    } catch (error) {
      console.error('[Kaboom][performance_trace] Failed to detach Chrome debugger after trace failure', {
        trace_id: active.traceId,
        reason,
        error: errorMessage(error)
      })
    }
  }
}

export function createPerformanceTraceController(deps: ControllerDeps): PerformanceTraceController {
  return new PerformanceTraceController(deps)
}

async function postLocalJSON(path: string, payload: unknown): Promise<unknown> {
  const response = await fetch(`${getServerUrl()}${path}`, buildDaemonJSONRequestInit(payload))
  if (!response.ok) {
    throw new Error(`performance trace daemon request ${path} failed: HTTP ${response.status} ${response.statusText}`)
  }
  return response.json() as Promise<unknown>
}

export function createDefaultPerformanceTraceController(): PerformanceTraceController {
  return createPerformanceTraceController({ debuggerApi: chrome.debugger, postJSON: postLocalJSON })
}

function boundedEventBatches(events: unknown[]): unknown[][] {
  const batches: unknown[][] = []
  let batch: unknown[] = []
  let bytes = 2
  for (const event of events) {
    const encoded = JSON.stringify(event)
    if (encoded === undefined) throw new Error('Chrome returned a non-JSON trace event')
    const eventBytes = new TextEncoder().encode(encoded).byteLength + (batch.length > 0 ? 1 : 0)
    if (eventBytes > MAX_CHUNK_JSON_BYTES)
      throw new Error('Chrome returned a trace event larger than the local upload bound')
    if (bytes + eventBytes > MAX_CHUNK_JSON_BYTES && batch.length > 0) {
      batches.push(batch)
      batch = []
      bytes = 2
    }
    batch.push(event)
    bytes += eventBytes
  }
  if (batch.length > 0) batches.push(batch)
  return batches
}

function requireStartResponse(value: unknown): WirePerformanceTraceStartResponse {
  if (!value || typeof value !== 'object' || typeof (value as { trace_id?: unknown }).trace_id !== 'string') {
    throw new Error('performance trace daemon returned an invalid start response')
  }
  return value as WirePerformanceTraceStartResponse
}

function requireTraceResult(value: unknown): WirePerformanceTraceResult {
  const candidate = value as Partial<WirePerformanceTraceResult> | null
  if (!candidate || typeof candidate.trace_id !== 'string' || typeof candidate.artifact_path !== 'string') {
    throw new Error('performance trace daemon returned an invalid artifact response')
  }
  return candidate as WirePerformanceTraceResult
}

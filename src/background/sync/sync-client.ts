/**
 * Purpose: Unified sync client that replaces multiple polling loops with a single /sync endpoint, handling settings, commands, and extension logs.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

/**
 * @fileoverview Unified Sync Client - Replaces multiple polling loops with single /sync endpoint.
 * Features: Simple exponential backoff, binary connection state, self-healing for MV3.
 */

import type { PendingQuery } from '../../types/runtime/queries.js'
import { errorMessage } from '../../lib/error-utils.js'
import { fetchWithTimeout } from '../../lib/timeout-utils.js'
import { buildDaemonJSONRequestInit } from '../../lib/daemon-http.js'
import { drainUIFeatures, restoreUIFeatures } from '../ui/ui-usage-tracker.js'
import { getServerInstallId, updateServerInstallId } from './install-identity.js'

// =============================================================================
// TYPES
// =============================================================================

/** Settings to send to server */
export interface SyncSettings {
  pilot_enabled: boolean
  tracking_enabled: boolean
  tracked_tab_id: number
  tracked_tab_url: string
  tracked_tab_title: string
  tab_status?: 'loading' | 'complete'
  tracked_tab_active?: boolean
  capture_logs: boolean
  capture_network: boolean
  capture_websocket: boolean
  capture_actions: boolean
  csp_restricted: boolean
  csp_level: string
}

/** Extension log entry */
export interface SyncExtensionLog {
  timestamp: string
  level: string
  message: string
  source: string
  category: string
  data?: unknown
}

/** Command result to send to server */
export interface SyncCommandResult {
  id: string
  correlation_id?: string
  status: 'complete' | 'error' | 'timeout' | 'cancelled'
  result?: unknown
  error?: string
}

/** Active command metadata sent on each sync heartbeat */
export interface SyncInProgress {
  id: string
  correlation_id?: string
  type?: string
  status?: 'running' | 'pending'
  progress_pct?: number
  started_at?: string
  updated_at?: string
}

/** Request sent to /sync */
interface SyncRequest {
  ext_session_id: string
  extension_version?: string
  settings?: SyncSettings
  extension_logs?: SyncExtensionLog[]
  last_command_ack?: string
  command_results?: SyncCommandResult[]
  in_progress?: SyncInProgress[]
  features_used?: Record<string, boolean>
}

/** Command from server */
export interface SyncCommand {
  id: string
  type: string
  params: unknown
  tab_id?: number
  correlation_id?: string
  trace_id?: string
}

/** Response from /sync */
interface SyncResponse {
  ack: boolean
  commands: SyncCommand[]
  next_poll_ms: number
  server_time: string
  server_version?: string
  install_id?: string
  capture_overrides?: Record<string, string>
}

/** Sync state */
export interface SyncState {
  connected: boolean
  lastSyncAt: number
  consecutiveFailures: number
  lastCommandAck: string | null
}

/** Callbacks for sync client */
export interface SyncClientCallbacks {
  onCommand: (command: SyncCommand) => Promise<void>
  onConnectionChange: (connected: boolean) => void
  onCaptureOverrides?: (overrides: Record<string, string>) => void
  onVersionMismatch?: (extensionVersion: string, serverVersion: string) => void
  commandTimeoutMs?: number
  uploadCommandTimeoutMs?: number
  getSettings: () => Promise<SyncSettings>
  getExtensionLogs: () => SyncExtensionLog[]
  acknowledgeExtensionLogs: (sentCount: number) => void
  debugLog?: (category: string, message: string, data?: unknown) => void
}

// =============================================================================
// CONSTANTS
// =============================================================================

const BASE_POLL_MS = 1000
const DEFAULT_COMMAND_TIMEOUT_MS = 65000
const ACKNOWLEDGED_COMMAND_HISTORY_LIMIT = 5

// =============================================================================
// SYNC CLIENT CLASS
// =============================================================================

export class SyncClient {
  private serverUrl: string
  private extSessionId: string
  private callbacks: SyncClientCallbacks
  private state: SyncState
  private intervalId: ReturnType<typeof setInterval> | null = null
  private running = false
  private syncing = false
  private flushRequested = false
  private pendingResults: SyncCommandResult[] = []
  private inProgressById = new Map<string, SyncInProgress>()
  private processedCommandSignatures: Set<string> = new Set()
  private extensionVersion: string

  constructor(serverUrl: string, extSessionId: string, callbacks: SyncClientCallbacks, extensionVersion = '') {
    this.serverUrl = serverUrl
    this.extSessionId = extSessionId
    this.callbacks = callbacks
    this.extensionVersion = extensionVersion
    this.state = {
      connected: false,
      lastSyncAt: 0,
      consecutiveFailures: 0,
      lastCommandAck: null
    }
  }

  /** Get current sync state */
  getState(): SyncState {
    return { ...this.state }
  }

  /** Check if connected */
  isConnected(): boolean {
    return this.state.connected
  }

  /** Start the sync loop */
  start(): void {
    if (this.running) return
    this.running = true
    this.log('Starting sync client')
    this.scheduleNextSync(0) // Sync immediately
  }

  /** Stop the sync loop */
  stop(): void {
    this.running = false
    if (this.intervalId) {
      clearTimeout(this.intervalId)
      this.intervalId = null
    }
    this.log('Stopped sync client')
  }

  /** Queue a command result to send on next sync, then flush immediately */
  queueCommandResult(result: SyncCommandResult): void {
    this.clearInProgressById(result.id)
    this.pendingResults.push(result)
    // Cap queue size to prevent memory leak if server is unreachable
    const MAX_PENDING_RESULTS = 200
    if (this.pendingResults.length > MAX_PENDING_RESULTS) {
      this.pendingResults = this.pendingResults.slice(-MAX_PENDING_RESULTS)
    }
    this.flush()
  }

  /** Trigger an immediate sync to deliver queued results with minimal latency */
  flush(): void {
    if (!this.running) return
    if (this.syncing) {
      // Sync in progress — schedule another immediately after it finishes
      this.flushRequested = true
      return
    }
    if (this.intervalId) {
      clearTimeout(this.intervalId)
    }
    this.scheduleNextSync(0)
  }

  /** Reset connection state (e.g., when user toggles pilot/tracking) */
  resetConnection(): void {
    this.state.consecutiveFailures = 0
    this.log('Connection state reset')
    // Trigger immediate sync if running
    if (this.running && this.intervalId) {
      clearTimeout(this.intervalId)
      this.scheduleNextSync(0)
    }
  }

  /** Update server URL */
  setServerUrl(url: string): void {
    if (url === this.serverUrl) return
    this.serverUrl = url
    this.resetConnection()
  }

  /** Optional progress updates for long-running commands */
  updateCommandProgress(commandId: string, progressPct?: number, status: 'running' | 'pending' = 'running'): void {
    const current = this.inProgressById.get(commandId)
    if (!current) return
    const next: SyncInProgress = {
      ...current,
      status,
      updated_at: new Date().toISOString()
    }
    if (typeof progressPct === 'number' && Number.isFinite(progressPct)) {
      next.progress_pct = clampPercent(progressPct)
    }
    this.inProgressById.set(commandId, next)
  }

  // =============================================================================
  // PRIVATE METHODS
  // =============================================================================

  private scheduleNextSync(delayMs: number): void {
    if (!this.running) return
    this.intervalId = setTimeout(() => this.doSync(), delayMs)
  }

  private async doSync(): Promise<void> {
    if (!this.running) return
    this.syncing = true
    this.flushRequested = false
    let features: Record<string, boolean> | undefined

    try {
      // Build request
      const settings = await this.callbacks.getSettings()
      const logs = this.callbacks.getExtensionLogs()

      const request: SyncRequest = {
        ext_session_id: this.extSessionId,
        extension_version: this.extensionVersion || undefined,
        settings,
        in_progress: this.getInProgressSnapshot()
      }

      // Include logs if any
      if (logs.length > 0) {
        request.extension_logs = logs
      }

      // Include pending command results
      const resultsSentCount = this.pendingResults.length
      if (resultsSentCount > 0) {
        request.command_results = this.pendingResults.slice(0, resultsSentCount)
      }

      // Include last command ack
      if (this.state.lastCommandAck) {
        request.last_command_ack = this.state.lastCommandAck
      }

      // Include UI-originated feature usage (screenshot button, draw mode, etc.)
      features = drainUIFeatures()
      if (features) {
        request.features_used = features
      }

      // Make request with timeout to prevent hanging forever (8s: server holds up to 5s + margin)
      const response = await fetchWithTimeout(
        `${this.serverUrl}/sync`,
        buildDaemonJSONRequestInit(request, {
          extensionVersion: this.extensionVersion || undefined
        }),
        8000
      )

      if (!response.ok) {
        throw new Error(
          `Sync request failed: HTTP ${response.status} ${response.statusText} from ${this.serverUrl}/sync`
        )
      }

      const data: SyncResponse = await response.json()

      // Log sync cycle summary
      this.log('Sync OK', {
        commands: data.commands?.length || 0,
        resultsSent: request.command_results?.length || 0,
        logsSent: request.extension_logs?.length || 0,
        nextPollMs: data.next_poll_ms
      })

      // Success - update state
      this.onSuccess()

      // Store server install ID for use as the single analytics identifier.
      if (data.install_id && data.install_id !== getServerInstallId()) {
        updateServerInstallId(data.install_id)
      }

      // Check for version mismatch (compare major.minor only, ignore patch)
      if (data.server_version && this.extensionVersion && this.callbacks.onVersionMismatch) {
        const serverMajorMinor = data.server_version.split('.').slice(0, 2).join('.')
        const extensionMajorMinor = this.extensionVersion.split('.').slice(0, 2).join('.')
        if (serverMajorMinor !== extensionMajorMinor) {
          this.callbacks.onVersionMismatch(this.extensionVersion, data.server_version)
        }
      }

      // Clear sent logs and results
      if (logs.length > 0) {
        this.callbacks.acknowledgeExtensionLogs(logs.length)
      }
      if (resultsSentCount > 0) {
        this.pendingResults = this.pendingResults.slice(resultsSentCount)
      }

      // Dispatch commands without blocking the heartbeat loop.
      // Command completion is returned asynchronously via queueCommandResult().
      if (data.commands && data.commands.length > 0) {
        this.log('Received commands', { count: data.commands.length, ids: data.commands.map((c) => c.id) })
        for (const command of data.commands) {
          const signature = this.getCommandSignature(command)

          if (command.id && this.processedCommandSignatures.has(signature)) {
            this.log('Skipping already processed command', {
              id: command.id,
              correlation_id: command.correlation_id,
              type: command.type
            })
            continue
          }

          // Dedup on RECEIPT — prevents re-execution if server re-sends before ack
          if (command.id) {
            this.processedCommandSignatures.add(signature)
            // This is a short duplicate-delivery guard, not a command archive.
            // Active work is protected separately by inProgressById; retaining a
            // large completed set can suppress valid IDs after a daemon restart.
            if (this.processedCommandSignatures.size > ACKNOWLEDGED_COMMAND_HISTORY_LIMIT) {
              const oldest = this.processedCommandSignatures.values().next().value
              if (oldest !== undefined) {
                this.processedCommandSignatures.delete(oldest)
              }
            }
          }

          this.log('Dispatching command', {
            id: command.id,
            type: command.type,
            correlation_id: command.correlation_id
          })
          void this.dispatchCommand(command)
        }
      }

      // Handle capture overrides
      if (data.capture_overrides && this.callbacks.onCaptureOverrides) {
        this.callbacks.onCaptureOverrides(data.capture_overrides)
      }

      // Schedule next sync — flush immediately if results were queued during this sync
      this.syncing = false
      if (this.flushRequested) {
        this.flushRequested = false
        this.scheduleNextSync(0)
      } else {
        const nextPollMs = data.next_poll_ms || BASE_POLL_MS
        this.scheduleNextSync(nextPollMs)
      }
    } catch (err) {
      this.syncing = false
      this.flushRequested = false
      this.onFailure()

      // Re-merge drained UI features so they aren't lost on failed sync.
      if (features) {
        restoreUIFeatures(features)
      }

      const retryMs = this.retryDelayMs()
      this.log('Sync failed, retrying', {
        correlation_id: this.extSessionId,
        error: errorMessage(err),
        retry_ms: retryMs
      })
      this.scheduleNextSync(retryMs)
    }
  }

  private onSuccess(): void {
    const wasDisconnected = !this.state.connected
    this.state.connected = true
    this.state.lastSyncAt = Date.now()
    this.state.consecutiveFailures = 0

    if (wasDisconnected) {
      this.log('Connected')
      this.callbacks.onConnectionChange(true)
    }
  }

  private onFailure(): void {
    this.state.consecutiveFailures++

    // Log at logarithmic intervals (10, 100, 1000) — not every failure.
    if (
      this.state.consecutiveFailures === 10 ||
      this.state.consecutiveFailures === 100 ||
      this.state.consecutiveFailures === 1000
    ) {
      this.log('Sync failure threshold reached', { failures: this.state.consecutiveFailures })
    }

    // Require 2+ consecutive failures before marking disconnected
    // to prevent a single transient timeout from flipping connection state
    if (this.state.consecutiveFailures >= 2 && this.state.connected) {
      this.state.connected = false
      this.log('Disconnected')
      this.callbacks.onConnectionChange(false)
    }
  }

  private retryDelayMs(): number {
    const exponent = Math.min(Math.max(this.state.consecutiveFailures - 1, 0), 5)
    const capped = Math.min(BASE_POLL_MS * 2 ** exponent, 30000)
    const jitter = 0.75 + Math.random() * 0.5
    return Math.round(capped * jitter)
  }

  private log(message: string, data?: unknown): void {
    if (this.callbacks.debugLog) {
      this.callbacks.debugLog('sync', message, data)
    } else {
      console.log(`[SyncClient] ${message}`, data || '') // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal sync state, not user-controlled
    }
  }

  private getCommandSignature(command: SyncCommand): string {
    // Include correlation_id and type so command ID reuse after daemon restart
    // does not suppress new commands with the same queue ID.
    const id = command.id || ''
    const correlationID = command.correlation_id || ''
    const type = command.type || ''
    return `${id}::${correlationID}::${type}`
  }

  private commandTimeoutFor(command: SyncCommand): number {
    if (command.type === 'upload' && typeof this.callbacks.uploadCommandTimeoutMs === 'number') {
      return Math.max(1, this.callbacks.uploadCommandTimeoutMs)
    }
    if (typeof this.callbacks.commandTimeoutMs === 'number') {
      return Math.max(1, this.callbacks.commandTimeoutMs)
    }
    return DEFAULT_COMMAND_TIMEOUT_MS
  }

  private async dispatchCommand(command: SyncCommand): Promise<void> {
    this.markInProgress(command)
    const timeoutMs = this.commandTimeoutFor(command)
    let timeoutHandle: ReturnType<typeof setTimeout> | null = null
    try {
      await Promise.race([
        Promise.resolve(this.callbacks.onCommand(command)),
        new Promise<never>((_, reject) => {
          timeoutHandle = setTimeout(
            () =>
              reject(
                new Error(
                  `Command ${command.id || '(unknown)'} (${command.type || 'unknown'}) timed out after ${timeoutMs}ms`
                )
              ),
            timeoutMs
          )
        })
      ])
      this.log('Command completed OK', { id: command.id })
    } catch (err) {
      const message = errorMessage(err, 'Command execution failed')
      this.log('Command execution FAILED', { id: command.id, error: message })
      this.queueCommandResult({
        id: command.id,
        status: 'error',
        error: message
      })
    } finally {
      if (timeoutHandle) {
        clearTimeout(timeoutHandle)
      }
      this.clearInProgressById(command.id)
      // Ack after dispatch completes (success or failure) — not on bare receipt
      if (command.id) {
        this.state.lastCommandAck = command.id
      }
    }
  }

  private markInProgress(command: SyncCommand): void {
    const now = new Date().toISOString()
    const current = this.inProgressById.get(command.id)
    this.inProgressById.set(command.id, {
      id: command.id,
      correlation_id: command.correlation_id,
      type: command.type,
      status: current?.status || 'running',
      progress_pct: current?.progress_pct,
      started_at: current?.started_at || now,
      updated_at: now
    })
  }

  private clearInProgressById(id?: string): void {
    if (!id) return
    this.inProgressById.delete(id)
  }

  private getInProgressSnapshot(): SyncInProgress[] {
    if (this.inProgressById.size === 0) {
      return []
    }
    return Array.from(this.inProgressById.values()).map((entry) => ({
      ...entry,
      updated_at: entry.updated_at || new Date().toISOString()
    }))
  }
}

function clampPercent(value: number): number {
  if (value < 0) return 0
  if (value > 100) return 100
  return Math.round(value * 100) / 100
}

// =============================================================================
// FACTORY FUNCTION
// =============================================================================

/**
 * Create a sync client instance
 */
export function createSyncClient(
  serverUrl: string,
  extSessionId: string,
  callbacks: SyncClientCallbacks,
  extensionVersion = ''
): SyncClient {
  return new SyncClient(serverUrl, extSessionId, callbacks, extensionVersion)
}

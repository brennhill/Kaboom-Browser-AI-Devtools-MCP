/**
 * Purpose: Manages sync client instance lifecycle (start/stop/reset) and wires dependencies to avoid circular imports with index.ts.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

// sync-manager.ts — Sync client lifecycle management.
// Owns the sync client instance and provides start/stop/reset operations.
// Dependencies are injected to avoid circular imports with index.ts.

import type { PendingQuery } from '../../types/runtime/queries.js'
import type { ConnectionStatus } from '../../types/runtime/state.js'
import type { ExtensionLogQueueEntry } from '../runtime-state/log-queue.js'
import { createSyncClient, type SyncClient, type SyncCommand, type SyncSettings } from './sync-client.js'
import { getLastCSPStatus } from '../runtime-state/csp-state.js'
import { DebugCategory } from '../debug.js'
import { updateBadge } from './server.js'
import { isQueryProcessing, addProcessingQuery, removeProcessingQuery } from '../caches/snapshots.js'
import { getTrackedTabInfo } from '../ui/tracked-tab-state.js'
import { handlePendingQuery as handlePendingQueryImpl } from '../pending-queries.js'
import { errorMessage } from '../../lib/error-utils.js'

// =============================================================================
// TYPES
// =============================================================================

type DebugLogFn = (category: string, message: string, data?: unknown) => void

/** Mutable connection status (same shape as index.ts) */
export type SyncConnectionStatusRef = Pick<
  ConnectionStatus,
  | 'connected'
  | 'extensionConnected'
  | 'extensionError'
  | 'error'
  | 'entries'
  | 'maxEntries'
  | 'errorCount'
  | 'logFile'
  | 'logFileSize'
  | 'serverVersion'
  | 'extensionVersion'
  | 'versionMismatch'
  | 'securityMode'
  | 'productionParity'
  | 'insecureRewritesApplied'
>

/** Dependencies injected by index.ts to avoid circular imports */
export interface SyncManagerDeps {
  getServerUrl: () => string
  getExtSessionId: () => string
  getConnectionStatus: () => SyncConnectionStatusRef
  setConnectionStatus: (patch: Partial<SyncConnectionStatusRef>) => void
  getAiControlled: () => boolean
  getAiWebPilotEnabledCache: () => boolean
  getExtensionLogQueue: () => ExtensionLogQueueEntry[]
  acknowledgeExtensionLogQueue: (sentCount: number) => void
  applyCaptureOverrides: (overrides: Record<string, string>) => void
  debugLog: DebugLogFn
}

// =============================================================================
// MODULE STATE
// =============================================================================

/** Sync client instance (initialized lazily) */
let syncClient: SyncClient | null = null

// =============================================================================
// HELPERS
// =============================================================================

/**
 * Get extension version safely
 */
function getExtensionVersion(): string {
  if (typeof chrome !== 'undefined' && chrome.runtime?.getManifest) {
    return chrome.runtime.getManifest().version
  }
  return ''
}

// =============================================================================
// SYNC CLIENT LIFECYCLE
// =============================================================================

/**
 * Start the sync client (unified /sync endpoint).
 * Safe to call multiple times — will no-op if already running.
 */
// #lizard forgives
export function startSyncClient(deps: SyncManagerDeps): void {
  if (syncClient) {
    syncClient.setServerUrl(deps.getServerUrl())
    return
  }

  syncClient = createSyncClient(
    deps.getServerUrl(),
    deps.getExtSessionId(),
    {
      // Handle commands from server
      // #lizard forgives
      onCommand: async (command: SyncCommand) => {
        deps.debugLog(DebugCategory.CONNECTION, 'Processing sync command', { type: command.type, id: command.id })
        if (isQueryProcessing(command.id)) {
          deps.debugLog(DebugCategory.CONNECTION, 'Skipping already processing command', { id: command.id })
          return
        }
        addProcessingQuery(command.id)
        try {
          await handlePendingQueryImpl(command as unknown as PendingQuery, syncClient!)
        } catch (err) {
          deps.debugLog(DebugCategory.CONNECTION, 'Error processing sync command', {
            type: command.type,
            error: errorMessage(err)
          })
        } finally {
          removeProcessingQuery(command.id)
        }
      },

      // Handle connection state changes
      onConnectionChange: (connected: boolean) => {
        // Sync heartbeat health is independent of daemon HTTP reachability.
        // A transient sync failure must not make a healthy daemon appear offline.
        deps.setConnectionStatus({ extensionConnected: connected })
        updateBadge(deps.getConnectionStatus())
        deps.debugLog(DebugCategory.CONNECTION, connected ? 'Sync connected' : 'Sync disconnected')

        // Notify popup
        if (typeof chrome !== 'undefined' && chrome.runtime) {
          chrome.runtime
            .sendMessage({
              type: 'status_update',
              status: { ...deps.getConnectionStatus(), aiControlled: deps.getAiControlled() }
            })
            .catch(() => {
              // EXPECTED_ABSENCE: no popup recipient is normal while the popup is
              // closed; logging it would misleadingly report a sync failure.
            })
        }
      },

      // Handle capture overrides from server
      onCaptureOverrides: (overrides: Record<string, string>) => {
        deps.applyCaptureOverrides(overrides)
        if (typeof chrome !== 'undefined' && chrome.runtime) {
          chrome.runtime
            .sendMessage({
              type: 'status_update',
              status: { ...deps.getConnectionStatus(), aiControlled: deps.getAiControlled() }
            })
            .catch(() => {
              // EXPECTED_ABSENCE: no popup recipient is normal while the popup is
              // closed; logging it would misleadingly report a sync failure.
            })
        }
      },

      // Handle version mismatch between extension and server
      onVersionMismatch: (extensionVersion: string, serverVersion: string) => {
        deps.debugLog(DebugCategory.CONNECTION, 'Version mismatch detected', { extensionVersion, serverVersion })
        // Update connection status with version info
        deps.setConnectionStatus({
          serverVersion,
          extensionVersion,
          versionMismatch: extensionVersion !== serverVersion
        })
        // Notify popup about version mismatch
        if (typeof chrome !== 'undefined' && chrome.runtime) {
          chrome.runtime
            .sendMessage({
              type: 'version_mismatch',
              extensionVersion,
              serverVersion
            })
            .catch(() => {
              // EXPECTED_ABSENCE: no popup recipient is normal while the popup is
              // closed; logging it would misleadingly report a version-check failure.
            })
        }
      },

      // Get current settings to send to server
      getSettings: async (): Promise<SyncSettings> => {
        const trackingInfo = await getTrackedTabInfo()
        const csp = getLastCSPStatus()
        return {
          pilot_enabled: deps.getAiWebPilotEnabledCache(),
          tracking_enabled: !!trackingInfo.trackedTabId,
          tracked_tab_id: trackingInfo.trackedTabId || 0,
          tracked_tab_url: trackingInfo.trackedTabUrl || '',
          tracked_tab_title: trackingInfo.trackedTabTitle || '',
          tab_status: trackingInfo.tabStatus || undefined,
          tracked_tab_active: trackingInfo.trackedTabActive ?? undefined,
          capture_logs: true,
          capture_network: true,
          capture_websocket: true,
          capture_actions: true,
          csp_restricted: csp.csp_restricted,
          csp_level: csp.csp_level
        }
      },

      // Get pending extension logs
      getExtensionLogs: () => {
        return deps.getExtensionLogQueue().map((log) => ({
          timestamp: log.timestamp,
          level: log.level,
          message: log.message,
          source: log.source,
          category: log.category,
          data: log.data
        }))
      },

      // Clear extension logs after sending
      acknowledgeExtensionLogs: (sentCount: number) => {
        deps.acknowledgeExtensionLogQueue(sentCount)
      },

      // Debug logging
      debugLog: (category: string, message: string, data?: unknown) => {
        deps.debugLog(DebugCategory.CONNECTION, `[Sync] ${message}`, data)
      }
    },
    getExtensionVersion()
  )

  syncClient.start()
  deps.debugLog(DebugCategory.CONNECTION, 'Sync client started')
}

/**
 * Stop the sync client
 */
export function stopSyncClient(debugLog: DebugLogFn): void {
  if (syncClient) {
    syncClient.stop()
    syncClient = null
    debugLog(DebugCategory.CONNECTION, 'Sync client stopped')
  }
}

/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export function resetSyncClientConnection(debugLog: DebugLogFn): void {
  if (syncClient) {
    syncClient.resetConnection()
    debugLog(DebugCategory.CONNECTION, 'Sync client connection reset')
  }
}

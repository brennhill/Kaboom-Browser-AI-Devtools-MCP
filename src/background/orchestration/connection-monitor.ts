/**
 * Purpose: Owns daemon health polling, status projection, and sync-client lifecycle wiring.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import { errorMessage, isNoReceiverError } from '../../lib/error-utils.js'
import { DebugCategory, debugLog } from '../debug.js'
import { isAiWebPilotEnabled } from '../runtime-state/pilot-state.js'
import { acknowledgeExtensionLogQueue, getExtensionLogQueueSnapshot } from '../runtime-state/log-queue.js'
import {
  applyConnectionOverrides,
  getConnectionStatus,
  isConnectionCheckRunning,
  setConnectionCheckRunning,
  setConnectionStatus,
  type MutableConnectionStatus
} from '../runtime-state/connection-state.js'
import { EXTENSION_SESSION_ID } from '../runtime-state/startup-state.js'
import { applySettingOverrides, getServerUrl, isAiControlled } from '../runtime-state/settings-state.js'
import { checkServerHealth, updateBadge } from '../sync/server.js'
import { resetSyncClientConnection as resetSyncClientConnectionImpl, startSyncClient } from '../sync/sync-manager.js'
import { updateVersionFromHealth } from '../sync/version-check.js'

const syncManagerDeps = {
  getServerUrl,
  getExtSessionId: () => EXTENSION_SESSION_ID,
  getConnectionStatus,
  setConnectionStatus: (patch: Partial<MutableConnectionStatus>) => setConnectionStatus(patch),
  getAiControlled: isAiControlled,
  getAiWebPilotEnabledCache: isAiWebPilotEnabled,
  getExtensionLogQueue: getExtensionLogQueueSnapshot,
  acknowledgeExtensionLogQueue,
  applyCaptureOverrides: (overrides: Record<string, string>) => {
    applySettingOverrides(overrides)
    applyConnectionOverrides(overrides)
  },
  debugLog
}

function applyHealthLogs(health: {
  logs?: { logFile?: string; logFileSize?: number; entries?: number; maxEntries?: number }
}): void {
  if (!health.logs) return
  const status = getConnectionStatus()
  setConnectionStatus({
    logFile: health.logs.logFile || status.logFile,
    logFileSize: health.logs.logFileSize,
    entries: health.logs.entries ?? status.entries,
    maxEntries: health.logs.maxEntries ?? status.maxEntries
  })
}

function applyVersionState(health: { connected: boolean; version?: string; available_version?: string }): void {
  if (health.connected) {
    try {
      updateVersionFromHealth(health, debugLog)
    } catch (error) {
      debugLog(DebugCategory.CONNECTION, 'Failed to update version info', { error: errorMessage(error) })
    }
  }
  if (!health.connected || !health.version || typeof chrome === 'undefined') return
  const extensionVersion = chrome.runtime.getManifest().version
  setConnectionStatus({
    serverVersion: health.version,
    extensionVersion,
    versionMismatch: health.version.split('.')[0] !== extensionVersion.split('.')[0]
  })
}

function broadcastStatusUpdate(): void {
  if (typeof chrome === 'undefined' || !chrome.runtime) return
  chrome.runtime
    .sendMessage({ type: 'status_update', status: { ...getConnectionStatus(), aiControlled: isAiControlled() } })
    .catch((error) => {
      if (!isNoReceiverError(error)) console.error(`${KABOOM_LOG_PREFIX} Error sending status update:`, error)
    })
}

export async function checkConnectionAndUpdate(): Promise<void> {
  if (isConnectionCheckRunning()) {
    debugLog(DebugCategory.CONNECTION, 'Skipping connection check - already running')
    return
  }
  setConnectionCheckRunning(true)
  try {
    const health = await checkServerHealth(getServerUrl())
    const wasConnected = getConnectionStatus().connected
    applyVersionState(health)
    setConnectionStatus({ ...health, connected: health.connected })
    applyHealthLogs(health)
    updateBadge(getConnectionStatus())
    if (wasConnected !== health.connected) {
      debugLog(DebugCategory.CONNECTION, health.connected ? 'Connected to server' : 'Disconnected from server', {
        entries: getConnectionStatus().entries,
        error: health.error || null,
        serverVersion: health.version || null
      })
    }
    startSyncClient(syncManagerDeps)
    broadcastStatusUpdate()
  } finally {
    setConnectionCheckRunning(false)
  }
}

export function resetSyncClientConnection(): void {
  resetSyncClientConnectionImpl(debugLog)
}

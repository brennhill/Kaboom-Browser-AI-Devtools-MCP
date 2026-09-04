/**
 * Purpose: Own connection lifecycle state for the background worker.
 * Why: Connection checks and health updates change together independently of user settings.
 */
import type { ConnectionStatus } from '../../types/runtime/state.js'
import { errorMessage } from '../../lib/error-utils.js'
import { pushExtensionLog } from './log-queue.js'

export type MutableConnectionStatus = { -readonly [Key in keyof ConnectionStatus]: ConnectionStatus[Key] }

/** Called on each edge of the daemon sync connection, never on repeats of the same value. */
export type ExtensionConnectionListener = (connected: boolean) => void

const defaultStatus: MutableConnectionStatus = {
  connected: false,
  entries: 0,
  maxEntries: 1000,
  errorCount: 0,
  logFile: '',
  securityMode: 'normal',
  productionParity: true,
  insecureRewritesApplied: []
}
let connectionStatus = { ...defaultStatus }
let connectionCheckRunning = false
const extensionConnectionListeners = new Set<ExtensionConnectionListener>()

export function getConnectionStatus(): Readonly<MutableConnectionStatus> {
  return Object.freeze({ ...connectionStatus })
}

/**
 * Observe the daemon sync connection edge. This is the authoritative live signal
 * that an MCP client session started or ended — owners of session-scoped browser
 * state (the driven tab group) subscribe here instead of mirroring the state in
 * storage, which goes stale when the worker dies without flushing (rule 18).
 * Returns an unsubscribe function.
 */
export function subscribeExtensionConnection(listener: ExtensionConnectionListener): () => void {
  extensionConnectionListeners.add(listener)
  return () => {
    extensionConnectionListeners.delete(listener)
  }
}

function notifyExtensionConnection(connected: boolean): void {
  for (const listener of extensionConnectionListeners) {
    try {
      listener(connected)
    } catch (error) {
      pushExtensionLog({
        timestamp: new Date().toISOString(),
        level: 'warn',
        message: 'Extension connection listener failed',
        source: 'background',
        category: 'lifecycle',
        data: { connected, error: errorMessage(error) }
      })
    }
  }
}

export function setConnectionStatus(patch: Partial<MutableConnectionStatus>): void {
  const previouslyConnected = connectionStatus.extensionConnected
  connectionStatus = { ...connectionStatus, ...patch }
  if (patch.extensionConnected === undefined || patch.extensionConnected === previouslyConnected) return
  notifyExtensionConnection(patch.extensionConnected)
}
export function isConnectionCheckRunning(): boolean {
  return connectionCheckRunning
}
export function setConnectionCheckRunning(running: boolean): void {
  connectionCheckRunning = running
}

export function applyConnectionOverrides(overrides: Readonly<Record<string, string>>): void {
  const rewrites = (overrides.insecure_rewrites_applied || '')
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
  setConnectionStatus({
    securityMode: overrides.security_mode === 'insecure_proxy' ? 'insecure_proxy' : 'normal',
    productionParity: overrides.production_parity !== 'false',
    insecureRewritesApplied: rewrites
  })
}

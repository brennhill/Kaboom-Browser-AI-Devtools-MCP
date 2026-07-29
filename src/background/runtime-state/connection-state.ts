/**
 * Purpose: Own connection lifecycle state for the background worker.
 * Why: Connection checks and health updates change together independently of user settings.
 */
import type { ConnectionStatus } from '../../types/index.js'

export type MutableConnectionStatus = { -readonly [Key in keyof ConnectionStatus]: ConnectionStatus[Key] }

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

export function getConnectionStatus(): Readonly<MutableConnectionStatus> {
  return Object.freeze({ ...connectionStatus })
}
export function setConnectionStatus(patch: Partial<MutableConnectionStatus>): void {
  connectionStatus = { ...connectionStatus, ...patch }
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

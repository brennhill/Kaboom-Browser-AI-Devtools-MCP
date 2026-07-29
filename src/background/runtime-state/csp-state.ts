/**
 * Purpose: Own the latest CSP probe result without coupling sync and execution modules.
 */
import type { CSPProbeResult } from '../exec/query-execution.js'

let lastCSPStatus: CSPProbeResult = { csp_restricted: false, csp_level: 'none' }

export function getLastCSPStatus(): CSPProbeResult {
  return lastCSPStatus
}

export function setLastCSPStatus(status: CSPProbeResult): void {
  lastCSPStatus = status
}

/**
 * Purpose: Own extension persisted-state recovery diagnostics sent to the daemon.
 * Why: Keeps raw state out of logs while making fallbacks visible in System Doctor.
 */
import type { StateRecoveryDiagnostic } from '../../types/runtime-messages.js'
import { pushExtensionLog } from './log-queue.js'

export function reportStateRecovery(diagnostic: StateRecoveryDiagnostic): void {
  pushExtensionLog({
    timestamp: new Date().toISOString(),
    level: 'warn',
    message: 'Persisted extension state recovered',
    source: 'background',
    category: 'state_recovery',
    data: { ...diagnostic }
  })
}

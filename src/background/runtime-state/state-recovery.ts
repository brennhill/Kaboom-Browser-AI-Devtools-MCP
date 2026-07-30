/**
 * Purpose: Own extension persisted-state recovery diagnostics sent to the daemon.
 * Why: Keeps raw state out of logs while making fallbacks visible in System Doctor.
 */
import type { StateRecoveryDiagnostic, StateRecoveryLifecycle } from '../../types/runtime-messages.js'
import { pushExtensionLog } from './log-queue.js'

export function reportStateRecovery(
  diagnostic: StateRecoveryDiagnostic,
  lifecycle: StateRecoveryLifecycle = 'active'
): void {
  pushExtensionLog({
    timestamp: new Date().toISOString(),
    level: lifecycle === 'active' ? 'warn' : 'info',
    message:
      lifecycle === 'active'
        ? 'Persisted extension state recovered with fallback'
        : 'Persisted extension state verified',
    source: 'background',
    category: 'state_recovery',
    data: { ...diagnostic, lifecycle }
  })
}

export function resolveStateRecovery(name: string): void {
  reportStateRecovery({ name, detail: '', fix: '' }, 'recovered')
}

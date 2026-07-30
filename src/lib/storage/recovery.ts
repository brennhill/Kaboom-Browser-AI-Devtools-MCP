/**
 * Purpose: Report redacted persisted-state recovery from non-background contexts.
 */
import type { ReportStateRecoveryMessage, StateRecoveryDiagnostic } from '../../types/runtime-messages.js'
import { KABOOM_LOG_PREFIX } from '../brand.js'

export function reportStateRecovery(diagnostic: StateRecoveryDiagnostic): void {
  console.warn(`${KABOOM_LOG_PREFIX} persisted state recovered: ${diagnostic.name}`)
  if (typeof chrome === 'undefined' || !chrome.runtime?.sendMessage) return
  const message: ReportStateRecoveryMessage = { type: 'report_state_recovery', diagnostic }
  try {
    const pending = chrome.runtime.sendMessage(message)
    if (pending && typeof pending.catch === 'function') {
      void pending.catch(() => undefined)
    }
  } catch {
    // Console warning remains available when the background worker is unavailable.
  }
}

/**
 * Purpose: Report redacted persisted-state recovery from non-background contexts.
 */
import type {
  ReportStateRecoveryMessage,
  StateRecoveryDiagnostic,
  StateRecoveryLifecycle
} from '../../types/runtime-messages.js'
import { KABOOM_LOG_PREFIX } from '../brand.js'

export function reportStateRecovery(diagnostic: StateRecoveryDiagnostic): void {
  console.warn(`${KABOOM_LOG_PREFIX} persisted state fallback active: ${diagnostic.name}`)
  sendTransition('active', diagnostic)
}

export function resolveStateRecovery(name: string): void {
  sendTransition('recovered', { name, detail: '', fix: '' })
}

function sendTransition(
  lifecycle: StateRecoveryLifecycle,
  diagnostic: StateRecoveryDiagnostic
): void {
  if (typeof chrome === 'undefined' || !chrome.runtime?.sendMessage) return
  const message: ReportStateRecoveryMessage = {
    type: 'report_state_recovery',
    lifecycle,
    diagnostic
  }
  try {
    const pending = chrome.runtime.sendMessage(message)
    if (pending && typeof pending.catch === 'function') {
      void pending.catch(() => {
        console.warn(
          `${KABOOM_LOG_PREFIX} state recovery transition was not delivered: ${lifecycle}/${diagnostic.name}`
        )
      })
    }
  } catch {
    // Console warning remains available when the background worker is unavailable.
  }
}

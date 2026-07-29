/**
 * Purpose: Own service-worker session identity and initialization readiness.
 */
export const EXTENSION_SESSION_ID = `ext_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
let resolveInitialization: (() => void) | null = null
export const initReady: Promise<void> = new Promise((resolve) => {
  resolveInitialization = resolve
})
export function markInitComplete(): void {
  resolveInitialization?.()
  resolveInitialization = null
}

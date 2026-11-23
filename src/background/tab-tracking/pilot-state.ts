/**
 * Purpose: Own AI Web Pilot cache initialization and pending initialization callback.
 */
let enabled = true
let initialized = false
let pendingInit: (() => void) | null = null

export function isAiWebPilotEnabled(): boolean {
  return enabled
}
export function setAiWebPilotEnabledCache(value: boolean): void {
  enabled = value
}
export function isAiWebPilotCacheInitialized(): boolean {
  return initialized
}
export function setAiWebPilotCacheInitialized(value: boolean): void {
  initialized = value
}
export function getPilotInitCallback(): (() => void) | null {
  return pendingInit
}
export function setPilotInitCallback(callback: (() => void) | null): void {
  pendingInit = callback
}
export function resetPilotCacheForTesting(value = false): void {
  enabled = value
}

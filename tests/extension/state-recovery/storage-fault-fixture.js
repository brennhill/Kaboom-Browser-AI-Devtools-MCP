// storage-fault-fixture.js — Deterministic, redacted extension-state failures for tests only.
export const STORAGE_FAULT_KINDS = Object.freeze([
  'read',
  'write',
  'sync',
  'rename',
  'directory_sync',
  'quota',
  'corruption',
  'partial_write',
  'cancellation',
  'restart'
])

export function createStorageFaultScenario(kind, privateSentinel) {
  void privateSentinel
  if (!STORAGE_FAULT_KINDS.includes(kind)) throw new Error('persisted_state_fault:unknown')
  return {
    kind,
    error: new Error(`persisted_state_fault:${kind}`),
    cancelled: kind === 'cancellation',
    storedValue(valid) {
      if (kind === 'corruption') return '{"schema_version":'
      if (kind === 'partial_write') {
        const entries = Object.entries(valid)
        return Object.fromEntries(entries.slice(0, Math.max(1, Math.floor(entries.length / 2))))
      }
      return structuredClone(valid)
    },
    nextGeneration(current) {
      return kind === 'restart' && current < Number.MAX_SAFE_INTEGER ? current + 1 : current
    }
  }
}

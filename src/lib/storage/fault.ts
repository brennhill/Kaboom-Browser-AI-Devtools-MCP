/**
 * Purpose: Classify persisted-state failures consistently across extension owners.
 * Why: Doctor diagnostics need stable, redacted failure transitions without test hooks in production.
 */

export const STORAGE_FAULT_KINDS = [
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
] as const

export type StorageFaultKind = (typeof STORAGE_FAULT_KINDS)[number]

export function classifyStorageFailure(error: unknown, operation: 'read' | 'write'): StorageFaultKind {
  if (error instanceof DOMException && error.name === 'AbortError') return 'cancellation'
  const name = error instanceof Error ? error.name.toLowerCase() : ''
  const message = error instanceof Error ? error.message.toLowerCase() : ''
  if (name.includes('quota') || message.includes('quota')) return 'quota'
  return operation
}

export function storageFaultDetail(kind: StorageFaultKind, consequence: string): string {
  return `Extension state ${kind} failure; ${consequence}`
}

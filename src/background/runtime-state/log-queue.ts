/**
 * Purpose: Own the bounded, redacted, session-persisted extension diagnostic queue.
 * Why: Preserve local failure evidence across MV3 worker restarts and daemon reconnects.
 */
import { StorageKey } from '../../lib/constants.js'
import { setSession } from '../../lib/storage/session.js'
import { readSessionState } from '../../lib/storage/validated.js'
import { classifyStorageFailure, type StorageFaultKind } from '../../lib/storage/fault.js'

export interface ExtensionLogQueueEntry {
  timestamp: string
  level: string
  message: string
  source: string
  category: string
  data?: unknown
}

export interface ExtensionLogQueueMetrics {
  entries: number
  droppedCount: number
  saturated: boolean
  persistenceFailures: number
}

export interface ExtensionLogQueueStorage {
  read: () => Promise<unknown>
  write: (value: PersistedExtensionLogQueue) => Promise<void>
}

export interface ExtensionLogQueueRecovery {
  status: 'empty' | 'restored' | 'recovered'
  restoredEntries: number
}

interface PersistedExtensionLogQueue {
  version: 1
  dropped_count: number
  entries: ExtensionLogQueueEntry[]
  lifecycle_events?: string[]
}

const MAX_ENTRIES = 200
const MAX_STRING_LENGTH = 1000
const MAX_REDACTION_DEPTH = 6
const REDACTED = '[REDACTED]'
const SATURATION_MESSAGE = 'Diagnostic queue saturated'
const INVALID_PERSISTED_QUEUE = Symbol('invalid_persisted_extension_diagnostics')
const SENSITIVE_KEY =
  /authorization|cookie|password|passwd|secret|token|api[_-]?key|request[_-]?body|response[_-]?body/i
const BEARER_CREDENTIAL = /(authorization\s*:\s*)?bearer\s+[^\s,;]+/gi
const SENSITIVE_PARAMETER = /([?&]|\b)(token|secret|password|passwd|api[_-]?key)=([^\s&#,;]+)/gi

const defaultStorage: ExtensionLogQueueStorage = {
  read: async () => {
    let invalid = false
    const value = await readSessionState<PersistedExtensionLogQueue | undefined>({
      key: StorageKey.EXTENSION_DIAGNOSTIC_LOGS,
      fallback: undefined,
      validate: isPersistedQueue,
      diagnostic: {
        name: 'extension_diagnostic_queue',
        detail: 'Saved extension diagnostics were invalid or unreadable; a clean local buffer is active.',
        fix: 'No action is required unless this warning repeats; include System Doctor output in a bug report.'
      },
      report: () => {
        invalid = true
      },
      resolve: () => {
        // EXPECTED_ABSENCE: a valid recovery is normal during initialization;
        // logging here would misleadingly duplicate the canonical transition
        // emitted after empty and restored state can be distinguished.
      }
    })
    return invalid ? INVALID_PERSISTED_QUEUE : value
  },
  write: (value) => setSession(StorageKey.EXTENSION_DIAGNOSTIC_LOGS, value)
}

let entries: ExtensionLogQueueEntry[] = []
let droppedCount = 0
let persistenceFailures = 0
let lifecycleEvents: string[] = []
let storage = defaultStorage
let persistenceTail: Promise<void> = Promise.resolve()

function sanitizeURL(value: string): string {
  try {
    const parsed = new URL(value)
    return `${parsed.origin}${parsed.pathname}`.slice(0, MAX_STRING_LENGTH)
  } catch {
    return value.slice(0, MAX_STRING_LENGTH)
  }
}

function sanitizeString(value: string): string {
  return value
    .replace(BEARER_CREDENTIAL, (_match, prefix: string | undefined) => `${prefix ?? ''}Bearer ${REDACTED}`)
    .replace(SENSITIVE_PARAMETER, (_match, separator: string, name: string) => `${separator}${name}=${REDACTED}`)
    .slice(0, MAX_STRING_LENGTH)
}

function redactValue(value: unknown, key = '', depth = 0): unknown {
  if (SENSITIVE_KEY.test(key)) return REDACTED
  if (depth >= MAX_REDACTION_DEPTH) return '[TRUNCATED]'
  if (typeof value === 'string') {
    return /url$/i.test(key) ? sanitizeURL(value) : sanitizeString(value)
  }
  if (value === null || typeof value === 'number' || typeof value === 'boolean') return value
  if (Array.isArray(value)) return value.slice(0, 50).map((item) => redactValue(item, '', depth + 1))
  if (typeof value !== 'object') return String(value).slice(0, MAX_STRING_LENGTH)
  const redacted: Record<string, unknown> = {}
  for (const [childKey, childValue] of Object.entries(value).slice(0, 50)) {
    redacted[childKey] = redactValue(childValue, childKey, depth + 1)
  }
  return redacted
}

function sanitizeEntry(entry: ExtensionLogQueueEntry): ExtensionLogQueueEntry {
  return {
    timestamp: entry.timestamp.slice(0, 64),
    level: entry.level.slice(0, 16),
    message: sanitizeString(entry.message),
    source: entry.source.slice(0, 64),
    category: entry.category.slice(0, 64),
    ...(entry.data === undefined ? {} : { data: redactValue(entry.data) })
  }
}

function isEntry(value: unknown): value is ExtensionLogQueueEntry {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Partial<ExtensionLogQueueEntry>
  return (
    typeof candidate.timestamp === 'string' &&
    typeof candidate.level === 'string' &&
    typeof candidate.message === 'string' &&
    typeof candidate.source === 'string' &&
    typeof candidate.category === 'string'
  )
}

function isPersistedQueue(value: unknown): value is PersistedExtensionLogQueue {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Partial<PersistedExtensionLogQueue>
  return (
    candidate.version === 1 &&
    Number.isInteger(candidate.dropped_count) &&
    (candidate.dropped_count ?? -1) >= 0 &&
    Array.isArray(candidate.entries) &&
    candidate.entries.every(isEntry) &&
    (candidate.lifecycle_events === undefined ||
      (Array.isArray(candidate.lifecycle_events) &&
        candidate.lifecycle_events.every((event) => typeof event === 'string')))
  )
}

function snapshotForPersistence(): PersistedExtensionLogQueue {
  return {
    version: 1,
    dropped_count: droppedCount,
    entries: entries.map((entry) => sanitizeEntry(entry)),
    lifecycle_events: [...lifecycleEvents]
  }
}

function recordStorageFailure(error: unknown, operation: 'read' | 'write'): void {
  persistenceFailures++
  const faultKind = classifyStorageFailure(error, operation)
  entries.push({
    timestamp: new Date().toISOString(),
    level: 'warn',
    message: operation === 'write' ? 'Diagnostic queue persistence failed' : 'Diagnostic queue read failed',
    source: 'background',
    category: 'diagnostic_queue',
    data: {
      reason: operation === 'write' ? 'session_storage_write_failed' : 'session_storage_read_failed',
      fault_kind: faultKind,
      occurrences: persistenceFailures
    }
  })
  enforceLimit()
}

function recordQueueRecovery(faultKind: StorageFaultKind): void {
  entries.push({
    timestamp: new Date().toISOString(),
    level: 'warn',
    message: 'Diagnostic queue state recovered',
    source: 'background',
    category: 'diagnostic_queue',
    data: { fault_kind: faultKind }
  })
  enforceLimit()
}

function persist(): void {
  const snapshot = snapshotForPersistence()
  const targetStorage = storage
  persistenceTail = persistenceTail
    .then(() => targetStorage.write(snapshot))
    .catch((error) => {
      // This failure cannot be persisted by definition. Retain a redacted in-memory
      // diagnostic so the next successful daemon sync still exposes it to Doctor.
      recordStorageFailure(error, 'write')
    })
}

function enforceLimit(): void {
  if (entries.length <= MAX_ENTRIES) return
  const withoutMarker = entries.filter((entry) => entry.message !== SATURATION_MESSAGE)
  const retained = withoutMarker.slice(-(MAX_ENTRIES - 1))
  droppedCount += withoutMarker.length - retained.length
  entries = [
    ...retained,
    {
      timestamp: new Date().toISOString(),
      level: 'warn',
      message: SATURATION_MESSAGE,
      source: 'background',
      category: 'diagnostic_queue',
      data: { dropped_count: droppedCount, capacity: MAX_ENTRIES }
    }
  ]
}

export async function initializeExtensionLogQueue(
  storageOverride: ExtensionLogQueueStorage = defaultStorage
): Promise<ExtensionLogQueueRecovery> {
  storage = storageOverride
  const startupEntries = entries.map((entry) => sanitizeEntry(entry))
  try {
    const persisted = await storage.read()
    if (persisted === undefined || persisted === null) {
      entries = startupEntries
      persist()
      return { status: 'empty', restoredEntries: 0 }
    }
    if (persisted === INVALID_PERSISTED_QUEUE || !isPersistedQueue(persisted)) {
      entries = startupEntries
      droppedCount = 0
      lifecycleEvents = []
      recordQueueRecovery('corruption')
      persist()
      return { status: 'recovered', restoredEntries: 0 }
    }
    droppedCount = persisted.dropped_count
    lifecycleEvents = (persisted.lifecycle_events ?? []).slice(-5)
    entries = [...persisted.entries.map(sanitizeEntry), ...startupEntries]
    enforceLimit()
    persist()
    return { status: 'restored', restoredEntries: persisted.entries.length }
  } catch (error) {
    entries = startupEntries
    droppedCount = 0
    lifecycleEvents = []
    recordStorageFailure(error, 'read')
    persist()
    return { status: 'recovered', restoredEntries: 0 }
  }
}

export function getExtensionLogQueueSnapshot(): ExtensionLogQueueEntry[] {
  return entries.map((entry) => sanitizeEntry(entry))
}

export function acknowledgeExtensionLogQueue(sentCount: number): void {
  if (sentCount > 0) {
    entries.splice(0, Math.min(sentCount, entries.length))
    persist()
  }
}

export function pushExtensionLog(entry: ExtensionLogQueueEntry): void {
  entries.push(sanitizeEntry(entry))
  enforceLimit()
  persist()
}

export function recordExtensionDiagnosticLifecycle(
  event: string,
  correlationId: string,
  data: Readonly<Record<string, unknown>> = {}
): void {
  lifecycleEvents = [...lifecycleEvents, event].slice(-5)
  pushExtensionLog({
    timestamp: new Date().toISOString(),
    level: event.includes('failed') || event.includes('suspend') ? 'warn' : 'debug',
    message: `Extension ${event.replaceAll('_', ' ')}`,
    source: 'background',
    category: 'diagnostic_lifecycle',
    data: { ...data, event, correlation_id: correlationId, lifecycle_sequence: [...lifecycleEvents] }
  })
}

export function getExtensionLogQueueMetrics(): ExtensionLogQueueMetrics {
  return {
    entries: entries.length,
    droppedCount,
    saturated: droppedCount > 0,
    persistenceFailures
  }
}

export async function flushExtensionLogPersistenceForTesting(): Promise<void> {
  await persistenceTail
}

export function clearExtensionLogsForTesting(): void {
  entries = []
  droppedCount = 0
  persistenceFailures = 0
  lifecycleEvents = []
  storage = defaultStorage
  persistenceTail = Promise.resolve()
}

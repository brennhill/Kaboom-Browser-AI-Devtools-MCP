/**
 * Purpose: Persists private environment snapshots locally across service-worker restarts.
 * Why: Recovery handles must survive extension suspension without exposing captured values to the daemon.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */

import type { EnvironmentSnapshot } from './browser-state-driver.js'

const STORAGE_KEY = 'environment_transaction_snapshots_v1'
const DOCUMENT_VERSION = 1

interface SnapshotRecord {
  readonly id: string
  readonly created_at: number
  readonly snapshot: EnvironmentSnapshot
}

interface SnapshotDocument {
  readonly version: 1
  readonly records: readonly SnapshotRecord[]
}

export interface EnvironmentSnapshotStore {
  readonly save: (snapshot: EnvironmentSnapshot) => Promise<string>
  readonly get: (id: string) => Promise<EnvironmentSnapshot | undefined>
  readonly delete: (id: string) => Promise<void>
}

export interface SnapshotStorageArea {
  readonly get: (key: string) => Promise<Record<string, unknown>>
  readonly set: (items: Record<string, unknown>) => Promise<void>
  readonly remove: (key: string) => Promise<void>
}

interface PersistentStoreDeps {
  readonly storage: SnapshotStorageArea
  readonly limit: number
  readonly now: () => number
  readonly newID: () => string
  readonly onNotice: (notice: string) => void
}

export function createPersistentEnvironmentSnapshotStore(deps: PersistentStoreDeps): EnvironmentSnapshotStore {
  const limit = Math.max(1, deps.limit)
  return {
    async save(snapshot) {
      const document = await readDocument(deps)
      const records = [...document.records]
      if (records.length >= limit) records.splice(oldestRecordIndex(records), 1)
      const id = deps.newID()
      records.push({ id, created_at: deps.now(), snapshot })
      await writeDocument(deps, { version: DOCUMENT_VERSION, records })
      return id
    },
    async get(id) {
      const document = await readDocument(deps)
      return document.records.find((record) => record.id === id)?.snapshot
    },
    async delete(id) {
      const document = await readDocument(deps)
      const records = document.records.filter((record) => record.id !== id)
      if (records.length === document.records.length) return
      await writeDocument(deps, { version: DOCUMENT_VERSION, records })
    }
  }
}

async function readDocument(deps: PersistentStoreDeps): Promise<SnapshotDocument> {
  let stored: Record<string, unknown>
  try {
    stored = await deps.storage.get(STORAGE_KEY)
  } catch {
    deps.onNotice('environment_snapshot_store_read_failed')
    throw new Error('environment_snapshot_store_read_failed')
  }
  const candidate = stored[STORAGE_KEY]
  if (candidate === undefined) return { version: DOCUMENT_VERSION, records: [] }
  if (isSnapshotDocument(candidate)) return candidate
  deps.onNotice('environment_snapshot_store_corrupt')
  try {
    await deps.storage.remove(STORAGE_KEY)
  } catch {
    deps.onNotice('environment_snapshot_store_recovery_failed')
    throw new Error('environment_snapshot_store_recovery_failed')
  }
  return { version: DOCUMENT_VERSION, records: [] }
}

async function writeDocument(deps: PersistentStoreDeps, document: SnapshotDocument): Promise<void> {
  try {
    await deps.storage.set({ [STORAGE_KEY]: document })
  } catch {
    deps.onNotice('environment_snapshot_store_write_failed')
    throw new Error('environment_snapshot_store_write_failed')
  }
}

function isSnapshotDocument(value: unknown): value is SnapshotDocument {
  if (!isRecord(value) || value.version !== DOCUMENT_VERSION || !Array.isArray(value.records)) return false
  return value.records.every(
    (record) =>
      isRecord(record) &&
      typeof record.id === 'string' &&
      record.id.length > 0 &&
      typeof record.created_at === 'number' &&
      Number.isFinite(record.created_at) &&
      isEnvironmentSnapshot(record.snapshot)
  )
}

function isEnvironmentSnapshot(value: unknown): value is EnvironmentSnapshot {
  return (
    isRecord(value) &&
    typeof value.tab_url === 'string' &&
    typeof value.window_id === 'number' &&
    isRecord(value.page_state) &&
    Array.isArray(value.cookies) &&
    isRestorePlan(value.restore_plan)
  )
}

function isRestorePlan(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.mutated_url === 'string' &&
    typeof value.setup_timeout_ms === 'number' &&
    Array.isArray(value.cookie_names) &&
    value.cookie_names.every((name) => typeof name === 'string') &&
    typeof value.page_state_touched === 'boolean' &&
    typeof value.navigation_changed === 'boolean'
  )
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function oldestRecordIndex(records: readonly SnapshotRecord[]): number {
  let oldest = 0
  for (let index = 1; index < records.length; index += 1) {
    const candidate = records[index]
    const current = records[oldest]
    if (!candidate || !current) continue
    if (candidate.created_at < current.created_at || (candidate.created_at === current.created_at && candidate.id < current.id)) {
      oldest = index
    }
  }
  return oldest
}

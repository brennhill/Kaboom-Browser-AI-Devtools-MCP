/**
 * Purpose: Persist named browser-state snapshots for interact state actions.
 */
import type { BrowserStateSnapshot } from '../types/runtime/state.js'
import { getLocal, setLocal } from '../lib/storage/local.js'

const SNAPSHOT_KEY = 'kaboom_state_snapshots'
interface StoredStateSnapshot extends BrowserStateSnapshot {
  name: string
  size_bytes: number
}
type StateSnapshotStorage = Record<string, StoredStateSnapshot>

export async function saveStateSnapshot(name: string, state: BrowserStateSnapshot) {
  const snapshots = ((await getLocal(SNAPSHOT_KEY)) as StateSnapshotStorage | undefined) || {}
  const sizeBytes = JSON.stringify(state).length
  snapshots[name] = { ...state, name, size_bytes: sizeBytes }
  await setLocal(SNAPSHOT_KEY, snapshots)
  return { success: true, snapshot_name: name, size_bytes: sizeBytes }
}
export async function loadStateSnapshot(name: string): Promise<StoredStateSnapshot | null> {
  const snapshots = ((await getLocal(SNAPSHOT_KEY)) as StateSnapshotStorage | undefined) || {}
  return snapshots[name] || null
}
export async function listStateSnapshots() {
  const snapshots = ((await getLocal(SNAPSHOT_KEY)) as StateSnapshotStorage | undefined) || {}
  return Object.values(snapshots).map(({ name, url, timestamp, size_bytes }) => ({ name, url, timestamp, size_bytes }))
}
export async function deleteStateSnapshot(name: string) {
  const snapshots = ((await getLocal(SNAPSHOT_KEY)) as StateSnapshotStorage | undefined) || {}
  delete snapshots[name]
  await setLocal(SNAPSHOT_KEY, snapshots)
  return { success: true, deleted: name }
}

/**
 * Purpose: Registers private extension commands for environment transaction snapshot, apply, and restore.
 * Why: Keeps sensitive snapshots extension-owned while exposing only opaque IDs to the daemon coordinator.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */

import type { WireQAFixture } from '../../types/wire/wire-qa-fixture.js'
import { registerCommand } from '../commands/registry.js'
import { createChromeEnvironmentStateDriver } from './chrome-state-adapter.js'
import type { EnvironmentStateDriver, EnvironmentSnapshot } from './browser-state-driver.js'

export interface EnvironmentSnapshotStore {
  readonly save: (snapshot: EnvironmentSnapshot) => string
  readonly get: (id: string) => EnvironmentSnapshot | undefined
  readonly delete: (id: string) => void
}

interface EnvironmentTransactionParams {
  readonly fixture?: WireQAFixture
  readonly snapshot_id?: string
}

export function createEnvironmentEnvironmentSnapshotStore(newID: () => string): EnvironmentSnapshotStore {
  const snapshots = new Map<string, EnvironmentSnapshot>()
  return {
    save(snapshot) {
      const id = newID()
      snapshots.set(id, snapshot)
      return id
    },
    get: (id) => snapshots.get(id),
    delete: (id) => snapshots.delete(id)
  }
}

export function registerEnvironmentTransactionCommands(driver: EnvironmentStateDriver, snapshots: EnvironmentSnapshotStore): void {
  registerCommand('environment_transaction_snapshot', async (ctx) => {
    const fixture = requireFixture(ctx.params)
    ctx.sendResult(await snapshotEnvironment(driver, snapshots, ctx.tabId, fixture))
  })

  registerCommand('environment_transaction_apply', async (ctx) => {
    const fixture = requireFixture(ctx.params)
    ctx.sendResult(await applyEnvironment(driver, ctx.tabId, fixture))
  })

  registerCommand('environment_transaction_restore', async (ctx) => {
    const params = ctx.params as EnvironmentTransactionParams
    const fixture = requireFixture(params)
    const snapshotID = requireSnapshotID(params)
    ctx.sendResult(await restoreEnvironment(driver, snapshots, ctx.tabId, fixture, snapshotID))
  })
}

export async function snapshotEnvironment(
  driver: EnvironmentStateDriver,
  snapshots: EnvironmentSnapshotStore,
  tabId: number,
  fixture: WireQAFixture
): Promise<{ readonly success: true; readonly snapshot_id: string }> {
  try {
    const snapshot = await driver.snapshot(tabId, fixture)
    return { success: true, snapshot_id: snapshots.save(snapshot) }
  } catch {
    throw new Error('fixture_snapshot_failed')
  }
}

export async function applyEnvironment(
  driver: EnvironmentStateDriver,
  tabId: number,
  fixture: WireQAFixture
): Promise<{ readonly success: true; readonly mutations: Awaited<ReturnType<EnvironmentStateDriver['apply']>> }> {
  try {
    return { success: true, mutations: await driver.apply(tabId, fixture) }
  } catch {
    throw new Error('fixture_apply_failed')
  }
}

export async function restoreEnvironment(
  driver: EnvironmentStateDriver,
  snapshots: EnvironmentSnapshotStore,
  tabId: number,
  fixture: WireQAFixture,
  snapshotID: string
): Promise<{ readonly success: true; readonly restored: true }> {
  const snapshot = snapshots.get(snapshotID)
  if (!snapshot) throw new Error('fixture_snapshot_not_found')
  try {
    await driver.restore(tabId, fixture, snapshot)
  } catch {
    throw new Error('fixture_restore_failed')
  }
  snapshots.delete(snapshotID)
  return { success: true, restored: true }
}

function requireFixture(params: object): WireQAFixture {
  const fixture = (params as EnvironmentTransactionParams).fixture
  if (!fixture || fixture.version !== 1) throw new Error('invalid_qa_fixture')
  return fixture
}

function requireSnapshotID(params: EnvironmentTransactionParams): string {
  if (!params.snapshot_id) throw new Error('fixture_snapshot_id_required')
  return params.snapshot_id
}

const driver = createChromeEnvironmentStateDriver()
const snapshots = createEnvironmentEnvironmentSnapshotStore(() => crypto.randomUUID())
registerEnvironmentTransactionCommands(driver, snapshots)

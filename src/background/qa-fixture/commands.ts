/**
 * Purpose: Registers private extension commands for QA fixture snapshot, apply, and restore.
 * Why: Keeps sensitive snapshots extension-owned while exposing only opaque IDs to the daemon coordinator.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */

import type { WireQAFixture } from '../../types/wire/wire-qa-fixture.js'
import { registerCommand } from '../commands/registry.js'
import { createChromeBrowserStateDriver } from './chrome-state-adapter.js'
import type { BrowserStateDriver, FixtureSnapshot } from './browser-state-driver.js'

export interface SnapshotStore {
  readonly save: (snapshot: FixtureSnapshot) => string
  readonly get: (id: string) => FixtureSnapshot | undefined
  readonly delete: (id: string) => void
}

interface FixtureCommandParams {
  readonly fixture?: WireQAFixture
  readonly snapshot_id?: string
}

export function createFixtureSnapshotStore(newID: () => string): SnapshotStore {
  const snapshots = new Map<string, FixtureSnapshot>()
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

export function registerFixtureCommands(driver: BrowserStateDriver, snapshots: SnapshotStore): void {
  registerCommand('qa_fixture_snapshot', async (ctx) => {
    const fixture = requireFixture(ctx.params)
    ctx.sendResult(await snapshotFixture(driver, snapshots, ctx.tabId, fixture))
  })

  registerCommand('qa_fixture_apply', async (ctx) => {
    const fixture = requireFixture(ctx.params)
    ctx.sendResult(await applyFixture(driver, ctx.tabId, fixture))
  })

  registerCommand('qa_fixture_restore', async (ctx) => {
    const params = ctx.params as FixtureCommandParams
    const fixture = requireFixture(params)
    const snapshotID = requireSnapshotID(params)
    ctx.sendResult(await restoreFixture(driver, snapshots, ctx.tabId, fixture, snapshotID))
  })
}

export async function snapshotFixture(
  driver: BrowserStateDriver,
  snapshots: SnapshotStore,
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

export async function applyFixture(
  driver: BrowserStateDriver,
  tabId: number,
  fixture: WireQAFixture
): Promise<{ readonly success: true; readonly mutations: Awaited<ReturnType<BrowserStateDriver['apply']>> }> {
  try {
    return { success: true, mutations: await driver.apply(tabId, fixture) }
  } catch {
    throw new Error('fixture_apply_failed')
  }
}

export async function restoreFixture(
  driver: BrowserStateDriver,
  snapshots: SnapshotStore,
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
  const fixture = (params as FixtureCommandParams).fixture
  if (!fixture || fixture.version !== 1) throw new Error('invalid_qa_fixture')
  return fixture
}

function requireSnapshotID(params: FixtureCommandParams): string {
  if (!params.snapshot_id) throw new Error('fixture_snapshot_id_required')
  return params.snapshot_id
}

const driver = createChromeBrowserStateDriver()
const snapshots = createFixtureSnapshotStore(() => crypto.randomUUID())
registerFixtureCommands(driver, snapshots)

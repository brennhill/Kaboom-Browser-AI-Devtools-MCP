/**
 * Purpose: Registers private extension commands for environment transaction snapshot, apply, and restore.
 * Why: Keeps sensitive snapshots extension-owned while exposing only opaque IDs to the daemon coordinator.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import { registerCommand } from '../commands/registry.js';
export function registerEnvironmentTransactionCommands(driver, snapshots) {
    registerCommand('environment_transaction_snapshot', async (ctx) => {
        const fixture = requireFixture(ctx.params);
        ctx.sendResult(await snapshotEnvironment(driver, snapshots, ctx.tabId, fixture));
    });
    registerCommand('environment_transaction_apply', async (ctx) => {
        const fixture = requireFixture(ctx.params);
        ctx.sendResult(await applyEnvironment(driver, ctx.tabId, fixture));
    });
    registerCommand('environment_transaction_restore', async (ctx) => {
        const params = ctx.params;
        const fixture = requireFixture(params);
        const snapshotID = requireSnapshotID(params);
        ctx.sendResult(await restoreEnvironment(driver, snapshots, ctx.tabId, fixture, snapshotID));
    });
}
export async function snapshotEnvironment(driver, snapshots, tabId, fixture) {
    try {
        const snapshot = await driver.snapshot(tabId, fixture);
        return { success: true, snapshot_id: await snapshots.save(snapshot) };
    }
    catch {
        throw new Error('fixture_snapshot_failed');
    }
}
export async function applyEnvironment(driver, tabId, fixture) {
    try {
        return { success: true, mutations: await driver.apply(tabId, fixture) };
    }
    catch {
        throw new Error('fixture_apply_failed');
    }
}
export async function restoreEnvironment(driver, snapshots, tabId, fixture, snapshotID) {
    const snapshot = await snapshots.get(snapshotID);
    if (!snapshot)
        throw new Error('fixture_snapshot_not_found');
    try {
        await driver.restore(tabId, fixture, snapshot);
    }
    catch {
        throw new Error('fixture_restore_failed');
    }
    await snapshots.delete(snapshotID);
    return { success: true, restored: true };
}
function requireFixture(params) {
    const fixture = params.fixture;
    if (!fixture || fixture.version !== 1)
        throw new Error('invalid_qa_fixture');
    return fixture;
}
function requireSnapshotID(params) {
    if (!params.snapshot_id)
        throw new Error('fixture_snapshot_id_required');
    return params.snapshot_id;
}
//# sourceMappingURL=commands.js.map
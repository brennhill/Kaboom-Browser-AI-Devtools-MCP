/**
 * Purpose: Registers private extension commands for environment transaction snapshot, apply, and restore.
 * Why: Keeps sensitive snapshots extension-owned while exposing only opaque IDs to the daemon coordinator.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import type { WireQAFixture } from '../../types/wire/wire-qa-fixture.js';
import type { EnvironmentStateDriver, EnvironmentSnapshot } from './browser-state-driver.js';
export interface EnvironmentSnapshotStore {
    readonly save: (snapshot: EnvironmentSnapshot) => string;
    readonly get: (id: string) => EnvironmentSnapshot | undefined;
    readonly delete: (id: string) => void;
}
export declare function createEnvironmentEnvironmentSnapshotStore(newID: () => string): EnvironmentSnapshotStore;
export declare function registerEnvironmentTransactionCommands(driver: EnvironmentStateDriver, snapshots: EnvironmentSnapshotStore): void;
export declare function snapshotEnvironment(driver: EnvironmentStateDriver, snapshots: EnvironmentSnapshotStore, tabId: number, fixture: WireQAFixture): Promise<{
    readonly success: true;
    readonly snapshot_id: string;
}>;
export declare function applyEnvironment(driver: EnvironmentStateDriver, tabId: number, fixture: WireQAFixture): Promise<{
    readonly success: true;
    readonly mutations: Awaited<ReturnType<EnvironmentStateDriver['apply']>>;
}>;
export declare function restoreEnvironment(driver: EnvironmentStateDriver, snapshots: EnvironmentSnapshotStore, tabId: number, fixture: WireQAFixture, snapshotID: string): Promise<{
    readonly success: true;
    readonly restored: true;
}>;
//# sourceMappingURL=commands.d.ts.map
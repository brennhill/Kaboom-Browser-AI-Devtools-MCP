/**
 * Purpose: Registers private extension commands for QA fixture snapshot, apply, and restore.
 * Why: Keeps sensitive snapshots extension-owned while exposing only opaque IDs to the daemon coordinator.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import type { WireQAFixture } from '../../types/wire/wire-qa-fixture.js';
import type { BrowserStateDriver, FixtureSnapshot } from './browser-state-driver.js';
export interface SnapshotStore {
    readonly save: (snapshot: FixtureSnapshot) => string;
    readonly get: (id: string) => FixtureSnapshot | undefined;
    readonly delete: (id: string) => void;
}
export declare function createFixtureSnapshotStore(newID: () => string): SnapshotStore;
export declare function registerFixtureCommands(driver: BrowserStateDriver, snapshots: SnapshotStore): void;
export declare function snapshotFixture(driver: BrowserStateDriver, snapshots: SnapshotStore, tabId: number, fixture: WireQAFixture): Promise<{
    readonly success: true;
    readonly snapshot_id: string;
}>;
export declare function applyFixture(driver: BrowserStateDriver, tabId: number, fixture: WireQAFixture): Promise<{
    readonly success: true;
    readonly mutations: Awaited<ReturnType<BrowserStateDriver['apply']>>;
}>;
export declare function restoreFixture(driver: BrowserStateDriver, snapshots: SnapshotStore, tabId: number, fixture: WireQAFixture, snapshotID: string): Promise<{
    readonly success: true;
    readonly restored: true;
}>;
//# sourceMappingURL=commands.d.ts.map
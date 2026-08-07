/**
 * Purpose: Persists private environment snapshots locally across service-worker restarts.
 * Why: Recovery handles must survive extension suspension without exposing captured values to the daemon.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import type { EnvironmentSnapshot } from './browser-state-driver.js';
import { type StorageFaultKind } from '../../lib/storage/fault.js';
export type EnvironmentSnapshotLookup = {
    readonly status: 'active';
    readonly snapshot: EnvironmentSnapshot;
} | {
    readonly status: 'consumed';
} | {
    readonly status: 'missing';
};
export interface EnvironmentSnapshotStore {
    readonly save: (snapshot: EnvironmentSnapshot) => Promise<string>;
    readonly lookup: (id: string) => Promise<EnvironmentSnapshotLookup>;
    readonly consume: (id: string) => Promise<void>;
    readonly reconcile: (activeIDs: readonly string[]) => Promise<{
        readonly pruned: number;
        readonly retained: number;
    }>;
}
export interface SnapshotStorageArea {
    readonly get: (key: string) => Promise<Record<string, unknown>>;
    readonly set: (items: Record<string, unknown>) => Promise<void>;
    readonly remove: (key: string) => Promise<void>;
}
interface PersistentStoreDeps {
    readonly storage: SnapshotStorageArea;
    readonly limit: number;
    readonly now: () => number;
    readonly newID: () => string;
    readonly onNotice: (notice: SnapshotStoreNotice) => void;
}
export interface SnapshotStoreNotice {
    readonly code: string;
    readonly fault_kind: StorageFaultKind;
    readonly lifecycle: 'active';
}
export declare function createPersistentEnvironmentSnapshotStore(deps: PersistentStoreDeps): EnvironmentSnapshotStore;
export {};
//# sourceMappingURL=snapshot-store.d.ts.map
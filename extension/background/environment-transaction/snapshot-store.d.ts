/**
 * Purpose: Persists private environment snapshots locally across service-worker restarts.
 * Why: Recovery handles must survive extension suspension without exposing captured values to the daemon.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import type { EnvironmentSnapshot } from './browser-state-driver.js';
export interface EnvironmentSnapshotStore {
    readonly save: (snapshot: EnvironmentSnapshot) => Promise<string>;
    readonly get: (id: string) => Promise<EnvironmentSnapshot | undefined>;
    readonly delete: (id: string) => Promise<void>;
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
    readonly onNotice: (notice: string) => void;
}
export declare function createPersistentEnvironmentSnapshotStore(deps: PersistentStoreDeps): EnvironmentSnapshotStore;
export {};
//# sourceMappingURL=snapshot-store.d.ts.map
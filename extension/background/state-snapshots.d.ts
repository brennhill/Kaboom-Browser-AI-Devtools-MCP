/**
 * Purpose: Persist named browser-state snapshots for interact state actions.
 */
import type { BrowserStateSnapshot } from '../types/index.js';
interface StoredStateSnapshot extends BrowserStateSnapshot {
    name: string;
    size_bytes: number;
}
export declare function saveStateSnapshot(name: string, state: BrowserStateSnapshot): Promise<{
    success: boolean;
    snapshot_name: string;
    size_bytes: number;
}>;
export declare function loadStateSnapshot(name: string): Promise<StoredStateSnapshot | null>;
export declare function listStateSnapshots(): Promise<{
    name: string;
    url: string;
    timestamp: number;
    size_bytes: number;
}[]>;
export declare function deleteStateSnapshot(name: string): Promise<{
    success: boolean;
    deleted: string;
}>;
export {};
//# sourceMappingURL=state-snapshots.d.ts.map
/**
 * Purpose: CRUD operations for saving, loading, listing, and deleting browser state snapshots in chrome.storage.local.
 * Docs: docs/features/feature/state-time-travel/index.md
 */
/**
 * @fileoverview State Snapshot Storage
 * Provides CRUD operations for saving/loading/listing/deleting browser state
 * snapshots in chrome.storage.local. Used by the interact tool's state_*
 * actions (save_state, load_state, list_states, delete_state).
 */
import type { BrowserStateSnapshot } from '../types/index.js';
interface StoredStateSnapshot extends BrowserStateSnapshot {
    name: string;
    size_bytes: number;
}
/**
 * Save a state snapshot to chrome.storage.local
 */
export declare function saveStateSnapshot(name: string, state: BrowserStateSnapshot): Promise<{
    success: boolean;
    snapshot_name: string;
    size_bytes: number;
}>;
/**
 * Load a state snapshot from chrome.storage.local
 */
export declare function loadStateSnapshot(name: string): Promise<StoredStateSnapshot | null>;
/**
 * List all state snapshots with metadata
 */
export declare function listStateSnapshots(): Promise<Array<{
    name: string;
    url: string;
    timestamp: number;
    size_bytes: number;
}>>;
/**
 * Delete a state snapshot from chrome.storage.local
 */
export declare function deleteStateSnapshot(name: string): Promise<{
    success: boolean;
    deleted: string;
}>;
export {};
//# sourceMappingURL=state-snapshots.d.ts.map
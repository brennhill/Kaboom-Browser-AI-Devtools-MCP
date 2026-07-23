/**
 * Purpose: CRUD operations for saving, loading, listing, and deleting browser state snapshots in chrome.storage.local.
 * Docs: docs/features/feature/state-time-travel/index.md
 */
// =============================================================================
// CONSTANTS & TYPES
// =============================================================================
const SNAPSHOT_KEY = 'gasoline_state_snapshots';
// =============================================================================
// CRUD OPERATIONS
// =============================================================================
/**
 * Save a state snapshot to chrome.storage.local
 */
export async function saveStateSnapshot(name, state) {
    return new Promise((resolve) => {
        chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
            const snapshots = result[SNAPSHOT_KEY] || {};
            const sizeBytes = JSON.stringify(state).length; // nosemgrep: no-stringify-keys
            snapshots[name] = {
                ...state,
                name,
                size_bytes: sizeBytes
            };
            chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
                resolve({
                    success: true,
                    snapshot_name: name,
                    size_bytes: sizeBytes
                });
            });
        });
    });
}
/**
 * Load a state snapshot from chrome.storage.local
 */
export async function loadStateSnapshot(name) {
    return new Promise((resolve) => {
        chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
            const snapshots = result[SNAPSHOT_KEY] || {};
            resolve(snapshots[name] || null);
        });
    });
}
/**
 * List all state snapshots with metadata
 */
export async function listStateSnapshots() {
    return new Promise((resolve) => {
        chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
            const snapshots = result[SNAPSHOT_KEY] || {};
            const list = Object.values(snapshots).map((s) => ({
                name: s.name,
                url: s.url,
                timestamp: s.timestamp,
                size_bytes: s.size_bytes
            }));
            resolve(list);
        });
    });
}
/**
 * Delete a state snapshot from chrome.storage.local
 */
export async function deleteStateSnapshot(name) {
    return new Promise((resolve) => {
        chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
            const snapshots = result[SNAPSHOT_KEY] || {};
            delete snapshots[name];
            chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
                resolve({ success: true, deleted: name });
            });
        });
    });
}
//# sourceMappingURL=state-snapshots.js.map
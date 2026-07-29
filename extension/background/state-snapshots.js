import { getLocal, setLocal } from '../lib/storage/local.js';
const SNAPSHOT_KEY = 'kaboom_state_snapshots';
export async function saveStateSnapshot(name, state) {
    const snapshots = (await getLocal(SNAPSHOT_KEY)) || {};
    const sizeBytes = JSON.stringify(state).length;
    snapshots[name] = { ...state, name, size_bytes: sizeBytes };
    await setLocal(SNAPSHOT_KEY, snapshots);
    return { success: true, snapshot_name: name, size_bytes: sizeBytes };
}
export async function loadStateSnapshot(name) {
    const snapshots = (await getLocal(SNAPSHOT_KEY)) || {};
    return snapshots[name] || null;
}
export async function listStateSnapshots() {
    const snapshots = (await getLocal(SNAPSHOT_KEY)) || {};
    return Object.values(snapshots).map(({ name, url, timestamp, size_bytes }) => ({ name, url, timestamp, size_bytes }));
}
export async function deleteStateSnapshot(name) {
    const snapshots = (await getLocal(SNAPSHOT_KEY)) || {};
    delete snapshots[name];
    await setLocal(SNAPSHOT_KEY, snapshots);
    return { success: true, deleted: name };
}
//# sourceMappingURL=state-snapshots.js.map
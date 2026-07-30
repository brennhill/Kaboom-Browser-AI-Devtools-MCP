import { setLocal } from '../lib/storage/local.js';
import { readLocalState } from '../lib/storage/validated.js';
const SNAPSHOT_KEY = 'kaboom_state_snapshots';
function readSnapshots() {
    return readLocalState({
        key: SNAPSHOT_KEY,
        fallback: {},
        validate: (value) => typeof value === 'object' &&
            value !== null &&
            !Array.isArray(value) &&
            Object.values(value).every((snapshot) => typeof snapshot === 'object' &&
                snapshot !== null &&
                typeof snapshot.name === 'string' &&
                typeof snapshot.size_bytes === 'number'),
        diagnostic: {
            name: 'browser_snapshot_state',
            detail: 'Saved browser snapshots were invalid or unreadable; an empty snapshot collection is active.',
            fix: 'Save the required browser snapshots again.'
        }
    });
}
export async function saveStateSnapshot(name, state) {
    const snapshots = await readSnapshots();
    const sizeBytes = JSON.stringify(state).length;
    snapshots[name] = { ...state, name, size_bytes: sizeBytes };
    await setLocal(SNAPSHOT_KEY, snapshots);
    return { success: true, snapshot_name: name, size_bytes: sizeBytes };
}
export async function loadStateSnapshot(name) {
    const snapshots = await readSnapshots();
    return snapshots[name] || null;
}
export async function listStateSnapshots() {
    const snapshots = await readSnapshots();
    return Object.values(snapshots).map(({ name, url, timestamp, size_bytes }) => ({ name, url, timestamp, size_bytes }));
}
export async function deleteStateSnapshot(name) {
    const snapshots = await readSnapshots();
    delete snapshots[name];
    await setLocal(SNAPSHOT_KEY, snapshots);
    return { success: true, deleted: name };
}
//# sourceMappingURL=state-snapshots.js.map
/**
 * Purpose: Persists private environment snapshots locally across service-worker restarts.
 * Why: Recovery handles must survive extension suspension without exposing captured values to the daemon.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
const STORAGE_KEY = 'environment_transaction_snapshots_v1';
const DOCUMENT_VERSION = 1;
export function createPersistentEnvironmentSnapshotStore(deps) {
    const limit = Math.max(1, deps.limit);
    return {
        async save(snapshot) {
            const document = await readDocument(deps);
            const records = [...document.records];
            if (records.length >= limit)
                records.splice(oldestRecordIndex(records), 1);
            const id = deps.newID();
            records.push({ id, created_at: deps.now(), snapshot });
            await writeDocument(deps, { version: DOCUMENT_VERSION, records });
            return id;
        },
        async get(id) {
            const document = await readDocument(deps);
            return document.records.find((record) => record.id === id)?.snapshot;
        },
        async delete(id) {
            const document = await readDocument(deps);
            const records = document.records.filter((record) => record.id !== id);
            if (records.length === document.records.length)
                return;
            await writeDocument(deps, { version: DOCUMENT_VERSION, records });
        }
    };
}
async function readDocument(deps) {
    let stored;
    try {
        stored = await deps.storage.get(STORAGE_KEY);
    }
    catch {
        deps.onNotice('environment_snapshot_store_read_failed');
        throw new Error('environment_snapshot_store_read_failed');
    }
    const candidate = stored[STORAGE_KEY];
    if (candidate === undefined)
        return { version: DOCUMENT_VERSION, records: [] };
    if (isSnapshotDocument(candidate))
        return candidate;
    deps.onNotice('environment_snapshot_store_corrupt');
    try {
        await deps.storage.remove(STORAGE_KEY);
    }
    catch {
        deps.onNotice('environment_snapshot_store_recovery_failed');
        throw new Error('environment_snapshot_store_recovery_failed');
    }
    return { version: DOCUMENT_VERSION, records: [] };
}
async function writeDocument(deps, document) {
    try {
        await deps.storage.set({ [STORAGE_KEY]: document });
    }
    catch {
        deps.onNotice('environment_snapshot_store_write_failed');
        throw new Error('environment_snapshot_store_write_failed');
    }
}
function isSnapshotDocument(value) {
    if (!isRecord(value) || value.version !== DOCUMENT_VERSION || !Array.isArray(value.records))
        return false;
    return value.records.every((record) => isRecord(record) &&
        typeof record.id === 'string' &&
        record.id.length > 0 &&
        typeof record.created_at === 'number' &&
        Number.isFinite(record.created_at) &&
        isEnvironmentSnapshot(record.snapshot));
}
function isEnvironmentSnapshot(value) {
    return (isRecord(value) &&
        typeof value.tab_url === 'string' &&
        typeof value.window_id === 'number' &&
        isRecord(value.page_state) &&
        Array.isArray(value.cookies) &&
        isRestorePlan(value.restore_plan));
}
function isRestorePlan(value) {
    return (isRecord(value) &&
        typeof value.mutated_url === 'string' &&
        typeof value.setup_timeout_ms === 'number' &&
        Array.isArray(value.cookie_names) &&
        value.cookie_names.every((name) => typeof name === 'string') &&
        typeof value.page_state_touched === 'boolean' &&
        typeof value.navigation_changed === 'boolean');
}
function isRecord(value) {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
}
function oldestRecordIndex(records) {
    let oldest = 0;
    for (let index = 1; index < records.length; index += 1) {
        const candidate = records[index];
        const current = records[oldest];
        if (!candidate || !current)
            continue;
        if (candidate.created_at < current.created_at || (candidate.created_at === current.created_at && candidate.id < current.id)) {
            oldest = index;
        }
    }
    return oldest;
}
//# sourceMappingURL=snapshot-store.js.map
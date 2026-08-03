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
            if (records.length >= limit) {
                deps.onNotice('environment_snapshot_store_full');
                throw new Error('environment_snapshot_store_full');
            }
            const id = deps.newID();
            records.push({ id, created_at: deps.now(), snapshot });
            await writeDocument(deps, { ...document, records });
            return id;
        },
        async lookup(id) {
            const document = await readDocument(deps);
            const active = document.records.find((record) => record.id === id);
            if (active)
                return { status: 'active', snapshot: active.snapshot };
            if (document.consumed.some((record) => record.id === id))
                return { status: 'consumed' };
            return { status: 'missing' };
        },
        async consume(id) {
            const document = await readDocument(deps);
            const records = document.records.filter((record) => record.id !== id);
            if (records.length === document.records.length) {
                if (document.consumed.some((record) => record.id === id))
                    return;
                throw new Error('environment_snapshot_store_consume_missing');
            }
            const consumed = [...document.consumed, { id, consumed_at: deps.now() }].slice(-limit);
            await writeDocument(deps, { version: DOCUMENT_VERSION, records, consumed });
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
        return emptyDocument();
    const document = parseSnapshotDocument(candidate);
    if (document)
        return document;
    deps.onNotice('environment_snapshot_store_corrupt');
    try {
        await deps.storage.remove(STORAGE_KEY);
    }
    catch {
        deps.onNotice('environment_snapshot_store_recovery_failed');
        throw new Error('environment_snapshot_store_recovery_failed');
    }
    return emptyDocument();
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
function parseSnapshotDocument(value) {
    if (!isRecord(value) || value.version !== DOCUMENT_VERSION || !Array.isArray(value.records))
        return undefined;
    const recordsValid = value.records.every((record) => isRecord(record) &&
        typeof record.id === 'string' &&
        record.id.length > 0 &&
        typeof record.created_at === 'number' &&
        Number.isFinite(record.created_at) &&
        isEnvironmentSnapshot(record.snapshot));
    const consumed = value.consumed === undefined ? [] : value.consumed;
    if (!recordsValid ||
        !Array.isArray(consumed) ||
        !consumed.every((record) => isRecord(record) &&
            typeof record.id === 'string' &&
            record.id.length > 0 &&
            typeof record.consumed_at === 'number' &&
            Number.isFinite(record.consumed_at))) {
        return undefined;
    }
    return { version: DOCUMENT_VERSION, records: value.records, consumed };
}
function emptyDocument() {
    return { version: DOCUMENT_VERSION, records: [], consumed: [] };
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
//# sourceMappingURL=snapshot-store.js.map
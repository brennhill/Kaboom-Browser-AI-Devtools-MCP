/**
 * Purpose: Persists private environment snapshots locally across service-worker restarts.
 * Why: Recovery handles must survive extension suspension without exposing captured values to the daemon.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import { classifyStorageFailure } from '../../lib/storage/fault.js';
const STORAGE_KEY = 'environment_transaction_snapshots_v1';
const DOCUMENT_VERSION = 1;
function notify(deps, code, faultKind) {
    deps.onNotice({ code, fault_kind: faultKind, lifecycle: 'active' });
}
export function createPersistentEnvironmentSnapshotStore(deps) {
    const limit = Math.max(1, deps.limit);
    let operationTail = Promise.resolve();
    const serialize = (operation) => {
        const result = operationTail.then(operation);
        operationTail = result.then(() => undefined, () => {
            // EXPECTED_ABSENCE: this rejection is expected at the private queue tail
            // because the original caller receives and reports the exact failure.
            // The tail only restores readiness; logging here would duplicate the
            // same incident and misleadingly imply that two operations failed.
        });
        return result;
    };
    return {
        save: (snapshot) => serialize(async () => {
            const document = await readDocument(deps);
            const records = [...document.records];
            if (records.length >= limit) {
                notify(deps, 'environment_snapshot_store_full', 'quota');
                throw new Error('environment_snapshot_store_full');
            }
            const id = deps.newID();
            records.push({ id, created_at: deps.now(), snapshot });
            await writeDocument(deps, { ...document, records });
            return id;
        }),
        lookup: (id) => serialize(async () => {
            const document = await readDocument(deps);
            const active = document.records.find((record) => record.id === id);
            if (active)
                return { status: 'active', snapshot: active.snapshot };
            if (document.consumed.some((record) => record.id === id))
                return { status: 'consumed' };
            return { status: 'missing' };
        }),
        consume: (id) => serialize(async () => {
            const document = await readDocument(deps);
            const records = document.records.filter((record) => record.id !== id);
            if (records.length === document.records.length) {
                if (document.consumed.some((record) => record.id === id))
                    return;
                throw new Error('environment_snapshot_store_consume_missing');
            }
            const consumed = [...document.consumed, { id, consumed_at: deps.now() }].slice(-limit);
            await writeDocument(deps, { version: DOCUMENT_VERSION, records, consumed });
        }),
        reconcile: (activeIDs) => serialize(async () => {
            if (activeIDs.length > limit ||
                activeIDs.some((id) => typeof id !== 'string' || id.length === 0 || id.length > 256)) {
                throw new Error('environment_snapshot_reconcile_invalid');
            }
            const active = new Set(activeIDs);
            const document = await readDocument(deps);
            const records = document.records.filter((record) => active.has(record.id));
            const pruned = document.records.length - records.length;
            if (pruned > 0)
                await writeDocument(deps, { ...document, records });
            return { pruned, retained: records.length };
        })
    };
}
async function readDocument(deps) {
    let stored;
    try {
        stored = await deps.storage.get(STORAGE_KEY);
    }
    catch (error) {
        notify(deps, 'environment_snapshot_store_read_failed', classifyStorageFailure(error, 'read'));
        throw new Error('environment_snapshot_store_read_failed');
    }
    const candidate = stored[STORAGE_KEY];
    // EXPECTED_ABSENCE: no snapshot document is the normal state before the first
    // environment transaction; logging it would misleadingly report first use as recovery.
    if (candidate === undefined)
        return emptyDocument();
    const document = parseSnapshotDocument(candidate);
    if (document)
        return document;
    notify(deps, 'environment_snapshot_store_corrupt', 'corruption');
    try {
        await deps.storage.remove(STORAGE_KEY);
    }
    catch (error) {
        notify(deps, 'environment_snapshot_store_recovery_failed', classifyStorageFailure(error, 'write'));
        throw new Error('environment_snapshot_store_recovery_failed');
    }
    return emptyDocument();
}
async function writeDocument(deps, document) {
    try {
        await deps.storage.set({ [STORAGE_KEY]: document });
    }
    catch (error) {
        notify(deps, 'environment_snapshot_store_write_failed', classifyStorageFailure(error, 'write'));
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
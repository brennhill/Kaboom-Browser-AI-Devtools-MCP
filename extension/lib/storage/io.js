/**
 * Purpose: Normalize callback- and Promise-based Chrome storage I/O.
 * Why: Keep error handling and fire-and-forget write reporting consistent.
 */
import { KABOOM_LOG_PREFIX } from '../brand.js';
import { reportStateRecovery } from './recovery.js';
function isPromiseLike(value) {
    return typeof value === 'object' && value !== null && typeof value.then === 'function';
}
function storageLastError() {
    if (typeof chrome === 'undefined' || !chrome.runtime)
        return null;
    const error = chrome.runtime.lastError;
    return error ? (error.message ?? 'unknown chrome.storage error') : null;
}
export function persist(write, context) {
    void write.catch((error) => {
        console.warn(`${KABOOM_LOG_PREFIX} storage write failed (${context}):`, error);
    });
}
export function readStorage(method, keys) {
    return new Promise((resolve, reject) => {
        let settled = false;
        const finish = (result = {}) => {
            if (settled)
                return;
            settled = true;
            resolve(result);
        };
        try {
            const maybePromise = method(keys, finish);
            if (isPromiseLike(maybePromise)) {
                maybePromise.then((result) => finish(result ?? {})).catch(reject);
            }
        }
        catch (error) {
            reject(error);
        }
    });
}
function runStorageWrite(label, invoke) {
    return new Promise((resolve, reject) => {
        let settled = false;
        const finish = () => {
            if (settled)
                return;
            settled = true;
            const errorMessage = storageLastError();
            if (errorMessage)
                reject(new Error(`chrome.storage ${label} failed: ${errorMessage}`));
            else
                resolve();
        };
        try {
            const maybePromise = invoke(finish);
            if (isPromiseLike(maybePromise)) {
                maybePromise.then(() => finish()).catch(reject);
            }
        }
        catch (error) {
            reject(error);
        }
    });
}
export function writeStorage(method, items) {
    return reportStorageMutationFailure(runStorageWrite('write', (finish) => method(items, finish)), 'saved');
}
export function removeFromStorage(method, keys) {
    return reportStorageMutationFailure(runStorageWrite('remove', (finish) => method(keys, finish)), 'removed');
}
export function setStorageAccessLevel(method, accessLevel) {
    return reportStorageMutationFailure(runStorageWrite('setAccessLevel', (finish) => method({ accessLevel }, finish)), 'configured');
}
async function reportStorageMutationFailure(operation, verb) {
    try {
        await operation;
    }
    catch (error) {
        reportStateRecovery({
            name: 'extension_storage_write_state',
            detail: `Extension state could not be ${verb}; the current in-memory value remains active.`,
            fix: 'Check extension storage permissions, then repeat the affected action.'
        });
        throw error;
    }
}
//# sourceMappingURL=io.js.map
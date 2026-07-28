/**
 * Purpose: Own persistent Chrome local-storage reads and mutations.
 * Why: Keep durable state operations separate from session lifecycle state.
 */
import { readStorage, removeFromStorage, writeStorage } from './io.js';
export async function getLocal(key) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return undefined;
    const result = await readStorage(chrome.storage.local.get.bind(chrome.storage.local), key);
    return result[key];
}
export async function getLocals(keys) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return {};
    return await readStorage(chrome.storage.local.get.bind(chrome.storage.local), keys);
}
export async function setLocal(key, value) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    await writeStorage(chrome.storage.local.set.bind(chrome.storage.local), { [key]: value });
}
export async function setLocals(items) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    await writeStorage(chrome.storage.local.set.bind(chrome.storage.local), items);
}
export async function removeLocal(key) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    await removeFromStorage(chrome.storage.local.remove.bind(chrome.storage.local), [key]);
}
export async function removeLocals(keys) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    await removeFromStorage(chrome.storage.local.remove.bind(chrome.storage.local), keys);
}
//# sourceMappingURL=local.js.map
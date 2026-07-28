/**
 * Purpose: Own Chrome storage change subscriptions.
 * Why: Keep event lifecycle separate from local and session data operations.
 */
export function onStorageChanged(listener) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return () => { };
    chrome.storage.onChanged.addListener(listener);
    return () => chrome.storage.onChanged.removeListener(listener);
}
//# sourceMappingURL=changes.js.map
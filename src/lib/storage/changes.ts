/**
 * Purpose: Own Chrome storage change subscriptions.
 * Why: Keep event lifecycle separate from local and session data operations.
 */

type StorageChange = { oldValue?: unknown; newValue?: unknown }
type StorageChangeListener = (changes: { [key: string]: StorageChange }, areaName: string) => void

export function onStorageChanged(listener: StorageChangeListener): () => void {
  if (typeof chrome === 'undefined' || !chrome.storage) return () => {}
  chrome.storage.onChanged.addListener(listener)
  return () => chrome.storage.onChanged.removeListener(listener)
}

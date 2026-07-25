/**
 * Purpose: Shared wrapper functions for chrome.storage supporting persistent (local) and ephemeral (session) storage with graceful degradation.
 * Why: Abstracts Chrome storage API differences and provides a single facade usable from both background and popup contexts.
 */

import type { ChromeStorageWithSession } from '../types/index.js'
import { KABOOM_LOG_PREFIX } from './brand.js'

type StorageReadResult = Record<string, unknown>
type StorageReadCallback = (result: StorageReadResult) => void
type StorageVoidCallback = () => void
type StorageGetMethod = (keys: string | string[], callback?: StorageReadCallback) => Promise<StorageReadResult> | void
type StorageSetMethod = (items: Record<string, unknown>, callback?: StorageVoidCallback) => Promise<void> | void
type StorageRemoveMethod = (keys: string | string[], callback?: StorageVoidCallback) => Promise<void> | void
type StorageAccessLevelMethod = (
  options: { accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS' },
  callback?: StorageVoidCallback
) => Promise<void> | void

// =============================================================================
// FEATURE DETECTION
// =============================================================================

/**
 * Type-safe access to chrome.storage with session storage support
 * Chrome.storage.session is only available in Chrome 102+
 */
function getStorageWithSession(): ChromeStorageWithSession | null {
  if (typeof chrome === 'undefined' || !chrome.storage) return null
  return chrome.storage as unknown as ChromeStorageWithSession
}

/**
 * Check if chrome.storage.session is available (Chrome 102+)
 */
function isSessionStorageAvailable(): boolean {
  const storage = getStorageWithSession()
  return storage !== null && storage.session !== undefined
}

function isPromiseLike<T>(value: Promise<T> | void): value is Promise<T> {
  return typeof value === 'object' && value !== null && typeof value.then === 'function'
}

/**
 * Read chrome.runtime.lastError inside a storage callback.
 * Chrome sets lastError (and still invokes the callback) when a callback-style
 * write fails — over quota, context invalidated, etc. Returns the message, or
 * null when the write succeeded. Only meaningful synchronously inside the callback.
 */
function storageLastError(): string | null {
  if (typeof chrome === 'undefined' || !chrome.runtime) return null
  const err = chrome.runtime.lastError
  return err ? (err.message ?? 'unknown chrome.storage error') : null
}

/**
 * Fire-and-forget a storage write whose result the caller intentionally does not
 * await, logging (never throwing) if it fails. Keeps non-critical writes honest
 * without leaking unhandled rejections — a mutating write must not fail *silently*
 * (CLAUDE.md rule 25), so we surface the failure in the log instead of swallowing it.
 */
export function persist(write: Promise<void>, context: string): void {
  void write.catch((err) => {
    console.warn(`${KABOOM_LOG_PREFIX} storage write failed (${context}):`, err)
  })
}

function readStorage(method: StorageGetMethod, keys: string | string[]): Promise<StorageReadResult> {
  return new Promise((resolve, reject) => {
    let settled = false
    const finish = (result: StorageReadResult = {}) => {
      if (settled) return
      settled = true
      resolve(result)
    }

    try {
      const maybePromise = method(keys, finish)
      if (isPromiseLike(maybePromise)) {
        maybePromise.then((result) => finish(result ?? {})).catch(reject)
      }
    } catch (error) {
      reject(error)
    }
  })
}

/**
 * Run a callback-or-promise-style chrome.storage *mutation* (set / remove /
 * setAccessLevel) and resolve only once it actually succeeded. All three kinds
 * share one body: dispatch `invoke`, and in the settle step fail loud on
 * chrome.runtime.lastError (reject) rather than trusting a silent no-op
 * (CLAUDE.md rule 25). `label` names the operation in the rejection message;
 * `invoke` owns the single method call so each caller keeps its own arg shape
 * (items / keys / { accessLevel }). readStorage stays separate — it resolves
 * with a result and has no lastError gate.
 */
function runStorageWrite(
  label: 'write' | 'remove' | 'setAccessLevel',
  invoke: (finish: StorageVoidCallback) => Promise<void> | void
): Promise<void> {
  return new Promise((resolve, reject) => {
    let settled = false
    const finish = () => {
      if (settled) return
      settled = true
      const errMsg = storageLastError()
      if (errMsg) reject(new Error(`chrome.storage ${label} failed: ${errMsg}`))
      else resolve()
    }

    try {
      const maybePromise = invoke(finish)
      if (isPromiseLike(maybePromise)) {
        maybePromise.then(() => finish()).catch(reject)
      }
    } catch (error) {
      reject(error)
    }
  })
}

function writeStorage(method: StorageSetMethod, items: Record<string, unknown>): Promise<void> {
  return runStorageWrite('write', (finish) => method(items, finish))
}

function removeFromStorage(method: StorageRemoveMethod, keys: string | string[]): Promise<void> {
  return runStorageWrite('remove', (finish) => method(keys, finish))
}

function setStorageAccessLevel(
  method: StorageAccessLevelMethod,
  accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS'
): Promise<void> {
  return runStorageWrite('setAccessLevel', (finish) => method({ accessLevel }, finish))
}

// =============================================================================
// LOCAL STORAGE (Promise-based)
// =============================================================================

/**
 * Get a persistent value from local storage (async)
 */
export async function getLocal(key: string): Promise<unknown> {
  if (typeof chrome === 'undefined' || !chrome.storage) return undefined
  const result = await readStorage(chrome.storage.local.get.bind(chrome.storage.local), key)
  return result[key]
}

/**
 * Get multiple persistent values from local storage (async)
 */
export async function getLocals(keys: string[]): Promise<Record<string, unknown>> {
  if (typeof chrome === 'undefined' || !chrome.storage) return {}
  return await readStorage(chrome.storage.local.get.bind(chrome.storage.local), keys)
}

/**
 * Set a persistent value in local storage (async)
 */
export async function setLocal(key: string, value: unknown): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.storage) return
  await writeStorage(chrome.storage.local.set.bind(chrome.storage.local), { [key]: value })
}

/**
 * Set multiple persistent values in local storage (async)
 */
export async function setLocals(items: Record<string, unknown>): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.storage) return
  await writeStorage(chrome.storage.local.set.bind(chrome.storage.local), items)
}

/**
 * Remove a persistent value from local storage (async)
 */
export async function removeLocal(key: string): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.storage) return
  await removeFromStorage(chrome.storage.local.remove.bind(chrome.storage.local), [key])
}

/**
 * Remove multiple persistent values from local storage (async)
 */
export async function removeLocals(keys: string[]): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.storage) return
  await removeFromStorage(chrome.storage.local.remove.bind(chrome.storage.local), keys)
}

// =============================================================================
// SESSION STORAGE (Promise-based)
// =============================================================================

/**
 * Get an ephemeral value from session storage (async)
 */
export async function getSession(key: string): Promise<unknown> {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) return undefined
  const result = await readStorage(storage.session.get.bind(storage.session), key)
  return result[key]
}

/**
 * Set an ephemeral value in session storage (async)
 */
export async function setSession(key: string, value: unknown): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) return
  await writeStorage(storage.session.set.bind(storage.session), { [key]: value })
}

/**
 * Remove an ephemeral value from session storage (async)
 */
export async function removeSession(key: string): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) return
  await removeFromStorage(storage.session.remove.bind(storage.session), [key])
}

/**
 * Remove multiple ephemeral values from session storage (async)
 */
export async function removeSessions(keys: string[]): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) return
  await removeFromStorage(storage.session.remove.bind(storage.session), keys)
}

// =============================================================================
// STORAGE CHANGE LISTENER
// =============================================================================

type StorageChange = { oldValue?: unknown; newValue?: unknown }
type StorageChangeListener = (changes: { [key: string]: StorageChange }, areaName: string) => void

/**
 * Register a storage change listener. Returns an unsubscribe function.
 */
export function onStorageChanged(listener: StorageChangeListener): () => void {
  if (typeof chrome === 'undefined' || !chrome.storage) return () => {}
  chrome.storage.onChanged.addListener(listener)
  return () => chrome.storage.onChanged.removeListener(listener)
}

// =============================================================================
// SESSION ACCESS LEVEL
// =============================================================================

/**
 * Set session storage access level (e.g., to allow content scripts access).
 * Required for terminal state persistence in content scripts.
 */
export async function setSessionAccessLevel(
  accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS'
): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage?.session?.setAccessLevel) return
  await setStorageAccessLevel(storage.session.setAccessLevel.bind(storage.session), accessLevel)
}

// =============================================================================
// STATE RECOVERY & DIAGNOSTICS
// =============================================================================

/**
 * Get diagnostic info about storage availability
 */
export function getStorageDiagnostics(): {
  sessionStorageAvailable: boolean
  localStorageAvailable: boolean
  browserVersion: string
} {
  return {
    sessionStorageAvailable: isSessionStorageAvailable(),
    localStorageAvailable: typeof chrome !== 'undefined' && !!chrome.storage?.local,
    browserVersion: navigator.userAgent
  }
}

/**
 * State version key for recovery detection
 */
const STATE_VERSION_KEY = 'kaboom_state_version'
const CURRENT_STATE_VERSION = '1.0.0'

/**
 * Check if service worker was restarted (state version mismatch)
 * Returns true if state was lost/cleared
 */
export async function wasServiceWorkerRestarted(): Promise<boolean> {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) {
    // Can't detect restart without session storage
    return false
  }
  const result = await readStorage(storage.session.get.bind(storage.session), [STATE_VERSION_KEY])
  return result[STATE_VERSION_KEY] !== CURRENT_STATE_VERSION
}

/**
 * Mark the current state version (call on init)
 */
export async function markStateVersion(): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) {
    return
  }
  await writeStorage(storage.session.set.bind(storage.session), { [STATE_VERSION_KEY]: CURRENT_STATE_VERSION })
}

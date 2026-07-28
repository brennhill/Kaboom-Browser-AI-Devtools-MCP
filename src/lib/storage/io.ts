/**
 * Purpose: Normalize callback- and Promise-based Chrome storage I/O.
 * Why: Keep error handling and fire-and-forget write reporting consistent.
 */

import { KABOOM_LOG_PREFIX } from '../brand.js'

export type StorageReadResult = Record<string, unknown>
export type StorageReadCallback = (result: StorageReadResult) => void
export type StorageVoidCallback = () => void
export type StorageGetMethod = (
  keys: string | string[],
  callback?: StorageReadCallback
) => Promise<StorageReadResult> | void
export type StorageSetMethod = (
  items: Record<string, unknown>,
  callback?: StorageVoidCallback
) => Promise<void> | void
export type StorageRemoveMethod = (
  keys: string | string[],
  callback?: StorageVoidCallback
) => Promise<void> | void
export type StorageAccessLevelMethod = (
  options: { accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS' },
  callback?: StorageVoidCallback
) => Promise<void> | void

function isPromiseLike<T>(value: Promise<T> | void): value is Promise<T> {
  return typeof value === 'object' && value !== null && typeof value.then === 'function'
}

function storageLastError(): string | null {
  if (typeof chrome === 'undefined' || !chrome.runtime) return null
  const error = chrome.runtime.lastError
  return error ? (error.message ?? 'unknown chrome.storage error') : null
}

export function persist(write: Promise<void>, context: string): void {
  void write.catch((error) => {
    console.warn(`${KABOOM_LOG_PREFIX} storage write failed (${context}):`, error)
  })
}

export function readStorage(method: StorageGetMethod, keys: string | string[]): Promise<StorageReadResult> {
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

function runStorageWrite(
  label: 'write' | 'remove' | 'setAccessLevel',
  invoke: (finish: StorageVoidCallback) => Promise<void> | void
): Promise<void> {
  return new Promise((resolve, reject) => {
    let settled = false
    const finish = () => {
      if (settled) return
      settled = true
      const errorMessage = storageLastError()
      if (errorMessage) reject(new Error(`chrome.storage ${label} failed: ${errorMessage}`))
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

export function writeStorage(method: StorageSetMethod, items: Record<string, unknown>): Promise<void> {
  return runStorageWrite('write', (finish) => method(items, finish))
}

export function removeFromStorage(method: StorageRemoveMethod, keys: string | string[]): Promise<void> {
  return runStorageWrite('remove', (finish) => method(keys, finish))
}

export function setStorageAccessLevel(
  method: StorageAccessLevelMethod,
  accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS'
): Promise<void> {
  return runStorageWrite('setAccessLevel', (finish) => method({ accessLevel }, finish))
}

/**
 * Purpose: Own ephemeral Chrome session-storage state and lifecycle checks.
 * Why: Keep service-worker recovery and session access policy together.
 */

import type { ChromeStorageWithSession } from '../../types/runtime/chrome.js'
import { readStorage, removeFromStorage, setStorageAccessLevel, writeStorage } from './io.js'

const STATE_VERSION_KEY = 'kaboom_state_version'
const CURRENT_STATE_VERSION = '1.0.0'

function getStorageWithSession(): ChromeStorageWithSession | null {
  if (typeof chrome === 'undefined' || !chrome.storage) return null
  return chrome.storage as unknown as ChromeStorageWithSession
}

function isSessionStorageAvailable(): boolean {
  return getStorageWithSession()?.session !== undefined
}

export async function getSession(key: string): Promise<unknown> {
  const storage = getStorageWithSession()
  if (!storage?.session) return undefined
  const result = await readStorage(storage.session.get.bind(storage.session), key)
  return result[key]
}

export async function setSession(key: string, value: unknown): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage?.session) return
  await writeStorage(storage.session.set.bind(storage.session), { [key]: value })
}

export async function removeSession(key: string): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage?.session) return
  await removeFromStorage(storage.session.remove.bind(storage.session), [key])
}

export async function removeSessions(keys: string[]): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage?.session) return
  await removeFromStorage(storage.session.remove.bind(storage.session), keys)
}

export async function setSessionAccessLevel(
  accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS'
): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage?.session?.setAccessLevel) return
  await setStorageAccessLevel(storage.session.setAccessLevel.bind(storage.session), accessLevel)
}

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

export async function wasServiceWorkerRestarted(): Promise<boolean> {
  const storage = getStorageWithSession()
  if (!storage?.session) return false
  const result = await readStorage(storage.session.get.bind(storage.session), [STATE_VERSION_KEY])
  return result[STATE_VERSION_KEY] !== CURRENT_STATE_VERSION
}

export async function markStateVersion(): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage?.session) return
  await writeStorage(storage.session.set.bind(storage.session), { [STATE_VERSION_KEY]: CURRENT_STATE_VERSION })
}

/**
 * Purpose: Defines typed wrappers for Chrome extension APIs used across messaging and storage flows.
 * Why: Prevents repeated ad-hoc Chrome API typing and reduces integration mistakes across extension modules.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

/**
 * @fileoverview Chrome API Wrapper Types
 * Chrome-specific types for message passing and storage
 */

/**
 * Chrome message sender info
 */
export interface ChromeMessageSender {
  readonly tab?: {
    readonly id?: number
    readonly url?: string
    readonly windowId?: number
  }
  readonly frameId?: number
  readonly url?: string
}

/**
 * Chrome tab info
 */
export interface ChromeTabInfo {
  readonly id?: number
  readonly url?: string
  readonly title?: string
  readonly windowId?: number
  readonly status?: string
  readonly active?: boolean
  readonly favIconUrl?: string
  readonly width?: number
  readonly height?: number
}

/**
 * Chrome storage change info
 */
export interface StorageChange<T = unknown> {
  readonly oldValue?: T
  readonly newValue?: T
}

/**
 * Storage area name
 */
export type StorageAreaName = 'sync' | 'local' | 'session'

/**
 * Chrome Session Storage interface (Chrome 102+)
 * Provides storage that resets on service worker restart
 */
export interface ChromeSessionStorage {
  get(keys: string | string[] | null): Promise<Record<string, unknown>>
  get(keys: string | string[] | null, callback: (result: Record<string, unknown>) => void): void
  set(items: Record<string, unknown>): Promise<void>
  set(items: Record<string, unknown>, callback: () => void): void
  remove(keys: string | string[]): Promise<void>
  remove(keys: string | string[], callback: () => void): void
  clear(): Promise<void>
  clear(callback: () => void): void
  setAccessLevel?(options: {
    accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS'
  }): Promise<void>
}

/**
 * Extended Chrome Storage interface that includes session storage (Chrome 102+)
 */
export interface ChromeStorageWithSession {
  local: chrome.storage.LocalStorageArea
  sync: chrome.storage.SyncStorageArea
  session?: ChromeSessionStorage
}

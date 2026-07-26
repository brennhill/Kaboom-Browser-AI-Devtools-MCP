/**
 * Purpose: Defines pending-query and browser-action request/response contracts exchanged with the daemon.
 * Why: Ensures query dispatch and result handling remain type-safe across extension/background boundaries.
 * Docs: docs/features/feature/query-service/index.md
 */

/**
 * @fileoverview Pending Query Types
 * Query types, browser actions, and tabs information
 */

/**
 * Query types from server
 */
export type QueryType =
  | 'dom'
  | 'a11y'
  | 'execute'
  | 'highlight'
  | 'page_info'
  | 'tabs'
  | 'browser_action'
  | 'waterfall'
  | 'dom_action'
  | 'state_capture'
  | 'state_save'
  | 'state_load'
  | 'state_list'
  | 'state_delete'
  | 'subtitle'
  | 'screenshot'
  | 'screen_recording_start'
  | 'screen_recording_stop'
  | 'link_health'
  | 'draw_mode'
  | 'upload'
  | 'cdp_action'
  | 'explore_page'
  | 'get_readable'
  | 'get_markdown'
  | 'page_summary'

/**
 * Pending query from server
 */
export interface PendingQuery {
  readonly id: string
  readonly type: QueryType
  readonly params: string | Record<string, unknown>
  readonly correlation_id?: string
  readonly tab_id?: number
}

/**
 * Browser action parameters
 */
export interface BrowserActionParams {
  readonly action: 'refresh' | 'navigate' | 'back' | 'forward'
  readonly url?: string
}

/**
 * Browser action result
 */
export interface BrowserActionResult {
  readonly success: boolean
  readonly action?: string
  readonly url?: string
  readonly content_script_status?: 'loaded' | 'refreshed' | 'failed' | 'unavailable'
  readonly message?: string
  readonly error?: string
}

/**
 * Tabs query result
 */
export interface TabInfo {
  readonly id: number
  readonly url: string
  readonly title: string
  readonly active: boolean
  readonly windowId: number
  readonly index: number
}

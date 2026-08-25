/**
 * Purpose: Shared recording helpers used by context menus, keyboard shortcuts, and runtime listeners.
 * Why: Keep recording slug generation consistent across all recording entry points.
 * Docs: docs/features/feature/flow-recording/index.md
 */

/**
 * Request context for starting a recording: how it was initiated and where results resolve.
 */
export interface RecordingStartContext {
  /** PendingQuery ID for result resolution */
  queryId?: string
  /** true when initiated from popup (activeTab already granted, skip reload) */
  fromPopup?: boolean
  /** Explicit target tab (defaults to the active tab) */
  targetTabId?: number
  /** Server connection generation guard */
  connectionGeneration?: number
}

/**
 * Build a filesystem-safe recording slug from the current tab URL.
 */
export function buildScreenRecordingSlug(url: string | undefined): string {
  try {
    const hostname = new URL(url ?? '').hostname.replace(/^www\./, '')
    return (
      hostname
        .replace(/[^a-z0-9]/gi, '-')
        .replace(/-+/g, '-')
        .replace(/^-|-$/g, '') || 'recording'
    )
  } catch {
    // EXPECTED_ABSENCE: malformed page URLs are expected inputs here; logging would mislabel the neutral filename fallback.
    return 'recording'
  }
}

/**
 * Build a short human-readable recording toast label from a tab URL.
 */
export function buildRecordingToastLabel(url: string | undefined): string {
  try {
    const parsed = new URL(url ?? '')
    const host = parsed.hostname.replace(/^www\./, '')
    const path = parsed.pathname === '/' ? '' : parsed.pathname
    const base = `${host}${path}`
    const clipped = base.length > 42 ? `${base.slice(0, 39)}...` : base
    return `Recording ${clipped}`
  } catch {
    // EXPECTED_ABSENCE: malformed page URLs are expected inputs here; logging would mislabel the neutral label fallback.
    return 'Recording started'
  }
}

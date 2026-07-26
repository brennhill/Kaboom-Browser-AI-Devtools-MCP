/**
 * Purpose: Shared draw-mode toggle handshake used by keyboard shortcuts and context-menu actions.
 * Why: Keep draw-mode control behavior consistent across all user entry points.
 * Docs: docs/features/feature/interact-explore/index.md
 */

/**
 * Toggle draw mode for a tab using the current content-script state when available.
 * If state lookup fails, it falls back to attempting a start command.
 */
export async function toggleDrawModeForTab(tabId: number): Promise<void> {
  try {
    const result = (await chrome.tabs.sendMessage(tabId, {
      type: 'kaboom_get_annotations'
    })) as { draw_mode_active?: boolean }

    if (result?.draw_mode_active) {
      await chrome.tabs.sendMessage(tabId, { type: 'kaboom_draw_mode_stop' })
      return
    }
  } catch {
    // Content script might not support state query yet; continue with start fallback.
  }

  await chrome.tabs.sendMessage(tabId, {
    type: 'kaboom_draw_mode_start',
    started_by: 'user'
  })
}

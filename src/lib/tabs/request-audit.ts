/**
 * Purpose: Shared helper that opens the terminal side panel and requests the tracked-site audit workflow.
 * Why: Keeps popup and hover entrypoints aligned on one audit-trigger contract.
 */

/**
 * @fileoverview request-audit.ts - Shared runtime helper for launching the Kaboom audit workflow.
 */

/**
 * @param pageUrl page being audited
 * @param tabId   tab that should host the side panel. REQUIRED from the popup:
 *   a popup is an extension page, so the background sees `sender.tab === undefined`
 *   and cannot resolve a tab synchronously. chrome.sidePanel.open() must be called
 *   without awaiting anything first or Chrome expires the forwarded user gesture and
 *   refuses to open — so the caller has to supply the tab id up front. Content
 *   scripts can omit it; the background falls back to sender.tab.id there.
 */
export async function requestAudit(pageUrl?: string, tabId?: number): Promise<void> {
  try {
    await chrome.runtime.sendMessage({ type: 'open_terminal_panel', tab_id: tabId })
  } catch {
    // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
    // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
    // Best effort: still request the audit workflow even if the side panel failed to open.
  }

  await chrome.runtime.sendMessage({ type: 'qa_scan_requested', page_url: pageUrl })
}

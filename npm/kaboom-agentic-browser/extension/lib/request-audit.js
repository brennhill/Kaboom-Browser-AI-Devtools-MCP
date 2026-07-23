/**
 * Purpose: Shared helper that opens the terminal side panel and requests the tracked-site audit workflow.
 * Why: Keeps popup and hover entrypoints aligned on one audit-trigger contract.
 */
/**
 * @fileoverview request-audit.ts - Shared runtime helper for launching the Kaboom audit workflow.
 */
export async function requestAudit(pageUrl) {
    try {
        await chrome.runtime.sendMessage({ type: 'open_terminal_panel' });
    }
    catch {
        // Best effort: still request the audit workflow even if the side panel failed to open.
    }
    await chrome.runtime.sendMessage({ type: 'qa_scan_requested', page_url: pageUrl });
}
//# sourceMappingURL=request-audit.js.map
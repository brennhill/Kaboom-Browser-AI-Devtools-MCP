/**
 * Purpose: Bring a tab to the foreground — make it the active tab AND focus the
 * window that contains it.
 * Why: Three entry points (the MCP `activate_tab` action, the popup URL click,
 * and background tab-state) all need the same "activate + focus its window"
 * gesture. Centralising it keeps the window-focus step from being forgotten in
 * one place and present in another. Callers layer their own toasts / logging /
 * tracking on top; this helper owns only the two Chrome calls.
 */
/**
 * Activate `tabId` and focus its window. Returns the resolved tab (from the
 * update result, falling back to a fresh get) so callers can read its final
 * index/url/windowId. Throws if the tab cannot be activated — callers that want
 * best-effort behaviour wrap the call in their own try/catch (matching the
 * pre-extraction semantics at each site).
 */
export declare function focusTabAndWindow(tabId: number): Promise<chrome.tabs.Tab>;
//# sourceMappingURL=tab-focus.d.ts.map
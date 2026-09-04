/**
 * Purpose: The popup control that grants or revokes the optional `tabGroups` permission.
 * Why: `tabGroups` carries the Chrome warning "View and manage your tab groups", so
 *      declaring it as required would disable the extension on update until every
 *      existing user re-approved — an unacceptable price for a cosmetic grouping
 *      label (see the contract in tests/extension/contracts/chrome-platform-limits.test.js).
 *      It therefore stays optional, and `chrome.permissions.request()` must be called
 *      "from inside a user gesture, like a button's click handler", which an MV3
 *      service worker never has. This toggle is the only surface that can grant it.
 *      Driving itself never depends on the grant: without it, tabs are driven
 *      ungrouped exactly as before.
 * Docs: docs/features/feature/driven-tab-group/index.md
 */
/** Checkbox that mirrors the live `tabGroups` grant. */
export declare const DRIVEN_GROUP_TOGGLE_ID = "toggle-driven-tab-group";
/** Row wrapping the checkbox, hidden outright on a browser without the permissions API. */
export declare const DRIVEN_GROUP_ROW_ID = "driven-tab-group-row";
/**
 * Wire the driven-tab-group permission toggle. Safe to call when the element is
 * absent (a popup variant that does not render the row) and on a browser with no
 * permissions API, where the row is hidden rather than left as a dead control.
 */
export declare function applyDrivenTabGroupToggle(): void;
//# sourceMappingURL=driven-tab-group-permission.d.ts.map
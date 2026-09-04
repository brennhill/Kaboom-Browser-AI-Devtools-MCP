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
export const DRIVEN_GROUP_TOGGLE_ID = 'toggle-driven-tab-group';
/** Row wrapping the checkbox, hidden outright on a browser without the permissions API. */
export const DRIVEN_GROUP_ROW_ID = 'driven-tab-group-row';
/** A fresh descriptor per call: the Chrome typings take a mutable permission list. */
function tabGroupsPermission() {
    return { permissions: ['tabGroups'] };
}
function toggleElement() {
    return document.getElementById(DRIVEN_GROUP_TOGGLE_ID) ?? null;
}
/**
 * Read the live grant. Chrome is the authority here — never a storage mirror, which
 * would go stale the moment the user revokes the permission from chrome://extensions
 * (CLAUDE.md rule 18).
 */
async function holdsPermission() {
    if (typeof chrome?.permissions?.contains !== 'function')
        return false;
    try {
        return await chrome.permissions.contains(tabGroupsPermission());
    }
    catch (err) {
        console.error('[KaBOOM!] Could not read the tabGroups permission state', err);
        return false;
    }
}
/** Settle the checkbox against what Chrome actually granted, not what was asked for. */
async function reconcileToggle(toggle) {
    toggle.checked = await holdsPermission();
}
/**
 * Handle a check. `chrome.permissions.request` is invoked as the first statement so
 * it runs inside the user gesture; awaiting anything beforehand discards the gesture
 * and makes every grant attempt throw.
 */
function grant(toggle) {
    let requested;
    try {
        requested = chrome.permissions.request(tabGroupsPermission());
    }
    catch (err) {
        console.error('[KaBOOM!] The tabGroups permission request was rejected by Chrome', err);
        void reconcileToggle(toggle);
        return;
    }
    requested
        .catch((err) => {
        console.error('[KaBOOM!] The tabGroups permission request was rejected by Chrome', err);
        return false;
    })
        .then(() => reconcileToggle(toggle))
        .catch((err) => {
        console.error('[KaBOOM!] Could not settle the tab group toggle', err);
    });
}
/** Handle an uncheck. Revoking needs no gesture, but the toggle still follows Chrome. */
function revoke(toggle) {
    const removal = typeof chrome?.permissions?.remove === 'function'
        ? chrome.permissions.remove(tabGroupsPermission())
        : Promise.resolve(false);
    removal
        .catch((err) => {
        console.error('[KaBOOM!] Could not revoke the tabGroups permission', err);
        return false;
    })
        .then(() => reconcileToggle(toggle))
        .catch((err) => {
        console.error('[KaBOOM!] Could not settle the tab group toggle', err);
    });
}
/**
 * Wire the driven-tab-group permission toggle. Safe to call when the element is
 * absent (a popup variant that does not render the row) and on a browser with no
 * permissions API, where the row is hidden rather than left as a dead control.
 */
export function applyDrivenTabGroupToggle() {
    const toggle = toggleElement();
    if (!toggle)
        return;
    if (typeof chrome?.permissions?.request !== 'function') {
        const row = document.getElementById(DRIVEN_GROUP_ROW_ID);
        if (row)
            row.style.display = 'none';
        toggle.checked = false;
        toggle.disabled = true;
        return;
    }
    void reconcileToggle(toggle);
    toggle.addEventListener('change', () => {
        if (toggle.checked)
            grant(toggle);
        else
            revoke(toggle);
    });
}
//# sourceMappingURL=driven-tab-group-permission.js.map
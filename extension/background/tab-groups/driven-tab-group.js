/**
 * Purpose: Own the single named Chrome tab group that holds every tab Kaboom drives.
 * Why: Background driving means the user is not looking at the driven tab. One
 *      titled, coloured group is how they find it, watch it, and close all of it at
 *      once. Every entry point that opens or adopts a tab calls
 *      `adoptTabIntoDrivenGroup`, so the join invariant lives in one helper and no
 *      call site can forget it (CLAUDE.md rule 19). Group membership is presentation,
 *      not enforcement: a tab outside the group is still driven.
 * Docs: docs/features/feature/driven-tab-group/index.md
 */
import { debugLog, DebugCategory } from '../debug.js';
import { errorMessage } from '../../lib/error-utils.js';
import { getConnectionGeneration } from '../runtime-state/connection-generation.js';
import { subscribeExtensionConnection } from '../runtime-state/connection-state.js';
// =============================================================================
// GROUP IDENTITY
// =============================================================================
/**
 * Title carried by every Kaboom-driven group. Deliberately distinct from the
 * terminal workspace group (`KaBOOM!`) so startup reconciliation never ungroups
 * the terminal's tabs while clearing an orphaned driving group.
 */
export const DRIVEN_TAB_GROUP_TITLE = 'KaBOOM! agent';
/** Colour of the driven group. Terminal workspace uses orange; driving is purple. */
export const DRIVEN_TAB_GROUP_COLOR = 'purple';
// =============================================================================
// MODULE STATE (in-memory only — never a storage mirror, CLAUDE.md rule 18)
// =============================================================================
let session = null;
let reconciledThisWorker = false;
let unsubscribeConnection = null;
let lastDegradeReason = null;
// =============================================================================
// DEGRADE PATH
// =============================================================================
/**
 * Record that grouping is unavailable and keep driving ungrouped. Logged once per
 * distinct reason so a denied permission does not spam the diagnostic queue while
 * still being visible in System Doctor (rule 27).
 */
function degrade(reason, detail) {
    if (lastDegradeReason !== reason) {
        lastDegradeReason = reason;
        debugLog(DebugCategory.LIFECYCLE, 'Kaboom tab grouping unavailable; driving continues ungrouped', {
            reason,
            ...detail
        });
    }
    return { adopted: false, degraded_reason: reason };
}
function clearDegrade() {
    lastDegradeReason = null;
}
// =============================================================================
// PERMISSION (granted from the popup toggle, never from here)
// =============================================================================
/** The minimum to put tabs in a titled group — all any caller needs to create one. */
function canCreateTabGroups() {
    return typeof chrome?.tabs?.group === 'function' && typeof chrome?.tabGroups?.update === 'function';
}
/**
 * Everything the *driven* group needs: creation plus the ungroup/query pair that
 * session teardown and startup reconciliation call. The terminal workspace only ever
 * creates, so it is held to `canCreateTabGroups` instead.
 */
function tabGroupApisPresent() {
    return (canCreateTabGroups() &&
        typeof chrome?.tabs?.ungroup === 'function' &&
        typeof chrome?.tabGroups?.query === 'function');
}
/**
 * Whether Kaboom may group tabs at all. `tabGroups` is a required manifest permission,
 * so this is purely a capability question: an old browser without the API. Exported so
 * the terminal workspace grouping asks it in the one place that owns it rather than
 * keeping a second copy that can drift.
 */
export function canGroupTabs() {
    return canCreateTabGroups();
}
/**
 * Resolve whether Kaboom may group tabs right now. Returns `null` when nothing blocks
 * grouping, or the degraded outcome to hand back to the caller.
 *
 * There is no permission check and no permission request here. `tabGroups` is declared
 * in `permissions`, not `optional_permissions`, so it is granted at install and
 * grouping needs no user action — which is the point, since a feature that exists to
 * show which tabs the agent holds is worthless while switched off. Requesting it at
 * runtime was never an option either: Chrome requires `permissions.request` to run
 * "from inside a user gesture, like a button's click handler", and an MV3 service
 * worker never has one.
 */
function groupingBlockedBy() {
    if (!tabGroupApisPresent())
        return degrade('tab_groups_api_unavailable');
    clearDegrade();
    return null;
}
// =============================================================================
// LIVE CHROME STATE
// =============================================================================
/** Ask Chrome, not storage, whether a group still exists (rule 18). */
async function liveGroupExists(groupId) {
    if (typeof chrome?.tabGroups?.get !== 'function')
        return false;
    try {
        const group = await chrome.tabGroups.get(groupId);
        return typeof group?.id === 'number';
    }
    catch {
        // EXPECTED_ABSENCE: closing the group is the documented way for the user to take
        // their tabs back, so a missing group is normal state — logging it would report the
        // user's own deliberate action as a failure.
        return false;
    }
}
/** Ungroup every tab in `groupId`. Returns how many tabs were released. */
async function releaseGroup(groupId, reason) {
    let tabs = [];
    try {
        tabs = await chrome.tabs.query({ groupId });
    }
    catch (err) {
        debugLog(DebugCategory.LIFECYCLE, 'Kaboom could not enumerate its driven tab group to release it', {
            reason,
            group_id: groupId,
            browser_error: errorMessage(err)
        });
        return 0;
    }
    const ids = tabs.map((tab) => tab.id).filter((id) => typeof id === 'number');
    const [first, ...rest] = ids;
    if (first === undefined)
        return 0;
    try {
        await chrome.tabs.ungroup([first, ...rest]);
    }
    catch (err) {
        debugLog(DebugCategory.LIFECYCLE, 'Kaboom could not release its driven tab group', {
            reason,
            group_id: groupId,
            tabs: ids.length,
            browser_error: errorMessage(err)
        });
        return 0;
    }
    debugLog(DebugCategory.LIFECYCLE, 'Released Kaboom driven tab group', {
        reason,
        group_id: groupId,
        tabs: ids.length
    });
    return ids.length;
}
/**
 * Ungroup every Kaboom driving group that is not the current session's, reading
 * live `chrome.tabGroups` state rather than a storage mirror. Run once per worker
 * lifetime before the first adoption, so a daemon or worker that died without
 * ungrouping cannot leave an orphan group behind.
 */
export async function reconcileDrivenTabGroups() {
    if (typeof chrome?.tabGroups?.query !== 'function')
        return 0;
    let groups;
    try {
        groups = await chrome.tabGroups.query({ title: DRIVEN_TAB_GROUP_TITLE });
    }
    catch (err) {
        debugLog(DebugCategory.LIFECYCLE, 'Kaboom could not query live tab groups to reconcile orphans', {
            browser_error: errorMessage(err)
        });
        return 0;
    }
    let released = 0;
    for (const group of groups) {
        if (session !== null && group.id === session.groupId)
            continue;
        released += await releaseGroup(group.id, 'startup_reconcile');
    }
    return released;
}
// =============================================================================
// SESSION LIFECYCLE
// =============================================================================
/** Ungroup the session's tabs and forget the group. Safe to call when idle. */
export async function endDrivenTabGroupSession(reason) {
    const current = session;
    session = null;
    if (current === null)
        return;
    await releaseGroup(current.groupId, reason);
}
/**
 * Watch the daemon connection so the group never outlives the session that made
 * it. Installed on first adoption — before any drive there is no group to orphan.
 */
function installSessionHooks() {
    if (unsubscribeConnection !== null)
        return;
    unsubscribeConnection = subscribeExtensionConnection((connected) => {
        if (connected)
            return;
        endDrivenTabGroupSession('daemon_disconnected').catch((err) => {
            debugLog(DebugCategory.LIFECYCLE, 'Kaboom driven tab group release failed after daemon disconnect', {
                browser_error: errorMessage(err)
            });
        });
    });
}
async function reconcileOncePerWorker() {
    if (reconciledThisWorker)
        return;
    reconciledThisWorker = true;
    await reconcileDrivenTabGroups();
}
/**
 * A new daemon connection generation is a new MCP client session: retire the old
 * group so each session gets its own. Generation 0 means "no sync yet", which is
 * not a session change.
 */
async function rotateOnSessionChange() {
    if (session === null)
        return;
    const generation = getConnectionGeneration();
    if (generation === session.generation)
        return;
    if (session.generation === 0 || generation === 0) {
        session = { groupId: session.groupId, generation: Math.max(session.generation, generation) };
        return;
    }
    await endDrivenTabGroupSession('mcp_session_changed');
}
/** Drop a remembered group the user (or Chrome) has already dissolved. */
async function dropSessionIfGroupGone() {
    if (session === null)
        return;
    if (await liveGroupExists(session.groupId))
        return;
    debugLog(DebugCategory.LIFECYCLE, 'Kaboom driven tab group no longer exists; the next drive opens a fresh one', {
        group_id: session.groupId
    });
    session = null;
}
// =============================================================================
// ADOPTION — the one helper every entry point goes through
// =============================================================================
async function labelDrivenGroup(groupId) {
    try {
        await chrome.tabGroups.update(groupId, {
            title: DRIVEN_TAB_GROUP_TITLE,
            color: DRIVEN_TAB_GROUP_COLOR,
            collapsed: false
        });
    }
    catch (err) {
        debugLog(DebugCategory.LIFECYCLE, 'Kaboom grouped a driven tab but could not title the group', {
            group_id: groupId,
            browser_error: errorMessage(err)
        });
    }
}
async function joinSessionGroup(tabId, entryPoint) {
    const existing = session;
    try {
        const groupId = existing === null
            ? await chrome.tabs.group({ tabIds: [tabId] })
            : await chrome.tabs.group({ tabIds: [tabId], groupId: existing.groupId });
        if (existing === null || existing.groupId !== groupId)
            await labelDrivenGroup(groupId);
        session = { groupId, generation: getConnectionGeneration() };
        clearDegrade();
        debugLog(DebugCategory.LIFECYCLE, 'Kaboom adopted a driven tab into its tab group', {
            tab_id: tabId,
            group_id: groupId,
            entry_point: entryPoint
        });
        return { adopted: true, group_id: groupId };
    }
    catch (err) {
        session = null;
        const reason = `group_failed: ${errorMessage(err)}`;
        debugLog(DebugCategory.LIFECYCLE, 'Kaboom could not add a driven tab to its group; driving continues ungrouped', {
            tab_id: tabId,
            entry_point: entryPoint,
            browser_error: errorMessage(err)
        });
        return { adopted: false, degraded_reason: reason };
    }
}
/**
 * Put `tabId` into Kaboom's driven group, creating and labelling the group on
 * first use. This is the single place the join invariant lives: `new_tab`,
 * `navigate` with `new_tab`, `switch_tab`, and tracked-tab hand-over all call it,
 * so a new entry point cannot ship without it.
 *
 * Never throws and never blocks the drive: a missing permission, a missing API, or
 * a Chrome error resolves to `{ adopted: false, degraded_reason }`.
 */
export async function adoptTabIntoDrivenGroup(tabId, entryPoint) {
    if (!Number.isInteger(tabId) || tabId < 0)
        return { adopted: false, degraded_reason: 'invalid_tab_id' };
    const blocked = groupingBlockedBy();
    if (blocked !== null)
        return blocked;
    installSessionHooks();
    await reconcileOncePerWorker();
    await rotateOnSessionChange();
    await dropSessionIfGroupGone();
    return joinSessionGroup(tabId, entryPoint);
}
/**
 * Reconcile the remembered group after Kaboom closed a driven tab. Chrome deletes
 * a group when its last tab goes, so the id must be dropped rather than reused —
 * otherwise the next drive would target a group that no longer exists.
 */
export async function noteDrivenTabClosed(tabId) {
    if (session === null)
        return;
    const groupId = session.groupId;
    if (await liveGroupExists(groupId))
        return;
    session = null;
    debugLog(DebugCategory.LIFECYCLE, 'Kaboom driven tab group ended with its last tab', {
        tab_id: tabId,
        group_id: groupId
    });
}
/** The group Kaboom currently drives into, or null when it holds none. */
export function getDrivenTabGroupId() {
    return session === null ? null : session.groupId;
}
/** Drop all in-memory ownership so each test starts from a cold worker. */
export function resetDrivenTabGroupStateForTesting() {
    if (unsubscribeConnection !== null)
        unsubscribeConnection();
    unsubscribeConnection = null;
    session = null;
    reconciledThisWorker = false;
    lastDegradeReason = null;
}
//# sourceMappingURL=driven-tab-group.js.map
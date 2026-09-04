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
/**
 * Title carried by every Kaboom-driven group. Deliberately distinct from the
 * terminal workspace group (`KaBOOM!`) so startup reconciliation never ungroups
 * the terminal's tabs while clearing an orphaned driving group.
 */
export declare const DRIVEN_TAB_GROUP_TITLE = "KaBOOM! agent";
/** Colour of the driven group. Terminal workspace uses orange; driving is purple. */
export declare const DRIVEN_TAB_GROUP_COLOR: "purple";
/** Where a tab entered Kaboom's control. Recorded on the adoption log line. */
export type DrivenTabGroupEntryPoint = 'new_tab' | 'switch_tab' | 'tracked_tab';
/** Result of one adoption attempt. `adopted:false` always names why. */
export interface DrivenTabGroupOutcome {
    adopted: boolean;
    group_id?: number;
    degraded_reason?: string;
}
/**
 * Whether Kaboom may group tabs at all: the APIs exist and the optional `tabGroups`
 * permission is held. Exported so the terminal workspace grouping asks this question
 * in the one place that owns it rather than keeping a second copy that can drift.
 */
export declare function canGroupTabs(): Promise<boolean>;
/**
 * Ungroup every Kaboom driving group that is not the current session's, reading
 * live `chrome.tabGroups` state rather than a storage mirror. Run once per worker
 * lifetime before the first adoption, so a daemon or worker that died without
 * ungrouping cannot leave an orphan group behind.
 */
export declare function reconcileDrivenTabGroups(): Promise<number>;
/** Ungroup the session's tabs and forget the group. Safe to call when idle. */
export declare function endDrivenTabGroupSession(reason: string): Promise<void>;
/**
 * Put `tabId` into Kaboom's driven group, creating and labelling the group on
 * first use. This is the single place the join invariant lives: `new_tab`,
 * `navigate` with `new_tab`, `switch_tab`, and tracked-tab hand-over all call it,
 * so a new entry point cannot ship without it.
 *
 * Never throws and never blocks the drive: a missing permission, a missing API, or
 * a Chrome error resolves to `{ adopted: false, degraded_reason }`.
 */
export declare function adoptTabIntoDrivenGroup(tabId: number, entryPoint: DrivenTabGroupEntryPoint): Promise<DrivenTabGroupOutcome>;
/**
 * Reconcile the remembered group after Kaboom closed a driven tab. Chrome deletes
 * a group when its last tab goes, so the id must be dropped rather than reused —
 * otherwise the next drive would target a group that no longer exists.
 */
export declare function noteDrivenTabClosed(tabId: number): Promise<void>;
/** The group Kaboom currently drives into, or null when it holds none. */
export declare function getDrivenTabGroupId(): number | null;
/** Drop all in-memory ownership so each test starts from a cold worker. */
export declare function resetDrivenTabGroupStateForTesting(): void;
//# sourceMappingURL=driven-tab-group.d.ts.map
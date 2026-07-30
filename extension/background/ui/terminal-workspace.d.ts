/**
 * Purpose: Resolves, focuses, groups, and persists the terminal panel's tab workspace.
 * Docs: docs/features/feature/terminal/index.md
 */
export interface TerminalWorkspaceTarget {
    hostTabId: number;
    mainTabId: number;
    tabGroupId: number;
}
export declare function resolveTerminalWorkspaceTarget(requestTabId?: number): Promise<TerminalWorkspaceTarget | null>;
//# sourceMappingURL=terminal-workspace.d.ts.map
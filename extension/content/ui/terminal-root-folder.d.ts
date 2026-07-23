/**
 * Purpose: The root-folder bar above the terminal — shows the working directory
 * and lets the user change it, which relaunches the shell there.
 * Why: A PTY's cwd is fixed at spawn, so pointing the agent at a different repo
 * is a restart, not a setting. It used to live only in the no-session state and
 * on the options page, so with a session running there was no way to see or
 * change where the shell actually was.
 * Docs: docs/features/feature/terminal/index.md
 */
export interface RootFolderBarOptions {
    /** Current root, or '' when the daemon picks one. */
    initialRoot: string;
    /** Apply a new root. The caller restarts the session. */
    onApply: (root: string) => void;
}
/**
 * Build the bar.
 *
 * Returns the element plus a `setRoot` so the panel can reflect a root that
 * changed elsewhere without rebuilding the bar and losing focus mid-typing.
 */
export declare function createRootFolderBar(options: RootFolderBarOptions): {
    element: HTMLDivElement;
    setRoot: (root: string) => void;
};
//# sourceMappingURL=terminal-root-folder.d.ts.map
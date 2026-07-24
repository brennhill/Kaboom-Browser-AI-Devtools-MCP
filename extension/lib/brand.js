/**
 * Purpose: Shared Kaboom brand metadata and user-facing copy helpers for extension surfaces.
 * Docs: docs/features/feature/tab-tracking-ux/index.md, docs/features/feature/terminal/index.md
 */
export const KABOOM_DOCS_URL = 'https://gokaboom.dev/docs';
export const KABOOM_REPOSITORY_URL = 'https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP';
export const KABOOM_DAEMON_COMMAND = 'npx kaboom-agentic-browser';
export const KABOOM_LOG_PREFIX = '[KaBOOM!]';
export const KABOOM_RECORDING_LOG_PREFIX = '[KaBOOM! REC]';
export function getTrackedTabLostToastDetail() {
    return 'Re-enable in KaBOOM! popup';
}
export function getDaemonStartHint() {
    return `Is the KaBOOM! daemon running? Start it with: ${KABOOM_DAEMON_COMMAND}`;
}
export function getReloadedExtensionWarning() {
    return '[KaBOOM!] Please refresh this page. The KaBOOM! extension was reloaded and this page still has the old content script. A page refresh will reconnect capture automatically.';
}
//# sourceMappingURL=brand.js.map
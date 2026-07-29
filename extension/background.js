/**
 * Purpose: Starts the extension background service worker.
 * Why: Keeps the manifest entrypoint limited to runtime initialization.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import { initializeExtension } from './background/init.js';
import { EXTENSION_SESSION_ID } from './background/runtime-state/startup-state.js';
if (typeof globalThis.process === 'undefined') {
    const moduleLoadTime = performance.now();
    console.log(`[DIAGNOSTIC] Module load start at ${moduleLoadTime.toFixed(2)}ms (${new Date().toISOString()})`);
    console.log(`[KaBOOM!] Background service worker loaded - session ${EXTENSION_SESSION_ID}`);
    initializeExtension();
}
//# sourceMappingURL=background.js.map
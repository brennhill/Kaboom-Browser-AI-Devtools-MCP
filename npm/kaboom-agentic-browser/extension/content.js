/**
 * Purpose: Implements the content-script bridge that forwards page telemetry to the extension background worker.
 * Why: Provides the safe boundary between page-context capture hooks and extension runtime message handling.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
/**
 * @fileoverview content.ts - Message bridge between page and extension contexts.
 * Injects inject.js into the page as a module script, then listens for
 * window.postMessage events (KABOOM_LOG, KABOOM_WS, KABOOM_NETWORK_BODY,
 * KABOOM_ENHANCED_ACTION, KABOOM_PERF_SNAPSHOT) and forwards them to the
 * background service worker via chrome.runtime.sendMessage.
 * Also handles chrome.runtime messages for on-demand queries (DOM, a11y, perf).
 * Design: Tab-scoped filtering - only forwards messages from the explicitly
 * tracked tab. Validates message origin (event.source === window) to prevent
 * cross-frame injection. Attaches tabId to all forwarded messages.
 */
import { isDomainCloaked } from './lib/cloaked-domains.js';
import { initTabTracking } from './content/tab-tracking.js';
import { initScriptInjection } from './content/script-injection.js';
import { initRequestTracking, getPendingRequestStats, clearPendingRequests, cleanupRequestTracking } from './content/request-tracking.js';
import { initWindowMessageListener } from './content/window-message-listener.js';
import { initRuntimeMessageListener } from './content/runtime-message-listener.js';
import { initFaviconReplacer } from './content/favicon-replacer.js';
import { setTrackedHoverLauncherEnabled } from './content/ui/tracked-hover-launcher.js';
// Export for testing
export { getPendingRequestStats, clearPendingRequests, cleanupRequestTracking };
// ============================================================================
// INITIALIZATION
// ============================================================================
// Bail out early on cloaked domains — prevents interference with sites
// that break when content scripts are present (e.g. Cloudflare dashboard).
isDomainCloaked().then((cloaked) => {
    if (cloaked)
        return;
    // Track whether scripts have been injected
    let scriptsInjected = false;
    // Initialize tab tracking first, with callback for injection
    initTabTracking((tracked) => {
        if (tracked && !scriptsInjected) {
            initScriptInjection();
            scriptsInjected = true;
        }
        setTrackedHoverLauncherEnabled(tracked);
    });
    // Initialize request tracking (cleanup handlers)
    initRequestTracking();
    // Initialize window message listener
    initWindowMessageListener();
    // Initialize runtime message listener
    initRuntimeMessageListener();
    // Initialize favicon replacer (visual indicator for tracked tabs)
    initFaviconReplacer();
});
//# sourceMappingURL=content.js.map
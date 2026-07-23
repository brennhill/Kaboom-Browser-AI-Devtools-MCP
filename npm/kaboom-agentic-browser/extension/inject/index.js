/**
 * Purpose: Main orchestration and barrel exports for the inject context -- combines API, observers, and message handlers for page-level capture.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview inject/index.ts - Main orchestration and barrel exports
 * Combines API, observers, and message handlers for page-level capture.
 */
// Re-export barrel pattern for tests and consumers
export { safeSerialize, getElementSelector, isSensitiveInput } from '../lib/serialize.js';
export { getContextAnnotations, setContextAnnotation, removeContextAnnotation, clearContextAnnotations } from '../lib/context.js';
export { getImplicitRole, isDynamicClass, computeCssPath, computeSelectors, recordEnhancedAction, getEnhancedActionBuffer, clearEnhancedActionBuffer, generatePlaywrightScript } from '../lib/reproduction.js';
export { recordAction, getActionBuffer, clearActionBuffer, handleClick, handleInput, handleScroll, handleKeydown, handleChange, installActionCapture, uninstallActionCapture, setActionCaptureEnabled, installNavigationCapture, uninstallNavigationCapture } from '../lib/actions.js';
export { parseResourceTiming, getNetworkWaterfall, trackPendingRequest, completePendingRequest, getPendingRequests, clearPendingRequests, getNetworkWaterfallForError, setNetworkWaterfallEnabled, isNetworkWaterfallEnabled, setNetworkBodyCaptureEnabled, isNetworkBodyCaptureEnabled, shouldCaptureUrl, setServerUrl, sanitizeHeaders, truncateRequestBody, truncateResponseBody, readResponseBody, readResponseBodyWithTimeout, wrapFetchWithBodies, wrapXHRWithBodies, unwrapXHR, adoptEarlyBodies } from '../lib/network.js';
export { getPerformanceMarks, getPerformanceMeasures, getCapturedMarks, getCapturedMeasures, installPerformanceCapture, uninstallPerformanceCapture, isPerformanceCaptureActive, getPerformanceSnapshotForError, setPerformanceMarksEnabled, isPerformanceMarksEnabled } from '../lib/performance.js';
export { postLog } from '../lib/bridge.js';
export { installConsoleCapture, uninstallConsoleCapture } from '../lib/console.js';
export { parseStackFrames, parseSourceMap, extractSnippet, extractSourceSnippets, detectFramework, getReactComponentAncestry, captureStateSnapshot, generateAiSummary, enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot, setSourceMapCache, getSourceMapCache, getSourceMapCacheSize } from '../lib/ai-context.js';
export { installExceptionCapture, uninstallExceptionCapture } from '../lib/exceptions.js';
export { getSize, formatPayload, truncateWsMessage, createConnectionTracker, installWebSocketCapture, setWebSocketCaptureMode, setWebSocketCaptureEnabled, getWebSocketCaptureMode, uninstallWebSocketCapture, resetForTesting } from '../lib/websocket.js';
export { executeDOMQuery, getPageInfo, runAxeAudit, runAxeAuditWithTimeout, formatAxeResults } from '../lib/dom-queries.js';
export { mapInitiatorType, aggregateResourceTiming, capturePerformanceSnapshot, installPerfObservers, uninstallPerfObservers, getLongTaskMetrics, getFCP, getLCP, getCLS, getINP, sendPerformanceSnapshot, isPerformanceSnapshotEnabled, setPerformanceSnapshotEnabled } from '../lib/perf-snapshot.js';
// Re-export constants that tests import from inject.js
export { MAX_WATERFALL_ENTRIES, MAX_PERFORMANCE_ENTRIES, SENSITIVE_HEADERS } from '../lib/constants.js';
// Export API module
export { installKaboomAPI, uninstallKaboomAPI } from './api.js';
// Export observer module
export { install, uninstall, wrapFetch, installFetchCapture, uninstallFetchCapture, installXHRCapture, uninstallXHRCapture, installPhase1, installPhase2, getDeferralState, setDeferralEnabled, shouldDeferIntercepts, checkMemoryPressure } from './observers.js';
// Export message handlers module
export { installMessageListener, executeJavaScript, safeSerializeForExecute } from './message-handlers.js';
// Export state management functions
export { captureState, restoreState, highlightElement, clearHighlight } from './state.js';
import { installKaboomAPI } from './api.js';
import { installPhase1 } from './observers.js';
import { installMessageListener } from './message-handlers.js';
import { captureState, restoreState } from './state.js';
import { sendPerformanceSnapshot } from '../lib/perf-snapshot.js';
/**
 * Auto-install when loaded in browser
 */
if (typeof window !== 'undefined' &&
    typeof document !== 'undefined' &&
    typeof globalThis.process === 'undefined') {
    // Install Phase 1 (lightweight API + observers)
    installPhase1();
    // Install message listener with state functions
    installMessageListener(captureState, restoreState);
    // Install Kaboom API
    installKaboomAPI();
    // Send performance snapshot after page load + 2s settling time
    window.addEventListener('load', () => {
        setTimeout(() => {
            sendPerformanceSnapshot();
        }, 2000);
    });
}
//# sourceMappingURL=index.js.map
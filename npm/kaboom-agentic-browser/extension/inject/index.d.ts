/**
 * Purpose: Main orchestration and barrel exports for the inject context -- combines API, observers, and message handlers for page-level capture.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview inject/index.ts - Main orchestration and barrel exports
 * Combines API, observers, and message handlers for page-level capture.
 */
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
export { MAX_WATERFALL_ENTRIES, MAX_PERFORMANCE_ENTRIES, SENSITIVE_HEADERS } from '../lib/constants.js';
export { installKaboomAPI, uninstallKaboomAPI, type KaboomAPI } from './api.js';
export { install, uninstall, wrapFetch, installFetchCapture, uninstallFetchCapture, installXHRCapture, uninstallXHRCapture, installPhase1, installPhase2, getDeferralState, setDeferralEnabled, shouldDeferIntercepts, checkMemoryPressure, type DeferralState } from './observers.js';
export { installMessageListener, executeJavaScript, safeSerializeForExecute } from './message-handlers.js';
export { captureState, restoreState, highlightElement, clearHighlight, type RestoreStateResult, type RestoredCounts, type HighlightResult } from './state.js';
//# sourceMappingURL=index.d.ts.map